package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Configuration defaults
const (
	DefaultOutputDir       = "./output"
	DefaultIntervalMinutes = 60
	DefaultAccountsFile    = "accounts.yaml"
	DefaultRunOnInterval   = false
	DefaultIdle            = false
)

type Config struct {
	OutputDir       string
	IntervalMinutes int
	AccountsFile    string
	RunOnInterval   bool
	Idle            bool
}

func Load() Config {
	// Load .env file if it exists
	if err := godotenv.Load(); err != nil {
		// It's okay if .env doesn't exist
	}

	cfg := Config{
		OutputDir:       os.Getenv("BACKUP_OUTPUT_DIR"),
		AccountsFile:    os.Getenv("BACKUP_ACCOUNTS_FILE"),
		RunOnInterval:   DefaultRunOnInterval,
		Idle:            DefaultIdle,
	}

	if cfg.OutputDir == "" {
		cfg.OutputDir = DefaultOutputDir
	}
	if cfg.AccountsFile == "" {
		cfg.AccountsFile = DefaultAccountsFile
	}

	intervalStr := os.Getenv("BACKUP_INTERVAL_MINUTES")
	if intervalStr == "" {
		cfg.IntervalMinutes = DefaultIntervalMinutes
	} else {
		val, err := strconv.Atoi(intervalStr)
		if err != nil {
			log.Printf("Invalid BACKUP_INTERVAL_MINUTES, using default: %v", err)
			cfg.IntervalMinutes = DefaultIntervalMinutes
		} else {
			cfg.IntervalMinutes = val
		}
	}

	runOnIntervalStr := os.Getenv("BACKUP_RUN_ON_INTERVAL")
	if runOnIntervalStr != "" {
		val, err := strconv.ParseBool(runOnIntervalStr)
		if err != nil {
			log.Printf("Invalid BACKUP_RUN_ON_INTERVAL, using default (%v): %v", DefaultRunOnInterval, err)
		} else {
			cfg.RunOnInterval = val
		}
	}

	idleStr := os.Getenv("BACKUP_IDLE")
	if idleStr != "" {
		val, err := strconv.ParseBool(idleStr)
		if err != nil {
			log.Printf("Invalid BACKUP_IDLE, using default (%v): %v", DefaultIdle, err)
		} else {
			cfg.Idle = val
		}
	}

	return cfg
}
