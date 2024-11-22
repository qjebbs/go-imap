// MIT License
//
// Copyright (c) 2024 Jebbs
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package serial

import (
	"context"
	"sync"
)

var closedCtx context.Context

func init() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	closedCtx = ctx
}

// Control is a serial task control that ensures that only
// one task is running at a time.
//
// When a new task created, it will either:
//   - Wait to be executed. (NewSerialControl())
//   - Cancel the current running task, if there is one. (NewSerialControl(WithCancelFormer()))
//   - Cancel the new task, if there is one running. (NewSerialControl(WithCancelLater()))
//
// Tasks order is guaranteed if WithOrdered(n) is used.
//
// Features:
//   - Create a new task context. (New())
//   - Cancel the current running context. (Cancel())
//   - Close the control (cancel current context and prevent new ones). (Close() / Done())
//   - Query status. (Idle() / Closed())
type Control struct {
	done    chan struct{}
	bucket  chan struct{}
	options serialControlOptions

	queue   chan func()
	newLock sync.Mutex

	fieldLock     sync.Mutex
	cancelCurrent func()
	closed        bool
}

// NewControl creates a new serial control.
func NewControl(options ...ControlOption) *Control {
	c := &Control{
		bucket:  make(chan struct{}, 1),
		done:    make(chan struct{}),
		options: defaultSerialControlOptions,
	}
	// put a token in the bucket
	c.bucket <- struct{}{}
	for _, opt := range options {
		opt(&c.options)
	}
	if c.options.queueCapacity > 0 {
		c.queue = make(chan func(), c.options.queueCapacity)
		go c.runQueue()
	}
	return c
}

// runQueue runs the queue.
func (c *Control) runQueue() {
	// loop until c.queue is closed and drained
	for task := range c.queue {
		task()
	}
}

// New cancels the previous context and accepts a new one.
//
// !!! It's caller's responsibility to call cancel() when the task is done or canceled,
// otherwise the SerialControl will be blocked forever.
func (c *Control) New(parent context.Context) (ctx context.Context, cancel func()) {
	if c.queue != nil {
		return c.newWithQueue(parent)
	}
	// make sure only one task is creating at a time,
	// also only one Cancel() at a time, or the cancellation of
	// tasks happens at a time, and most of them cancel nothing.
	c.newLock.Lock()
	defer c.newLock.Unlock()
	return c.new(parent)
}

// newWithQueue is the same as New, but it will put the task into the queue.
func (c *Control) newWithQueue(parent context.Context) (ctx context.Context, cancel func()) {
	c.fieldLock.Lock()
	if c.closed {
		c.fieldLock.Unlock()
		return closedCtx, func() {}
	}
	done := make(chan struct{})
	c.queue <- func() {
		defer close(done)
		ctx, cancel = c.new(parent)
	}
	c.fieldLock.Unlock()
	<-done
	return ctx, cancel
}

func (c *Control) new(parent context.Context) (ctx context.Context, cancel func()) {
	switch c.options.cancelOnNew {
	case cancelOnNewNothing:
	case cancelOnNewLater:
		if !c.Idle() {
			return closedCtx, func() {}
		}
	case cancelOnNewFormer:
		c.Cancel()
	}

	// don't do field lock here before tasks get the token,
	// otherwise c.Cancel() will be blocked util task is done,
	// and it actually does nothing.

	// start a new task, consume the token in the bucket
	<-c.bucket
	// lock until the task is created, so that the c.cancelCurrent is not nil.
	// Or the next call of c.New() will find c.cancelCurrent is nil,
	// missing the chance to cancel the current task.
	c.fieldLock.Lock()
	defer c.fieldLock.Unlock()
	if c.closed {
		// put the token back
		c.bucket <- struct{}{}
		return closedCtx, func() {}
	}
	ctx, ctxCancel := context.WithCancel(parent)
	c.cancelCurrent = ctxCancel
	var once sync.Once
	return ctx, func() {
		// in case done() is called multiple times
		once.Do(func() {
			ctxCancel()
			// put the token back
			c.bucket <- struct{}{}
		})
	}
}

// Idle returns true if no task is running.
func (c *Control) Idle() bool {
	return len(c.bucket) > 0
}

// Closed returns true if the control is closed.
func (c *Control) Closed() bool {
	c.fieldLock.Lock()
	defer c.fieldLock.Unlock()
	return c.closed
}

// Cancel cancels the current task.
//
// !!! It cancels ONLY the current task, not preventing new tasks from being created.
// If you want to make sure all tasks associated with the SerialControl are canceled,
// you should call Close() instead.
func (c *Control) Cancel() {
	c.fieldLock.Lock()
	defer c.fieldLock.Unlock()
	if c.cancelCurrent == nil {
		return
	}
	c.cancelCurrent()
	c.cancelCurrent = nil
}

// CancelAndWait cancels the current task and waits for it to finish.
//
// !!! It cancels ONLY the current task, not preventing new tasks from being created.
// If you want to make sure all tasks associated with the SerialControl are canceled,
// you should call Close() and Done() instead.
func (c *Control) CancelAndWait() {
	c.Cancel()
	<-c.bucket
	c.bucket <- struct{}{}
}

// Close closes the control, it cancels the current task and prevents new ones.
//
// !!! MUST be called when the SerialControl is no longer needed,
// or it will cause a goroutine leak when WithOrdered(n) is used.
//
// !!! Close is the only way to make sure all tasks associated with the SerialControl are canceled.
func (c *Control) Close() {
	c.fieldLock.Lock()
	defer c.fieldLock.Unlock()
	if c.closed {
		return
	}
	c.closed = true
	if c.queue != nil {
		close(c.queue)
	}
	go func() {
		c.CancelAndWait()
		close(c.done)
	}()
}

// Done returns a channel that's closed when the SerialControl is CLOSED and the current task is done.
func (c *Control) Done() <-chan struct{} {
	return c.done
}
