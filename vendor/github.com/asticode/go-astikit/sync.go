package astikit

import (
	"bytes"
	"context"
	"sync"
)

const (
	ChanAddStrategyBlockWhenStarted = "block.when.started"
	ChanAddStrategyNoBlock          = "no.block"
	ChanOrderFIFO                   = "fifo"
	ChanOrderFILO                   = "filo"
)

// Chan is an object capable of executing funcs in a specific order while controlling the conditions
// in which adding new funcs is blocking
// Check out ChanOptions for detailed options
type Chan struct {
	cancel   context.CancelFunc
	c        *sync.Cond
	ctx      context.Context
	fs       []func()
	mc       *sync.Mutex // Locks ctx
	mf       *sync.Mutex // Locks fs
	o        ChanOptions
	oStart   *sync.Once
	oStop    *sync.Once
	statWait *DurationPercentageStat
}

// ChanOptions are Chan options
type ChanOptions struct {
	// Determines the conditions in which adding new funcs is blocking.
	// Possible strategies are :
	//   - calling Add() never blocks (default). Use the ChanAddStrategyNoBlock constant.
	//   - calling Add() only blocks if the chan has been started and the ctx
	//     has not been canceled. Use the ChanAddStrategyBlockWhenStarted constant.
	AddStrategy string
	// Order in which the funcs will be processed. See constants with the pattern ChanOrder*
	Order string
	// By default the funcs not yet processed when the context is cancelled are dropped.
	// If "ProcessAll" is true,  ALL funcs are processed even after the context is cancelled.
	// However, no funcs can be added after the context is cancelled
	ProcessAll bool
}

// NewChan creates a new Chan
func NewChan(o ChanOptions) *Chan {
	return &Chan{
		c:      sync.NewCond(&sync.Mutex{}),
		mc:     &sync.Mutex{},
		mf:     &sync.Mutex{},
		o:      o,
		oStart: &sync.Once{},
		oStop:  &sync.Once{},
	}
}

// Start starts the chan by looping through functions in the buffer and executing them if any, or waiting for a new one
// otherwise
func (c *Chan) Start(ctx context.Context) {
	// Make sure to start only once
	c.oStart.Do(func() {
		// Create context
		c.mc.Lock()
		c.ctx, c.cancel = context.WithCancel(ctx)
		c.mc.Unlock()

		// Reset once
		c.oStop = &sync.Once{}

		// Handle context
		go func() {
			// Wait for context to be done
			<-c.ctx.Done()

			// Signal
			c.c.L.Lock()
			c.c.Signal()
			c.c.L.Unlock()
		}()

		// Loop
		for {
			// Lock cond here in case a func is added between retrieving l and doing the if on it
			c.c.L.Lock()

			// Get number of funcs in buffer
			c.mf.Lock()
			l := len(c.fs)
			c.mf.Unlock()

			// Only return if context has been cancelled and:
			//   - the user wants to drop funcs that has not yet been processed
			//   - the buffer is empty otherwise
			c.mc.Lock()
			if c.ctx.Err() != nil && (!c.o.ProcessAll || l == 0) {
				c.mc.Unlock()
				c.c.L.Unlock()
				return
			}
			c.mc.Unlock()

			// No funcs in buffer
			if l == 0 {
				if c.statWait != nil {
					c.statWait.Begin()
				}
				c.c.Wait()
				if c.statWait != nil {
					c.statWait.End()
				}
				c.c.L.Unlock()
				continue
			}
			c.c.L.Unlock()

			// Get first func
			c.mf.Lock()
			fn := c.fs[0]
			c.mf.Unlock()

			// Execute func
			fn()

			// Remove first func
			c.mf.Lock()
			c.fs = c.fs[1:]
			c.mf.Unlock()
		}
	})
}

// Stop stops the chan
func (c *Chan) Stop() {
	// Make sure to stop only once
	c.oStop.Do(func() {
		// Cancel context
		if c.cancel != nil {
			c.cancel()
		}

		// Reset once
		c.oStart = &sync.Once{}
	})
}

// Add adds a new item to the chan
func (c *Chan) Add(i func()) {
	// Check context
	c.mc.Lock()
	if c.ctx != nil && c.ctx.Err() != nil {
		c.mc.Unlock()
		return
	}
	c.mc.Unlock()

	// Wrap the function
	var fn func()
	var wg *sync.WaitGroup
	if c.o.AddStrategy == ChanAddStrategyBlockWhenStarted {
		wg = &sync.WaitGroup{}
		wg.Add(1)
		fn = func() {
			defer wg.Done()
			i()
		}
	} else {
		fn = i
	}

	// Add func to buffer
	c.mf.Lock()
	if c.o.Order == ChanOrderFILO {
		c.fs = append([]func(){fn}, c.fs...)
	} else {
		c.fs = append(c.fs, fn)
	}
	c.mf.Unlock()

	// Signal
	c.c.L.Lock()
	c.c.Signal()
	c.c.L.Unlock()

	// Wait
	if wg != nil {
		wg.Wait()
	}
}

// Reset resets the chan
func (c *Chan) Reset() {
	c.mf.Lock()
	defer c.mf.Unlock()
	c.fs = []func(){}
}

// AddStats adds stats to the stater
func (c *Chan) AddStats(s *Stater) {
	// Create stats
	if c.statWait == nil {
		c.statWait = NewDurationPercentageStat()
	}

	// Add wait stat
	s.AddStat(StatMetadata{
		Description: "Percentage of time spent listening and waiting for new object",
		Label:       "Wait ratio",
		Unit:        "%",
	}, c.statWait)
}

// BufferPool represents a *bytes.Buffer pool
type BufferPool struct {
	bp *sync.Pool
}

// NewBufferPool creates a new BufferPool
func NewBufferPool() *BufferPool {
	return &BufferPool{bp: &sync.Pool{New: func() interface{} { return &bytes.Buffer{} }}}
}

// New creates a new BufferPoolItem
func (p *BufferPool) New() *BufferPoolItem {
	return newBufferPoolItem(p.bp.Get().(*bytes.Buffer), p.bp)
}

// BufferPoolItem represents a BufferPool item
type BufferPoolItem struct {
	*bytes.Buffer
	bp *sync.Pool
}

func newBufferPoolItem(b *bytes.Buffer, bp *sync.Pool) *BufferPoolItem {
	return &BufferPoolItem{
		Buffer: b,
		bp:     bp,
	}
}

// Close implements the io.Closer interface
func (i *BufferPoolItem) Close() error {
	i.Reset()
	i.bp.Put(i.Buffer)
	return nil
}

// GoroutineLimiter is an object capable of doing several things in parallel while maintaining the
// max number of things running in parallel under a threshold
type GoroutineLimiter struct {
	busy   int
	c      *sync.Cond
	ctx    context.Context
	cancel context.CancelFunc
	o      GoroutineLimiterOptions
}

// GoroutineLimiterOptions represents GoroutineLimiter options
type GoroutineLimiterOptions struct {
	Max int
}

// NewGoroutineLimiter creates a new GoroutineLimiter
func NewGoroutineLimiter(o GoroutineLimiterOptions) (l *GoroutineLimiter) {
	l = &GoroutineLimiter{
		c: sync.NewCond(&sync.Mutex{}),
		o: o,
	}
	if l.o.Max <= 0 {
		l.o.Max = 1
	}
	l.ctx, l.cancel = context.WithCancel(context.Background())
	go l.handleCtx()
	return
}

// Close closes the limiter properly
func (l *GoroutineLimiter) Close() error {
	l.cancel()
	return nil
}

func (l *GoroutineLimiter) handleCtx() {
	<-l.ctx.Done()
	l.c.L.Lock()
	l.c.Broadcast()
	l.c.L.Unlock()
}

// GoroutineLimiterFunc is a GoroutineLimiter func
type GoroutineLimiterFunc func()

// Do executes custom work in a goroutine
func (l *GoroutineLimiter) Do(fn GoroutineLimiterFunc) (err error) {
	// Check context in case the limiter has already been closed
	if err = l.ctx.Err(); err != nil {
		return
	}

	// Lock
	l.c.L.Lock()

	// Wait for a goroutine to be available
	for l.busy >= l.o.Max {
		l.c.Wait()
	}

	// Check context in case the limiter has been closed while waiting
	if err = l.ctx.Err(); err != nil {
		return
	}

	// Increment
	l.busy++

	// Unlock
	l.c.L.Unlock()

	// Execute in a goroutine
	go func() {
		// Decrement
		defer func() {
			l.c.L.Lock()
			l.busy--
			l.c.Signal()
			l.c.L.Unlock()
		}()

		// Execute
		fn()
	}()
	return
}
