[![Go Reference](https://pkg.go.dev/badge/github.com/rifaideen/workerpool.svg)](https://pkg.go.dev/github.com/rifaideen/workerpool)
[![Go Report Card](https://goreportcard.com/badge/github.com/rifaideen/workerpool)](https://goreportcard.com/report/github.com/rifaideen/workerpool)

# Workerpool

A simple way to execute tasks concurrently.

## Features

- Execute tasks concurrently
- Configurable workerpool instance (See `WorkerPoolConfig`)
- Configurable retry limits and threshold per task (See `TaskConfig`)
- Easy to use
- Callbacks on success and failure
- Create new tasks easily by implementing the **Task** interface

## Installation

```sh
go get github.com/rifaideen/workerpool
```

## Usage

```go
    package main

    import "fmt"
    import "github.com/rifaideen/workerpool"

    type MyTask struct {
        config *workerpool.TaskConfig
    }

    func (t *MyTask) Execute(ctx context.Context) error {
        // Do something.
        return nil
    }

    func (t *MyTask) OnSuccess() {
        fmt.Println("Success!")
    }

    func (t *MyTask) OnError(err error) {
        fmt.Println("Error:", err)
    }

    func (t *MyTask) Init() *workerpool.TaskConfig {
        return t.config
    }

    func main()  {
    	config := workerpool.WorkerPoolConfig{
    		WorkersCount: 3,
    		Verbose:      false,
    	}

    	pool, err := workerpool.NewWorkerPool(&config)

    	if err != nil {
    	    log.Fatal(err)
    	}

    	pool.Start()
    	defer pool.Stop()

        // Create a task config.
        taskConfig := &workerpool.TaskConfig{
            RetryLimit: 3,
            RetryThreshold: 1000, // in ms
        }

        // Create a task.
        task := &MyTask{
            config: taskConfig,
        }

        // call to Add() may block, if the tasks are full.
        // The task size can be configured using WorkerPoolConfig.TaskSize during the initialization
        if added := pool.Add(task); !added {
            fmt.Println("Cannot add task.")
        }

        time.Sleep(time.Second * 1)
    }
```

## Feedback

- [Submit feedback](https://github.com/rifaideen/workerpool/issues/new)

## Disclaimer of Non-Liability

This project is provided **"as is"** and **without any express or implied warranties**, including, but not limited to, the implied warranties of merchantability and fitness for a particular purpose. In no event shall the authors or copyright holders be liable for any claim, damages or other liability, whether in an action of contract, tort or otherwise, arising from, out of or in connection with the software or the use or other dealings in the software.

## License

This project is licensed under the [Apache License, Version 2.0](https://www.apache.org/licenses/LICENSE-2.0).
