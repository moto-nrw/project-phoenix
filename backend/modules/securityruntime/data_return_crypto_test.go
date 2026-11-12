package securityruntime_test

import (
	"bytes"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/moto-ogs/project-phoenix/modules/securityruntime"
	"github.com/stretchr/testify/require"
)

func TestDataReturnArchiveRoundTripsAtChunkEdges(t *testing.T) {
	t.Parallel()
	key, err := securityruntime.NewDataReturnArchiveKey()
	require.NoError(t, err)
	for _, size := range []int{0, 1, 64<<10 - 1, 64 << 10, 64<<10 + 1, 3 * 64 << 10, 200_000} {
		plain := make([]byte, size)
		_, _ = rand.Read(plain)
		var sealed bytes.Buffer
		require.NoError(t, securityruntime.EncryptDataReturnArchive(key, &sealed, bytes.NewReader(plain)))
		require.False(t, size > 16 && bytes.Contains(sealed.Bytes(), plain[:16]), "plaintext leaks at size %d", size)
		var opened bytes.Buffer
		require.NoError(t, securityruntime.DecryptDataReturnArchive(key, &opened, bytes.NewReader(sealed.Bytes())), "size %d", size)
		require.Equal(t, len(plain), opened.Len(), "size %d", size)
		require.True(t, bytes.Equal(plain, opened.Bytes()), "size %d", size)
	}
}

func TestDataReturnArchiveRefusesWrongKeyTruncationAndTampering(t *testing.T) {
	t.Parallel()
	key, err := securityruntime.NewDataReturnArchiveKey()
	require.NoError(t, err)
	other, err := securityruntime.NewDataReturnArchiveKey()
	require.NoError(t, err)
	plain := bytes.Repeat([]byte("moto"), 40_000) // three chunks
	var sealed bytes.Buffer
	require.NoError(t, securityruntime.EncryptDataReturnArchive(key, &sealed, bytes.NewReader(plain)))
	data := sealed.Bytes()

	require.ErrorIs(t, securityruntime.DecryptDataReturnArchive(other, &bytes.Buffer{}, bytes.NewReader(data)), securityruntime.ErrDataReturnCiphertext)
	// Cut after the first sealed chunk: it was not sealed as the last one.
	cut := 4 + 64<<10 + 16
	require.ErrorIs(t, securityruntime.DecryptDataReturnArchive(key, &bytes.Buffer{}, bytes.NewReader(data[:cut])), securityruntime.ErrDataReturnCiphertext)
	tampered := bytes.Clone(data)
	tampered[len(tampered)-20] ^= 1
	require.ErrorIs(t, securityruntime.DecryptDataReturnArchive(key, &bytes.Buffer{}, bytes.NewReader(tampered)), securityruntime.ErrDataReturnCiphertext)
}

func TestDataReturnTokenWrapsTheArchiveKey(t *testing.T) {
	t.Parallel()
	key, err := securityruntime.NewDataReturnArchiveKey()
	require.NoError(t, err)
	token, err := securityruntime.NewDataReturnToken()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(token, securityruntime.DataReturnTokenPrefix))
	require.Len(t, token, len(securityruntime.DataReturnTokenPrefix)+43)
	other, err := securityruntime.NewDataReturnToken()
	require.NoError(t, err)
	require.NotEqual(t, token, other)

	sealed, err := securityruntime.SealDataReturnKey(token, key)
	require.NoError(t, err)
	opened, err := securityruntime.OpenDataReturnKey(token, sealed)
	require.NoError(t, err)
	require.Equal(t, key, opened)
	_, err = securityruntime.OpenDataReturnKey(other, sealed)
	require.ErrorIs(t, err, securityruntime.ErrDataReturnCiphertext)

	lookup := securityruntime.DataReturnTokenLookup(token)
	require.Len(t, lookup, 64)
	require.NotContains(t, lookup, token)
	require.NotEqual(t, lookup, securityruntime.DataReturnTokenLookup(other))

	require.True(t, securityruntime.DataReturnKeyMatches(key, securityruntime.DataReturnKeyCheck(key)))
	require.False(t, securityruntime.DataReturnKeyMatches(key, securityruntime.DataReturnKeyCheck(opened[:0:0])))
	decoded, err := securityruntime.DecodeDataReturnArchiveKey(securityruntime.EncodeDataReturnArchiveKey(key))
	require.NoError(t, err)
	require.Equal(t, key, decoded)
	_, err = securityruntime.DecodeDataReturnArchiveKey("short")
	require.Error(t, err)
}
