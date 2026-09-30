package listing_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/domain/listing"
	"github.com/Forvi/maxrent/internal/testutil/listingfixture"
)

// userID Арендодатель, от имени которого создаются заявки в тестах.
var userID = listingfixture.LandlordID

func TestCodeIsSixDigits(t *testing.T) {
	seen := make(map[listing.Code]struct{}, 50)

	for range 50 {
		code, err := listing.GenerateCode()
		require.NoError(t, err)
		require.Len(t, code.String(), listing.CodeLength)

		// Ведущий ноль сделал бы «012345» неотличимым от «12345».
		require.NotEqual(t, '0', rune(code.String()[0]))
		seen[code] = struct{}{}
	}

	// Случайность проверяем косвенно: 50 шестизначных кодов почти наверняка
	// не все совпадут, хотя полное совпадение теоретически возможно.
	require.Greater(t, len(seen), 1, "генератор выдаёт один и тот же код")
}

func TestParseCode(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    listing.Code
		wantErr bool
	}{
		{name: "plain", raw: "123456", want: "123456"},
		{name: "with spaces", raw: "  123456  ", want: "123456"},
		{name: "too short", raw: "12345", wantErr: true},
		{name: "too long", raw: "1234567", wantErr: true},
		{name: "letters", raw: "12345a", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := listing.ParseCode(tt.raw)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, listing.ErrInvalidAnswer)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestMoney(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    listing.Money
		wantStr string
		wantErr bool
	}{
		{name: "rubles", raw: "35000", want: 3500000, wantStr: "35000"},
		{name: "with spaces", raw: "35 000", want: 3500000, wantStr: "35000"},
		{name: "kopeks", raw: "35000,50", want: 3500050, wantStr: "35000,50"},
		{name: "one kopeks digit", raw: "35000,5", want: 3500050, wantStr: "35000,50"},
		{name: "zero", raw: "0", want: 0, wantStr: "0"},
		{name: "negative", raw: "-100", wantErr: true},
		{name: "letters", raw: "много", wantErr: true},
		{name: "three decimals", raw: "100,123", wantErr: true},
		{name: "empty", raw: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := listing.ParseMoney(tt.raw)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, listing.ErrInvalidAnswer)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
			require.Equal(t, tt.wantStr, got.String())
		})
	}
}

func TestAddress(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "normal", raw: "Москва, ул. Тверская, д. 1, кв. 5"},
		{name: "extra spaces", raw: "  Москва,   Тверская  1  "},
		{name: "too short", raw: "Москва", wantErr: true},
		{name: "punctuation only", raw: ".,;:!!!!!!!!!", wantErr: true},
		{name: "empty", raw: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := listing.NewAddress(tt.raw)

			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, listing.ErrInvalidAnswer)

				return
			}

			require.NoError(t, err)
			require.NotEmpty(t, got.String())
		})
	}
}

func TestStepOrder(t *testing.T) {
	step := listing.StepAddress

	want := []listing.Step{
		listing.StepPrice,
		listing.StepDeposit,
		listing.StepTerm,
		listing.StepUtilities,
		listing.StepDescription,
		listing.StepDone,
	}

	for _, expected := range want {
		next, err := step.Next()
		require.NoError(t, err)
		require.Equal(t, expected, next)
		step = next
	}

	// Заполненная анкета не задаёт новых вопросов.
	_, err := step.Next()
	require.NoError(t, err)
}

func TestDraftStartsAtAddress(t *testing.T) {
	draft := listing.Draft(userID, time.Now())

	require.Equal(t, listing.StatusDraft, draft.Status)
	require.Equal(t, listing.StepAddress, draft.Step)
	require.True(t, draft.IsDraft())
	require.True(t, draft.IsActive())
	require.False(t, draft.HasCode())
}

func TestApplyAnswerWalksThroughAllSteps(t *testing.T) {
	answers := listingfixture.Answers()
	current := listing.Draft(userID, time.Now())

	for current.Step != listing.StepDone {
		answer, ok := answers[current.Step]
		require.True(t, ok, "нет ответа для шага %s", current.Step)

		updated, err := current.ApplyAnswer(answer)
		require.NoError(t, err)
		current = updated
	}

	require.Equal(t, "Москва, ул. Тверская, д. 1, кв. 5", current.Address.String())
	require.Equal(t, listing.Money(3500000), current.Price)
	require.Equal(t, listing.Money(3500000), current.Deposit)
	require.Equal(t, listing.TermShort, current.Term)
	require.Equal(t, listing.UtilitiesShared, current.Utilities)
	require.Equal(t, "Свежий ремонт, мебель остаётся", current.Description)
}

func TestApplyAnswerRejectsBadValues(t *testing.T) {
	tests := []struct {
		name  string
		step  listing.Step
		input string
	}{
		{name: "short address", step: listing.StepAddress, input: "Москва"},
		{name: "zero price", step: listing.StepPrice, input: "0"},
		{name: "negative deposit", step: listing.StepDeposit, input: "-1"},
		{name: "text instead of term", step: listing.StepTerm, input: "на год"},
		{name: "unknown utilities", step: listing.StepUtilities, input: "utilities_magic"},
		{name: "too long description", step: listing.StepDescription, input: longText(1001)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			draft := listing.Draft(userID, time.Now())
			draft.Step = tt.step

			_, err := draft.ApplyAnswer(tt.input)
			require.Error(t, err)
			require.ErrorIs(t, err, listing.ErrInvalidAnswer)
		})
	}
}

func TestZeroDepositIsAllowed(t *testing.T) {
	draft := listing.Draft(userID, time.Now())
	draft.Step = listing.StepDeposit

	updated, err := draft.ApplyAnswer("0")
	require.NoError(t, err)
	require.Equal(t, listing.Money(0), updated.Deposit)
}

func TestPriceMustBePositive(t *testing.T) {
	draft := listing.Draft(userID, time.Now())
	draft.Step = listing.StepPrice

	_, err := draft.ApplyAnswer("0")
	require.Error(t, err, "нулевая аренда — ошибка ввода")
	require.ErrorIs(t, err, listing.ErrInvalidAnswer)
}

func TestCompletePublishesWithCode(t *testing.T) {
	draft := listing.Draft(userID, time.Now())
	draft.Address = "Москва, ул. Тверская, д. 1"

	published := draft.Complete("123456", time.Now())

	require.Equal(t, "123456", published.Code.String())
	require.Equal(t, listing.StatusPublished, published.Status)
	require.Equal(t, listing.StepDone, published.Step)
	require.True(t, published.IsActive())
}

func TestWithTenantPairs(t *testing.T) {
	draft := listing.Draft(userID, time.Now())
	published := draft.Complete("123456", time.Now())

	paired := published.WithTenant(userID, time.Now())

	require.Equal(t, listing.StatusPaired, paired.Status)
	require.NotNil(t, paired.TenantID)
	require.Equal(t, userID, *paired.TenantID)
	// Собственная заявка не должна проходить проверку в сервисе.
	require.Equal(t, userID, paired.LandlordID)
}

func TestCancelledIsInactive(t *testing.T) {
	draft := listing.Draft(userID, time.Now())
	published := draft.Complete("123456", time.Now())

	cancelled := published.Cancelled(time.Now())

	require.Equal(t, listing.StatusCancelled, cancelled.Status)
	require.False(t, cancelled.IsActive())
	// Код сохраняется, чтобы старый код не достался новой заявке.
	require.Equal(t, "123456", cancelled.Code.String())
}

func TestStepQuestionsAndButtons(t *testing.T) {
	require.False(t, listing.StepAddress.UsesButtons())
	require.False(t, listing.StepPrice.UsesButtons())
	require.False(t, listing.StepDescription.UsesButtons())
	require.True(t, listing.StepTerm.UsesButtons())
	require.True(t, listing.StepUtilities.UsesButtons())

	for _, step := range []listing.Step{
		listing.StepAddress, listing.StepPrice, listing.StepDeposit,
		listing.StepTerm, listing.StepUtilities, listing.StepDescription,
	} {
		require.NotEmpty(t, step.Question(), "у шага %s должен быть текст вопроса", step)
	}
}

func longText(n int) string {
	const filler = "абвгдёжзийклмнопрстуфхцчшщъыьэюя "

	out := make([]rune, 0, n)
	for range n {
		out = append(out, []rune(filler)...)
	}

	return string(out[:n])
}
