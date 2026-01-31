package service

import (
	"context"
	"fmt"
	"time"

	"task-manager-api/internal/models"
	"task-manager-api/internal/repository"

	"github.com/google/uuid"
)

type TaskService interface {
	CreateTask(ctx context.Context, userID uuid.UUID, req models.CreateTaskRequest) (*models.Task, error)
	GetTasks(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error)
	GetTask(ctx context.Context, id uuid.UUID) (*models.Task, error)
	UpdateTask(ctx context.Context, id uuid.UUID, req models.UpdateTaskRequest) (*models.Task, error)
	DeleteTask(ctx context.Context, id uuid.UUID) error
}

type taskService struct {
	repo repository.TaskRepository
}

func NewTaskService(repo repository.TaskRepository) TaskService {
	return &taskService{repo: repo}
}

func (s *taskService) CreateTask(ctx context.Context, userID uuid.UUID, req models.CreateTaskRequest) (*models.Task, error) {
	task := &models.Task{
		ID:          uuid.New(),
		UserID:      userID,
		Title:       req.Title,
		Description: req.Description,
		Status:      models.StatusPending,
		Priority:    req.Priority,
		DueDate:     req.DueDate,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.repo.Create(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *taskService) GetTasks(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	return s.repo.GetTasksWithConcurrency(ctx, userID, filter)
}

func (s *taskService) GetTask(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	return s.repo.FindByID(ctx, id)
}

func (s *taskService) UpdateTask(ctx context.Context, id uuid.UUID, req models.UpdateTaskRequest) (*models.Task, error) {
	task, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, fmt.Errorf("task not found")
	}

	// Update fields if provided
	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		task.Status = *req.Status
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}

	task.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (s *taskService) DeleteTask(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
