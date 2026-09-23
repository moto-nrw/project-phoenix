package domain

import (
	"fmt"
	"strconv"
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
	monthNumber := int(month)
	year += (monthNumber - 1) / 12
	monthNumber = (monthNumber-1)%12 + 1
	return fmt.Sprintf("%04d-%02d-%02d", year, monthNumber, day)
}

// BillingPeriod is the first day of the month of keyDate; one row per school
// and period exists at most.
func BillingPeriod(keyDate string) string {
	year, month, _, ok := billingDateParts(keyDate)
	if !ok {
		return ""
	}
	return billingDate(year, time.Month(month), 1)
}

func billingDateParts(value string) (year, month, day int, ok bool) {
	if len(value) != len(billingDateLayout) || value[4] != '-' || value[7] != '-' {
		return 0, 0, 0, false
	}
	for _, index := range []int{0, 1, 2, 3, 5, 6, 8, 9} {
		if value[index] < '0' || value[index] > '9' {
			return 0, 0, 0, false
		}
	}
	year, _ = strconv.Atoi(value[:4])
	month, _ = strconv.Atoi(value[5:7])
	day, _ = strconv.Atoi(value[8:])
	if month < int(time.January) || month > int(time.December) || day < 1 || day > billingDaysInMonth(year, month) {
		return 0, 0, 0, false
	}
	return year, month, day, true
}

func billingDaysInMonth(year, month int) int {
	switch time.Month(month) {
	case time.April, time.June, time.September, time.November:
		return 30
	case time.February:
		if year%4 == 0 && (year%100 != 0 || year%400 == 0) {
			return 29
		}
		return 28
	default:
		return 31
	}
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

// BillingKeyDateWasConfigured reports whether the current setting was in
// place when this month's key date became capturable. A later setting change
// must not invent a past key date from live data.
func BillingKeyDateWasConfigured(updatedAt, local time.Time, keyDay int) bool {
	captureAt := time.Date(
		local.Year(), local.Month(), keyDay, BillingCaptureHour, 0, 0, 0, local.Location(),
	)
	return !updatedAt.In(local.Location()).After(captureAt)
}

// NextBillingKeyDate is the next key date whose counts are not due yet at
// local: from the key date's capture hour on, that is next month's.
func NextBillingKeyDate(local time.Time, keyDay int) string {
	if _, due := DueBillingKeyDate(local, keyDay); due {
		return billingDate(local.Year(), local.Month()+1, keyDay)
	}
	return billingDate(local.Year(), local.Month(), keyDay)
}
