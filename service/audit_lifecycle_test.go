package service

import (
	"net"
	"testing"

	"github.com/op/go-logging"
	"github.com/yongwei9527-art/s-ui-go/config"
	"github.com/yongwei9527-art/s-ui-go/database"
	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/logger"
)

func TestTLSRejectsUnknownSaveAction(t *testing.T) {
	if err := (&TlsService{}).Save(nil, "unsupported", nil, ""); err == nil {
		t.Fatal("unknown TLS action accepted")
	}
}

func TestXrayClientReloadDoesNotRestartExplicitlyStoppedRuntime(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	t.Setenv("SUI_DB_FOLDER", t.TempDir())
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "test-only-password")
	t.Setenv("SUI_AUTOCONFIGURE_XRAY_REALITY", "1")
	if err := database.InitDB(config.GetDBPath()); err != nil {
		t.Fatal(err)
	}
	sidecar := &XrayXHTTPService{}
	t.Cleanup(func() {
		_ = sidecar.Stop()
		if sqlDB, err := database.GetDB().DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	var inbounds []model.XrayXHTTPInbound
	if err := database.GetDB().Find(&inbounds).Error; err != nil {
		t.Fatal(err)
	}
	for _, inbound := range inbounds {
		listener, err := net.Listen("tcp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		port := listener.Addr().(*net.TCPAddr).Port
		_ = listener.Close()
		if err := database.GetDB().Model(&model.XrayXHTTPInbound{}).Where("id = ?", inbound.Id).Updates(map[string]interface{}{"port": port, "listen": "127.0.0.1", "dest": "127.0.0.1:1"}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := sidecar.Stop(); err != nil {
		t.Fatal(err)
	}
	// Simulate a previously queued goroutine running only after Stop returned.
	if err := sidecar.ReloadAfterClientChange(); err != nil {
		t.Fatal(err)
	}
	if sidecar.Status()["running"] != false {
		t.Fatal("background client reload reopened the stopped runtime")
	}
	// A deliberate lifecycle start must still work after the stop guard.
	if err := sidecar.StartIfEnabled(); err != nil {
		t.Fatal(err)
	}
	if sidecar.Status()["running"] != true {
		t.Fatal("explicit start did not resume Xray")
	}
}

func TestWireGuardRejectsInvalidPrivateKey(t *testing.T) {
	result := (&ServerService{}).GenKeypair("wireguard", "not-a-key")
	if len(result) == 0 || result[0] != "Failed to parse WireGuard private key: " {
		t.Fatalf("invalid key produced public key: %v", result)
	}
}
