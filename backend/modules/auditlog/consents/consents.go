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

// ConsentTransition is one RecordTransitions call of a batch.
type ConsentTransition struct {
	Before, After  *ConsentSnapshot
	Source         string
	ActorAccountID *int64
	ChangedAt      time.Time
}

// ConsentTransitionBatches records the transitions of many students at once
// (#5552, ADR 0067 addendum of 2026-11-08: RecordTransitionsBatch is a
// leaf). Each transition is checked like RecordTransitions; all events go to
// Audit in one AppendAll, all or nothing.
type ConsentTransitionBatches interface {
	RecordTransitionsAll(context.Context, []ConsentTransition) error
}
