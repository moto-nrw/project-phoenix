package api

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	careStudentsHTTP "github.com/moto-nrw/project-phoenix/modules/careplan/http"
	"github.com/moto-nrw/project-phoenix/roots/owners"
)

// staffPickupSignatures binds the Audit Platform's signature ledger to the
// OGS proof of a signed parent request (#4780).
type staffPickupSignatures struct{ ledger auditlog.SignatureLedger }

func (s staffPickupSignatures) PickupSignatureProof(ctx context.Context, studentID, signatureID int64, statement string) (*careStudentsHTTP.PickupSignatureProof, error) {
	proofs, err := owners.PickupSignatureProofs(ctx, s.ledger, studentID, map[int64]string{signatureID: statement})
	if err != nil {
		return nil, err
	}
	proof, ok := proofs[signatureID]
	if !ok {
		return nil, nil
	}
	return &careStudentsHTTP.PickupSignatureProof{
		Method: proof.Method, TypedName: proof.TypedName, SignerName: proof.SignerName,
		PNG: proof.PNG, SignedAt: proof.SignedAt, Intact: proof.Intact,
	}, nil
}
