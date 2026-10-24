package staffoffboarding

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The frame of the school's time-tracking window (ADR 0048): the setting
// offers 2 to 8 years, and the retention refuses a value outside it rather
// than deleting on a wrong period. The database function of 1.15.459
// enforces the same frame.
const (
	MinRetentionDays = 730
	MaxRetentionDays = 2920
)

var (
	// ErrIsolationFailed marks a failure of the savepoint around one staff
	// member. The tenant transaction can no longer be trusted, so the run
	// stops.
	ErrIsolationFailed = errors.New("staff retention: isolating one staff member failed")
	// ErrPersonChanged refuses the anonymization of a person edited since it
	// was read; the staff member stays for the next run.
	ErrPersonChanged = errors.New("staff retention: person changed while it was anonymized")
	// errNoneDue ends the run: no staff member above the cursor is due.
	errNoneDue = errors.New("staff retention: no staff member is due")
)

// RetiredMembership is the School Membership half of the automatic deletion:
// it erases the next due tombstone above the cursor.
type RetiredMembership interface {
	EraseNextRetiredStaff(ctx context.Context, retiredBefore time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, error)
}

// LiveStaff tells whether a person is on the staff again under a newer
// membership.
type LiveStaff interface {
	FindStaffByPerson(ctx context.Context, personID int64) (schoolmembership.Staff, error)
}

// RetiredWorkforce erases the Workforce records of a retired staff member.
type RetiredWorkforce interface {
	EraseRetiredStaffRecords(ctx context.Context, staffID int64, retiredBefore time.Time) (map[string]int64, error)
}

// RetiredTimetable removes every remaining timetable assignment of retired
// staff. Historical instance assignments remain at ordinary offboarding but
// are personal data once the retention window expires.
type RetiredTimetable interface {
	DeleteRetiredStaffAssignments(ctx context.Context, staffID int64) (timetable.RetiredStaffAssignments, error)
}

// RetiredPeople anonymizes the person of a retired staff member.
type RetiredPeople interface {
	FindPersonForMutation(ctx context.Context, id int64) (peopledirectory.Person, error)
	ListStudentsByPersonID(ctx context.Context, personIDs []int64) ([]peopledirectory.Student, error)
	AnonymizeRetiredStaffPerson(ctx context.Context, personID int64, updatedAt time.Time) (bool, error)
}

// Erasure is what the automatic deletion removed for one staff member; the
// deletion log records it.
type Erasure struct {
	StaffID             int64
	Membership          schoolmembership.RetiredStaffErasure
	Workforce           map[string]int64
	Supervisions        int64
	PlannedSupervisions int64
	InstanceAssignments int64
	// PersonAnonymized is false when the person is on the staff again
	// under a newer membership, or was anonymized before.
	PersonAnonymized bool
}

// Counts are the deleted rows per record kind, the metadata of the deletion
// log entry.
func (e Erasure) Counts() map[string]int64 {
	counts := map[string]int64{
		"teachers": e.Membership.Teachers, "group_teacher": e.Membership.GroupAssignments,
		"external_caregivers": e.Membership.ExternalCaregiverRecords,
		"class_teachers":      e.Membership.ClassAssignments, "group_supervisors": e.Supervisions,
		"planned_supervisors":      e.PlannedSupervisions,
		"timetable_instance_staff": e.InstanceAssignments,
	}
	for kind, count := range e.Workforce {
		counts[kind] += count
	}
	return counts
}

// Records counts every deleted row and the anonymized person.
func (e Erasure) Records() int {
	records := 0
	if e.PersonAnonymized {
		records = 1
	}
	for _, count := range e.Counts() {
		records += int(count)
	}
	return records
}

// RetentionDependencies are the ports of the automatic deletion. Every one
// runs in the school's tenant transaction of the nightly cleanup.
type RetentionDependencies struct {
	// Window resolves the school's time-tracking window in days.
	Window func(context.Context) (int, error)
	// Tenant is the school of the tenant transaction in ctx.
	Tenant     func(context.Context) int64
	Today      func() calendar.Date
	Membership RetiredMembership
	LiveStaff  LiveStaff
	Workforce  RetiredWorkforce
	Timetable  RetiredTimetable
	// SupervisionIDs lists every supervision record of a staff member and
	// RemoveSupervision removes one (Student Presence).
	SupervisionIDs    func(ctx context.Context, staffID int64) ([]int64, error)
	RemoveSupervision func(ctx context.Context, id int64) error
	People            RetiredPeople
	// AppendAudit writes the deletion log entry of one erased staff member.
	AppendAudit func(context.Context, Erasure) error
	// Isolate runs one staff member's erasure in a savepoint, so a refused
	// staff member rolls back alone. A savepoint failure wraps
	// ErrIsolationFailed.
	Isolate func(context.Context, func(context.Context) error) error
	Logger  *slog.Logger
}

// Retention deletes, in the nightly cleanup of a school, every staff member
// whose exit lies more than the school's time-tracking window back (#3963).
// The exit is the moment the offboarding retired the membership. By then no
// time tracking of the person is kept, so nothing that still has to be kept
// hangs on them.
//
// The membership stays as an erased tombstone: check-ins, approvals, notes,
// appointments and other records of other people name it as their actor
// and do not cascade. Every record of the staff member is deleted by its
// owner, and the person is anonymized like a deleted child's.
type Retention struct{ deps RetentionDependencies }

// RetentionResult reports one run: the erased staff members, the refused
// ones the next run tries again, and the window applied.
type RetentionResult struct {
	Deleted       int
	Refused       int
	RetentionDays int
}

func NewRetention(deps RetentionDependencies) (*Retention, error) {
	if deps.Window == nil || deps.Tenant == nil || deps.Today == nil || deps.Membership == nil || deps.LiveStaff == nil ||
		deps.Workforce == nil || deps.Timetable == nil || deps.SupervisionIDs == nil || deps.RemoveSupervision == nil || deps.People == nil || deps.AppendAudit == nil || deps.Isolate == nil {
		return nil, errors.New("staff retention: all dependencies are required")
	}
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &Retention{deps: deps}, nil
}

// DeleteExpiredStaff erases, in ID order, the staff members of the school in
// ctx who left before the first Berlin day the window keeps. A staff member
// whose person was edited meanwhile is skipped and logged; it stays for the
// next run.
func (r *Retention) DeleteExpiredStaff(ctx context.Context) (RetentionResult, error) {
	days, err := r.deps.Window(ctx)
	if err != nil {
		return RetentionResult{}, fmt.Errorf("staff retention: resolve window: %w", err)
	}
	if days < MinRetentionDays || days > MaxRetentionDays {
		return RetentionResult{}, fmt.Errorf("staff retention: window of %d days is outside %d to %d", days, MinRetentionDays, MaxRetentionDays)
	}
	tenantID := r.deps.Tenant(ctx)
	if tenantID <= 0 {
		return RetentionResult{}, ErrUnauthorized
	}
	// The window counts whole Berlin days from the day after the exit
	// (docs/reference/loeschfristen.md), as the student retention does: with
	// 730 days, staff who left on 1 June 2026 go in the run on 1 June 2028,
	// whose cutoff is 2 June 2026 00:00 because 2028 has a 29 February. A
	// later cutoff would delete time tracking before the window has passed.
	retiredBefore := r.deps.Today().AddDays(-days).BerlinMidnight()
	result := RetentionResult{RetentionDays: days}
	// The cursor moves past every staff member taken, so a refused one,
	// whose savepoint rolled the marking back, is not taken again.
	for cursor := int64(0); ; {
		err := r.deps.Isolate(ctx, func(staffCtx context.Context) error {
			return r.eraseNext(staffCtx, retiredBefore, &cursor)
		})
		switch {
		case errors.Is(err, errNoneDue):
			return result, nil
		case err == nil:
			result.Deleted++
		case errors.Is(err, ErrIsolationFailed) || ctx.Err() != nil:
			return RetentionResult{}, err
		case errors.Is(err, ErrPersonChanged):
			result.Refused++
			r.deps.Logger.Warn("staff retention skipped a staff member",
				"tenant_id", tenantID,
				"staff_id", cursor,
				"error", err.Error(),
			)
		default:
			return RetentionResult{}, fmt.Errorf("staff retention: erase staff %d: %w", cursor, err)
		}
	}
}

// eraseNext takes the next due staff member above the cursor and moves the
// cursor to it. School Membership marks the tombstone first and locks it,
// so a concurrent run never takes the same staff member.
func (r *Retention) eraseNext(ctx context.Context, retiredBefore time.Time, cursor *int64) error {
	membership, found, err := r.deps.Membership.EraseNextRetiredStaff(ctx, retiredBefore, *cursor)
	if err != nil {
		return err
	}
	if !found {
		return errNoneDue
	}
	*cursor = membership.StaffID
	erasure := Erasure{StaffID: membership.StaffID, Membership: membership}
	if erasure.Workforce, err = r.deps.Workforce.EraseRetiredStaffRecords(ctx, erasure.StaffID, retiredBefore); err != nil {
		return err
	}
	timetableAssignments, err := r.deps.Timetable.DeleteRetiredStaffAssignments(ctx, erasure.StaffID)
	if err != nil {
		return err
	}
	erasure.PlannedSupervisions = timetableAssignments.PlannedSupervisors
	erasure.InstanceAssignments = timetableAssignments.InstanceStaff
	if erasure.Supervisions, err = r.removeSupervisions(ctx, erasure.StaffID); err != nil {
		return err
	}
	if erasure.PersonAnonymized, err = r.anonymize(ctx, membership.PersonID); err != nil {
		return err
	}
	return r.deps.AppendAudit(ctx, erasure)
}

func (r *Retention) removeSupervisions(ctx context.Context, staffID int64) (int64, error) {
	ids, err := r.deps.SupervisionIDs(ctx, staffID)
	if err != nil {
		return 0, err
	}
	for _, id := range ids {
		if err := r.deps.RemoveSupervision(ctx, id); err != nil {
			return 0, err
		}
	}
	return int64(len(ids)), nil
}

// anonymize leaves a person alone who is on the staff again under a newer
// membership, still has a student membership, or was anonymized by an earlier
// membership's erasure.
func (r *Retention) anonymize(ctx context.Context, personID int64) (bool, error) {
	person, err := r.deps.People.FindPersonForMutation(ctx, personID)
	if errors.Is(err, peopledirectory.ErrPersonNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if person.DeletedAt != nil {
		return false, nil
	}
	// Lock the person before checking for a newer membership. A rehire locks
	// this row too, a new membership through its foreign key and a reinstated
	// one explicitly, so it cannot complete between this final check and the
	// anonymization. A rehire committed while the other owners
	// erased the old membership is therefore visible here.
	_, err = r.deps.LiveStaff.FindStaffByPerson(ctx, personID)
	if err == nil {
		return false, nil
	}
	if !errors.Is(err, schoolmembership.ErrStaffNotFound) {
		return false, err
	}
	// A person can also be a student in the same school. Keep that identity
	// until the student retention workflow has removed its own membership.
	// The person lock above prevents a concurrent student profile from being
	// attached between this check and the anonymization.
	students, err := r.deps.People.ListStudentsByPersonID(ctx, []int64{personID})
	if err != nil {
		return false, err
	}
	if len(students) > 0 {
		return false, nil
	}
	anonymized, err := r.deps.People.AnonymizeRetiredStaffPerson(ctx, person.ID, person.UpdatedAt)
	if err != nil {
		return false, err
	}
	if !anonymized {
		return false, ErrPersonChanged
	}
	return true, nil
}
