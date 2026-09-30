package listing

import (
	"fmt"
	"slices"
)

// Step Текущий вопрос анкеты арендодателя.
type Step string

// Шаги анкеты в порядке задания.
const (
	// StepAddress Адрес объекта.
	StepAddress Step = "address"
	// StepPrice Стоимость аренды в месяц.
	StepPrice Step = "price"
	// StepDeposit Залог.
	StepDeposit Step = "deposit"
	// StepTerm Срок найма.
	StepTerm Step = "term"
	// StepUtilities Кто платит коммунальные.
	StepUtilities Step = "utilities"
	// StepDescription Свободное описание.
	StepDescription Step = "description"
	// StepDone Анкета заполнена.
	StepDone Step = "done"
)

// stepOrder Порядок шагов анкеты.
var stepOrder = []Step{
	StepAddress,
	StepPrice,
	StepDeposit,
	StepTerm,
	StepUtilities,
	StepDescription,
}

// allSteps Все известные шаги, включая терминальный.
var allSteps = append(append([]Step{}, stepOrder...), StepDone)

// ParseStep Восстанавливает шаг из значения, сохранённого в БД.
// Неизвестное значение трактуется как начало анкеты.
func ParseStep(raw string) Step {
	step := Step(raw)
	if slices.Contains(allSteps, step) {
		return step
	}

	return StepAddress
}

// Next Возвращает вопрос, который задаётся после текущего.
// У терминального шага следующего вопроса нет.
func (s Step) Next() (Step, error) {
	if s == StepDone {
		return StepDone, nil
	}

	for i, known := range stepOrder {
		if known != s {
			continue
		}

		if i+1 >= len(stepOrder) {
			return StepDone, nil
		}

		return stepOrder[i+1], nil
	}

	return StepDone, fmt.Errorf("%w: неизвестный шаг %q", ErrInvalidAnswer, s)
}

// Question Текст вопроса для шага.
func (s Step) Question() string {
	switch s {
	case StepAddress:
		return "Укажите адрес объекта: город, улицу, дом, квартиру."
	case StepPrice:
		return "Сколько стоит аренда в месяц? Напишите число, например 35000."
	case StepDeposit:
		return "Какой залог берёте? Напишите число, например 35000. Если без залога — 0."
	case StepTerm:
		return "На какой срок сдаёте?"
	case StepUtilities:
		return "Кто оплачивает коммунальные платежи?"
	case StepDescription:
		return "Опишите объект: состояние, ремонт, мебель, соседи. Можно пропустить — отправьте «-»."
	default:
		return "Анкета заполнена."
	}
}

// UsesButtons Сообщает, что на этот вопрос отвечают кнопками.
func (s Step) UsesButtons() bool {
	return s == StepTerm || s == StepUtilities
}
