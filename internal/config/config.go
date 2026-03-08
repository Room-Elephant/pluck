package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Room-Elephant/pluck/internal/client"
	"github.com/Room-Elephant/pluck/internal/log"
	"github.com/Room-Elephant/pluck/internal/placer"
)

// Config holds all runtime configuration sourced from environment variables.
// Every field mirrors a PLUCK_* variable
type Config struct {
	Client         string
	ClientURL      string
	ClientUser     string
	ClientPass     string
	Mode           string
	WatchDir       string
	RulesFile      string
	StateFile      string
	RescanInterval time.Duration
	RescanRaw      string
	DryRun         bool
	LogLevel       string
}

func Load() (Config, error) {
	appConfig := Config{
		Client:     getEnv("PLUCK_CLIENT", "transmission"),
		ClientURL:  getEnv("PLUCK_CLIENT_URL", "http://transmission:9091"),
		ClientUser: getEnv("PLUCK_CLIENT_USER", ""),
		ClientPass: getEnv("PLUCK_CLIENT_PASS", ""),
		Mode:       getEnv("PLUCK_MODE", "hardlink"),
		WatchDir:   getEnv("PLUCK_WATCH_DIR", "/data/downloads"),
		RulesFile:  getEnv("PLUCK_RULES_FILE", "/config/rules.conf"),
		StateFile:  getEnv("PLUCK_STATE_FILE", "/config/state.txt"),
		RescanRaw:  getEnv("PLUCK_RESCAN_INTERVAL", "3600"),
		DryRun:     getEnv("PLUCK_DRY_RUN", "false") == "true",
		LogLevel:   getEnv("PLUCK_LOG_LEVEL", "info"),
	}

	if err := appConfig.validate(); err != nil {
		return Config{}, err
	}
	return appConfig, nil
}

func (appConfig *Config) validate() error {
	var validationErrors []string

	if !contains(client.SupportedClients(), appConfig.Client) {
		validationErrors = append(validationErrors, fmt.Sprintf("unsupported client %q (supported: %s)", appConfig.Client, strings.Join(client.SupportedClients(), ", ")))
	}

	if !contains(placer.SupportedModes(), appConfig.Mode) {
		validationErrors = append(validationErrors, fmt.Sprintf("unsupported mode %q (supported: %s)", appConfig.Mode, strings.Join(placer.SupportedModes(), ", ")))
	}

	if !contains(log.SupportedLevels(), appConfig.LogLevel) {
		validationErrors = append(validationErrors, fmt.Sprintf("unsupported log level %q (supported: %s)", appConfig.LogLevel, strings.Join(log.SupportedLevels(), ", ")))
	}
	if appConfig.ClientURL == "" {
		validationErrors = append(validationErrors, "PLUCK_CLIENT_URL must not be empty")
	}
	if appConfig.WatchDir == "" {
		validationErrors = append(validationErrors, "PLUCK_WATCH_DIR must not be empty")
	} else if _, err := os.Stat(appConfig.WatchDir); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("PLUCK_WATCH_DIR %q: %v", appConfig.WatchDir, err))
	}
	if appConfig.RulesFile == "" {
		validationErrors = append(validationErrors, "PLUCK_RULES_FILE must not be empty")
	} else if _, err := os.Stat(appConfig.RulesFile); err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("PLUCK_RULES_FILE %q: %v", appConfig.RulesFile, err))
	}
	if appConfig.StateFile == "" {
		validationErrors = append(validationErrors, "PLUCK_STATE_FILE must not be empty")
	}

	rescanSecs, err := strconv.Atoi(appConfig.RescanRaw)
	if err != nil {
		validationErrors = append(validationErrors, fmt.Sprintf("PLUCK_RESCAN_INTERVAL %q is not a valid integer", appConfig.RescanRaw))
	} else if rescanSecs <= 0 {
		validationErrors = append(validationErrors, fmt.Sprintf("PLUCK_RESCAN_INTERVAL must be positive, got %d", rescanSecs))
	} else {
		appConfig.RescanInterval = time.Duration(rescanSecs) * time.Second
	}

	if len(validationErrors) > 0 {
		return fmt.Errorf("invalid configuration:\n  - %s", strings.Join(validationErrors, "\n  - "))
	}
	return nil
}

func contains(list []string, targetValue string) bool {
	for _, item := range list {
		if item == targetValue {
			return true
		}
	}
	return false
}

func getEnv(key, fallback string) string {
	if envValue := os.Getenv(key); envValue != "" {
		return envValue
	}
	return fallback
}
