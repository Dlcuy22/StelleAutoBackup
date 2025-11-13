package main

import (
	"log"
	"os"

	"StelleAutoBackup/service"
)

func main() {
	// Check command line arguments
	configPath := "config.json"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	// Create backup system
	backupSystem, err := service.NewBackupSystem(configPath)
	if err != nil {
		log.Fatalf("Failed to initialize backup system: %v", err)
	}

	// Start backup system
	if err := backupSystem.Start(); err != nil {
		log.Fatalf("Backup system failed: %v", err)
	}
}
