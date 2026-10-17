package care

import (
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Care Plan values are complete workflow-owned snapshots, not owner aliases.
type ArrivalException struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StudentID         int64
	ExceptionDate     calendar.Date
	ExpectedArrival   *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
}
type ArrivalSchedule struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StudentID       int64
	Weekday         int
	ExpectedArrival time.Time
	Notes           *string
	CreatedBy       int64
	Source          string
	SourceClass     string
	SourceLabel     string
}
type CourseCatalog struct {
	Enabled                bool
	DisabledReason         string
	PhaseName              string
	EffectiveFrom          calendar.Date
	PendingRequestID       int64
	PendingSubmittedBySelf bool
	OtherRequestPending    bool
	CanRequest             bool
	ReasonRequired         bool
	Items                  []CourseCatalogItem
}
type CourseCatalogItem struct {
	OfferingID       int64
	ActivityGroupID  int64
	Name             string
	Description      string
	AvailableDays    []string
	Capacity         *int
	FreeSlots        *int
	Booked           bool
	Requested        bool
	Waitlisted       bool
	WaitlistPosition int
}
type CreateCourseRequestInput struct {
	StudentID  int64
	AccountID  int64
	OfferingID int64
	Note       string
}
type CreateOfferingChangeInput struct {
	StudentID                   int64
	AccountID                   int64
	Selections                  []OfferingChangeSelection
	EffectiveFrom               calendar.Date
	Note                        string
	CompleteWithdrawalConfirmed bool
}
type DepartureException struct {
	ID            int64
	TenantID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	StudentID     int64
	ExceptionDate calendar.Date
	DepartureMode string
	Reason        *string
	CreatedBy     *int64
}
type EffectivePickupTime struct {
	Date              calendar.Date `json:"date"`
	PickupTime        *time.Time    `json:"pickup_time"`
	WeekdayName       string        `json:"weekday_name"`
	IsException       bool          `json:"is_exception"`
	Notes             string        `json:"notes,omitempty"`
	DayNotes          []NoteData    `json:"day_notes,omitempty"`
	RegularPickupTime *time.Time    `json:"regular_pickup_time,omitempty"`
	ChangedAt         *time.Time    `json:"changed_at,omitempty"`
	DayDepartureMode  string        `json:"day_departure_mode,omitempty"`
}
type ExcusedAbsenceRequest struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudentID      int64
	SubmittedBy    int64
	Dates          []calendar.Date
	Note           string
	AbsenceStatus  string
	Status         string
	DecisionReason *string
	ReviewedBy     *int64
	ReviewedAt     *time.Time
	AppliedAt      *time.Time
}
type ExcusedRequestCreateInput struct {
	StudentID         int64
	GuardianAccountID int64
	Dates             []calendar.Date
	Note              string
	AbsenceStatus     string
	NoteRequired      bool
}
type ExcusedRequestEditInput struct {
	RequestID         int64
	StudentID         int64
	GuardianAccountID int64
	ExpectedVersion   string
	NoteRequired      bool
	Dates             []calendar.Date
	Note              string
}
type GuardianAbsenceReport struct {
	StudentID         int64
	GuardianAccountID int64
	Dates             []calendar.Date
	Status            string
	Note              *string
	ReportedAt        time.Time
}
type GuardianPickupChange struct {
	TenantID          int64
	StudentID         int64
	GuardianAccountID int64
	Date              calendar.Date
	PickupTime        *time.Time
	Reason            *string
}
type NoteData struct {
	ID      int64  `json:"id"`
	Content string `json:"content"`
}
type OfferingChangeCatalog struct {
	PhaseID               int64
	PhaseName             string
	SelectionMode         string
	EarliestEffectiveFrom calendar.Date
	LatestEffectiveFrom   calendar.Date
	TargetGradeLevel      *int16
	TargetSchoolClass     string
	CourseCapacityUntil   calendar.Date `json:"-"`
	Items                 []OfferingChangeCatalogItem
}
type OfferingChangeCatalogItem struct {
	OfferingID      int64
	Name            string
	Description     string
	DaysOfWeekMode  string
	AvailableDays   []string
	SelectionGroup  string
	SelectionRule   string
	IsRequired      bool
	PriceCents      *int
	IncludesLunch   bool
	IncludesHoliday bool
	CountsAsCare    bool
	PickupTimes     map[string]string
	Selected        bool
	SelectedDays    []string
	Automatic       bool
	IsActive        bool
	Capacity        *int
	FreeSlots       *int
	ActivityGroupID *int64
}
type OfferingChangeDecision struct {
	ID                  int64
	SubmittedBy         int64
	SubmittedBySelf     bool
	Status              string
	CompleteWithdrawal  bool
	DecidedAt           time.Time
	EffectiveFrom       calendar.Date
	Reason              string
	Requested           []OfferingChangeRequestedItem
	AppliedDiff         []OfferingChangeDiffEntry
	OverriddenOfferings []OfferingOverride
}
type OfferingChangeDiffEntry struct {
	OfferingID          int64
	Label               string
	OldState            string
	OldDays             []string
	NewState            string
	NewDays             []string
	NewAutomaticDays    []string
	NewRuleDays         []string
	NewDaysWithoutRules []string
	AutoTriggerIDs      []int64
	AutoTriggerNames    []string
	IsCourse            bool
}
type OfferingChangeRequest struct {
	ID                          int64
	TenantID                    int64
	CreatedAt                   time.Time
	UpdatedAt                   time.Time
	StudentID                   int64
	RequestChildID              int64
	SubmittedBy                 int64
	CompleteWithdrawalConfirmed bool
	WithdrawalConfirmedBy       *int64
	WithdrawalConfirmedAt       *time.Time
	ApprovedCompleteWithdrawal  bool
	Payload                     json.RawMessage
	EffectiveFrom               string
	ParentNote                  *string
	Status                      string
	DecisionReason              *string
	DecisionSnapshot            json.RawMessage
	ReviewedBy                  *int64
	ReviewedAt                  *time.Time
	AppliedAt                   *time.Time
}
type OfferingChangeRequestedItem struct {
	OfferingID int64
	Name       string
	Days       []string
}
type OfferingChangeSelection struct {
	OfferingID   int64
	SelectedDays []string
}
type OfferingChangeView struct {
	Request      *OfferingChangeRequest
	Diff         []OfferingChangeDiffEntry
	LastDecision *OfferingChangeDecision
}
type OfferingOverride struct {
	OfferingID int64
	Name       string
}
type PickupException struct {
	ManualPartialAbsence  bool `json:"-"`
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	StudentID             int64
	ExceptionDate         calendar.Date
	PickupTime            *time.Time
	Reason                *string
	ExcusedFrom           *time.Time
	ExcusedReason         *string
	ExcusedCreatedBy      *int64
	ExcusedOwnsPickupTime bool
	ExcusedAuto           bool
	Source                string
	CreatedBy             int64
	CreatedByGuardian     *int64
}
type PickupSchedule struct {
	ID               int64
	TenantID         int64
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StudentID        int64
	Weekday          int
	PickupTime       time.Time
	Notes            *string
	CreatedBy        int64
	Source           string
	CareOfferingID   *int64
	CareOfferingName string
}
type StudentScheduleFilter struct {
	StudentIDs []int64
	Date       calendar.Date
	From       calendar.Date
	To         calendar.Date
}
type StudentStatusDay struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StudentID         int64
	Date              calendar.Date
	Status            string
	ReportedAt        time.Time
	ClearedAt         *time.Time
	Source            string
	GuardianAccountID *int64
	Note              *string
}
