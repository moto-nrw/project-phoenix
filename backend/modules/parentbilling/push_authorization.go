package parentbilling

import "context"

// DocumentPushCandidate names one queued announcement, not an inbox read.
// Key is the caller's correlation ID. Checks join the Träger transaction.
type DocumentPushCandidate struct {
	Key        int64  `json:"key"`
	DocumentID int64  `json:"document_id"`
	Email      string `json:"email"`
}

type DocumentPushValidity interface {
	ValidDocumentPushes(context.Context, int64, []DocumentPushCandidate) (map[int64]bool, error)
}
