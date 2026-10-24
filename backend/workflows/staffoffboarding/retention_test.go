package staffoffboarding_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// retiredDoubles stands in for the owners of the automatic deletion: due
// lists the staff members School Membership hands out in ID order.
type retiredDoubles struct {
	due                           []int64
	cursors                       []int64
	refuse                        map[int64]bool
	failStaff                     map[int64]error
	plannedSupervisions           map[int64]int64
	instanceAssignments           map[int64]int64
	retiredBefore                 []time.Time
	logged                        []int64
	live                          map[int64]bool
	student                       map[int64]bool
	rehireBeforeMembershipRecheck map[int64]bool
	anonymizationCalls            int
}

func (d *retiredDoubles) EraseNextRetiredStaff(_ context.Context, retiredBefore time.Time, afterID int64) (schoolmembership.RetiredStaffErasure, bool, error) {
	d.retiredBefore = append(d.retiredBefore, retiredBefore)
	d.cursors = append(d.cursors, afterID)
	for _, id := range d.due {
		if id > afterID {
			return schoolmembership.RetiredStaffErasure{StaffID: id, PersonID: id * 10}, true, nil
		}
	}
	return schoolmembership.RetiredStaffErasure{}, false, nil
}

func (d *retiredDoubles) FindStaffByPerson(_ context.Context, personID int64) (schoolmembership.Staff, error) {
	if d.live[personID] {
		return schoolmembership.Staff{PersonID: personID}, nil
	}
	return schoolmembership.Staff{}, schoolmembership.ErrStaffNotFound
}

func (d *retiredDoubles) EraseRetiredStaffRecords(_ context.Context, staffID int64, _ time.Time) (map[string]int64, error) {
	return map[string]int64{"master_data": 1}, d.failStaff[staffID]
}

func (d *retiredDoubles) DeleteRetiredStaffAssignments(_ context.Context, staffID int64) (timetable.RetiredStaffAssignments, error) {
	return timetable.RetiredStaffAssignments{PlannedSupervisors: d.plannedSupervisions[staffID], InstanceStaff: d.instanceAssignments[staffID]}, nil
}

func (d *retiredDoubles) FindPersonForMutation(_ context.Context, id int64) (peopledirectory.Person, error) {
	if d.rehireBeforeMembershipRecheck[id] {
		d.live[id] = true
	}
	return peopledirectory.Person{ID: id, UpdatedAt: time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)}, nil
}

func (d *retiredDoubles) ListStudentsByPersonID(_ context.Context, personIDs []int64) ([]peopledirectory.Student, error) {
	if d.student[personIDs[0]] {
		return []peopledirectory.Student{{PersonID: personIDs[0]}}, nil
	}
	return nil, nil
}

func (d *retiredDoubles) AnonymizeRetiredStaffPerson(_ context.Context, personID int64, _ time.Time) (bool, error) {
	d.anonymizationCalls++
	return !d.refuse[personID/10], nil
}

func newRetentionDoubles(t *testing.T, doubles *retiredDoubles, window int, isolate func(context.Context, func(context.Context) error) error) *staffoffboarding.Retention {
	t.Helper()
	if isolate == nil {
		isolate = func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
	}
	retention, err := staffoffboarding.NewRetention(staffoffboarding.RetentionDependencies{
		Window:            func(context.Context) (int, error) { return window, nil },
		Tenant:            func(context.Context) int64 { return 7 },
		Today:             func() calendar.Date { return calendar.NewDate(2028, time.June, 1) },
		Membership:        doubles,
		LiveStaff:         doubles,
		Workforce:         doubles,
		Timetable:         doubles,
		SupervisionIDs:    func(context.Context, int64) ([]int64, error) { return []int64{1}, nil },
		RemoveSupervision: func(context.Context, int64) error { return nil },
		People:            doubles,
		AppendAudit: func(_ context.Context, erasure staffoffboarding.Erasure) error {
			doubles.logged = append(doubles.logged, erasure.StaffID)
			return nil
		},
		Isolate: isolate,
	})
	require.NoError(t, err)
	return retention
}

func TestStaffRetentionSkipsARefusedPersonAndGoesOn(t *testing.T) {
	t.Parallel()
	doubles := &retiredDoubles{due: []int64{3, 5, 8}, refuse: map[int64]bool{5: true}}

	result, err := newRetentionDoubles(t, doubles, 730, nil).DeleteExpiredStaff(context.Background())

	require.NoError(t, err)
	assert.Equal(t, staffoffboarding.RetentionResult{Deleted: 2, Refused: 1, RetentionDays: 730}, result)
	assert.Equal(t, []int64{3, 8}, doubles.logged, "a refused staff member is not logged")
	assert.Equal(t, []int64{0, 3, 5, 8}, doubles.cursors, "the cursor moves past every staff member taken, refused ones included")
}

func TestStaffRetentionDoesNotAnonymizeAPersonRehiredBeforeTheFinalMembershipCheck(t *testing.T) {
	t.Parallel()
	doubles := &retiredDoubles{
		due:                           []int64{3},
		live:                          map[int64]bool{},
		rehireBeforeMembershipRecheck: map[int64]bool{30: true},
	}

	result, err := newRetentionDoubles(t, doubles, 730, nil).DeleteExpiredStaff(context.Background())

	require.NoError(t, err)
	assert.Equal(t, staffoffboarding.RetentionResult{Deleted: 1, RetentionDays: 730}, result)
	assert.Zero(t, doubles.anonymizationCalls, "the locked membership check keeps a rehired person's identity")
}

func TestStaffRetentionDoesNotAnonymizeAPersonWithAStudentMembership(t *testing.T) {
	t.Parallel()
	doubles := &retiredDoubles{
		due:     []int64{3},
		student: map[int64]bool{30: true},
	}

	result, err := newRetentionDoubles(t, doubles, 730, nil).DeleteExpiredStaff(context.Background())

	require.NoError(t, err)
	assert.Equal(t, staffoffboarding.RetentionResult{Deleted: 1, RetentionDays: 730}, result)
	assert.Zero(t, doubles.anonymizationCalls, "the student membership keeps the shared identity")
}

func TestStaffRetentionCountsTheWindowFromTheDayAfterTheExit(t *testing.T) {
	t.Parallel()
	doubles := &retiredDoubles{due: []int64{3}}

	_, err := newRetentionDoubles(t, doubles, 730, nil).DeleteExpiredStaff(context.Background())

	require.NoError(t, err)
	require.NotEmpty(t, doubles.retiredBefore)
	assert.Equal(t, calendar.NewDate(2026, time.June, 2).BerlinMidnight(), doubles.retiredBefore[0],
		"an exit on 1 June 2026 is due in the run on 1 June 2028, an exit on 2 June 2026 is not")
}

func TestStaffRetentionCountsTheRecordsOfOneErasure(t *testing.T) {
	t.Parallel()
	erasure := staffoffboarding.Erasure{
		StaffID:             3,
		Membership:          schoolmembership.RetiredStaffErasure{Teachers: 1, GroupAssignments: 2, ClassAssignments: 2},
		Workforce:           map[string]int64{"master_data": 1, "documents": 4},
		Supervisions:        3,
		PlannedSupervisions: 2,
		InstanceAssignments: 2,
		PersonAnonymized:    true,
	}

	assert.Equal(t, map[string]int64{
		"teachers": 1, "group_teacher": 2, "external_caregivers": 0, "class_teachers": 2, "group_supervisors": 3, "planned_supervisors": 2, "timetable_instance_staff": 2, "master_data": 1, "documents": 4,
	}, erasure.Counts())
	assert.Equal(t, 18, erasure.Records(), "every deleted row and the anonymized person")
}

func TestStaffRetentionStopsWhenTheSavepointFails(t *testing.T) {
	t.Parallel()
	doubles := &retiredDoubles{due: []int64{3, 5}}
	broken := errors.Join(staffoffboarding.ErrIsolationFailed, errors.New("savepoint lost"))
	isolate := func(context.Context, func(context.Context) error) error { return broken }

	_, err := newRetentionDoubles(t, doubles, 730, isolate).DeleteExpiredStaff(context.Background())

	require.ErrorIs(t, err, staffoffboarding.ErrIsolationFailed)
	assert.Empty(t, doubles.logged)
}

func TestStaffRetentionStopsOnAnUnexpectedOwnerError(t *testing.T) {
	t.Parallel()
	failure := errors.New("workforce unavailable")
	doubles := &retiredDoubles{due: []int64{3, 5}, failStaff: map[int64]error{3: failure}}

	_, err := newRetentionDoubles(t, doubles, 730, nil).DeleteExpiredStaff(context.Background())

	require.ErrorIs(t, err, failure, "an error that is no refusal fails the run so the scheduler retries it")
	assert.Empty(t, doubles.logged)
	assert.Equal(t, []int64{0}, doubles.cursors, "no staff member after the failure is taken")
}

func TestStaffRetentionRefusesAWindowOutsideTheFrame(t *testing.T) {
	t.Parallel()
	for _, window := range []int{staffoffboarding.MinRetentionDays - 1, staffoffboarding.MaxRetentionDays + 1} {
		doubles := &retiredDoubles{due: []int64{3}}

		_, err := newRetentionDoubles(t, doubles, window, nil).DeleteExpiredStaff(context.Background())

		require.ErrorContains(t, err, "outside 730 to 2920")
		assert.Empty(t, doubles.cursors, "nothing is erased on a wrong period")
	}
}
