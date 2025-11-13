package service

import (
	"StelleAutoBackup/config"
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/t3rm1n4l/go-mega"
)

// BackupSystem manages the backup operations
type BackupSystem struct {
	config *config.Configuration
	mega   *mega.Mega
	logger *log.Logger
}

// NewBackupSystem creates a new backup system instance
func NewBackupSystem(configPath string) (*BackupSystem, error) {
	// Read configuration file
	config, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %v", err)
	}

	// Setup logger
	logFile, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}

	logger := log.New(io.MultiWriter(os.Stdout, logFile), "[BACKUP] ", log.LstdFlags)

	return &BackupSystem{
		config: config,
		logger: logger,
	}, nil
}

// Start begins the backup system
func (bs *BackupSystem) Start() error {
	bs.logger.Println("Starting backup system...")

	// Connect to MEGA
	if err := bs.connectMega(); err != nil {
		return fmt.Errorf("failed to connect to MEGA: %v", err)
	}

	bs.logger.Println("Successfully connected to MEGA")

	// Start backup jobs
	for i := range bs.config.BackupJobs {
		job := &bs.config.BackupJobs[i]
		if job.Enabled {
			go bs.runBackupJob(job)
			bs.logger.Printf("Started backup job: %s (interval: %d minutes)\n",
				job.Name, job.IntervalMinutes)
		}
	}

	// Keep the program running
	select {}
}

// connectMega establishes connection to MEGA
func (bs *BackupSystem) connectMega() error {
	bs.mega = mega.New()
	return bs.mega.Login(bs.config.MegaEmail, bs.config.MegaPassword)
}

// runBackupJob executes a backup job periodically
func (bs *BackupSystem) runBackupJob(job *config.BackupJob) {
	ticker := time.NewTicker(time.Duration(job.IntervalMinutes) * time.Minute)
	defer ticker.Stop()

	// Run immediately on start
	bs.performBackup(job)

	// Then run on interval
	for range ticker.C {
		bs.performBackup(job)
	}
}

// performBackup executes the actual backup operation
func (bs *BackupSystem) performBackup(job *config.BackupJob) {
	bs.logger.Printf("Starting backup job: %s\n", job.Name)
	startTime := time.Now()

	// Create archive filename with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	archiveName := fmt.Sprintf("%s_%s.tar.gz", job.Name, timestamp)
	archivePath := filepath.Join(os.TempDir(), archiveName)

	// Compress files
	bs.logger.Printf("[%s] Compressing files...\n", job.Name)
	if err := bs.compressFiles(job.SourcePaths, archivePath); err != nil {
		bs.logger.Printf("[%s] ERROR: Compression failed: %v\n", job.Name, err)
		return
	}

	// Get file size for logging
	fileInfo, _ := os.Stat(archivePath)
	fileSizeMB := float64(fileInfo.Size()) / (1024 * 1024)
	bs.logger.Printf("[%s] Archive created: %.2f MB\n", job.Name, fileSizeMB)

	// Upload to MEGA
	bs.logger.Printf("[%s] Uploading to MEGA...\n", job.Name)
	if err := bs.uploadToMega(archivePath, archiveName, job.MegaFolder); err != nil {
		bs.logger.Printf("[%s] ERROR: Upload failed: %v\n", job.Name, err)
		os.Remove(archivePath)
		return
	}

	// Clean up local archive
	os.Remove(archivePath)

	// Update last run time
	job.LastRun = time.Now().Format(time.RFC3339)

	duration := time.Since(startTime)
	bs.logger.Printf("[%s] Backup completed successfully in %v\n", job.Name, duration)
}

// compressFiles creates a tar.gz archive from the specified paths
func (bs *BackupSystem) compressFiles(sourcePaths []string, destPath string) error {
	// Create output file
	outFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Create gzip writer
	gzipWriter := gzip.NewWriter(outFile)
	defer gzipWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	// Add each source path to the archive
	for _, sourcePath := range sourcePaths {
		if err := bs.addToArchive(tarWriter, sourcePath, ""); err != nil {
			return fmt.Errorf("failed to add %s: %v", sourcePath, err)
		}
	}

	return nil
}

// addToArchive recursively adds files/folders to the tar archive
func (bs *BackupSystem) addToArchive(tarWriter *tar.Writer, sourcePath, baseInArchive string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	// Determine the path in the archive
	var pathInArchive string
	if baseInArchive == "" {
		pathInArchive = filepath.Base(sourcePath)
	} else {
		pathInArchive = filepath.Join(baseInArchive, filepath.Base(sourcePath))
	}

	if info.IsDir() {
		// Add directory
		entries, err := os.ReadDir(sourcePath)
		if err != nil {
			return err
		}

		// Recursively add directory contents
		for _, entry := range entries {
			entryPath := filepath.Join(sourcePath, entry.Name())
			if err := bs.addToArchive(tarWriter, entryPath, pathInArchive); err != nil {
				return err
			}
		}
	} else {
		// Add file
		return bs.addFileToArchive(tarWriter, sourcePath, pathInArchive, info)
	}

	return nil
}

// addFileToArchive adds a single file to the tar archive
func (bs *BackupSystem) addFileToArchive(tarWriter *tar.Writer, filePath, pathInArchive string, info os.FileInfo) error {
	// Create tar header
	header := &tar.Header{
		Name:    pathInArchive,
		Size:    info.Size(),
		Mode:    int64(info.Mode()),
		ModTime: info.ModTime(),
	}

	// Write header
	if err := tarWriter.WriteHeader(header); err != nil {
		return err
	}

	// Open and copy file content
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(tarWriter, file)
	return err
}

// uploadToMega uploads a file to MEGA cloud storage
func (bs *BackupSystem) uploadToMega(localPath, fileName, megaFolder string) error {
	// Get or create the destination folder
	parentNode, err := bs.getOrCreateMegaFolder(megaFolder)
	if err != nil {
		return err
	}

	// Upload the file
	_, err = bs.mega.UploadFile(localPath, parentNode, fileName, nil)
	return err
}

// getOrCreateMegaFolder gets or creates a folder in MEGA
func (bs *BackupSystem) getOrCreateMegaFolder(folderPath string) (*mega.Node, error) {
	if folderPath == "" || folderPath == "/" {
		return bs.mega.FS.GetRoot(), nil
	}

	// Try to find existing folder
	nodes, err := bs.mega.FS.PathLookup(bs.mega.FS.GetRoot(), []string{folderPath})
	if err == nil && len(nodes) > 0 {
		return nodes[len(nodes)-1], nil
	}

	// Create the folder if it doesn't exist
	bs.logger.Printf("Creating MEGA folder: %s\n", folderPath)
	return bs.mega.CreateDir(folderPath, bs.mega.FS.GetRoot())
}
