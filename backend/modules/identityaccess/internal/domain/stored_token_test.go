package domain

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashTokenIsTheHexSHA256(t *testing.T) {
	t.Parallel()
	// FIPS 180-2 test vector for "abc".
	assert.Equal(t, "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad", HashToken("abc"))
}

func TestSuccessorRefreshTokenFollowsOnlyFromTheRawValue(t *testing.T) {
	t.Parallel()
	presented := "9b2f6a8e-3f1c-4a5e-8d0b-6c7e1f2a3b4c"
	successor := SuccessorRefreshToken(presented)

	assert.Equal(t, successor, SuccessorRefreshToken(presented), "a lost rotation response is recomputed from the presented value")
	assert.NotEqual(t, presented, successor)
	assert.NotEqual(t, HashToken(presented), successor, "the stored hash of the predecessor must not be the successor's credential")
	assert.NotEqual(t, successor, SuccessorRefreshToken(HashToken(presented)), "a database reader cannot derive the successor from the stored hash")
	assert.NotEqual(t, successor, SuccessorRefreshToken(presented+"x"))
	assert.Len(t, successor, 43, "32 bytes, unpadded base64url")
}
