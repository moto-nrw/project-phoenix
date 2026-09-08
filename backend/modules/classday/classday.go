package classday

import (
	"context"
	"errors"
	"time"
)

// ErrClassNotAssigned is returned when the requested class is not among the
// caller's education.class_teachers assignments.
var ErrClassNotAssigned = errors.New("Diese Klasse ist Ihnen nicht zugewiesen") //nolint:staticcheck // ST1005: user-facing German message

// ErrStaffRecordRequired is returned when the caller has no users.staff row
// to attribute an entry to.
var ErrStaffRecordRequired = errors.New("Zu Ihrem Konto gibt es keinen Mitarbeiterdatensatz") //nolint:staticcheck // ST1005: user-facing German message

// ErrInvalidReportFilter marks a day-report request the report refuses
// before reading anything (an empty class name).
var ErrInvalidReportFilter = errors.New("class day report filter is invalid")

// ErrArrivalExceptionsNotConfigured is returned by every arrival-exception
// capability when the projection was composed without the write seam. The
// HTTP adapter keeps its route table stable and answers 500.
var ErrArrivalExceptionsNotConfigured = errors.New("arrival exceptions are not configured")

// Sentinels the HTTP layer classifies for class-wide arrival day exceptions.
// Their messages equal the retained schedule service's errors, so the client
// keeps reading the same text (see compose for the parity test).
var (
	// ErrArrivalExceptionPastDate refuses writes and deletes for dates
	// before today.
	ErrArrivalExceptionPastDate = errors.New("class arrival exception date lies in the past")
	// ErrArrivalExceptionWeekend refuses exceptions on Saturday and Sunday.
	ErrArrivalExceptionWeekend = errors.New("class arrival exceptions can only be set from Monday to Friday")
	// ErrArrivalExceptionClassNotFound means no active child carries the
	// class.
	ErrArrivalExceptionClassNotFound = errors.New("school class has no active students")
	// ErrArrivalExceptionNotFound means there is nothing to delete.
	ErrArrivalExceptionNotFound = errors.New("class arrival exception not found")
)

// ArrivalExceptionOriginSchool marks an entry a Lehrkraft made through the
// school portal.
const ArrivalExceptionOriginSchool = "school"

// Actor is the authenticated account a day report is served to; the report
// records the access in the GDPR log under these values.
type Actor struct {
	AccountID int64
	// Roles is the comma-joined role list of the token.
	Roles string
}

// DayRow is one student of the class on the requested day, reduced to what
// the handoff after lessons needs (#1772). Deliberately NO guardian names or
// contact details: the Lehrkraft view is a privacy-reduced projection of the
// class roster — who stays, who goes home, and how.
type DayRow struct {
	StudentID int64  `json:"student_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	// ListEntry marks a class-list-only entry (#2382): a child of the class
	// cohort with NO OGS record ("Keine Betreuung"). StudentID is 0;
	// ListEntryID carries the users.class_list_entries id, serialized as a
	// JSON string because JavaScript clients round numbers beyond 2^53.
	ListEntry   bool     `json:"list_entry,omitempty"`
	ListEntryID int64    `json:"list_entry_id,string,omitempty"`
	GroupName   string   `json:"group_name,omitempty"`
	Registered  bool     `json:"registered"`
	StaysToday  bool     `json:"stays_today"`
	Offerings   []string `json:"offerings"`
	Arrival     string   `json:"arrival,omitempty"`
	Pickup      string   `json:"pickup,omitempty"`
	Departure   string   `json:"departure,omitempty"`
	// Status is the scheduled day status ("sick" / "excused" / "class_trip",
	// plus the derived "cancelled" when a pickup exception calls the care day
	// off), empty when none is reported. The free-text note stays private.
	Status string `json:"status,omitempty"`
	// PickupChanged marks a pickup time that deviates from the child's
	// recurring plan for this weekday (#2294).
	PickupChanged bool `json:"pickup_changed,omitempty"`
	// PickupRegular is the recurring plan's time for the weekday ("15:00"),
	// set only alongside PickupChanged and only when the plan has one.
	PickupRegular string `json:"pickup_regular,omitempty"`
	// ReportedAt is when the deviation became known. Empty for rows without
	// a deviation.
	ReportedAt *time.Time `json:"reported_at,omitempty"`
}

// DayTotals summarize one day report.
type DayTotals struct {
	Students int `json:"students"`
	Staying  int `json:"staying"`
	Leaving  int `json:"leaving"`
	Absent   int `json:"absent"`
	// ListEntries counts the class-list-only entries (#2382) among Students.
	ListEntries int `json:"list_entries"`
}

// DayArrivalException is the class-wide arrival day exception as the class
// view shows it.
type DayArrivalException struct {
	ArrivalTime string `json:"arrival_time"`
	Reason      string `json:"reason,omitempty"`
	// Origin is "ogs" or "school": which portal entered it.
	Origin string `json:"origin"`
}

// DayReport is the read-only per-class day view. Its JSON shape is the wire
// contract of GET /school/class-day.
type DayReport struct {
	SchoolClass string `json:"school_class"`
	Date        Date   `json:"date"`
	// Weekday is the report day key ("mon".."fri"), empty on weekends.
	Weekday   string `json:"weekday"`
	SchoolDay bool   `json:"school_day"`
	PhaseName string `json:"phase_name,omitempty"`
	// EnrollmentKnown is false when no enrollment phase covers the date: the
	// stays/leaves split is then unknowable, NOT "nobody stays".
	EnrollmentKnown bool      `json:"enrollment_known"`
	Totals          DayTotals `json:"totals"`
	Rows            []DayRow  `json:"rows"`
	// ClassArrivalException is the class-wide arrival day exception of the
	// date (#2962/#2970), whoever entered it.
	ClassArrivalException *DayArrivalException `json:"class_arrival_exception,omitempty"`
}

// ArrivalException is one class-wide arrival day exception.
type ArrivalException struct {
	SchoolClass string
	// Date is the ISO calendar day.
	Date Date
	// ArrivalTime is "HH:MM".
	ArrivalTime string
	Reason      *string
	// CreatedAt is RFC 3339.
	CreatedAt string
	// Origin is "ogs" or "school".
	Origin string
}

// ArrivalExceptionWrite is one class-wide arrival day exception as the
// school enters it.
type ArrivalExceptionWrite struct {
	SchoolClass string
	Date        Date
	// ArrivalTime is a wall-clock value; only hour and minute are used.
	ArrivalTime time.Time
	Reason      *string
	// CreatedBy is the users.staff row the entry is attributed to.
	CreatedBy int64
}

// ClassDay is the school-portal capability of the projection: the caller's
// class assignments, the per-class day view and the class-wide arrival day
// exceptions (#2970). Caller scoping (which classes the account may see) is
// resolved here; the HTTP adapter only maps outcomes to status codes.
type ClassDay interface {
	// AssignedClasses returns the school classes assigned to the caller.
	AssignedClasses(ctx context.Context) ([]string, error)
	// ResolveClass picks the class to show: the requested one when it is one
	// of the caller's assignments (normalized comparison), otherwise the
	// first assigned class when nothing was requested. ErrClassNotAssigned
	// when neither applies.
	ResolveClass(ctx context.Context, requested string) (string, error)
	// DayReport builds the day view for one class the caller may see.
	DayReport(ctx context.Context, schoolClass string, date Date, actor Actor) (*DayReport, error)
	// CurrentStaffID resolves the caller's users.staff row, which becomes
	// created_by of an arrival exception. ErrStaffRecordRequired when the
	// account has none.
	CurrentStaffID(ctx context.Context) (int64, error)
	// MayWriteArrivalExceptions applies operations.school_portal_write_scope.
	MayWriteArrivalExceptions(ctx context.Context) (bool, error)
	// ArrivalExceptions returns the exceptions of one class with
	// from <= date <= to.
	ArrivalExceptions(ctx context.Context, schoolClass string, from, to Date) ([]ArrivalException, error)
	// SetArrivalException stores the exception of one class and date as
	// entered by the school.
	SetArrivalException(ctx context.Context, in ArrivalExceptionWrite) (*ArrivalException, error)
	// ClearArrivalException deletes the exception of one class and date.
	ClearArrivalException(ctx context.Context, schoolClass string, date Date) error
	// EarliestBlockStart returns the "HH:MM" start of the first block of the
	// date that addresses the class, "" when there is none.
	EarliestBlockStart(ctx context.Context, schoolClass string, date Date) (string, error)
}
