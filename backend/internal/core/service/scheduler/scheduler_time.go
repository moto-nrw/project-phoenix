package scheduler

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/logger"
)

// parseScheduledTime parses a HH:MM time string into hour and minute components.
// Returns an error if the format is invalid.
func parseScheduledTime(timeStr string) (hour, minute int, err error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid time format: %s (expected HH:MM)", timeStr)
	}

	hour, err = strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return 0, 0, fmt.Errorf("invalid hour in time: %s", timeStr)
	}

	minute, err = strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return 0, 0, fmt.Errorf("invalid minute in time: %s", timeStr)
	}

	return hour, minute, nil
}

// calculateNextRun calculates the next run time for a daily task at the given hour and minute.
// If the time has already passed today, it schedules for tomorrow.
func calculateNextRun(hour, minute int) time.Time {
	now := time.Now()
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if now.After(nextRun) {
		nextRun = nextRun.Add(24 * time.Hour)
	}
	return nextRun
}

// runDailyTask is a generic runner for tasks that execute once per day at a scheduled time.
// It handles parsing the schedule, waiting for the first run, and running on a 24-hour cycle.
func (s *Scheduler) runDailyTask(task *ScheduledTask, execute func()) {
	defer s.wg.Done()

	hour, minute, err := parseScheduledTime(task.Schedule)
	if err != nil {
		logger.Logger.WithField("task", task.Name).WithError(err).Error("Invalid scheduled time")
		return
	}

	nextRun := calculateNextRun(hour, minute)
	initialWait := time.Until(nextRun)
	logger.Logger.WithFields(map[string]interface{}{
		"task":     task.Name,
		"wait":     initialWait.Round(time.Minute).String(),
		"next_run": nextRun.Format("2006-01-02 15:04:05"),
	}).Info("Task scheduled")

	select {
	case <-time.After(initialWait):
		execute()
	case <-s.done:
		return
	}

	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			execute()
		case <-s.done:
			return
		}
	}
}
