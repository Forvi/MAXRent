package ports

import (
	"context"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
)

// ContractPartyRepository Порт работы со сведениями сторон договора.
// Реализуется адаптером в repositories.
type ContractPartyRepository interface {
	// SetPartyData записывает ФИО и телефон конкретной стороны заявки.
	SetPartyData(ctx context.Context, listingID listing.ID, party contract.Party, data listing.ContractData) error
	// SaveDocumentToken сохраняет токен загруженного файла договора.
	SaveDocumentToken(ctx context.Context, listingID listing.ID, token string) error
}
