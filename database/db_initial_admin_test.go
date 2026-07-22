package database

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yongwei9527-art/s-ui-go/database/model"
	"github.com/yongwei9527-art/s-ui-go/util/common"
)

func TestInitUserUsesConfiguredInitialCredentials(t *testing.T) {
	t.Setenv("SUI_INITIAL_ADMIN_USERNAME", "bootstrap-admin")
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "a-secure-bootstrap-password")
	generatedInitialAdminPassword = ""

	if err := OpenDB(filepath.Join(t.TempDir(), "initial-admin.db")); err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user table: %v", err)
	}
	if err := initUser(); err != nil {
		t.Fatalf("initialize user: %v", err)
	}

	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("load initialized user: %v", err)
	}
	if user.Username != "bootstrap-admin" {
		t.Fatalf("username = %q, want configured bootstrap username", user.Username)
	}
	if !common.CheckPassword(user.Password, "a-secure-bootstrap-password") {
		t.Fatal("configured bootstrap password did not authenticate")
	}
	if generatedInitialAdminPassword != "" {
		t.Fatal("configured bootstrap password must not be copied into generated-password output")
	}
}

func TestInitUserRejectsBlankConfiguredCredentials(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
	}{
		{name: "empty password", username: "admin", password: ""},
		{name: "whitespace password", username: "admin", password: " \t "},
		{name: "empty username", username: "", password: "a-secure-password"},
		{name: "whitespace username", username: "  \t", password: "a-secure-password"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("SUI_INITIAL_ADMIN_USERNAME", tt.username)
			t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", tt.password)
			generatedInitialAdminPassword = ""

			if err := OpenDB(filepath.Join(t.TempDir(), "invalid-bootstrap.db")); err != nil {
				t.Fatalf("open database: %v", err)
			}
			closeTestDatabase(t)
			if err := db.AutoMigrate(&model.User{}); err != nil {
				t.Fatalf("migrate user table: %v", err)
			}
			if err := initUser(); err == nil {
				t.Fatal("initUser accepted blank configured bootstrap credentials")
			}
		})
	}
}

func TestInitUserGeneratedPasswordIsOneTime(t *testing.T) {
	unsetEnvForTest(t, "SUI_INITIAL_ADMIN_USERNAME")
	unsetEnvForTest(t, "SUI_INITIAL_ADMIN_PASSWORD")
	generatedInitialAdminPassword = ""

	if err := OpenDB(filepath.Join(t.TempDir(), "generated-bootstrap.db")); err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user table: %v", err)
	}
	if err := initUser(); err != nil {
		t.Fatalf("initialize user: %v", err)
	}

	password := TakeGeneratedInitialAdminPassword()
	if password == "" {
		t.Fatal("generated bootstrap password is empty")
	}
	if again := TakeGeneratedInitialAdminPassword(); again != "" {
		t.Fatal("generated bootstrap password was returned more than once")
	}

	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("load initialized user: %v", err)
	}
	if user.Username != "admin" {
		t.Fatalf("username = %q, want default admin", user.Username)
	}
	if !common.CheckPassword(user.Password, password) {
		t.Fatal("generated bootstrap password did not authenticate")
	}
}

func TestInitUserDoesNotChangeExistingAccountFromBootstrapEnvironment(t *testing.T) {
	t.Setenv("SUI_INITIAL_ADMIN_USERNAME", "replacement-admin")
	t.Setenv("SUI_INITIAL_ADMIN_PASSWORD", "replacement-password")
	generatedInitialAdminPassword = ""

	if err := OpenDB(filepath.Join(t.TempDir(), "existing-user.db")); err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t)
	if err := db.AutoMigrate(&model.User{}); err != nil {
		t.Fatalf("migrate user table: %v", err)
	}
	existingPassword, err := common.HashPassword("existing-password")
	if err != nil {
		t.Fatalf("hash existing password: %v", err)
	}
	if err := db.Create(&model.User{Username: "existing-admin", Password: existingPassword}).Error; err != nil {
		t.Fatalf("create existing user: %v", err)
	}

	if err := initUser(); err != nil {
		t.Fatalf("initialize existing database: %v", err)
	}

	var user model.User
	if err := db.First(&user).Error; err != nil {
		t.Fatalf("load existing user: %v", err)
	}
	if user.Username != "existing-admin" || !common.CheckPassword(user.Password, "existing-password") {
		t.Fatal("bootstrap environment changed an existing administrator account")
	}
	if generatedInitialAdminPassword != "" {
		t.Fatal("existing database unexpectedly generated a bootstrap password")
	}
}
func unsetEnvForTest(t *testing.T, key string) {
	t.Helper()
	oldValue, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("unset %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			_ = os.Setenv(key, oldValue)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

func closeTestDatabase(t *testing.T) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get SQL database: %v", err)
	}
	t.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			t.Errorf("close SQL database: %v", err)
		}
	})
}
