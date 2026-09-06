package compose

import (
	"context"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/exporttransfer/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Resolving the counterpart from the tenant settings (#3050). Every test here
// circles one rule: an incomplete or switched-off configuration produces NO
// target, so no connection is ever opened from one.

const (
	keyHost        = "sftp.host"
	keyPort        = "sftp.port"
	keyUsername    = "sftp.username"
	keyPassword    = "sftp.password"
	keyDirectory   = "sftp.remote_directory"
	keyFingerprint = "sftp.host_key_fingerprint"
)

func testKeys() SettingKeys {
	return SettingKeys{
		Host:               keyHost,
		Port:               keyPort,
		Username:           keyUsername,
		Password:           keyPassword,
		RemoteDirectory:    keyDirectory,
		HostKeyFingerprint: keyFingerprint,
	}
}

// completeValues is a fully configured counterpart. Individual tests remove or
// damage exactly one value, so a failure names the field that caused it.
func completeValues() map[string]string {
	return map[string]string{
		keyHost:        "dateien.beispiel.de",
		keyUsername:    "lohn-export",
		keyPassword:    "s3hr-geheim",
		keyDirectory:   "/upload/lohn",
		keyFingerprint: "SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU",
	}
}

func newResolver(enabled bool, port int, values map[string]string) settingsResolver {
	read := func(key string) func(context.Context) (string, error) {
		return func(context.Context) (string, error) { return values[key], nil }
	}
	return settingsResolver{
		settings: Settings{
			Enabled:            func(context.Context) (bool, error) { return enabled, nil },
			Port:               func(context.Context) (int, error) { return port, nil },
			Host:               read(keyHost),
			Username:           read(keyUsername),
			Password:           read(keyPassword),
			RemoteDirectory:    read(keyDirectory),
			HostKeyFingerprint: read(keyFingerprint),
		},
		keys: testKeys(),
	}
}

func TestResolve_CompleteConfiguration(t *testing.T) {
	t.Parallel()

	target, err := newResolver(true, 2222, completeValues()).Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "dateien.beispiel.de", target.Host)
	assert.Equal(t, 2222, target.Port)
	assert.Equal(t, "lohn-export", target.Username)
	assert.Equal(t, "s3hr-geheim", target.Password)
	assert.Equal(t, "/upload/lohn", target.RemoteDirectory)
}

// A complete form is not consent to transmit.
func TestResolve_SwitchedOffYieldsNoTarget(t *testing.T) {
	t.Parallel()

	_, err := newResolver(false, 22, completeValues()).Resolve(context.Background())
	require.ErrorIs(t, err, domain.ErrNotConfigured)
}

func TestResolve_EachMissingValueBlocksTransfer(t *testing.T) {
	t.Parallel()

	for _, key := range []string{keyHost, keyUsername, keyPassword, keyDirectory, keyFingerprint} {
		t.Run(key, func(t *testing.T) {
			t.Parallel()

			values := completeValues()
			delete(values, key)

			_, err := newResolver(true, 22, values).Resolve(context.Background())
			require.ErrorIsf(t, err, domain.ErrNotConfigured, "missing %s must block the transfer", key)
			assert.Contains(t, err.Error(), key)
		})
	}
}

// The missing fingerprint is the one people will ask to skip. There is no
// trust-on-first-use path: without a verified key the transfer stays blocked.
func TestResolve_RejectsFingerprintShapesThatAreNotSHA256(t *testing.T) {
	t.Parallel()

	for name, fingerprint := range map[string]string{
		"md5 fingerprint":  "MD5:1f:aa:bb:cc:dd:ee:ff:00:11:22:33:44:55:66:77:88",
		"bare base64":      "47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU",
		"truncated digest": "SHA256:47DEQpj8HBSa",
		"whole public key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIexample",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			values := completeValues()
			values[keyFingerprint] = fingerprint

			_, err := newResolver(true, 22, values).Resolve(context.Background())
			require.ErrorIs(t, err, domain.ErrNotConfigured)
		})
	}
}

// The directory is never taken from the browser, but it IS typed into a
// settings field. A traversal segment would put payroll files outside the
// agreed directory.
func TestResolve_RejectsDirectoryTraversalAndRelativePaths(t *testing.T) {
	t.Parallel()

	for name, dir := range map[string]string{
		"traversal segment":  "/upload/../../etc",
		"trailing traversal": "/upload/..",
		"relative path":      "upload/lohn",
		"empty":              "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			values := completeValues()
			values[keyDirectory] = dir

			_, err := newResolver(true, 22, values).Resolve(context.Background())
			require.ErrorIs(t, err, domain.ErrNotConfigured)
		})
	}
}

// A directory whose NAME merely contains dots is legitimate — only a whole
// ".." segment is traversal.
func TestResolve_AcceptsDottedDirectoryNames(t *testing.T) {
	t.Parallel()

	values := completeValues()
	values[keyDirectory] = "/upload/lohn.2026/..daten"

	target, err := newResolver(true, 22, values).Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "/upload/lohn.2026/..daten", target.RemoteDirectory)
}

func TestResolve_RejectsPortOutsideRange(t *testing.T) {
	t.Parallel()

	for _, port := range []int{0, -1, 65536} {
		_, err := newResolver(true, port, completeValues()).Resolve(context.Background())
		require.ErrorIsf(t, err, domain.ErrNotConfigured, "port %d must block the transfer", port)
	}
}

// A password is taken exactly as stored: trimming it would authenticate with a
// different secret than the school entered.
func TestResolve_PreservesPasswordWhitespace(t *testing.T) {
	t.Parallel()

	values := completeValues()
	values[keyPassword] = "  geheim mit rand  "

	target, err := newResolver(true, 22, values).Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "  geheim mit rand  ", target.Password)
}

// Copy-paste whitespace around a host or fingerprint is an artefact, not part
// of the value, and must not present as "not configured".
func TestResolve_TrimsPastedWhitespace(t *testing.T) {
	t.Parallel()

	values := completeValues()
	values[keyHost] = "  dateien.beispiel.de\n"
	values[keyFingerprint] = " SHA256:47DEQpj8HBSa+/TImW+5JCeuQeRkm5NMpJWZG3hSuFU "

	target, err := newResolver(true, 22, values).Resolve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "dateien.beispiel.de", target.Host)
}

// A settings store that cannot answer is NOT an unconfigured school. Mapping
// the two onto each other would report an outage as "please fill in the form"
// and hide it from whoever has to fix it.
func TestResolve_StoreFailureIsNotReportedAsUnconfigured(t *testing.T) {
	t.Parallel()

	boom := errors.New("settings unavailable")
	resolver := newResolver(true, 22, completeValues())
	resolver.settings.Enabled = func(context.Context) (bool, error) { return false, boom }

	_, err := resolver.Resolve(context.Background())
	require.Error(t, err)
	assert.ErrorIs(t, err, boom)
	assert.NotErrorIs(t, err, domain.ErrNotConfigured)
}

func TestState_NamesEveryMissingSettingWithoutLeakingThePassword(t *testing.T) {
	t.Parallel()

	values := completeValues()
	delete(values, keyUsername)
	delete(values, keyPassword)

	state, err := newResolver(true, 22, values).State(context.Background())
	require.NoError(t, err)
	assert.True(t, state.Enabled)
	assert.False(t, state.Ready(), "an incomplete target is never ready")
	assert.ElementsMatch(t, []string{keyUsername, keyPassword}, state.MissingSettings)
	// The destination is named so nobody transfers to a place they cannot see;
	// the credentials are not part of the state at all.
	assert.Equal(t, "dateien.beispiel.de", state.Host)
	assert.Equal(t, "/upload/lohn", state.RemoteDirectory)
}

func TestState_ReadyOnlyWhenSwitchedOnAndComplete(t *testing.T) {
	t.Parallel()

	ready, err := newResolver(true, 22, completeValues()).State(context.Background())
	require.NoError(t, err)
	assert.True(t, ready.Ready())
	assert.Empty(t, ready.MissingSettings)

	off, err := newResolver(false, 22, completeValues()).State(context.Background())
	require.NoError(t, err)
	assert.False(t, off.Ready(), "a complete but switched-off target must not read as ready")
	assert.Empty(t, off.MissingSettings, "nothing is missing — it is simply off")
}
