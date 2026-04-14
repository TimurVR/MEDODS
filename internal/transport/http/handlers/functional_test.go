package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	"example.com/taskservice/internal/repository/postgres"
	taskusecase "example.com/taskservice/internal/usecase/task"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
	"github.com/stretchr/testify/assert"
)

var (
	testPool *pgxpool.Pool
	testRepo *postgres.Repository
	ctx      context.Context
)

func TestMain(m *testing.M) {
	var err error
	ctx = context.Background()
	dockerPool, err := dockertest.NewPool("")
	if err != nil {
		fmt.Printf("Could not construct pool: %s\n", err)
		os.Exit(1)
	}
	err = dockerPool.Client.Ping()
	if err != nil {
		fmt.Printf("Could not connect to Docker: %s\n", err)
		os.Exit(1)
	}
	resource, err := dockerPool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "16-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=testpass",
			"POSTGRES_USER=testuser",
			"POSTGRES_DB=testdb",
		},
	}, func(config *docker.HostConfig) {
		config.AutoRemove = true
		config.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		fmt.Printf("Could not start resource: %s\n", err)
		os.Exit(1)
	}
	hostAndPort := resource.GetHostPort("5432/tcp")
	databaseUrl := fmt.Sprintf("postgres://testuser:testpass@%s/testdb?sslmode=disable", hostAndPort)
	resource.Expire(120)
	if err := dockerPool.Retry(func() error {
		var err error
		testPool, err = pgxpool.New(ctx, databaseUrl)
		if err != nil {
			return err
		}
		return testPool.Ping(ctx)
	}); err != nil {
		fmt.Printf("Could not connect to database: %s\n", err)
		dockerPool.Purge(resource)
		os.Exit(1)
	}
	if err := createTables(); err != nil {
		fmt.Printf("Could not create tables: %s\n", err)
		testPool.Close()
		dockerPool.Purge(resource)
		os.Exit(1)
	}
	testRepo = postgres.New(testPool)
	code := m.Run()
	testPool.Close()
	if err := dockerPool.Purge(resource); err != nil {
		fmt.Printf("Could not purge resource: %s\n", err)
	}

	os.Exit(code)
}

func createTables() error {
	_, err := testPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS task_templates (
			id BIGSERIAL PRIMARY KEY,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL,
			interval INT DEFAULT 0,
			day_of_month INT,
			parity TEXT,
			specific_days TIMESTAMPTZ[], 
			is_active BOOLEAN DEFAULT true,
			starts_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			last_generated_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		)
	`)
	if err != nil {
		return err
	}
	_, err = testPool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id BIGSERIAL PRIMARY KEY,
			parent_id BIGINT REFERENCES task_templates(id) ON DELETE CASCADE, 
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			due_date TIMESTAMPTZ NOT NULL DEFAULT NOW(), 
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`)
	return err
}

func cleanupTables() {
	testPool.Exec(ctx, "TRUNCATE task_templates, tasks CASCADE")
}
func TestTaskSystem_Functional(t *testing.T) {
	repo := postgres.New(testPool)
	service := taskusecase.NewService(repo)
	handler := NewTaskHandler(service)
	router := mux.NewRouter()
	router.HandleFunc("/tasks", handler.Create).Methods(http.MethodPost)
	router.HandleFunc("/tasks", handler.List).Methods(http.MethodGet)
	router.HandleFunc("/tasks/templates/{id}", handler.DeleteTemplate).Methods(http.MethodDelete)

	t.Run("Full Cycle: Create Recurring -> List Tasks -> Delete Template", func(t *testing.T) {
		startsAt := time.Now()
		payload := taskMutationDTO{
			Title:       "Functional Test Task",
			Description: "Checking if tasks are generated",
			Status:      "new",
			Recurrence: &taskdomain.RecurrenceRule{
				Type:     "daily",
				Interval: 1,
				StartsAt: &startsAt,
			},
		}

		body, _ := json.Marshal(payload)
		req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
		var createdTemplate taskdomain.TaskTemplate
		json.Unmarshal(rec.Body.Bytes(), &createdTemplate)
		templateID := createdTemplate.ID
		req = httptest.NewRequest(http.MethodGet, "/tasks", nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		var tasks []taskDTO
		json.Unmarshal(rec.Body.Bytes(), &tasks)
		assert.Greater(t, len(tasks), 0)
		found := false
		for _, task := range tasks {
			if task.Title == "Functional Test Task" {
				found = true
				break
			}
		}
		assert.True(t, found, "Задачи не были найдены в общем списке после генерации")
		req = httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/tasks/templates/%d", templateID), nil)
		req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprintf("%d", templateID)})
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)

		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
}
func TestTaskSystem_Update(t *testing.T) {
	repo := postgres.New(testPool)
	service := taskusecase.NewService(repo)
	handler := NewTaskHandler(service)

	router := mux.NewRouter()
	router.HandleFunc("/tasks", handler.Create).Methods(http.MethodPost)
	router.HandleFunc("/tasks", handler.List).Methods(http.MethodGet)
	router.HandleFunc("/tasks/templates/{id}", handler.UpdateTemplate).Methods(http.MethodPut)
	router.HandleFunc("/tasks/templates/{id}", handler.GetByIDTemplate).Methods(http.MethodGet)
	cleanupTables()

	t.Run("Switch from Daily to Monthly and verify Sync", func(t *testing.T) {
		startsAt := time.Now()
		createPayload := taskMutationDTO{
			Title: "Daily Operation",
			Recurrence: &taskdomain.RecurrenceRule{
				Type:     "daily",
				Interval: 1,
				StartsAt: &startsAt,
			},
		}
		body, _ := json.Marshal(createPayload)
		req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)

		var template taskdomain.TaskTemplate
		json.Unmarshal(rec.Body.Bytes(), &template)
		req = httptest.NewRequest(http.MethodGet, "/tasks", nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		var tasksBefore []taskDTO
		json.Unmarshal(rec.Body.Bytes(), &tasksBefore)
		assert.Greater(t, len(tasksBefore), 1, "Должно быть создано много задач для daily")
		updatePayload := taskMutationDTO{
			Title: "Monthly Checkup",
			Recurrence: &taskdomain.RecurrenceRule{
				Type:       "monthly",
				DayOfMonth: startsAt.Day(),
				StartsAt:   &startsAt,
			},
		}
		updBody, _ := json.Marshal(updatePayload)
		updURL := fmt.Sprintf("/tasks/templates/%d", template.ID)
		req = httptest.NewRequest(http.MethodPut, updURL, bytes.NewBuffer(updBody))
		req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprintf("%d", template.ID)})
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
		req = httptest.NewRequest(http.MethodGet, updURL, nil)
		req = mux.SetURLVars(req, map[string]string{"id": fmt.Sprintf("%d", template.ID)})
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		
		assert.Equal(t, http.StatusOK, rec.Code)	
		var updatedTemplate taskTemplateDTO
		json.Unmarshal(rec.Body.Bytes(), &updatedTemplate)
		
		assert.Equal(t, "monthly", string(updatedTemplate.Type))
		assert.Equal(t, "Monthly Checkup", updatedTemplate.Title)
		assert.Equal(t, startsAt.Day(), updatedTemplate.DayOfMonth)
		req = httptest.NewRequest(http.MethodGet, "/tasks", nil)
		rec = httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		
		var tasksAfter []taskDTO
		json.Unmarshal(rec.Body.Bytes(), &tasksAfter)
		assert.LessOrEqual(t, len(tasksAfter), 2, "Лишние ежедневные задачи не были удалены!")
		
		for _, task := range tasksAfter {
			assert.Equal(t, "Monthly Checkup", task.Title)
			assert.NotEqual(t, "Daily Operation", task.Title, "Старый заголовок остался в базе")
		}
	})
}

