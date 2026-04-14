package postgres

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	taskdomain "example.com/taskservice/internal/domain/task"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Create(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		INSERT INTO tasks (title, description, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, title, description, status, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.CreatedAt, task.UpdatedAt)
	created, err := scanTask(row)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (r *Repository) InsertTemplate(ctx context.Context, tx pgx.Tx, template *taskdomain.TaskTemplate) (int64, error) {
	query := `
		INSERT INTO task_templates (title, description, type, interval, day_of_month, specific_days, parity, starts_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	startsAt := time.Now()
	if template.StartsAt != nil {
		startsAt = *template.StartsAt
	}
	var parityToInsert interface{}
	if template.Type == "parity" && template.Parity != "" {
		parityToInsert = template.Parity
	}

	var intervalToInsert interface{}
	if template.Type == "daily" {
		intervalToInsert = template.Interval
	}

	var dayOfMonthToInsert interface{}
	if template.Type == "monthly" {
		dayOfMonthToInsert = template.DayOfMonth
	}

	var id int64
	err := tx.QueryRow(ctx, query,
		template.Title,
		template.Description,
		template.Type,
		intervalToInsert,
		dayOfMonthToInsert,
		template.SpecificDays,
		parityToInsert,
		startsAt,
		template.CreatedAt,
	).Scan(&id)

	if err != nil {
		return 0, err
	}

	return id, nil
}

func (r *Repository) InsertTask(ctx context.Context, tx pgx.Tx, task taskdomain.Task) error {
	query := `
		INSERT INTO tasks (parent_id, title, description, status, due_date, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := tx.Exec(ctx, query,
		task.ParentID,
		task.Title,
		task.Description,
		task.Status,
		task.DueDate,
		task.CreatedAt,
		task.UpdatedAt,
	)
	return err
}
func (r *Repository) UpdateLastGenerated(ctx context.Context, tx pgx.Tx, templateID int64, lastGenerated time.Time) error {
	query := `UPDATE task_templates SET last_generated_at = $1, updated_at = NOW() WHERE id = $2`
	_, err := tx.Exec(ctx, query, lastGenerated, templateID)
	return err
}
func (r *Repository) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) GetByIDTemplate(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error) {
	const query = `
	SELECT id,
   		title,
    	description,
    	type,
    	interval,
   		day_of_month,
    	parity,
    	specific_days, 
    	is_active,
		starts_at,
		last_generated_at,
    	created_at,
		updated_at
	FROM task_templates
	WHERE id = $1
	`
	row := r.pool.QueryRow(ctx, query, id)
	found, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return found, nil
}

func (r *Repository) Update(ctx context.Context, task *taskdomain.Task) (*taskdomain.Task, error) {
	const query = `
		UPDATE tasks
		SET title = $1,
			description = $2,
			status = $3,
			updated_at = $4
		WHERE id = $5
		RETURNING id, title, description, status, created_at, updated_at
	`

	row := r.pool.QueryRow(ctx, query, task.Title, task.Description, task.Status, task.UpdatedAt, task.ID)
	updated, err := scanTask(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, taskdomain.ErrNotFound
		}

		return nil, err
	}

	return updated, nil
}

func (r *Repository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM tasks WHERE id = $1`

	result, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}

	if result.RowsAffected() == 0 {
		return taskdomain.ErrNotFound
	}

	return nil
}
func (r *Repository) DeleteTemplate(ctx context.Context, id int64) error {
	const query = `UPDATE task_templates SET is_active = false, updated_at = NOW() WHERE id = $1`
	_, err := r.pool.Exec(ctx, query, id)
	if err != nil {
		return err
	}
	const queryTasks = `DELETE FROM tasks WHERE parent_id = $1 AND status = 'pending' AND due_date > NOW()`
	_, err = r.pool.Exec(ctx, queryTasks, id)

	return err
}

func (r *Repository) List(ctx context.Context) ([]taskdomain.Task, error) {
	const query = `
		SELECT id, title, description, status, created_at, updated_at
		FROM tasks
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := make([]taskdomain.Task, 0)
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}

		tasks = append(tasks, *task)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tasks, nil
}
func (r *Repository) ListTemplate(ctx context.Context) ([]taskdomain.TaskTemplate, error) {
	const query = `
		SELECT id,
    title,
    description,
    type,
    interval,
    day_of_month,
    parity,
    specific_days, 
    is_active,
	starts_at,
	last_generated_at,
    created_at,updated_at
		FROM task_templates
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]taskdomain.TaskTemplate, 0)
	for rows.Next() {
		template, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}

		templates = append(templates, *template)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return templates, nil
}

func (r *Repository) ListActiveTemplates(ctx context.Context) ([]taskdomain.TaskTemplate, error) {
	const query = `
		SELECT id,
    title,
    description,
    type,
    interval,
    day_of_month,
    parity,
    specific_days, 
    is_active,
	starts_at,
	last_generated_at,
    created_at,updated_at
		FROM task_templates
		WHERE is_active = true
		ORDER BY id DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	templates := make([]taskdomain.TaskTemplate, 0)
	for rows.Next() {
		template, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}

		templates = append(templates, *template)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return templates, nil
}
func (r *Repository) Begin(ctx context.Context) (pgx.Tx, error) {
	return r.pool.Begin(ctx)
}

type taskScanner interface {
	Scan(dest ...any) error
}
type templateScanner interface {
	Scan(dest ...any) error
}

func scanTask(scanner taskScanner) (*taskdomain.Task, error) {
	var (
		task   taskdomain.Task
		status string
	)

	if err := scanner.Scan(
		&task.ID,
		&task.Title,
		&task.Description,
		&status,
		&task.CreatedAt,
		&task.UpdatedAt,
	); err != nil {
		return nil, err
	}

	task.Status = taskdomain.Status(status)

	return &task, nil
}

func scanTemplate(scanner templateScanner) (*taskdomain.TaskTemplate, error) {

	var (
		template      taskdomain.TaskTemplate
		interval      sql.NullInt64
		dayOfMonth    sql.NullInt64
		parity        sql.NullString
		lastGenerated sql.NullTime
	)

	if err := scanner.Scan(
		&template.ID,
		&template.Title,
		&template.Description,
		&template.Type,
		&interval,
		&dayOfMonth,
		&parity,
		&template.SpecificDays,
		&template.IsActive,
		&template.StartsAt,
		&lastGenerated,
		&template.CreatedAt,
		&template.UpdatedAt,
	); err != nil {
		return nil, err
	}

	if interval.Valid {
		template.Interval = int(interval.Int64)
	}
	if dayOfMonth.Valid {
		template.DayOfMonth = int(dayOfMonth.Int64)
	}
	if parity.Valid {
		template.Parity = parity.String
	}
	if lastGenerated.Valid {
		template.LastGeneratedAt = &lastGenerated.Time
	}

	return &template, nil
}

func (r *Repository) DeleteFutureTasks(ctx context.Context, tx pgx.Tx, templateID int64) error {
	const query = `
		DELETE FROM tasks 
		WHERE parent_id = $1 
	`
	_, err := tx.Exec(ctx, query, templateID)
	return err
}

func (r *Repository) UpdateFutureTasksMetadata(ctx context.Context, tx pgx.Tx, templateID int64, title, description string) error {
	const query = `
		UPDATE tasks 
		SET title = $1, description = $2, updated_at = NOW()
		WHERE parent_id = $3 
		  AND status = 'pending' OR status = 'new'
	`
	_, err := tx.Exec(ctx, query, title, description, templateID)
	return err
}

func (r *Repository) UpdateTemplate(ctx context.Context, tx pgx.Tx, template *taskdomain.TaskTemplate) error {
	const query = `
		UPDATE task_templates 
		SET title = $1, description = $2, type = $3, interval = $4, 
		    day_of_month = $5, parity = $6, specific_days = $7, 
		    starts_at = $8, updated_at = NOW()
		WHERE id = $9
	`
	var parity interface{}
	if template.Type == "parity" {
		parity = template.Parity
	}
	var interval interface{}
	if template.Type == "daily" {
		interval = template.Interval
	}
	var dayOfMonth interface{}
	if template.Type == "monthly" {
		dayOfMonth = template.DayOfMonth
	}
	_, err := tx.Exec(ctx, query,
		template.Title, template.Description, template.Type, interval,
		dayOfMonth, parity, template.SpecificDays, template.StartsAt, template.ID,
	)
	return err
}
