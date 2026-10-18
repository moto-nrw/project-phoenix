package staffoffboarding_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/workflows/staffoffboarding"
)

type liveStaffMembershipStub struct {
	calls *[]string
	err   error
}

func (m liveStaffMembershipStub) WithLiveStaff(ctx context.Context, _ int64, write func(context.Context) error) error {
	*m.calls = append(*m.calls, "lock")
	if m.err != nil {
		return m.err
	}
	return write(ctx)
}

type staffEmploymentWritesStub struct {
	calls *[]string
	err   error
}

func (e staffEmploymentWritesStub) AppendStaffNotes(context.Context, int64, string) (workforce.StaffEmployment, error) {
	*e.calls = append(*e.calls, "notes")
	return workforce.StaffEmployment{}, e.err
}

func (e staffEmploymentWritesStub) SetStaffBirthdayDisplayOptOut(context.Context, int64, bool) error {
	*e.calls = append(*e.calls, "opt-out")
	return e.err
}

func TestLiveStaffEmploymentWritesInsideTheMembershipLock(t *testing.T) {
	t.Parallel()
	var calls []string
	employment, err := staffoffboarding.NewLiveStaffEmployment(liveStaffMembershipStub{calls: &calls}, staffEmploymentWritesStub{calls: &calls})
	require.NoError(t, err)

	require.NoError(t, employment.SetBirthdayDisplayOptOut(t.Context(), 0, true))
	require.NoError(t, employment.AppendNotes(t.Context(), 0, "Notiz"))
	assert.Equal(t, []string{"lock", "opt-out", "lock", "notes"}, calls)
}

func TestLiveStaffEmploymentSkipsTheWriteForAFormerMember(t *testing.T) {
	t.Parallel()
	var calls []string
	gone := errors.New("staff not found")
	employment, err := staffoffboarding.NewLiveStaffEmployment(liveStaffMembershipStub{calls: &calls, err: gone}, staffEmploymentWritesStub{calls: &calls})
	require.NoError(t, err)

	assert.ErrorIs(t, employment.SetBirthdayDisplayOptOut(t.Context(), 0, true), gone)
	assert.ErrorIs(t, employment.AppendNotes(t.Context(), 0, "Notiz"), gone)
	assert.Equal(t, []string{"lock", "lock"}, calls)
}

func TestLiveStaffEmploymentPassesTheWorkforceErrorThrough(t *testing.T) {
	t.Parallel()
	var calls []string
	employment, err := staffoffboarding.NewLiveStaffEmployment(liveStaffMembershipStub{calls: &calls}, staffEmploymentWritesStub{calls: &calls, err: workforce.ErrStaffEmploymentNotFound})
	require.NoError(t, err)

	assert.ErrorIs(t, employment.AppendNotes(t.Context(), 0, "Notiz"), workforce.ErrStaffEmploymentNotFound)
	_, err = staffoffboarding.NewLiveStaffEmployment(nil, staffEmploymentWritesStub{calls: &calls})
	assert.Error(t, err)
}
