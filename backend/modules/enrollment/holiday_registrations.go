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
	// HolidayWishRule says that the period takes the holiday care wish into
	// account (#4703); FirstDay is the first day of its first week, the day
	// whose school year the wish counts for. Zero without weeks.
	HolidayWishRule bool
	FirstDay        calendar.Date
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
	// MaxDaysPerChild bounds the care days per child, nil for no bound
	// (#4701).
	MaxDaysPerChild *int
	// AllocationMode is how the period gives its places (#4700); Criteria
	// are the questions of its active criteria, empty by arrival.
	AllocationMode string
	Criteria       []HolidayCriterionQuestion
	// FullDays are the days of each week by days without a free place, by
	// week ID (#4701); a booking of such a day waits.
	FullDays map[int64][]calendar.Date
	Open     bool
	// Form is the current version of the period's form (#5198).
	Form HolidayPeriodForm
	// ConsentTexts are the German texts of the fixed declarations the form
	// shows, in HolidayConsentVersionCurrent (#5626) with the retention of
	// the Träger (#6691).
	ConsentTexts HolidayConsentTexts
	// HolidayWishMode is HolidayWishModeOff, or the Träger's
	// holiday_registration.wish_mode when the period takes the holiday care
	// wish into account (#4703).
	HolidayWishMode string
}

// HolidayGuardianInput holds the registering or optional second guardian’s contact details.
type HolidayGuardianInput struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
	Phone     string `json:"phone"`
}

// HolidayAddress is the postal address of the parent who registers
// (#5625): all three parts or none. The form of the period decides whether
// it is asked.
type HolidayAddress struct {
	Street     string `json:"street"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
}

// HolidayBookingInput is the time model a child books in one week. Days
// name the booked days of a week by days and are empty for a whole week
// (#4701).
type HolidayBookingInput struct {
	WeekID      int64
	TimeModelID int64
	Days        []calendar.Date
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
	// CustomData are the answers to the questions of the form per child
	// (#5198), by field key.
	CustomData HolidayAnswers
	// WishProof is the sealed statement of the child's holiday care wish
	// the prefill gave a parent with an account (#4703); empty otherwise.
	WishProof string
	// HolidayWish and HolidayWishSource are what the registration knows of
	// the wish. Only the registration sets them, from a WishProof it
	// opened; whatever a caller puts in is dropped.
	HolidayWish       string
	HolidayWishSource string
	// Criteria are the answers to the child criteria of a period by
	// criteria, by criterion ID (#4700).
	Criteria map[int64]bool
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
// BenefitsDeclared states that the family receives benefits (#4664); the
// Träger-Büro checks it before it applies an exemption. BenefitsProofTokens
// name the uploaded proofs of the benefits (#5238), which the registration
// claims; they need BenefitsDeclared. FormVersion names
// the version of the period's form the parent filled in (0 when the client
// does not say); CustomData are the answers per family (#5198).
// CustodyConfirmed and DeclarationName are required for every registration:
// the parent confirms the custody and gives the declarations with the typed
// name (#5626).
// GuardianAddress is the postal address of the parent who registers, when
// the form asks for it (#5625).
type HolidayRegistrationInput struct {
	Guardian          HolidayGuardianInput
	GuardianAddress   *HolidayAddress
	SecondGuardian    *HolidayGuardianInput
	Children          []HolidayChildInput
	EmergencyContacts []HolidayContactInput
	PickupPersons     []HolidayContactInput
	PrivacyConsent    bool
	HealthConsent     bool
	BindingConfirmed  bool
	CustodyConfirmed  bool
	DeclarationName   string
	BenefitsDeclared  bool
	// BenefitsProofTokens are the single-use tokens of the uploaded proofs.
	BenefitsProofTokens []string
	FormVersion         int
	CustomData          HolidayAnswers
	// Criteria are the answers to the family criteria of a period by
	// criteria, by criterion ID (#4700).
	Criteria         map[int64]bool
	LegalAcceptances map[string]bool
	// ConsentVersion is the version of the consent texts the parent agreed
	// to (#4722). A new registration names HolidayConsentVersionCurrent
	// (#5626); from HolidayConsentVersionHostHealth on the hosting OGS sees
	// the health information.
	ConsentVersion int
	// RetentionDays are the days the privacy text the parent agreed to
	// names (#6691): the retention of the Träger at the submission. The
	// registration keeps them, whatever the Träger sets later.
	RetentionDays int
	// RemoteIP is the client address the stored registration counts under
	// (#6726); it is not stored with the registration.
	RemoteIP string
}

// HolidayRegistrationReceipt confirms a stored registration. It repeats no
// personal data.
type HolidayRegistrationReceipt struct {
	ID          int64
	SubmittedAt time.Time
	// Placements say for every booking whether the child got a place or
	// waits for one (#4699). They name the child by its position in the
	// form, not by its name.
	Placements []HolidayReceiptPlacement
}

// HolidayRegistrations serves the public holiday registration.
// ResolveHolidayRegistrationLink runs in the administrative transaction,
// every other operation in the Träger transaction of the link's Träger.
type HolidayRegistrations interface {
	PublicHolidayFormDocument(context.Context, HolidayRegistrationLink, string) (HolidayFormDocument, error)
	ResolveHolidayRegistrationLink(ctx context.Context, token string) (HolidayRegistrationLink, error)
	HolidayRegistrationOffer(ctx context.Context, link HolidayRegistrationLink) (HolidayRegistrationOffer, error)
	// SubmitHolidayRegistration stores the registration and counts it per IP
	// address and per e-mail address in the same transaction. Past a limit
	// it refuses with ErrRateLimited or ErrEmailRateLimited and stores
	// nothing (#6726).
	SubmitHolidayRegistration(ctx context.Context, link HolidayRegistrationLink, input HolidayRegistrationInput) (HolidayRegistrationReceipt, error)
	// CountHolidayProofUpload counts one upload per IP address and refuses
	// with ErrUploadRateLimited past the limit (#5238).
	CountHolidayProofUpload(ctx context.Context, organizationID int64, remoteIP string) error
	// UploadHolidayProof stores an unclaimed proof of benefits for the
	// period of the link while its registration window is open, and returns
	// the token the submission claims it with (#5238).
	UploadHolidayProof(ctx context.Context, link HolidayRegistrationLink, upload HolidayProofUpload) (HolidayProofReceipt, error)
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
	// WaitlistWeeks are the weeks the child waits for a place in (#4699);
	// Weeks then are the weeks with a place. A mail enqueued before has none.
	WaitlistWeeks []string `json:"waitlist_weeks,omitempty"`
	// PendingWeeks are the weeks the Büro still decides on (#4700).
	PendingWeeks []string `json:"pending_weeks,omitempty"`
}

// HolidayRegistrationMail is the payload of the receipt mail. It carries no
// health information and no date of birth. Declaration is the parent's copy
// of the declarations (#5626); a mail enqueued before has none.
type HolidayRegistrationMail struct {
	Recipient    string                              `json:"recipient_email"`
	GuardianName string                              `json:"guardian_name"`
	TraegerName  string                              `json:"traeger_name"`
	PeriodName   string                              `json:"period_name"`
	Binding      bool                                `json:"binding"`
	Children     []HolidayRegistrationMailChild      `json:"children"`
	Declaration  *HolidayRegistrationMailDeclaration `json:"declaration,omitempty"`
}

// HolidayRegistrationDeletionLogRetentionYears is how long the deletion log
// of the holiday registrations keeps an entry, like the deletion log of a
// school (ADR 0118).
const HolidayRegistrationDeletionLogRetentionYears = 3

// HolidayRegistrationReference names a registration in its Träger.
type HolidayRegistrationReference struct {
	OrganizationID int64
	RegistrationID int64
}

// HolidayRegistrationMailErasure removes copies of deleted registrations in
// the caller's administrative transaction. The root binds Delivery's command.
type HolidayRegistrationMailErasure interface {
	EraseHolidayRegistrationMails(context.Context, []HolidayRegistrationReference) (int, error)
}

// HolidayRegistrationRetention deletes the registrations of every Träger
// whose period ended more days ago than the registration keeps (#6691), and
// the health information of the registrations whose period ended more than
// HolidayHealthRetentionDays ago. It logs each period in the deletion log of
// its Träger, and deletes the expired abuse counters and log entries. It runs in the administrative
// transaction of the hourly cleanup.
type HolidayRegistrationRetention interface {
	DeleteExpiredHolidayRegistrations(ctx context.Context) (int, error)
}

// HolidayRegistrationAnnouncement is a published holiday period a parent
// can still register for or will soon be able to (#6671): what the parents
// app shows before the form is opened. FirstDay and LastDay span the weeks
// of the period and are zero without weeks.
type HolidayRegistrationAnnouncement struct {
	OrganizationID       int64
	PeriodID             int64
	Token                string
	Name                 string
	RegistrationOpensOn  calendar.Date
	RegistrationClosesOn calendar.Date
	FirstDay             calendar.Date
	LastDay              calendar.Date
}

// HolidayRegistrationAnnouncements lists the published periods of the
// given Träger whose registration has not closed before on, by the end of
// the registration. Like the link lookup it reads across Träger and
// therefore runs only in the administrative transaction; it returns nothing
// of a draft and no registration.
type HolidayRegistrationAnnouncements interface {
	AnnouncedHolidayRegistrations(ctx context.Context, organizationIDs []int64, on calendar.Date) ([]HolidayRegistrationAnnouncement, error)
}
