package studentpresencehttp

import (
	errors "errors"
	http "net/http"
	strings "strings"
	time "time"

	absencerecords "github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
	calendar "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Constants for date formats
const (
	dateFormatYYYYMMDD = "2006-01-02"
)

type CreateStudentStatusDaysRequest struct {
	Status string   `json:"status"`
	Dates  []string `json:"dates"`
	Reason string   `json:"reason,omitempty"` // optional free-text reason stamped on each day
}

const maxStudentStatusDayCreateDays = 366

type BulkCreateStudentStatusDaysRequest struct {
	StudentIDs []int64 `json:"student_ids"`
	Status     string  `json:"status"`
	From       string  `json:"from"`
	To         string  `json:"to"`
	Reason     string  `json:"reason,omitempty"`
}

// PrivacyConsentResponse represents a privacy consent response
type PrivacyConsentResponse struct {
	ID                int64                  `json:"id"`
	StudentID         int64                  `json:"student_id"`
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	AcceptedAt        *time.Time             `json:"accepted_at,omitempty"`
	ExpiresAt         *time.Time             `json:"expires_at,omitempty"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	RenewalRequired   bool                   `json:"renewal_required"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
	CreatedAt         time.Time              `json:"created_at"`
	UpdatedAt         time.Time              `json:"updated_at"`
}

// PrivacyConsentRequest represents a privacy consent update request
type PrivacyConsentRequest struct {
	PolicyVersion     string                 `json:"policy_version"`
	Accepted          bool                   `json:"accepted"`
	DurationDays      *int                   `json:"duration_days,omitempty"`
	DataRetentionDays int                    `json:"data_retention_days"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

func (req *CreateStudentStatusDaysRequest) Bind(_ *http.Request) error {
	req.Status = strings.TrimSpace(req.Status)
	if !isValidStudentStatusDayStatus(req.Status) {
		return errors.New("status must be sick, excused, or class_trip")
	}
	return validateStudentStatusDayDates(req.Dates)
}

func validateStudentStatusDayDates(dates []string) error {
	if len(dates) == 0 {
		return errors.New("dates are required")
	}
	if len(dates) > maxStudentStatusDayCreateDays {
		return errors.New("dates cannot exceed 366 items")
	}

	seen := make(map[string]struct{}, len(dates))
	var earliest, latest calendar.Date
	for i, rawDate := range dates {
		date, parsed, err := normalizeStudentStatusDayDate(rawDate)
		if err != nil {
			return err
		}
		if earliest.IsZero() || parsed.Before(earliest) {
			earliest = parsed
		}
		if latest.IsZero() || parsed.After(latest) {
			latest = parsed
		}
		if _, ok := seen[date]; ok {
			return errors.New("duplicate dates are not allowed")
		}
		seen[date] = struct{}{}
		dates[i] = date
	}
	if earliest.DaysUntil(latest) >= maxStudentStatusDayCreateDays {
		return errors.New("dates cannot span more than 366 days")
	}
	return nil
}

func normalizeStudentStatusDayDate(rawDate string) (string, calendar.Date, error) {
	date := strings.TrimSpace(rawDate)
	if date == "" {
		return "", calendar.Date(""), errors.New("date cannot be empty")
	}
	parsed, err := calendar.ParseDate(date)
	if err != nil {
		return "", calendar.Date(""), errors.New("invalid date format, expected YYYY-MM-DD")
	}
	return date, parsed, nil
}

func (req *BulkCreateStudentStatusDaysRequest) Bind(_ *http.Request) error {
	req.Status = strings.TrimSpace(req.Status)
	if !isValidStudentStatusDayStatus(req.Status) {
		return errors.New("status must be sick, excused, or class_trip")
	}
	if len(req.StudentIDs) == 0 {
		return errors.New("student_ids are required")
	}
	if len(req.StudentIDs) > 500 {
		return errors.New("student_ids cannot exceed 500 items")
	}
	seen := make(map[int64]struct{}, len(req.StudentIDs))
	for _, id := range req.StudentIDs {
		if id <= 0 {
			return errors.New("student_ids must be positive")
		}
		if _, ok := seen[id]; ok {
			return errors.New("duplicate student_ids are not allowed")
		}
		seen[id] = struct{}{}
	}
	req.From = strings.TrimSpace(req.From)
	req.To = strings.TrimSpace(req.To)
	if req.From == "" || req.To == "" {
		return errors.New("from and to are required")
	}
	if _, err := time.Parse(dateFormatYYYYMMDD, req.From); err != nil {
		return errors.New("invalid from date format, expected YYYY-MM-DD")
	}
	if _, err := time.Parse(dateFormatYYYYMMDD, req.To); err != nil {
		return errors.New("invalid to date format, expected YYYY-MM-DD")
	}
	return nil
}

func isValidStudentStatusDayStatus(status string) bool {
	switch status {
	case absencerecords.StudentStatusDaySick, absencerecords.StudentStatusDayExcused, absencerecords.StudentStatusDayClassTrip:
		return true
	default:
		return false
	}
}

// Bind validates the privacy consent request
func (req *PrivacyConsentRequest) Bind(_ *http.Request) error {
	if req.PolicyVersion == "" {
		return errors.New("policy version is required")
	}
	if req.DataRetentionDays < studentpresence.MinPrivacyConsentRetentionDays ||
		req.DataRetentionDays > studentpresence.MaxPrivacyConsentRetentionDays {
		return errors.New("data retention days must be between 1 and 31")
	}
	return nil
}
