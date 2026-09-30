package listing

// Status Состояние заявки.
type Status string

// Состояния заявки.
const (
	// StatusDraft Заполняется анкета арендодателя.
	StatusDraft Status = "draft"
	// StatusPublished Анкета заполнена, заявка опубликована, код выдан.
	StatusPublished Status = "published"
	// StatusPaired Арендатор подключился, заявка собрана.
	StatusPaired Status = "paired"
	// StatusCancelled Арендодатель отменил заявку.
	StatusCancelled Status = "cancelled"
)

// Title Возвращает состояние для показа пользователю.
func (s Status) Title() string {
	switch s {
	case StatusDraft:
		return "заполняется"
	case StatusPublished:
		return "ждёт арендатора"
	case StatusPaired:
		return "арендатор нашёлся"
	case StatusCancelled:
		return "отменена"
	default:
		return string(s)
	}
}

// Valid Сообщает, что состояние известно.
func (s Status) Valid() bool {
	switch s {
	case StatusDraft, StatusPublished, StatusPaired, StatusCancelled:
		return true
	default:
		return false
	}
}
