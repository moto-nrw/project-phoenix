package erasureledger

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloseReleasesLockWithOutstandingFileReference(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "ledger")
	require.NoError(t, Initialize(path))
	ledger := New(path, func([]byte) string { return "unused" })
	file, err := ledger.open()
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() })

	// Retain the open file description to model delayed kernel cleanup after
	// close. Releasing the lock must not depend on the last reference going away.
	fd, err := syscall.Dup(int(file.Fd()))
	require.NoError(t, err)
	duplicate := os.NewFile(uintptr(fd), path)
	t.Cleanup(func() { _ = duplicate.Close() })
	_, err = ledger.open()
	require.Error(t, err)
	closeLedger(file)

	next, err := ledger.open()
	require.NoError(t, err)
	t.Cleanup(func() { _ = next.Close() })
}
