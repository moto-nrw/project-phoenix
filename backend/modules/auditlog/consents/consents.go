package consents

import (
	"context"
	"time"
)

const ConsentSourceImport = "import"

// ConsentSnapshot contains only the live consent facts compared by Audit.
type ConsentSnapshot struct {
	StudentID                                                                            int64
	AGBAcceptedAt, DataProcessingAcceptedAt, EmailContactAcceptedAt, PhotoConsentGivenAt *time.Time
}

// ConsentChange is an append-only event, with no persistence implementation types.
type ConsentChange struct {
	TenantID, StudentID        int64
	ConsentKey, Action, Source string
	ActorAccountID             *int64
	ChangedAt                  time.Time
}

func (e *ConsentChange) GetTenantID() int64   { return e.TenantID }
func (e *ConsentChange) SetTenantID(id int64) { e.TenantID = id }

type ConsentTransitions interface {
	RecordTransitions(context.Context, *ConsentSnapshot, *ConsentSnapshot, string, *int64, time.Time) error
}
