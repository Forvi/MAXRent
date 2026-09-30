package repositories

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
)

// listingColumns Колонки заявки в фиксированном порядке —
// их используют и запросы, и сборка сущности при сканировании.
const listingColumns = `id, code, landlord_id, tenant_id, status, step,
	address, price, deposit, term, utilities, description,
	tenant_full_name, tenant_phone, landlord_full_name, landlord_phone,
	contract_document_token, created_at, updated_at`

// ListingRepositoryAdapter Хранение заявок в PostgreSQL.
type ListingRepositoryAdapter struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewListingRepositoryAdapter Создаёт адаптер хранилища заявок.
func NewListingRepositoryAdapter(db *sql.DB, logger *slog.Logger) *ListingRepositoryAdapter {
	return &ListingRepositoryAdapter{
		db:     db,
		logger: logger,
	}
}

// Create Сохраняет новую заявку и проставляет её идентификатор.
func (r *ListingRepositoryAdapter) Create(ctx context.Context, l listing.Listing) (listing.Listing, error) {
	const query = `
		INSERT INTO listings (code, landlord_id, tenant_id, status, step,
			address, price, deposit, term, utilities, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	if err := r.db.QueryRowContext(ctx, query,
		codeToDB(l.Code),
		l.LandlordID.Int64(),
		tenantToDB(l.TenantID),
		string(l.Status),
		string(l.Step),
		addressToDB(l.Address),
		moneyToDB(l.Price),
		moneyToDB(l.Deposit),
		termToDB(l.Term),
		utilitiesToDB(l.Utilities),
		descriptionToDB(l.Description),
		l.CreatedAt,
		l.UpdatedAt,
	).Scan(&l.ID); err != nil {
		r.logger.ErrorContext(ctx, "failed to create listing", "err", err, "landlord_id", l.LandlordID.String())

		return l, fmt.Errorf("create listing: %w", err)
	}

	return l, nil
}

// FindByID Возвращает заявку по идентификатору.
func (r *ListingRepositoryAdapter) FindByID(ctx context.Context, id listing.ID) (listing.Listing, error) {
	const query = `SELECT ` + listingColumns + ` FROM listings WHERE id = $1`

	found, err := scanListing(r.db.QueryRowContext(ctx, query, int64(id)))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return listing.Listing{}, listing.ErrNotFound
		}

		r.logger.ErrorContext(ctx, "failed to find listing", "err", err, "listing_id", id.String())

		return listing.Listing{}, fmt.Errorf("find listing by id: %w", err)
	}

	return found, nil
}

// FindActiveByLandlord Возвращает активную заявку арендодателя:
// черновик или опубликованную.
func (r *ListingRepositoryAdapter) FindActiveByLandlord(
	ctx context.Context,
	landlordID domainuser.ID,
) (listing.Listing, error) {
	const query = `SELECT ` + listingColumns + `
		FROM listings
		WHERE landlord_id = $1 AND status IN ('draft', 'published')
		ORDER BY id DESC
		LIMIT 1`

	found, err := scanListing(r.db.QueryRowContext(ctx, query, landlordID.Int64()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return listing.Listing{}, listing.ErrNoActiveListing
		}

		r.logger.ErrorContext(ctx, "failed to find active listing",
			"err", err, "landlord_id", landlordID.String())

		return listing.Listing{}, fmt.Errorf("find active listing: %w", err)
	}

	return found, nil
}

// FindActiveByTenant Возвращает заявку, к которой подключён арендатор.
func (r *ListingRepositoryAdapter) FindActiveByTenant(
	ctx context.Context,
	tenantID domainuser.ID,
) (listing.Listing, error) {
	const query = `SELECT ` + listingColumns + `
		FROM listings
		WHERE tenant_id = $1 AND status IN ('draft', 'published', 'paired')
		ORDER BY id DESC
		LIMIT 1`

	found, err := scanListing(r.db.QueryRowContext(ctx, query, tenantID.Int64()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return listing.Listing{}, listing.ErrNoActiveListing
		}

		r.logger.ErrorContext(ctx, "failed to find listing by tenant",
			"err", err, "tenant_id", tenantID.String())

		return listing.Listing{}, fmt.Errorf("find active listing by tenant: %w", err)
	}

	return found, nil
}

// FindByCode Возвращает заявку по коду.
// Черновики не находятся (код выдаётся только при публикации),
// отменённые исключены, чтобы старый код не вёл в закрытую сделку.
func (r *ListingRepositoryAdapter) FindByCode(ctx context.Context, code listing.Code) (listing.Listing, error) {
	const query = `SELECT ` + listingColumns + `
		FROM listings
		WHERE code = $1 AND status IN ('published', 'paired')`

	found, err := scanListing(r.db.QueryRowContext(ctx, query, code.String()))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return listing.Listing{}, listing.ErrNotFound
		}

		r.logger.ErrorContext(ctx, "failed to find listing by code", "err", err)

		return listing.Listing{}, fmt.Errorf("find listing by code: %w", err)
	}

	return found, nil
}

// Update сохраняет изменённую заявку.
func (r *ListingRepositoryAdapter) Update(ctx context.Context, l listing.Listing) error {
	const query = `
		UPDATE listings
		SET code = $1, tenant_id = $2, status = $3, step = $4, address = $5,
			price = $6, deposit = $7, term = $8, utilities = $9, description = $10,
			updated_at = $11
		WHERE id = $12`

	res, err := r.db.ExecContext(ctx, query,
		codeToDB(l.Code),
		tenantToDB(l.TenantID),
		string(l.Status),
		string(l.Step),
		addressToDB(l.Address),
		moneyToDB(l.Price),
		moneyToDB(l.Deposit),
		termToDB(l.Term),
		utilitiesToDB(l.Utilities),
		descriptionToDB(l.Description),
		l.UpdatedAt,
		int64(l.ID),
	)
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to update listing", "err", err, "listing_id", l.ID.String())

		return fmt.Errorf("update listing: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return listing.ErrNotFound
	}

	return nil
}

// CodeTaken сообщает, что код уже используется другой заявкой.
func (r *ListingRepositoryAdapter) CodeTaken(ctx context.Context, code listing.Code) (bool, error) {
	const query = `SELECT EXISTS (SELECT 1 FROM listings WHERE code = $1)`

	var taken bool
	if err := r.db.QueryRowContext(ctx, query, code.String()).Scan(&taken); err != nil {
		r.logger.ErrorContext(ctx, "failed to check code availability", "err", err)

		return false, fmt.Errorf("check code availability: %w", err)
	}

	return taken, nil
}

// rowScanner Позволяет читать одинаково результат запроса и *sql.Row.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanListing Собирает заявку из строки результата запроса.
// Значения, уже проверенные ограничениями БД, не валидируются повторно:
// иначе изменение правил сделало бы старые записи нечитаемыми.
func scanListing(row rowScanner) (listing.Listing, error) {
	var (
		id          int64
		code        sql.NullString
		landlordID  int64
		tenantID    sql.NullInt64
		status      string
		step        string
		address     sql.NullString
		price       sql.NullString
		deposit     sql.NullString
		term        sql.NullString
		utilities   sql.NullString
		description sql.NullString
		createdAt   time.Time
		updatedAt   time.Time

		tenantName    sql.NullString
		tenantPhone   sql.NullString
		landlordName  sql.NullString
		landlordPhone sql.NullString
		documentToken sql.NullString
	)

	if err := row.Scan(
		&id, &code, &landlordID, &tenantID, &status, &step,
		&address, &price, &deposit, &term, &utilities, &description,
		&tenantName, &tenantPhone, &landlordName, &landlordPhone, &documentToken,
		&createdAt, &updatedAt,
	); err != nil {
		//nolint:wrapcheck // sql.ErrNoRows разбирается вызывающим через errors.Is
		return listing.Listing{}, err
	}

	result := listing.Listing{
		ID:          listing.ID(id),
		LandlordID:  domainuser.NewID(landlordID),
		Status:      listing.Status(status),
		Step:        listing.ParseStep(step),
		Address:     listing.Address(address.String),
		Term:        listing.Term(term.String),
		Utilities:   listing.Utilities(utilities.String),
		Description: description.String,
		ContractData: listing.ContractData{
			TenantFullName:   tenantName.String,
			TenantPhone:      tenantPhone.String,
			LandlordFullName: landlordName.String,
			LandlordPhone:    landlordPhone.String,
			DocumentToken:    documentToken.String,
		},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if code.Valid && code.String != "" {
		parsed, err := listing.ParseCode(code.String)
		if err != nil {
			return listing.Listing{}, fmt.Errorf("stored code is invalid: %w", err)
		}

		result.Code = parsed
	}

	if tenantID.Valid {
		tenant := domainuser.NewID(tenantID.Int64)
		result.TenantID = &tenant
	}

	// Деньги читаем текстом: numeric → float64 → копейки теряет точность.
	if price.Valid && price.String != "" {
		parsed, err := listing.ParseMoney(price.String)
		if err != nil {
			return listing.Listing{}, fmt.Errorf("stored price is invalid: %w", err)
		}

		result.Price = parsed
	}

	if deposit.Valid && deposit.String != "" {
		parsed, err := listing.ParseMoney(deposit.String)
		if err != nil {
			return listing.Listing{}, fmt.Errorf("stored deposit is invalid: %w", err)
		}

		result.Deposit = parsed
	}

	return result, nil
}

// codeToDB Код у черновика пуст и хранится как NULL:
// несколько NULL не нарушают ограничение UNIQUE.
func codeToDB(code listing.Code) any {
	if code == "" {
		return nil
	}

	return code.String()
}

// tenantToDB Арендатор ещё не подключился — NULL.
func tenantToDB(tenantID *domainuser.ID) any {
	if tenantID == nil {
		return nil
	}

	return tenantID.Int64()
}

// addressToDB Пустой адрес хранится как NULL, а не пустой строкой.
func addressToDB(address listing.Address) any {
	if address == "" {
		return nil
	}

	return address.String()
}

// moneyToDB Деньги уходят в numeric в рублях, а не в копейках.
func moneyToDB(money listing.Money) any {
	if money == 0 {
		return nil
	}

	return float64(money) / 100
}

// termToDB Пустой срок хранится как NULL.
func termToDB(term listing.Term) any {
	if term == "" {
		return nil
	}

	return string(term)
}

// utilitiesToDB Пустой вариант оплаты хранится как NULL.
func utilitiesToDB(utilities listing.Utilities) any {
	if utilities == "" {
		return nil
	}

	return string(utilities)
}

// descriptionToDB Пустое описание хранится как NULL.
func descriptionToDB(description string) any {
	if description == "" {
		return nil
	}

	return description
}
