package repositories

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
)

// ContractPartyAdapter Запись сведений сторон договора.
type ContractPartyAdapter struct {
	db     *sql.DB
	logger *slog.Logger
}

// NewContractPartyAdapter Создаёт адаптер хранения сведений сторон.
func NewContractPartyAdapter(db *sql.DB, logger *slog.Logger) *ContractPartyAdapter {
	return &ContractPartyAdapter{
		db:     db,
		logger: logger,
	}
}

// SetPartyData записывает ФИО и телефон конкретной стороны заявки.
func (r *ContractPartyAdapter) SetPartyData(
	ctx context.Context,
	listingID listing.ID,
	party contract.Party,
	data listing.ContractData,
) error {
	var (
		query    string
		fullName string
		phone    string
	)

	switch party {
	case contract.PartyTenant:
		query = `UPDATE listings SET tenant_full_name = $1, tenant_phone = $2,
			updated_at = NOW() WHERE id = $3`
		fullName, phone = data.TenantFullName, data.TenantPhone
	case contract.PartyLandlord:
		query = `UPDATE listings SET landlord_full_name = $1, landlord_phone = $2,
			updated_at = NOW() WHERE id = $3`
		fullName, phone = data.LandlordFullName, data.LandlordPhone
	default:
		return fmt.Errorf("unknown contract party: %q", party)
	}

	res, err := r.db.ExecContext(ctx, query, fullName, phone, int64(listingID))
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to save party data", "err", err, "party", party)

		return fmt.Errorf("save party data: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return listing.ErrNotFound
	}

	return nil
}

// SaveDocumentToken сохраняет токен загруженного файла договора.
func (r *ContractPartyAdapter) SaveDocumentToken(
	ctx context.Context,
	listingID listing.ID,
	token string,
) error {
	const query = `UPDATE listings SET contract_document_token = $1, updated_at = NOW() WHERE id = $2`

	res, err := r.db.ExecContext(ctx, query, token, int64(listingID))
	if err != nil {
		r.logger.ErrorContext(ctx, "failed to save document token", "err", err)

		return fmt.Errorf("save document token: %w", err)
	}

	if affected, err := res.RowsAffected(); err == nil && affected == 0 {
		return listing.ErrNotFound
	}

	return nil
}
