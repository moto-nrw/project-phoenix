package care

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
)

// PickupProof is the guardian's proof of one signed request (#4780): what
// was asked, the wording signed and the stored signature. Intact is false
// when the signature, its image or the wording no longer match their stored
// checksums.
type PickupProof struct {
	Request    carerequests.Request
	ChildName  string
	SchoolName string
	Statement  string
	Summary    []carerequests.DiffEntry
	Signature  *PickupSignature
	Intact     bool
}

// PickupRequestProof returns the proof of a signed request the caller filed
// for the child. Other guardians' signatures are never shown in the portal,
// so a request of another account is not found.
func (s *Service) PickupRequestProof(ctx context.Context, accountID, studentID, requestID int64) (*PickupProof, error) {
	child, err := s.ResolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if s.CareRequests == nil || s.PickupSignatures == nil {
		return nil, errors.New("parent: pickup permission proof is not configured")
	}
	var proof *PickupProof
	err = s.Runtime.WithinTenant(ctx, child.TenantID, func(txCtx context.Context) error {
		var readErr error
		proof, readErr = s.pickupProofInTx(txCtx, child, accountID, studentID, requestID)
		return readErr
	})
	if err != nil {
		if errors.Is(err, ErrPickupProofNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("parent: pickup permission proof: %w", err)
	}
	return proof, nil
}

// pickupProofInTx reads the caller's own signed request and its signature.
func (s *Service) pickupProofInTx(txCtx context.Context, child *Child, accountID, studentID, requestID int64) (*PickupProof, error) {
	request, err := s.CareRequests.FindStudentRequest(txCtx, studentID, requestID)
	if errors.Is(err, carerequests.ErrNotFound) {
		return nil, ErrPickupProofNotFound
	}
	if err != nil {
		return nil, err
	}
	signed := carerequests.StoredSignature(request.Payload)
	if request.SubmittedBy != accountID || signed == nil {
		return nil, ErrPickupProofNotFound
	}
	signatures, err := s.PickupSignatures.PickupSignatures(txCtx, studentID, map[int64]string{signed.SignatureID: signed.Statement})
	if err != nil {
		return nil, err
	}
	signature := signatures[signed.SignatureID]
	return &PickupProof{
		Request: *request, ChildName: child.StudentName, SchoolName: child.SchoolName,
		Statement: signed.Statement, Summary: carerequests.RequestedSummary(request.Payload),
		Signature: signature, Intact: signature != nil && signature.Intact,
	}, nil
}
