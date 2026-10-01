package care

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// OfferingChangeRequests is the part of Care Plan's offering-change review
// the portal drives: a child's booking catalog, the change requests and the
// course requests (#3561). Every call runs in the tenant unit of work the
// caller opened.
type OfferingChangeRequests interface {
	careplan.OfferingChangeRequests
	careplan.CourseRequests
}

// MealPlan is the consumer-owned port over the Meal Plan capability of the
// child's school. Every call runs in the tenant unit of work the caller opened.
// The binding translates the owner's refusals: a switched-off meal plan is
// ErrMealPlanDisabled, switched-off registration ErrMealRegistrationDisabled,
// a passed cutoff ErrMealParticipationCutoff and a refused change
// ErrInvalidMealParticipation. Every other error passes through.
type MealPlan interface {
	Available(context.Context) (bool, error)
	Week(ctx context.Context, monday timezone.Date) ([]MealPlanEntry, error)
	RegistrationAvailable(context.Context) (bool, error)
	Participation(ctx context.Context, studentID int64, from, to timezone.Date) (MealParticipationPlan, error)
	ReplaceParticipationSchedule(context.Context, MealParticipationSchedule) (string, error)
	SetParticipationForDay(context.Context, MealParticipationChange) error
	ClearParticipationForDay(context.Context, MealParticipationChange) error
}

// MealParticipationSchedule replaces a child's standing weekday participation.
type MealParticipationSchedule struct {
	StudentID         int64
	GuardianAccountID int64
	Weekdays          []MealWeekday
}

// MealParticipationChange sets or clears one day's participation.
type MealParticipationChange struct {
	StudentID         int64
	GuardianAccountID int64
	Date              timezone.Date
	Participating     bool
}

// Guardian change trail vocabulary. The values are the Audit Platform's
// stored change types and field names.
const (
	GuardianChangeTypeContact = "contact"
	GuardianChangeTypePickup  = "pickup"

	GuardianFieldCanPickup         = "can_pickup"
	GuardianFieldEmergencyContact  = "is_emergency_contact"
	GuardianFieldFirstName         = "first_name"
	GuardianFieldLastName          = "last_name"
	GuardianFieldEmail             = "email"
	GuardianFieldAddressStreet     = "address_street"
	GuardianFieldAddressCity       = "address_city"
	GuardianFieldAddressPostalCode = "address_postal_code"
	GuardianFieldPhones            = "phones"
)

// Student consent vocabulary of the Audit Platform's consent trail.
const (
	StudentConsentPhoto              = "photo"
	StudentConsentSourceParentPortal = "parent_portal"
)

// GuardianChange is one append-only row of the guardian change trail.
type GuardianChange struct {
	StudentID          int64
	GuardianProfileID  int64
	ActorAccountID     *int64
	ActorNameSnapshot  *string
	ActorEmailSnapshot *string
	ChangeType         string
	FieldName          string
	OldValue           *string
	NewValue           *string
}

// GuardianChangeLog is the consumer-owned port over the Audit Platform's
// guardian change trail. It appends inside the caller's tenant unit of work,
// so the trail commits or rolls back with the change it records.
type GuardianChangeLog interface {
	RecordGuardianChanges(context.Context, []GuardianChange) error
}

func deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// GuardianContactRecord is the contact slice of a guardian profile.
type GuardianContactRecord struct {
	FirstName              string
	LastName               string
	Email                  *string
	AddressStreet          *string
	AddressCity            *string
	AddressPostalCode      *string
	PreferredContactMethod string
	LanguagePreference     string
}

// GuardianPhoneRecord is one phone row of a guardian profile.
type GuardianPhoneRecord struct {
	PhoneNumber string
	PhoneType   string
	Label       *string
	IsPrimary   bool
	Priority    int
}

// GuardianContactLink is a new relationship of a contact to the child.
type GuardianContactLink struct {
	StudentID          int64
	GuardianProfileID  int64
	RelationshipType   string
	GuardianRole       string
	IsPrimary          bool
	IsEmergencyContact bool
	CanPickup          bool
	PickupNotes        *string
	EmergencyPriority  int
	IsPayer            bool
	Permissions        json.RawMessage
}

// GuardianLinkPickupPatch writes only the supplied pickup columns of one
// relationship; SetPickupNotes says the note was supplied at all.
type GuardianLinkPickupPatch struct {
	LinkID             int64
	CanPickup          *bool
	IsEmergencyContact *bool
	SetPickupNotes     bool
	PickupNotes        *string
}

// GuardianRecords is the consumer-owned port over People Directory's guardian
// rows. Every call writes in the caller's tenant unit of work. The binding
// reports a duplicate e-mail as ErrGuardianEmailConflict.
type GuardianRecords interface {
	CreateGuardianContact(context.Context, GuardianContactRecord) (int64, error)
	UpdateGuardianContact(context.Context, int64, GuardianContactRecord) error
	ReplaceGuardianPhones(context.Context, int64, []GuardianPhoneRecord) error
	AddGuardianPhoneRecord(context.Context, int64, GuardianPhoneRecord) (int64, error)
	SetGuardianPhoneNumber(context.Context, int64, string) error
	DeleteGuardianPhoneRecord(context.Context, int64) error
	// LinkGuardianContact inserts the relationship unless the pair is already
	// linked and returns the new relationship ID, zero when nothing was
	// inserted.
	LinkGuardianContact(context.Context, GuardianContactLink) (int64, bool, error)
	PatchGuardianLinkPickup(context.Context, GuardianLinkPickupPatch) (int64, error)
	SetGuardianPortalLocale(ctx context.Context, accountID int64, locale string) (int64, error)
	// ListGuardianProfileTenants returns the schools in which the account has
	// a guardian profile. It is a cross-tenant read for the administrative
	// unit of work.
	ListGuardianProfileTenants(ctx context.Context, accountID int64) ([]int64, error)
}

// StudentLiveAbsence is the child's live absence flags, written as given.
type StudentLiveAbsence struct {
	StudentID    int64
	Sick         *bool
	SickSince    *time.Time
	Excused      *bool
	ExcusedSince *time.Time
}

// StudentPhotoState is the child's photo and its voluntary consent, written
// as given.
type StudentPhotoState struct {
	StudentID           int64
	PhotoPath           *string
	PhotoConsentGivenAt *time.Time
	PhotoConsentGivenBy *int64
}

// StudentRecords is the consumer-owned port over the writes of the child's own
// rows, in the caller's tenant unit of work: the care profile (health
// information, live absence flags) belongs to Care Plan, the photo consent to
// People Directory.
type StudentRecords interface {
	SetStudentHealthInfo(ctx context.Context, studentID int64, healthInfo *string) error
	SetStudentLiveAbsence(context.Context, StudentLiveAbsence) error
	SetStudentPhotoConsent(context.Context, StudentPhotoState) error
}

// StudentDataRequestCommands is the Care Plan command surface for the
// child's Stammdaten change requests.
type StudentDataRequestCommands interface {
	CreateStudentDataRequest(context.Context, careplan.StudentDataChangeRequest) (careplan.StudentDataChangeRequest, error)
	UpdatePendingStudentDataRequest(context.Context, int64, json.RawMessage) error
}
