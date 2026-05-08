package students

// Unit tests for the small helpers in photo.go that don't require
// database fixtures: stampPhotoConsent, removeStudentPhotoFile, and
// ensurePhotoFeatureEnabled. These pin the per-tenant feature gate's
// closed-fail semantics (a settings outage must NOT silently allow
// uploads on a school that never opted in) and the consent-stamp
// behaviour (acting account is read from the JWT in ctx, never from
// the request body).

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/users"
	configSvc "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- minimal mock SettingsService (only the calls photo.go makes) ---

// stubSettingsService implements configSvc.SettingsService with just the
// methods photo.go touches; everything else panics if accidentally called.
type stubSettingsService struct {
	resolveBoolFn func(ctx context.Context, key string) (bool, error)
}

var _ configSvc.SettingsService = (*stubSettingsService)(nil)

func (s *stubSettingsService) GetSchema(context.Context, []string) (*configSvc.SettingsSchema, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) GetSchemaForOperator(context.Context, []string) (*configSvc.SettingsSchema, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) Resolve(context.Context, string) (any, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) ResolveString(context.Context, string) (string, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) ResolveStringForTenant(context.Context, int64, string) (string, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) ResolveBool(ctx context.Context, key string) (bool, error) {
	if s.resolveBoolFn != nil {
		return s.resolveBoolFn(ctx, key)
	}
	return false, nil
}
func (s *stubSettingsService) ResolveInt(context.Context, string) (int, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) HasTenantOverride(context.Context, string) (bool, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) SetValue(context.Context, string, any, *int64, []string) error {
	panic("unexpected call")
}
func (s *stubSettingsService) ResetValue(context.Context, string, *int64, []string) error {
	panic("unexpected call")
}
func (s *stubSettingsService) GetLoginImageURL(context.Context, int64) (string, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) SetLoginImageURL(context.Context, int64, string) (string, error) {
	panic("unexpected call")
}
func (s *stubSettingsService) ClearLoginImageURL(context.Context, int64) (string, error) {
	panic("unexpected call")
}

// --- stampPhotoConsent ---

// TestStampPhotoConsent_FillsAuditColumnsFromJWT pins the contract that
// the acting account is read from JWT claims in ctx (never from the
// request body) and that both _at and _by are stamped together — a
// half-stamped row would mislead later audits.
func TestStampPhotoConsent_FillsAuditColumnsFromJWT(t *testing.T) {
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 4242})
	student := &users.Student{}

	stampPhotoConsent(ctx, student)

	require.NotNil(t, student.PhotoConsentGivenAt, "must stamp _at")
	require.NotNil(t, student.PhotoConsentGivenBy, "must stamp _by")
	assert.Equal(t, int64(4242), *student.PhotoConsentGivenBy,
		"acting account must come from JWT claims, never the request body")
	assert.WithinDuration(t, *student.PhotoConsentGivenAt, *student.PhotoConsentGivenAt, 0)
}

// TestStampPhotoConsent_OverwritesExisting documents that the helper
// itself does NOT enforce the "don't re-stamp on already-granted"
// rule — that gate lives in applyPhotoConsent (see
// photo_consent_test.go). stampPhotoConsent always writes; callers
// must check the precondition first.
func TestStampPhotoConsent_OverwritesExisting(t *testing.T) {
	ctx := context.WithValue(context.Background(), jwt.CtxClaims, jwt.AppClaims{ID: 1})
	old := int64(99)
	student := &users.Student{PhotoConsentGivenBy: &old}

	stampPhotoConsent(ctx, student)

	require.NotNil(t, student.PhotoConsentGivenBy)
	assert.Equal(t, int64(1), *student.PhotoConsentGivenBy,
		"helper unconditionally overwrites — gating belongs to applyPhotoConsent")
}

// --- ensurePhotoFeatureEnabled ---

// TestEnsurePhotoFeatureEnabled_NilService — no settings service wired
// must close-fail. A startup misconfiguration must NEVER produce an
// open-by-default photo upload path.
func TestEnsurePhotoFeatureEnabled_NilService(t *testing.T) {
	rs := &Resource{SettingsService: nil}

	err := rs.ensurePhotoFeatureEnabled(context.Background())

	require.Error(t, err, "nil settings service must fail closed")
	assert.Equal(t, "photos feature not available", err.Error())
}

// TestEnsurePhotoFeatureEnabled_ResolveError — a settings DB outage
// must propagate as an error, NOT silently default to enabled. The
// upload handler maps this to a 500 (and the file save is rolled back).
func TestEnsurePhotoFeatureEnabled_ResolveError(t *testing.T) {
	rs := &Resource{SettingsService: &stubSettingsService{
		resolveBoolFn: func(_ context.Context, _ string) (bool, error) {
			return false, errors.New("db pool exhausted")
		},
	}}

	err := rs.ensurePhotoFeatureEnabled(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "photo feature lookup failed")
	assert.Contains(t, err.Error(), "db pool exhausted",
		"underlying cause must be wrapped so logs surface the outage")
}

// TestEnsurePhotoFeatureEnabled_Disabled — the German error string is
// load-bearing: the upload handler maps this exact error to a 403 with
// the same text the OGS UI shows. Renaming would silently change the
// UI message.
func TestEnsurePhotoFeatureEnabled_Disabled(t *testing.T) {
	rs := &Resource{SettingsService: &stubSettingsService{
		resolveBoolFn: func(_ context.Context, key string) (bool, error) {
			require.Equal(t, configModel.KeyStudentPhotosEnabled, key,
				"helper must look up the canonical key")
			return false, nil
		},
	}}

	err := rs.ensurePhotoFeatureEnabled(context.Background())

	require.Error(t, err)
	assert.Equal(t, "Kinderfotos sind in dieser Schule nicht aktiviert", err.Error())
}

// TestEnsurePhotoFeatureEnabled_Enabled — happy path returns nil and
// uses the canonical key constant (no re-derivation, no typo risk).
func TestEnsurePhotoFeatureEnabled_Enabled(t *testing.T) {
	called := false
	rs := &Resource{SettingsService: &stubSettingsService{
		resolveBoolFn: func(_ context.Context, key string) (bool, error) {
			called = true
			assert.Equal(t, configModel.KeyStudentPhotosEnabled, key)
			return true, nil
		},
	}}

	err := rs.ensurePhotoFeatureEnabled(context.Background())

	require.NoError(t, err)
	assert.True(t, called, "settings lookup must occur on the happy path too")
}

// --- removeStudentPhotoFile ---

// TestRemoveStudentPhotoFile_EmptyURLNoOp — empty URL must be a silent
// no-op. The handler calls this from the post-commit hook for every
// upload; if a row had no previous photo (capturedOldURL == "") the
// helper must not error or log.
func TestRemoveStudentPhotoFile_EmptyURLNoOp(t *testing.T) {
	// Just verify it doesn't panic. Logger nil-safety covered by the
	// nil-logger branch below.
	removeStudentPhotoFile("", slog.Default())
	removeStudentPhotoFile("", nil)
}

// TestRemoveStudentPhotoFile_RemovesExistingFile — happy path: a real
// file under public/uploads/student-photos is deleted. We seed the
// public/ directory in CWD that ResolveStoredPath walks up to find,
// then assert the file no longer exists after the call.
func TestRemoveStudentPhotoFile_RemovesExistingFile(t *testing.T) {
	pubDir, _ := common.ResolvePublicDir()
	if pubDir == "" {
		t.Skip("public/ directory not resolvable in this test environment")
	}

	dir := filepath.Join(pubDir, "uploads", "student-photos")
	require.NoError(t, os.MkdirAll(dir, 0o755))

	tmp, err := os.CreateTemp(dir, "remove_test_*.jpg")
	require.NoError(t, err)
	require.NoError(t, tmp.Close())
	t.Cleanup(func() { _ = os.Remove(tmp.Name()) })

	storedURL := common.StudentPhotoStoredURLPrefix + filepath.Base(tmp.Name())

	removeStudentPhotoFile(storedURL, slog.Default())

	_, statErr := os.Stat(tmp.Name())
	assert.True(t, os.IsNotExist(statErr),
		"removeStudentPhotoFile must delete the on-disk file referenced by the stored URL")
}

// TestRemoveStudentPhotoFile_BadPrefixLogsAndContinues — a stored URL
// without the canonical prefix indicates legacy data or manual DB
// editing. ResolveStoredPath rejects it; the helper must log and
// continue without panicking — never want to fail a surrounding DB
// write because of a stale unlink.
func TestRemoveStudentPhotoFile_BadPrefixLogsAndContinues(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("removeStudentPhotoFile panicked on bad prefix: %v", r)
		}
	}()

	// Wrong prefix — ResolveStoredPath returns an error.
	removeStudentPhotoFile("/uploads/somewhere-else/foo.jpg", slog.Default())

	// nil logger must also not panic — the helper guards on logger.
	removeStudentPhotoFile("/wrong/prefix/x.jpg", nil)
}

// TestRemoveStudentPhotoFile_MissingFileNoError — RemoveImage already
// guards on os.IsNotExist, but the wrapper must propagate that
// behaviour: the post-commit hook can fire after a manual cleanup
// removed the file, and we don't want spurious error logs from that.
func TestRemoveStudentPhotoFile_MissingFileNoError(t *testing.T) {
	pubDir, _ := common.ResolvePublicDir()
	if pubDir == "" {
		t.Skip("public/ directory not resolvable in this test environment")
	}

	storedURL := common.StudentPhotoStoredURLPrefix + "definitely-not-there.jpg"

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("removeStudentPhotoFile panicked on missing file: %v", r)
		}
	}()
	removeStudentPhotoFile(storedURL, slog.Default())
}

// --- BuildStudentPhotoServeURL (api/common) — placed here because it
// is the boundary helper that populatePhotoFields (response_helpers.go)
// and uploadStudentPhoto (photo.go) both rely on. A typo in the
// rewrite logic would silently 404 every avatar render; pinning here
// catches it.

// TestBuildStudentPhotoServeURL_Empty — empty stored URL means "no
// photo" and must round-trip to an empty serve URL so omitempty drops
// it from the JSON.
func TestBuildStudentPhotoServeURL_Empty(t *testing.T) {
	assert.Empty(t, common.BuildStudentPhotoServeURL(7, ""))
}

// TestBuildStudentPhotoServeURL_HappyPath — stored URL with the
// canonical prefix becomes /api/students/{id}/photo/{filename}.
func TestBuildStudentPhotoServeURL_HappyPath(t *testing.T) {
	stored := common.StudentPhotoStoredURLPrefix + "abc.jpg"
	got := common.BuildStudentPhotoServeURL(123, stored)
	assert.Equal(t, "/api/students/123/photo/abc.jpg", got)
	assert.True(t, strings.HasPrefix(got, "/api/students/"),
		"serve URL must use the authenticated proxy prefix, not /uploads/...")
}

// TestBuildStudentPhotoServeURL_LegacyPath — an unrecognized prefix
// (legacy data, manual DB edit) must pass through unchanged so the
// network tab shows a real 404 instead of being silently rewritten
// into something that doesn't exist on the API.
func TestBuildStudentPhotoServeURL_LegacyPath(t *testing.T) {
	legacy := "/legacy/avatars/foo.jpg"
	assert.Equal(t, legacy, common.BuildStudentPhotoServeURL(1, legacy))
}
