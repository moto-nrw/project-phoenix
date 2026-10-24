package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
)

// SignatureLedger is the Audit Platform's signature store (#4785) over the
// Security Runtime's SHA-256 content identity. Every consumer appends and
// reads inside its own tenant unit of work.
func SignatureLedger() (auditlog.SignatureLedger, error) {
	return auditCompose.NewSignatureLedger(securityruntime.Fingerprint)
}

// CommunicationSignatures binds the ledger to Communication's reader of the
// signatures of signed Erklärungen.
type CommunicationSignatures struct{ Ledger auditlog.SignatureLedger }

func (c CommunicationSignatures) DeclarationSignatures(ctx context.Context, ids []int64) (map[int64]communication.DeclarationSignature, error) {
	stored, err := c.Ledger.Signatures(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]communication.DeclarationSignature, len(stored))
	for id, s := range stored {
		out[id] = communication.DeclarationSignature{
			Method: s.Method, TypedName: s.TypedName, PNG: s.PNG, ImageSHA256: s.ImageSHA256,
			DocumentSHA256: s.DocumentSHA256, SignedAt: s.SignedAt, IntegrityOK: s.IntegrityOK,
		}
	}
	return out, nil
}
