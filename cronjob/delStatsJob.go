package cronjob

import (
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/service"
)

type DelStatsJob struct {
	service.StatsService
	trafficAge int
}

func NewDelStatsJob(ta int) *DelStatsJob {
	return &DelStatsJob{
		trafficAge: ta,
	}
}

func (s *DelStatsJob) Run() {
	err := s.StatsService.DelOldStats(s.trafficAge)
	if err != nil {
		logger.Warning("Deleting old statistics failed: ", err)
		return
	}
	logger.Debug("Stats older than ", s.trafficAge, " days were deleted")
}
