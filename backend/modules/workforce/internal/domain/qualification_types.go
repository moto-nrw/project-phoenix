package domain

import (
	"errors"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Nachweisarten (#4808): the kinds of proof a school tracks per staff member.
// The standard list matches the module catalogue; a school adds and archives
// its own kinds. A qualification without a kind is "Sonstiges" with a
// free-text name.

// Standard kinds. The keys are stable; the names are the school's to rename.
const (
	QualificationStandardFirstAidChild     = "first_aid_child"
	QualificationStandardRescueAbility     = "rescue_ability"
	QualificationStandardPoliceCertificate = "extended_police_certificate"
)

var (
	// ErrQualificationTypeNameTaken: a live kind of the school has the name.
	ErrQualificationTypeNameTaken = errors.New("staff qualification type name is already taken")
	// ErrQualificationTypeNotFound: the school has no such kind.
	ErrQualificationTypeNotFound = errors.New("staff qualification type not found")
)

// DefaultReminderLeadWeeks is the agreed reminder lead of a kind (#4808).
const DefaultReminderLeadWeeks = 8

// ExpiryReminderCatchUpDays is how long after the expiry day the reminder of
// that day is still sent when the job could not run on the day itself.
const ExpiryReminderCatchUpDays = 6

const (
	maxQualificationTypeName = 100
	maxValidityMonths        = 240
	maxReminderLeadWeeks     = 52
)

// Statuses of a qualification on the Nachweise list.
const (
	QualificationStatusValid        = "valid"
	QualificationStatusExpiringSoon = "expiring_soon"
	QualificationStatusExpired      = "expired"
	QualificationStatusMissing      = "missing"
)

// Reminder stages: once when the lead starts, once on the expiry day.
const (
	QualificationReminderLead   = "lead"
	QualificationReminderExpiry = "expiry"
)

// QualificationType is one kind of proof of a school.
type QualificationType struct {
	ID                int64
	TenantID          int64
	StandardKey       string
	Name              string
	ValidityMonths    *int
	ReminderLeadWeeks int
	Mandatory         bool
	SortOrder         int
	ArchivedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// Archived reports whether the kind is no longer offered for new entries.
func (t QualificationType) Archived() bool { return t.ArchivedAt != nil }

// Normalize trims the name and checks the bounds the table enforces.
func (t QualificationType) Normalize() (QualificationType, error) {
	t.Name = strings.TrimSpace(t.Name)
	switch {
	case t.Name == "":
		return t, invalidStaffRecord("qualification type name is required")
	case len([]rune(t.Name)) > maxQualificationTypeName:
		return t, invalidStaffRecord("qualification type name must not exceed 100 characters")
	case t.ValidityMonths != nil && (*t.ValidityMonths < 1 || *t.ValidityMonths > maxValidityMonths):
		return t, invalidStaffRecord("validity_months must be between 1 and 240")
	case t.ReminderLeadWeeks < 0 || t.ReminderLeadWeeks > maxReminderLeadWeeks:
		return t, invalidStaffRecord("reminder_lead_weeks must be between 0 and 52")
	}
	return t, nil
}

func months(value int) *int { return &value }

// StandardQualificationTypes is the standard list a school starts with. The
// validity suggests the expiry date: the first-aid refresher every two years
// (DGUV Vorschrift 1 § 26), the rescue ability every four years (NRW,
// Sicherheitsförderung im Schulsport), the extended certificate of conduct
// every five years. None is mandatory until the school says so.
func StandardQualificationTypes() []QualificationType {
	return []QualificationType{
		{StandardKey: QualificationStandardFirstAidChild, Name: "Erste Hilfe am Kind", ValidityMonths: months(24), ReminderLeadWeeks: DefaultReminderLeadWeeks, SortOrder: 0},
		{StandardKey: QualificationStandardRescueAbility, Name: "Rettungsfähigkeit", ValidityMonths: months(48), ReminderLeadWeeks: DefaultReminderLeadWeeks, SortOrder: 1},
		{StandardKey: QualificationStandardPoliceCertificate, Name: "Erweitertes Führungszeugnis", ValidityMonths: months(60), ReminderLeadWeeks: DefaultReminderLeadWeeks, SortOrder: 2},
	}
}

// QualificationStatus is the status of a qualification on a calendar day: an
// entry without an expiry date stays valid, the expiry day itself still
// counts as valid ("gültig bis"), and the lead before it is "läuft bald ab".
func QualificationStatus(expiresOn string, leadWeeks int, today calendar.Date) string {
	if expiresOn == "" {
		return QualificationStatusValid
	}
	expiry, err := calendar.ParseDate(expiresOn)
	if err != nil {
		return QualificationStatusValid
	}
	switch {
	case expiry.Before(today):
		return QualificationStatusExpired
	case !today.Before(expiry.AddDays(-7 * leadWeeks)):
		return QualificationStatusExpiringSoon
	default:
		return QualificationStatusValid
	}
}

// DueQualificationReminder names the reminder a qualification is due for on
// a calendar day, if any. The lead reminder goes out from the first day of
// the lead until the day before expiry; the expiry reminder on the expiry
// day, or within the catch-up days after it. Each goes out once per expiry
// date: a changed expiry date arms both again. A lead of zero weeks sends
// only the expiry reminder.
func DueQualificationReminder(q StaffQualification, leadWeeks int, today calendar.Date) (string, bool) {
	if q.ExpiresOn == "" {
		return "", false
	}
	expiry, err := calendar.ParseDate(q.ExpiresOn)
	if err != nil {
		return "", false
	}
	switch {
	case today.Before(expiry):
		if leadWeeks <= 0 || today.Before(expiry.AddDays(-7*leadWeeks)) || q.LeadRemindedFor == q.ExpiresOn {
			return "", false
		}
		return QualificationReminderLead, true
	case today.After(expiry.AddDays(ExpiryReminderCatchUpDays)) || q.ExpiryRemindedFor == q.ExpiresOn:
		return "", false
	default:
		return QualificationReminderExpiry, true
	}
}
