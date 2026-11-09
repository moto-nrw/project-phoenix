package fieldcipher

import (
	"crypto/rand"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
)

func bankTestRing(t *testing.T, id string) string {
	t.Helper()
	key := make([]byte, 32)
	_, err := rand.Read(key)
	require.NoError(t, err)
	return id + ":" + base64.StdEncoding.EncodeToString(key)
}

func TestBankCipherTransitionRotationAndIndependence(t *testing.T) {
	t.Parallel()
	healthKeys := bankTestRing(t, "shared-id")
	bankKeys := bankTestRing(t, "shared-id") // identical IDs, different material
	health, err := Parse(healthKeys)
	require.NoError(t, err)
	transition, err := ParseBank(bankKeys, healthKeys)
	require.NoError(t, err)
	aad := []byte("bank-field|organization|reference")
	plain := []byte("synthetic account holder")
	old, err := health.Seal(plain, aad)
	require.NoError(t, err)
	got, err := transition.Open(old, aad)
	require.NoError(t, err)
	require.Equal(t, plain, got)
	migrated, err := transition.Rewrap(old, aad)
	require.NoError(t, err)
	require.NotEqual(t, old, migrated)
	independent, err := ParseBank(bankKeys, "") // health ring removed
	require.NoError(t, err)
	got, err = independent.Open(migrated, aad)
	require.NoError(t, err)
	require.Equal(t, plain, got)
	_, err = independent.Open(old, aad)
	require.ErrorIs(t, err, ErrOpen)
	_, err = health.Open(migrated, aad)
	require.ErrorIs(t, err, ErrOpen)
	repeated, err := independent.Rewrap(migrated, aad)
	require.NoError(t, err)
	require.Equal(t, migrated, repeated)
	rotated, err := ParseBank(bankTestRing(t, "next")+","+bankKeys, "")
	require.NoError(t, err)
	newer, err := rotated.Rewrap(migrated, aad)
	require.NoError(t, err)
	require.Equal(t, "next", string(newer[2:6]))
	_, err = independent.Open(newer, aad)
	require.ErrorIs(t, err, ErrOpen)
	_, err = transition.Rewrap(old, []byte("foreign organization or field"))
	require.ErrorIs(t, err, ErrOpen)
	old[len(old)-1] ^= 1
	_, err = transition.Rewrap(old, aad)
	require.ErrorIs(t, err, ErrOpen)
	fresh, err := transition.Seal(plain, aad)
	require.NoError(t, err)
	_, err = health.Open(fresh, aad)
	require.ErrorIs(t, err, ErrOpen)
}

func TestBankCipherRequiresBankRingEvenWithLegacy(t *testing.T) {
	t.Parallel()
	legacy := bankTestRing(t, "legacy")
	for _, invalid := range []string{"", " ", "not-a-key", "id:c2hvcnQ="} {
		_, err := ParseBank(invalid, legacy)
		require.Error(t, err)
		require.NotContains(t, err.Error(), legacy)
	}
	_, err := ParseBank(bankTestRing(t, "bank"), "invalid legacy")
	require.Error(t, err)
}
