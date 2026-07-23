package service

import (
	"strings"
	"testing"
	"time"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

func TestStartCoreDoesNotReportBusyOrCooldownAsSuccess(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	startCoreMu.Lock()
	oldCore := corePtr
	oldInProgress := startCoreInProgress
	oldLastFailure := lastStartFailTime
	oldFailureCount := startFailCount
	corePtr = &core.Core{}
	startCoreInProgress = true
	lastStartFailTime = time.Time{}
	startFailCount = 0
	startCoreMu.Unlock()
	t.Cleanup(func() {
		startCoreMu.Lock()
		corePtr = oldCore
		startCoreInProgress = oldInProgress
		lastStartFailTime = oldLastFailure
		startFailCount = oldFailureCount
		startCoreMu.Unlock()
	})

	service := &ConfigService{}
	if err := service.StartCore(); err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("busy StartCore error = %v", err)
	}

	startCoreMu.Lock()
	startCoreInProgress = false
	lastStartFailTime = time.Now()
	startFailCount = 1
	startCoreMu.Unlock()
	if err := service.StartCore(); err == nil || !strings.Contains(err.Error(), "cooling down") {
		t.Fatalf("cooldown StartCore error = %v", err)
	}
}
