package sftp_test

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp/sftptest"
)

// The in-process counterpart these tests drive lives in sftptest, so the
// capability's integration test uses the very same server.

type testServer struct{ sftptest.Server }

func startTestSFTPServer(t *testing.T) testServer {
	t.Helper()
	return testServer{Server: sftptest.Start(t)}
}

// target builds a client Target pointing at this server.
func (s testServer) target(dir string) sftp.Target {
	return sftp.Target{
		Host:               s.Host,
		Port:               s.Port,
		Username:           sftptest.User,
		Password:           sftptest.Password,
		RemoteDirectory:    dir,
		HostKeyFingerprint: s.Fingerprint,
	}
}

func (s testServer) address() string { return s.Address() }

type allowLoopbackPolicy = sftptest.AllowLoopbackPolicy
