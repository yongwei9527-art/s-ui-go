package cronjob

import (
	"github.com/yongwei9527-art/s-ui-go/service"
)

type CheckCoreJob struct {
	service.ConfigService
}

func NewCheckCoreJob() *CheckCoreJob {
	return &CheckCoreJob{}
}

func (s *CheckCoreJob) Run() {
	s.ConfigService.StartCore()
}
