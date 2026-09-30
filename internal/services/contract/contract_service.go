// Package contract Сценарии подготовки договора найма.
package contract

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/ports"
)

// GeneratorPort Порт формирования документа.
type GeneratorPort interface {
	// Render возвращает содержимое PDF-файла договора.
	Render(d contract.Document) ([]byte, error)
}

// Service Сценарии сбора данных сторон и формирования договора.
type Service struct {
	parties   ports.ContractPartyRepository
	generator GeneratorPort
	logger    *slog.Logger
	now       func() time.Time
}

// NewService Создаёт сервис договоров.
func NewService(
	parties ports.ContractPartyRepository,
	generator GeneratorPort,
	logger *slog.Logger,
) *Service {
	return &Service{
		parties:   parties,
		generator: generator,
		logger:    logger,
		now:       func() time.Time { return time.Now().UTC() },
	}
}

// SavePartyData Сохраняет сведения сторон заявки.
//
// Поля анкеты собираются по одному, поэтому валидируются в домене
// (contract.ValidateName / contract.ValidatePhone), а сервис только сохраняет.
// Заявку заново не читаем: обработчик уже её загрузил.
func (s *Service) SavePartyData(
	ctx context.Context,
	listingID listing.ID,
	party contract.Party,
	data listing.ContractData,
) error {
	if party != contract.PartyTenant && party != contract.PartyLandlord {
		return fmt.Errorf("%w: неизвестная сторона %q", contract.ErrInvalidAnswer, party)
	}

	if err := s.parties.SetPartyData(ctx, listingID, party, data); err != nil {
		return fmt.Errorf("save party data: %w", err)
	}

	s.logger.InfoContext(ctx, "party data saved",
		"party", party, "listing_id", listingID.String())

	return nil
}

// BuildDocument Собирает договор по заявке и данным обеих сторон.
func (s *Service) BuildDocument(ctx context.Context, l listing.Listing) (contract.Document, error) {
	tenant, err := contract.NewPerson(l.ContractData.TenantFullName, l.ContractData.TenantPhone)
	if err != nil {
		return contract.Document{}, fmt.Errorf("%w: наниматель: %w", contract.ErrNotReady, err)
	}

	landlord, err := contract.NewPerson(l.ContractData.LandlordFullName, l.ContractData.LandlordPhone)
	if err != nil {
		return contract.Document{}, fmt.Errorf("%w: наймодатель: %w", contract.ErrNotReady, err)
	}

	doc := contract.NewDocument(l, tenant, landlord, s.now())
	if !doc.IsReady() {
		return contract.Document{}, fmt.Errorf("%w: не хватает данных: %s",
			contract.ErrNotReady, strings.Join(doc.MissingFields(), ", "))
	}

	return doc, nil
}

// Render Формирует PDF-договор.
func (s *Service) Render(doc contract.Document) ([]byte, error) {
	content, err := s.generator.Render(doc)
	if err != nil {
		return nil, fmt.Errorf("render contract: %w", err)
	}

	return content, nil
}

// PartyFor Сопоставляет роль пользователя стороне договора.
// Пустая строка означает, что роль неизвестна.
func PartyFor(role domainuser.Role) contract.Party {
	switch role {
	case domainuser.RoleTenant:
		return contract.PartyTenant
	case domainuser.RoleLandlord:
		return contract.PartyLandlord
	default:
		return ""
	}
}
