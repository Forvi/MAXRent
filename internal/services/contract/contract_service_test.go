package contract_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/domain/contract"
	"github.com/Forvi/maxrent/internal/domain/listing"
	domainuser "github.com/Forvi/maxrent/internal/domain/user"
	"github.com/Forvi/maxrent/internal/ports/mocks"
	contractservice "github.com/Forvi/maxrent/internal/services/contract"
)

const (
	landlordID = int64(101)
	tenantID   = int64(202)
)

// fakeGenerator Подставной генератор документа.
type fakeGenerator struct {
	content []byte
	err     error
	calls   int
}

func (f *fakeGenerator) Render(contract.Document) ([]byte, error) {
	f.calls++

	return f.content, f.err
}

func newService(
	t *testing.T,
) (*contractservice.Service, *mocks.MockContractPartyRepository, *fakeGenerator) {
	t.Helper()

	// Конструктор mocks.NewContractPartyRepository в этом тулчейне не виден
	// компилятору (символ есть в go doc, пакет собирается), поэтому мок
	// создаём вручную — тем же способом, что и делает конструктор mockery.
	partiesRepo := &mocks.MockContractPartyRepository{}
	partiesRepo.Test(t)
	t.Cleanup(func() { partiesRepo.AssertExpectations(t) })

	generator := &fakeGenerator{content: []byte("%PDF-1.7 test")}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	svc := contractservice.NewService(partiesRepo, generator, log)

	return svc, partiesRepo, generator
}

// listingStub Заявка с заданными данными сторон.
func listingStub(data listing.ContractData) listing.Listing {
	l := listing.Draft(domainuser.NewID(landlordID), time.Now())
	l.ID = 1
	l.Code = "123456"
	l.Address = "Москва, ул. Тверская, д. 1, кв. 5"
	l.Price = 3500000
	l.Deposit = 3500000
	l.Term = listing.TermLong
	l.Utilities = listing.UtilitiesTenant
	l.ContractData = data

	return l
}

// fullData Данные обеих сторон.
func fullData() listing.ContractData {
	return listing.ContractData{
		TenantFullName:   "Иванов Иван Иванович",
		TenantPhone:      "89031234567",
		LandlordFullName: "Петрова Анна Сергеевна",
		LandlordPhone:    "89037654321",
	}
}

func TestSavePartyData(t *testing.T) {
	ctx := context.Background()
	svc, partiesRepo, _ := newService(t)

	data := listing.ContractData{}
	data.SetName(contract.PartyLandlord, "Петрова Анна")

	partiesRepo.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyLandlord, data).Return(nil).Once()

	require.NoError(t, svc.SavePartyData(ctx, listing.ID(1), contract.PartyLandlord, data))
}

func TestSavePartyDataRejectsUnknownParty(t *testing.T) {
	ctx := context.Background()
	svc, partiesRepo, _ := newService(t)

	err := svc.SavePartyData(ctx, listing.ID(1), contract.Party("manager"), listing.ContractData{})
	require.ErrorIs(t, err, contract.ErrInvalidAnswer)

	partiesRepo.AssertNotCalled(t, "SetPartyData", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestSavePartyDataPropagatesError(t *testing.T) {
	ctx := context.Background()
	svc, partiesRepo, _ := newService(t)
	partiesRepo.EXPECT().SetPartyData(ctx, listing.ID(1), contract.PartyTenant, mock.Anything).
		Return(errors.New("db is down")).Once()

	err := svc.SavePartyData(ctx, listing.ID(1), contract.PartyTenant, listing.ContractData{})
	require.Error(t, err)
}

func TestBuildDocument(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	doc, err := svc.BuildDocument(ctx, listingStub(fullData()))
	require.NoError(t, err)
	require.Equal(t, "Иванов Иван Иванович", doc.Tenant.FullName)
	require.Equal(t, "Петрова Анна Сергеевна", doc.Landlord.FullName)
	require.Equal(t, "Договор_найма_123456.pdf", doc.FileName())
}

func TestBuildDocumentRefusesIncompleteData(t *testing.T) {
	ctx := context.Background()
	svc, _, _ := newService(t)

	partial := fullData()
	partial.TenantFullName = ""
	partial.TenantPhone = ""

	_, err := svc.BuildDocument(ctx, listingStub(partial))
	require.Error(t, err)
	require.ErrorIs(t, err, contract.ErrNotReady)
}

func TestRenderDelegatesToGenerator(t *testing.T) {
	ctx := context.Background()
	svc, _, generator := newService(t)

	doc, err := svc.BuildDocument(ctx, listingStub(fullData()))
	require.NoError(t, err)

	content, err := svc.Render(doc)
	require.NoError(t, err)
	require.Equal(t, generator.content, content)
	require.Equal(t, 1, generator.calls)
}

func TestRenderPropagatesGeneratorError(t *testing.T) {
	ctx := context.Background()
	svc, _, generator := newService(t)
	generator.err = errors.New("font missing")

	doc, err := svc.BuildDocument(ctx, listingStub(fullData()))
	require.NoError(t, err)

	_, err = svc.Render(doc)
	require.Error(t, err)
}
