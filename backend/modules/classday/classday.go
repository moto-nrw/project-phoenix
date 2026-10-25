package classday

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ErrClassNotAssigned is returned when the requested class is not among the
// caller's education.class_teachers assignments.
var ErrClassNotAssigned = errors.New("diese Klasse ist Ihnen nicht zugewiesen")

// ErrStaffRecordRequired is returned when the caller has no users.staff row
// to attribute an entry to.
var ErrStaffRecordRequired = errors.New("zu Ihrem Konto gibt es keinen Mitarbeiterdatensatz")

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

// ErrClassTripsNotConfigured is returned by the class-trip capability when
// the projection was composed without its write seams. The HTTP adapter
// keeps its route table stable and answers 500.
var ErrClassTripsNotConfigured = errors.New("class trips are not configured")

// Sentinels the HTTP layer classifies for a class trip a Lehrkraft reports
// through moto schule (#4799).
var (
	// ErrClassTripsDisabled means the school keeps class trips closed in
	// operations.school_portal_write_scope.
	ErrClassTripsDisabled = errors.New("the school has not opened class trips for moto schule")
	// ErrClassTripPastDate refuses a trip that starts before today.
	ErrClassTripPastDate = errors.New("class trip starts in the past")
	// ErrClassTripInvalidRange refuses an end before the start or a trip
	// longer than ClassTripMaxDays.
	ErrClassTripInvalidRange = errors.New("class trip date range is invalid")
	// ErrClassTripReturnTimeMultiDay refuses a return time on a trip of more
	// than one day: the arrival in the OGS is set for one day only.
	ErrClassTripReturnTimeMultiDay = errors.New("a return time is only possible for a one-day trip")
	// ErrClassTripReturnTimeWeekend refuses a return time on Saturday or
	// Sunday, where no arrival in the OGS is planned.
	ErrClassTripReturnTimeWeekend = errors.New("a return time can only be set from Monday to Friday")
	// ErrClassTripNoStudents refuses a report without children.
	ErrClassTripNoStudents = errors.New("class trip names no children")
	// ErrClassTripStudentNotInClass refuses a child that is not an OGS child
	// of the reported class.
	ErrClassTripStudentNotInClass = errors.New("a child of the class trip is not in this class")
	// ErrClassTripStatusConflict means a child already has a status (sick,
	// excused, class trip) or a partial absence on one of the days. Nothing
	// is written; the OGS clears the old entry first.
	ErrClassTripStatusConflict = errors.New("a child already has a status on a day of the class trip")
	// ErrClassTripArrivalConflict means the OGS already entered another
	// arrival time for a child on that day. Nothing is written.
	ErrClassTripArrivalConflict = errors.New("a child already has another arrival time on the day of the trip")
)

// ClassTripMaxDays is the longest trip one report covers, the limit of the
// OGS's class-trip dialog.
const ClassTripMaxDays = 31

// ClassTripKind says what a class trip report wrote.
const (
	// ClassTripKindStatusDays: the status "Klassenfahrt" for every day.
	ClassTripKindStatusDays = "class_trip"
	// ClassTripKindReturnTime: a one-day outing with a return time; the
	// children's arrival in the OGS is set to that time.
	ClassTripKindReturnTime = "return_time"
)

// ClassTripWrite is a class trip or day outing a Lehrkraft reports for an
// assigned class (#4799).
type ClassTripWrite struct {
	SchoolClass string
	From        Date
	To          Date
	StudentIDs  []int64
	// ReturnTime is set only for a one-day outing; only hour and minute are
	// used.
	ReturnTime *time.Time
	// Reason is the optional destination or occasion ("Zoo").
	Reason *string
	// CreatedBy is the users.staff row the entries are attributed to.
	CreatedBy int64
}

// ClassTripResult says what a class trip report wrote.
type ClassTripResult struct {
	Kind         string `json:"kind"`
	StudentCount int    `json:"student_count"`
	DateCount    int    `json:"date_count"`
}

// ErrDayHintsNotConfigured is returned by every day-hint capability when the
// projection was composed without the day-hint seam (#4795).
var ErrDayHintsNotConfigured = errors.New("day hints are not configured")

// Sentinels the HTTP layer classifies for the Lehrkraft's day hints (#4795).
var (
	// ErrDayHintNotToday refuses a write or removal for any day but today:
	// the hint is a message for this afternoon, not a plan.
	ErrDayHintNotToday = errors.New("a day hint can only be written for today")
	// ErrDayHintStudentNotInClass refuses a hint for a child that is not in
	// the class on that day.
	ErrDayHintStudentNotInClass = errors.New("the child is not in this class")
	// ErrDayHintClassEmpty refuses a class hint for a class without a child
	// in OGS care records.
	ErrDayHintClassEmpty = errors.New("the class has no children with an OGS record")
	// ErrDayHintInvalid refuses an empty or too long hint.
	ErrDayHintInvalid = errors.New("a day hint needs between 1 and 500 characters")
	// ErrDayHintsClosed refuses every write while the school keeps the day
	// hints closed (operations.school_portal_write_scope).
	ErrDayHintsClosed = errors.New("die OGS hat Hinweise der Schule nicht freigegeben")
	// ErrDayHintNotFound means there is no such hint of the caller to remove.
	ErrDayHintNotFound = errors.New("day hint not found")
)

// ClassTripConflict is one day a chosen child already has a status on.
type ClassTripConflict struct {
	StudentID int64  `json:"student_id"`
	Date      Date   `json:"date"`
	Status    string `json:"status"`
}

// ClassTripStatusConflictError names the days that refused a class trip,
// like the OGS's own status-day write does: at most a sample of Conflicts
// and the full Total. It answers errors.Is for ErrClassTripStatusConflict.
type ClassTripStatusConflictError struct {
	Conflicts []ClassTripConflict
	Total     int
}

func (e *ClassTripStatusConflictError) Error() string { return ErrClassTripStatusConflict.Error() }

func (e *ClassTripStatusConflictError) Is(target error) bool {
	return target == ErrClassTripStatusConflict
}

// ErrMorningAttendanceNotConfigured is returned by the morning attendance
// write when the projection was composed without the Student Presence seam.
var ErrMorningAttendanceNotConfigured = errors.New("morning attendance is not configured")

// Sentinels the HTTP layer classifies for the morning attendance (#4790).
var (
	// ErrMorningAttendanceDisabled means the school switched the morning
	// attendance off (operations.school_portal_morning_attendance).
	ErrMorningAttendanceDisabled = errors.New("die OGS hat die Anwesenheit am Vormittag ausgeschaltet")
	// ErrMorningAttendanceNotToday refuses a write for any day but today.
	ErrMorningAttendanceNotToday = errors.New("morning attendance can only be recorded for today")
	// ErrMorningAttendanceNoSchoolDay refuses a write on a day without
	// school (a weekend that does not follow Friday's plan).
	ErrMorningAttendanceNoSchoolDay = errors.New("morning attendance can only be recorded on a school day")
	// ErrMorningAttendanceUnknownChild refuses a mark for a child that is
	// not on the class roster of the day.
	ErrMorningAttendanceUnknownChild = errors.New("child is not on the class roster of the day")
	// ErrMorningAttendanceInvalid refuses a mark without a child or with an
	// unknown status.
	ErrMorningAttendanceInvalid = errors.New("morning attendance mark is invalid")
)

// ErrClassHandoverNotConfigured is returned by the class handover when the
// projection was composed without the Student Presence seam.
var ErrClassHandoverNotConfigured = errors.New("class handover is not configured")

// Sentinels the HTTP layer classifies for the class handover (#4794).
var (
	// ErrClassHandoverDisabled means the school did not tick the handover in
	// operations.school_portal_write_scope.
	ErrClassHandoverDisabled = errors.New("die OGS hat das Übergeben der Klasse nicht freigegeben")
	// ErrClassHandoverNotToday refuses a handover or withdrawal for any day
	// but today.
	ErrClassHandoverNotToday = errors.New("a class can only be handed over today")
	// ErrClassHandoverNoSchoolDay refuses a handover on a day without school.
	ErrClassHandoverNoSchoolDay = errors.New("a class can only be handed over on a school day")
	// ErrClassHandoverUnknownChild refuses a child without an OGS record on
	// the class roster of the day.
	ErrClassHandoverUnknownChild = errors.New("child with an OGS record is not on the class roster of the day")
	// ErrClassHandoverNotFound answers a withdrawal without a handover.
	ErrClassHandoverNotFound = errors.New("the class has not been handed over today")
	// ErrClassHandoverInvalid refuses a malformed handover.
	ErrClassHandoverInvalid = errors.New("class handover is invalid")
)

// Morning attendance statuses (#4790) and where a row's status comes from:
// the Lehrkraft marked it, or a reported absence (sick, excused, class trip)
// fills it in until she does.
const (
	MorningAttendancePresent = "present"
	MorningAttendanceAbsent  = "absent"

	MorningAttendanceSourceTeacher = "teacher"
	MorningAttendanceSourceReport  = "report"
)

// ArrivalExceptionOriginSchool marks an entry a Lehrkraft made through the
// school portal; ArrivalExceptionOriginOGS one the OGS made, and every entry
// older than the origin stamp.
const (
	ArrivalExceptionOriginSchool = "school"
	ArrivalExceptionOriginOGS    = "ogs"
)

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
	// DayHints are the Lehrkraft hints of the day for this child alone
	// (#4795); the hints for the whole class are on the report.
	DayHints []DayHint `json:"day_hints,omitempty"`
	// MorningAttendance is the child's school-morning status (#4790): what
	// the Lehrkraft marked, else "absent" filled in from a reported sick,
	// excused or class-trip day. Nil when neither exists or on a day without
	// school.
	MorningAttendance *DayMorningAttendance `json:"morning_attendance,omitempty"`
	// HomeworkFeedback is what the OGS fed back on the class's homework of
	// the date (#4796). Nil without feedback and for class-list-only entries.
	HomeworkFeedback *DayHomeworkFeedback `json:"homework_feedback,omitempty"`
	// HandoverAbsent is true when the Lehrkraft marked the child "nicht da"
	// at today's handover of the class (#4794).
	HandoverAbsent bool `json:"handover_absent,omitempty"`
}

// DayMorningAttendance is one child's school-morning status on the wire.
type DayMorningAttendance struct {
	// Status is "present" or "absent".
	Status string `json:"status"`
	// Source is "teacher" (marked) or "report" (filled in from a reported
	// absence, not yet marked).
	Source string `json:"source"`
}

// DayHint is a Lehrkraft's hint for the OGS for one day (#4795), for one
// child or for the whole class.
type DayHint struct {
	Body string `json:"body"`
	// AuthorName is the author's current name, empty when the person is gone.
	AuthorName string `json:"author_name,omitempty"`
	// Own marks a hint of the caller, the only ones the caller may change.
	Own       bool      `json:"own"`
	UpdatedAt time.Time `json:"updated_at"`
}

// DayHintWrite is one day hint as the Lehrkraft enters it: for the whole
// class when StudentID is 0, otherwise for that child of the class.
type DayHintWrite struct {
	SchoolClass string
	Date        Date
	StudentID   int64
	Body        string
	// AuthorAccountID is the caller's account.
	AuthorAccountID int64
}

// DayHintTarget names one of the caller's day hints: the class hint when
// StudentID is 0, otherwise the hint for that child.
type DayHintTarget struct {
	SchoolClass     string
	Date            Date
	StudentID       int64
	AuthorAccountID int64
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
	// ClassDayHints are the Lehrkraft hints of the day for the whole class
	// (#4795), oldest first.
	ClassDayHints []DayHint `json:"class_day_hints,omitempty"`
	// CanWriteDayHints is true when the caller may write today's hints for
	// this class: the date is today and the school opened it (projection),
	// and the token holds the permission (HTTP adapter).
	CanWriteDayHints bool `json:"can_write_day_hints"`
	// CanRecordMorningAttendance is true when the caller may mark the
	// morning attendance of this report (#4790): the date is today and a
	// school day, the school left the setting on, and the token holds
	// class_day:morning_attendance_write.
	CanRecordMorningAttendance bool `json:"can_record_morning_attendance"`
	// Homework lists the class's homework of the date (#4796), oldest first.
	// Omitted when there is none.
	Homework []DayHomework `json:"homework,omitempty"`
	// CanWriteHomework is true when the caller may enter homework for this
	// class: the token holds class_day:homework_write. The HTTP layer sets it.
	CanWriteHomework bool `json:"can_write_homework"`
	// Handover is the class's handover to the OGS on the date (#4794), nil
	// while the class has not been handed over.
	Handover *DayHandover `json:"handover,omitempty"`
	// CanHandOver is true when the caller may hand the class over or take
	// the handover back (#4794): the date is today and a school day, the
	// school ticked the handover in its write scope, and the token holds
	// class_day:handover_write.
	CanHandOver bool `json:"can_hand_over"`
}

// DayHandover is the handover of a class as the class view shows it.
type DayHandover struct {
	HandedOverAt time.Time `json:"handed_over_at"`
	// HandedOverBy is the display name of the staff member, empty when the
	// staff member was removed since.
	HandedOverBy string `json:"handed_over_by,omitempty"`
	// AbsentCount is the number of children marked "nicht da".
	AbsentCount int `json:"absent_count"`
}

// ClassHandoverChange hands one class over to the OGS after lessons, or
// takes today's handover back (#4794).
type ClassHandoverChange struct {
	SchoolClass string
	Date        Date
	// HandedOverBy is the users.staff_school_memberships row of the
	// Lehrkraft.
	HandedOverBy int64
	// PresentStudentIDs and AbsentStudentIDs are the children with an OGS
	// record she handed over "da" and "nicht da".
	PresentStudentIDs []int64
	AbsentStudentIDs  []int64
	// Withdraw takes the handover back instead; the children are ignored.
	Withdraw bool
}

// MorningAttendanceMark is one child's mark as the Lehrkraft enters it:
// StudentID for a child with an OGS record, ListEntryID for a class-list-only
// entry (#2382); exactly one is set.
type MorningAttendanceMark struct {
	StudentID   int64
	ListEntryID int64
	Status      string
}

// MorningAttendanceWrite is the morning attendance of one class and date as
// the Lehrkraft enters it.
type MorningAttendanceWrite struct {
	SchoolClass string
	Date        Date
	// RecordedBy is the users.staff_school_memberships row of the Lehrkraft.
	RecordedBy int64
	Marks      []MorningAttendanceMark
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

// ClassTrips lets a Lehrkraft report a class trip or day outing of an
// assigned class through moto schule (#4799).
type ClassTrips interface {
	// MayReportClassTrips applies operations.school_portal_write_scope to
	// class trips (#4799).
	MayReportClassTrips(ctx context.Context) (bool, error)
	// ReportClassTrip records a class trip or day outing of an assigned
	// class: the status "Klassenfahrt" for every day through the OGS's
	// status-day write, or, for one day with a return time, the children's
	// arrival in the OGS at that time. All or nothing.
	ReportClassTrip(ctx context.Context, in ClassTripWrite) (*ClassTripResult, error)
}

// ClassDay is the school-portal capability of the projection: the caller's
// class assignments, the per-class day view and the class-wide arrival day
// exceptions (#2970). Caller scoping (which classes the account may see) is
// resolved here; the HTTP adapter only maps outcomes to status codes.
type ClassDay interface {
	// GetMySchoolClasses returns the school classes assigned to the caller.
	GetMySchoolClasses(ctx context.Context) ([]string, error)
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
	// RecordMorningAttendance stores the morning marks of one class the
	// caller may see (#4790): only for today, on a school day, for children
	// on the class roster of the day, and only while the school leaves the
	// setting on. It never changes the OGS attendance.
	RecordMorningAttendance(ctx context.Context, in MorningAttendanceWrite) error
	// ClassHomework enters, changes and deletes the homework of a class
	// (#4796).
	ClassHomework
	// ChangeClassHandover hands one class the caller may see over to the OGS
	// or takes today's handover back (#4794): only for today, on a school day,
	// for children with an OGS record on the class roster of the day, and
	// only while the school ticks the handover. It never changes the morning
	// attendance or the OGS attendance.
	ChangeClassHandover(ctx context.Context, in ClassHandoverChange) error
	// EarliestBlockStart returns the "HH:MM" start of the first block of the
	// date that addresses the class, "" when there is none.
	EarliestBlockStart(ctx context.Context, schoolClass string, date Date) (string, error)
	// ClassTrips is the class-trip write of moto schule (#4799).
	ClassTrips
	// SupervisionStudentSheet is the per-child pickup and emergency sheet a
	// supervisor opens from a running block (#2527). It carries guardian
	// names and phone numbers, which the day report deliberately does not,
	// so the caller MUST have proven the assignment to a block holding this
	// child before calling. Every call writes a GDPR access-log row.
	// ErrInvalidReportFilter refuses a request without a known child.
	SupervisionStudentSheet(ctx context.Context, in SupervisionSheetInput) (*SupervisionStudentSheet, error)
}

// SupervisionContact is one adult a supervisor may need during an Aufsicht:
// the name, how they relate to the child, and their phone numbers.
//
// Deliberately no e-mail address: this sheet is opened when somebody has to be
// reached NOW, and an inbox is not a way to reach anybody. Everything the sheet
// does not need is a detail a Lehrkraft has no business seeing.
type SupervisionContact struct {
	Name string `json:"name"`
	// Relationship is the stored relationship type ("parent", "guardian", …),
	// not a display string: the portal renders it through the same table as
	// every other guardian screen.
	Relationship string `json:"relationship,omitempty"`
	// Phones carries one entry per stored number, never a joined string: the
	// portal turns each into a tel: link, and a link holding two numbers dials
	// neither. A guardian with a mobile and a work number is the normal case.
	Phones []string `json:"phones"`
	// Note is the pickup remark stored on the relationship ("nur mit
	// Vollmacht", "holt immer freitags ab"). Empty for most contacts.
	Note string `json:"note,omitempty"`
}

// SupervisionStudentSheet is the per-child information one supervisor needs
// while running the block a child is in (#2527): when and how the child leaves
// today, who may collect them, and whom to call in an emergency.
//
// This is a deliberate widening over the class day view (#1772), which carries
// no guardian names at all. It is bounded three ways instead: the caller must
// be assigned to the block, the child must be on that block's roster, and every
// single call writes a GDPR access-log row naming the child.
type SupervisionStudentSheet struct {
	StudentID   int64         `json:"student_id"`
	FirstName   string        `json:"first_name"`
	LastName    string        `json:"last_name"`
	SchoolClass string        `json:"school_class,omitempty"`
	Date        calendar.Date `json:"date"`
	// Arrival / Pickup are the effective times of the day ("07:45" / "15:30"),
	// empty when the plan names none.
	Arrival string `json:"arrival,omitempty"`
	Pickup  string `json:"pickup,omitempty"`
	// Departure renders how the child goes home today ("Bus", "Abholung",
	// "Geht alleine"). Missing plan data reads as "Keine Angabe", never as
	// permission to send the child off alone.
	Departure string `json:"departure"`
	// Status is the reported day status ("sick" / "excused" / "class_trip"),
	// empty when none is reported. Cancelled care ("kommt heute nicht") is
	// reported as "cancelled", matching the class day view.
	Status            string               `json:"status,omitempty"`
	PickupContacts    []SupervisionContact `json:"pickup_contacts"`
	EmergencyContacts []SupervisionContact `json:"emergency_contacts"`
}

// SupervisionSheetInput parameterizes the sheet.
type SupervisionSheetInput struct {
	StudentID int64
	// Date is the day of the sheet; the zero date means today.
	Date calendar.Date
	// CompanionBoundary is the set of student IDs the caller ALREADY sees —
	// the roster of the block. An accompanied departure ("läuft mit …") names
	// only children from this set, exactly like the class day view names only
	// children of the class being served: a Laufgemeinschaft may pair the
	// child with one from a class the caller never gets to see.
	CompanionBoundary []int64
	ActorAccountID    int64
	ActorRole         string
}

// DayHints is the day-hint write of the school portal (#4795), a facade of its
// own beside ClassDay: the Lehrkraft writes today's hint for an assigned class
// or one of its children. The day report carries the hints; this writes them.
// The capability ClassDay returns implements it.
type DayHints interface {
	// SetDayHint writes or rewords the caller's day hint for the class or
	// one of its children, today only. ErrDayHintsClosed when the school's
	// operations.school_portal_write_scope does not open the day hints.
	SetDayHint(ctx context.Context, in DayHintWrite) error
	// ClearDayHint removes one of the caller's day hints, today only.
	ClearDayHint(ctx context.Context, target DayHintTarget) error
}
