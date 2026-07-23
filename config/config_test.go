package config

import (
	"path/filepath"
	"testing"
)

func TestProductRenameKeepsLegacyDatabasePath(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("SUI_DB_FOLDER", dbDir)
	if got := GetName(); got != "s-ui-go" {
		t.Fatalf("product name = %q, want s-ui-go", got)
	}
	if got := GetVersion(); got != "0.1" {
		t.Fatalf("product version = %q, want 0.1", got)
	}
	if got := GetDatabaseSchemaVersion(); got != "1.3" {
		t.Fatalf("database schema version = %q, want 1.3", got)
	}
	if got, want := GetDBPath(), filepath.Join(dbDir, "s-ui.db"); got != want {
		t.Fatalf("database path = %q, want legacy-compatible %q", got, want)
	}
}
