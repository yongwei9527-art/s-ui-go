package logger

import (
	"sync"
	"testing"
)

func TestLogBufferCountAndConcurrency(t *testing.T) {
	bufferMu.Lock()
	logBuffer = nil
	bufferMu.Unlock()
	for i := 0; i < 5; i++ {
		addToBuffer("INFO", "test")
	}
	if got := len(GetLogs(2, "INFO")); got != 2 {
		t.Fatalf("count=%d want 2", got)
	}
	if len(GetLogs(0, "INFO")) != 0 {
		t.Fatal("zero count returned logs")
	}
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				addToBuffer("INFO", "parallel")
				GetLogs(10, "INFO")
			}
		}()
	}
	wg.Wait()
}
