package config

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/render"
	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/internal/adapter/handler/http/common"
	"github.com/moto-nrw/project-phoenix/internal/adapter/logger"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
)

// RetentionSettingsResponse represents the data retention settings response
type RetentionSettingsResponse struct {
	VisitRetentionDays   int        `json:"visit_retention_days"`
	DefaultRetentionDays int        `json:"default_retention_days"`
	MinRetentionDays     int        `json:"min_retention_days"`
	MaxRetentionDays     int        `json:"max_retention_days"`
	LastCleanupRun       *time.Time `json:"last_cleanup_run,omitempty"`
	NextScheduledCleanup *time.Time `json:"next_scheduled_cleanup,omitempty"`
}

// RetentionSettingsRequest represents a request to update retention settings
type RetentionSettingsRequest struct {
	VisitRetentionDays int `json:"visit_retention_days"`
}

// Bind validates the retention settings request
func (req *RetentionSettingsRequest) Bind(_ *http.Request) error {
	return validation.ValidateStruct(req,
		validation.Field(&req.VisitRetentionDays,
			validation.Required,
			validation.Min(1),
			validation.Max(31),
		),
	)
}

// getRetentionSettings handles getting current retention settings
func (rs *Resource) getRetentionSettings(w http.ResponseWriter, r *http.Request) {
	// Get default visit retention days from config
	defaultRetentionSetting, err := rs.ConfigService.GetSettingByKey(r.Context(), "default_visit_retention_days")
	if err != nil {
		// If setting doesn't exist, use default
		defaultRetentionSetting = &config.Setting{
			Key:   "default_visit_retention_days",
			Value: "30",
		}
	}

	defaultDays, _ := strconv.Atoi(defaultRetentionSetting.Value)
	if defaultDays < 1 || defaultDays > 31 {
		defaultDays = 30
	}

	// Get last cleanup run time
	var lastCleanupRun *time.Time
	lastCleanupSetting, err := rs.ConfigService.GetSettingByKey(r.Context(), "last_retention_cleanup")
	if err == nil && lastCleanupSetting.Value != "" {
		if t, err := time.Parse(time.RFC3339, lastCleanupSetting.Value); err == nil {
			lastCleanupRun = &t
		}
	}

	// Calculate next scheduled cleanup (daily at 2 AM)
	now := time.Now()
	nextCleanup := time.Date(now.Year(), now.Month(), now.Day(), 2, 0, 0, 0, now.Location())
	if now.After(nextCleanup) {
		nextCleanup = nextCleanup.AddDate(0, 0, 1)
	}

	response := RetentionSettingsResponse{
		VisitRetentionDays:   defaultDays,
		DefaultRetentionDays: defaultDays,
		MinRetentionDays:     1,
		MaxRetentionDays:     31,
		LastCleanupRun:       lastCleanupRun,
		NextScheduledCleanup: &nextCleanup,
	}

	common.Respond(w, r, http.StatusOK, response, "Retention settings retrieved successfully")
}

// updateRetentionSettings handles updating retention settings
func (rs *Resource) updateRetentionSettings(w http.ResponseWriter, r *http.Request) {
	// Parse request
	req := &RetentionSettingsRequest{}
	if err := render.Bind(r, req); err != nil {
		common.RenderError(w, r, ErrorInvalidRequest(err))
		return
	}

	// Update or create the setting
	setting, err := rs.ConfigService.GetSettingByKey(r.Context(), "default_visit_retention_days")
	if err != nil {
		// Create new setting
		setting = &config.Setting{
			Key:         "default_visit_retention_days",
			Value:       strconv.Itoa(req.VisitRetentionDays),
			Category:    "privacy",
			Description: "Default number of days to retain visit data (1-31)",
		}
		if err := rs.ConfigService.CreateSetting(r.Context(), setting); err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
	} else {
		// Update existing setting
		setting.Value = strconv.Itoa(req.VisitRetentionDays)
		if err := rs.ConfigService.UpdateSetting(r.Context(), setting); err != nil {
			common.RenderError(w, r, ErrorRenderer(err))
			return
		}
	}

	common.Respond(w, r, http.StatusOK, newSettingResponse(setting), "Retention settings updated successfully")
}

// triggerRetentionCleanup handles manual triggering of retention cleanup
func (rs *Resource) triggerRetentionCleanup(w http.ResponseWriter, r *http.Request) {
	// Check if cleanup service is available
	if rs.CleanupService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("cleanup service not available")))
		return
	}

	// Run cleanup
	result, err := rs.CleanupService.CleanupExpiredVisits(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Update last cleanup run time
	lastCleanupSetting, _ := rs.ConfigService.GetSettingByKey(r.Context(), "last_retention_cleanup")
	if lastCleanupSetting == nil {
		lastCleanupSetting = &config.Setting{
			Key:         "last_retention_cleanup",
			Category:    "privacy",
			Description: "Timestamp of last retention cleanup run",
		}
	}
	lastCleanupSetting.Value = time.Now().Format(time.RFC3339)
	if lastCleanupSetting.ID == 0 {
		if err := rs.ConfigService.CreateSetting(r.Context(), lastCleanupSetting); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithField("error", err).Warn("Failed to record cleanup timestamp")
			}
		}
	} else {
		if err := rs.ConfigService.UpdateSetting(r.Context(), lastCleanupSetting); err != nil {
			if logger.Logger != nil {
				logger.Logger.WithField("error", err).Warn("Failed to update cleanup timestamp")
			}
		}
	}

	// Build response
	response := map[string]interface{}{
		"success":            result.Success,
		"students_processed": result.StudentsProcessed,
		"records_deleted":    result.RecordsDeleted,
		"started_at":         result.StartedAt,
		"completed_at":       result.CompletedAt,
		"duration_seconds":   result.CompletedAt.Sub(result.StartedAt).Seconds(),
	}

	if len(result.Errors) > 0 {
		response["error_count"] = len(result.Errors)
		// Include first few errors
		maxErrors := 5
		if len(result.Errors) < maxErrors {
			maxErrors = len(result.Errors)
		}
		errorSummary := make([]string, maxErrors)
		for i := 0; i < maxErrors; i++ {
			errorSummary[i] = result.Errors[i].Error
		}
		response["error_summary"] = errorSummary
	}

	statusCode := http.StatusOK
	message := "Retention cleanup completed successfully"
	if !result.Success {
		statusCode = http.StatusPartialContent
		message = "Retention cleanup completed with errors"
	}

	common.Respond(w, r, statusCode, response, message)
}

// getRetentionStats handles getting retention statistics
func (rs *Resource) getRetentionStats(w http.ResponseWriter, r *http.Request) {
	// Check if cleanup service is available
	if rs.CleanupService == nil {
		common.RenderError(w, r, ErrorInternalServer(errors.New("cleanup service not available")))
		return
	}

	// Get retention statistics
	stats, err := rs.CleanupService.GetRetentionStatistics(r.Context())
	if err != nil {
		common.RenderError(w, r, ErrorInternalServer(err))
		return
	}

	// Build response
	response := map[string]interface{}{
		"total_expired_visits":    stats.TotalExpiredVisits,
		"students_affected":       stats.StudentsAffected,
		"oldest_expired_visit":    stats.OldestExpiredVisit,
		"expired_visits_by_month": stats.ExpiredVisitsByMonth,
	}

	common.Respond(w, r, http.StatusOK, response, "Retention statistics retrieved successfully")
}
