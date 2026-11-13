package enrollment

import (
	"context"
	"errors"
	"time"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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
	// HolidayAuditProof records a proof of benefits the Träger-Büro opened
	// (#5238).
	HolidayAuditProof = "proof"
	// HolidayAuditHealth records a read of the health information of a
	// registration (#4722).
	HolidayAuditHealth = "health"
	// HolidayAuditDebtorNumber records a change of the debtor number of a
	// confirmed registration (#5621).
	HolidayAuditDebtorNumber = "debtor_number"
	// HolidayAuditProofUpload records a proof of benefits the Träger-Büro
	// uploaded for the family (#5627).
	HolidayAuditProofUpload = "proof_upload"
	// HolidayAuditEdit records a change of the details the family entered
	// (#5629). It names the changed fields, never their values.
	HolidayAuditEdit = "edit"
	// HolidayAuditSiblingMerge and HolidayAuditSiblingSplit record that the
	// Elternabrechnung merged the registration with others as siblings or
	// took it out of its sibling group (#5628).
	HolidayAuditSiblingMerge = "sibling_merge"
	HolidayAuditSiblingSplit = "sibling_split"
	// HolidayAuditSiblingCancel records that a cancellation took a
	// registration of the sibling group out of it and the Elternabrechnung
	// computed the rest of the group again (#5662), in the trail of the
	// cancelled registration and of every other one of the group.
	HolidayAuditSiblingCancel = "sibling_cancel"
)

// HolidayDebtorNumberMaxLength is the longest debtor number a registration
// keeps (#5621): the reference field of a SEPA transfer has 35 characters.
const HolidayDebtorNumberMaxLength = 35

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
	// Wish is HolidayWishFilterCheck, HolidayWishFilterYes or
	// HolidayWishFilterNo: registrations with such a child (#4703).
	Wish string
}

// HolidayBookingReplacement identifies a week replaced by a rebooking.
// The review reconstructs these ranges from its durable audit trail.
type HolidayBookingReplacement struct {
	WeekID       int64
	WeekStartsOn calendar.Date
	WeekEndsOn   calendar.Date
}

// HolidayRegistrationBooking is the time model a child books in one week.
type HolidayRegistrationBooking struct {
	WeekID        int64         `json:"week_id"`
	WeekStartsOn  calendar.Date `json:"week_starts_on"`
	WeekEndsOn    calendar.Date `json:"week_ends_on"`
	HostSchoolID  int64         `json:"host_school_id"`
	TimeModelID   int64         `json:"time_model_id"`
	TimeModelName string        `json:"time_model_name"`
	// PriceCents is the price the booking has now: the price of the time
	// model, times the booked days in a week by days (#4701). The
	// contribution a confirmation freezes is the Elternabrechnung's (#4664).
	PriceCents int64 `json:"price_cents"`
	// Days are the booked days of a week by days, empty for a whole week
	// (#4701).
	Days []calendar.Date `json:"days,omitempty"`
	// Placement is HolidayPlacementPlaced, HolidayPlacementWaitlisted
	// (#4699) or HolidayPlacementPending (#4700); WaitlistPosition is the place in the waiting list of the
	// week, from 1, for a waitlisted booking of a registration that counts.
	Placement        string     `json:"placement"`
	WaitlistPosition *int       `json:"waitlist_position,omitempty"`
	PromotedAt       *time.Time `json:"promoted_at,omitempty"`
	// ReplacedWeeks is an internal projection for contribution calculation.
	ReplacedWeeks []HolidayBookingReplacement `json:"-"`
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
	// HolidayWish and HolidayWishSource are what is known of the child's
	// holiday care wish (#4703): yes or no from the prefill of a parent with
	// an account or from the office, empty when unknown. HolidayWishCheck
	// marks a child of a school of the Träger whose wish the office still
	// has to check in a period that takes the wish into account. Only the
	// review of the Träger-Büro shows them.
	HolidayWish       string `json:"holiday_wish"`
	HolidayWishSource string `json:"holiday_wish_source"`
	HolidayWishCheck  bool   `json:"holiday_wish_check"`
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

	// DebtorNumber is the number of the family in the finance system of the
	// Träger, entered when the Träger-Büro confirms; empty without one
	// (#5621). The letter names it as the reference to pay with.
	DebtorNumber string `json:"debtor_number"`

	// RetentionDays are the days after the last week of the period the
	// registration stays, as its privacy text says (#6691). DeletesOn is the
	// day the retention deletes it, HealthDeletesOn the day its health
	// information goes; both are nil for a period without weeks.
	RetentionDays   int            `json:"retention_days"`
	DeletesOn       *calendar.Date `json:"deletes_on"`
	HealthDeletesOn *calendar.Date `json:"health_deletes_on"`

	// MailProblems are the mails of the registration that did not reach the
	// parent who registered (#6737, #6976): receipt, acceptance, place,
	// closing day notice and parent account offer; empty while they are on
	// their way or arrived. Only the detail names them; the list reads them
	// through HolidayRegistrationMailReview.
	MailProblems []HolidayRegistrationMailProblem `json:"mail_problems,omitempty"`
}

// What became of a mail of the registration that did not reach the parents
// (#6737): moto gave it up or the mail server returned it, the address is
// blocked so the mail stayed back, or it is unknown whether it went out
// because sending stopped midway or the server never reported a result.
// A mail that arrived can still be one (#6976): the parents reported it as
// unwanted or unsubscribed from the mails.
const (
	HolidayMailFailed       = "failed"
	HolidayMailBlocked      = "blocked"
	HolidayMailUnknown      = "unknown"
	HolidayMailComplained   = "complained"
	HolidayMailUnsubscribed = "unsubscribed"
)

// HolidayRegistrationMailProblem is a mail of the registration that did not
// reach the parents: Kind is one of the holiday MailKind values, State one
// of the HolidayMail values.
type HolidayRegistrationMailProblem struct {
	Kind  string `json:"kind"`
	State string `json:"state"`
}

// HolidayRegistrationMails names the mails of one registration that did not
// reach the parents.
type HolidayRegistrationMails struct {
	RegistrationID int64                            `json:"registration_id"`
	MailProblems   []HolidayRegistrationMailProblem `json:"mail_problems"`
}

// HolidayRegistrationMailReview lists, for the list of a period, the
// registrations with a mail that did not reach the parents (#6737, #6976), in the Träger transaction of the reviewer's Träger. A period of
// another Träger is ErrHolidayPeriodNotFound. It reads no personal data.
type HolidayRegistrationMailReview interface {
	HolidayRegistrationMailProblems(ctx context.Context, reviewer HolidayReviewer, periodID int64) ([]HolidayRegistrationMails, error)
}

// HolidayMailStates reads, in the caller's Träger transaction, what became of
// the Träger's mails to the parents of the registrations and of the mails
// with the idempotency keys, in one read (#6737, #6976): every mail, oldest
// first. The root binds it to Delivery.
type HolidayMailStates interface {
	HolidayMails(ctx context.Context, organizationID int64, registrationIDs []int64, idempotencyKeys []string) ([]HolidayMailState, error)
}

// HolidayMailState is what became of one mail of the Träger: Kind is its
// MailKind, RegistrationID the registration it was enqueued for (0 without
// one), Outcome the delivery outcome in Delivery's words, CreatedAt when it
// was enqueued and Problem a
// HolidayMail value for a mail that did not reach the parents, empty
// otherwise.
type HolidayMailState struct {
	Kind           string
	RegistrationID int64
	IdempotencyKey string
	Outcome        string
	Problem        string
	CreatedAt      time.Time
}

// HolidayRegistrationList is the list of a period: the registrations of the
// filter and the counts of the whole period per status.
type HolidayRegistrationList struct {
	Registrations []HolidayRegistrationSummary `json:"registrations"`
	Total         int                          `json:"total"`
	StatusCounts  map[string]int               `json:"status_counts"`
}

// HolidayRegistrationHealth is the health information of one child: the
// fixed health fields and the answers to the child questions marked as
// health information. Only the health read of a registration shows it
// (#4722).
type HolidayRegistrationHealth struct {
	ChildID     int64          `json:"child_id"`
	Allergies   string         `json:"allergies"`
	Medication  string         `json:"medication"`
	HealthNotes string         `json:"health_notes"`
	CustomData  HolidayAnswers `json:"custom_data"`
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
	// Waitlisted marks a booking the rebooking replaced while it waited for
	// a place (#4699); the host school's care windows skip it.
	Waitlisted bool `json:"waitlisted,omitempty"`
	// PromotedAt preserves the earliest care day when rebooking removes the booking.
	PromotedAt *time.Time `json:"promoted_at,omitempty"`
	// Days are the booked days of a week by days (#4701); the host school's
	// care windows of the trail keep to them.
	Days []calendar.Date `json:"days,omitempty"`
}

// HolidayDebtorNumberChange is the debtor number before and after a
// confirmation or a change of it (#5621); empty means none.
type HolidayDebtorNumberChange struct {
	Before string `json:"before"`
	After  string `json:"after"`
}

// HolidayRegistrationEventDetails name what a change changed: the status
// before a confirmation or cancellation, the child and its bookings before
// and after a rebooking, the debtor number a confirmation entered or a
// change changed, the changed fields of an edit as field paths such as
// "guardian.phone" or "children.0.last_name" (#5629), the other
// registrations of a sibling merge or split (#5628), a decision on a place
// and a check of an answer by criteria (#4700).
type HolidayRegistrationEventDetails struct {
	From         string                     `json:"from,omitempty"`
	ChildID      int64                      `json:"child_id,omitempty"`
	Before       []HolidayBookingChange     `json:"before,omitempty"`
	After        []HolidayBookingChange     `json:"after,omitempty"`
	DebtorNumber *HolidayDebtorNumberChange `json:"debtor_number,omitempty"`
	Fields       []string                   `json:"fields,omitempty"`
	Siblings     []int64                    `json:"siblings,omitempty"`
	// WeekIDs, Placement, Proposal and Reason name a decision of the Büro by
	// criteria (#4700), Checks its checks of the parents' answers.
	WeekIDs   []int64                 `json:"week_ids,omitempty"`
	Placement string                  `json:"placement,omitempty"`
	Proposal  string                  `json:"proposal,omitempty"`
	Reason    string                  `json:"reason,omitempty"`
	Checks    []HolidayCriterionCheck `json:"checks,omitempty"`
	// Wish is the office's decision on a child's holiday care wish, From the
	// wish before, empty when unknown (#4703).
	Wish string `json:"wish,omitempty"`
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
	LegalAcceptances map[string]time.Time
	FormVersion      int
	CustomData       HolidayAnswers
	Children         []HolidayChildAnswers
}

// HolidayRegistrationDetail is one registration with everything the parent
// entered except the health information, its consents and its history. Form
// is the version of the period's form the registration was submitted with;
// CustomData and ChildCustomData are the answers to its questions without
// health information (#5198); the health read returns those (#4722).
// BenefitsProofs are the proofs of benefits the parents attached (#5238);
// only the detail names them.
type HolidayRegistrationDetail struct {
	HolidayRegistrationSummary
	// GuardianAddress is the postal address of the parent who registered,
	// when the form asked for it (#5625). Only the detail and the export
	// name it, not the list.
	GuardianAddress  *HolidayAddress              `json:"guardian_address"`
	LegalAcceptances map[string]time.Time         `json:"legal_acceptances"`
	SecondGuardian   *HolidayGuardianInput        `json:"second_guardian,omitempty"`
	Form             *HolidayPeriodForm           `json:"form"`
	CustomData       HolidayAnswers               `json:"custom_data"`
	ChildCustomData  []HolidayChildAnswers        `json:"child_custom_data"`
	Contacts         []HolidayRegistrationContact `json:"contacts"`
	PrivacyConsentAt time.Time                    `json:"privacy_consent_at"`
	HealthConsentAt  *time.Time                   `json:"health_consent_at"`
	// ConsentVersion is the version of the consent texts the parents agreed
	// to; 0 for a registration before #4722, which shares no health
	// information with the hosting school. ConsentTexts are the texts of
	// that version, nil when they are not kept.
	ConsentVersion     int                  `json:"consent_version"`
	ConsentTexts       *HolidayConsentTexts `json:"consent_texts"`
	BindingConfirmedAt *time.Time           `json:"binding_confirmed_at"`
	// CustodyConfirmedAt and DeclarationName are the custody and the typed
	// name of the declaration (#5626); a registration before has neither.
	CustodyConfirmedAt *time.Time                 `json:"custody_confirmed_at"`
	DeclarationName    string                     `json:"declaration_name"`
	BenefitsProofs     []HolidayRegistrationProof `json:"benefits_proofs"`
	// BenefitsProofAttached reports a proof without reading its list; the
	// recipients of the Elternabrechnung carry it (#5627), no answer does.
	BenefitsProofAttached bool                       `json:"-"`
	History               []HolidayRegistrationEvent `json:"history"`
	// Criteria are the answers to the criteria of a period by criteria
	// (#4700), empty for a period by arrival. Only the detail names them.
	Criteria []HolidayRegistrationCriterion `json:"criteria"`
	// Mails are the mails of the registration to the parents with their
	// delivery outcome, oldest first (#6976); only the detail names them.
	// A Träger keeps no address list, so no entry carries a mark or a stop.
	Mails []RequestMailOutcome `json:"mails,omitempty"`
	// Promotions are the bookings of other registrations that moved up from
	// the waiting list because of the change that answers with this detail:
	// a cancellation or a rebooking (#4699). A read has none.
	Promotions []HolidayPromotion `json:"promotions,omitempty"`
}

// HolidayRegistrationExportRow is one booking of the export: one row per
// child and week. Answers are the family's and the child's answers to the
// export's Columns, never to a question with health information (#5198).
// AnswerFields keeps their definitions from the submitted form version.
type HolidayRegistrationExportRow struct {
	GuardianAddress *HolidayAddress
	SecondGuardian  *HolidayGuardianInput
	Registration    HolidayRegistrationSummary
	Child           HolidayRegistrationChild
	Booking         HolidayRegistrationBooking
	Answers         HolidayAnswers
	AnswerFields    []HolidayFormField
}

// HolidayRegistrationRecipient is a registration with the postal address of
// the parent who registered, the recipient of its invoice (#4668, #5625).
type HolidayRegistrationRecipient struct {
	HolidayRegistrationSummary
	GuardianAddress *HolidayAddress
	// HasBenefitsProof reports a proof of benefits of the registration, for
	// the list a cost bearer gets (#5627). Only this read names it.
	HasBenefitsProof bool
}

// HolidayRegistrationExport is the export of a period: its rows and one
// column per question without health information of the form versions the
// rows were submitted with, labelled as the newest of them labels it.
type HolidayRegistrationExport struct {
	Period  HolidayPeriod
	Rows    []HolidayRegistrationExportRow
	Columns []HolidayFormField
}

// HolidayRegistrationEdit is a change of the details a family entered, made
// by the Träger-Büro (#5629). A nil section, and a child the edit does not
// name, stays as it is. Children are neither added nor removed, bookings,
// benefits, consents and health information do not change here.
type HolidayRegistrationEdit struct {
	// Guardians replaces the registering parent and the second guardian.
	Guardians *HolidayGuardiansEdit
	// Children replaces the details and the answers of the named children.
	Children []HolidayChildEdit
	// Contacts replaces the emergency contacts and the pickup persons.
	Contacts *HolidayContactsEdit
	// CustomData replaces the family's answers to the questions of the form
	// without health information.
	CustomData *HolidayAnswers
}

// HolidayGuardiansEdit are the parent who registered with the postal
// address and the optional second guardian; a nil SecondGuardian or
// GuardianAddress removes it. The address follows the form version of the
// registration (#5625); an issued invoice keeps the address it was issued
// with.
type HolidayGuardiansEdit struct {
	Guardian        HolidayGuardianInput
	GuardianAddress *HolidayAddress
	SecondGuardian  *HolidayGuardianInput
}

// HolidayChildEdit are the details of one child of the registration. A zero
// HomeSchoolID makes the child a holiday guest. CustomData are its answers to
// the questions of the form without health information.
type HolidayChildEdit struct {
	ChildID         int64
	FirstName       string
	LastName        string
	BirthDate       calendar.Date
	HomeSchoolID    int64
	GuestSchoolName string
	CustomData      HolidayAnswers
}

// HolidayContactsEdit are the emergency contacts and the persons allowed to
// collect the children.
type HolidayContactsEdit struct {
	EmergencyContacts []HolidayContactInput
	PickupPersons     []HolidayContactInput
}

// HolidayReviewer is the person of the Träger-Büro who reviews: the Träger
// and the account of the session.
type HolidayReviewer struct {
	OrganizationID int64
	AccountID      int64
}

// HolidaySiblingTrail is what the review offers the Elternabrechnung for
// its sibling groups (#5628): it holds the period while a group is read and
// decided on, and records a merge or split in each registration's trail.
type HolidaySiblingTrail interface {
	// HoldHolidayPeriod locks the period of the session's Träger until the
	// transaction ends, like every change of one of its registrations, so
	// none changes while the Elternabrechnung reads and decides on several
	// of them (#5628). It reads no personal data and records nothing.
	HoldHolidayPeriod(ctx context.Context, reviewer HolidayReviewer, periodID int64) error
	// RecordHolidaySiblings records a sibling merge, split or cancellation
	// of the Elternabrechnung (#5628, #5662): action is
	// HolidayAuditSiblingMerge, HolidayAuditSiblingSplit or
	// HolidayAuditSiblingCancel, siblingIDs the other registrations it
	// concerns. Any state of the registration takes it.
	RecordHolidaySiblings(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, action string, siblingIDs []int64) error
}

// HolidayRegistrationReview lets the Träger-Büro review the registrations of
// one period. Every operation runs in the Träger transaction of the
// reviewer's Träger and records itself in the registration's audit trail;
// a failed audit entry fails the operation.
type HolidayRegistrationReview interface {
	HolidaySiblingTrail
	ListHolidayRegistrations(ctx context.Context, reviewer HolidayReviewer, periodID int64, filter HolidayRegistrationFilter) (HolidayRegistrationList, error)
	// ListHolidayRegistrationRecipients lists the registrations of the
	// filter with the postal address of the parent who registered, for the
	// invoices of the Elternabrechnung (#5625). It records the read like the
	// list.
	ListHolidayRegistrationRecipients(ctx context.Context, reviewer HolidayReviewer, periodID int64, filter HolidayRegistrationFilter) ([]HolidayRegistrationRecipient, error)
	HolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistrationDetail, error)
	// HolidayRegistrationHealth opens the health information of one
	// registration and records the read (#4722). The route lets only the
	// right traeger_health:read call it.
	HolidayRegistrationHealth(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64) (HolidayRegistrationHealthDetail, error)
	// ExportHolidayRegistrations lists one row per booking of the filter, for
	// the CSV and Excel export. format names the file format in the audit
	// trail.
	ExportHolidayRegistrations(ctx context.Context, reviewer HolidayReviewer, periodID int64, filter HolidayRegistrationFilter, format string) (HolidayRegistrationExport, error)
	// ChangeHolidayRegistration cancels, freezes or reopens a registration:
	// action is HolidayAuditCancel, HolidayAuditFreeze or HolidayAuditReopen.
	ChangeHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, action string) (HolidayRegistrationDetail, error)
	// ConfirmHolidayRegistration confirms a received registration, frozen or
	// not, with the debtor number of the family when debtorNumber is not
	// blank (#5621). A debtor number longer than
	// HolidayDebtorNumberMaxLength or with a line break is a validation
	// error on debtor_number; whether one is required is the caller's rule.
	ConfirmHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, debtorNumber string) (HolidayRegistrationDetail, error)
	// SetHolidayDebtorNumber changes the debtor number of a confirmed
	// registration; a blank one removes it (#5621). Whether the number may
	// still change is the caller's rule.
	SetHolidayDebtorNumber(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, debtorNumber string) (HolidayRegistrationDetail, error)
	// RebookHolidayChild replaces the bookings of one child: one time model
	// per week, at least one week.
	RebookHolidayChild(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID, childID int64, bookings []HolidayBookingInput) (HolidayRegistrationDetail, error)
	// EditHolidayRegistration changes the details a family entered (#5629)
	// with the checks of the public form; the answers are checked against the
	// form version of the registration, and schools are the Träger's
	// undeleted schools. A cancelled registration refuses with
	// ErrHolidayRegistrationStateConflict; an edit without a change writes
	// nothing.
	EditHolidayRegistration(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, edit HolidayRegistrationEdit, schools []HolidaySchool) (HolidayRegistrationDetail, error)
	// OpenHolidayRegistrationProof returns one proof of benefits of a
	// registration with its bytes (#5238).
	OpenHolidayRegistrationProof(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID, proofID int64) (HolidayProofFile, error)
	// UploadHolidayRegistrationProof stores a proof of benefits the family
	// brought on paper as a proof of the registration (#5627), with the
	// limits of HolidayProofOptions; a file outside them is an
	// UploadRejection, one more file than the limit ErrHolidayProofLimit, a
	// cancelled registration ErrHolidayRegistrationStateConflict.
	UploadHolidayRegistrationProof(ctx context.Context, reviewer HolidayReviewer, periodID, registrationID int64, upload HolidayProofUpload) (HolidayRegistrationDetail, error)
}

// HolidaySchoolNames names the schools of a Träger for the export of the
// registrations, deleted schools excluded. The root binds it to the Träger
// projection, which reads in its own administrative unit; the export route
// asks before it opens the Träger transaction (ADR 0114).
type HolidaySchoolNames interface {
	HolidaySchoolNames(ctx context.Context, organizationID int64) (map[int64]string, error)
}
