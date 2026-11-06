package carecontract

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The ports of the Betreuungsvertrag. The root binds each one; none names
// another owner's types.

// School is a school of the Träger.
type School struct {
	ID   int64
	Name string
}

// TraegerSchools lists the schools of a Träger. It opens its own unit and
// must be called outside any transaction.
type TraegerSchools interface {
	TraegerSchools(ctx context.Context, organizationID int64) ([]School, error)
}

// Booking is an approved booking of an offering by an application child.
type Booking struct {
	CareOfferingID int64
	Days           []string
	ValidFrom      *calendar.Date
	ValidUntil     *calendar.Date
}

// Application is an accepted application child of one school, with its
// guardian and its bookings.
type Application struct {
	RequestChildID    int64
	StudentID         *int64
	ChildFirstName    string
	ChildLastName     string
	ChildBirthDate    *calendar.Date
	GuardianFirstName string
	GuardianLastName  string
	GuardianEmail     string
	GuardianAccountID *int64
	PhaseName         string
	ServiceStartDate  *calendar.Date
	ActivateOn        *calendar.Date
	Bookings          []Booking
}

// Offering is a care offering of one school.
type Offering struct {
	ID         int64
	PhaseID    int64
	Name       string
	PhaseName  string
	PriceCents *int
	Active     bool
}

// SchoolAccess is one read of personal data of a school by the Träger-Büro,
// for the access log of that school (ADR 0084 section 7).
type SchoolAccess struct {
	OrganizationID int64
	AccountID      int64
	View           string
	Rows           int
}

// SchoolApplications reads the accepted applications of the school whose
// tenant transaction the caller holds (Enrollment).
type SchoolApplications interface {
	// AcceptedApplications lists the accepted application children without
	// their bookings; a non-empty requestChildIDs keeps only those.
	AcceptedApplications(ctx context.Context, requestChildIDs []int64) ([]Application, error)
	// NamePhases fills in the name of each offering's enrollment phase.
	NamePhases(ctx context.Context, offerings []Offering) ([]Offering, error)
}

// SchoolBookings reads the bookings and the offerings of the school whose
// tenant transaction the caller holds (Care Plan).
type SchoolBookings interface {
	// Bookings lists the bookings of the application children by child.
	Bookings(ctx context.Context, requestChildIDs []int64) (map[int64][]Booking, error)
	// Offerings lists the school's offerings; a non-empty ids keeps only
	// those.
	Offerings(ctx context.Context, ids []int64) ([]Offering, error)
}

// SchoolAccessLog appends a read to the access log of the school whose
// tenant transaction the caller holds (ADR 0084 section 7).
type SchoolAccessLog interface {
	RecordAccess(ctx context.Context, access SchoolAccess) error
}

// Runtime opens the transactions the intake uses one after the other.
type Runtime interface {
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
}

// LetterTemplate is the Träger's current template of the contract letter.
type LetterTemplate struct {
	Version    int
	Subject    string
	Salutation string
	Body       string
	Closing    string
}

// Letterhead is the master data and branding of the Träger as the contract
// prints it.
type Letterhead struct {
	AccentColor  string
	Logo         []byte
	SenderText   string
	FooterText   string
	ContactName  string
	ContactPhone string
	ContactEmail string
	Template     LetterTemplate
}

// Letterheads reads the letterhead in the Träger transaction of the
// reviewer.
type Letterheads interface {
	Letterhead(ctx context.Context, reviewer Reviewer) (Letterhead, error)
}

// MailState is the state of one mail of the outbox.
type MailState struct {
	OutboxID int64
	State    string
}

// Post enqueues the mail of a contract in the caller's Träger transaction
// and reads the state of its mails.
type Post interface {
	EnqueueContractMail(ctx context.Context, mail ContractMail) (int64, error)
	ContractMailStates(ctx context.Context, organizationID, contractID int64) ([]MailState, error)
}
