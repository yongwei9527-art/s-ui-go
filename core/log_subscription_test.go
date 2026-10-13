package core

import (
	"context"
	"github.com/op/go-logging"
	"github.com/sagernet/sing-box/log"
	projectlog "github.com/yongwei9527-art/s-ui-go/logger"
	"io"
	"testing"
	"time"
)

func TestCoreLogSubscriptionLifecycle(t *testing.T) {
	projectlog.InitLogger(logging.ERROR)
	f := NewDefaultFactory(context.Background(), log.Formatter{}, io.Discard, "")
	if err := f.Start(); err != nil {
		t.Fatal(err)
	}
	sub, done, err := f.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	f.NewLogger("test").Info("subscription-entry")
	select {
	case entry := <-sub:
		if entry.Level != log.LevelInfo {
			t.Fatalf("wrong level %v", entry)
		}
	case <-time.After(time.Second):
		t.Fatal("log entry not emitted")
	}
	f.UnSubscribe(sub)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("unsubscribe did not close listener")
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.Subscribe(); err == nil {
		t.Fatal("closed factory accepted subscription")
	}
}
