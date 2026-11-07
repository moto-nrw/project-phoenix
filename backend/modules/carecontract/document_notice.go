package carecontract

import "context"

// The push „Neues Dokument vom Träger“ of a sent contract (#5941). The first
// send of a contract to an address puts a deferred push on Delivery's outbox
// in the same Träger transaction as the mailing; the worker finds the
// account of the address and checks its relationship to the contract's child
// when it sends. A further send to the same address, a reminder or a draft
// puts nothing there. The mailing history is the durable proof: it outlives
// the outbox's retention.

// PushKindContractDocument is the outbox kind of that push. It differs from
// the Elternabrechnung's kind, so the worker routes it to the contract check.
const PushKindContractDocument = "care_contract_document"

// PushRelatedContractDocument is the related entity type of that push. A
// contract and a billing document may share a numeric ID; the type keeps
// their pushes apart.
const PushRelatedContractDocument = "care_contract_document"

// ContractDocumentNotice is the deferred push of a contract sent to
// RecipientEmail. It names no child and no device: the worker resolves the
// account, its relationship and its devices when it sends.
type ContractDocumentNotice struct {
	OrganizationID int64  `json:"organization_id"`
	ContractID     int64  `json:"contract_id"`
	RecipientEmail string `json:"recipient_email"`
}

// DocumentNotices puts the push of a sent contract on Delivery's outbox in
// the caller's Träger transaction. The key names the contract and the
// address, so a retry of the same send is a duplicate.
type DocumentNotices interface {
	EnqueueContractDocumentNotice(ctx context.Context, notice ContractDocumentNotice, idempotencyKey string) error
}

// ContractNoticeTarget is the school and the child of a contract whose push
// the worker sends.
type ContractNoticeTarget struct {
	SchoolID  int64
	StudentID int64
}

// InboxNotices tells the worker, in the caller's Träger transaction, whether
// a contract still lies in the inbox of an address: sent to it, assigned to a
// child and neither a draft nor ended, terminated or withdrawn. found is
// false otherwise. It records no read: a push opens nothing.
type InboxNotices interface {
	ContractNoticeTarget(ctx context.Context, organizationID, contractID int64, email string) (target ContractNoticeTarget, found bool, err error)
}
