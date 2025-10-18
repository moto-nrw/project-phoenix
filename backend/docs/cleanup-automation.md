# Automated Data Cleanup Documentation

This document describes the automated cleanup system for expired student visit data in Project Phoenix.

## Overview

The cleanup system automatically deletes student visit records based on individual privacy consent settings. Each student can have a configured data retention period (1-31 days), and visits older than this period are automatically deleted.

## Implementation Options

### Option 1: Built-in Scheduler (Recommended for Development)

The application includes a built-in scheduler that runs cleanup tasks automatically.

**Configuration (in `dev.env` or environment variables):**
```bash
# Enable automated cleanup
CLEANUP_SCHEDULER_ENABLED=true

# Time to run daily cleanup (24-hour format)
CLEANUP_SCHEDULER_TIME=02:00

# Cleanup timeout in minutes
CLEANUP_SCHEDULER_TIMEOUT_MINUTES=30
```

**How it works:**
- When the server starts with `CLEANUP_SCHEDULER_ENABLED=true`, it automatically schedules the cleanup task
- The cleanup runs daily at the specified time (default: 2:00 AM)
- The scheduler handles server restarts gracefully

### Option 2: System Cron Job (Recommended for Production)

For production deployments, you can use a system cron job for better reliability.

**Setup:**
```bash
# Add to system crontab
0 2 * * * cd /path/to/project && ./main cleanup visits >> /var/log/phoenix-cleanup.log 2>&1
```

**Benefits:**
- Runs independently of the application server
- Survives application crashes
- Can be monitored by system monitoring tools

### Option 3: Docker with Built-in Scheduler (Recommended)

For containerized deployments, the built-in scheduler is **enabled by default** in `docker-compose.yml`.

**Usage:**
```bash
# Scheduler runs automatically with default settings
docker compose up -d

# Or run cleanup manually
docker compose exec server ./main cleanup visits
```

**Configuration:**
The scheduler is enabled by default with these environment variables (in `docker-compose.yml`):
- `CLEANUP_SCHEDULER_ENABLED=true` (enabled by default)
- `CLEANUP_SCHEDULER_TIME=02:00` (runs at 2:00 AM by default)
- `CLEANUP_SCHEDULER_TIMEOUT_MINUTES=30` (30 minute timeout)
- `SESSION_END_SCHEDULER_ENABLED=true` (enabled by default)
- `SESSION_END_TIME=18:00` (ends sessions at 6:00 PM by default)

**To disable the scheduler:**
Set `CLEANUP_SCHEDULER_ENABLED=false` in your `.env` file.

### Option 4: Kubernetes CronJob

For Kubernetes deployments, create a CronJob resource:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: phoenix-cleanup
spec:
  schedule: "0 2 * * *"  # Daily at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: cleanup
            image: phoenix-backend:latest
            command: ["./main", "cleanup", "visits"]
            env:
            - name: DB_DSN
              valueFrom:
                secretKeyRef:
                  name: phoenix-secrets
                  key: db-dsn
          restartPolicy: OnFailure
```

## Manual Cleanup Commands

You can also run cleanup manually:

```bash
# Run cleanup for all students
go run main.go cleanup visits

# Preview what would be deleted (dry run)
go run main.go cleanup preview

# Show cleanup statistics
go run main.go cleanup stats

# With Docker
docker compose exec server ./main cleanup visits
```

## Monitoring and Logging

### Log Output

The cleanup service logs important information:
- Start and completion times
- Number of students processed
- Number of records deleted
- Any errors encountered

Example log output:
```
2024-01-08 02:00:00 Starting scheduled visit cleanup...
2024-01-08 02:00:15 Scheduled cleanup completed in 15s: processed 150 students, deleted 3421 records, success: true
```

### Monitoring Recommendations

1. **Log Monitoring**: Monitor cleanup logs for errors or failures
2. **Metrics**: Track:
   - Cleanup execution time
   - Number of records deleted
   - Error rates
3. **Alerts**: Set up alerts for:
   - Cleanup failures
   - Cleanup not running for >25 hours
   - Excessive deletion counts

## Privacy and Compliance

### GDPR Compliance

The cleanup system ensures GDPR compliance by:
- Respecting individual data retention preferences
- Creating audit trails for all deletions in `audit.data_deletions` table
- Only deleting completed visits (with exit_time set)
- Never deleting active session data

### Audit Trail

All deletions are logged in the database:
```sql
SELECT * FROM audit.data_deletions 
WHERE deletion_type = 'visit_retention'
ORDER BY deleted_at DESC;
```

### Data Retention Configuration

Students' data retention periods are configured in the `users.privacy_consents` table:
- Default: 30 days
- Range: 1-31 days
- Can be updated through the application UI

## Troubleshooting

### Cleanup Not Running

1. Check if `CLEANUP_SCHEDULER_ENABLED=true` is set
2. Verify the scheduled time format is correct (HH:MM)
3. Check server logs for scheduler initialization messages
4. Ensure the database connection is working

### Performance Issues

1. The cleanup processes students in batches of 100
2. For large datasets, consider:
   - Running cleanup during off-peak hours
   - Increasing the timeout (`CLEANUP_SCHEDULER_TIMEOUT_MINUTES`)
   - Adding database indexes if needed

### Common Errors

**"No privacy consent found"**
- Normal for students without consent records
- System uses 30-day default retention

**"Failed to delete expired visits"**
- Check database connectivity
- Verify user permissions on the database
- Check for database locks

## Best Practices

1. **Schedule during low-traffic periods** (e.g., 2-4 AM)
2. **Monitor cleanup logs** regularly
3. **Test cleanup in staging** before production
4. **Backup data** before first production run
5. **Set up alerting** for cleanup failures
6. **Review retention policies** periodically with legal team