package workforce

import (
	"context"
	"errors"
	"log/slog"
)

// Nachweisarten, Nachweise list and reminders (#4808). Workforce owns the
// kinds of proof and the qualifications; the root binds the staff directory,
// Delivery's notifications and mail outbox and the school's name to the
// ports below.

var (
	// ErrQualificationTypeNameTaken: a live kind of the school has the name.
	ErrQualificationTypeNameTaken = errors.New("staff qualification type name is already taken")
	// ErrQualificationTypeNotFound: the school has no such kind.
	ErrQualificationTypeNotFound = errors.New("staff qualification type not found")
)

// DefaultQualificationReminderLeadWeeks is the agreed reminder lead of a kind
// when a request names none (#4808).
const DefaultQualificationReminderLeadWeeks = 8

// Statuses on the Nachweise list.
const (
	QualificationStatusValid        = "valid"
	QualificationStatusExpiringSoon = "expiring_soon"
	QualificationStatusExpired      = "expired"
	QualificationStatusMissing      = "missing"
)

// Reminder stages.
const (
	QualificationReminderLead   = "lead"
	QualificationReminderExpiry = "expiry"
)

// QualificationType is one kind of proof of a school. StandardKey is set for
// the standard list.
type QualificationType struct {
	ID                int64
	StandardKey       string
	Name              string
	ValidityMonths    *int
	ReminderLeadWeeks int
	Mandatory         bool
	Archived          bool
}

// QualificationTypeInput is the full editable state of a kind.
type QualificationTypeInput struct {
	Name              string
	ValidityMonths    *int
	ReminderLeadWeeks int
	Mandatory         bool
	Archived          bool
}

// StaffQualificationTypes administers the kinds of the school. The list
// adds the standard kinds a school does not have yet.
type StaffQualificationTypes interface {
	ListQualificationTypes(ctx context.Context) ([]QualificationType, error)
	CreateQualificationType(ctx context.Context, input QualificationTypeInput) (QualificationType, error)
	UpdateQualificationType(ctx context.Context, id int64, input QualificationTypeInput) (QualificationType, error)
}

// QualificationEntry is one qualification with its status on a calendar
// day. A missing mandatory kind has no QualificationID.
type QualificationEntry struct {
	StaffID         int64
	QualificationID *int64
	TypeID          *int64
	Name            string
	AcquiredOn      string
	ExpiresOn       string
	Status          string
	Mandatory       bool
}

// QualificationOverviewRow is one line of the Nachweise list.
type QualificationOverviewRow struct {
	QualificationEntry
	FirstName string
	LastName  string
}

// StaffQualificationOverview reads the Nachweise: the list over every live
// staff member and one person's own entries, each with the missing
// mandatory kinds.
type StaffQualificationOverview interface {
	QualificationOverview(ctx context.Context) ([]QualificationOverviewRow, error)
	StaffQualificationStatus(ctx context.Context, staffID int64) ([]QualificationEntry, error)
}

// QualificationStaffMember is a live staff member with the contact the
// reminders use.
type QualificationStaffMember struct {
	StaffID   int64
	FirstName string
	LastName  string
	AccountID int64
	Email     string
}

// QualificationStaff is the staff directory the Nachweise need.
type QualificationStaff interface {
	// ListStaffMembers returns the live staff of the school.
	ListStaffMembers(ctx context.Context) ([]QualificationStaffMember, error)
	// StaffContact returns one staff member with account and e-mail.
	StaffContact(ctx context.Context, staffID int64) (QualificationStaffMember, error)
	// StammdatenManagers returns the holders of staff:stammdaten.
	StammdatenManagers(ctx context.Context) ([]QualificationStaffMember, error)
}

// QualificationNotice is one in-app reminder. Own is the notice to the
// affected person, the rest goes to the Leitung. The push lands on a lock
// screen, so the Leitung's text names no person.
type QualificationNotice struct {
	TenantID            int64
	StaffID             int64
	QualificationID     int64
	Stage               string
	Own                 bool
	Title               string
	Body                string
	DeepLink            string
	IdempotencyKey      string
	RecipientAccountIDs []int64
}

// QualificationNotifier delivers an in-app reminder. sent=false means the
// school's notification window holds it back now; the reminder is repeated
// on a later run.
type QualificationNotifier func(ctx context.Context, notice QualificationNotice) (sent bool, err error)

// QualificationMailContent is what the reminder mail shows.
type QualificationMailContent struct {
	SchoolName        string
	FirstName         string
	StaffName         string
	QualificationName string
	ExpiresOn         string
	ExpiryDay         bool
	// Lapsed: the expiry day is already over (a caught-up reminder).
	Lapsed  bool
	Own     bool
	LinkURL string
}

// QualificationMailMessage is one reminder mail for Delivery's outbox.
type QualificationMailMessage struct {
	TenantID       int64
	ToName         string
	ToAddress      string
	Subject        string
	Template       string
	Content        QualificationMailContent
	Type           string
	ReferenceID    int64
	IdempotencyKey string
}

// QualificationMail is Delivery's durable outbox; Enqueue joins the
// caller's transaction.
type QualificationMail interface {
	Enqueue(context.Context, QualificationMailMessage) error
}

// QualificationReminderPorts are the collaborators of the reminders.
type QualificationReminderPorts struct {
	Staff       QualificationStaff
	Notify      QualificationNotifier
	Mail        QualificationMail
	Schools     PayrollSchools
	FrontendURL string
	Logger      *slog.Logger
}

// QualificationReminders sends the reminders that are due in the school of
// the context: once at the start of the lead, once on the expiry day.
type QualificationReminders interface {
	RunDue(ctx context.Context) error
}
