// Package workerpool provides a simple way to execute tasks concurrently.
//
// A worker pool is a pool of goroutines that can be used to execute tasks concurrently. When a task is added to the worker pool, it is executed by one of the worker goroutines.
//
// The worker pool ensures that only a certain number of tasks are executed concurrently, which can help to prevent resource exhaustion.
//
// To use the worker pool, you first need to create a new WorkerPool instance. The WorkerPool constructor takes a configuration object as its argument. The configuration object allows you to specify the maximum number of workers in the pool, as well as other options.
//
// Once you have created a WorkerPool instance, you can start the workers by calling Start method and start adding tasks to it. The Add method takes a Task object as its argument. A Task object is an interface that represents a task that can be executed. The Task interface has four methods:
//
// * Execute: This method is called to execute the task. The context argument can be used to cancel the task if it takes too long to run.
//
// * OnSuccess: This method is called when the task succeeds.
//
// * OnError: This method is called when the task fails.
//
// * Init: This method is called to initialize the task. This is the good place to initialize any defaults and this method should return the instance of TaskConfig.
//
// When a task is added to the worker pool, it is queued up to be executed by one of the worker goroutines. The worker goroutines will execute the tasks in the order in which they were added.
//
// You can stop the worker pool by calling the Stop method. Once the worker pool is stopped, it will no longer execute tasks.
//
// Example:
//
//	package main
//
//	import "github.com/rifaideen/workerpool"
//
//	func main()  {
//		config := WorkerPoolConfig{
//			WorkersCount: 3,
//			Verbose:      false,
//		}
//
//		pool, err := NewWorkerPool(&config)
//
//		if err != nil {
//		    log.Fatal(err)
//		}
//
//		pool.Start()
//		defer pool.Stop()
//
//		// Add your task here.
//	}
package workerpool

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"
)

// WorkerPool is a pool of workers that can be used to execute tasks concurrently.
type WorkerPool struct {
	// Number of workers in the pool.
	numWorkers uint8

	// Tasks to be executed in the pool.
	tasks chan Task

	// Quit channel to be closed when we want to terminate the pool.
	quit chan struct{}

	// Worker pool starter to start the pool of workers once.
	starter sync.Once

	// Worker pool stopper to stop the pool of workers once.
	stopper sync.Once

	// Worker pool stop status to identify whether the pool is stopped or not.
	stopped bool

	// Display verbose information on various occassions.
	verbose bool
}

// WorkerPoolConfig is used to configure a WorkerPool.
type WorkerPoolConfig struct {
	// Number of workers in the pool.
	WorkersCount uint8
	// Size of tasks the pool can hold, before it's blocking the queue. Defaults to 10.
	TaskSize uint8
	// Whether to display verbose information.
	Verbose bool
}

// ErrWorkerPoolConfig is the error returned when the worker pool configuration is invalid.
var ErrWorkerPoolConfig = errors.New("invalid worker pool configuration")

// NewWorkerPool creates a new WorkerPool with the specified configuration.
//
// It returns a new WorkerPool instance with the specified configuration and error if any configuration error.
func NewWorkerPool(config *WorkerPoolConfig) (*WorkerPool, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: configuration cannot be nil", ErrWorkerPoolConfig)
	}

	if config.WorkersCount <= 0 {
		return nil, fmt.Errorf("%w: workers count must be greater than 0", ErrWorkerPoolConfig)
	}

	taskSize := config.TaskSize

	if taskSize <= 0 {
		taskSize = 10
	}

	pool := &WorkerPool{
		numWorkers: config.WorkersCount,
		tasks:      make(chan Task, taskSize),
		quit:       make(chan struct{}),
		starter:    sync.Once{},
		stopper:    sync.Once{},
		verbose:    config.Verbose,
	}

	return pool, nil
}

// Start starts the worker pool.
//
// It can only be called once, because once it is called, the worker pool will start the workers and listen for tasks. If you try to call Start() again, it will simply ignore it.
//
// The Start() method is typically called in the main() function.
func (pool *WorkerPool) Start() {
	pool.starter.Do(func() {
		for i := 1; i <= int(pool.numWorkers); i++ {
			go pool.startListening(i)
		}
	})
}

// start the workers internally and listen for task and quit signals
func (pool *WorkerPool) startListening(id int) {
	if pool.verbose {
		fmt.Printf("Started Worker Id: #%d\n", id)
	}

	for {
		select {
		// wait for quit signal and quit worker
		case <-pool.quit:
			if pool.verbose {
				fmt.Printf("Quit signal received. Quitting worker. Worker Id: #%d\n", id)
			}

			return
		// wait for task and execute it
		case task, ok := <-pool.tasks:
			if !ok {
				if pool.verbose {
					fmt.Printf("Channel closed. Quitting worker. Worker Id: #%d\n", id)
				}

				return
			}

			var err error

			// Init the task and get the task level configurations.
			config := task.Init()

			// Validate the task configurations
			if config.RetryLimit == 0 {
				log.Fatalf("%s: retry limit must be greater than 0", ErrTaskConfig)
			}

			// Execute the task till the tass is either successful or retry till it reaches the retry limit
			for i := 0; i < int(config.RetryLimit); i++ {
				if pool.verbose {
					fmt.Printf("Executing Task on Worker Id: #%d\n", id)
					fmt.Printf("Task: %+v\n", task)
				}

				// TODO: make use of context with timeout
				err = task.Execute(context.TODO())

				// Task execution success
				if err == nil {
					// call the success callback
					task.OnSuccess()

					if pool.verbose {
						fmt.Println("Done")
					}

					break
				}

				// Task execution failed, retry the task
				if pool.verbose {
					fmt.Printf("Task Execution Failed: %s\nRetrying task.\n", err)
				}

				// sleep a little bit before retrying the task, if the threshold is set.
				if config.RetryThreshold > 0 {
					time.Sleep(time.Millisecond * time.Duration(config.RetryThreshold))
				}
			}

			// Task execution failed permanently
			if err != nil {
				if pool.verbose {
					fmt.Printf("Task Execution Failed Permanently: %s\n", err)
				}

				// call the error callback
				task.OnError(err)
			}
		}
	}
}

// Stop stops the worker pool.
//
// It can only be called once, because once it is called, the worker pool will stop executing tasks. If you try to call Stop() again, it will be ignored.
//
// The Stop() method is typically called in the `main()` function.
func (pool *WorkerPool) Stop() {
	pool.stopper.Do(func() {
		if pool.verbose {
			fmt.Println("Stopping worker pools.")
		}

		// close the worker pool's quit channel
		close(pool.quit)
		pool.stopped = true

		if pool.verbose {
			fmt.Println("Done")
		}
	})
}

// Add adds a task to the worker pool.
//
// The task will be executed by one of the worker goroutines in the pool.
func (pool *WorkerPool) Add(task Task) bool {
	if pool.stopped {
		return false
	}

	pool.tasks <- task

	return true
}
