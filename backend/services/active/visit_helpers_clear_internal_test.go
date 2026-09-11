package active

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type failingPlannedStatusRead struct {
	activeModels.StudentStatusDayRepository
	err                   error
	clearErr              error
	upsertErr, historyErr error
}

func (r *failingPlannedStatusRead) UpsertReported(context.Context, *activeModels.StudentStatusDay) error {
	return r.upsertErr
}

func (r *failingPlannedStatusRead) MarkCleared(context.Context, int64, string, timezone.Date, time.Time, string) error {
	return r.historyErr
}

func TestLiveStatusHistoryFailuresDoNotClearFlags(t *testing.T) {
	t.Parallel()
	injected := errors.New("status history write failed")
	for _, flag := range []string{"sick", "excused"} {
		for _, stage := range []string{"upsert", "clear"} {
			t.Run(flag+"/"+stage, func(t *testing.T) {
				set := true
				student := &userModels.Student{Model: base.Model{ID: 42}, Sick: &set, Excused: &set}
				students := &mockStudentRepoForClear{}
				statuses := &failingPlannedStatusRead{}
				if stage == "upsert" {
					statuses.upsertErr = injected
				} else {
					statuses.historyErr = injected
				}
				svc := &service{ServiceDependencies: ServiceDependencies{StudentRepo: students, StudentStatusRepo: statuses}}
				var err error
				if flag == "sick" {
					err = svc.clearSickFlagOnCheckin(context.Background(), student, time.Now())
				} else {
					err = svc.clearExcusedFlagOnCheckin(context.Background(), student, time.Now())
				}
				require.ErrorIs(t, err, injected)
				assert.Zero(t, students.updateCalls)
				assert.True(t, *student.Sick)
				assert.True(t, *student.Excused)
			})
		}
	}
}

func (r *failingPlannedStatusRead) MarkClearedByID(context.Context, int64, time.Time, string) error {
	return r.clearErr
}

func (r *failingPlannedStatusRead) FindActiveByStudentAndDateRange(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return nil, r.err
}

func (r *failingPlannedStatusRead) FindActiveByStudentIDsAndDate(context.Context, []int64, timezone.Date) ([]*activeModels.StudentStatusDay, error) {
	return nil, r.err
}

func TestPlannedStatusClearFailuresPropagate(t *testing.T) {
	t.Parallel()
	injected := errors.New("planned status clear failed")
	for _, stage := range []string{"status write", "student read", "student write"} {
		t.Run(stage, func(t *testing.T) {
			statuses := &failingPlannedStatusRead{}
			students := &mockStudentRepoForClear{findByIDFunc: func(context.Context, interface{}) (*userModels.Student, error) {
				if stage == "student read" {
					return nil, injected
				}
				return &userModels.Student{Model: base.Model{ID: 42}}, nil
			}}
			if stage == "status write" {
				statuses.clearErr = injected
			}
			if stage == "student write" {
				students.updateErr = injected
			}
			svc := &service{ServiceDependencies: ServiceDependencies{StudentStatusRepo: statuses, StudentRepo: students}}
			rows := []*activeModels.StudentStatusDay{{StudentID: 42, Status: activeModels.StudentStatusDaySick, Source: activeModels.StudentStatusSourcePlanned}}
			require.ErrorIs(t, svc.clearPlannedStatusRows(context.Background(), 42, nil, rows, time.Now()), injected)
			if stage != "student write" {
				assert.Zero(t, students.updateCalls)
			}
		})
	}
}

func TestCheckinPlannedStatusReadFailuresPropagate(t *testing.T) {
	t.Parallel()
	injected := errors.New("planned status read failed")
	svc := &service{ServiceDependencies: ServiceDependencies{StudentStatusRepo: &failingPlannedStatusRead{err: injected}}}
	svc.settings = &fakeSettingsResolver{resolved: configModel.ClearModeManual}
	require.ErrorIs(t, svc.autoClearPlannedStudentStatuses(context.Background(), 42), injected)
	require.ErrorIs(t, svc.autoClearOnBatchCheckin(context.Background(), []int64{42}, nil, time.Now(), timezone.TodayDate()), injected)
}

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

func TestResolveClearModeUsesResolvedValueAndPropagatesErrors(t *testing.T) {
	t.Parallel()
	injected := errors.New("settings unavailable")
	for _, scenario := range []struct {
		name     string
		resolver SettingsResolver
		want     string
		wantErr  error
		missing  bool
	}{
		{name: "missing wiring", missing: true},
		{name: "read failure", resolver: &fakeSettingsResolver{resolveErr: injected}, wantErr: injected},
		{name: "registry default", resolver: &fakeSettingsResolver{resolved: "next_checkin"}, want: "next_checkin"},
		{name: "explicit override", resolver: &fakeSettingsResolver{hasOverride: true, resolved: "manual"}, want: "manual"},
		{name: "override equals default", resolver: &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}, want: "next_checkin"},
		{name: "no override probe", resolver: &fakeSettingsResolver{hasOverrideErr: injected, resolved: "end_of_day"}, want: "end_of_day"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			svc := &service{settings: scenario.resolver}
			value, err := svc.resolveClearMode(context.Background(), configModel.KeySickClearMode)
			if scenario.missing {
				require.ErrorContains(t, err, "settings service is not configured")
			} else if scenario.wantErr != nil {
				require.ErrorIs(t, err, scenario.wantErr)
			} else {
				require.NoError(t, err)
			}
			assert.Equal(t, scenario.want, value)
		})
	}
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
	if s == nil {
		s = &configtest.Mock{ResolveStringFn: func(_ context.Context, key string) (string, error) {
			return configModel.GetDefinition(key).Default.(string), nil
		}}
	}
	return &service{ServiceDependencies: ServiceDependencies{StudentRepo: repo, Logger: slog.New(slog.NewTextHandler(new(bytes.Buffer), nil))}, settings: s}
}

// TestAutoClearStudentSickness_SkipsWhenModeNotNextCheckin — no studentRepo
// calls happen when the tenant has chosen a different clear mode.
func TestAutoClearStudentSickness_SkipsWhenModeNotNextCheckin(t *testing.T) {
	t.Parallel()

	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			t.Fatal("repo should not be called when mode != next_checkin")
			return nil, nil
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "manual"}
	s := newTestServiceWithLogger(settings, repo)

	require.NoError(t, s.autoClearStudentSickness(context.Background(), 42))
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentSickness_FindByIDError — error from repo is swallowed
// (logged), and no Update call happens.
func TestAutoClearStudentSickness_FindByIDError(t *testing.T) {
	t.Parallel()

	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return nil, errors.New("db down")
		},
	}
	// No settings -> resolveClearMode returns the fallback, which is
	// next_checkin for sickness — so we actually enter the repo branch.
	s := newTestServiceWithLogger(nil, repo)

	require.ErrorContains(t, s.autoClearStudentSickness(context.Background(), 99), "db down")
	assert.Equal(t, 0, repo.updateCalls, "Update must not be called when FindByID fails")
}

// TestAutoClearStudentSickness_AlreadyHealthy — student has sick=false, so no
// Update is issued (early return path).
func TestAutoClearStudentSickness_AlreadyHealthy(t *testing.T) {
	t.Parallel()

	falseVal := false
	healthy := &userModels.Student{Model: base.Model{ID: 1}, Sick: &falseVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return healthy, nil
		},
	}
	s := newTestServiceWithLogger(nil, repo)

	require.NoError(t, s.autoClearStudentSickness(context.Background(), 1))
	assert.Equal(t, 0, repo.updateCalls, "Update must not fire when student is already healthy")
}

// TestAutoClearStudentSickness_UpdateErrorPropagates preserves the write failure.
func TestAutoClearStudentSickness_UpdateErrorPropagates(t *testing.T) {
	t.Parallel()

	trueVal := true
	sickStudent := &userModels.Student{Model: base.Model{ID: 2}, Sick: &trueVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return sickStudent, nil
		},
		updateErr: errors.New("update failed"),
	}
	s := newTestServiceWithLogger(nil, repo)

	require.ErrorContains(t, s.autoClearStudentSickness(context.Background(), 2), "update failed")
	require.Equal(t, 1, repo.updateCalls)
}

// TestAutoClearStudentExcused_SkipsWhenModeNotNextCheckin — excused default is
// end_of_day, so a nil settings resolver keeps the flag untouched.
func TestAutoClearStudentExcused_SkipsWhenModeNotNextCheckin(t *testing.T) {
	t.Parallel()

	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			t.Fatal("repo should not be called under default end_of_day mode")
			return nil, nil
		},
	}
	s := newTestServiceWithLogger(nil, repo)

	require.NoError(t, s.autoClearStudentExcused(context.Background(), 42))
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentExcused_FindByIDError — with override set to
// next_checkin, the read error propagates without an Update.
func TestAutoClearStudentExcused_FindByIDError(t *testing.T) {
	t.Parallel()

	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return nil, errors.New("db down")
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	require.ErrorContains(t, s.autoClearStudentExcused(context.Background(), 99), "db down")
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentExcused_AlreadyNotExcused — no-op when excused=false.
func TestAutoClearStudentExcused_AlreadyNotExcused(t *testing.T) {
	t.Parallel()

	falseVal := false
	notExcused := &userModels.Student{Model: base.Model{ID: 1}, Excused: &falseVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return notExcused, nil
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	require.NoError(t, s.autoClearStudentExcused(context.Background(), 1))
	assert.Equal(t, 0, repo.updateCalls)
}

// TestAutoClearStudentExcused_Clears — happy path: mode=next_checkin,
// student excused, Update succeeds, flag and timestamp both zeroed.
func TestAutoClearStudentExcused_Clears(t *testing.T) {
	t.Parallel()

	trueVal := true
	excStudent := &userModels.Student{Model: base.Model{ID: 3}, Excused: &trueVal}
	repo := &mockStudentRepoForClear{
		findByIDFunc: func(_ context.Context, _ interface{}) (*userModels.Student, error) {
			return excStudent, nil
		},
	}
	settings := &fakeSettingsResolver{hasOverride: true, resolved: "next_checkin"}
	s := newTestServiceWithLogger(settings, repo)

	require.NoError(t, s.autoClearStudentExcused(context.Background(), 3))
	require.Equal(t, 1, repo.updateCalls)
	require.NotNil(t, repo.lastUpdate)
	require.NotNil(t, repo.lastUpdate.Excused)
	assert.False(t, *repo.lastUpdate.Excused)
	assert.Nil(t, repo.lastUpdate.ExcusedSince)
}

// TestAutoClearStudentExcused_UpdateError preserves the write failure.
func TestAutoClearStudentExcused_UpdateError(t *testing.T) {
	t.Parallel()

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

	require.ErrorContains(t, s.autoClearStudentExcused(context.Background(), 4), "update failed")
	require.Equal(t, 1, repo.updateCalls)
}
