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

package workerpool

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

// Simple multiply operation performed on the even numbers
type MultiplyTask struct {
	value    int
	result   int
	hasError bool
	wg       *sync.WaitGroup
}

// task executor
func (m *MultiplyTask) Execute(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if (m.value % 2) != 0 {
		return errors.New("only even numbers are allowed. Got: " + strconv.Itoa(m.value))
	}

	m.result = karatsuba(m.value, 2)

	return nil
}

// init the task
func (m *MultiplyTask) Init() *TaskConfig {
	return &TaskConfig{
		RetryLimit:     1,
		RetryThreshold: 100,
	}
}

// success callback
func (m *MultiplyTask) OnSuccess() {
	m.wg.Done()
}

// error callback
func (m *MultiplyTask) OnError(err error) {
	m.hasError = true
	m.wg.Done()
}

func TestWorkerPoolInstance(t *testing.T) {
	_, err := NewWorkerPool(nil)

	if err == nil {
		t.Error("Expected error, got nil")
	}

	config := WorkerPoolConfig{
		WorkersCount: 0,
	}
	_, err = NewWorkerPool(&config)

	if err == nil {
		t.Errorf("Expected worker pool configuration error, got %s", err)
	}

	config.WorkersCount = 3
	pool, _ := NewWorkerPool(&config)

	if pool == nil {
		t.Error("Expected worker pool instance, got nil")
	}

	pool.Start()
	defer pool.Stop()
	time.Sleep(time.Second)
}

func TestWorkerPoolWork(t *testing.T) {
	config := WorkerPoolConfig{
		WorkersCount: 0,
		Verbose:      false,
	}
	_, err := NewWorkerPool(&config)

	if err == nil {
		t.Errorf("Expected worker pool configuration error, got %s", err)
	}

	config.Verbose = true
	config.WorkersCount = 10

	pool, err := NewWorkerPool(&config)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if pool == nil {
		t.Error("Expected worker pool instance, got nil")
	}

	pool.Start()
	defer pool.Stop()

	var tasks []*MultiplyTask
	wg := sync.WaitGroup{}
	iterations := 500
	wg.Add(iterations)

	for i := 1; i <= iterations; i++ {
		task := &MultiplyTask{
			value: i,
			wg:    &wg,
		}
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		pool.Add(task)
	}

	wg.Wait()

	if err := verify(tasks); err != nil {
		t.Error(err)
	}
}

func TestWorkerPoolWorkAddSuccess(t *testing.T) {
	config := WorkerPoolConfig{
		WorkersCount: 10,
		Verbose:      true,
	}

	pool, err := NewWorkerPool(&config)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if pool == nil {
		t.Error("Expected worker pool instance, got nil")
	}

	pool.Start()
	defer pool.Stop()

	var tasks []*MultiplyTask
	wg := sync.WaitGroup{}
	iterations := 100
	wg.Add(iterations)

	for i := 1; i <= iterations; i++ {
		task := &MultiplyTask{
			value: i,
			wg:    &wg,
		}
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		pool.Add(task)
	}

	wg.Wait()

	if err := verify(tasks); err != nil {
		t.Error(err)
	}

	task := &MultiplyTask{
		value: iterations + 1,
		wg:    &wg,
	}

	pool.Add(task)
}

func TestWorkerPoolWorkSkipAdd(t *testing.T) {
	config := WorkerPoolConfig{
		WorkersCount: 10,
		Verbose:      true,
	}

	pool, err := NewWorkerPool(&config)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}

	if pool == nil {
		t.Error("Expected worker pool instance, got nil")
	}

	pool.Start()
	pool.Stop()

	var tasks []*MultiplyTask
	wg := sync.WaitGroup{}
	iterations := 3

	for i := 1; i <= iterations; i++ {
		task := &MultiplyTask{
			value: i,
			wg:    &wg,
		}
		tasks = append(tasks, task)
	}

	for _, task := range tasks {
		if status := pool.Add(task); status {
			t.Errorf("Expected add task status to false, got %v", status)
		}
	}
}

func BenchmarkWorkerPool(b *testing.B) {
	config := WorkerPoolConfig{
		WorkersCount: 3,
	}
	pool, _ := NewWorkerPool(&config)

	if pool == nil {
		b.Error("Expected worker pool instance, got nil")
	}

	pool.Start()
	defer pool.Stop()

	var tasks []*MultiplyTask
	wg := sync.WaitGroup{}

	for i := 1; i <= b.N; i++ {
		wg.Add(1)
		tasks = append(tasks, &MultiplyTask{
			value: i,
			wg:    &wg,
		})
	}

	b.ResetTimer()

	for _, task := range tasks {
		pool.Add(task)
	}

	wg.Wait()

	if err := verify(tasks); err != nil {
		b.Error(err)
	}
}

func karatsuba(x, y int) int {
	if x < 100 || y < 100 {
		return x * y
	}

	// Split x and y into two equal halves.
	x1, x2 := x/2, x%2
	y1, y2 := y/2, y%2

	// Calculate the products of the two halves.
	p1 := karatsuba(x1, y1)
	p2 := karatsuba(x2, y2)
	p3 := karatsuba(x1+x2, y1+y2)

	// Combine the products to get the final result.
	return 3*p1 + 2*p2 + p3
}

// verify the results of multiplication
func verify(tasks []*MultiplyTask) error {
	for _, task := range tasks {
		// fmt.Println(task.value, task.result, task.hasError)
		if (task.value % 2) == 0 {
			if task.hasError == true {
				return fmt.Errorf("Expected hasError to be false when even number present(%d), got: %v", task.value, task.hasError)
			}

			if task.result != (task.value * 2) {
				return fmt.Errorf("Expected %d, got: %v", task.value*2, task.result)
			}

		}

		if (task.value % 2) != 0 {
			if task.result != 0 {
				return fmt.Errorf("Expected odd value %d multiplication result to be 0, got: %v", task.value, task.result)
			}

			if task.hasError == false {
				return fmt.Errorf("Expected hasError to be true when odd number present(%d), got: %v", task.value, task.hasError)
			}
		}
	}

	return nil
}
