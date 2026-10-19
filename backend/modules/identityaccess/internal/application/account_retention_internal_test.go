package application

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

type erasingStore struct {
	erased []domain.ErasedAccount
	err    error
}

func (s erasingStore) EraseUnusedAccounts(context.Context) ([]domain.ErasedAccount, domain.OperationStats, error) {
	return s.erased, domain.OperationStats{}, s.err
}

func unattached(ctx context.Context) context.Context { return ctx }

func runNow(_ context.Context, callback func()) { callback() }

// The hourly erasure (#4043) removes the uploaded avatar files of the
// accounts the database tombstoned. A file already removed by an earlier
// attempt makes the retry idempotent.
func TestAccountRetentionRemovesUploadedAvatarFiles(t *testing.T) {
	t.Parallel()
	var removed []string
	retention := NewAccountRetention(erasingStore{erased: []domain.ErasedAccount{
		{ID: 41, AvatarFiles: []string{"/uploads/avatars/a.png", "https://example.com/external.png"}},
		{ID: 42, AvatarFiles: []string{"/uploads/avatars/missing.png"}},
		{ID: 43},
	}}, unattached, runNow, nil)
	retention.removeFile = func(path string) error {
		removed = append(removed, path)
		if path == "public/uploads/avatars/missing.png" {
			return fs.ErrNotExist
		}
		return nil
	}

	erased, err := retention.EraseUnusedAccounts(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 3, erased)
	assert.Equal(t, []string{"public/uploads/avatars/a.png", "public/uploads/avatars/missing.png"}, removed,
		"only uploads are files of moto")
}

func TestAccountRetentionRemovesAvatarsOnlyAfterCommit(t *testing.T) {
	t.Parallel()
	var afterCommit func()
	retention := NewAccountRetention(erasingStore{erased: []domain.ErasedAccount{{
		ID: 41, AvatarFiles: []string{"/uploads/avatars/a.png"},
	}}}, unattached, func(_ context.Context, callback func()) { afterCommit = callback }, nil)
	removed := false
	retention.removeFile = func(string) error {
		removed = true
		return nil
	}

	erased, err := retention.EraseUnusedAccounts(context.Background())

	require.NoError(t, err)
	assert.Equal(t, 1, erased)
	assert.False(t, removed, "avatar files must stay while the transaction can still roll back")
	require.NotNil(t, afterCommit)
	afterCommit()
	assert.True(t, removed)
}

func TestAccountRetentionReportsAFailedErasure(t *testing.T) {
	t.Parallel()
	retention := NewAccountRetention(erasingStore{err: errors.New("permission denied")}, unattached, runNow, nil)
	retention.removeFile = func(string) error {
		t.Fatal("no file goes when the erasure failed")
		return nil
	}

	erased, err := retention.EraseUnusedAccounts(context.Background())

	require.ErrorContains(t, err, "permission denied")
	assert.Zero(t, erased)
}
