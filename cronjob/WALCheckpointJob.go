package cronjob

import (
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

type WALCheckpointJob struct{}

func NewWALCheckpointJob() *WALCheckpointJob {
	return &WALCheckpointJob{}
}

func (s *WALCheckpointJob) Run() {
	db := database.GetDB()
	if err := db.Exec("PRAGMA wal_checkpoint(FULL)").Error; err != nil {
		logger.Error("Error checkpointing WAL: ", err.Error())
	}
}
