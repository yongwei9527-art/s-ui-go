package service

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/core"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

func TestInvalidCoreConfigSavePreservesDatabaseAndRunningCore(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-initial-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	oldCore, oldConfig := corePtr, lastSuccessfulConfig
	corePtr = core.NewCore()
	s := &ConfigService{}
	valid := json.RawMessage(`{"log":{"disabled":true},"outbounds":[{"type":"direct","tag":"direct"}]}`)
	if err := corePtr.Start(valid); err != nil {
		t.Fatal(err)
	}
	lastSuccessfulConfig = append(json.RawMessage(nil), valid...)
	t.Cleanup(func() {
		_ = corePtr.Stop()
		corePtr, lastSuccessfulConfig = oldCore, oldConfig
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	before, err := s.SettingService.GetConfig()
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Save("config", "edit", json.RawMessage(`{"route":{"final":"missing-outbound"}}`), "", "test", "")
	if err == nil {
		t.Fatal("invalid core configuration was accepted")
	}
	if !errors.Is(err, errPreviousCoreRestored) {
		t.Fatalf("expected restored-core error, got %v", err)
	}
	if !corePtr.IsRunning() {
		t.Fatal("previous core was not restored")
	}
	after, readErr := s.SettingService.GetConfig()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if after != before {
		t.Fatal("failed configuration save changed the persisted configuration")
	}
}
