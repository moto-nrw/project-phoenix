package workforce

import (
	"encoding/json"
	"time"
)

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
