package database

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"sync"
	"time"
)

// The application owns runtime lifecycle; the database package must never
// stop the HTTP server from inside its own import handler (Shutdown deadlocks).
type RecoveryHooks struct {
	Quiesce          func() error
	ResumeAfterAbort func() error
}

var recoveryHooksMu sync.RWMutex
var recoveryHooks RecoveryHooks

func RegisterRecoveryHooks(hooks RecoveryHooks) func() {
	recoveryHooksMu.Lock()
	previous := recoveryHooks
	recoveryHooks = hooks
	recoveryHooksMu.Unlock()
	return func() {
		recoveryHooksMu.Lock()
		recoveryHooks = previous
		recoveryHooksMu.Unlock()
	}
}

func ImportDBContext(ctx context.Context, file multipart.File) error {
	recoveryHooksMu.RLock()
	hooks := recoveryHooks
	recoveryHooksMu.RUnlock()
	return coordinatedImport(ctx, file, hooks, scheduleImportRestart)
}

func coordinatedImport(ctx context.Context, file multipart.File, hooks RecoveryHooks, schedule func(<-chan bool) error) (resultErr error) {
	if file == nil {
		return errors.New("missing database upload")
	}
	if hooks.Quiesce == nil || hooks.ResumeAfterAbort == nil {
		return errors.New("database recovery lifecycle is not initialized")
	}
	// Bound the drain. Cancellation before drain completion cannot close or
	// replace the old DB. Once replacement starts it must finish/roll back.
	drainCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := applicationDBAccess.beginRecovery(drainCtx); err != nil {
		return err
	}
	decision := make(chan bool, 1)
	attemptedQuiesce, swapStarted := false, false
	completed, dispatchRestart, waitForRestart := false, false, false
	defer func() {
		if !completed {
			// A panic must neither dispatch a successful reload nor strand busy
			// forever. Once runtime quiescing began, require explicit startup.
			waitForRestart = attemptedQuiesce
		}
		// New requests and jobs stay blocked after a replacement until a full
		// application restart reloads tokens, settings, subscriptions and cores.
		applicationDBAccess.finishRecovery(waitForRestart)
		decision <- completed && dispatchRestart
	}()
	resultErr = importDBBeforeClose(file, func() error { return schedule(decision) }, func() error {
		attemptedQuiesce = true
		if err := hooks.Quiesce(); err != nil {
			return fmt.Errorf("quiesce runtime before database recovery: %w", err)
		}
		swapStarted = true
		return nil
	})
	waitForRestart = resultErr == nil || swapStarted
	if resultErr != nil && attemptedQuiesce && !swapStarted {
		// Keep finalization in the outer defer even if the resume hook panics.
		waitForRestart = true
		if resumeErr := hooks.ResumeAfterAbort(); resumeErr != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("resume runtime after recovery abort: %w", resumeErr))
		} else {
			waitForRestart = false
		}
	}
	if resultErr != nil && waitForRestart {
		resultErr = fmt.Errorf("%w; database access remains paused: inspect the logs and any retained recovery file, then restart the service", resultErr)
	}
	dispatchRestart = resultErr == nil
	completed = true
	return resultErr
}
