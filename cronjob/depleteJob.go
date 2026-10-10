package cronjob

import (
	"github.com/yongwei9527-art/s-ui-go/logger"
	"github.com/yongwei9527-art/s-ui-go/service"
)

type DepleteJob struct {
	service.ClientService
	service.ConfigService
}

func NewDepleteJob() *DepleteJob {
	return new(DepleteJob)
}

func (s *DepleteJob) Run() {
	inboundIds, xrayAuthorizationChanged, err := s.ClientService.DepleteClients()
	if err != nil {
		logger.Warning("Disable depleted users failed: ", err)
		return
	}
	if len(inboundIds) > 0 {
		if err := s.ConfigService.EnqueueCurrentConfigRestart(); err != nil {
			logger.Error("unable to schedule core restart after disabling depleted users: ", err)
		}
	}
	if xrayAuthorizationChanged {
		if err := (&service.XrayXHTTPService{}).ReloadAfterClientChange(); err != nil {
			logger.Error("unable to refresh Xray XHTTP clients: ", err)
		}
	}
}
