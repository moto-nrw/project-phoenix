package active

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/models/base"
)

// Attendance represents attendance tracking for RFID check-ins/outs
type Attendance struct {
	base.Model `bun:"schema:active,table:attendance"`
	base.TenantModel
	StudentID    int64         `bun:"student_id,notnull" json:"student_id"`
	Date         timezone.Date `bun:"date,notnull,type:date" json:"date"`
	CheckInTime  time.Time     `bun:"check_in_time,notnull" json:"check_in_time"`
	CheckOutTime *time.Time    `bun:"check_out_time" json:"check_out_time,omitempty"`
	// CheckedInBy is zero when an authenticated device records attendance
	// without a verified personal staff credential. DeviceID remains the
	// authenticated audit principal for those legacy kiosk requests.
	CheckedInBy  int64  `bun:"checked_in_by,nullzero" json:"checked_in_by"`
	CheckedOutBy *int64 `bun:"checked_out_by" json:"checked_out_by,omitempty"`
	DeviceID     int64  `bun:"device_id,notnull" json:"device_id"`
	// YardSince is set when the student transitions to "Schulhof" in binary
	// mode and cleared on a school checkout. Schema and read paths
	// (deriveAttendanceStatus, ResolveBinaryLocation, performCheckOut) are
	// in place, but the *write* originates from PyrePortal's "Schulhof"
	// kiosk endpoint, which is tracked separately. Until that ships,
	// `on_yard` is unreachable in production. See the cross-repo PyrePortal
	// ticket linked in the PR description.
	YardSince *time.Time `bun:"yard_since" json:"yard_since,omitempty"`
}

// IsOnYard returns true if the student is currently marked as being on the school yard
// (still on premises, outside the building). Only meaningful while IsCheckedIn is also true.
func (a *Attendance) IsOnYard() bool {
	return a.YardSince != nil && a.CheckOutTime == nil
}

// IsCheckedIn returns true if the student is currently checked in
func (a *Attendance) IsCheckedIn() bool {
	return a.CheckOutTime == nil
}

// AttendanceRepository defines the interface for attendance data operations
type AttendanceRepository interface {
	// LockStudentAttendance serializes attendance-sensitive decisions for one
	// student within the current transaction.
	LockStudentAttendance(ctx context.Context, studentID int64) error

	// Create creates a new attendance record
	Create(ctx context.Context, attendance *Attendance) error

	// CreateIfNoOpenForToday inserts the attendance row using ON CONFLICT
	// against the partial unique index on (student_id, date) WHERE
	// check_out_time IS NULL (migration 1.15.42). Returns inserted=true when
	// the row was written, false when the conflict path swallowed the insert
	// (a concurrent caller already opened today's attendance for the student).
	CreateIfNoOpenForToday(ctx context.Context, attendance *Attendance) (bool, error)

	// Update updates an existing attendance record
	Update(ctx context.Context, attendance *Attendance) error

	// CloseOpenForToday closes the currently-open attendance row for the
	// given student on the given calendar day via a state-checked UPDATE
	// (WHERE check_out_time IS NULL). The caller supplies the day instead of
	// the repository re-deriving it: single checkouts pass the day of their
	// own now, batch checkouts pass their batch-wide snapshot date so a batch
	// crossing Berlin midnight closes every item on the same day it read and
	// refreshes (review #2372). Returns the updated row when an open row was
	// actually closed, nil when no open row existed (e.g. student was never
	// checked in or another concurrent caller already closed it). The caller
	// treats both cases as successful idempotent checkouts.
	CloseOpenForToday(ctx context.Context, studentID int64, now time.Time, today timezone.Date, staffID int64) (*Attendance, error)

	// CreateIfNoOpenForTodayBatch is the multi-row form of
	// CreateIfNoOpenForToday: one INSERT … ON CONFLICT DO NOTHING for the
	// whole batch. Returns the student ids whose row was actually inserted;
	// conflicting students are absorbed like the single-row method. The
	// caller must hold the students' row locks in ascending id order.
	CreateIfNoOpenForTodayBatch(ctx context.Context, rows []*Attendance) ([]int64, error)

	// CloseOpenForDateByStudentIDs is the multi-student form of
	// CloseOpenForToday: one state-checked UPDATE over the whole batch,
	// returning only the rows actually closed. Students without an open row
	// on the date are idempotent no-ops and missing from the result.
	CloseOpenForDateByStudentIDs(ctx context.Context, studentIDs []int64, now time.Time, day timezone.Date, staffID int64) ([]*Attendance, error)

	// FindByID finds an attendance record by ID
	FindByID(ctx context.Context, id int64) (*Attendance, error)

	// HasOpenAttendanceOn reports whether any attendance row on the given
	// calendar date is still open (check_out_time IS NULL). Used by the
	// operator presence-mode switch guard.
	HasOpenAttendanceOn(ctx context.Context, date timezone.Date) (bool, error)

	// HasAnyInRange reports whether any attendance row of the current tenant
	// exists between the two dates (inclusive), regardless of student. Used
	// as the "school records attendance at all" signal by the parent
	// today-status derivation.
	HasAnyInRange(ctx context.Context, startDate, endDate timezone.Date) (bool, error)

	// FindByStudentAndDate finds all attendance records for a student on a specific date
	FindByStudentAndDate(ctx context.Context, studentID int64, date timezone.Date) ([]*Attendance, error)

	// FindByStudentAndDateRange finds all attendance records for a student between two
	// dates (inclusive), ordered by date descending then check_in_time descending.
	FindByStudentAndDateRange(ctx context.Context, studentID int64, startDate, endDate timezone.Date) ([]*Attendance, error)

	// FindLatestByStudent finds the most recent attendance record for a student
	FindLatestByStudent(ctx context.Context, studentID int64) (*Attendance, error)

	// GetStudentCurrentStatus gets the current check-in status for a student
	GetStudentCurrentStatus(ctx context.Context, studentID int64) (*Attendance, error)

	// Delete deletes an attendance record
	Delete(ctx context.Context, id int64) error

	// GetTodayByStudentID gets today's attendance record for a student
	GetTodayByStudentID(ctx context.Context, studentID int64) (*Attendance, error)

	// GetTodayByStudentIDs gets today's attendance record for multiple students
	GetTodayByStudentIDs(ctx context.Context, studentIDs []int64) (map[int64]*Attendance, error)

	// GetOpenTodayByStudentIDsForUpdate gets today's open attendance rows for
	// multiple students and locks them for the current transaction.
	GetOpenTodayByStudentIDsForUpdate(ctx context.Context, studentIDs []int64) (map[int64]*Attendance, error)

	// FindForDate finds all attendance records for a specific date
	FindForDate(ctx context.Context, date timezone.Date) ([]*Attendance, error)

	// FindForDateByStudentIDs finds attendance records for a specific date and
	// the supplied students. An empty student list returns no rows.
	FindForDateByStudentIDs(ctx context.Context, date timezone.Date, studentIDs []int64) ([]*Attendance, error)

	// ListOpenStudentIDsForDate returns unique student IDs with an open
	// attendance row for the given date.
	ListOpenStudentIDsForDate(ctx context.Context, date timezone.Date) ([]int64, error)

	// FindStaleOpen returns attendance rows dated before the given day that
	// still lack a check-out time. Feeds the nightly stale-attendance
	// cleanup and its preview.
	FindStaleOpen(ctx context.Context, before timezone.Date) ([]*Attendance, error)

	// UpdateColumns is the generic partial-update helper promoted from the
	// embedded base repository: updates only the named columns by primary
	// key and returns the number of rows affected.
	UpdateColumns(ctx context.Context, attendance *Attendance, columns ...string) (int64, error)

	// CountByStaffID counts attendance records where the staff member checked in or checked out students
	CountByStaffID(ctx context.Context, staffID int64) (int, error)
}
