package task

import (
	"context"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	"github.com/jackc/pgx/v5"
)

type Repository interface {
	Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error)
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	Begin(ctx context.Context) (pgx.Tx, error)
	InsertTask(ctx context.Context, tx pgx.Tx, task taskdomain.Task) error
	InsertTemplate(ctx context.Context, tx pgx.Tx, template *taskdomain.TaskTemplate) (int64, error)
	ListTemplate(ctx context.Context) ([]taskdomain.TaskTemplate, error)
	ListActiveTemplates(ctx context.Context) ([]taskdomain.TaskTemplate, error)
	UpdateLastGenerated(ctx context.Context, tx pgx.Tx, templateID int64, lastGenerated time.Time) error
	GetByIDTemplate(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error)
	DeleteTemplate(ctx context.Context, id int64) error
	UpdateTemplate(ctx context.Context, tx pgx.Tx, template *taskdomain.TaskTemplate) error
	UpdateFutureTasksMetadata(ctx context.Context, tx pgx.Tx, templateID int64, title, description string) error
	DeleteFutureTasks(ctx context.Context, tx pgx.Tx, templateID int64) error
}

type Usecase interface {
	Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error)
	CreateTemplate(ctx context.Context, input CreateInput) (*taskdomain.TaskTemplate, error)
	GetByID(ctx context.Context, id int64) (*taskdomain.Task, error)
	Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error)
	UpdateTemplate(ctx context.Context, id int64, input UpdateInput) error
	Delete(ctx context.Context, id int64) error
	List(ctx context.Context) ([]taskdomain.Task, error)
	ListTemplate(ctx context.Context) ([]taskdomain.TaskTemplate, error)
	GetByIDTemplate(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error)
	DeleteTemplate(ctx context.Context, id int64) error
}

type CreateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *taskdomain.RecurrenceRule
}

type UpdateInput struct {
	Title       string
	Description string
	Status      taskdomain.Status
	Recurrence  *taskdomain.RecurrenceRule
}
