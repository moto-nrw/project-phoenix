package test

import (
	"io"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// ContentFingerprint verifies SHA-256 artifact identities in behavior tests
// without giving those suites a general cryptographic implementation import.
func ContentFingerprint(content []byte) string { return securityruntime.Fingerprint(content) }

func ReaderContentFingerprint(reader io.Reader) (string, int64, error) {
	return securityruntime.ReaderFingerprint(reader)
}

// The data return cryptography (ADR 0201) for behavior tests, as the root
// binds it, without a crypto import there.
var (
	DataReturnNewToken      = securityruntime.NewDataReturnToken
	DataReturnTokenLookup   = securityruntime.DataReturnTokenLookup
	DataReturnNewArchiveKey = securityruntime.NewDataReturnArchiveKey
	DataReturnKeyCheck      = securityruntime.DataReturnKeyCheck
	DataReturnKeyMatches    = securityruntime.DataReturnKeyMatches
	DataReturnSealKey       = securityruntime.SealDataReturnKey
	DataReturnOpenKey       = securityruntime.OpenDataReturnKey
	DataReturnEncrypt       = securityruntime.EncryptDataReturnArchive
	DataReturnDecrypt       = securityruntime.DecryptDataReturnArchive
	DataReturnFingerprint   = securityruntime.NewDataReturnFingerprint
)
