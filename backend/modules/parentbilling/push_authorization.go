package parentbilling

import "context"

// DocumentPushCandidate names one queued announcement, not an inbox read.
// Key is the caller's correlation ID. Checks join the Träger transaction.
type DocumentPushCandidate struct {
	Key        int64  `json:"key"`
	DocumentID int64  `json:"document_id"`
	Email      string `json:"email"`
	// SchoolID and StudentIDs are the school of the push and the children
	// the account's relationship there grants the inbox now; a monthly
	// letter pushes only for its own child (ADR 0165).
	SchoolID   int64   `json:"school_id"`
	StudentIDs []int64 `json:"student_ids"`
}

type DocumentPushValidity interface {
	ValidDocumentPushes(context.Context, int64, []DocumentPushCandidate) (map[int64]bool, error)
}
