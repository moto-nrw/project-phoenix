package application

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type erasingStore struct {
	erased  []domain.ErasedAccount
	pending []domain.AvatarCleanup
	err     error
}

func (s *erasingStore) EraseUnusedAccounts(context.Context) ([]domain.ErasedAccount, domain.OperationStats, error) {
	return s.erased, domain.OperationStats{}, s.err
}
func (s *erasingStore) PendingAvatarCleanups(context.Context) ([]domain.AvatarCleanup, error) {
	return s.pending, s.err
}
func (s *erasingStore) FinishAvatarCleanup(_ context.Context, id int64, removed bool) error {
	if removed {
		for i, entry := range s.pending {
			if entry.ID == id {
				s.pending = append(s.pending[:i], s.pending[i+1:]...)
				break
			}
		}
	}
	return s.err
}
func unattached(ctx context.Context) context.Context                        { return ctx }
func runAdminNow(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

func TestAccountRetentionRemovesUploadedAvatarFiles(t *testing.T) {
	t.Parallel()
	store := &erasingStore{pending: []domain.AvatarCleanup{
		{ID: 41, Avatar: "/uploads/avatars/a.png"},
		{ID: 42, Avatar: "/uploads/avatars/missing.png"},
	}}
	var removed []string
	retention := NewAccountRetention(store, unattached, runAdminNow, nil, nil)
	retention.removeFile = func(path string) error {
		removed = append(removed, path)
		if path == "/uploads/avatars/missing.png" {
			return fs.ErrNotExist
		}
		return nil
	}
	count, err := retention.CleanupErasedAccountAvatars(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Equal(t, []string{"/uploads/avatars/a.png", "/uploads/avatars/missing.png"}, removed)
	assert.Empty(t, store.pending)
}

func TestAccountRetentionRemovesAvatarsOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	store := &erasingStore{erased: []domain.ErasedAccount{{ID: 41, AvatarFiles: []string{"/uploads/avatars/a.png"}}}}
	retention := NewAccountRetention(store, unattached, runAdminNow, nil, nil)
	retention.removeFile = func(string) error { t.Fatal("erasure must never touch the filesystem before commit"); return nil }
	erased, err := retention.EraseUnusedAccounts(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, erased)
}

func TestAccountRetentionReportsAFailedErasure(t *testing.T) {
	t.Parallel()
	retention := NewAccountRetention(&erasingStore{err: errors.New("permission denied")}, unattached, runAdminNow, nil, nil)
	retention.removeFile = func(string) error { t.Fatal("no file goes when the erasure failed"); return nil }
	erased, err := retention.EraseUnusedAccounts(context.Background())
	require.ErrorContains(t, err, "permission denied")
	assert.Zero(t, erased)
}

func TestAccountRetentionRetriesFileFailures(t *testing.T) {
	t.Parallel()
	store := &erasingStore{pending: []domain.AvatarCleanup{{ID: 41, Avatar: "/uploads/avatars/global/retry.png"}}}
	retention := NewAccountRetention(store, unattached, runAdminNow, nil, nil)
	retention.removeFile = func(string) error {
		return &fs.PathError{Op: "remove", Path: "private-file-path", Err: fs.ErrPermission}
	}
	count, err := retention.CleanupErasedAccountAvatars(context.Background())
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "private-file-path")
	assert.Zero(t, count)
	require.Len(t, store.pending, 1)
	retention.removeFile = func(string) error { return nil }
	count, err = retention.CleanupErasedAccountAvatars(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Empty(t, store.pending)
}

func TestAccountRetentionKeepsIntentsWhenAvatarRootIsMissing(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "avatars")
	store := &erasingStore{pending: []domain.AvatarCleanup{{ID: 41, Avatar: "/uploads/avatars/global/missing.png"}}}
	retention := NewAccountRetention(store, unattached, runAdminNow, func(avatar string) error {
		return removeErasedAvatarFileFrom(root, avatar)
	}, nil)
	count, err := retention.CleanupErasedAccountAvatars(t.Context())
	require.Error(t, err, "a missing storage root must not acknowledge deletion intents")
	assert.NotContains(t, err.Error(), root)
	assert.Zero(t, count)
	require.Len(t, store.pending, 1)

	// Once storage returns, only the absent file is acknowledged as removed.
	require.NoError(t, os.Mkdir(root, 0o700))
	count, err = retention.CleanupErasedAccountAvatars(t.Context())
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Empty(t, store.pending)
}

func TestAccountAvatarRemovalStaysInsideAvatarStorage(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "avatars")
	for _, folder := range []string{"global", "123"} {
		require.NoError(t, os.MkdirAll(filepath.Join(root, folder), 0o700))
		require.NoError(t, os.WriteFile(filepath.Join(root, folder, "synthetic.png"), []byte("synthetic-avatar"), 0o600))
		require.NoError(t, removeErasedAvatarFileFrom(root, "/uploads/avatars/"+folder+"/synthetic.png"))
		_, err := os.Stat(filepath.Join(root, folder, "synthetic.png"))
		require.ErrorIs(t, err, fs.ErrNotExist)
	}
	outside := filepath.Join(base, "untouched.png")
	require.NoError(t, os.WriteFile(outside, []byte("unrelated-file"), 0o600))
	require.NoError(t, os.Symlink(base, filepath.Join(root, "escape")))
	for _, avatar := range []string{
		"https://example.invalid/external.png",
		"/uploads/avatars/../untouched.png",
		"/uploads/avatars/escape/untouched.png",
		"/uploads/avatars/",
	} {
		require.Error(t, removeErasedAvatarFileFrom(root, avatar), avatar)
	}
	_, err := os.Stat(outside)
	require.NoError(t, err)
}
