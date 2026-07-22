package core

import (
	"context"
	"time"

	urltest "github.com/sagernet/sing-box/common/urltest"
)

const checkTimeout = 15 * time.Second

type CheckOutboundResult struct {
	OK    bool
	Delay uint16
	Error string
}

// CheckOutbound takes a stable snapshot of the active outbound and context
// before starting the network operation. This prevents a core Start/Stop from
// racing with reads of the package-level sing-box managers while still letting
// a restart proceed instead of waiting up to the URL-test timeout.
func (c *Core) CheckOutbound(tag string, link string) (result CheckOutboundResult) {
	c.mu.RLock()
	if !c.isRunning || outbound_manager == nil {
		c.mu.RUnlock()
		result.Error = "core not running"
		return result
	}
	ob, ok := outbound_manager.Outbound(tag)
	ctx := globalCtx
	c.mu.RUnlock()

	if !ok {
		result.Error = "outbound not found"
		return result
	}

	ctx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	delay, err := urltest.URLTest(ctx, link, ob)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.OK = true
	result.Delay = delay
	return result
}
