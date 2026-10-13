package cronjob

import (
	"github.com/robfig/cron/v3"
	"testing"
	"time"
)

func TestStopWaitsForRunningJobs(t *testing.T) {
	scheduler := cron.New(cron.WithSeconds())
	entered, release := make(chan struct{}), make(chan struct{})
	if _, err := scheduler.AddFunc("@every 1s", func() {
		select {
		case <-entered:
		default:
			close(entered)
		}
		<-release
	}); err != nil {
		t.Fatal(err)
	}
	scheduler.Start()
	c := &CronJob{cron: scheduler}
	defer func() { close(release); c.Stop() }()
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("job did not start")
	}
	stopped := make(chan struct{})
	go func() { c.Stop(); close(stopped) }()
	select {
	case <-stopped:
		t.Fatal("Stop returned before running job finished")
	case <-time.After(50 * time.Millisecond):
	}
	// Release the job without closing the channel twice during cleanup.
	release <- struct{}{}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after job finished")
	}
}
