package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolsetup"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	setupTenant    = 41
	setupAdmin     = 73
	setupColleague = 74
)

// fakeSchoolSetupStore keeps the state of one school in memory. The tenant is
// implicit, as it is in the repository, where RLS supplies it.
type fakeSchoolSetupStore struct {
	setup     *schoolsetup.State
	dismissed map[int64]bool
	upserts   int
}

func newFakeSchoolSetupStore() *fakeSchoolSetupStore {
	return &fakeSchoolSetupStore{dismissed: map[int64]bool{}}
}

func (f *fakeSchoolSetupStore) Find(context.Context) (*schoolsetup.State, error) {
	if f.setup == nil {
		return nil, nil
	}
	copied := *f.setup
	copied.SkippedSteps = append([]string{}, f.setup.SkippedSteps...)
	return &copied, nil
}

func (f *fakeSchoolSetupStore) Upsert(_ context.Context, setup *schoolsetup.State) error {
	f.upserts++
	copied := *setup
	f.setup = &copied
	return nil
}

func (f *fakeSchoolSetupStore) IsDismissed(_ context.Context, accountID int64) (bool, error) {
	return f.dismissed[accountID], nil
}

func (f *fakeSchoolSetupStore) SetDismissed(_ context.Context, _, accountID int64, dismissed bool) error {
	f.dismissed[accountID] = dismissed
	return nil
}

type fakeSetupProgress struct {
	facts schoolsetup.Facts
	reads int
}

func (f *fakeSetupProgress) Facts(context.Context, int64) (schoolsetup.Facts, error) {
	f.reads++
	return f.facts, nil
}

type fakeSetupSettings struct {
	presenceMode     string
	groupMode        string
	timetableEnabled bool
}

func (f *fakeSetupSettings) PresenceMode(context.Context) (string, error) {
	return f.presenceMode, nil
}

func (f *fakeSetupSettings) GroupMode(context.Context) (string, error) {
	return f.groupMode, nil
}

func (f *fakeSetupSettings) TimetableEnabled(context.Context) (bool, error) {
	return f.timetableEnabled, nil
}

type setupFixture struct {
	service  *Service
	store    *fakeSchoolSetupStore
	progress *fakeSetupProgress
	settings *fakeSetupSettings
	written  []string
	writeErr error
	now      time.Time
}

func newSetupFixture(t *testing.T) *setupFixture {
	t.Helper()
	f := &setupFixture{
		store:    newFakeSchoolSetupStore(),
		progress: &fakeSetupProgress{},
		settings: &fakeSetupSettings{
			presenceMode:     schoolsetup.PresenceModeDetailed,
			groupMode:        schoolsetup.GroupModeFixedGroups,
			timetableEnabled: true,
		},
		now: time.Date(2026, time.September, 22, 9, 30, 0, 0, time.UTC),
	}
	presence := func(_ context.Context, tenantID, accountID int64, mode string) error {
		assert.Equal(t, int64(setupTenant), tenantID)
		assert.Equal(t, int64(setupAdmin), accountID)
		if f.writeErr != nil {
			return f.writeErr
		}
		f.written = append(f.written, mode)
		f.settings.presenceMode = mode
		return nil
	}
	service, err := New(f.store, f.progress, f.settings, presence, func() time.Time { return f.now })
	require.NoError(t, err)
	f.service = service
	return f
}

func (f *setupFixture) status(t *testing.T) schoolsetup.Status {
	t.Helper()
	status, err := f.service.Status(context.Background(), setupTenant, setupAdmin)
	require.NoError(t, err)
	return status
}

func stepByKey(t *testing.T, status schoolsetup.Status, step schoolsetup.StepKey) schoolsetup.Step {
	t.Helper()
	for _, candidate := range status.Steps {
		if candidate.Key == string(step) {
			return candidate
		}
	}
	t.Fatalf("step %q missing", step)
	return schoolsetup.Step{}
}

func TestNewRequiresEveryDependency(t *testing.T) {
	t.Parallel()
	_, err := New(nil, &fakeSetupProgress{}, &fakeSetupSettings{}, func(context.Context, int64, int64, string) error { return nil }, time.Now)
	require.Error(t, err)
}

func TestStatusOfANewSchool(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)

	status := f.status(t)

	assert.False(t, status.Completed)
	assert.False(t, status.Dismissed)
	assert.Nil(t, status.Basics.ParentAppUsed)
	keys := make([]string, 0, len(status.Steps))
	for _, step := range status.Steps {
		keys = append(keys, step.Key)
		assert.True(t, step.Applies, "every step applies to a detailed, fixed-group school before it answered: %s", step.Key)
		assert.False(t, step.Done, step.Key)
		assert.False(t, step.Skipped, step.Key)
	}
	assert.Equal(t, []string{"basics", "team", "rooms", "groups", "students", "guardians"}, keys)
}

func TestSchoolSetupStepsFollowTheSchoolsAnswers(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)
	f.settings.groupMode = "open_care"

	require.NoError(t, f.service.ConfirmBasics(context.Background(), setupTenant, setupAdmin, schoolsetup.PresenceModeBinary, false))
	status := f.status(t)

	assert.True(t, stepByKey(t, status, schoolsetup.StepBasics).Done)
	assert.False(t, stepByKey(t, status, schoolsetup.StepRooms).Applies, "rooms only apply when the school tracks rooms")
	assert.False(t, stepByKey(t, status, schoolsetup.StepGroups).Applies, "groups only apply to fixed groups")
	assert.False(t, stepByKey(t, status, schoolsetup.StepGuardians).Applies, "no parent app, no parent step")
	assert.Equal(t, schoolsetup.PresenceModeBinary, status.Basics.PresenceMode)
	require.NotNil(t, status.Basics.ParentAppUsed)
	assert.False(t, *status.Basics.ParentAppUsed)
}

func TestSchoolSetupStepsAreDoneFromTheProjection(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)
	f.progress.facts = schoolsetup.Facts{StaffInvited: true, RoomCreated: true, GroupCreated: true, StudentEnrolled: true, GuardianInvited: true}

	status := f.status(t)

	for _, step := range []schoolsetup.StepKey{schoolsetup.StepTeam, schoolsetup.StepRooms, schoolsetup.StepGroups, schoolsetup.StepStudents, schoolsetup.StepGuardians} {
		assert.True(t, stepByKey(t, status, step).Done, step)
	}
	assert.False(t, stepByKey(t, status, schoolsetup.StepBasics).Done, "the first step is only done once confirmed")
}

func TestSchoolSetupConfirmBasicsWritesThePresenceModeOnlyOnChange(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)

	require.NoError(t, f.service.ConfirmBasics(context.Background(), setupTenant, setupAdmin, schoolsetup.PresenceModeDetailed, true))
	assert.Empty(t, f.written, "an unchanged presence mode is not rewritten")

	require.NoError(t, f.service.ConfirmBasics(context.Background(), setupTenant, setupAdmin, schoolsetup.PresenceModeBinary, true))
	assert.Equal(t, []string{schoolsetup.PresenceModeBinary}, f.written)
	require.NotNil(t, f.store.setup.BasicsConfirmedAt)
	assert.Equal(t, f.now, *f.store.setup.BasicsConfirmedAt)
	assert.Equal(t, int64(setupTenant), f.store.setup.TenantID)
	assert.Equal(t, int64(setupAdmin), *f.store.setup.UpdatedBy)
}

func TestSchoolSetupConfirmBasicsRejectsAnUnknownPresenceMode(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)

	err := f.service.ConfirmBasics(context.Background(), setupTenant, setupAdmin, "rooms_only", true)

	require.ErrorIs(t, err, schoolsetup.ErrInvalidPresenceMode)
	assert.Zero(t, f.store.upserts)
}

func TestSchoolSetupConfirmBasicsKeepsTheAnswersWhenTheGuardRefuses(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)
	f.writeErr = schoolsetup.ErrPresenceModeBlocked

	err := f.service.ConfirmBasics(context.Background(), setupTenant, setupAdmin, schoolsetup.PresenceModeBinary, true)

	require.ErrorIs(t, err, schoolsetup.ErrPresenceModeBlocked)
	assert.Zero(t, f.store.upserts, "no answer is stored when the presence mode could not be written")
}

func TestSchoolSetupSkippingAStep(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)

	require.NoError(t, f.service.SetStepSkipped(context.Background(), setupTenant, setupAdmin, string(schoolsetup.StepRooms), true))
	assert.True(t, stepByKey(t, f.status(t), schoolsetup.StepRooms).Skipped)

	require.NoError(t, f.service.SetStepSkipped(context.Background(), setupTenant, setupAdmin, string(schoolsetup.StepRooms), false))
	assert.False(t, stepByKey(t, f.status(t), schoolsetup.StepRooms).Skipped)

	err := f.service.SetStepSkipped(context.Background(), setupTenant, setupAdmin, string(schoolsetup.StepBasics), true)
	require.ErrorIs(t, err, schoolsetup.ErrStepNotSkippable)
}

func TestSchoolSetupCompletesOnlyWithoutOpenSteps(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)
	ctx := context.Background()

	require.NoError(t, f.service.ConfirmBasics(ctx, setupTenant, setupAdmin, schoolsetup.PresenceModeDetailed, true))
	require.ErrorIs(t, f.service.Complete(ctx, setupTenant, setupAdmin), schoolsetup.ErrIncomplete)

	f.progress.facts = schoolsetup.Facts{StaffInvited: true, RoomCreated: true, StudentEnrolled: true}
	require.NoError(t, f.service.SetStepSkipped(ctx, setupTenant, setupAdmin, string(schoolsetup.StepGroups), true))
	require.NoError(t, f.service.SetStepSkipped(ctx, setupTenant, setupAdmin, string(schoolsetup.StepGuardians), true))
	require.NoError(t, f.service.Complete(ctx, setupTenant, setupAdmin))

	status := f.status(t)
	assert.True(t, status.Completed)
	assert.Empty(t, status.Steps, "a finished school needs no steps")
}

// TestSchoolSetupIsClosedAfterCompletion pins ADR 0040: once completed, the
// presence mode is operator-only again, so no wizard write goes through.
func TestSchoolSetupIsClosedAfterCompletion(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)
	completed := f.now
	f.store.setup = &schoolsetup.State{TenantID: setupTenant, BasicsConfirmedAt: &completed, CompletedAt: &completed}
	ctx := context.Background()

	require.ErrorIs(t, f.service.ConfirmBasics(ctx, setupTenant, setupAdmin, schoolsetup.PresenceModeBinary, true), schoolsetup.ErrCompleted)
	require.ErrorIs(t, f.service.SetStepSkipped(ctx, setupTenant, setupAdmin, string(schoolsetup.StepRooms), true), schoolsetup.ErrCompleted)
	require.ErrorIs(t, f.service.Complete(ctx, setupTenant, setupAdmin), schoolsetup.ErrCompleted)
	assert.Empty(t, f.written)
	assert.Zero(t, f.progress.reads, "a finished school reads no progress")
}

func TestSchoolSetupDismissalIsPersonal(t *testing.T) {
	t.Parallel()
	f := newSetupFixture(t)
	ctx := context.Background()

	require.NoError(t, f.service.SetDismissed(ctx, setupTenant, setupAdmin, true))

	assert.True(t, f.status(t).Dismissed)
	colleague, err := f.service.Status(ctx, setupTenant, setupColleague)
	require.NoError(t, err)
	assert.False(t, colleague.Dismissed, "hiding the wizard does not hide it for other admins")

	require.NoError(t, f.service.SetDismissed(ctx, setupTenant, setupAdmin, false))
	assert.False(t, f.status(t).Dismissed)
}

func TestSchoolSetupPropagatesProgressErrors(t *testing.T) {
	t.Parallel()
	failure := errors.New("projection unavailable")
	service, err := New(newFakeSchoolSetupStore(), failingSetupProgress{err: failure}, &fakeSetupSettings{}, func(context.Context, int64, int64, string) error { return nil }, time.Now)
	require.NoError(t, err)

	_, err = service.Status(context.Background(), setupTenant, setupAdmin)

	require.ErrorIs(t, err, failure)
}

type failingSetupProgress struct{ err error }

func (f failingSetupProgress) Facts(context.Context, int64) (schoolsetup.Facts, error) {
	return schoolsetup.Facts{}, f.err
}
