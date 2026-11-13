package enrollment

import (
	"context"
	"errors"
	"time"
)

// Träger phase (ADR 0195): a Träger keeps an enrollment phase once in its
// own tables (Träger transaction) and a workflow copies it into its schools
// (each school's tenant transaction). The school's row stays an ordinary
// phase; its link to the Träger phase is a TraegerCopyLink.

var (
	// ErrTraegerPhaseNotFound reports a Träger phase that does not exist for
	// the Träger of the session.
	ErrTraegerPhaseNotFound = errors.New("träger phase not found")
	// ErrTraegerPhaseWithdrawn rejects a change to a withdrawn Träger phase.
	ErrTraegerPhaseWithdrawn = errors.New("träger phase is withdrawn")
	// ErrTraegerSchoolOutsideTraeger rejects a school selection that names a
	// school of another Träger or a deleted school; the database decides.
	ErrTraegerSchoolOutsideTraeger = errors.New("school is not a school of the träger")
	// ErrTraegerCopyDelete rejects a school's delete of a copy: only the
	// Träger office withdraws it (ADR 0195 section 7).
	ErrTraegerCopyDelete = errors.New("a copy of the träger is withdrawn by the träger office, not deleted by the school")
)

// Outcomes of a copy at one school (ADR 0195 section 4).
const (
	TraegerOutcomeApplied   = "applied"
	TraegerOutcomeRejected  = "rejected"
	TraegerOutcomeStale     = "stale"
	TraegerOutcomeWithdrawn = "withdrawn"
)

// Field states of a copy in a preview or an apply (ADR 0195 section 5).
const (
	// TraegerFieldNew is a field of a copy that did not exist before.
	TraegerFieldNew = "new"
	// TraegerFieldChanges takes the Träger's new value.
	TraegerFieldChanges = "changes"
	// TraegerFieldKeeps keeps the school's deviating value.
	TraegerFieldKeeps = "keeps"
	// TraegerFieldUnchanged keeps the value it already had.
	TraegerFieldUnchanged = "unchanged"
)

// TraegerCopyLink is the Träger origin of a school's phase or offering.
type TraegerCopyLink struct {
	SourceID   int64    `json:"source_id"`
	Version    int      `json:"version"`
	Deviations []string `json:"deviations"`
}

// TraegerPhaseFields are the fields a Träger sets on a phase. The JSON
// names are the column names of enrollment.phases: they are the keys of the
// copy's traeger_values.
type TraegerPhaseFields struct {
	IsActive                  *bool      `json:"is_active,omitempty"`
	Name                      string     `json:"name"`
	Kind                      string     `json:"kind"`
	ServiceStartDate          Date       `json:"service_start_date"`
	ServiceEndDate            Date       `json:"service_end_date"`
	EnrollmentOpenAt          *time.Time `json:"enrollment_open_at"`
	EnrollmentCloseAt         *time.Time `json:"enrollment_close_at"`
	Audience                  string     `json:"audience"`
	CareOverflowMode          string     `json:"care_overflow_mode"`
	CareOfferingSelectionMode string     `json:"care_offering_selection_mode"`
}

// TraegerPhaseInput is the office's draft of a Träger phase and its schools.
type TraegerPhaseInput struct {
	Fields     TraegerPhaseFields
	AllSchools bool
	SchoolIDs  []int64
}

// TraegerCatalogSchool is one school of a Träger phase or offering with the
// last outcome of its copy there.
type TraegerCatalogSchool struct {
	SchoolID       int64      `json:"school_id"`
	Selected       bool       `json:"selected"`
	AppliedVersion *int       `json:"applied_version,omitempty"`
	Outcome        string     `json:"outcome,omitempty"`
	ErrorCode      string     `json:"error_code,omitempty"`
	RecordedAt     *time.Time `json:"recorded_at,omitempty"`
}

// TraegerPhase is a Träger phase with its draft, its released version and
// its schools.
type TraegerPhase struct {
	ID              int64                  `json:"id"`
	OrganizationID  int64                  `json:"organization_id"`
	Draft           TraegerPhaseFields     `json:"draft"`
	AllSchools      bool                   `json:"all_schools"`
	ReleasedVersion *int                   `json:"released_version,omitempty"`
	Released        *TraegerPhaseFields    `json:"released,omitempty"`
	WithdrawnAt     *time.Time             `json:"withdrawn_at,omitempty"`
	Schools         []TraegerCatalogSchool `json:"schools"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// TraegerCopyOutcome records the outcome of one school's copy.
type TraegerCopyOutcome struct {
	SchoolID       int64
	AppliedVersion *int
	Outcome        string
	ErrorCode      string
}

// TraegerPhases keeps the Träger phases. Every method runs in the Träger
// transaction of organizationID, which the caller opens (ADR 0105 point 3).
type TraegerPhases interface {
	ListTraegerPhases(ctx context.Context, organizationID int64) ([]TraegerPhase, error)
	TraegerPhase(ctx context.Context, organizationID, id int64) (TraegerPhase, error)
	CreateTraegerPhase(ctx context.Context, organizationID int64, input TraegerPhaseInput) (TraegerPhase, error)
	UpdateTraegerPhase(ctx context.Context, organizationID, id int64, input TraegerPhaseInput) (TraegerPhase, error)
	// WithdrawTraegerPhase marks the phase withdrawn; the next rollout
	// withdraws its copies (ADR 0195 section 7).
	WithdrawTraegerPhase(ctx context.Context, organizationID, id int64) (TraegerPhase, error)
	// LockTraegerPhase reads the phase under its row lock for a release.
	LockTraegerPhase(ctx context.Context, organizationID, id int64) (TraegerPhase, error)
	// ReleaseTraegerPhase writes a new version when the draft differs from
	// the last one and returns the released version. The caller holds the
	// lock of LockTraegerPhase.
	ReleaseTraegerPhase(ctx context.Context, organizationID, id, accountID int64) (int, error)
	RecordTraegerPhaseOutcomes(ctx context.Context, organizationID, id int64, outcomes []TraegerCopyOutcome) error
}

// TraegerFieldChange is one field of a copy in a preview or an apply.
type TraegerFieldChange struct {
	Field   string `json:"field"`
	State   string `json:"state"`
	Traeger any    `json:"traeger"`
	School  any    `json:"school"`
}

// TraegerCopyState is the state of a school's copy that a preview reads and
// an apply compares under the copy's lock (ADR 0195 section 5).
type TraegerCopyState struct {
	Exists  bool
	CopyID  int64
	Version int
	// Stamp changes with every write of the copy.
	Stamp string
}

// TraegerPhaseCopy is the released version of a Träger phase for one school.
type TraegerPhaseCopy struct {
	SourceID int64
	Version  int
	Fields   TraegerPhaseFields
}

// TraegerPhaseApplied is the result of applying a Träger phase in a school.
type TraegerPhaseApplied struct {
	CopyID  int64                `json:"copy_id"`
	Created bool                 `json:"created"`
	Fields  []TraegerFieldChange `json:"fields"`
	// CareBeyondEnd counts the children whose care now runs past the copy's
	// shortened end (#6813, ADR 0211). The copy changes none of them; the
	// school ends each care on purpose.
	CareBeyondEnd int `json:"care_beyond_end"`
}

// TraegerWithdrawal reports how a copy left the school.
type TraegerWithdrawal struct {
	// Deleted is true when the copy had nothing hanging off it; otherwise
	// it stays as an inactive ordinary row of the school.
	Deleted bool `json:"deleted"`
	Found   bool `json:"found"`
}

// TraegerPhaseCopies applies a Träger phase in one school. Every method runs
// in the tenant transaction of that school, which the caller opens.
type TraegerPhaseCopies interface {
	// TraegerPhaseCopyState reads the copy of sourceID; lock takes its row
	// lock first.
	TraegerPhaseCopyState(ctx context.Context, sourceID int64, lock bool) (TraegerCopyState, error)
	// ApplyTraegerPhase creates or updates the copy through the school's own
	// phase commands. Deviating fields keep the school's value.
	ApplyTraegerPhase(ctx context.Context, copy TraegerPhaseCopy) (TraegerPhaseApplied, error)
	// WithdrawTraegerPhase deletes a copy without enrollment requests or
	// care offerings; otherwise it deactivates the copy and drops its link.
	WithdrawTraegerPhase(ctx context.Context, sourceID int64) (TraegerWithdrawal, error)
}

// Active preserves the default of drafts and immutable versions written before
// the active flag was available. An explicit false temporarily deactivates it.
func (f TraegerPhaseFields) Active() bool { return f.IsActive == nil || *f.IsActive }
