package sftp_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-ogs/project-phoenix/modules/exporttransfer/internal/adapters/sftp/sftptest"
)

// A counterpart may answer with the password it received. Its text never
// reaches the error, which ends in the log (ADR 0218 section 3); the reason
// and the SFTP status code stay.
func TestUpload_KeepsTheCounterpartsTextOutOfTheError(t *testing.T) {
	t.Parallel()
	server := testServer{Server: sftptest.StartEchoing(t)}
	err := newTestClient(t).Upload(context.Background(), server.target(t.TempDir()), "zeitkonten.csv", []byte("4711;100\r\n"))
	require.ErrorIs(t, err, sftp.ErrUpload)
	assert.NotContains(t, err.Error(), server.Password)
	assert.Contains(t, err.Error(), "SFTP status")
}

// A counterpart that drops the connection on the rename leaves the outcome
// open: the file may sit under its final name (ADR 0218 section 6).
func TestUpload_AnUnansweredRenameLeavesTheOutcomeOpen(t *testing.T) {
	t.Parallel()
	server := testServer{Server: sftptest.StartDroppingRename(t)}
	err := newTestClient(t).Upload(context.Background(), server.target("/"), "zeitkonten.csv", []byte("4711;100\r\n"))
	require.ErrorIs(t, err, sftp.ErrUpload)
	var uncertain interface{ OutcomeUncertain() bool }
	require.ErrorAs(t, err, &uncertain)
	assert.True(t, uncertain.OutcomeUncertain())
}

// A rename the counterpart refused with an SFTP status failed for certain.
// pkg/sftp hands a denied rename on as os.ErrPermission: an answer too.
func TestUpload_ARefusedRenameFailsForCertain(t *testing.T) {
	t.Parallel()
	for name, start := range map[string]func(*testing.T) sftptest.Server{
		"failure status": sftptest.StartRefusingRename, "permission denied": sftptest.StartDenyingRename,
	} {
		server := testServer{Server: start(t)}
		err := newTestClient(t).Upload(context.Background(), server.target("/"), "zeitkonten.csv", []byte("4711;100\r\n"))
		require.ErrorIs(t, err, sftp.ErrUpload, name)
		var uncertain interface{ OutcomeUncertain() bool }
		assert.False(t, errors.As(err, &uncertain), "%s is an answer", name)
	}
}
