package crawler

import (
	"context"
	"sync"
)

type Control struct {
	mu       sync.Mutex
	paused   bool
	resumeCh chan struct{}
}

func NewControl() *Control {
	return &Control{resumeCh: make(chan struct{})}
}

func (c *Control) Pause() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused {
		return
	}
	c.paused = true
	c.resumeCh = make(chan struct{})
}

func (c *Control) Resume() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.paused {
		return
	}
	c.paused = false
	close(c.resumeCh)
}

func (c *Control) IsPaused() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.paused
}

func (c *Control) Wait(ctx context.Context) error {
	c.mu.Lock()
	if !c.paused {
		c.mu.Unlock()
		return nil
	}
	resume := c.resumeCh
	c.mu.Unlock()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-resume:
		return nil
	}
}
