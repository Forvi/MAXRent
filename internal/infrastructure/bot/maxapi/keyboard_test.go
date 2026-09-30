package maxapi_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Forvi/maxrent/internal/infrastructure/bot/maxapi"
)

func TestKeyboardToModelKeepsButtons(t *testing.T) {
	kb := maxapi.NewKeyboard()
	kb.AddRow().AddCallbackButton("Арендатор", "role_tenant")
	kb.AddRow().AddCallbackButton("Арендодатель", "role_landlord")

	require.False(t, kb.IsEmpty(), "клавиатура с кнопками не должна считаться пустой")

	// Конвертация в типы MAX: именно здесь раньше терялись кнопки,
	// и API отвечал "proto.payload : errors.empty".
	converted := kb.ToModel().Build()
	require.Len(t, converted.Payload.Buttons, 2, "ожидались два ряда кнопок")

	first := converted.Payload.Buttons[0]
	require.Len(t, first, 1)
	require.Equal(t, "Арендатор", first[0].Text)
	require.Equal(t, "role_tenant", first[0].Payload)
	require.Equal(t, "callback", string(first[0].Type))

	second := converted.Payload.Buttons[1]
	require.Len(t, second, 1)
	require.Equal(t, "Арендодатель", second[0].Text)
	require.Equal(t, "role_landlord", second[0].Payload)
}

func TestKeyboardSeveralButtonsInOneRow(t *testing.T) {
	kb := maxapi.NewKeyboard()
	kb.AddRow().
		AddCallbackButton("Арендатор", "role_tenant").
		AddCallbackButton("Арендодатель", "role_landlord")

	converted := kb.ToModel().Build()
	require.Len(t, converted.Payload.Buttons, 1, "ожидался один ряд")
	require.Len(t, converted.Payload.Buttons[0], 2, "в ряду должно быть две кнопки")
}

func TestKeyboardRowsSurviveLaterReallocation(t *testing.T) {
	// Ряды добавляются по одному: каждый AddRow перевыделяет срез,
	// указатель на поле владельца обязан остаться действительным.
	kb := maxapi.NewKeyboard()
	kb.AddRow().AddCallbackButton("А", "a")

	for range 5 {
		kb.AddRow().AddCallbackButton("Б", "b")
	}

	converted := kb.ToModel().Build()
	require.Len(t, converted.Payload.Buttons, 6)
	require.Equal(t, "a", converted.Payload.Buttons[0][0].Payload)
	require.Equal(t, "b", converted.Payload.Buttons[5][0].Payload)
}

func TestEmptyKeyboardHasNoButtonsInJSON(t *testing.T) {
	kb := maxapi.NewKeyboard()
	kb.AddRow() // ряд без кнопок

	require.True(t, kb.IsEmpty())

	// Пустая клавиатура не должна уходить в API: payload без buttons
	// сервер отклоняет. Адаптер проверяет IsEmpty перед отправкой.
	converted := kb.ToModel().Build()
	require.Empty(t, converted.Payload.Buttons)
}

// TestKeyboardJSONIsNotEmpty — проверка уровня, на котором валидирует API.
// Кнопки с payload обязаны попасть в JSON, иначе MAX вернёт errors.empty.
func TestKeyboardJSONIsNotEmpty(t *testing.T) {
	kb := maxapi.NewKeyboard()
	kb.AddRow().AddCallbackButton("Арендатор", "role_tenant")

	converted := kb.ToModel().Build()
	raw, err := json.Marshal(converted)
	require.NoError(t, err)
	require.Contains(t, string(raw), "role_tenant", "payload кнопки обязан сериализоваться")

	var decoded struct {
		Payload struct {
			Buttons [][]struct {
				Text    string `json:"text"`
				Payload string `json:"payload"`
			} `json:"buttons"`
		} `json:"payload"`
	}
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.Len(t, decoded.Payload.Buttons, 1, "payload не должен оказаться пустым")
	require.Len(t, decoded.Payload.Buttons[0], 1)
}
