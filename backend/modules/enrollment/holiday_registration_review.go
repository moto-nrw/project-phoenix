package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Review of the holiday registrations of a Träger (spec #4611 user stories 35
// and 36, #4663, ADR 0114): the Träger-Büro lists the registrations of a
// period with their status, school, week and time model, sees possible
// duplicates, confirms, cancels and rebooks them, and exports the list. After
// the registration deadline a registration is frozen; the Träger-Büro reopens
// it on purpose before it cancels or rebooks it. Every operation runs in the
// Träger transaction of the session's Träger, which the caller opens, and
// writes an entry into the registration's own audit trail: every read of
// personal data and every change.

// States of a reviewed registration. A new registration is
// HolidayRegistrationReceived.
const (
	HolidayRegistrationConfirmed = "confirmed"
	HolidayRegistrationCancelled = "cancelled"
)

// HolidayRegistrationGuestSchool is the school filter for holiday guests:
// children without a school of the Träger.
const HolidayRegistrationGuestSchool = "guest"

// Actions of the audit trail of a holiday registration. The changes form the
// history of a registration; the reads record who saw personal data.
const (
	HolidayAuditList    = "list"
	HolidayAuditRead    = "read"
	HolidayAuditExport  = "export"
	HolidayAuditConfirm = "confirm"
	HolidayAuditCancel  = "cancel"
	HolidayAuditRebook  = "rebook"
	HolidayAuditFreeze  = "freeze"
	HolidayAuditReopen  = "reopen"
)

var (
	// ErrHolidayRegistrationEntryNotFound reports a registration that does
	// not exist in the period of the session's Träger.
	ErrHolidayRegistrationEntryNotFound = errors.New("holiday registration not found")
	// ErrHolidayRegistrationFrozen reports a cancellation or rebooking of a
	// frozen registration; it needs to be reopened first.
	ErrHolidayRegistrationFrozen = errors.New("the holiday registration is frozen")
	// ErrHolidayRegistrationStateConflict reports an action the current state
	// of the registration does not allow, such as confirming a cancelled one.
	ErrHolidayRegistrationStateConflict = errors.New("the holiday registration does not allow this action in its current state")
	// ErrInvalidHolidayRegistrationFilter reports an unknown status or a
	// malformed filter value.
	ErrInvalidHolidayRegistrationFilter = errors.New("invalid holiday registration filter")
)

// HolidayRegistrationFilter narrows the list and the export. Status matches
// the registration; School (a school ID or HolidayRegistrationGuestSchool),
// WeekID and TimeModel (a time model name) match one child and one of its
// bookings together. Empty values match everything.
type HolidayRegistrationFilter struct {
	Status    string
	School    string
	WeekID    int64
	TimeModel string
}

// HolidayRegistrationBooking is the time model a child books in one week.
type HolidayRegistrationBooking struct {
	WeekID        int64         `json:"week_id"`
	WeekStartsOn  calendar.Date `json:"week_starts_on"`
	WeekEndsOn    calendar.Date `json:"week_ends_on"`
	HostSchoolID  int64         `json:"host_school_id"`
	TimeModelID   int64         `json:"time_model_id"`
	TimeModelName string        `json:"time_model_name"`
	// PriceCents is the price the time model has now. The contribution a
	// confirmation freezes is the Elternabrechnung's (#4664).
	PriceCents int64 `json:"price_cents"`
}

// HolidayRegistrationChild is one child of a reviewed registration. A nil
// HomeSchoolID marks a holiday guest. DuplicateRegistrationIDs names the other
// registrations of the period, not cancelled, with a child of the same name
// and date of birth.
type HolidayRegistrationChild struct {
	ID                       int64                        `json:"id"`
	FirstName                string                       `json:"first_name"`
	LastName                 string                       `json:"last_name"`
	BirthDate                calendar.Date                `json:"birth_date"`
	HomeSchoolID             *int64                       `json:"home_school_id"`
	GuestSchoolName          string                       `json:"guest_school_name"`
	Bookings                 []HolidayRegistrationBooking `json:"bookings"`
	DuplicateRegistrationIDs []int64                      `json:"duplicate_registration_ids"`
}

// HolidayRegistrationSummary is a registration in the list. It carries no
// health information and no contacts.
type HolidayRegistrationSummary struct {
	ID          int64     `json:"id"`
	Status      string    `json:"status"`
	Frozen      bool      `json:"frozen"`
	SubmittedAt time.Time `json:"submitted_at"`
	// Guardian is the parent who registered.
	GuardianFirstName string                     `json:"guardian_first_name"`
	GuardianLastName  string                     `json:"guardian_last_name"`
	GuardianEmail     string                     `json:"guardian_email"`
	GuardianPhone     string                     `json:"guardian_phone"`
	Children          []HolidayRegistrationChild `json:"children"`

	// BenefitsDeclared is the parents' statement that they receive
	// benefits; the Träger-Büro decides on the exemption when it confirms
	// (#4664).
	BenefitsDeclared bool `json:"benefits_declared"`
}

// HolidayRegistrationList is the list of a period: the registrations of the
// filter and the counts of the whole period per status.
type HolidayRegistrationList struct {
	Registrations []HolidayRegistrationSummary `json:"registrations"`
	Total         int                          `json:"total"`
	StatusCounts  map[string]int               `json:"status_counts"`
}

// HolidayRegistrationHealth is the health information of one child. Only
// the detail of a registration shows it.
type HolidayRegistrationHealth struct {
	ChildID     int64  `json:"child_id"`
	Allergies   string `json:"allergies"`
	Medication  string `json:"medication"`
	HealthNotes string `json:"health_notes"`
}

// HolidayRegistrationContact is an emergency contact or a person allowed to
// collect the children.
type HolidayRegistrationContact struct {
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Relationship string `json:"relationship"`
}

// HolidayBookingChange is a booking in the history of a rebooking.
type HolidayBookingChange struct {
	WeekID      int64 `json:"week_id"`
	TimeModelID int64 `json:"time_model_id"`
}

// HolidayRegistrationEventDetails name what a change changed: the status
// before a confirmation or cancellation, the child and its bookings before
// and after a rebooking.
type HolidayRegistrationEventDetails struct {
	From    string                 `json:"from,omitempty"`
	ChildID int64                  `json:"child_id,omitempty"`
	Before  []HolidayBookingChange `json:"before,omitempty"`
	After   []HolidayBookingChange `json:"after,omitempty"`
}

// HolidayRegistrationEvent is one change in the history of a registration.
type HolidayRegistrationEvent struct {
	Action         string                          `json:"action"`
	OccurredAt     time.Time                       `json:"occurred_at"`
	ActorAccountID *int64                          `json:"actor_account_id"`
	Details        HolidayRegistrationEventDetails `json:"details"`
}

// HolidayChildAnswers are the answers of one child to the questions of the
// form.
type HolidayChildAnswers struct {
	ChildID    int64          `json:"child_id"`
	CustomData HolidayAnswers `json:"custom_data"`
}

// HolidayRegistrationAnswers are the form version and the answers of one
// registration as the store reads them, the children in their order.
type HolidayRegistrationAnswers struct {
	FormVersion int
	CustomData  HolidayAnswers
	Children    []HolidayChildAnswers
}

// HolidayRegistrationDetail is one registration with everything the parent
// entered, its consents and its history. Form is the version of the
// period's form the registration was submitted with; CustomData and
// ChildCustomData are the answers to its questions, health information
// included (#5198).
type HolidayRegistrationDetail struct {
	HolidayRegistrationSummary
	SecondGuardian     *HolidayGuardianInput        `json:"second_guardian,omitempty"`
	Health             []HolidayRegistrationHealth  `json:"health"`
	Form               *HolidayPeriodForm           `json:"form"`
	CustomData         HolidayAnswers               `json:"custom_data"`
	ChildCustomData    []HolidayChildAnswers        `json:"child_custom_data"`
	Contacts           []HolidayRegistrationContact `json:"contacts"`
	PrivacyConsentAt   time.Time                    `json:"privacy_consent_at"`
	HealthConsentAt    *time.Time                   `json:"health_consent_at"`
	BindingConfirmedAt *time.Time                   `json:"binding_confirmed_at"`
	History            []HolidayRegistrationEvent   `json:"history"`
}

// HolidayRegistrationExportRow is one booking of the export: one row per
// child and week. Answers are the family's and the child's answers to the
// export's Columns, never to a question with health information (#5198).
// AnswerFields keeps their definitions from the submitted form version.
type HolidayRegistrationExportRow struct {
	SecondGuardian *HolidayGuardianInput
	Registration   HolidayRegistrationSummary
	Child          HolidayRegistrationChild
	Booking        HolidayRegistrationBooking
	Answers        HolidayAnswers
	AnswerFields   []HolidayFormField
}

// HolidayRegistrationExport is the export of a period: its rows and one
// column per question without health information of the form versions the
// rows were submitted with, labelled as the newest of them labels it.
type HolidayRegistrationExport struct {
	Period  HolidayPeriod
	Rows    []HolidayRegistrationExportRow
	Columns []HolidayFormField
}

// HolidayReviewer is the person of the Träger-Büro who reviews: the Träger
// and the account of the session.
type HolidayReviewer struct {
	OrganizationID int64
	AccountID      int64
}

// HolidayRegistrationReview lets the Träger-Büro review the registrations of
// one period. Every operation runs in the Träger transaction of the
// reviewer's Träger and records itself in the registration's audit trail;
// a failed audit entry fails the operation.
type HolidayRegistrationReview interface {
	ListHolidayRegistrations(ctx context.Context, reviewer HolidayReviewer, periodID int64, filter HolidayRegistrationFilter) (HolidayRegistrationList, error)
	HolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistrationDetail, error)
	// ExportHolidayRegistrations lists one row per booking of the filter, for
	// the CSV and Excel export. format names the file format in the audit
	// trail.
	ExportHolidayRegistrations(ctx context.Context, reviewer HolidayReviewer, periodID int64, filter HolidayRegistrationFilter, format string) (HolidayRegistrationExport, error)
	// ChangeHolidayRegistration confirms, cancels, freezes or reopens a
	// registration: action is HolidayAuditConfirm, HolidayAuditCancel,
	// HolidayAuditFreeze or HolidayAuditReopen.
	ChangeHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, action string) (HolidayRegistrationDetail, error)
	// RebookHolidayChild replaces the bookings of one child: one time model
	// per week, at least one week.
	RebookHolidayChild(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID, childID int64, bookings []HolidayBookingInput) (HolidayRegistrationDetail, error)
}

// HolidaySchoolNames names the schools of a Träger for the export of the
// registrations, deleted schools excluded. The root binds it to the Träger
// projection, which reads in its own administrative unit; the export route
// asks before it opens the Träger transaction (ADR 0114).
type HolidaySchoolNames interface {
	HolidaySchoolNames(ctx context.Context, organizationID int64) (map[int64]string, error)
}
