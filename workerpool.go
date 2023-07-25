// Copyright (c) 2023 Rifaudeen. All rights reserved.
//
// This file is part of github.com/rifaideen/workerpool.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
//	import "fmt"
//	import "github.com/rifaideen/workerpool"
//	import "github.com/rifaideen/workerpool/task"
//
//	type MyTask struct {
//		config *task.Config
//	}
//
//	func (t *MyTask) Execute(ctx context.Context) error {
//		// Do something.
//		return nil
//	}
//
//	func (t *MyTask) OnSuccess() {
//		fmt.Println("Success!")
//	}
//
//	func (t *MyTask) OnError(err error) {
//		fmt.Println("Error:", err)
//	}
//
//	func (t *MyTask) Init() *task.Config {
//		return t.config
//	}
//
//	func main()  {
//		config := workerpool.Config{
//			WorkersCount: 3,
//			Verbose:      false,
//		}
//
//		pool, err := workerpool.New(&config)
//
//		if err != nil {
//			log.Fatal(err)
//		}
//
//		pool.Start()
//		defer pool.Stop()
//
//		// Create a task config.
//		taskConfig := &task.Config{
//			RetryLimit: 3,
//			RetryThreshold: 1000, // in ms
//			Verbose: true, // display verbose output
//		}
//
//		// Create a task.
//		task := &MyTask{
//			config: taskConfig,
//		}
//
//		// call to Add() may block, if the tasks are full.
//		// The task size can be configured using Config.TaskSize during the initialization
//		if added := pool.Add(task); !added {
//			fmt.Println("Cannot add task.")
//		}
//
//		time.Sleep(time.Second * 1)
//	}
package workerpool

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rifaideen/workerpool/task"
)

// WorkerPool is a pool of workers that can be used to execute tasks concurrently.
type WorkerPool struct {
	// Number of workers in the pool.
	numWorkers uint8

	// Tasks to be executed in the pool.
	tasks chan task.Task

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

// Config is used to configure a WorkerPool.
type Config struct {
	// Number of workers in the pool.
	WorkersCount uint8
	// Size of tasks the pool can hold, before it's blocking the queue. Defaults to 10.
	TaskSize uint8
	// Whether to display verbose information.
	Verbose bool
}

// ErrConfig is the error returned when the worker pool configuration is invalid.
var ErrConfig = errors.New("invalid worker pool configuration")

// NewWorkerPool creates a new WorkerPool with the specified configuration.
//
// It returns a new WorkerPool instance with the specified configuration and error if any configuration error.
func New(config *Config) (*WorkerPool, error) {
	if config == nil {
		return nil, fmt.Errorf("%w: configuration cannot be nil", ErrConfig)
	}

	if config.WorkersCount <= 0 {
		return nil, fmt.Errorf("%w: workers count must be greater than 0", ErrConfig)
	}

	taskSize := config.TaskSize

	if taskSize <= 0 {
		taskSize = 10
	}

	pool := &WorkerPool{
		numWorkers: config.WorkersCount,
		tasks:      make(chan task.Task, taskSize),
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
	pool.log(fmt.Sprintf("Started Worker Id: #%d\n", id))

	for {
		select {
		// wait for quit signal and quit worker
		case <-pool.quit:
			pool.log(fmt.Sprintf("Quit signal received. Quitting worker. Worker Id: #%d\n", id))

			return
		// wait for task and execute it
		case currentTask, ok := <-pool.tasks:
			if !ok {
				pool.log(fmt.Sprintf("Channel closed. Quitting worker. Worker Id: #%d\n", id))

				return
			}

			var err error

			// Init the task and get the task level configurations.
			config := currentTask.Init()

			// Validate the task configurations
			if config.RetryLimit == 0 {
				log.Fatalf("%s: retry limit must be greater than 0", task.ErrConfig)
			}

			// Execute the task till the tass is either successful or retry till it reaches the retry limit
			for i := 0; i < int(config.RetryLimit); i++ {
				pool.log(fmt.Sprintf("Executing Task on Worker Id: #%d\n", id))
				pool.log(fmt.Sprintf("Task: %+v\n", currentTask))

				// TODO: make use of context with timeout
				err = currentTask.Execute(context.TODO())

				// Task execution success
				if err == nil {
					// call the success callback
					currentTask.OnSuccess()
					pool.log("Done")

					break
				}

				// Task execution failed, retry the task
				pool.log(fmt.Sprintf("Task Execution Failed: %s\nRetrying task.\n", err))

				// sleep a little bit before retrying the task, if the threshold is set.
				if config.RetryThreshold > 0 {
					time.Sleep(time.Millisecond * time.Duration(config.RetryThreshold))
				}
			}

			// Task execution failed permanently
			if err != nil {
				pool.log(fmt.Sprintf("Task Execution Failed Permanently: %s\n", err))

				// call the error callback
				currentTask.OnError(err)
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
		pool.log("Stopping worker pools.")

		// close the worker pool's quit channel
		close(pool.quit)
		pool.stopped = true
		pool.log("Done")
	})
}

// Add adds a task to the worker pool.
//
// The task will be executed by one of the worker goroutines in the pool.
func (pool *WorkerPool) Add(task task.Task) bool {
	if pool.stopped {
		return false
	}

	pool.tasks <- task

	return true
}

// log logs the message if verbose flag is set to true.
func (pool *WorkerPool) log(message string) {
	if pool.verbose {
		fmt.Println(message)
	}
}
