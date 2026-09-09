package workforce

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

// This file is the public time-tracking contract the web adapters consume
// (#2690): the MA-facing stamp/history/absence surface and the Leitung's
// administration of working time. The retained work session, absence, month,
// ledger, month-close, overview, audit-log and export services serve it
// through the composition root until their own move into the Workforce
// module; the wire shapes below are byte-for-byte what those services
// rendered, so the frontend contract does not move.
//
// Calendar days travel as YYYY-MM-DD strings; instants stay time.Time.

// Sentinel failures of the administration surface. Every adapter reports the
// retained service's sentinel as one of these kinds with the original wording
// preserved (see TimeTrackingError), so the HTTP layer classifies by kind and
// renders the message it always rendered.
var (
	ErrManagerControlledAbsence = errors.New("absence type is manager-controlled")
	ErrVacationQuotaInvalid     = errors.New("vacation quota is invalid")

	ErrAdjustmentInvalid           = errors.New("balance adjustment is invalid")
	ErrAdjustmentNotFound          = errors.New("balance adjustment not found")
	ErrAdjustmentExceedsBalance    = errors.New("balance adjustment exceeds the balance")
	ErrAdjustmentInClosedMonth     = errors.New("balance adjustment falls into a closed month")
	ErrAdjustmentHasDependentReset = errors.New("balance adjustment has a dependent reset")
	ErrBalanceAlreadyReset         = errors.New("balance already reset")
	ErrOpeningAlreadyExists        = errors.New("opening balance already exists")

	ErrMonthCloseInvalid = errors.New("month close is invalid")
	ErrMonthNotClosable  = errors.New("month cannot be closed yet")
	ErrMonthNotClosed    = errors.New("month is not closed")
	ErrLaterMonthClosed  = errors.New("a later month is closed")
	ErrMonthOutOfRange   = errors.New("month out of range")

	ErrVacationOpeningInvalid              = errors.New("vacation opening is invalid")
	ErrVacationOpeningNotFound             = errors.New("vacation opening not found")
	ErrVacationOpeningExists               = errors.New("vacation opening already exists")
	ErrVacationOpeningAbsencesBeforeCutoff = errors.New("absences exist before the vacation opening cutoff")

	ErrInvalidTargetRange      = errors.New("target range is invalid")
	ErrOverviewInvalid         = errors.New("overview request is invalid")
	ErrAuditLogInvalid         = errors.New("audit log request is invalid")
	ErrTimeExportInvalid       = errors.New("time export request is invalid")
	ErrPayrollConfigIncomplete = errors.New("payroll configuration is incomplete")
	ErrScheduleValidation      = errors.New("schedule validation failed")
)

// WorkSessionWire serializes a work session for the endpoints that return
// the raw block. JavaScript rounds int64 values above Number.MAX_SAFE_INTEGER
// while parsing numeric JSON, so the IDs travel as decimal strings; a nil
// session renders as null.
type WorkSessionWire struct {
	*WorkSession
}

func (ws WorkSessionWire) MarshalJSON() ([]byte, error) {
	if ws.WorkSession == nil {
		return []byte("null"), nil
	}
	type alias WorkSession
	return json.Marshal(struct {
		*alias
		ID        string  `json:"id"`
		TenantID  string  `json:"tenant_id"`
		StaffID   string  `json:"staff_id"`
		CreatedBy string  `json:"created_by"`
		UpdatedBy *string `json:"updated_by,omitempty"`
	}{
		alias:     (*alias)(ws.WorkSession),
		ID:        strconv.FormatInt(ws.ID, 10),
		TenantID:  strconv.FormatInt(ws.TenantID, 10),
		StaffID:   strconv.FormatInt(ws.StaffID, 10),
		CreatedBy: strconv.FormatInt(ws.CreatedBy, 10),
		UpdatedBy: formatOptionalID(ws.UpdatedBy),
	})
}

func formatOptionalID(id *int64) *string {
	if id == nil {
		return nil
	}
	value := strconv.FormatInt(*id, 10)
	return &value
}

// BreakDurationUpdate corrects one break's duration inside a session edit.
type BreakDurationUpdate struct {
	ID              int64 `json:"id"`
	DurationMinutes int   `json:"duration_minutes"`
}

// SessionUpdateRequest is the body of a session edit; nil leaves a field
// untouched.
type SessionUpdateRequest struct {
	Date         *time.Time            `json:"date"`
	CheckInTime  *time.Time            `json:"check_in_time"`
	CheckOutTime *time.Time            `json:"check_out_time"`
	BreakMinutes *int                  `json:"break_minutes"`
	Status       *string               `json:"status"`
	Notes        *string               `json:"notes"`
	Breaks       []BreakDurationUpdate `json:"breaks"`
}

// AdminCreateSessionRequest is the body of the admin nachtragen flow. Date
// stays an instant on the wire (the frontend sends RFC3339 UTC midnight); the
// service derives the Berlin calendar day.
type AdminCreateSessionRequest struct {
	Date         time.Time `json:"date"`
	CheckInTime  time.Time `json:"check_in_time"`
	CheckOutTime time.Time `json:"check_out_time"`
	BreakMinutes int       `json:"break_minutes"`
	Status       string    `json:"status"`
	Notes        string    `json:"notes"`
}

// SessionResponse is one block of the history with its as-of-now figures.
// BreakMinutes shadows the block's cached value on the wire on purpose: the
// cache holds ended breaks only while NetMinutes already deducts a running
// one, and gross = net + break must hold for the reader.
type SessionResponse struct {
	*WorkSession
	BreakMinutes     int                 `json:"break_minutes"`
	NetMinutes       int                 `json:"net_minutes"`
	IsOvertime       bool                `json:"is_overtime"`
	IsBreakCompliant bool                `json:"is_break_compliant"`
	Breaks           []*WorkSessionBreak `json:"breaks"`
	EditCount        int                 `json:"edit_count"`
	AuditCount       int                 `json:"audit_count"`
}

// MarshalJSON preserves exact history IDs and the calculated break-minute field.
func (sr SessionResponse) MarshalJSON() ([]byte, error) {
	type alias SessionResponse
	if sr.WorkSession == nil {
		return json.Marshal(alias(sr))
	}
	return json.Marshal(struct {
		alias
		ID        string  `json:"id"`
		TenantID  string  `json:"tenant_id"`
		StaffID   string  `json:"staff_id"`
		CreatedBy string  `json:"created_by"`
		UpdatedBy *string `json:"updated_by,omitempty"`
	}{
		alias:     alias(sr),
		ID:        strconv.FormatInt(sr.ID, 10),
		TenantID:  strconv.FormatInt(sr.TenantID, 10),
		StaffID:   strconv.FormatInt(sr.StaffID, 10),
		CreatedBy: strconv.FormatInt(sr.CreatedBy, 10),
		UpdatedBy: formatOptionalID(sr.UpdatedBy),
	})
}

// WorkWeekSummary aggregates the recorded work of one ISO week.
type WorkWeekSummary struct {
	WeekNumber      int  `json:"week_number"`
	Year            int  `json:"year"`
	TotalNetMinutes int  `json:"total_net_minutes"`
	TargetMinutes   *int `json:"target_minutes,omitempty"`
	DeltaMinutes    *int `json:"delta_minutes,omitempty"`
	SessionCount    int  `json:"session_count"`
	IsOverWeeklyMax bool `json:"is_over_weekly_max"`
}

// HistoryResponse is the session history of a range with its weekly sums.
type HistoryResponse struct {
	Sessions        []*SessionResponse `json:"sessions"`
	WeeklySummaries []WorkWeekSummary  `json:"weekly_summaries"`
}

// WorkSessionEdit is one audit row of a session edit.
type WorkSessionEdit struct {
	ID        int64     `json:"id"`
	TenantID  int64     `json:"tenant_id"`
	SessionID int64     `json:"session_id"`
	StaffID   int64     `json:"staff_id"`
	EditedBy  int64     `json:"edited_by"`
	FieldName string    `json:"field_name"`
	OldValue  *string   `json:"old_value"`
	NewValue  *string   `json:"new_value"`
	Notes     *string   `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
}

// WorkSessionEditView decorates an audit row with the editor's display name
// and whether the owner edited their own block.
type WorkSessionEditView struct {
	*WorkSessionEdit
	EditorName string `json:"editor_name"`
	IsSelfEdit bool   `json:"is_self_edit"`
}

// ExportFile is a rendered download.
type ExportFile struct {
	Data        []byte
	Filename    string
	ContentType string
}

// ScheduleEntry is one day of a schedule update.
type ScheduleEntry struct {
	WeekIndex     int
	DayOfWeek     int
	TargetMinutes int
	StartTime     *string
}

// ScheduleUpdateInput mirrors the schedule PUT at the capability boundary.
type ScheduleUpdateInput struct {
	Mode               string
	ModelID            *int64
	RotationLength     int
	RotationAnchorDate string
	Entries            []ScheduleEntry
	SaveAsTemplateName string
}

// WorkSessions is the work session administration of the web app: stamps,
// history, edits, exports and the staff work schedule.
type WorkSessions interface {
	CheckIn(ctx context.Context, staffID int64, status, source, reason string) (*WorkSession, error)
	CheckOut(ctx context.Context, staffID int64, reason string) (*WorkSession, error)
	StartBreak(ctx context.Context, staffID int64, plannedDurationMinutes *int) (*WorkSessionBreak, error)
	EndBreak(ctx context.Context, staffID int64) (*WorkSession, error)
	// LatestOpenSession is the block still running, whichever day it was
	// opened on; nil when there is none.
	LatestOpenSession(ctx context.Context, staffID int64) (*WorkSession, error)
	SessionBreaks(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionBreak, error)
	UpdateSession(ctx context.Context, staffID, sessionID int64, updates SessionUpdateRequest) (*WorkSession, error)
	UpdateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID, sessionID int64, updates SessionUpdateRequest) (*WorkSession, error)
	CreateSessionAsAdmin(ctx context.Context, editorStaffID, targetStaffID int64, request AdminCreateSessionRequest) (*WorkSession, error)
	// HistoryIntersecting reads every block that reaches into [from, to].
	HistoryIntersecting(ctx context.Context, staffID int64, from, to string) (*HistoryResponse, error)
	SessionEdits(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error)
	SessionEditsForStaff(ctx context.Context, staffID, sessionID int64) ([]*WorkSessionEditView, error)
	// TodayPresenceMap maps staff id to the work status of everyone clocked
	// in right now.
	TodayPresenceMap(ctx context.Context) (map[int64]string, error)
	ExportSessions(ctx context.Context, staffID int64, from, to, format string) (*ExportFile, error)
	// UpdateStaffSchedule resolves the requested mode (template or custom),
	// validates and applies the change; validation failures carry
	// ErrScheduleValidation. The current schedule and the templates are read
	// through Query.
	UpdateStaffSchedule(ctx context.Context, staffID int64, input ScheduleUpdateInput) error
}

// StaffAbsenceResponse is an absence with its derived duration. AbsenceTypeID
// shadows the record's field on the wire so the BIGINT arrives as a lossless
// decimal string.
type StaffAbsenceResponse struct {
	*StaffAbsence
	DurationDays  int     `json:"duration_days"`
	AbsenceTypeID *string `json:"absence_type_id,omitempty"`
}

// StaffAbsenceRequestItem is one request of the Anfragen module: the absence
// plus the person it belongs to and, in the history, who decided it.
type StaffAbsenceRequestItem struct {
	*StaffAbsenceResponse
	StaffName     string `json:"staff_name"`
	DecidedByName string `json:"decided_by_name,omitempty"`
}

// CreateAbsenceRequest files an absence.
type CreateAbsenceRequest struct {
	AbsenceType   string `json:"absence_type"`
	AbsenceTypeID *int64 `json:"absence_type_id"`
	DateStart     string `json:"date_start"`
	DateEnd       string `json:"date_end"`
	HalfDay       bool   `json:"half_day"`
	Note          string `json:"note"`
}

// UpdateAbsenceRequest edits an absence; nil leaves a field untouched.
// AbsenceTypeIDSet distinguishes an omitted absence_type_id from JSON null,
// which clears a previously selected school-defined art.
type UpdateAbsenceRequest struct {
	AbsenceType      *string `json:"absence_type"`
	AbsenceTypeID    *int64  `json:"absence_type_id"`
	AbsenceTypeIDSet bool    `json:"-"`
	DateStart        *string `json:"date_start"`
	DateEnd          *string `json:"date_end"`
	HalfDay          *bool   `json:"half_day"`
	Note             *string `json:"note"`
}

// RequestVacationRequest is what a staff member submits via "Urlaub
// beantragen"; the half-day flags cover the first and last day only.
type RequestVacationRequest struct {
	DateStart         string `json:"date_start"`
	DateEnd           string `json:"date_end"`
	StartHalfDay      bool   `json:"start_half_day"`
	EndHalfDay        bool   `json:"end_half_day"`
	Note              string `json:"note"`
	SubstituteStaffID *int64 `json:"substitute_staff_id,omitempty"`
}

// VacationDecisionRequest is the Leitung's approve/deny/question payload.
type VacationDecisionRequest struct {
	DecisionNote string `json:"decision_note"`
}

// StaffAbsenceListFilter selects a staff member's absences by overlapping
// date range, status, or both. From and To are both set or both empty.
type StaffAbsenceListFilter struct {
	From   string
	To     string
	Status string
}

// AbsenceRequestListQuery selects the open work list or the decided history
// of the Anfragen module, narrowed by absence type and staff name.
type AbsenceRequestListQuery struct {
	History bool
	Types   []string
	Search  string
}

// VacationQuotaSummary aggregates entitled, taken and reserved vacation days
// of one staff member and year.
type VacationQuotaSummary struct {
	StaffID         int64                 `json:"staff_id"`
	Year            int                   `json:"year"`
	EntitledDays    float64               `json:"entitled_days"`
	CarryoverDays   float64               `json:"carryover_days"`
	TakenBeforeDays float64               `json:"taken_before_days"`
	TakenDays       float64               `json:"taken_days"`
	ReservedDays    float64               `json:"reserved_days"`
	RemainingDays   float64               `json:"remaining_days"`
	Opening         *StaffVacationOpening `json:"opening,omitempty"`
}

// SetVacationOpeningRequest carries the vacation takeover: the Resturlaub as
// of the Stichtag EffectiveDate.
type SetVacationOpeningRequest struct {
	EffectiveDate string
	RemainingDays float64
	Note          string
}

// CompTimeBalancePreview is what the create modal shows before a
// Freizeitausgleich is confirmed. All values are minutes.
type CompTimeBalancePreview struct {
	CurrentBalanceMinutes    int `json:"current_balance_minutes"`
	DeductionMinutes         int `json:"deduction_minutes"`
	RealizedDeductionMinutes int `json:"realized_deduction_minutes"`
	FutureCommitmentMinutes  int `json:"future_commitment_minutes"`
	FutureAdjustmentMinutes  int `json:"future_adjustment_minutes"`
	ProjectedBalanceMinutes  int `json:"projected_balance_minutes"`
}

// StaffAbsences is the absence and vacation administration of the web app.
type StaffAbsences interface {
	ListAbsences(ctx context.Context, staffID int64, filter StaffAbsenceListFilter) ([]*StaffAbsenceResponse, error)
	AbsencesForRange(ctx context.Context, staffID int64, from, to string) ([]*StaffAbsenceResponse, error)
	CreateOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, request CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	CreateAbsenceFor(ctx context.Context, subjectStaffID, createdByStaffID int64, actorAccountID *int64, request CreateAbsenceRequest) (*StaffAbsenceResponse, error)
	UpdateAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64, request UpdateAbsenceRequest) (*StaffAbsenceResponse, error)
	DeleteOwnAbsence(ctx context.Context, staffID int64, actorAccountID *int64, absenceID int64) error
	DeleteAbsenceFor(ctx context.Context, subjectStaffID, actorStaffID int64, actorAccountID *int64, absenceID int64) error
	PreviewCompTimeBalance(ctx context.Context, staffID int64, start, end string, halfDay bool) (*CompTimeBalancePreview, error)
	RequestVacation(ctx context.Context, staffID int64, request RequestVacationRequest) (*StaffAbsenceResponse, error)
	CancelAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64) error
	ResubmitAbsence(ctx context.Context, staffID, actorAccountID, absenceID int64, note string) (*StaffAbsenceResponse, error)
	ApproveAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, note string) (*StaffAbsenceResponse, error)
	DenyAbsence(ctx context.Context, absenceID, actorAccountID, decidedByStaffID int64, reason string) (*StaffAbsenceResponse, error)
	QuestionAbsence(ctx context.Context, absenceID, actorAccountID int64, note string) (*StaffAbsenceResponse, error)
	ListPendingRequests(ctx context.Context) ([]*StaffAbsenceResponse, error)
	ListAbsenceRequests(ctx context.Context, query AbsenceRequestListQuery) ([]*StaffAbsenceRequestItem, error)
	VacationQuotaSummary(ctx context.Context, staffID int64, year int) (*VacationQuotaSummary, error)
	UpsertVacationQuota(ctx context.Context, staffID int64, year int, entitled, carryover float64) error
	SetVacationOpening(ctx context.Context, staffID, decidedBy int64, request SetVacationOpeningRequest) (*StaffVacationOpening, error)
	DeleteVacationOpening(ctx context.Context, staffID, deletedBy int64, year int) error
}

// AdjustmentView is one Stundenkonto transaction as the Monatskarte shows it.
type AdjustmentView struct {
	ID            int64     `json:"id"`
	Type          string    `json:"type"`
	MinutesDelta  int       `json:"minutes_delta"`
	EffectiveDate string    `json:"effective_date"`
	Note          string    `json:"note"`
	DecidedBy     int64     `json:"decided_by"`
	DecidedAt     time.Time `json:"decided_at"`
}

// MonthSummary is the Monatskarte of one staff member and month.
type MonthSummary struct {
	StaffID int64 `json:"staff_id"`
	Year    int   `json:"year"`
	Month   int   `json:"month"`

	CarryInMinutes          int     `json:"carry_in_minutes"`
	TargetMinutes           int     `json:"target_minutes"`
	TargetMinutesToDate     int     `json:"target_minutes_to_date"`
	ActualMinutes           int     `json:"actual_minutes"`
	CreditedSickMinutes     int     `json:"credited_sick_minutes"`
	CreditedVacationMinutes int     `json:"credited_vacation_minutes"`
	CreditedTrainingMinutes int     `json:"credited_training_minutes"`
	CreditedOtherMinutes    int     `json:"credited_other_minutes"`
	SickDays                float64 `json:"sick_days"`
	VacationDays            float64 `json:"vacation_days"`
	TrainingDays            float64 `json:"training_days"`

	PlannedShiftMinutes *int `json:"planned_shift_minutes,omitempty"`

	AdjustmentMinutes int              `json:"adjustment_minutes"`
	Adjustments       []AdjustmentView `json:"adjustments,omitempty"`

	BalanceMinutes        int `json:"balance_minutes"`
	ClosingBalanceMinutes int `json:"closing_balance_minutes"`

	IsClosed                    bool       `json:"is_closed"`
	ClosedAt                    *time.Time `json:"closed_at,omitempty"`
	ClosedBy                    *int64     `json:"closed_by,omitempty"`
	CloseReason                 string     `json:"close_reason,omitempty"`
	FrozenClosingBalanceMinutes *int       `json:"frozen_closing_balance_minutes,omitempty"`
	DriftMinutes                int        `json:"drift_minutes"`
	CarryInFrozen               bool       `json:"carry_in_frozen"`
	CarryInFrozenFromMonth      *string    `json:"carry_in_frozen_from_month,omitempty"`
}

// DailyProjection is the priced work-time picture of one calendar day.
type DailyProjection struct {
	Date           string `json:"date"`
	TargetMinutes  int    `json:"target_minutes"`
	CreditMinutes  int    `json:"credit_minutes"`
	ActualMinutes  int    `json:"actual_minutes"`
	BalanceMinutes int    `json:"balance_minutes"`
}

// DailyTarget is the contractual Soll of one calendar day.
type DailyTarget struct {
	Date          string `json:"date"`
	TargetMinutes int    `json:"target_minutes"`
}

// WorkTimeMonths is the Monatskarte read model.
type WorkTimeMonths interface {
	MonthSummary(ctx context.Context, staffID int64, year, month int) (*MonthSummary, error)
	DailyTargets(ctx context.Context, staffID int64, from, to string) ([]DailyTarget, error)
	DailyProjection(ctx context.Context, staffID int64, from, to string) ([]DailyProjection, error)
}

// CreateBalanceAdjustmentRequest carries a payout or lump-sum comp-time grant;
// MinutesDelta is signed and negative.
type CreateBalanceAdjustmentRequest struct {
	Type          string
	MinutesDelta  int
	EffectiveDate string
	Note          string
}

// BalanceAdjustments is the Stundenkonto ledger administration.
type BalanceAdjustments interface {
	ListAdjustments(ctx context.Context, staffID int64, from, to string) ([]*StaffBalanceAdjustment, error)
	CreateAdjustment(ctx context.Context, staffID, decidedBy int64, request CreateBalanceAdjustmentRequest) (*StaffBalanceAdjustment, error)
	DeleteAdjustment(ctx context.Context, staffID, adjustmentID, deletedBy int64) error
	ResetBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, carryoverMinutes int, note string) (*StaffBalanceAdjustment, error)
	CreateOpeningBalance(ctx context.Context, staffID, decidedBy int64, effectiveDate string, balanceMinutes int, note string) (*StaffBalanceAdjustment, error)
}

// StaffMonthBalanceSnapshot is one frozen month of one staff member.
type StaffMonthBalanceSnapshot struct {
	ID                    int64      `json:"id"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	TenantID              int64      `json:"tenant_id"`
	StaffID               int64      `json:"staff_id"`
	Year                  int        `json:"year"`
	Month                 int        `json:"month"`
	ClosingBalanceMinutes int        `json:"closing_balance_minutes"`
	CarryInMinutes        int        `json:"carry_in_minutes"`
	TargetMinutes         int        `json:"target_minutes"`
	ActualMinutes         int        `json:"actual_minutes"`
	CreditedMinutes       int        `json:"credited_minutes"`
	AdjustmentMinutes     int        `json:"adjustment_minutes"`
	ClosedAt              time.Time  `json:"closed_at"`
	ClosedBy              int64      `json:"closed_by"`
	CloseReason           string     `json:"close_reason,omitempty"`
	Source                string     `json:"source"`
	ReopenedAt            *time.Time `json:"reopened_at,omitempty"`
	ReopenedBy            *int64     `json:"reopened_by,omitempty"`
	ReopenReason          string     `json:"reopen_reason,omitempty"`
}

// MonthCloseResult reports what a school-wide close did.
type MonthCloseResult struct {
	Year         int                          `json:"year"`
	Month        int                          `json:"month"`
	ClosedStaff  int                          `json:"closed_staff"`
	SkippedStaff int                          `json:"skipped_staff"`
	Snapshots    []*StaffMonthBalanceSnapshot `json:"snapshots"`
}

// MonthClosing is the Monatsabschluss.
type MonthClosing interface {
	CloseMonth(ctx context.Context, closedBy int64, year, month int, reason string) (*MonthCloseResult, error)
	ReopenMonth(ctx context.Context, staffID, reopenedBy int64, year, month int, reason string) error
	ListMonthStatus(ctx context.Context, year, month int) ([]*StaffMonthBalanceSnapshot, error)
}

// Overview periods of the Leitungs-Dashboard.
const (
	OverviewPeriodWeek  = "week"
	OverviewPeriodMonth = "month"
)

// DashboardPeriodTotals is the tenant-wide Soll/Ist/Delta of a period.
type DashboardPeriodTotals struct {
	SollMinutes  int `json:"soll_minutes"`
	IstMinutes   int `json:"ist_minutes"`
	DeltaMinutes int `json:"delta_minutes"`
}

// DashboardSummary is the Leitungs-Dashboard aggregate.
type DashboardSummary struct {
	ActiveStaffCount        int                   `json:"active_staff_count"`
	SickToday               int                   `json:"sick_today"`
	VacationToday           int                   `json:"vacation_today"`
	CurrentlyClockedIn      int                   `json:"currently_clocked_in"`
	ExpectedClockedIn       int                   `json:"expected_clocked_in"`
	Period                  DashboardPeriodTotals `json:"period"`
	SaldoSchoolTotalMinutes int                   `json:"saldo_school_total_minutes"`
	PendingRequestsCount    int                   `json:"pending_requests_count"`
}

// TimeTrackingOverviewRow is one staff member's Soll/Ist/Saldo/Resturlaub.
type TimeTrackingOverviewRow struct {
	StaffID               int64   `json:"staff_id"`
	FirstName             string  `json:"first_name"`
	LastName              string  `json:"last_name"`
	Name                  string  `json:"name"`
	EmploymentType        string  `json:"employment_type"`
	SollMinutes           int     `json:"soll_minutes"`
	IstMinutes            int     `json:"ist_minutes"`
	BalanceMinutes        int     `json:"balance_minutes"`
	RemainingVacationDays float64 `json:"remaining_vacation_days"`
}

// TimeTrackingOverview is the cross-staff list; Rows is never nil.
type TimeTrackingOverview struct {
	Year  int                       `json:"year"`
	Month int                       `json:"month"`
	Rows  []TimeTrackingOverviewRow `json:"rows"`
}

// OverviewFilters narrows the cross-staff list. Year and Month both zero
// select the current month.
type OverviewFilters struct {
	EmploymentType string
	SaldoMin       *int
	SaldoMax       *int
	SortBy         string
	Descending     bool
	Year           int
	Month          int
}

// StaffOverview serves the tenant-wide time-tracking views.
type StaffOverview interface {
	DashboardSummary(ctx context.Context, period string) (*DashboardSummary, error)
	TimeTrackingOverview(ctx context.Context, filters OverviewFilters) (*TimeTrackingOverview, error)
}

// AuditLogPerson names a staff member in the audit feed.
type AuditLogPerson struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

// AuditLogActor is the acting person of an audit event; StaffID is nil for
// system actions.
type AuditLogActor struct {
	StaffID  *int64 `json:"staff_id,omitempty"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
	IsSelf   bool   `json:"is_self"`
}

// AuditLogEvent is one feed entry: common envelope plus source-typed detail.
type AuditLogEvent struct {
	OccurredAt time.Time       `json:"occurred_at"`
	Source     string          `json:"source"`
	EntryID    int64           `json:"entry_id"`
	Staff      *AuditLogPerson `json:"staff,omitempty"`
	Actor      AuditLogActor   `json:"actor"`
	Reason     string          `json:"reason"`
	Detail     json.RawMessage `json:"detail"`
}

// AuditLogPage is one keyset page; NextCursor is empty on the last page.
type AuditLogPage struct {
	Events          []*AuditLogEvent `json:"events"`
	NextCursor      string           `json:"next_cursor,omitempty"`
	RetentionCutoff string           `json:"retention_cutoff"`
}

// AuditLogListRequest carries the parsed audit-log filters.
type AuditLogListRequest struct {
	From         string
	To           string
	StaffID      int64
	ActorStaffID int64
	Sources      []string
	Cursor       string
	Limit        int
}

// TimeTrackingAuditLog serves the cross-staff audit feed.
type TimeTrackingAuditLog interface {
	ListAuditLog(ctx context.Context, request AuditLogListRequest) (*AuditLogPage, error)
}

// TimeExportRequest selects what to export; Month == 0 means the whole year.
type TimeExportRequest struct {
	Year        int
	Month       int
	Granularity string
	Format      string
	TimeFormat  string
}

// DatevSkippedStaff is one staff member left out of a DATEV file.
type DatevSkippedStaff struct {
	LastName  string `json:"last_name"`
	FirstName string `json:"first_name"`
	Reason    string `json:"reason"`
}

// DatevExportReport is the visible companion of a DATEV file.
type DatevExportReport struct {
	Format                 string              `json:"format"`
	Year                   int                 `json:"year"`
	Month                  int                 `json:"month"`
	LineCount              int                 `json:"line_count"`
	StaffExported          int                 `json:"staff_exported"`
	StaffSkipped           []DatevSkippedStaff `json:"staff_skipped"`
	UnconfiguredCategories []string            `json:"unconfigured_categories"`
	OpenMonth              bool                `json:"open_month"`
}

// PublicHoliday is one public holiday of the tenant's federal state.
type PublicHoliday struct {
	Date string `json:"date"`
	Name string `json:"name"`
}

// ClosingPeriod is one closure period of the school.
type ClosingPeriod struct {
	StartDate string
	EndDate   string
	Reason    string
}

// PlanningCalendar is the tenant-global calendar the time-tracking views mark:
// public holidays and closure periods overlapping a range of days.
type PlanningCalendar interface {
	HolidaysInRange(ctx context.Context, from, to string) ([]PublicHoliday, error)
	ClosingDaysInRange(ctx context.Context, from, to string) ([]ClosingPeriod, error)
}

// StaffTimeExport renders the cross-staff export.
type StaffTimeExport interface {
	ExportStaffTime(ctx context.Context, request TimeExportRequest, actorAccountID int64, actorRole string) (*ExportFile, error)
	DatevReport(ctx context.Context, request TimeExportRequest) (*DatevExportReport, error)
}
