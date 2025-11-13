package config

import (
	"encoding/json"
	"os"
)

type Configuration struct {
	MegaEmail    string      `json:"mega_email"`
	MegaPassword string      `json:"mega_password"`
	BackupJobs   []BackupJob `json:"backup_jobs"`
	LogFile      string      `json:"log_file"`
}

type Telegram struct {
	TelegramBotAPI  string `json:"telegram_bot_api"`
	TelegramUserID  string `json:"tele_user_id"`
	TelegramMessage string
}

// BackupJob defines a single backup task
type BackupJob struct {
	Name            string   `json:"name"`
	SourcePaths     []string `json:"source_paths"`     // Files/folders to backup
	IntervalMinutes int      `json:"interval_minutes"` // Backup interval in minutes
	MegaFolder      string   `json:"mega_folder"`      // Destination folder in MEGA
	Enabled         bool     `json:"enabled"`
	LastRun         string   `json:"last_run,omitempty"`
}

// loadConfig reads and parses the configuration file
func LoadConfig(path string) (*Configuration, error) {
	file, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Configuration
	if err := json.Unmarshal(file, &config); err != nil {
		return nil, err
	}

	// Set default log file if not specified
	if config.LogFile == "" {
		config.LogFile = "backup.log"
	}

	return &config, nil
}
