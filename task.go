package workerpool

import (
	"context"
	"errors"
)

// Task is an interface that represents a task that can be executed.
type Task interface {
	// Execute executes the task.
	//
	// The context can be used to cancel the task if it takes too long to run.
	Execute(ctx context.Context) error

	// OnSuccess is called when the task succeeds.
	OnSuccess()

	// OnError is called when the task fails.
	OnError(error)

	// Init is called to initialize the task.
	Init() *TaskConfig
}

// TaskConfig is used to configure the behavior of a task.
type TaskConfig struct {
	// RetryLimit is the maximum number of times that a task will be retried if it fails.
	RetryLimit uint8
	// RetryThreshold is the number of milliseconds to hold before a task will be retried.
	RetryThreshold int
}

// ErrTaskConfig is the error returned when the task configuration is invalid.
var ErrTaskConfig = errors.New("invalid task configuration")
