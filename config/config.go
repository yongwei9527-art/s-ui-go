package config

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed version
var version string

//go:embed name
var name string

type LogLevel string

const (
	Debug LogLevel = "debug"
	Info  LogLevel = "info"
	Warn  LogLevel = "warn"
	Error LogLevel = "error"
)

func GetVersion() string {
	return strings.TrimSpace(version)
}

func GetName() string {
	return strings.TrimSpace(name)
}

func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("SUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

func IsDebug() bool {
	return os.Getenv("SUI_DEBUG") == "true"
}

// GetInitialAdminUsername returns the username to create only when the
// database has no users yet. It is intentionally ignored on later starts.
func GetInitialAdminUsername() string {
	username := strings.TrimSpace(os.Getenv("SUI_INITIAL_ADMIN_USERNAME"))
	if username == "" {
		return "admin"
	}
	return username
}

// ValidateInitialAdminUsername rejects an explicitly configured blank
// username. An unset username intentionally defaults to admin for backwards
// compatibility, while a typo such as SUI_INITIAL_ADMIN_USERNAME=" " must not
// silently weaken the bootstrap account.
func ValidateInitialAdminUsername() error {
	username, configured := os.LookupEnv("SUI_INITIAL_ADMIN_USERNAME")
	if configured && strings.TrimSpace(username) == "" {
		return fmt.Errorf("SUI_INITIAL_ADMIN_USERNAME must not be empty or whitespace")
	}
	return nil
}

// GetInitialAdminPassword returns the operator-provided first-start password.
// The boolean distinguishes an intentionally configured value from an unset
// variable so callers can reject an empty configured password safely.
func GetInitialAdminPassword() (string, bool) {
	return os.LookupEnv("SUI_INITIAL_ADMIN_PASSWORD")
}

func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("SUI_DB_FOLDER")
	if dbFolderPath == "" {
		dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
		if err != nil {
			// Cross-platform fallback path
			if runtime.GOOS == "windows" {
				return "C:\\Program Files\\s-ui\\db"
			}
			return "/usr/local/s-ui/db"
		}
		dbFolderPath = filepath.Join(dir, "db")
	}
	return dbFolderPath
}

func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}
