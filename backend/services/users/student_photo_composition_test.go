package users_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/realtime"
	servicesPkg "github.com/moto-nrw/project-phoenix/services"
	userService "github.com/moto-nrw/project-phoenix/services/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The cases below moved here with the photo lifecycle (#3349). What the
// composition seam still owns is the tenant-wide side of a feature toggle:
// the purge it triggers, the files it detaches and the two broadcasts that
// tell open tabs to re-resolve. The lifecycle itself lives in
// modules/peopledirectory and is exercised through api/students/photo_routes.

// stubPhotoDirectory stands in for the owner capability.
type stubPhotoDirectory struct {
	purged    []string
	purgeErr  error
	unlinked  []string
	purgeSeen int32
}

func (*stubPhotoDirectory) FindStudentPhoto(context.Context, int64, string) (string, error) {
	return "", nil
}
func (*stubPhotoDirectory) CommitStudentPhoto(context.Context, int64, string, bool) error { return nil }
func (*stubPhotoDirectory) ClearStudentPhoto(context.Context, int64) (string, error) {
	return "", nil
}

func (d *stubPhotoDirectory) PurgeStudentPhotos(context.Context) ([]string, error) {
	atomic.AddInt32(&d.purgeSeen, 1)
	return d.purged, d.purgeErr
}

func (*stubPhotoDirectory) ApplyStudentPhotoConsent(
	_ context.Context, current peopleModule.StudentPhotoState, _ *bool,
) peopleModule.StudentPhotoState {
	return current
}

func (*stubPhotoDirectory) ScheduleStudentPhotoUnlink(context.Context, string) {}

type stubPhotoUnlinker struct{ urls []string }

func (u *stubPhotoUnlinker) UnlinkStored(url string) { u.urls = append(u.urls, url) }

// tenantIDsOf extracts the TenantID of every "tenant" broadcast call, in order.
func tenantIDsOf(calls []testpkg.BroadcastCall) []int64 {
	out := make([]int64, len(calls))
	for i, call := range calls {
		out[i] = call.TenantID
	}
	return out
}

func newPhotoSeam(
	t *testing.T,
	directory *stubPhotoDirectory,
	broadcaster servicesPkg.PhotoBroadcaster,
	unlinker userService.PhotoUnlinker,
	logger *slog.Logger,
) userService.StudentPhotoService {
	t.Helper()
	return servicesPkg.NewStudentPhotos(directory, nil, servicesPkg.StudentPhotoRuntimeDependencies{
		Broadcaster: broadcaster, Unlinker: unlinker, Logger: logger,
	})
}

func TestPurgeAllPhotosUnlinksAndBroadcasts(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	unlinker := &stubPhotoUnlinker{}
	directory := &stubPhotoDirectory{purged: []string{"/uploads/a.jpg", "/uploads/b.jpg"}}
	seam := newPhotoSeam(t, directory, broadcaster, unlinker, slog.Default())

	post, err := seam.PurgeAllPhotos(context.Background(), 13)
	require.NoError(t, err)
	require.NotNil(t, post)
	// Nothing happens before the commit: the files must survive a rollback.
	assert.Empty(t, unlinker.urls)
	assert.Empty(t, broadcaster.Events())

	post()

	assert.Equal(t, []string{"/uploads/a.jpg", "/uploads/b.jpg"}, unlinker.urls)
	require.Len(t, broadcaster.Events(), 1)
	assert.Equal(t, realtime.EventStudentUpdated, broadcaster.Events()[0].Type)
	assert.Equal(t, []int64{13}, tenantIDsOf(broadcaster.CallsByMethod("tenant")))
}

func TestPurgeAllPhotosPropagatesTheOwnerError(t *testing.T) {
	t.Parallel()

	directory := &stubPhotoDirectory{purgeErr: errors.New("purge failed")}
	seam := newPhotoSeam(t, directory, testpkg.NewRecordingBroadcaster(), &stubPhotoUnlinker{}, slog.Default())

	post, err := seam.PurgeAllPhotos(context.Background(), 1)
	require.ErrorContains(t, err, "purge failed")
	assert.Nil(t, post)
}

func TestHandleFeatureToggleEnableSkipsThePurge(t *testing.T) {
	t.Parallel()

	for name, value := range map[string]any{"enabled": true, "non-bool": "not-a-bool"} {
		t.Run(name, func(t *testing.T) {
			broadcaster := testpkg.NewRecordingBroadcaster()
			directory := &stubPhotoDirectory{}
			seam := newPhotoSeam(t, directory, broadcaster, &stubPhotoUnlinker{}, slog.Default())

			post, err := seam.HandleFeatureToggle(context.Background(), 5, value)
			require.NoError(t, err)
			require.NotNil(t, post)
			post()

			assert.Zero(t, atomic.LoadInt32(&directory.purgeSeen), "enabling must never purge")
			require.Len(t, broadcaster.Events(), 2)
			var sawEnableSource, sawSettingsChanged bool
			for _, event := range broadcaster.Events() {
				switch {
				case event.Type == realtime.EventStudentUpdated &&
					event.Data.Source != nil && *event.Data.Source == "tenant_photos_enabled":
					sawEnableSource = true
				case event.Type == realtime.EventTenantSettingsChanged &&
					event.Data.Source != nil && *event.Data.Source == configModel.KeyStudentPhotosEnabled:
					sawSettingsChanged = true
				}
			}
			assert.True(t, sawEnableSource, "enable path must label its broadcast source")
			assert.True(t, sawSettingsChanged, "open tabs must re-resolve the tenant setting")
		})
	}
}

func TestHandleFeatureToggleDisablePurgesAndBroadcasts(t *testing.T) {
	t.Parallel()

	broadcaster := testpkg.NewRecordingBroadcaster()
	unlinker := &stubPhotoUnlinker{}
	directory := &stubPhotoDirectory{purged: []string{"/uploads/c.jpg"}}
	seam := newPhotoSeam(t, directory, broadcaster, unlinker, slog.Default())

	post, err := seam.HandleFeatureToggle(context.Background(), 7, false)
	require.NoError(t, err)
	require.NotNil(t, post)
	post()

	assert.Equal(t, int32(1), atomic.LoadInt32(&directory.purgeSeen))
	assert.Equal(t, []string{"/uploads/c.jpg"}, unlinker.urls)
	require.Len(t, broadcaster.Events(), 2)
}

func TestBroadcastFailuresAreLoggedNotPropagated(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	broadcaster := testpkg.NewRecordingBroadcaster()
	broadcaster.Err = errors.New("hub stopped")
	seam := newPhotoSeam(t, &stubPhotoDirectory{}, broadcaster, &stubPhotoUnlinker{}, logger)

	post, err := seam.HandleFeatureToggle(context.Background(), 1, true)
	require.NoError(t, err)
	require.NotPanics(t, post)
	assert.Contains(t, buf.String(), "failed to broadcast student_updated")
	assert.Contains(t, buf.String(), "tenant_settings_changed broadcast failed")
	assert.Contains(t, buf.String(), "hub stopped")
}

func TestBroadcastWithoutABroadcasterIsANoOp(t *testing.T) {
	t.Parallel()

	seam := newPhotoSeam(t, &stubPhotoDirectory{}, nil, nil, slog.Default())
	post, err := seam.HandleFeatureToggle(context.Background(), 1, true)
	require.NoError(t, err)
	require.NotPanics(t, post)
}

// A photo route reached without a tenant refuses before it touches the owner.
func TestPhotoRoutesRequireATenantContext(t *testing.T) {
	t.Parallel()

	seam := newPhotoSeam(t, &stubPhotoDirectory{}, testpkg.NewRecordingBroadcaster(), nil, slog.Default())

	require.ErrorIs(t,
		seam.CommitUpload(context.Background(), userService.CommitUploadRequest{StudentID: 1}),
		userService.ErrPhotoNoTenant)
	_, err := seam.CommitDelete(context.Background(), 1)
	require.ErrorIs(t, err, userService.ErrPhotoNoTenant)
	_, err = seam.LookupForRead(context.Background(), 1, "a.jpg")
	require.ErrorIs(t, err, userService.ErrPhotoNoTenant)
}
