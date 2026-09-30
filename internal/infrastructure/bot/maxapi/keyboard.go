package maxapi

import "github.com/max-messenger/max-bot-api-client-go/v2/model"

// Keyboard Инлайн-клавиатура в терминах приложения.
// Фичи собирают её через NewKeyboard, не импортируя типы библиотеки MAX.
type Keyboard struct {
	rows [][]Button
}

// Button Кнопка инлайн-клавиатуры. Заполняется через AddCallbackButton.
type Button struct {
	// Text подпись на кнопке.
	Text string
	// Payload значение, которое вернётся боту при нажатии.
	Payload string
}

// NewKeyboard Создаёт пустую клавиатуру.
func NewKeyboard() *Keyboard {
	return &Keyboard{
		rows: make([][]Button, 0),
	}
}

// AddRow Добавляет ряд кнопок и возвращает его для заполнения.
func (k *Keyboard) AddRow() *KeyboardRow {
	k.rows = append(k.rows, nil)

	// Ряд получает указатель на поле владельца и свой индекс, а не копию среза.
	// Копия заголовка среза осталась бы с len = 0, и кнопки не попали бы в
	// клавиатуру: API отвечал бы "proto.payload : errors.empty".
	return &KeyboardRow{rows: &k.rows, index: len(k.rows) - 1}
}

// KeyboardRow Ряд кнопок клавиатуры.
type KeyboardRow struct {
	rows  *[][]Button
	index int
}

// AddCallbackButton Добавляет кнопку, при нажатии которой бот получит payload.
func (r *KeyboardRow) AddCallbackButton(text, payload string) *KeyboardRow {
	(*r.rows)[r.index] = append((*r.rows)[r.index], Button{Text: text, Payload: payload})

	return r
}

// IsEmpty сообщает, что в клавиатуре нет ни одной кнопки.
func (k *Keyboard) IsEmpty() bool {
	for _, row := range k.rows {
		if len(row) > 0 {
			return false
		}
	}

	return true
}

// ToModel конвертирует клавиатуру в тип библиотеки MAX.
// Вызывается только адаптером бота: фичи работают с Keyboard.
// Ряды без кнопок пропускаются: API не принимает payload с пустым рядом.
func (k *Keyboard) ToModel() *model.Keyboard {
	kb := model.NewKeyboard()
	for _, buttons := range k.rows {
		if len(buttons) == 0 {
			continue
		}

		row := kb.AddRow()
		for _, b := range buttons {
			row.AddCallBack(b.Text, b.Payload)
		}
	}

	return kb
}
