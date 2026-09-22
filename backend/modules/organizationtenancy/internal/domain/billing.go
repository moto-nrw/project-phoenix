package domain

import (
	"time"
)

// The billing key day (#2791) is one day of the month for every school.
// It stops at 28 so every month, February included, has it.
const (
	MinBillingKeyDay     = 1
	MaxBillingKeyDay     = 28
	DefaultBillingKeyDay = 15
	// BillingCaptureHour is the local hour from which the key date's counts
	// are captured. By then the hourly student activation has moved the
	// children whose care starts or ends that day.
	BillingCaptureHour = 6
)

// billingDateLayout is how billing dates travel: a calendar day as
// YYYY-MM-DD, bound to the DATE columns through an explicit ::date cast, so
// no instant and no timezone conversion is involved.
const billingDateLayout = "2006-01-02"

// BillingSettings is the one key day that applies to every school.
type BillingSettings struct {
	KeyDay              int
	UpdatedAt           time.Time
	UpdatedByOperatorID *int64
}

// BillingSchool is a school the capture writes a row for, with the names the
// row keeps so a later rename does not change an issued invoice.
type BillingSchool struct {
	ID               int64
	Name             string
	OrganizationName string
}

// BillingKeyDateCount is one school's captured figures of one month. Period
// and KeyDate are calendar days (YYYY-MM-DD); Period is the month's first.
type BillingKeyDateCount struct {
	SchoolID         int64
	SchoolName       string
	OrganizationName string
	Period           string
	KeyDate          string
	ActiveStudents   int
	ActiveTerminals  int
	RecordedAt       time.Time
}

// ValidBillingKeyDay reports whether day can be a key day.
func ValidBillingKeyDay(day int) bool {
	return day >= MinBillingKeyDay && day <= MaxBillingKeyDay
}

func billingDate(year int, month time.Month, day int) string {
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC).Format(billingDateLayout)
}

// BillingPeriod is the first day of the month of keyDate; one row per school
// and period exists at most.
func BillingPeriod(keyDate string) string {
	day, err := time.Parse(billingDateLayout, keyDate)
	if err != nil {
		return ""
	}
	return billingDate(day.Year(), day.Month(), 1)
}

// DueBillingKeyDate returns the key date of the month of local, and whether
// its counts are due. local must be the current time on the school calendar's
// clock (Berlin). The counts are due from the key date's capture hour until
// the month ends; a capture after the key date (the worker was down) is late
// and keeps its own recorded time.
func DueBillingKeyDate(local time.Time, keyDay int) (string, bool) {
	keyDate := billingDate(local.Year(), local.Month(), keyDay)
	switch {
	case local.Day() > keyDay:
		return keyDate, true
	case local.Day() == keyDay && local.Hour() >= BillingCaptureHour:
		return keyDate, true
	default:
		return keyDate, false
	}
}

// NextBillingKeyDate is the first key date on or after the calendar day of
// local.
func NextBillingKeyDate(local time.Time, keyDay int) string {
	if local.Day() > keyDay {
		return billingDate(local.Year(), local.Month()+1, keyDay)
	}
	return billingDate(local.Year(), local.Month(), keyDay)
}
