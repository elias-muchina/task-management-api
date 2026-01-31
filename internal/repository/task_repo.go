package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"task-manager-api/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/redis/go-redis/v9"
)

type TaskRepository interface {
	Create(ctx context.Context, task *models.Task) error
	FindByID(ctx context.Context, id uuid.UUID) (*models.Task, error)
	FindByUserID(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error)
	Update(ctx context.Context, task *models.Task) error
	Delete(ctx context.Context, id uuid.UUID) error
	GetTasksWithConcurrency(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error)
}

type taskRepository struct {
	db    *pgx.Conn
	cache *redis.Client
	mu    sync.RWMutex
}

func NewTaskRepository(db *pgx.Conn, cache *redis.Client) TaskRepository {
	return &taskRepository{
		db:    db,
		cache: cache, // This can be nil
	}
}

// Helper method to generate cache key
func (r *taskRepository) getCacheKey(userID uuid.UUID, filter models.TaskFilter) string {
	key := fmt.Sprintf("tasks:%s", userID)

	if filter.Status != nil {
		key += fmt.Sprintf(":status:%s", *filter.Status)
	}
	if filter.Priority != nil {
		key += fmt.Sprintf(":priority:%d", *filter.Priority)
	}
	key += fmt.Sprintf(":limit:%d:offset:%d", filter.Limit, filter.Offset)

	return key
}

// Get tasks from Redis cache (safe with nil cache)
func (r *taskRepository) getTasksFromCache(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	// If Redis is not available, return nil (cache miss)
	if r.cache == nil {
		return nil, nil
	}

	key := r.getCacheKey(userID, filter)

	val, err := r.cache.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss, not an error
		}
		return nil, fmt.Errorf("failed to get from cache: %w", err)
	}

	var tasks []models.Task
	if err := json.Unmarshal([]byte(val), &tasks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cached tasks: %w", err)
	}

	return tasks, nil
}

// Get tasks from PostgreSQL database
func (r *taskRepository) getTasksFromDB(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	query := `
		SELECT id, user_id, title, description, status, priority, due_date, completed_at, created_at, updated_at
		FROM tasks
		WHERE user_id = $1
	`

	args := []interface{}{userID}
	argIndex := 2

	// Apply filters
	if filter.Status != nil {
		query += fmt.Sprintf(" AND status = $%d", argIndex)
		args = append(args, *filter.Status)
		argIndex++
	}

	if filter.Priority != nil {
		query += fmt.Sprintf(" AND priority = $%d", argIndex)
		args = append(args, *filter.Priority)
		argIndex++
	}

	if filter.FromDate != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, *filter.FromDate)
		argIndex++
	}

	if filter.ToDate != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, *filter.ToDate)
		argIndex++
	}

	// Ordering and pagination
	query += " ORDER BY created_at DESC"
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, filter.Limit, filter.Offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tasks: %w", err)
	}
	defer rows.Close()

	var tasks []models.Task
	for rows.Next() {
		var task models.Task
		err := rows.Scan(
			&task.ID, &task.UserID, &task.Title, &task.Description,
			&task.Status, &task.Priority, &task.DueDate, &task.CompletedAt,
			&task.CreatedAt, &task.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan task: %w", err)
		}
		tasks = append(tasks, task)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return tasks, nil
}

// Cache tasks in Redis with expiration (safe with nil cache)
func (r *taskRepository) cacheTasks(ctx context.Context, userID uuid.UUID, filter models.TaskFilter, tasks []models.Task) error {
	// If Redis is not available, skip caching
	if r.cache == nil {
		return nil
	}

	key := r.getCacheKey(userID, filter)

	data, err := json.Marshal(tasks)
	if err != nil {
		return fmt.Errorf("failed to marshal tasks for caching: %w", err)
	}

	// Cache for 5 minutes
	err = r.cache.Set(ctx, key, data, 5*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to cache tasks: %w", err)
	}

	return nil
}

// GetTasksWithConcurrency uses goroutine pattern (safe with nil cache)
func (r *taskRepository) GetTasksWithConcurrency(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	// If Redis is not available, just use database directly
	if r.cache == nil {
		return r.getTasksFromDB(ctx, userID, filter)
	}

	// Create channels for concurrent processing
	tasksChan := make(chan []models.Task)
	errChan := make(chan error, 2)

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Try to get from cache
	go func() {
		defer wg.Done()
		r.mu.RLock()
		defer r.mu.RUnlock()

		cachedTasks, err := r.getTasksFromCache(ctx, userID, filter)
		if err == nil && cachedTasks != nil {
			tasksChan <- cachedTasks
			return
		}
		errChan <- err
	}()

	// Goroutine 2: Get from database
	go func() {
		defer wg.Done()
		dbTasks, err := r.getTasksFromDB(ctx, userID, filter)
		if err != nil {
			errChan <- err
			return
		}

		// Cache the results
		go r.cacheTasks(ctx, userID, filter, dbTasks)

		tasksChan <- dbTasks
	}()

	// Wait for results
	go func() {
		wg.Wait()
		close(tasksChan)
		close(errChan)
	}()

	// Return first successful result
	select {
	case tasks := <-tasksChan:
		return tasks, nil
	case err := <-errChan:
		if len(errChan) == 2 { // Both goroutines failed
			return nil, fmt.Errorf("both cache and DB failed: %v", err)
		}
		// Try to get from other channel
		select {
		case tasks := <-tasksChan:
			return tasks, nil
		case err2 := <-errChan:
			return nil, fmt.Errorf("failed to get tasks: %v, %v", err, err2)
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// CRUD methods

func (r *taskRepository) Create(ctx context.Context, task *models.Task) error {
	query := `
		INSERT INTO tasks (id, user_id, title, description, status, priority, due_date)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		task.ID, task.UserID, task.Title, task.Description,
		task.Status, task.Priority, task.DueDate,
	).Scan(&task.CreatedAt, &task.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create task: %w", err)
	}

	// Invalidate cache for this user
	go r.invalidateUserCache(ctx, task.UserID)

	return nil
}

func (r *taskRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	query := `
		SELECT id, user_id, title, description, status, priority, due_date, completed_at, created_at, updated_at
		FROM tasks
		WHERE id = $1
	`

	var task models.Task
	err := r.db.QueryRow(ctx, query, id).Scan(
		&task.ID, &task.UserID, &task.Title, &task.Description,
		&task.Status, &task.Priority, &task.DueDate, &task.CompletedAt,
		&task.CreatedAt, &task.UpdatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find task: %w", err)
	}

	return &task, nil
}

func (r *taskRepository) FindByUserID(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	// Use the concurrent method by default
	return r.GetTasksWithConcurrency(ctx, userID, filter)
}

func (r *taskRepository) Update(ctx context.Context, task *models.Task) error {
	query := `
		UPDATE tasks 
		SET title = $2, description = $3, status = $4, priority = $5, 
		    due_date = $6, completed_at = $7, updated_at = CURRENT_TIMESTAMP
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.db.QueryRow(
		ctx,
		query,
		task.ID, task.Title, task.Description, task.Status,
		task.Priority, task.DueDate, task.CompletedAt,
	).Scan(&task.UpdatedAt)

	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("task not found with id: %s", task.ID)
		}
		return fmt.Errorf("failed to update task: %w", err)
	}

	// Invalidate cache for this user
	go r.invalidateUserCache(ctx, task.UserID)

	return nil
}

func (r *taskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	// First get the task to know which user's cache to invalidate
	task, err := r.FindByID(ctx, id)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task not found with id: %s", id)
	}

	query := `DELETE FROM tasks WHERE id = $1`

	result, err := r.db.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete task: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("task not found with id: %s", id)
	}

	// Invalidate cache for this user
	go r.invalidateUserCache(ctx, task.UserID)

	return nil
}

// Helper to invalidate all cache entries for a user (safe with nil cache)
func (r *taskRepository) invalidateUserCache(ctx context.Context, userID uuid.UUID) {
	// If Redis is not available, skip invalidation
	if r.cache == nil {
		return
	}

	pattern := fmt.Sprintf("tasks:%s*", userID)

	// Use SCAN to find all matching keys
	iter := r.cache.Scan(ctx, 0, pattern, 0).Iterator()
	for iter.Next(ctx) {
		r.cache.Del(ctx, iter.Val())
	}
}
