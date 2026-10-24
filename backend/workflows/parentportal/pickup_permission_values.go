package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"

// Pickup permission values (#4780): the signed pickup permission request, the
// one-day collector, the way home alone and their proof.
type (
	PickupSignatureInput   = care.PickupSignatureInput
	PickupSignatureEntry   = care.PickupSignatureEntry
	PickupSignature        = care.PickupSignature
	PickupSignatureLedger  = care.PickupSignatureLedger
	PickupPermissionInput  = care.PickupPermissionInput
	PickupChangeSubmission = care.PickupChangeSubmission
	PickupChangeEdit       = care.PickupChangeEdit
	PickupProof            = care.PickupProof
)

// Signatures is the shared signature store as the portal uses it: the
// Erklärungen of messaging and the pickup permissions of the child flows.
type Signatures interface {
	SignatureLedger
	PickupSignatureLedger
}

var (
	ErrPickupSignatureRequired         = care.ErrPickupSignatureRequired
	ErrPickupSignatureInvalid          = care.ErrPickupSignatureInvalid
	ErrPickupPersonNotLinked           = care.ErrPickupPersonNotLinked
	ErrPickupPermissionRequestRequired = care.ErrPickupPermissionRequestRequired
	ErrPickupPermissionUnchanged       = care.ErrPickupPermissionUnchanged
	ErrPickupProofNotFound             = care.ErrPickupProofNotFound
)
