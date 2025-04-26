/*
 * @Author: AsisYu 2773943729@qq.com
 * @Date: 2025-04-10 16:08:00
 * @Description: 工作池模式实现
 */
package services

import (
	"context"
	"sync"
)

// WorkerPool represents a pool of workers
type WorkerPool struct {
	tasks   chan func()
	wg      sync.WaitGroup
	workers int
}

// NewWorkerPool creates a new worker pool with specified number of workers
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		tasks:   make(chan func(), workers*2), // 缓冲大小为工作者数量的两倍
		workers: workers,
	}
}

// Start starts the worker pool
func (p *WorkerPool) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for task := range p.tasks {
				task()
			}
		}()
	}
}

// Submit submits a task to the worker pool
func (p *WorkerPool) Submit(task func()) bool {
	select {
	case p.tasks <- task:
		return true
	default:
		return false // 任务队列已满
	}
}

// SubmitWithContext submits a task with context
func (p *WorkerPool) SubmitWithContext(ctx context.Context, task func()) bool {
	select {
	case p.tasks <- task:
		return true
	case <-ctx.Done():
		return false
	default:
		return false
	}
}

// Stop stops the worker pool
func (p *WorkerPool) Stop() {
	close(p.tasks)
	p.wg.Wait()
}
