package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// NewDataReturnCrypto binds the cryptography of the data return (ADR 0203)
// to Security Runtime, for the HTTPS retrieval and the release CLI alike.
func NewDataReturnCrypto() organizationtenancy.DataReturnCrypto {
	return organizationtenancy.DataReturnCrypto{
		NewToken: securityruntime.NewDataReturnToken, TokenLookup: securityruntime.DataReturnTokenLookup,
		NewArchiveKey: securityruntime.NewDataReturnArchiveKey, KeyCheck: securityruntime.DataReturnKeyCheck,
		KeyMatches: securityruntime.DataReturnKeyMatches, SealKey: securityruntime.SealDataReturnKey,
		OpenKey: securityruntime.OpenDataReturnKey, Encrypt: securityruntime.EncryptDataReturnArchive,
		Decrypt: securityruntime.DecryptDataReturnArchive, Fingerprint: securityruntime.NewDataReturnFingerprint,
	}
}
