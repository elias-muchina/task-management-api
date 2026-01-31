package handlers

import (
	"context"
	"fmt"
	"net/http"

	"task-manager-api/internal/models"
	"task-manager-api/internal/service"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// TaskHandler handles HTTP requests for tasks
type TaskHandler struct {
	taskService service.TaskService
	taskWorker  *service.TaskWorker
}

// NewTaskHandler creates a new TaskHandler
func NewTaskHandler(taskService service.TaskService, taskWorker *service.TaskWorker) *TaskHandler {
	return &TaskHandler{
		taskService: taskService,
		taskWorker:  taskWorker,
	}
}

// @Summary Get all tasks
// @Description Get tasks with filtering and pagination
// @Tags tasks
// @Accept json
// @Produce json
// @Param status query string false "Task status"
// @Param priority query int false "Priority level"
// @Param limit query int false "Limit" default(10)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /tasks [get]
func (h *TaskHandler) GetTasks(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var filter models.TaskFilter
	if err := c.ShouldBindQuery(&filter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Use concurrent fetching pattern
	tasks, err := h.taskService.GetTasks(c.Request.Context(), userID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tasks": tasks,
		"meta": gin.H{
			"total":  len(tasks),
			"limit":  filter.Limit,
			"offset": filter.Offset,
		},
	})
}

// @Summary Create a new task
// @Description Create a task with the provided details
// @Tags tasks
// @Accept json
// @Produce json
// @Param request body models.CreateTaskRequest true "Task data"
// @Success 201 {object} models.Task
// @Router /tasks [post]
func (h *TaskHandler) CreateTask(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var req models.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	task, err := h.taskService.CreateTask(c.Request.Context(), userID, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, task)
}

// @Summary Get a single task
// @Description Get a task by ID
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 200 {object} models.Task
// @Router /tasks/{id} [get]
func (h *TaskHandler) GetTask(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	task, err := h.taskService.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	// Ensure user can only access their own tasks
	if task.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, task)
}

// @Summary Update a task
// @Description Update an existing task
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Param request body models.UpdateTaskRequest true "Updated task data"
// @Success 200 {object} models.Task
// @Router /tasks/{id} [put]
func (h *TaskHandler) UpdateTask(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// First, get the task to check ownership
	task, err := h.taskService.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	if task.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	var req models.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	updatedTask, err := h.taskService.UpdateTask(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedTask)
}

// @Summary Delete a task
// @Description Delete a task by ID
// @Tags tasks
// @Accept json
// @Produce json
// @Param id path string true "Task ID"
// @Success 204 "No Content"
// @Router /tasks/{id} [delete]
func (h *TaskHandler) DeleteTask(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid task ID"})
		return
	}

	// First, get the task to check ownership
	task, err := h.taskService.GetTask(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	if task.UserID != userID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.taskService.DeleteTask(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Batch process tasks
// @Description Process multiple tasks asynchronously
// @Tags tasks
// @Accept json
// @Produce json
// @Param request body BatchProcessRequest true "Task IDs to process"
// @Success 202 "Accepted"
// @Router /tasks/batch [post]
func (h *TaskHandler) BatchProcessTasks(c *gin.Context) {
	userID := c.MustGet("userID").(uuid.UUID)

	var req BatchProcessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate all tasks belong to the user
	for _, taskID := range req.TaskIDs {
		task, err := h.taskService.GetTask(c.Request.Context(), taskID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error validating task %s: %v", taskID, err)})
			return
		}
		if task == nil || task.UserID != userID {
			c.JSON(http.StatusForbidden, gin.H{"error": fmt.Sprintf("Access denied to task %s", taskID)})
			return
		}
	}

	// Start batch processing in background
	go func() {
		ctx := context.Background()
		if err := h.taskWorker.BatchProcessTasks(ctx, req.TaskIDs, req.BatchSize, req.Status); err != nil {
			fmt.Printf("Batch processing failed: %v\n", err)
		}
	}()

	c.Status(http.StatusAccepted)
}

// BatchProcessRequest represents a request to process multiple tasks
type BatchProcessRequest struct {
	TaskIDs   []uuid.UUID       `json:"task_ids" binding:"required,min=1"`
	BatchSize int               `json:"batch_size" binding:"min=1,max=100"`
	Status    models.TaskStatus `json:"status" binding:"required,oneof=pending in_progress completed cancelled"`
}
