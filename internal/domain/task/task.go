package task

import "time"

type Status string
type Recurrence string

const (
	StatusNew        Status = "new"
	StatusInProgress Status = "in_progress"
	StatusDone       Status = "done"
)

type Task struct {
	ID          int64     `json:"id"`
	ParentID    *int64    `json:"parent_id,omitempty"` // ID шаблона (null, если задача разовая)
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Status      Status    `json:"status"`
	DueDate     time.Time `json:"due_date"` // дата, на которую запланирована задача
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

const (
	Daily   Recurrence = "daily"
	Monthly Recurrence = "monthly"
	Dates   Recurrence = "dates"
	Parity  Recurrence = "parity"
)

type TaskTemplate struct {
	ID              int64       `json:"id"`
	Title           string      `json:"title"`
	Description     string      `json:"description"`
	Type            Recurrence  `json:"type"`
	Interval        int         `json:"interval,omitempty"`
	DayOfMonth      int         `json:"day_of_month,omitempty"`
	SpecificDays    []time.Time `json:"specific_days,omitempty"`
	IsActive        bool        `json:"is_active"`
	StartsAt        *time.Time  `json:"starts_at"`
	Parity          string      `json:"parity,omitempty"`
	LastGeneratedAt *time.Time  `json:"last_generated_at"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
}

type RecurrenceRule struct {
	Type         Recurrence  `json:"type"`                    // daily, monthly, parity, specific_days
	Interval     int         `json:"interval"`                // (каждый n-й день)
	DayOfMonth   int         `json:"day_of_month"`            // 1-31(для monthly)(если у месяца дата будет выходить за рамки, то мы ставим просто последний день)
	SpecificDays []time.Time `json:"specific_days,omitempty"` //(для  specific_days)
	Parity       string      `json:"parity"`                  // "even" или "odd" (для parity)
	StartsAt     *time.Time  `json:"starts_at"`               // Если nil, то с текущего момента
}

func (s Status) Valid() bool {
	switch s {
	case StatusNew, StatusInProgress, StatusDone:
		return true
	default:
		return false
	}
}
