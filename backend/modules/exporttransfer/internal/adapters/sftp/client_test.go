package sftp_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/adapters/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Integration tests against the in-process SFTP server (#3050). They cover
// the acceptance criteria that can only be shown against a real server:
// the file arrives byte-identical under its final name, a partial upload
// never appears under that name, and a wrong host key stops the transfer.

func newTestClient(t *testing.T) *sftp.Client {
	t.Helper()
	return sftp.New(
		sftp.WithAddressPolicy(allowLoopbackPolicy{}),
		sftp.WithTimeout(15*time.Second),
	)
}

func TestUpload_DeliversFileUnderFinalName(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()
	payload := []byte("Personalnummer;Lohnart;Stunden\r\n4711;100;38,50\r\n")

	err := newTestClient(t).Upload(
		context.Background(),
		server.target(dir),
		"zeitkonten-2026-08.csv",
		payload,
	)
	require.NoError(t, err)

	written, err := os.ReadFile(filepath.Join(dir, "zeitkonten-2026-08.csv"))
	require.NoError(t, err)
	assert.Equal(t, payload, written, "the transferred file must be byte-identical to the export")
}

// The temporary name exists only during the upload. Afterwards the directory
// holds exactly the final file — a payroll office that lists the directory
// must not have to guess which of two files is the real one.
func TestUpload_LeavesNoTemporaryFileBehind(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()

	require.NoError(t, newTestClient(t).Upload(
		context.Background(),
		server.target(dir),
		"zeitkonten-2026-08.csv",
		[]byte("inhalt"),
	))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1, "only the final file may remain")
	assert.Equal(t, "zeitkonten-2026-08.csv", entries[0].Name())
}

// A failing upload must leave the directory as it was. In particular the
// final name must not exist, so nobody imports an empty or partial file.
func TestUpload_FailedTransferLeavesNoFinalFile(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()
	missingDir := filepath.Join(dir, "gibt-es-nicht")

	err := newTestClient(t).Upload(
		context.Background(),
		server.target(missingDir),
		"zeitkonten-2026-08.csv",
		[]byte("inhalt"),
	)
	require.Error(t, err)
	assert.ErrorIs(t, err, sftp.ErrUpload)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "a failed transfer must not create anything")
}

// Re-transferring a corrected month is the normal case, so the final name is
// replaced rather than refused.
func TestUpload_ReplacesAnEarlierTransferOfTheSameFile(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()
	client := newTestClient(t)
	target := server.target(dir)

	require.NoError(t, client.Upload(context.Background(), target, "monat.csv", []byte("erste fassung")))
	require.NoError(t, client.Upload(context.Background(), target, "monat.csv", []byte("korrigiert")))

	written, err := os.ReadFile(filepath.Join(dir, "monat.csv"))
	require.NoError(t, err)
	assert.Equal(t, []byte("korrigiert"), written)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

// The host key is the whole guarantee that the payroll file reaches the
// agreed counterpart. A mismatch aborts before any data is sent.
func TestUpload_RefusesAConnectionWithTheWrongHostKey(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()
	target := server.target(dir)
	target.HostKeyFingerprint = "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU"

	err := newTestClient(t).Upload(context.Background(), target, "monat.csv", []byte("inhalt"))
	require.ErrorIs(t, err, sftp.ErrHostKeyMismatch)

	// The error names the fingerprint the counterpart actually presented. A
	// counterpart usually offers several host keys and the negotiated one
	// decides which arrives, so without this the mismatch is undiagnosable
	// from the log — and it is the only place that detail may appear.
	assert.Contains(t, err.Error(), server.Fingerprint,
		"the log must name the presented fingerprint")
	assert.Contains(t, err.Error(), target.HostKeyFingerprint,
		"and the configured one, so the two can be compared")

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "nothing may be written to an unverified counterpart")
}

func TestUpload_ReportsRejectedCredentials(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()
	target := server.target(dir)
	target.Password = "falsch"

	err := newTestClient(t).Upload(context.Background(), target, "monat.csv", []byte("inhalt"))
	require.ErrorIs(t, err, sftp.ErrAuthFailed)
}

// The address policy is not a formality that the dialer can bypass: a
// production client must refuse a loopback target even when something is
// listening there.
func TestUpload_ProductionPolicyRefusesLoopbackTargets(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()

	// sftp.New() without WithAddressPolicy — the production configuration.
	err := sftp.New(sftp.WithTimeout(5*time.Second)).Upload(
		context.Background(),
		server.target(dir),
		"monat.csv",
		[]byte("inhalt"),
	)
	require.ErrorIs(t, err, sftp.ErrAddressNotAllowed)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestUpload_RefusesFilesOverTheSizeLimit(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()

	client := sftp.New(sftp.WithAddressPolicy(allowLoopbackPolicy{}), sftp.WithMaxBytes(10))
	err := client.Upload(context.Background(), server.target(dir), "monat.csv", []byte("mehr als zehn bytes"))
	require.ErrorIs(t, err, sftp.ErrFileTooLarge)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries, "the size check must happen before any connection")
}

// The filename comes from the export service, never from the browser. The
// check stays because the remote path is assembled by concatenation.
func TestValidateFilename(t *testing.T) {
	t.Parallel()

	valid := []string{
		"zeitkonten-2026-08.csv",
		"LODAS_2026_08.txt",
		"Zeitkonten Übersicht.xlsx",
	}
	for _, name := range valid {
		assert.NoErrorf(t, sftp.ValidateFilename(name), "%q should be accepted", name)
	}

	invalid := map[string]string{
		"empty":              "",
		"absolute path":      "/etc/passwd",
		"relative traversal": "../../etc/passwd",
		"parent directory":   "..",
		"current directory":  ".",
		"backslash":          `ordner\datei.csv`,
		"newline":            "datei\n.csv",
		"null byte":          "datei\x00.csv",
		"too long":           strings.Repeat("a", 256),
	}
	for name, filename := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, sftp.ValidateFilename(filename), sftp.ErrInvalidFilename)
		})
	}
}

// A filename that survived validation still must not escape the configured
// directory once it is joined onto the remote path.
func TestUpload_RejectsFilenameBeforeConnecting(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	// No server at all: if the client tried to connect, this would be a
	// connection error rather than a filename error.
	client := sftp.New(sftp.WithAddressPolicy(allowLoopbackPolicy{}))
	err := client.Upload(context.Background(), sftp.Target{
		Host:            "127.0.0.1",
		Port:            1,
		RemoteDirectory: dir,
	}, "../entwischt.csv", []byte("inhalt"))

	require.ErrorIs(t, err, sftp.ErrInvalidFilename)
}

func TestUpload_HonoursACancelledContext(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	dir := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := newTestClient(t).Upload(ctx, server.target(dir), "monat.csv", []byte("inhalt"))
	require.Error(t, err)
	assert.ErrorIs(t, err, sftp.ErrConnect)
}

// A sanity check that the test server is reachable at all — otherwise a
// green "refused" test could be green for the wrong reason.
func TestTestServerIsReachable(t *testing.T) {
	t.Parallel()

	server := startTestSFTPServer(t)
	conn, err := net.DialTimeout("tcp", server.address(), 5*time.Second)
	require.NoError(t, err)
	require.NoError(t, conn.Close())
}
