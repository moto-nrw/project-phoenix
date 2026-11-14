package workforce

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// The Personalstamm of ADR 0089 (#4705): one record per person the Träger
// employs, over all its schools. The record keeps the Träger's personnel
// number and the bank and tax values; the schools keep their persons,
// employments and everything of the day. A record exists only for a Träger
// the moto team released for „Personal beim Träger“.

// The two statements a Träger confirms before the release (ADR 0089
// section 2, decided on 13.11.2026 in #4705): an agreement with the works
// council or the church employee representation on time recording and
// planning across sites, or no such representation.
const (
	TraegerStaffRepresentationAgreement = "agreement"
	TraegerStaffRepresentationNone      = "none"
)

// The fields of a record. A link chooses each of them from one source.
const (
	TraegerStaffFieldFirstName            = "first_name"
	TraegerStaffFieldLastName             = "last_name"
	TraegerStaffFieldBirthday             = "birthday"
	TraegerStaffFieldPersonnelNumber      = "personnel_number"
	TraegerStaffFieldIBAN                 = "iban"
	TraegerStaffFieldTaxID                = "tax_id"
	TraegerStaffFieldSocialSecurityNumber = "social_security_number"
)

// TraegerStaffSourceRecord names the record's own value as the source of a
// field when employments join an existing record. Every other source is the
// decimal ID of the school whose employment brings the value.
const TraegerStaffSourceRecord = "record"

// Why moto suggests employments for one record (ADR 0089 section 5), in
// the order it tries them. A single employment without a match is offered
// on its own.
const (
	TraegerStaffMatchAccount         = "same_account"
	TraegerStaffMatchPersonnelNumber = "same_personnel_number"
	TraegerStaffMatchNameBirthday    = "same_name_birthday"
	TraegerStaffMatchNone            = "single"
)

// The kinds of a trail entry.
const (
	TraegerStaffChangeLinked    = "linked"
	TraegerStaffChangeNotChosen = "not_chosen"
	TraegerStaffChangeChanged   = "changed"
)

// The access log views of the Träger-Portal on staff data (ADR 0084
// section 7, ADR 0089 section 6).
const (
	TraegerStaffViewOverview        = "staff_overview"
	TraegerStaffViewRecord          = "staff_record"
	TraegerStaffViewLinkPreview     = "staff_link_preview"
	TraegerStaffViewFinancial       = "staff_financial_view"
	TraegerStaffViewFinancialReveal = "staff_financial_reveal"
)

var (
	// ErrTraegerStaffNotReleased refuses a Personalstamm of a Träger the moto
	// team has not released for „Personal beim Träger“.
	ErrTraegerStaffNotReleased = errors.New("workforce: personal beim träger is not released")
	// ErrTraegerStaffReleaseInvalid refuses a release without one of the two
	// statements.
	ErrTraegerStaffReleaseInvalid = errors.New("workforce: the release needs one of the two statements")
	// ErrTraegerStaffReleaseExists refuses a second release.
	ErrTraegerStaffReleaseExists = errors.New("workforce: the Träger is already released")
	// ErrTraegerStaffReleaseInUse refuses a withdrawal while the Träger keeps
	// records; the Personalverwaltung dissolves them first.
	ErrTraegerStaffReleaseInUse = errors.New("workforce: the Träger still keeps staff records")
	// ErrTraegerStaffReleaseNotFound answers a withdrawal without a release.
	ErrTraegerStaffReleaseNotFound = errors.New("workforce: the Träger is not released")
	// ErrTraegerStaffRecordNotFound answers an unknown or foreign record.
	ErrTraegerStaffRecordNotFound = errors.New("workforce: staff record not found")
	// ErrTraegerStaffLinkInvalid refuses a link; TraegerStaffLinkError names
	// the field.
	ErrTraegerStaffLinkInvalid = errors.New("workforce: the link is invalid")
	// ErrTraegerStaffAlreadyLinked refuses an employment that already
	// belongs to a record, or a second employment of one school.
	ErrTraegerStaffAlreadyLinked = errors.New("workforce: the employment already belongs to a staff record")
	// ErrStaffKeptByTraeger refuses a school's change of a value its Träger
	// keeps in the Personalstamm: the personnel number and the bank and tax
	// values (ADR 0089 section 6).
	ErrStaffKeptByTraeger = errors.New("workforce: the Träger keeps this value in its staff record")
)

// TraegerStaffLinkError names the part of a link request that is invalid.
type TraegerStaffLinkError struct {
	Field  string
	Reason string
}

func (e *TraegerStaffLinkError) Error() string {
	return fmt.Sprintf("workforce: link %s: %s", e.Field, e.Reason)
}

func (e *TraegerStaffLinkError) Unwrap() error { return ErrTraegerStaffLinkInvalid }

// TraegerStaffRelease is the release „Personal beim Träger“ of one Träger.
type TraegerStaffRelease struct {
	Representation       string    `json:"representation"`
	ReleasedAt           time.Time `json:"released_at"`
	ReleasedByOperatorID int64     `json:"-"`
}

// TraegerStaffReleaseState is what the moto team sees of a Träger: the
// release, nil without one, and how many records the Träger keeps.
type TraegerStaffReleaseState struct {
	Release *TraegerStaffRelease `json:"release"`
	Records int                  `json:"records"`
}

// TraegerStaffActor is the session of the Träger-Büro that reads or writes.
type TraegerStaffActor struct {
	OrganizationID int64
	AccountID      int64
}

// TraegerStaffSchool is one school of the Träger.
type TraegerStaffSchool struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// TraegerSchoolStaffMember is one live employment of a school with its
// person, read in that school's tenant transaction. Birthday is a calendar
// day (YYYY-MM-DD) or empty.
type TraegerSchoolStaffMember struct {
	StaffID         int64
	PersonID        int64
	AccountID       *int64
	FirstName       string
	LastName        string
	Birthday        string
	PersonnelNumber *string
}

// TraegerStaffEmployment is one Einsatzort of a record: the employment of
// one school. Active is false once the school ended the employment.
type TraegerStaffEmployment struct {
	SchoolID   int64  `json:"school_id"`
	SchoolName string `json:"school_name"`
	StaffID    int64  `json:"staff_id"`
	Active     bool   `json:"active"`
}

// TraegerStaffRecordSummary is one record in the list.
type TraegerStaffRecordSummary struct {
	ID              int64                    `json:"id"`
	FirstName       string                   `json:"first_name"`
	LastName        string                   `json:"last_name"`
	Birthday        string                   `json:"birthday,omitempty"`
	PersonnelNumber *string                  `json:"personnel_number"`
	HasFinancial    bool                     `json:"has_financial"`
	Employments     []TraegerStaffEmployment `json:"employments"`
}

// TraegerStaffCandidate is a live employment no record holds yet.
type TraegerStaffCandidate struct {
	SchoolID        int64   `json:"school_id"`
	SchoolName      string  `json:"school_name"`
	StaffID         int64   `json:"staff_id"`
	FirstName       string  `json:"first_name"`
	LastName        string  `json:"last_name"`
	Birthday        string  `json:"birthday,omitempty"`
	PersonnelNumber *string `json:"personnel_number"`
	HasAccount      bool    `json:"has_account"`
}

// TraegerStaffSuggestion is one proposal: employments that may be one
// person, or employments that may join the existing record RecordID. The
// Personalverwaltung confirms every proposal; nothing joins by itself.
type TraegerStaffSuggestion struct {
	Reason   string                  `json:"reason"`
	RecordID *int64                  `json:"staff_record_id,omitempty"`
	Members  []TraegerStaffCandidate `json:"members"`
}

// TraegerStaffOverview is the Personalstamm of a Träger: the release, its
// schools, its records and the open proposals.
type TraegerStaffOverview struct {
	Release     *TraegerStaffRelease        `json:"release"`
	Schools     []TraegerStaffSchool        `json:"schools"`
	Records     []TraegerStaffRecordSummary `json:"records"`
	Suggestions []TraegerStaffSuggestion    `json:"suggestions"`
}

// TraegerStaffMemberRef names one employment of one school.
type TraegerStaffMemberRef struct {
	SchoolID int64 `json:"school_id"`
	StaffID  int64 `json:"staff_id"`
}

// TraegerStaffLinkRequest joins employments to a new record or, with
// RecordID, to an existing one. Choices names the source of every field
// whose sources disagree: TraegerStaffSourceRecord or a school's ID.
type TraegerStaffLinkRequest struct {
	RecordID *int64
	Members  []TraegerStaffMemberRef
	Choices  map[string]string
}

// TraegerStaffFieldOption is one value a source brings for a field. Bank
// and tax values show only their mask.
type TraegerStaffFieldOption struct {
	Source     string `json:"source"`
	SchoolName string `json:"school_name,omitempty"`
	Value      string `json:"value"`
}

// TraegerStaffFieldPreview lists the different values of one field.
// Conflict is true when more than one value exists and a choice is needed.
type TraegerStaffFieldPreview struct {
	Field    string                    `json:"field"`
	Options  []TraegerStaffFieldOption `json:"options"`
	Conflict bool                      `json:"conflict"`
}

// TraegerStaffLinkPreview shows what a link would bring together before
// the Personalverwaltung confirms it.
type TraegerStaffLinkPreview struct {
	Record  *TraegerStaffRecordSummary `json:"record,omitempty"`
	Members []TraegerStaffCandidate    `json:"members"`
	Fields  []TraegerStaffFieldPreview `json:"fields"`
}

// TraegerStaffFinancial is the bank and tax section of a record, masked or,
// after an explicit reveal, as entered.
type TraegerStaffFinancial struct {
	IBAN                 *string `json:"iban"`
	TaxID                *string `json:"tax_id"`
	SocialSecurityNumber *string `json:"social_security_number"`
}

// TraegerStaffChange is one entry of a record's trail. Value is the value
// as text; bank and tax values show only their mask.
type TraegerStaffChange struct {
	Kind       string    `json:"kind"`
	Field      string    `json:"field"`
	Value      string    `json:"value"`
	SchoolID   *int64    `json:"school_id,omitempty"`
	SchoolName string    `json:"school_name,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

// TraegerStaffRecordDetail is one record with its masked bank and tax
// section and its trail.
type TraegerStaffRecordDetail struct {
	TraegerStaffRecordSummary
	Financial TraegerStaffFinancial `json:"financial"`
	Changes   []TraegerStaffChange  `json:"changes"`
}

// TraegerStaffRecords is the Personalstamm in the Träger-Portal. Every
// method checks the release; the route checks the right „Personal“. Each
// read of personal data writes an access log entry in the tenant of every
// school whose data it shows (ADR 0084 section 7).
type TraegerStaffRecords interface {
	Overview(ctx context.Context, actor TraegerStaffActor) (TraegerStaffOverview, error)
	Record(ctx context.Context, actor TraegerStaffActor, recordID int64) (TraegerStaffRecordDetail, error)
	PreviewLink(ctx context.Context, actor TraegerStaffActor, request TraegerStaffLinkRequest) (TraegerStaffLinkPreview, error)
	Link(ctx context.Context, actor TraegerStaffActor, request TraegerStaffLinkRequest) (TraegerStaffRecordDetail, error)
	UpdatePersonnelNumber(ctx context.Context, actor TraegerStaffActor, recordID int64, value *string) (TraegerStaffRecordDetail, error)
	Financial(ctx context.Context, actor TraegerStaffActor, recordID int64, reveal bool) (TraegerStaffFinancial, error)
	UpdateFinancial(ctx context.Context, actor TraegerStaffActor, recordID int64, input StammdatenFinancialInput) error
	Dissolve(ctx context.Context, actor TraegerStaffActor, recordID int64) error
}

// TraegerStaffReleases is the release in the Operator portal. Each call
// joins the caller's administrative transaction, which holds the operator
// audit entry of a change.
type TraegerStaffReleases interface {
	ReleaseState(ctx context.Context, organizationID int64) (TraegerStaffReleaseState, error)
	GrantRelease(ctx context.Context, organizationID, operatorID int64, representation string) (TraegerStaffRelease, error)
	WithdrawRelease(ctx context.Context, organizationID int64) error
}

// TraegerStaffSchools lists the live schools of a Träger.
type TraegerStaffSchools interface {
	TraegerSchools(ctx context.Context, organizationID int64) ([]TraegerStaffSchool, error)
}

// TraegerSchoolEmployment is one live employment of a school as School
// Membership keeps it, with the personnel number of its profile.
type TraegerSchoolEmployment struct {
	StaffID         int64
	PersonID        int64
	PersonnelNumber *string
}

// TraegerSchoolEmployments reads the live employments of the school of the
// caller's tenant transaction, staff of the Träger only (no external
// caregivers); staffIDs narrows the read, nil reads all.
type TraegerSchoolEmployments interface {
	SchoolEmployments(ctx context.Context, staffIDs []int64) ([]TraegerSchoolEmployment, error)
}

// TraegerSchoolPerson is the person behind an employment.
type TraegerSchoolPerson struct {
	PersonID  int64
	AccountID *int64
	FirstName string
	LastName  string
	Birthday  string
}

// TraegerSchoolPersons reads persons of the school of the caller's tenant
// transaction by ID.
type TraegerSchoolPersons interface {
	SchoolPersons(ctx context.Context, personIDs []int64) ([]TraegerSchoolPerson, error)
}

// TraegerStaffAccess is one access log entry of the Träger-Portal in the
// tenant of one school: which view showed how many of its people.
type TraegerStaffAccess struct {
	OrganizationID int64
	AccountID      int64
	View           string
	Rows           int
	// StaffIDs names the employments of the school the view showed;
	// RecordID the record a record view read, 0 for the overview.
	StaffIDs []int64
	RecordID int64
}

// TraegerStaffAccessLog writes an access entry in the caller's tenant
// transaction. A failed entry fails the read.
type TraegerStaffAccessLog interface {
	RecordTraegerStaffAccess(ctx context.Context, access TraegerStaffAccess) error
}

// TraegerStaffRuntime opens a school's tenant transaction or the Träger
// transaction, never both at once (ADR 0105).
type TraegerStaffRuntime interface {
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
}

// TraegerStaffPorts are the collaborators the Personalstamm does not own.
type TraegerStaffPorts struct {
	Schools     TraegerStaffSchools
	Employments TraegerSchoolEmployments
	Persons     TraegerSchoolPersons
	Access      TraegerStaffAccessLog
	Runtime     TraegerStaffRuntime
}
