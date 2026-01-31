package models

import (
	"time"

	"github.com/google/uuid"
)

type TaskStatus string

const (
	StatusPending    TaskStatus = "pending"
	StatusInProgress TaskStatus = "in_progress"
	StatusCompleted  TaskStatus = "completed"
	StatusCancelled  TaskStatus = "cancelled"
)

type Task struct {
	ID          uuid.UUID  `json:"id"`
	UserID      uuid.UUID  `json:"user_id"`
	Title       string     `json:"title" binding:"required,min=1,max=255"`
	Description string     `json:"description,omitempty"`
	Status      TaskStatus `json:"status"`
	Priority    int        `json:"priority" binding:"min=1,max=5"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreateTaskRequest struct {
	Title       string     `json:"title" binding:"required,min=1,max=255"`
	Description string     `json:"description,omitempty"`
	Priority    int        `json:"priority" binding:"min=1,max=5"`
	DueDate     *time.Time `json:"due_date,omitempty"`
}

type UpdateTaskRequest struct {
	Title       *string     `json:"title,omitempty"`
	Description *string     `json:"description,omitempty"`
	Status      *TaskStatus `json:"status,omitempty"`
	Priority    *int        `json:"priority,omitempty" binding:"omitempty,min=1,max=5"`
	DueDate     *time.Time  `json:"due_date,omitempty"`
}

type TaskFilter struct {
	Status   *TaskStatus `form:"status"`
	Priority *int        `form:"priority"`
	FromDate *time.Time  `form:"from_date"`
	ToDate   *time.Time  `form:"to_date"`
	Limit    int         `form:"limit,default=10" binding:"min=1,max=100"`
	Offset   int         `form:"offset,default=0" binding:"min=0"`
}
