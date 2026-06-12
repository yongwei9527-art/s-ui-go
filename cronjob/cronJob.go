package cronjob

import (
	"time"

	"github.com/robfig/cron/v3"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

type CronJob struct {
	cron *cron.Cron
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int) error {
	c.cron = cron.New(
		cron.WithLocation(loc),
		cron.WithSeconds(),
		cron.WithChain(cron.Recover(cron.DefaultLogger)),
	)

	jobs := []struct {
		spec string
		job  cron.Job
	}{
		{"@every 10s", NewStatsJob(trafficAge > 0)},
		{"@every 1m", NewDepleteJob()},
		{"@every 5s", NewCheckCoreJob()},
		{"@every 10m", NewWALCheckpointJob()},
	}
	if trafficAge > 0 {
		jobs = append(jobs, struct {
			spec string
			job  cron.Job
		}{"@daily", NewDelStatsJob(trafficAge)})
	}
	for _, item := range jobs {
		if _, err := c.cron.AddJob(item.spec, item.job); err != nil {
			logger.Error("add cron job failed:", item.spec, err)
			return err
		}
	}

	c.cron.Start()
	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
