package handlers

import (
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type taskMutationDTO struct {
	Title       string                     `json:"title"`
	Description string                     `json:"description"`
	Status      taskdomain.Status          `json:"status"`
	Recurrence  *taskdomain.RecurrenceRule `json:"recurrence,omitempty"`
}

type taskDTO struct {
	ID          int64             `json:"id"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Status      taskdomain.Status `json:"status"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}
type taskTemplateDTO struct {
	ID           int64                 `json:"id"`
	Title        string                `json:"title"`
	Description  string                `json:"description"`
	Type         taskdomain.Recurrence `json:"type"`
	Interval     int                   `json:"interval,omitempty"`
	DayOfMonth   int                   `json:"day_of_month,omitempty"`
	SpecificDays []time.Time           `json:"specific_days,omitempty"`
	Parity       string                `json:"parity,omitempty"`
	IsActive     bool                  `json:"is_active"`
	StartsAt     time.Time             `json:"starts_at"`
}

func newTaskDTO(task *taskdomain.Task) taskDTO {
	return taskDTO{
		ID:          task.ID,
		Title:       task.Title,
		Description: task.Description,
		Status:      task.Status,
		CreatedAt:   task.CreatedAt,
		UpdatedAt:   task.UpdatedAt,
	}
}
func newTemplateDTO(template *taskdomain.TaskTemplate) taskTemplateDTO {
	return taskTemplateDTO{
		ID:           template.ID,
		Title:        template.Title,
		Description:  template.Description,
		Type:         template.Type,
		Interval:     template.Interval,
		DayOfMonth:   template.DayOfMonth,
		SpecificDays: template.SpecificDays,
		Parity:       template.Parity,
		IsActive:     template.IsActive,
		StartsAt:     *template.StartsAt,
	}
}
