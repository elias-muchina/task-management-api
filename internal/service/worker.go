package service

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"task-manager-api/internal/models"
	"task-manager-api/internal/repository"

	"github.com/google/uuid"
)

type TaskWorker struct {
	taskChan   chan models.Task
	workerPool chan struct{}
	wg         sync.WaitGroup
	repo       repository.TaskRepository
}

type TaskUpdate struct {
	Task      models.Task
	NewStatus models.TaskStatus
}

func NewTaskWorker(maxWorkers int, repo repository.TaskRepository) *TaskWorker {
	return &TaskWorker{
		taskChan:   make(chan models.Task, 100),
		workerPool: make(chan struct{}, maxWorkers),
		repo:       repo,
	}
}

// ProcessTaskAsync demonstrates goroutine pool pattern
func (w *TaskWorker) ProcessTaskAsync(ctx context.Context, task models.Task, newStatus models.TaskStatus) {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		w.workerPool <- struct{}{}
		defer func() { <-w.workerPool }()

		processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := w.processTask(processCtx, task, newStatus); err != nil {
			log.Printf("Failed to process task %s: %v", task.ID, err)
		}
	}()
}

func (w *TaskWorker) processTask(ctx context.Context, task models.Task, newStatus models.TaskStatus) error {
	select {
	case <-time.After(100 * time.Millisecond):
		task.Status = newStatus

		if newStatus == models.StatusCompleted {
			completedAt := time.Now()
			task.CompletedAt = &completedAt
		}

		return w.repo.Update(ctx, &task)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BatchProcessTasks demonstrates channel-based batch processing
func (w *TaskWorker) BatchProcessTasks(ctx context.Context, taskIDs []uuid.UUID, batchSize int, newStatus models.TaskStatus) error {
	// Create batches
	batches := make([][]uuid.UUID, 0, (len(taskIDs)+batchSize-1)/batchSize)

	for i := 0; i < len(taskIDs); i += batchSize {
		end := i + batchSize
		if end > len(taskIDs) {
			end = len(taskIDs)
		}
		batches = append(batches, taskIDs[i:end])
	}

	// Process batches concurrently
	errChan := make(chan error, len(batches))
	var wg sync.WaitGroup

	for _, batch := range batches {
		wg.Add(1)

		go func(batch []uuid.UUID) {
			defer wg.Done()

			for _, taskID := range batch {
				select {
				case <-ctx.Done():
					errChan <- ctx.Err()
					return
				default:
					task, err := w.repo.FindByID(ctx, taskID)
					if err != nil {
						errChan <- err
						continue
					}

					w.ProcessTaskAsync(ctx, *task, newStatus) // Added newStatus parameter
				}
			}
		}(batch)
	}

	// Wait for all goroutines
	go func() {
		wg.Wait()
		close(errChan)
	}()

	// Collect errors
	var errors []error
	for err := range errChan {
		if err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("batch processing completed with %d errors", len(errors))
	}

	return nil
}

func (w *TaskWorker) Wait() {
	w.wg.Wait()
}
