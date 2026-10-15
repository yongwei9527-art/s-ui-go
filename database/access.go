package database

import (
	"context"
	"errors"
	"sync"
)

var ErrRecoveryInProgress = errors.New("database recovery in progress; retry after the application restarts")

// accessGate leases cover a complete request/job, not one SQL statement. Never
// use a blocking/nested read lock: recovery must drain existing work while new
// requests and cron ticks fail closed instead of joining the old database.
type accessGate struct {
	mu      sync.Mutex
	active  int
	busy    bool
	paused  bool
	drained chan struct{}
}

var applicationDBAccess accessGate

func (g *accessGate) tryAcquire() (func(), bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.busy || g.paused {
		return nil, false
	}
	g.active++
	var once sync.Once
	return func() { once.Do(g.release) }, true
}

func (g *accessGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.active--
	if g.active == 0 && g.drained != nil {
		close(g.drained)
		g.drained = nil
	}
}

func (g *accessGate) beginRecovery(ctx context.Context) error {
	g.mu.Lock()
	if g.busy || g.paused {
		g.mu.Unlock()
		return ErrRecoveryInProgress
	}
	g.busy = true
	done := make(chan struct{})
	if g.active == 0 {
		close(done)
	} else {
		g.drained = done
	}
	g.mu.Unlock()
	select {
	case <-ctx.Done():
		g.mu.Lock()
		g.busy = false
		g.drained = nil
		g.mu.Unlock()
		return ctx.Err()
	case <-done:
		if err := ctx.Err(); err != nil {
			g.finishRecovery(false)
			return err
		}
		return nil
	}
}

func (g *accessGate) finishRecovery(waitForRestart bool) {
	g.mu.Lock()
	g.paused = waitForRestart
	g.busy = false
	g.mu.Unlock()
}

func (g *accessGate) beginStart() (func(bool), error) {
	g.mu.Lock()
	if g.busy {
		g.mu.Unlock()
		return nil, ErrRecoveryInProgress
	}
	// Also close admission on a fresh process: the HTTP listener starts before
	// subscriptions and proxy cores. An early readiness response must not turn
	// that partial startup into a successful recovery signal.
	g.paused = true
	g.active++
	g.mu.Unlock()
	var once sync.Once
	return func(success bool) {
		once.Do(func() {
			g.mu.Lock()
			if success {
				g.paused = false
			}
			g.mu.Unlock()
			g.release()
		})
	}, nil
}

// TryAcquireAccess must be called once at each asynchronous entry boundary.
// Synchronous service calls inherit their caller's lease. The returned release
// must run after all database reads, writes, and runtime changes have completed.
func TryAcquireAccess() (func(), bool) { return applicationDBAccess.tryAcquire() }

func RecoveryBlocked() bool {
	applicationDBAccess.mu.Lock()
	defer applicationDBAccess.mu.Unlock()
	return applicationDBAccess.busy || applicationDBAccess.paused
}

// BeginApplicationStart admits lifecycle reads during the post-import pause.
// Only successful startup reopens ordinary requests and background work.
func BeginApplicationStart() (func(bool), error) { return applicationDBAccess.beginStart() }
