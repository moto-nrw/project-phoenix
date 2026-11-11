package careplan

import (
	"context"
	"errors"
	"time"
)

// Träger care offering (ADR 0195): a Träger keeps a care offering once for
// the schools of its Träger phase, with the price and the contract template
// the schools get. A workflow copies it into each school's catalog. The copy
// stays an ordinary care offering; its link is a TraegerCopyLink.

var (
	// ErrTraegerOfferingNotFound reports a Träger care offering that does not
	// exist for the Träger of the session.
	ErrTraegerOfferingNotFound = errors.New("träger care offering not found")
	// ErrTraegerOfferingWithdrawn rejects a change to a withdrawn offering.
	ErrTraegerOfferingWithdrawn = errors.New("träger care offering is withdrawn")
	// ErrTraegerCopyDelete rejects a school's delete of a copy: only the
	// Träger office withdraws it (ADR 0195 section 7).
	ErrTraegerCopyDelete = errors.New("a copy of the träger is withdrawn by the träger office, not deleted by the school")
)

// TraegerCopyLink is the Träger origin of a school's care offering.
type TraegerCopyLink struct {
	SourceID   int64
	Version    int
	Deviations []string
}

// TraegerOfferingFields are the fields a Träger sets on a care offering. The
// JSON names are the column names of enrollment.care_offerings: they are the
// keys of the copy's traeger_values.
type TraegerOfferingFields struct {
	Name                string            `json:"name"`
	Description         *string           `json:"description"`
	DaysOfWeekMode      string            `json:"days_of_week_mode"`
	AvailableDays       []string          `json:"available_days"`
	IncludesHolidayCare bool              `json:"includes_holiday_care"`
	IncludesLunch       bool              `json:"includes_lunch"`
	Capacity            *int              `json:"capacity"`
	PriceCents          *int              `json:"price_cents"`
	IsRequired          bool              `json:"is_required"`
	CountsAsCare        bool              `json:"counts_as_care"`
	SelectionGroup      string            `json:"selection_group"`
	SelectionRule       string            `json:"selection_rule"`
	SortOrder           int               `json:"sort_order"`
	PickupTimes         map[string]string `json:"pickup_times"`
}

// TraegerCarePrice is one price of the module: a booked weekday or a scale
// step by days per week, with the price for every further child (the price
// form of the monthly contribution, ADR 0158).
type TraegerCarePrice struct {
	Weekday      string `json:"weekday,omitempty"`
	DaysPerWeek  *int   `json:"days_per_week,omitempty"`
	MonthlyCents int64  `json:"monthly_cents"`
	SiblingCents *int64 `json:"sibling_cents,omitempty"`
	MealExtra    bool   `json:"meal_extra"`
}

// TraegerContractTemplate is the contract template every school's copy gets
// (Care Contract's template input).
type TraegerContractTemplate struct {
	TermKind               string `json:"term_kind"`
	TermMonths             *int   `json:"term_months,omitempty"`
	NoticeMonths           int    `json:"notice_months"`
	NoticeTo               string `json:"notice_to"`
	SpecialTermination     bool   `json:"special_termination"`
	SpecialTerminationNote string `json:"special_termination_note"`
	StatuteRequired        bool   `json:"statute_required"`
	StatuteTitle           string `json:"statute_title"`
	StatuteURL             string `json:"statute_url"`
	ContractPartner        string `json:"contract_partner"`
}

// TraegerOfferingDraft is everything the office keeps on a Träger offering.
type TraegerOfferingDraft struct {
	Fields           TraegerOfferingFields    `json:"fields"`
	Prices           []TraegerCarePrice       `json:"prices"`
	ContractTemplate *TraegerContractTemplate `json:"contract_template,omitempty"`
}

// TraegerOfferingSchool is one school with the last outcome of the copy.
type TraegerOfferingSchool struct {
	SchoolID       int64      `json:"school_id"`
	AppliedVersion *int       `json:"applied_version,omitempty"`
	Outcome        string     `json:"outcome,omitempty"`
	ErrorCode      string     `json:"error_code,omitempty"`
	RecordedAt     *time.Time `json:"recorded_at,omitempty"`
}

// TraegerOffering is a Träger care offering with its draft, its released
// version and the outcome per school.
type TraegerOffering struct {
	ID              int64                   `json:"id"`
	OrganizationID  int64                   `json:"organization_id"`
	TraegerPhaseID  int64                   `json:"traeger_phase_id"`
	Draft           TraegerOfferingDraft    `json:"draft"`
	ReleasedVersion *int                    `json:"released_version,omitempty"`
	Released        *TraegerOfferingDraft   `json:"released,omitempty"`
	WithdrawnAt     *time.Time              `json:"withdrawn_at,omitempty"`
	Schools         []TraegerOfferingSchool `json:"schools"`
	UpdatedAt       time.Time               `json:"updated_at"`
}

// TraegerOfferingOutcome records the outcome of one school's copy.
type TraegerOfferingOutcome struct {
	SchoolID       int64
	AppliedVersion *int
	Outcome        string
	ErrorCode      string
}

// TraegerOfferings keeps the Träger care offerings. Every method runs in the
// Träger transaction of organizationID, which the caller opens.
type TraegerOfferings interface {
	// ListTraegerOfferings returns the offerings of one Träger phase.
	ListTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID int64) ([]TraegerOffering, error)
	TraegerOffering(ctx context.Context, organizationID, id int64) (TraegerOffering, error)
	CreateTraegerOffering(ctx context.Context, organizationID, traegerPhaseID int64, draft TraegerOfferingDraft) (TraegerOffering, error)
	UpdateTraegerOffering(ctx context.Context, organizationID, id int64, draft TraegerOfferingDraft) (TraegerOffering, error)
	// WithdrawTraegerOffering marks the offering withdrawn; the next rollout
	// of its phase withdraws its copies.
	WithdrawTraegerOffering(ctx context.Context, organizationID, id int64) (TraegerOffering, error)
	// ReleaseTraegerOfferings writes a new version of every offering of the
	// phase whose draft differs from its last one. The caller holds the lock
	// of the Träger phase.
	ReleaseTraegerOfferings(ctx context.Context, organizationID, traegerPhaseID, accountID int64) ([]TraegerOffering, error)
	RecordTraegerOfferingOutcomes(ctx context.Context, organizationID, id int64, outcomes []TraegerOfferingOutcome) error
}

// TraegerOfferingCopy is the released version of a Träger offering for one
// school.
type TraegerOfferingCopy struct {
	SourceID int64
	Version  int
	Fields   TraegerOfferingFields
	// PhaseID is the school's copy of the offering's Träger phase.
	PhaseID int64
}

// TraegerFieldChange is one field of a copy in a preview or an apply; the
// states are those of Enrollment's Träger phase copy.
type TraegerFieldChange struct {
	Field   string `json:"field"`
	State   string `json:"state"`
	Traeger any    `json:"traeger"`
	School  any    `json:"school"`
}

// TraegerOfferingApplied is the result of applying a Träger offering.
type TraegerOfferingApplied struct {
	SourceID int64                `json:"source_id"`
	CopyID   int64                `json:"copy_id"`
	Created  bool                 `json:"created"`
	Fields   []TraegerFieldChange `json:"fields"`
}

// TraegerOfferingCopyState is the state of one copy a preview reads and an
// apply compares under the copy's lock.
type TraegerOfferingCopyState struct {
	SourceID int64
	CopyID   int64
	Version  int
	Stamp    string
}

// TraegerOfferingWithdrawal reports how a copy left the school.
type TraegerOfferingWithdrawal struct {
	Found   bool `json:"found"`
	Deleted bool `json:"deleted"`
}

// TraegerOfferingCopies applies Träger offerings in one school. Every method
// runs in the tenant transaction of that school, which the caller opens.
type TraegerOfferingCopies interface {
	// TraegerOfferingCopyStates reads the copies of sourceIDs; lock takes
	// their row locks in ID order.
	TraegerOfferingCopyStates(ctx context.Context, sourceIDs []int64, lock bool) ([]TraegerOfferingCopyState, error)
	// ApplyTraegerOffering creates or updates the copy through the catalog's
	// own commands. Deviating fields keep the school's value.
	ApplyTraegerOffering(ctx context.Context, copy TraegerOfferingCopy) (TraegerOfferingApplied, error)
	// WithdrawTraegerOffering deletes a copy without bookings and
	// selections; otherwise it deactivates it and drops its link.
	WithdrawTraegerOffering(ctx context.Context, sourceID int64) (TraegerOfferingWithdrawal, error)
}
