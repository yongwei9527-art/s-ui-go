package service

import (
	"strconv"
	"testing"

	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
)

func TestPersistentRevisionDoesNotReusePreviousDatabaseCache(t *testing.T) {
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-password")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if db, err := database.GetDB().DB(); err == nil {
			_ = db.Close()
		}
	})
	lastUpdateMu.Lock()
	old, oldInitialized := LastUpdate, lastUpdateInitialized
	LastUpdate, lastUpdateInitialized = 41, true
	lastUpdateMu.Unlock()
	t.Cleanup(func() {
		lastUpdateMu.Lock()
		LastUpdate, lastUpdateInitialized = old, oldInitialized
		lastUpdateMu.Unlock()
	})
	// A restored/imported database can have an older history than the old cache.
	if err := database.GetDB().Create(&model.Changes{Id: 7, Key: "db", Action: "fixture"}).Error; err != nil {
		t.Fatal(err)
	}
	s := &ConfigService{}
	if got := s.CurrentChangeRevision(); got != 7 {
		t.Fatalf("revision=%d want persisted 7, not stale cache", got)
	}
	changed, err := s.CheckChanges(strconv.FormatUint(41, 10))
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("restored database was hidden by the previous in-memory revision")
	}
}
