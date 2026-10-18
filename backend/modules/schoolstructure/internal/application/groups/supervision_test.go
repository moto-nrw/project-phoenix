package groups

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// supervisionReads answers the owner reads of the supervision rule from
// fixed tables and records what was asked.
type supervisionReads struct {
	teacherStaff map[int64]int64
	live         map[int64]bool
	substitutes  []domain.StaffGroupID
	asked        []string
	failTeachers error
	failSubs     error
}

func (r *supervisionReads) sources() SupervisionSources {
	return SupervisionSources{
		TeacherStaffIDs: func(_ context.Context, ids []int64) (map[int64]int64, error) {
			r.asked = append(r.asked, "teachers")
			if r.failTeachers != nil {
				return nil, r.failTeachers
			}
			result := map[int64]int64{}
			for _, id := range ids {
				if staffID, ok := r.teacherStaff[id]; ok {
					result[id] = staffID
				}
			}
			return result, nil
		},
		LiveStaffIDs: func(_ context.Context, ids []int64) (map[int64]bool, error) {
			r.asked = append(r.asked, "staff")
			result := map[int64]bool{}
			for _, id := range ids {
				if r.live[id] {
					result[id] = true
				}
			}
			return result, nil
		},
		GroupSubstitutes: func(_ context.Context, groupIDs []int64, on string) ([]domain.StaffGroupID, error) {
			r.asked = append(r.asked, "substitutes "+on)
			return r.substitutes, r.failSubs
		},
	}
}

func TestSupervisingStaffJoinsLiveTeachersAndLiveSubstitutes(t *testing.T) {
	t.Parallel()
	reads := &supervisionReads{
		// Teacher 11 is gone, teacher 12's staff member is offboarded.
		teacherStaff: map[int64]int64{10: 100, 12: 120, 13: 100},
		live:         map[int64]bool{100: true, 200: true},
		substitutes:  []domain.StaffGroupID{{StaffID: 200, GroupID: 1}, {StaffID: 300, GroupID: 2}, {StaffID: 100, GroupID: 1}},
	}
	resolve := SupervisingStaff(reads.sources())
	require.NotNil(t, resolve)

	pairs, err := resolve(context.Background(), domain.GroupMembershipPairs{
		Assigned: []domain.TeacherGroupID{
			{TeacherID: 10, GroupID: 1}, {TeacherID: 11, GroupID: 1}, {TeacherID: 12, GroupID: 2},
			{TeacherID: 13, GroupID: 2}, {TeacherID: 0, GroupID: 3},
		},
		GroupIDs: []int64{1, 2}, On: calendar.Date("2026-10-19"),
	})
	require.NoError(t, err)
	assert.Equal(t, []domain.StaffGroupID{
		{StaffID: 100, GroupID: 1}, {StaffID: 100, GroupID: 2}, {StaffID: 200, GroupID: 1},
	}, pairs, "assignments first, then substitutes, without the gone, the offboarded and the duplicate")
	assert.Equal(t, []string{"teachers", "staff", "substitutes 2026-10-19", "staff"}, reads.asked)
}

func TestSupervisingStaffAsksNoOwnerForEmptyReferences(t *testing.T) {
	t.Parallel()
	reads := &supervisionReads{}
	pairs, err := SupervisingStaff(reads.sources())(context.Background(), domain.GroupMembershipPairs{})
	require.NoError(t, err)
	assert.Empty(t, pairs)
	assert.Equal(t, []string{"staff"}, reads.asked, "no teacher and no substitution read without assignments or groups")
}

func TestSupervisingStaffReportsMissingBindingsAndFailedReads(t *testing.T) {
	t.Parallel()
	assert.Nil(t, SupervisingStaff(SupervisionSources{}), "without School Membership the repository reports the missing binding")

	reads := &supervisionReads{}
	sources := reads.sources()
	sources.GroupSubstitutes = nil
	_, err := SupervisingStaff(sources)(context.Background(), domain.GroupMembershipPairs{GroupIDs: []int64{1}})
	require.EqualError(t, err, "group supervision resolves substitutions through Workforce")

	refused := errors.New("refused")
	reads = &supervisionReads{failTeachers: refused}
	_, err = SupervisingStaff(reads.sources())(context.Background(), domain.GroupMembershipPairs{Assigned: []domain.TeacherGroupID{{TeacherID: 1, GroupID: 1}}})
	require.ErrorIs(t, err, refused)
	assert.Contains(t, err.Error(), "load teachers of group assignments")

	reads = &supervisionReads{failSubs: refused}
	_, err = SupervisingStaff(reads.sources())(context.Background(), domain.GroupMembershipPairs{GroupIDs: []int64{1}})
	require.ErrorIs(t, err, refused)
	assert.Contains(t, err.Error(), "load substitutions of group supervision")
}
