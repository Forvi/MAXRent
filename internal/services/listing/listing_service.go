// Package listing Сценарии работы с заявками: анкета арендодателя и подключение арендатора.
package listing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/ports"
)

// codeAttempts Сколько попыток подобрать чужой код допускаем до блокировки.
const codeAttempts = 5

// codeAttemptWindow За окно столько попыток считается перебором.
const codeAttemptWindow = time.Hour

// codeRetries Сколько раз пробуем сгенерировать свободный код.
const codeRetries = 10

// Service Сценарии заявок арендодателя.
type Service struct {
	repo    ports.ListingRepository
	logger  *slog.Logger
	now     func() time.Time
	newCode func() (listing.Code, error)

	// attemptsMu защищает счётчик попыток ввода кода.
	// Счётчик живёт в памяти процесса: после перезапуска лимит сбрасывается.
	attemptsMu sync.Mutex
	attempts   map[domainuser.ID]*attemptsWindow
}

// attemptsWindow Счётчик неудачных попыток входа по коду.
type attemptsWindow struct {
	count int
	since time.Time
}

// NewService Создаёт сервис заявок.
func NewService(repo ports.ListingRepository, logger *slog.Logger) *Service {
	return &Service{
		repo:     repo,
		logger:   logger,
		now:      func() time.Time { return time.Now().UTC() },
		newCode:  listing.GenerateCode,
		attempts: make(map[domainuser.ID]*attemptsWindow),
	}
}

// StartOrResume Возвращает активную заявку арендодателя, а если её нет —
// создаёт новый черновик. Повторный вызов не затирает начатую анкету.
func (s *Service) StartOrResume(ctx context.Context, landlordID domainuser.ID) (listing.Listing, error) {
	active, err := s.repo.FindActiveByLandlord(ctx, landlordID)
	if err == nil {
		return active, nil
	}
	if !errors.Is(err, listing.ErrNoActiveListing) {
		return listing.Listing{}, fmt.Errorf("lookup active listing: %w", err)
	}

	draft := listing.Draft(landlordID, s.now())

	created, err := s.repo.Create(ctx, draft)
	if err != nil {
		return listing.Listing{}, fmt.Errorf("create draft: %w", err)
	}

	s.logger.InfoContext(ctx, "listing draft created",
		"listing_id", created.ID.String(),
		"landlord_id", landlordID.String(),
	)

	return created, nil
}

// Answer Применяет ответ арендодателя к текущему вопросу анкеты.
// Возвращает обновлённую заявку и флаг публикации.
func (s *Service) Answer(ctx context.Context, l listing.Listing, answer string) (listing.Listing, bool, error) {
	if !l.IsDraft() {
		return l, false, fmt.Errorf("%w: анкета уже заполнена", listing.ErrInvalidAnswer)
	}

	// Ошибку валидации возвращаем как есть: обработчик должен показать пользователю
	// уточнение к текущему вопросу, а не техническое сообщение.
	updated, err := l.ApplyAnswer(answer)
	if err != nil {
		return l, false, err
	}

	updated.UpdatedAt = s.now()

	if updated.Step != listing.StepDone {
		if err := s.repo.Update(ctx, updated); err != nil {
			return l, false, fmt.Errorf("save answer: %w", err)
		}

		return updated, false, nil
	}

	published, err := s.publish(ctx, updated)
	if err != nil {
		return l, false, err
	}

	return published, true, nil
}

// publish Подбирает свободный код и переводит заявку в опубликованную.
func (s *Service) publish(ctx context.Context, l listing.Listing) (listing.Listing, error) {
	code, err := s.freeCode(ctx)
	if err != nil {
		return l, fmt.Errorf("generate listing code: %w", err)
	}

	published := l.Complete(code, s.now())
	if err := s.repo.Update(ctx, published); err != nil {
		return l, fmt.Errorf("publish listing: %w", err)
	}

	s.logger.InfoContext(ctx, "listing published",
		"listing_id", published.ID.String(),
		"landlord_id", published.LandlordID.String(),
	)

	return published, nil
}

// freeCode Подбирает код, который ещё не занят.
func (s *Service) freeCode(ctx context.Context) (listing.Code, error) {
	for range codeRetries {
		code, err := s.newCode()
		if err != nil {
			return "", err
		}

		taken, err := s.repo.CodeTaken(ctx, code)
		if err != nil {
			return "", err
		}

		if !taken {
			return code, nil
		}
	}

	return "", listing.ErrCodeTaken
}

// FindByCode Ищет заявку по коду с учётом лимита попыток.
func (s *Service) FindByCode(ctx context.Context, tenantID domainuser.ID, rawCode string) (listing.Listing, error) {
	if s.rateLimited(tenantID) {
		return listing.Listing{}, ErrTooManyAttempts
	}

	code, err := listing.ParseCode(rawCode)
	if err != nil {
		s.registerFailedAttempt(tenantID)

		return listing.Listing{}, err
	}

	found, err := s.repo.FindByCode(ctx, code)
	if err != nil {
		if errors.Is(err, listing.ErrNotFound) {
			s.registerFailedAttempt(tenantID)
		}

		return listing.Listing{}, err
	}

	s.resetAttempts(tenantID)

	return found, nil
}

// Join Подключает арендатора к заявке.
func (s *Service) Join(ctx context.Context, tenantID domainuser.ID, l listing.Listing) (listing.Listing, error) {
	if l.Status != listing.StatusPublished {
		return l, fmt.Errorf("%w: заявка больше не принимает арендаторов", listing.ErrAlreadyPaired)
	}

	if l.LandlordID == tenantID {
		return l, listing.ErrSameUser
	}

	paired := l.WithTenant(tenantID, s.now())
	if err := s.repo.Update(ctx, paired); err != nil {
		return l, fmt.Errorf("pair listing with tenant: %w", err)
	}

	s.logger.InfoContext(ctx, "listing paired",
		"listing_id", paired.ID.String(),
		"landlord_id", paired.LandlordID.String(),
		"tenant_id", tenantID.String(),
	)

	return paired, nil
}

// IsPaired Сообщает, подключился ли арендатор к заявке.
//
// Нужна обработчику, чтобы решить, чьё это сообщение: подключённому арендатору
// шестизначный код не нужен, его ввод принадлежит анкете договора.
func (s *Service) IsPaired(ctx context.Context, tenantID domainuser.ID) (bool, error) {
	found, err := s.repo.FindActiveByTenant(ctx, tenantID)
	if err != nil {
		if errors.Is(err, listing.ErrNoActiveListing) {
			return false, nil
		}

		return false, fmt.Errorf("find active listing by tenant: %w", err)
	}

	return found.Status == listing.StatusPaired, nil
}

// Cancel Отменяет заявку арендодателя.
func (s *Service) Cancel(ctx context.Context, landlordID domainuser.ID) error {
	active, err := s.repo.FindActiveByLandlord(ctx, landlordID)
	if err != nil {
		return err
	}

	if err := s.repo.Update(ctx, active.Cancelled(s.now())); err != nil {
		return fmt.Errorf("cancel listing: %w", err)
	}

	s.logger.InfoContext(ctx, "listing cancelled",
		"listing_id", active.ID.String(),
		"landlord_id", landlordID.String(),
	)

	return nil
}

// ErrTooManyAttempts Превышен лимит попыток ввода кода.
var ErrTooManyAttempts = errors.New("too many code attempts")

// rateLimited Сообщает, что лимит попыток исчерпан.
func (s *Service) rateLimited(userID domainuser.ID) bool {
	window := s.window(userID)
	if window == nil {
		return false
	}

	if s.now().Sub(window.since) > codeAttemptWindow {
		s.resetAttempts(userID)

		return false
	}

	return window.count >= codeAttempts
}

// registerFailedAttempt Учитывает неудачную попытку.
func (s *Service) registerFailedAttempt(userID domainuser.ID) {
	window := s.window(userID)
	if window == nil {
		return
	}

	window.count++
}

// window Возвращает счётчик попыток пользователя, создавая его при первом обращении.
// Возвращённый счётчик менять нужно под блокировкой.
func (s *Service) window(userID domainuser.ID) *attemptsWindow {
	s.attemptsMu.Lock()
	defer s.attemptsMu.Unlock()

	window, ok := s.attempts[userID]
	if !ok {
		window = &attemptsWindow{since: s.now()}
		s.attempts[userID] = window
	}

	return window
}

// resetAttempts Сбрасывает счётчик попыток.
func (s *Service) resetAttempts(userID domainuser.ID) {
	s.attemptsMu.Lock()
	defer s.attemptsMu.Unlock()

	delete(s.attempts, userID)
}
