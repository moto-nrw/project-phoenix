package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type ActivityRecoveryRecords interface {
	// CompletionTimestamp returns PostgreSQL's stable time for the current
	// transaction. It is the baseline for distinguishing completion writes from
	// later attendance and supervisor edits during reopen.
	CompletionTimestamp(ctx context.Context) (time.Time, error)
	// LockOpenVisits takes FOR UPDATE locks on every still-open visit of the
	// group so completion can snapshot the same rows EndActivitySession closes.
	LockOpenVisits(ctx context.Context, activeGroupID int64) error
	// LockOpenSupervisors locks every still-open supervisor of the group so
	// completion snapshots the same rows EndActivitySession closes.
	LockOpenSupervisors(ctx context.Context, activeGroupID int64) error
	// LockSupervisors locks the snapshot supervisor rows during reopen so a
	// concurrent staffing change cannot hide from the unchanged-row check.
	LockSupervisors(ctx context.Context, supervisorIDs []int64) error
	// LockAttendance locks every instance_students row of the instance so
	// complete/cancel serialize with attendance PATCH and reopen can refuse
	// a restore after a post-completion attendance edit.
	LockAttendance(ctx context.Context, instanceID int64) error
	Restore(ctx context.Context, instanceID int64, snapshot schedulerecords.ActivityCompletionSnapshot, now time.Time) error
}

type CalendarPeriodRecords interface {
	Create(context.Context, *schedulerecords.CalendarPeriod) error
	FindByID(context.Context, int64) (*schedulerecords.CalendarPeriod, error)
	Update(context.Context, *schedulerecords.CalendarPeriod) error
	Delete(context.Context, int64) error

	// FindByTenantID finds all calendar periods for a tenant
	FindByTenantID(ctx context.Context) ([]*schedulerecords.CalendarPeriod, error)

	// FindActiveByTenantID finds all active calendar periods for a tenant
	FindActiveByTenantID(ctx context.Context) ([]*schedulerecords.CalendarPeriod, error)

	// FindByName finds a calendar period by name within the current tenant
	FindByName(ctx context.Context, name string) (*schedulerecords.CalendarPeriod, error)

	// CreateIfAbsent inserts the period unless a row with the same
	// (tenant_id, name) already exists. It reports whether the insert
	// happened (false = a period with that name was already present).
	CreateIfAbsent(ctx context.Context, p *schedulerecords.CalendarPeriod) (bool, error)

	// FindActiveOverlapping returns all active periods of the current tenant
	// whose [start_date, end_date] range overlaps [start, end] (inclusive on
	// both ends), excluding the period with excludeID.
	FindActiveOverlapping(ctx context.Context, start, end calendar.Date, excludeID int64) ([]*schedulerecords.CalendarPeriod, error)

	// FindActiveOverlappingByType behaves like FindActiveOverlapping but only
	// considers active periods of the given period_type. It backs the hard
	// same-type overlap rejection; the untyped variant stays advisory.
	FindActiveOverlappingByType(ctx context.Context, periodType string, start, end calendar.Date, excludeID int64) ([]*schedulerecords.CalendarPeriod, error)

	// UsageCounts returns, per calendar period of the current tenant, how many
	// rows reference it through nullable calendar_period_id FKs. Periods without
	// references are omitted from the map.
	UsageCounts(ctx context.Context) (map[int64]schedulerecords.CalendarPeriodUsage, error)
}

type CareScheduleChangeRequestRecords interface {
	Create(ctx context.Context, req *schedulerecords.CareScheduleChangeRequest) error
	// FindByID takes `any` to match the embedded generic base.Repository.
	FindByID(ctx context.Context, id int64) (*schedulerecords.CareScheduleChangeRequest, error)

	// Decide moves a pending row to approved/rejected/withdrawn, stamping
	// decision_reason, reviewed_by, reviewed_at and (for approvals)
	// applied_at. Withdrawals carry no reviewer stamp.
	Decide(ctx context.Context, id int64, newStatus string, reason *string, reviewedBy *int64, applied bool) error
}

type ClassArrivalExceptionRecords interface {
	// Upsert stores the exception of one class and date, replacing what was
	// there.
	Upsert(ctx context.Context, row *schedulerecords.ClassArrivalException) error
}

type ClosingDayRecords interface {
	Create(context.Context, *schedulerecords.ClosingDay) error
	FindByID(context.Context, int64) (*schedulerecords.ClosingDay, error)
	Update(context.Context, *schedulerecords.ClosingDay) error
	Delete(context.Context, int64) error

	// FindByTenantID finds all closing days for the current tenant,
	// ordered by start_date.
	FindByTenantID(ctx context.Context) ([]*schedulerecords.ClosingDay, error)
}

type DateframeRecords interface {
	Create(context.Context, *schedulerecords.Dateframe) error
	FindByID(context.Context, int64) (*schedulerecords.Dateframe, error)
	Update(context.Context, *schedulerecords.Dateframe) error
	Delete(context.Context, int64) error

	// FindByName finds a dateframe by its name
	FindByName(ctx context.Context, name string) (*schedulerecords.Dateframe, error)
}

type StaffNoticeRecords interface {
	Create(ctx context.Context, n *schedulerecords.StaffNotice) error
	Update(ctx context.Context, n *schedulerecords.StaffNotice) error
	FindByID(ctx context.Context, id int64) (*schedulerecords.StaffNotice, error)
	Delete(ctx context.Context, id int64) error
	// DeleteEndedBefore löscht die Hinweise, deren letzter gültiger Tag vor
	// cutoff liegt oder die vor dessen Berliner Mitternacht ausgeschaltet
	// wurden (#4045), und gibt ihre Zahl zurück.
	DeleteEndedBefore(ctx context.Context, cutoff calendar.Date) (int, error)
	// List gibt die Hinweise des Mandanten zurück, neueste zuerst.
	// includeInactive steuert, ob abgeschaltete Zeilen erscheinen.
	List(ctx context.Context, includeInactive bool) ([]*schedulerecords.StaffNotice, error)
	// ListValidOn gibt die aktiven Hinweise zurück, deren Zeitraum den Tag
	// enthält und deren Zielgruppe die Leserart einschließt. Wochentag und
	// Wochenmuster prüft der Service — die Datenbank grenzt nur grob ein.
	ListValidOn(ctx context.Context, date calendar.Date, reader string) ([]*schedulerecords.StaffNotice, error)
	// Acknowledge stempelt die Kenntnisnahme einer Person; ein zweiter Aufruf
	// ändert nichts.
	Acknowledge(ctx context.Context, noticeID, accountID int64) error
	// AcknowledgedAtFor gibt je Hinweis-Id den Zeitpunkt der eigenen
	// Kenntnisnahme zurück.
	AcknowledgedAtFor(ctx context.Context, accountID int64, noticeIDs []int64) (map[int64]time.Time, error)
	// AcknowledgedCounts gibt je Hinweis-Id die Zahl der Kenntnisnahmen zurück.
	AcknowledgedCounts(ctx context.Context, noticeIDs []int64) (map[int64]int, error)
	// Acknowledgements gibt alle Kenntnisnahmen eines Hinweises zurück,
	// neueste zuerst — ohne Namen, die löst der Service auf.
	Acknowledgements(ctx context.Context, noticeID int64) ([]*schedulerecords.StaffNoticeAck, error)
}

type StudentArrivalExceptionRecords interface {
	Create(context.Context, *schedulerecords.StudentArrivalException) error
	FindByID(context.Context, int64) (*schedulerecords.StudentArrivalException, error)
	Update(context.Context, *schedulerecords.StudentArrivalException) error

	// FindByStudentID finds all arrival exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalException, error)

	// FindUpcomingByStudentID finds upcoming arrival exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalException, error)

	// FindByStudentIDAndDate finds an arrival exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) (*schedulerecords.StudentArrivalException, error)

	// FindByStudentIDsAndDate finds arrival exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentArrivalException, error)

	// FindByStudentIDAndDateRange finds arrival exceptions for a student whose
	// exception_date falls within the inclusive [from, to] range, sorted by
	// date. Used by the timetable per-student week endpoint to pre-load all
	// exceptions in a single query.
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*schedulerecords.StudentArrivalException, error)

	// DeleteByStudentID deletes all arrival exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

type StudentArrivalNoteRecords interface {
	Create(context.Context, *schedulerecords.StudentArrivalNote) error
	FindByID(context.Context, int64) (*schedulerecords.StudentArrivalNote, error)
	Update(context.Context, *schedulerecords.StudentArrivalNote) error

	// FindByStudentID finds all arrival notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalNote, error)

	// FindByStudentIDAndDate finds all arrival notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) ([]*schedulerecords.StudentArrivalNote, error)

	// FindByStudentIDsAndDate finds all arrival notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentArrivalNote, error)

	// DeleteByStudentID deletes all arrival notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

type StudentArrivalScheduleRecords interface {
	Create(context.Context, *schedulerecords.StudentArrivalSchedule) error
	FindByID(context.Context, int64) (*schedulerecords.StudentArrivalSchedule, error)
	Update(context.Context, *schedulerecords.StudentArrivalSchedule) error

	// FindByStudentID finds all arrival schedules for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentArrivalSchedule, error)

	// FindByStudentIDAndWeekday finds an arrival schedule for a specific student and weekday
	FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*schedulerecords.StudentArrivalSchedule, error)

	// FindByStudentIDsAndWeekday finds arrival schedules for multiple students and a specific weekday (bulk query)
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedulerecords.StudentArrivalSchedule, error)

	// UpsertSchedule creates or updates an arrival schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *schedulerecords.StudentArrivalSchedule) error

	// DeleteByStudentID deletes all arrival schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

type StudentPickupExceptionRecords interface {
	Create(context.Context, *schedulerecords.StudentPickupException) error
	FindByID(context.Context, int64) (*schedulerecords.StudentPickupException, error)
	Update(context.Context, *schedulerecords.StudentPickupException) error

	// FindByStudentID finds all pickup exceptions for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupException, error)

	// FindUpcomingByStudentID finds upcoming pickup exceptions for a student (from today onwards)
	FindUpcomingByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupException, error)

	// FindByStudentIDAndDate finds a pickup exception for a specific student and date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) (*schedulerecords.StudentPickupException, error)

	// FindByStudentIDsAndDate finds pickup exceptions for multiple students and a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentPickupException, error)

	// FindByStudentIDAndDateRange finds pickup exceptions for a student whose
	// exception_date falls within the inclusive [from, to] range, sorted by
	// date. Used by the timetable per-student week endpoint to pre-load all
	// exceptions in a single query.
	FindByStudentIDAndDateRange(ctx context.Context, studentID int64, from, to calendar.Date) ([]*schedulerecords.StudentPickupException, error)

	// DeleteByStudentID deletes all pickup exceptions for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

type StudentPickupNoteRecords interface {
	Create(context.Context, *schedulerecords.StudentPickupNote) error
	FindByID(context.Context, int64) (*schedulerecords.StudentPickupNote, error)
	Update(context.Context, *schedulerecords.StudentPickupNote) error

	// FindByStudentID finds all pickup notes for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupNote, error)

	// FindByStudentIDAndDate finds all pickup notes for a student on a specific date
	FindByStudentIDAndDate(ctx context.Context, studentID int64, date calendar.Date) ([]*schedulerecords.StudentPickupNote, error)

	// FindByStudentIDsAndDate finds all pickup notes for multiple students on a specific date (bulk query)
	FindByStudentIDsAndDate(ctx context.Context, studentIDs []int64, date calendar.Date) ([]*schedulerecords.StudentPickupNote, error)

	// DeleteByStudentID deletes all pickup notes for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

type StudentPickupScheduleRecords interface {
	Create(context.Context, *schedulerecords.StudentPickupSchedule) error
	FindByID(context.Context, int64) (*schedulerecords.StudentPickupSchedule, error)
	Update(context.Context, *schedulerecords.StudentPickupSchedule) error

	// FindByStudentID finds all pickup schedules for a student
	FindByStudentID(ctx context.Context, studentID int64) ([]*schedulerecords.StudentPickupSchedule, error)

	// FindByStudentIDAndWeekday finds a pickup schedule for a specific student and weekday
	FindByStudentIDAndWeekday(ctx context.Context, studentID int64, weekday int) (*schedulerecords.StudentPickupSchedule, error)

	// FindByStudentIDsAndWeekday finds pickup schedules for multiple students and a specific weekday (bulk query)
	FindByStudentIDsAndWeekday(ctx context.Context, studentIDs []int64, weekday int) ([]*schedulerecords.StudentPickupSchedule, error)

	// UpsertSchedule creates or updates a pickup schedule for a student and weekday
	UpsertSchedule(ctx context.Context, schedule *schedulerecords.StudentPickupSchedule) error

	// DeleteByStudentID deletes all pickup schedules for a student
	DeleteByStudentID(ctx context.Context, studentID int64) error
}

// ParticipantListingFilter pages planned rows before narrowing their execution status.
type ParticipantListingFilter struct {
	IDs         []int64
	InstanceIDs []int64
	StudentIDs  []int64
	Status      *string
	Limit       int
	Offset      int
}
