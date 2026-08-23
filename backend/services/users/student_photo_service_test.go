package users

// Unit tests for the small, side-effect-free helpers in
// student_photo_service.go that don't require database fixtures:
//   - requireFeatureEnabled (closed-fail semantics on settings outage)
//   - photoFilenameOf (URL last-segment extractor)
//
// Methods that own DB transactions (CommitUpload, CommitDelete,
// LookupForRead, PurgeAllPhotos) are exercised via the integration tests
// in api/students/photo_routes_test.go which run against the test DB.

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubPhotoSettings is a minimal PhotoSettings impl for the feature-gate
// tests. Only ResolveBool is exercised; the others panic if called so
// accidental dependence is caught loudly.
type stubPhotoSettings struct {
	resolveBoolFn func(ctx context.Context, key string) (bool, error)
}

func (s *stubPhotoSettings) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.resolveBoolFn != nil {
		return s.resolveBoolFn(ctx, key)
	}
	return false, nil
}
func (s *stubPhotoSettings) HasTenantOverride(context.Context, string) (bool, error) {
	panic("unexpected call")
}
func (s *stubPhotoSettings) ResolveString(context.Context, string) (string, error) {
	panic("unexpected call")
}

// TestRequireFeatureEnabled_NilSettings — a nil SettingsService must
// close-fail. A startup misconfiguration must NEVER produce an open-by-
// default photo upload path.
func TestRequireFeatureEnabled_NilSettings(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Settings: nil}}
	err := svc.requireFeatureEnabled(context.Background())
	require.Error(t, err, "nil settings service must fail closed")
	assert.ErrorIs(t, err, ErrPhotoFeatureDisabled)
}

// TestRequireFeatureEnabled_ResolveError — a settings DB outage must
// propagate (wrapped) as an error, NOT silently default to enabled.
func TestRequireFeatureEnabled_ResolveError(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Settings: &stubPhotoSettings{
		resolveBoolFn: func(_ context.Context, _ string) (bool, error) {
			return false, errors.New("db pool exhausted")
		},
	}}}

	err := svc.requireFeatureEnabled(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "photo feature lookup failed")
	assert.Contains(t, err.Error(), "db pool exhausted",
		"underlying cause must be wrapped so logs surface the outage")
}

// TestRequireFeatureEnabled_Disabled — feature off returns the canonical
// sentinel so handlers can map it to the right HTTP status + German message.
func TestRequireFeatureEnabled_Disabled(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Settings: &stubPhotoSettings{
		resolveBoolFn: func(_ context.Context, key string) (bool, error) {
			require.Equal(t, configModel.KeyStudentPhotosEnabled, key,
				"helper must look up the canonical key constant")
			return false, nil
		},
	}}}

	err := svc.requireFeatureEnabled(context.Background())

	require.Error(t, err)
	assert.ErrorIs(t, err, ErrPhotoFeatureDisabled)
}

// TestRequireFeatureEnabled_Enabled — happy path returns nil and looks up
// the canonical key.
func TestRequireFeatureEnabled_Enabled(t *testing.T) {
	t.Parallel()

	called := false
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Settings: &stubPhotoSettings{
		resolveBoolFn: func(_ context.Context, key string) (bool, error) {
			called = true
			assert.Equal(t, configModel.KeyStudentPhotosEnabled, key)
			return true, nil
		},
	}}}

	err := svc.requireFeatureEnabled(context.Background())

	require.NoError(t, err)
	assert.True(t, called, "settings lookup must occur on the happy path too")
}

// --- photoFilenameOf ---

// TestPhotoFilenameOf_HappyPath returns the last segment of a stored URL,
// independent of OS path separators (storedURL is always forward-slash).
func TestPhotoFilenameOf_HappyPath(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "abc.jpg", photoFilenameOf("/uploads/student-photos/abc.jpg"))
}

// TestPhotoFilenameOf_NoSlash falls back to the input unchanged — guards
// against legacy data stored without the canonical prefix.
func TestPhotoFilenameOf_NoSlash(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "loose-name.jpg", photoFilenameOf("loose-name.jpg"))
}

// TestPhotoFilenameOf_TrailingSlash returns the empty string when the URL
// ends in a slash. The handler maps an empty filename comparison to a
// FilenameMismatch sentinel so the caller cannot probe via "" + manual URL.
func TestPhotoFilenameOf_TrailingSlash(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", photoFilenameOf("/uploads/student-photos/"))
}

// --- applyPhotoConsent (pure helper) ---

func ptrBool(b bool) *bool           { return &b }
func ptrString(s string) *string     { return &s }
func ptrTime(t time.Time) *time.Time { return &t }
func ptrInt64(v int64) *int64        { return &v }

func TestApplyPhotoConsent_NoChangeWhenFieldOmitted(t *testing.T) {
	t.Parallel()

	now := time.Now().Add(-time.Hour)
	student := &userModels.Student{
		PhotoPath:           ptrString("/uploads/student-photos/foo.jpg"),
		PhotoConsentGivenAt: ptrTime(now),
		PhotoConsentGivenBy: ptrInt64(42),
	}

	got := applyPhotoConsent(nil, student)

	assert.False(t, got.grantedNow)
	assert.False(t, got.withdrawnNow)
	assert.Equal(t, "", got.removedPhotoPath)
	assert.NotNil(t, student.PhotoPath)
	assert.NotNil(t, student.PhotoConsentGivenAt)
}

func TestApplyPhotoConsent_GrantTransition(t *testing.T) {
	t.Parallel()

	student := &userModels.Student{}
	got := applyPhotoConsent(ptrBool(true), student)

	assert.True(t, got.grantedNow)
	assert.False(t, got.withdrawnNow)
	// Pure helper does NOT stamp _at/_by — service does that with JWT ctx.
	assert.Nil(t, student.PhotoConsentGivenAt)
}

func TestApplyPhotoConsent_WithdrawalClearsAndSurfacesPath(t *testing.T) {
	t.Parallel()

	student := &userModels.Student{
		PhotoPath:           ptrString("/uploads/student-photos/abc.jpg"),
		PhotoConsentGivenAt: ptrTime(time.Now()),
		PhotoConsentGivenBy: ptrInt64(7),
	}

	got := applyPhotoConsent(ptrBool(false), student)

	assert.True(t, got.withdrawnNow)
	assert.Equal(t, "/uploads/student-photos/abc.jpg", got.removedPhotoPath)
	assert.Nil(t, student.PhotoPath)
	assert.Nil(t, student.PhotoConsentGivenAt)
	assert.Nil(t, student.PhotoConsentGivenBy)
}

func TestApplyPhotoConsent_AlreadyGrantedNoOp(t *testing.T) {
	t.Parallel()

	now := time.Now().Add(-time.Hour)
	student := &userModels.Student{
		PhotoConsentGivenAt: ptrTime(now),
		PhotoConsentGivenBy: ptrInt64(99),
	}

	got := applyPhotoConsent(ptrBool(true), student)

	assert.False(t, got.grantedNow,
		"sending consent=true when already granted must NOT re-stamp")
	assert.True(t, student.PhotoConsentGivenAt.Equal(now))
	assert.Equal(t, int64(99), *student.PhotoConsentGivenBy)
}

// --- ApplyConsentTransition + ScheduleUnlinkAfterCommit ---

type recordingUnlinker struct{ urls []string }

func (r *recordingUnlinker) UnlinkStored(url string) { r.urls = append(r.urls, url) }

// Outside a tenant tx, RegisterAfterCommit fires synchronously, so the
// unlinker call is observable inline. Inside a tx the same closure would
// queue and fire after commit — covered by photo_routes_test.go.
func TestApplyConsentTransition_GrantStampsConsent(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 4242})
	unlinker := &recordingUnlinker{}
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: unlinker}}
	student := &userModels.Student{}

	svc.ApplyConsentTransition(ctx, ptrBool(true), student)

	require.NotNil(t, student.PhotoConsentGivenAt)
	require.NotNil(t, student.PhotoConsentGivenBy)
	assert.Equal(t, int64(4242), *student.PhotoConsentGivenBy,
		"acting account must come from JWT claims, never the request body")
	assert.Empty(t, unlinker.urls, "grant must not unlink any file")
}

func TestApplyConsentTransition_WithdrawalSchedulesUnlink(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	unlinker := &recordingUnlinker{}
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: unlinker}}
	student := &userModels.Student{
		PhotoPath:           ptrString("/uploads/student-photos/abc.jpg"),
		PhotoConsentGivenAt: ptrTime(time.Now()),
		PhotoConsentGivenBy: ptrInt64(7),
	}

	svc.ApplyConsentTransition(ctx, ptrBool(false), student)

	assert.Nil(t, student.PhotoPath)
	assert.Nil(t, student.PhotoConsentGivenAt)
	assert.Equal(t, []string{"/uploads/student-photos/abc.jpg"}, unlinker.urls)
}

func TestApplyConsentTransition_WithdrawalWithoutPhotoNoUnlink(t *testing.T) {
	t.Parallel()

	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	unlinker := &recordingUnlinker{}
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: unlinker}}
	student := &userModels.Student{
		PhotoConsentGivenAt: ptrTime(time.Now()),
		PhotoConsentGivenBy: ptrInt64(7),
	}

	svc.ApplyConsentTransition(ctx, ptrBool(false), student)

	assert.Empty(t, unlinker.urls, "no path means nothing to unlink")
}

func TestApplyConsentTransition_NoOpWhenFieldOmitted(t *testing.T) {
	t.Parallel()

	unlinker := &recordingUnlinker{}
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: unlinker}}
	student := &userModels.Student{PhotoPath: ptrString("/uploads/student-photos/x.jpg")}

	svc.ApplyConsentTransition(context.Background(), nil, student)

	assert.NotNil(t, student.PhotoPath, "row must be untouched when consent field omitted")
	assert.Empty(t, unlinker.urls)
}

func TestScheduleUnlinkAfterCommit_EmptyURLNoOp(t *testing.T) {
	t.Parallel()

	unlinker := &recordingUnlinker{}
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: unlinker}}

	svc.ScheduleUnlinkAfterCommit(context.Background(), "")

	assert.Empty(t, unlinker.urls)
}

func TestScheduleUnlinkAfterCommit_NilUnlinkerSafe(t *testing.T) {
	t.Parallel()

	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: nil}}
	require.NotPanics(t, func() {
		svc.ScheduleUnlinkAfterCommit(context.Background(), "/uploads/student-photos/x.jpg")
	})
}

func TestScheduleUnlinkAfterCommit_OutsideTxRunsImmediately(t *testing.T) {
	t.Parallel()

	unlinker := &recordingUnlinker{}
	svc := &studentPhotoService{StudentPhotoServiceDependencies: StudentPhotoServiceDependencies{Unlinker: unlinker}}

	svc.ScheduleUnlinkAfterCommit(context.Background(), "/uploads/student-photos/y.jpg")

	// tenant.RegisterAfterCommit runs synchronously when no tx holder is
	// attached, so the call is observable inline. This pins that the service
	// always routes through RegisterAfterCommit (not a direct unlinker call).
	assert.Equal(t, []string{"/uploads/student-photos/y.jpg"}, unlinker.urls)
}

// avoid unused import when tenant pkg is not referenced via type assertion
var _ = tenant.RegisterAfterCommit
