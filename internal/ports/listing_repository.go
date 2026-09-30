package ports

import (
	"context"

	domainuser "github.com/Forvi/maxrent/internal/domain/user"

	"github.com/Forvi/maxrent/internal/domain/listing"
)

// ListingRepository Порт хранения заявок арендодателя.
// Реализуется адаптером в repositories.
type ListingRepository interface {
	// Create Сохраняет новую заявку и проставляет её идентификатор.
	Create(ctx context.Context, l listing.Listing) (listing.Listing, error)
	// FindByID Возвращает заявку по идентификатору.
	FindByID(ctx context.Context, id listing.ID) (listing.Listing, error)
	// FindActiveByLandlord Возвращает активную заявку арендодателя
	// (черновик или опубликованную). Если её нет — listing.ErrNoActiveListing.
	FindActiveByLandlord(ctx context.Context, landlordID domainuser.ID) (listing.Listing, error)
	// FindActiveByTenant Возвращает заявку, к которой подключён арендатор.
	// Если он ещё ни к какой не подключён — listing.ErrNoActiveListing.
	FindActiveByTenant(ctx context.Context, tenantID domainuser.ID) (listing.Listing, error)
	// FindByCode Возвращает опубликованную заявку по коду.
	FindByCode(ctx context.Context, code listing.Code) (listing.Listing, error)
	// Update сохраняет изменённую заявку.
	Update(ctx context.Context, l listing.Listing) error
	// CodeTaken сообщает, что код уже используется другой заявкой.
	CodeTaken(ctx context.Context, code listing.Code) (bool, error)
}
