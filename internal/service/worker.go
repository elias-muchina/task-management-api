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

func NewTaskWorker(maxWorkers int, repo repository.TaskRepository) *TaskWorker {
	return &TaskWorker{
		taskChan:   make(chan models.Task, 100),
		workerPool: make(chan struct{}, maxWorkers),
		repo:       repo,
	}
}

// ProcessTaskAsync demonstrates goroutine pool pattern
func (w *TaskWorker) ProcessTaskAsync(ctx context.Context, task models.Task) {
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()

		// Acquire worker slot
		w.workerPool <- struct{}{}
		defer func() { <-w.workerPool }()

		// Process task with timeout
		processCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		if err := w.processTask(processCtx, task); err != nil {
			log.Printf("Failed to process task %s: %v", task.ID, err)
			// Retry logic could be added here
		}
	}()
}

func (w *TaskWorker) processTask(ctx context.Context, task models.Task) error {
	// Simulate some processing time
	select {
	case <-time.After(100 * time.Millisecond):
		// Task processing logic here
		log.Printf("Processed task: %s - %s", task.ID, task.Title)

		// Update task status in database
		completedAt := time.Now()
		task.Status = models.StatusCompleted
		task.CompletedAt = &completedAt

		return w.repo.Update(ctx, &task)
	case <-ctx.Done():
		return ctx.Err()
	}
}

// BatchProcessTasks demonstrates channel-based batch processing
func (w *TaskWorker) BatchProcessTasks(ctx context.Context, taskIDs []uuid.UUID, batchSize int) error {
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

					w.ProcessTaskAsync(ctx, *task)
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
