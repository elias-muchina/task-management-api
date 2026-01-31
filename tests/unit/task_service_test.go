package unit

import (
	"context"
	"testing"
	"time"

	"task-manager-api/internal/models"
	"task-manager-api/internal/service"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock repository
type MockTaskRepository struct {
	mock.Mock
}

func (m *MockTaskRepository) Create(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Task, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*models.Task), args.Error(1)
}

func (m *MockTaskRepository) FindByUserID(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	args := m.Called(ctx, userID, filter)
	return args.Get(0).([]models.Task), args.Error(1)
}

func (m *MockTaskRepository) Update(ctx context.Context, task *models.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockTaskRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockTaskRepository) GetTasksWithConcurrency(ctx context.Context, userID uuid.UUID, filter models.TaskFilter) ([]models.Task, error) {
	args := m.Called(ctx, userID, filter)
	return args.Get(0).([]models.Task), args.Error(1)
}

func TestTaskWorker_ProcessConcurrentTasks(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	worker := service.NewTaskWorker(5, mockRepo)

	tasks := []models.Task{
		{ID: uuid.New(), Title: "Task 1"},
		{ID: uuid.New(), Title: "Task 2"},
		{ID: uuid.New(), Title: "Task 3"},
	}

	// Setup mock for Update calls
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Task")).
		Return(nil).Times(len(tasks))

	// Setup mock for FindByID calls (for BatchProcessTasks test)
	for _, task := range tasks {
		mockRepo.On("FindByID", mock.Anything, task.ID).
			Return(&task, nil).Once()
	}

	// Process tasks concurrently
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, task := range tasks {
		worker.ProcessTaskAsync(ctx, task)
	}

	worker.Wait()

	// Verify all tasks were processed
	mockRepo.AssertExpectations(t)
}

func TestTaskWorker_BatchProcessTasks(t *testing.T) {
	mockRepo := new(MockTaskRepository)
	worker := service.NewTaskWorker(3, mockRepo)

	taskIDs := []uuid.UUID{
		uuid.New(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
		uuid.New(),
	}

	// Setup mock for FindByID calls
	for _, id := range taskIDs {
		task := models.Task{
			ID:    id,
			Title: "Task " + id.String()[:8],
		}
		mockRepo.On("FindByID", mock.Anything, id).
			Return(&task, nil).Once()
	}

	// Setup mock for Update calls
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*models.Task")).
		Return(nil).Times(len(taskIDs))

	// Test batch processing
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := worker.BatchProcessTasks(ctx, taskIDs, 2) // Batch size of 2
	assert.NoError(t, err)

	worker.Wait()
	mockRepo.AssertExpectations(t)
}
