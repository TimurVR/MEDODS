package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	taskdomain "example.com/taskservice/internal/domain/task"
	taskusecase "example.com/taskservice/internal/usecase/task"
	"github.com/go-playground/assert/v2"
	"github.com/gorilla/mux"
	asserttest "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockService struct {
	mock.Mock
}

func (m *MockService) Create(ctx context.Context, in taskusecase.CreateInput) (*taskdomain.Task, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*taskdomain.Task), args.Error(1)
}

func (m *MockService) CreateTemplate(ctx context.Context, in taskusecase.CreateInput) (*taskdomain.TaskTemplate, error) {
	args := m.Called(ctx, in)
	return args.Get(0).(*taskdomain.TaskTemplate), args.Error(1)
}

func (m *MockService) GetByID(ctx context.Context, id int64) (*taskdomain.Task, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*taskdomain.Task), args.Error(1)
}

func (m *MockService) GetByIDTemplate(ctx context.Context, id int64) (*taskdomain.TaskTemplate, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*taskdomain.TaskTemplate), args.Error(1)
}

func (m *MockService) Update(ctx context.Context, id int64, in taskusecase.UpdateInput) (*taskdomain.Task, error) {
	args := m.Called(ctx, id, in)
	return args.Get(0).(*taskdomain.Task), args.Error(1)
}

func (m *MockService) UpdateTemplate(ctx context.Context, id int64, in taskusecase.UpdateInput) error {
	args := m.Called(ctx, id, in)
	return args.Error(0)
}

func (m *MockService) Delete(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockService) DeleteTemplate(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockService) List(ctx context.Context) ([]taskdomain.Task, error) {
	args := m.Called(ctx)
	return args.Get(0).([]taskdomain.Task), args.Error(1)
}

func (m *MockService) ListTemplate(ctx context.Context) ([]taskdomain.TaskTemplate, error) {
	args := m.Called(ctx)
	return args.Get(0).([]taskdomain.TaskTemplate), args.Error(1)
}

func TestTaskHandler(t *testing.T) {
	mockSvc := new(MockService)
	h := NewTaskHandler(mockSvc)

	t.Run("Create Recurring Task", func(t *testing.T) {
		input := taskMutationDTO{
			Title:      "Test Template",
			Recurrence: &taskdomain.RecurrenceRule{Type: "daily", Interval: 1},
		}
		mockSvc.On("CreateTemplate", mock.Anything, mock.Anything).Return(&taskdomain.TaskTemplate{ID: 1}, nil).Once()

		body, _ := json.Marshal(input)
		req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()

		h.Create(rec, req)
		assert.Equal(t, http.StatusCreated, rec.Code)
	})
	t.Run("Create Task - Database Error", func(t *testing.T) {
		mockSvc.On("Create", mock.Anything, mock.Anything).
			Return((*taskdomain.Task)(nil), fmt.Errorf("db connection lost")).Once()
		body, _ := json.Marshal(taskMutationDTO{Title: "Broken Task"})
		req := httptest.NewRequest(http.MethodPost, "/tasks", bytes.NewBuffer(body))
		rec := httptest.NewRecorder()
		h.Create(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var resp map[string]string
		json.Unmarshal(rec.Body.Bytes(), &resp)
		asserttest.Contains(t, resp["error"], "db connection lost")
	})
	t.Run("Get Template By ID", func(t *testing.T) {
		now := time.Now()
		mockSvc.On("GetByIDTemplate", mock.Anything, int64(1)).Return(&taskdomain.TaskTemplate{
			ID:        1,
			Title:     "Test",
			StartsAt:  &now,
			CreatedAt: now,
		}, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/tasks/templates/1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "1"})
		rec := httptest.NewRecorder()

		h.GetByIDTemplate(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
	t.Run("Get Template By ID - Database Error", func(t *testing.T) {
		mockSvc.On("GetByIDTemplate", mock.Anything, mock.Anything).
			Return((*taskdomain.TaskTemplate)(nil), fmt.Errorf("db connection lost")).Once()
		req := httptest.NewRequest(http.MethodGet, "/tasks/templates/1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "1"})
		rec := httptest.NewRecorder()
		h.GetByIDTemplate(rec, req)
		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		var resp map[string]string
		json.Unmarshal(rec.Body.Bytes(), &resp)
		asserttest.Contains(t, resp["error"], "db connection lost")
	})
	t.Run("Get Template By Bad ID", func(t *testing.T) {
		now := time.Now()
		mockSvc.On("GetByIDTemplate", mock.Anything, int64(1)).Return(&taskdomain.TaskTemplate{
			ID:        1,
			Title:     "Test",
			StartsAt:  &now,
			CreatedAt: now,
		}, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/tasks/templates/-1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "-1"})
		rec := httptest.NewRecorder()

		h.GetByIDTemplate(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		var resp map[string]string
		json.Unmarshal(rec.Body.Bytes(), &resp)
		asserttest.Contains(t, resp["error"], "invalid task id")
	})
	t.Run("Update Template", func(t *testing.T) {
		mockSvc.On("UpdateTemplate", mock.Anything, int64(1), mock.Anything).Return(nil).Once()

		body, _ := json.Marshal(taskMutationDTO{Title: "Updated"})
		req := httptest.NewRequest(http.MethodPut, "/tasks/templates/1", bytes.NewBuffer(body))
		req = mux.SetURLVars(req, map[string]string{"id": "1"})
		rec := httptest.NewRecorder()

		h.UpdateTemplate(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
	t.Run("Delete Template", func(t *testing.T) {
		mockSvc.On("DeleteTemplate", mock.Anything, int64(1)).Return(nil).Once()

		req := httptest.NewRequest(http.MethodDelete, "/tasks/templates/1", nil)
		req = mux.SetURLVars(req, map[string]string{"id": "1"})
		rec := httptest.NewRecorder()

		h.DeleteTemplate(rec, req)
		assert.Equal(t, http.StatusNoContent, rec.Code)
	})
	t.Run("List Templates", func(t *testing.T) {
		now := time.Now()
		mockSvc.On("ListTemplate", mock.Anything).Return([]taskdomain.TaskTemplate{
			{
				ID:        1,
				Title:     "Test",
				StartsAt:  &now,
				CreatedAt: now,
			},
		}, nil).Once()

		req := httptest.NewRequest(http.MethodGet, "/tasks/templates", nil)
		rec := httptest.NewRecorder()

		h.ListTemplates(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)
	})
}
