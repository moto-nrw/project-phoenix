package active

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeSettingsResolver is a minimal stub that satisfies the SettingsResolver
// interface. Each field is independently settable so tests can script the
// full branch space of resolveClearMode without pulling in the real service.
type fakeSettingsResolver struct {
	hasOverride    bool
	hasOverrideErr error
	resolved       string
	resolveErr     error
}

func (f *fakeSettingsResolver) HasTenantOverride(_ context.Context, _ string) (bool, error) {
	return f.hasOverride, f.hasOverrideErr
}

func (f *fakeSettingsResolver) ResolveString(_ context.Context, _ string) (string, error) {
	return f.resolved, f.resolveErr
}

func (f *fakeSettingsResolver) ResolveInt(_ context.Context, _ string) (int, error) {
	return 0, nil
}

// TestResolveClearMode_NilSettings — when no SettingsResolver is injected the
// fallback value is returned untouched.
func TestResolveClearMode_NilSettings(t *testing.T) {
	s := &service{}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "next_checkin")
	assert.Equal(t, "next_checkin", got)
}

// TestResolveClearMode_HasOverrideError — override check failure must not
// crash and must degrade gracefully to the fallback.
func TestResolveClearMode_HasOverrideError(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverrideErr: errors.New("db error"),
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.excused_clear_mode", "end_of_day")
	assert.Equal(t, "end_of_day", got)
}

// TestResolveClearMode_NoOverride — when the tenant has not set a value the
// service returns the fallback instead of the registry default (because
// ResolveString would return the registry default, which the caller wants to
// distinguish from a real override).
func TestResolveClearMode_NoOverride(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{hasOverride: false},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "next_checkin")
	assert.Equal(t, "next_checkin", got)
}

// TestResolveClearMode_ResolveStringError — override exists but read fails;
// fall through to the caller-supplied fallback.
func TestResolveClearMode_ResolveStringError(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverride: true,
			resolveErr:  errors.New("boom"),
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "manual")
	assert.Equal(t, "manual", got)
}

// TestResolveClearMode_EmptyString — an empty override is treated as absent so
// the caller's fallback wins (matches the scheduler's resolveStringSetting
// contract).
func TestResolveClearMode_EmptyString(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverride: true,
			resolved:    "",
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "manual")
	assert.Equal(t, "manual", got)
}

// TestResolveClearMode_OverrideReturned — the tenant value wins when
// everything succeeds.
func TestResolveClearMode_OverrideReturned(t *testing.T) {
	s := &service{
		settings: &fakeSettingsResolver{
			hasOverride: true,
			resolved:    "end_of_day",
		},
	}
	got := s.resolveClearMode(context.Background(), "operations.sick_clear_mode", "next_checkin")
	assert.Equal(t, "end_of_day", got)
}

// mockStudentRepoForClear is a thin stub that only needs to implement the
// two methods autoClearStudent{Sick,Excused} call. Every other method of
// userModels.StudentRepository panics if invoked, which keeps the test
// surface small and catches accidental extra calls.
type mockStudentRepoForClear struct {
	userModels.StudentRepository

	findByIDFunc func(ctx context.Context, id interface{}) (*userModels.Student, error)
	updateErr    error
	updateCalls  int
	lastUpdate   *userModels.Student
}

func (m *mockStudentRepoForClear) FindByID(ctx context.Context, id interface{}) (*userModels.Student, error) {
	return m.findByIDFunc(ctx, id)
}

func (m *mockStudentRepoForClear) Update(_ context.Context, s *userModels.Student) error {
	m.updateCalls++
	m.lastUpdate = s
	return m.updateErr
}

// newTestServiceWithLogger returns a minimal *service wired up to a discard
// logger and the given settings + student-repo stubs. Only the fields that
// auto-clear exercises are populated.
func newTestServiceWithLogger(s SettingsResolver, repo userModels.StudentRepository) *service {
	return &service{ServiceDependencies: ServiceDependencies{StudentRepo: repo, Logger: slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))}, settings: s}
}

// TestAutoClearStudentSickness_SkipsWhenModeNotNextCheckin — no studentRepo
// calls happen when the tenant has chosen a different clear mode.
func TestAutoClearStudentSickness_SkipsWhenModeNotNextCheckin(t *testing.T) {
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			t.Fatal("repo should not be called when mode != next_checkin")
			return nil, nil
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "manual"}
	s := newTestServiceWithLogger(settings, repo)

	s.autoClearStudentSickness(context.Background(), 42)
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentSickness_FindByIDError — error from repo is swallowed
// (logged), and no Update call happens.
func TestAutoClearStudentSickness_FindByIDError(t *testing.T) {
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return nil, errors.New("db down")
		},
	}
	// No settings -> resolveClearMode returns the fallback, which is
	// next_checkin for sickness — so we actually enter the repo branch.
	s := newTestServiceWithLogger(nil, repo)

	s.autoClearStudentSickness(context.Background(), 99)
	assert.Equal(t, 0, repo.updateCalls, "Update must not be called when FindByID fails")
}

// TestAutoClearStudentSickness_AlreadyHealthy — student has sick=false, so no
// Update is issued (early return path).
func TestAutoClearStudentSickness_AlreadyHealthy(t *testing.T) {
	falseVal := false
	healthy := &userModels.Student{Model: base.Model{ID: 1}, Sick: &falseVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return healthy, nil
		},
	}
	s := newTestServiceWithLogger(nil, repo)

	s.autoClearStudentSickness(context.Background(), 1)
	assert.Equal(t, 0, repo.updateCalls, "Update must not fire when student is already healthy")
}

// TestAutoClearStudentSickness_UpdateErrorIsLogged — Update failure is logged
// but does not panic or leak outside the helper.
func TestAutoClearStudentSickness_UpdateErrorIsLogged(t *testing.T) {
	trueVal := true
	sickStudent := &userModels.Student{Model: base.Model{ID: 2}, Sick: &trueVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return sickStudent, nil
		},
		updateErr: errors.New("update failed"),
	}
	s := newTestServiceWithLogger(nil, repo)

	s.autoClearStudentSickness(context.Background(), 2)
	require.Equal(t, 1, repo.updateCalls)
}

// TestAutoClearStudentExcused_SkipsWhenModeNotNextCheckin — excused default is
// end_of_day, so a nil settings resolver keeps the flag untouched.
func TestAutoClearStudentExcused_SkipsWhenModeNotNextCheckin(t *testing.T) {
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			t.Fatal("repo should not be called under default end_of_day mode")
			return nil, nil
		},
	}
	s := newTestServiceWithLogger(nil, repo)

	s.autoClearStudentExcused(context.Background(), 42)
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentExcused_FindByIDError — with override set to
// next_checkin, repo error must be swallowed without Update being called.
func TestAutoClearStudentExcused_FindByIDError(t *testing.T) {
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return nil, errors.New("db down")
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	s.autoClearStudentExcused(context.Background(), 99)
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentExcused_AlreadyNotExcused — no-op when excused=false.
func TestAutoClearStudentExcused_AlreadyNotExcused(t *testing.T) {
	falseVal := false
	notExcused := &userModels.Student{Model: base.Model{ID: 1}, Excused: &falseVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return notExcused, nil
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	s.autoClearStudentExcused(context.Background(), 1)
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentExcused_Clears — happy path: mode=next_checkin,
// student excused, Update succeeds, flag and timestamp both zeroed.
func TestAutoClearStudentExcused_Clears(t *testing.T) {
	trueVal := true
	excStudent := &userModels.Student{Model: base.Model{ID: 3}, Excused: &trueVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return excStudent, nil
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	s.autoClearStudentExcused(context.Background(), 3)
	require.Equal(t, 1, repo.updateCalls)
	require.NotNil(t, repo.lastUpdate)
	require.NotNil(t, repo.lastUpdate.Excused)
	assert.False(t, *repo.lastUpdate.Excused)
	assert.Nil(t, repo.lastUpdate.ExcusedSince)
}

// TestAutoClearStudentExcused_UpdateError — Update failure is swallowed; no
// panic. Complements the sickness equivalent to close the branch matrix.
func TestAutoClearStudentExcused_UpdateError(t *testing.T) {
	trueVal := true
	excStudent := &userModels.Student{Model: base.Model{ID: 4}, Excused: &trueVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return excStudent, nil
		},
		updateErr: errors.New("update failed"),
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	s.autoClearStudentExcused(context.Background(), 4)
	require.Equal(t, 1, repo.updateCalls)
}
