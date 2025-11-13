# MEGA Auto Backup System

A simple and robust automatic backup system that compresses and uploads files to MEGA cloud storage at configurable intervals.

## Features

- ✅ Automatic scheduled backups
- ✅ Multiple backup jobs with individual schedules
- ✅ Tar.gz compression for efficient storage
- ✅ Recursive folder backup
- ✅ Detailed logging
- ✅ Simple JSON configuration
- ✅ Auto-creates MEGA folders
- ✅ Enable/disable individual jobs

## Installation

### 1. Install Go (if not already installed)
```bash
# Download and install Go from https://golang.org/dl/
```

### 2. Install Dependencies
```bash
go get github.com/t3rm1n4l/go-mega
```

### 3. Build the Application
```bash
go build -o backup-system main.go
```

## Configuration

Create a `config.json` file with your backup settings:

```json
{
  "mega_email": "your-email@example.com",
  "mega_password": "your-mega-password",
  "log_file": "backup.log",
  "backup_jobs": [
    {
      "name": "documents_backup",
      "source_paths": [
        "/home/user/Documents",
        "/home/user/important.txt"
      ],
      "interval_minutes": 60,
      "mega_folder": "Backups/Documents",
      "enabled": true
    }
  ]
}
```

### Configuration Options

| Field | Type | Description |
|-------|------|-------------|
| `mega_email` | string | Your MEGA account email |
| `mega_password` | string | Your MEGA account password |
| `log_file` | string | Path to log file (default: backup.log) |
| `backup_jobs` | array | List of backup jobs to run |

### Backup Job Options

| Field | Type | Description |
|-------|------|-------------|
| `name` | string | Unique identifier for this backup job |
| `source_paths` | array | List of files/folders to backup |
| `interval_minutes` | int | Backup interval in minutes |
| `mega_folder` | string | Destination folder in MEGA (auto-created) |
| `enabled` | bool | Whether this job should run |

### Common Intervals

- `5` - Every 5 minutes
- `60` - Every hour
- `360` - Every 6 hours
- `720` - Every 12 hours
- `1440` - Daily (24 hours)
- `10080` - Weekly (7 days)

## Usage

### Run with default config.json
```bash
./backup-system
```

### Run with custom configuration file
```bash
./backup-system /path/to/custom-config.json
```

### Run as background service (Linux)
```bash
nohup ./backup-system > /dev/null 2>&1 &
```

### Run as systemd service (Linux)

Create `/etc/systemd/system/mega-backup.service`:

```ini
[Unit]
Description=MEGA Auto Backup Service
After=network.target

[Service]
Type=simple
User=your-username
WorkingDirectory=/path/to/backup-system
ExecStart=/path/to/backup-system/backup-system
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl enable mega-backup
sudo systemctl start mega-backup
sudo systemctl status mega-backup
```

## How It Works

1. **Read Configuration**: Loads settings from `config.json`
2. **Connect to MEGA**: Authenticates with your MEGA account
3. **Schedule Jobs**: Each enabled backup job runs on its own schedule
4. **Compress Files**: Creates a `.tar.gz` archive with timestamp
5. **Upload to MEGA**: Uploads the archive to the specified MEGA folder
6. **Clean Up**: Removes temporary local archive after successful upload
7. **Repeat**: Waits for the next interval and repeats

### Archive Naming

Archives are named: `{job_name}_{timestamp}.tar.gz`

Example: `documents_backup_2024-11-13_14-30-00.tar.gz`

## Example Scenarios

### Hourly Document Backup
```json
{
  "name": "docs_hourly",
  "source_paths": ["/home/user/Documents"],
  "interval_minutes": 60,
  "mega_folder": "Backups/Hourly/Documents",
  "enabled": true
}
```

### Daily Full System Backup
```json
{
  "name": "system_daily",
  "source_paths": [
    "/home/user",
    "/etc"
  ],
  "interval_minutes": 1440,
  "mega_folder": "Backups/Daily/System",
  "enabled": true
}
```

### Weekly Database Backup
```json
{
  "name": "database_weekly",
  "source_paths": ["/var/lib/mysql"],
  "interval_minutes": 10080,
  "mega_folder": "Backups/Weekly/Database",
  "enabled": true
}
```

## Monitoring

### Check Logs
```bash
tail -f backup.log
```

### Sample Log Output
```
[BACKUP] 2024/11/13 14:30:00 Starting backup system...
[BACKUP] 2024/11/13 14:30:02 Successfully connected to MEGA
[BACKUP] 2024/11/13 14:30:02 Started backup job: documents_backup (interval: 60 minutes)
[BACKUP] 2024/11/13 14:30:02 Starting backup job: documents_backup
[BACKUP] 2024/11/13 14:30:05 [documents_backup] Compressing files...
[BACKUP] 2024/11/13 14:30:12 [documents_backup] Archive created: 45.23 MB
[BACKUP] 2024/11/13 14:30:12 [documents_backup] Uploading to MEGA...
[BACKUP] 2024/11/13 14:31:45 [documents_backup] Backup completed successfully in 1m43s
```

## Troubleshooting

### Login Failed
- Verify your MEGA email and password in `config.json`
- Check if your MEGA account has 2FA enabled (currently not supported by go-mega)

### Upload Failed
- Check your MEGA storage quota
- Verify network connection
- Check if the archive size exceeds MEGA limits

### Permission Denied
- Ensure the program has read access to source paths
- Ensure write access to log file location
- Use `sudo` if backing up system directories

### Compression Failed
- Check if source paths exist
- Verify sufficient disk space in temp directory
- Check read permissions on source files

## Best Practices

1. **Test First**: Start with a small backup job to verify everything works
2. **Monitor Logs**: Regularly check logs for errors
3. **Storage Management**: Periodically clean old backups from MEGA to free space
4. **Secure Config**: Protect `config.json` as it contains your MEGA password
   ```bash
   chmod 600 config.json
   ```
5. **Backup Important Data**: Don't rely solely on automated backups - maintain multiple backup strategies
6. **Schedule Wisely**: Avoid scheduling large backups during peak hours

## Security Notes

- Store `config.json` securely (contains credentials)
- Use strong MEGA password
- Consider using environment variables for credentials instead of config file
- Regularly rotate MEGA password

## Limitations

- Does not support MEGA accounts with 2FA enabled (go-mega library limitation)
- Maximum file size depends on available RAM (archives are created in memory)
- No incremental backup support (full backups only)

## License

MIT License - Feel free to modify and use as needed.

## Support

For issues with:
- **This backup system**: Check logs and configuration
- **go-mega library**: Visit https://github.com/t3rm1n4l/go-mega
- **MEGA service**: Visit https://mega.nz/support