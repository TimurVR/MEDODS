package task

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	"github.com/jackc/pgx/v5"
)

type Service struct {
	repo Repository
	now  func() time.Time
}

func NewService(repo Repository) *Service {
	return &Service{
		repo: repo,
		now:  func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) Create(ctx context.Context, input CreateInput) (*taskdomain.Task, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
	}
	now := s.now()
	model.CreatedAt = now
	model.UpdatedAt = now

	created, err := s.repo.Create(ctx, model)
	if err != nil {
		return nil, err
	}

	return created, nil
}
func (s *Service) calculateDueDates(t *taskdomain.TaskTemplate) []time.Time {
	var dates []time.Time
	start := *t.StartsAt
	genStart := start
	horizon := time.Date(genStart.Year(), genStart.Month()+1, 0, 23, 59, 59, 0, genStart.Location())
	switch t.Type {
	case "daily":
		interval := t.Interval
		if interval <= 0 {
			interval = 1
		}
		for d := start; !d.After(horizon); d = d.AddDate(0, 0, interval) {
			if !d.Before(genStart) {
				dates = append(dates, d)
			}
		}
	case "monthly":
		for i := 0; i < 12; i++ {
			checkMonth := genStart.AddDate(0, i, 0)
			lastDay := time.Date(checkMonth.Year(), checkMonth.Month()+1, 0, 0, 0, 0, 0, start.Location()).Day()
			day := t.DayOfMonth
			if day > lastDay {
				day = lastDay
			}
			taskDate := time.Date(checkMonth.Year(), checkMonth.Month(), day, start.Hour(), start.Minute(), 0, 0, start.Location())
			if !taskDate.Before(genStart) && !taskDate.After(horizon) {
				dates = append(dates, taskDate)
			}
		}
	case "parity":
		for d := genStart; !d.After(horizon); d = d.AddDate(0, 0, 1) {
			isEven := (d.Day()%2 == 0)
			if (t.Parity == "even" && isEven) || (t.Parity == "odd" && !isEven) {
				dates = append(dates, d)
			}
		}
	case "specific_days":
		for _, d := range t.SpecificDays {
			if !d.Before(genStart) {
				dates = append(dates, d)
			}
		}
	}
	return dates
}

func (s *Service) CreateTemplate(ctx context.Context, input CreateInput) (*taskdomain.TaskTemplate, error) {
	normalized, err := validateCreateInput(input)
	if err != nil {
		return nil, err
	}
	taskTemplate := &taskdomain.TaskTemplate{
		Title:        normalized.Title,
		Description:  normalized.Description,
		Type:         input.Recurrence.Type,
		Interval:     input.Recurrence.Interval,
		DayOfMonth:   input.Recurrence.DayOfMonth,
		Parity:       input.Recurrence.Parity,
		SpecificDays: input.Recurrence.SpecificDays,
		IsActive:     true,
		StartsAt:     input.Recurrence.StartsAt,
		CreatedAt:    s.now(),
		UpdatedAt:    s.now(),
	}
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	templateID, err := s.repo.InsertTemplate(ctx, tx, taskTemplate)
	if err != nil {
		return nil, err
	}
	taskTemplate.ID = templateID
	dueDates := s.calculateDueDates(taskTemplate)
	var lastGenerated time.Time
	for _, dueDate := range dueDates {
		err = s.repo.InsertTask(ctx, tx, taskdomain.Task{
			ParentID:    &templateID,
			Title:       taskTemplate.Title,
			Description: taskTemplate.Description,
			Status:      taskdomain.StatusInProgress,
			DueDate:     dueDate,
			CreatedAt:   s.now(),
			UpdatedAt:   s.now(),
		})
		if err != nil {
			return nil, err
		}
		lastGenerated = dueDate
	}
	if !lastGenerated.IsZero() {
		err = s.repo.UpdateLastGenerated(ctx, tx, templateID, lastGenerated)
		if err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return taskTemplate, nil
}

func (s *Service) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByID(ctx, id)
}

func (s *Service) GetByIDTemplate(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.GetByIDTemplate(ctx, id)
}
func (s *Service) Update(ctx context.Context, id int64, input UpdateInput) (*taskdomain.Task, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	normalized, err := validateUpdateInput(input)
	if err != nil {
		return nil, err
	}

	model := &taskdomain.Task{
		ID:          id,
		Title:       normalized.Title,
		Description: normalized.Description,
		Status:      normalized.Status,
		UpdatedAt:   s.now(),
	}

	updated, err := s.repo.Update(ctx, model)
	if err != nil {
		return nil, err
	}

	return updated, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, id int64, input UpdateInput) error {
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	oldTemplate, err := s.repo.GetByIDTemplate(ctx, id) 
	if err != nil {
		return err
	}
	if oldTemplate == nil {
		return taskdomain.ErrNotFound
	}
	newTemplate := &taskdomain.TaskTemplate{
		ID:          id,
		Title:       input.Title,
		Description: input.Description,
		IsActive:    oldTemplate.IsActive,
	}

	if input.Recurrence != nil {
		newTemplate.Type = input.Recurrence.Type
		newTemplate.Interval = input.Recurrence.Interval
		newTemplate.DayOfMonth = input.Recurrence.DayOfMonth
		newTemplate.Parity = input.Recurrence.Parity
		newTemplate.SpecificDays = input.Recurrence.SpecificDays
		newTemplate.StartsAt = input.Recurrence.StartsAt
	} else {
		newTemplate.Type = oldTemplate.Type
		newTemplate.Interval = oldTemplate.Interval
		newTemplate.DayOfMonth = oldTemplate.DayOfMonth
		newTemplate.Parity = oldTemplate.Parity
		newTemplate.SpecificDays = oldTemplate.SpecificDays
		newTemplate.StartsAt = oldTemplate.StartsAt
	}
	if err := s.repo.UpdateTemplate(ctx, tx, newTemplate); err != nil {
		return err
	}
	isRecurrenceChanged := oldTemplate.Type != newTemplate.Type ||
		oldTemplate.Interval != newTemplate.Interval ||
		oldTemplate.DayOfMonth != newTemplate.DayOfMonth ||
		oldTemplate.Parity != newTemplate.Parity ||
		!reflect.DeepEqual(oldTemplate.SpecificDays, newTemplate.SpecificDays)
	if isRecurrenceChanged {
		if err := s.repo.DeleteFutureTasks(ctx, tx, id); err != nil {
			return err
		}
		dueDates := s.calculateDueDates(newTemplate)
		var lastGenerated time.Time
		for _, dueDate := range dueDates {
			err = s.repo.InsertTask(ctx, tx, taskdomain.Task{
				ParentID:    &id,
				Title:       newTemplate.Title,
				Description: newTemplate.Description,
				Status:      taskdomain.StatusInProgress,
				DueDate:     dueDate,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			})
			if err != nil {
				return err
			}
			lastGenerated = dueDate
		}

		if !lastGenerated.IsZero() {
			_ = s.repo.UpdateLastGenerated(ctx, tx, id, lastGenerated)
		}
	} else {
		_ = s.repo.UpdateFutureTasksMetadata(ctx, tx, id, newTemplate.Title, newTemplate.Description)
	}

	return tx.Commit(ctx)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.Delete(ctx, id)
}

func (s *Service) DeleteTemplate(ctx context.Context, id int64) error {
	if id <= 0 {
		return fmt.Errorf("%w: id must be positive", ErrInvalidInput)
	}

	return s.repo.DeleteTemplate(ctx, id)
}
func (s *Service) List(ctx context.Context) ([]taskdomain.Task, error) {
	return s.repo.List(ctx)
}
func (s *Service) ListTemplate(ctx context.Context) ([]taskdomain.TaskTemplate, error) {
	return s.repo.ListTemplate(ctx)
}

func validateCreateInput(input CreateInput) (CreateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return CreateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if input.Status == "" {
		input.Status = taskdomain.StatusNew
	}

	if !input.Status.Valid() {
		return CreateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	return input, nil
}

func validateUpdateInput(input UpdateInput) (UpdateInput, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)

	if input.Title == "" {
		return UpdateInput{}, fmt.Errorf("%w: title is required", ErrInvalidInput)
	}

	if !input.Status.Valid() {
		return UpdateInput{}, fmt.Errorf("%w: invalid status", ErrInvalidInput)
	}

	return input, nil
}

func (s *Service) StartWorker(ctx context.Context) {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.generateMissingTasks(ctx)
		}
	}
}

func (s *Service) generateMissingTasks(ctx context.Context) {
	templates, err := s.repo.ListActiveTemplates(ctx)
	if err != nil {
		log.Printf("Worker error: failed to list templates: %v", err)
		return
	}

	for _, t := range templates {
		lastGen := t.CreatedAt
		if t.LastGeneratedAt != nil {
			lastGen = *t.LastGeneratedAt
		}
		if lastGen.Before(time.Now().AddDate(0, 0, 25)) {
			newStart := lastGen.AddDate(0, 0, 1)
			t.StartsAt = &newStart
			dueDates := s.calculateDueDates(&t)
			if len(dueDates) == 0 {
				continue
			}
			s.runInTransaction(ctx, func(tx pgx.Tx) error {
				var maxDate time.Time
				for _, d := range dueDates {
					err := s.repo.InsertTask(ctx, tx, taskdomain.Task{
						ParentID:    &t.ID,
						Title:       t.Title,
						Description: t.Description,
						Status:      taskdomain.StatusInProgress,
						DueDate:     d,
					})
					if err != nil {
						return err
					}
					maxDate = d
				}
				return s.repo.UpdateLastGenerated(ctx, tx, t.ID, maxDate)
			})
		}
	}
}

func (s *Service) runInTransaction(ctx context.Context, fn func(pgx.Tx) error) {
	tx, err := s.repo.Begin(ctx)
	if err != nil {
		return
	}
	defer tx.Rollback(ctx)

	if err := fn(tx); err == nil {
		tx.Commit(ctx)
	}
}
