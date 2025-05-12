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

// WorkerPool 工作池结构体
type WorkerPool struct {
	tasks   chan func()
	wg      sync.WaitGroup
	workers int
}

// NewWorkerPool 创建一个指定工作者数量的工作池
func NewWorkerPool(workers int) *WorkerPool {
	return &WorkerPool{
		tasks:   make(chan func(), workers*2), // 缓冲大小为工作者数量的两倍
		workers: workers,
	}
}

// Start 启动工作池
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

// Submit 提交任务到工作池
func (p *WorkerPool) Submit(task func()) bool {
	select {
	case p.tasks <- task:
		return true
	default:
		return false // 任务队列已满
	}
}

// SubmitWithContext 提交带有上下文的任务
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

// Stop 停止工作池
func (p *WorkerPool) Stop() {
	close(p.tasks)
	p.wg.Wait()
}
