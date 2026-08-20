package users

// In-package unit tests for the side-effect helpers and feature-toggle
// dispatcher on studentPhotoService. The integration tests in
// api/students/photo_routes_test.go exercise the DB-bound methods but live
// in a different Go package — they don't credit this file under
// SonarCloud's "new code coverage" metric. These tests close the broadcast
// + toggle paths without standing up a real DB.

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/services/config/sideeffects"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// tenantIDsOf extracts the TenantID of every "tenant" broadcast call, in
// order.
func tenantIDsOf(calls []testpkg.BroadcastCall) []int64 {
	out := make([]int64, len(calls))
	for i, c := range calls {
		out[i] = c.TenantID
	}
	return out
}

// --- NewStudentPhotoService --------------------------------------------

func TestNewStudentPhotoService_WiresAllFields(t *testing.T) {
	t.Parallel()

	unlinker := &recordingUnlinker{}
	broadcaster := testpkg.NewRecordingBroadcaster()
	logger := slog.Default()
	settings := &stubPhotoSettings{}

	svc := NewStudentPhotoService(StudentPhotoServiceDependencies{
		StudentRepo: nil, // not used by the broadcast helpers
		Settings:    settings,
		UserContext: nil,
		Broadcaster: broadcaster,
		Unlinker:    unlinker,
		DB:          nil,
		Logger:      logger,
	})

	concrete, ok := svc.(*studentPhotoService)
	require.True(t, ok, "NewStudentPhotoService must return *studentPhotoService")
	assert.Same(t, broadcaster, concrete.Broadcaster)
	assert.Same(t, unlinker, concrete.Unlinker)
	assert.Equal(t, settings, concrete.Settings)
	assert.Same(t, logger, concrete.Logger)
}

// --- broadcastStudentUpdated -------------------------------------------

func TestBroadcastStudentUpdated_NilBroadcasterIsNoOp(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: nil}}
	require.NotPanics(t, func() {
		svc.broadcastStudentUpdated(1, 2, photoSourceUploaded)
	})
}

func TestBroadcastStudentUpdated_NonPositiveTenantSkipsAndLogs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: logger}}

	svc.broadcastStudentUpdated(0, 42, photoSourceDeleted)

	assert.Empty(t, b.Events(), "tenantID<=0 must never broadcast")
	assert.Contains(t, buf.String(), "no tenant context")
}

func TestBroadcastStudentUpdated_HappyPath(t *testing.T) {
	t.Parallel()

	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: slog.Default()}}

	svc.broadcastStudentUpdated(7, 99, photoSourceUploaded)

	require.Len(t, b.Events(), 1)
	assert.Equal(t, realtime.EventStudentUpdated, b.Events()[0].Type)
	require.NotNil(t, b.Events()[0].Data.Source)
	assert.Equal(t, photoSourceUploaded, *b.Events()[0].Data.Source)
	assert.Equal(t, []int64{7}, tenantIDsOf(b.CallsByMethod("tenant")))
}

func TestBroadcastStudentUpdated_ErrorIsLoggedNotPropagated(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	b := testpkg.NewRecordingBroadcaster()
	b.Err = errors.New("hub stopped")
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: logger}}

	require.NotPanics(t, func() {
		svc.broadcastStudentUpdated(1, 2, photoSourceDeleted)
	})
	assert.Contains(t, buf.String(), "failed to broadcast student_updated")
	assert.Contains(t, buf.String(), "hub stopped")
}

// --- deferTenantSettingsChanged ----------------------------------------

func TestDeferTenantSettingsChanged_NilBroadcasterIsNoOp(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: nil}}
	notify := svc.deferTenantSettingsChanged(1, configModel.KeyStudentPhotosEnabled)
	require.NotPanics(t, notify)
}

func TestDeferTenantSettingsChanged_BroadcastsAndCarriesKey(t *testing.T) {
	t.Parallel()

	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: slog.Default()}}

	notify := svc.deferTenantSettingsChanged(13, configModel.KeyStudentPhotosEnabled)
	notify()

	require.Len(t, b.Events(), 1)
	assert.Equal(t, realtime.EventTenantSettingsChanged, b.Events()[0].Type)
	require.NotNil(t, b.Events()[0].Data.Source)
	assert.Equal(t, configModel.KeyStudentPhotosEnabled, *b.Events()[0].Data.Source)
	assert.Equal(t, []int64{13}, tenantIDsOf(b.CallsByMethod("tenant")))
}

func TestDeferTenantSettingsChanged_BroadcastErrorLogged(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	b := testpkg.NewRecordingBroadcaster()
	b.Err = errors.New("hub overflow")
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: logger}}

	notify := svc.deferTenantSettingsChanged(1, "operations.student_photos_enabled")
	require.NotPanics(t, notify)
	assert.Contains(t, buf.String(), "tenant_settings_changed broadcast failed")
	assert.Contains(t, buf.String(), "hub overflow")
}

// --- HandleFeatureToggle (paths that don't require the repo) -----------

func TestHandleFeatureToggle_NonBoolValueTreatedAsEnable(t *testing.T) {
	t.Parallel()

	// Anything that isn't a bool falls through the "enable" branch — the
	// service never tries to purge.
	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: slog.Default()}}

	post, err := svc.HandleFeatureToggle(context.Background(), 9, "not-a-bool")
	require.NoError(t, err)
	require.NotNil(t, post)

	post()

	// One student_updated + one tenant_settings_changed broadcast.
	require.Len(t, b.Events(), 2)
	types := []realtime.EventType{b.Events()[0].Type, b.Events()[1].Type}
	assert.Contains(t, types, realtime.EventStudentUpdated)
	assert.Contains(t, types, realtime.EventTenantSettingsChanged)
}

func TestHandleFeatureToggle_TrueEmitsEnableBroadcasts(t *testing.T) {
	t.Parallel()

	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Broadcaster: b, Logger: slog.Default()}}

	post, err := svc.HandleFeatureToggle(context.Background(), 5, true)
	require.NoError(t, err)
	require.NotNil(t, post)
	post()

	require.Len(t, b.Events(), 2)
	// Source on the student_updated event labels the enable trigger.
	var sawEnableSource bool
	for _, ev := range b.Events() {
		if ev.Type == realtime.EventStudentUpdated && ev.Data.Source != nil && *ev.Data.Source == photoSourceFeatureEnabled {
			sawEnableSource = true
		}
	}
	assert.True(t, sawEnableSource, "enable path must label its broadcast source")
}

// --- RegisterStudentPhotoSettingsSideEffects ---------------------------

type fakeStudentPhotoService struct {
	StudentPhotoService
	calls int32
}

func (f *fakeStudentPhotoService) HandleFeatureToggle(_ context.Context, _ int64, _ any) (func(), error) {
	atomic.AddInt32(&f.calls, 1)
	return func() {}, nil
}

func TestRegisterStudentPhotoSettingsSideEffects_DispatchesToService(t *testing.T) {
	t.Parallel()

	registry := sideeffects.NewRegistry()
	svc := &fakeStudentPhotoService{}

	RegisterStudentPhotoSettingsSideEffects(registry, svc)

	post, err := registry.Dispatch(
		context.Background(),
		1,
		configModel.KeyStudentPhotosEnabled,
		true,
	)
	require.NoError(t, err)
	require.NotNil(t, post)
	assert.Equal(t, int32(1), atomic.LoadInt32(&svc.calls),
		"registry must route the photo feature key to the service")
}

func TestRegisterStudentPhotoSettingsSideEffects_OtherKeysIgnored(t *testing.T) {
	t.Parallel()

	registry := sideeffects.NewRegistry()
	svc := &fakeStudentPhotoService{}

	RegisterStudentPhotoSettingsSideEffects(registry, svc)

	post, err := registry.Dispatch(context.Background(), 1, "operations.session_end_time", "18:00")
	require.NoError(t, err)
	assert.Nil(t, post, "unrelated keys must return a nil post-commit callback")
	assert.Equal(t, int32(0), atomic.LoadInt32(&svc.calls),
		"only the photos key may invoke the photo handler")
}

// --- no-tenant-ctx early returns (close 3 service entry points) ---

func TestCommitUpload_NoTenantContext(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{}
	err := svc.CommitUpload(context.Background(), CommitUploadRequest{StudentID: 1})
	assert.ErrorIs(t, err, ErrPhotoNoTenant)
}

func TestCommitDelete_NoTenantContext(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{}
	_, err := svc.CommitDelete(context.Background(), 1)
	assert.ErrorIs(t, err, ErrPhotoNoTenant)
}

func TestLookupForRead_NoTenantContext(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{}
	_, err := svc.LookupForRead(context.Background(), 1, "x.jpg")
	assert.ErrorIs(t, err, ErrPhotoNoTenant)
}

// --- PurgeAllPhotos + HandleFeatureToggle(false) via stub repo ---

// stubRepo embeds the interface so we only implement what we call.
type stubRepo struct {
	userModels.StudentRepository
	urls []string
	err  error
}

func (s *stubRepo) PurgeAllPhotos(_ context.Context) ([]string, error) {
	return s.urls, s.err
}

func TestPurgeAllPhotos_RepoErrorPropagates(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{StudentRepo: &stubRepo{err: errors.New("boom")}}}
	post, err := svc.PurgeAllPhotos(context.Background(), 7)
	assert.Nil(t, post)
	require.Error(t, err)
}

func TestPurgeAllPhotos_UnlinksAndBroadcasts(t *testing.T) {
	t.Parallel()

	u := &recordingUnlinker{}
	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{StudentRepo: &stubRepo{urls: []string{"/uploads/student-photos/a.jpg", "/uploads/student-photos/b.jpg"}}, Unlinker: u, Broadcaster: b, Logger: slog.Default()}}
	post, err := svc.PurgeAllPhotos(context.Background(), 7)
	require.NoError(t, err)
	require.NotNil(t, post)
	post()
	assert.Equal(t, []string{"/uploads/student-photos/a.jpg", "/uploads/student-photos/b.jpg"}, u.urls)
	require.Len(t, b.Events(), 1)
	assert.Equal(t, realtime.EventStudentUpdated, b.Events()[0].Type)
}

func TestHandleFeatureToggle_FalsePurgesAndBroadcasts(t *testing.T) {
	t.Parallel()

	u := &recordingUnlinker{}
	b := testpkg.NewRecordingBroadcaster()
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{StudentRepo: &stubRepo{urls: []string{"/uploads/student-photos/x.jpg"}}, Unlinker: u, Broadcaster: b, Logger: slog.Default()}}
	post, err := svc.HandleFeatureToggle(context.Background(), 9, false)
	require.NoError(t, err)
	require.NotNil(t, post)
	post()
	assert.Equal(t, []string{"/uploads/student-photos/x.jpg"}, u.urls)
	require.Len(t, b.Events(), 2, "purge broadcast + tenant_settings_changed")
}

func TestLookupForRead_EmptyFilename(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{}
	// Tenant must be present to reach the filename check.
	ctx := tenant.WithTenantID(context.Background(), 5)
	_, err := svc.LookupForRead(ctx, 1, "")
	assert.ErrorIs(t, err, ErrPhotoFilenameMismatch)
}
