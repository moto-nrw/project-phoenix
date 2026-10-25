package enrollment

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Public holiday registration (spec #4611 user stories 26 and 28 to 31,
// #4661, ADR 0110): parents without an account register up to three siblings
// for a published holiday period through its one public link. A child
// without a school of the Träger is a holiday guest. A registration is no
// enrollment request of a school and links no child to a student: typing a
// name and a date of birth unlocks no data. The link is resolved in the
// administrative transaction like the school slug of the online
// registration; everything else runs in the Träger transaction of the
// resolved Träger, which the caller opens.

// HolidayRegistrationReceived is the state of a submitted registration. The
// Träger-Büro reviews it later (#4663).
const HolidayRegistrationReceived = "received"

// Kinds of a holiday registration contact.
const (
	HolidayContactEmergency = "emergency"
	HolidayContactPickup    = "pickup"
)

// Limits of one registration (user story 28: up to three siblings).
const (
	HolidayRegistrationMaxChildren          = 3
	HolidayRegistrationMaxEmergencyContacts = 3
	HolidayRegistrationMaxPickupPersons     = 5
)

var (
	// ErrHolidayRegistrationNotFound reports a link without a published
	// period of an active Träger.
	ErrHolidayRegistrationNotFound = errors.New("holiday registration link not found")
	// ErrHolidayRegistrationClosed reports a registration outside the
	// registration window of the period.
	ErrHolidayRegistrationClosed = errors.New("holiday registration is closed")
	// ErrHolidayWeekBooked reports a change of a period that removes a week or
	// a time model a registration books.
	ErrHolidayWeekBooked = errors.New("a holiday week or time model with registrations cannot be removed")
)

// HolidaySchool is a school of the Träger, by name.
type HolidaySchool struct {
	ID   int64
	Name string
}

// HolidayTraeger is the Träger of a link with its name and its undeleted
// schools, as Organisation & Tenancy reports it.
type HolidayTraeger struct {
	Name    string
	Schools []HolidaySchool
}

// HolidayTraegerDirectory reads the Träger of a link in the administrative
// transaction the link resolution runs in. The root binds it to
// Organisation & Tenancy; an inactive or unknown Träger is
// ErrHolidayRegistrationNotFound.
type HolidayTraegerDirectory interface {
	HolidayTraeger(ctx context.Context, organizationID int64) (HolidayTraeger, error)
}

// HolidayRegistrationLink is what a public link names: the Träger, the
// period and the Träger's schools.
type HolidayRegistrationLink struct {
	OrganizationID int64
	PeriodID       int64
	Traeger        HolidayTraeger
}

// HolidayRegistrationOffer is the published period behind a link. Open
// reports whether today lies in its registration window.
type HolidayRegistrationOffer struct {
	Name                 string
	RegistrationOpensOn  calendar.Date
	RegistrationClosesOn calendar.Date
	Binding              bool
	Notes                string
	Weeks                []HolidayWeek
	Open                 bool
}

// HolidayGuardianInput is the parent who registers.
type HolidayGuardianInput struct {
	FirstName string
	LastName  string
	Email     string
	Phone     string
}

// HolidayBookingInput is the time model a child books in one week.
type HolidayBookingInput struct {
	WeekID      int64
	TimeModelID int64
}

// HolidayChildInput is one child of a registration. A zero HomeSchoolID
// makes the child a holiday guest, who may name the school it attends.
type HolidayChildInput struct {
	FirstName       string
	LastName        string
	BirthDate       calendar.Date
	HomeSchoolID    int64
	GuestSchoolName string
	Allergies       string
	Medication      string
	HealthNotes     string
	Bookings        []HolidayBookingInput
}

// HolidayContactInput is an emergency contact or a person allowed to
// collect the children.
type HolidayContactInput struct {
	Name         string
	Phone        string
	Relationship string
}

// HolidayRegistrationInput is the whole registration as a parent submits it.
// PrivacyConsent is always required, HealthConsent whenever a child carries
// health information and BindingConfirmed when the period is binding.
type HolidayRegistrationInput struct {
	Guardian          HolidayGuardianInput
	Children          []HolidayChildInput
	EmergencyContacts []HolidayContactInput
	PickupPersons     []HolidayContactInput
	PrivacyConsent    bool
	HealthConsent     bool
	BindingConfirmed  bool
}

// HolidayRegistrationReceipt confirms a stored registration. It repeats no
// personal data.
type HolidayRegistrationReceipt struct {
	ID          int64
	SubmittedAt time.Time
}

// HolidayRegistrations serves the public holiday registration.
// ResolveHolidayRegistrationLink runs in the administrative transaction,
// every other operation in the Träger transaction of the link's Träger.
type HolidayRegistrations interface {
	ResolveHolidayRegistrationLink(ctx context.Context, token string) (HolidayRegistrationLink, error)
	HolidayRegistrationOffer(ctx context.Context, link HolidayRegistrationLink) (HolidayRegistrationOffer, error)
	// CountHolidayRegistrationAttempt counts one attempt per IP address and
	// per e-mail address and refuses with ErrRateLimited past the limit.
	CountHolidayRegistrationAttempt(ctx context.Context, organizationID int64, remoteIP, email string) error
	SubmitHolidayRegistration(ctx context.Context, link HolidayRegistrationLink, input HolidayRegistrationInput) (HolidayRegistrationReceipt, error)
}

// HolidayRegistrationMailKind is the Delivery outbox kind of the receipt
// mail; the root registers its renderer under the same kind.
const (
	MailKindHolidayRegistrationReceived = "holiday_registration_received"
	MailRelatedHolidayRegistration      = "holiday_registration"
)

// TraegerMail is a mail of a Träger for Delivery's outbox: no school, the
// Träger of the caller's Träger transaction (ADR 0110).
type TraegerMail struct {
	OrganizationID int64
	Kind           string
	Recipient      string
	Payload        json.RawMessage
	RelatedType    string
	RelatedID      int64
	IdempotencyKey string
}

// TraegerMailOutbox enqueues a mail of a Träger in the caller's Träger
// transaction, so the mail and the write that causes it commit together.
// The root binds it to Delivery.
type TraegerMailOutbox interface {
	EnqueueTraegerMail(ctx context.Context, mail TraegerMail) error
}

// HolidayRegistrationMailChild is one child of the receipt mail with one
// line per booked week.
type HolidayRegistrationMailChild struct {
	Name  string   `json:"name"`
	Guest bool     `json:"guest"`
	Weeks []string `json:"weeks"`
}

// HolidayRegistrationMail is the payload of the receipt mail. It carries no
// health information and no date of birth.
type HolidayRegistrationMail struct {
	Recipient    string                         `json:"recipient_email"`
	GuardianName string                         `json:"guardian_name"`
	TraegerName  string                         `json:"traeger_name"`
	PeriodName   string                         `json:"period_name"`
	Binding      bool                           `json:"binding"`
	Children     []HolidayRegistrationMailChild `json:"children"`
}

// HolidayRegistrationRetentionDays is how long the registrations of a period
// stay after its last week: three months (spec #4611 user story 121). A
// setting of the Träger follows with #4720.
const HolidayRegistrationRetentionDays = 92

// HolidayRegistrationRetention deletes the registrations of every Träger
// whose period ended more than HolidayRegistrationRetentionDays ago, and the
// expired abuse counters. It runs in the administrative transaction of the
// hourly cleanup.
type HolidayRegistrationRetention interface {
	DeleteExpiredHolidayRegistrations(ctx context.Context) (int, error)
}
