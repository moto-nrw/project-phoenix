package sftp_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp/sftptest"
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
