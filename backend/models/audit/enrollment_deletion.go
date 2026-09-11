package audit

import (
	"context"
	"time"
)

const (
	EnrollmentDeletionScopeRequest = "request"
	EnrollmentDeletionScopeChild   = "child"
	EnrollmentDeletionActorAdmin   = "admin"
	EnrollmentDeletionActorSystem  = "system"
)

// EnrollmentDeletion is an append-only, data-minimal audit event. It stores
// identifiers and counts only; no names, contact data, health data, or snapshot.
type EnrollmentDeletion struct {
	ID int64 `bun:"id,pk,autoincrement" json:"id"`
	TenantModel
	RequestID      int64                    `bun:"request_id,notnull" json:"request_id"`
	ChildID        *int64                   `bun:"child_id" json:"child_id,omitempty"`
	ActorAccountID *int64                   `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	ActorType      string                   `bun:"actor_type,notnull" json:"actor_type"`
	Scope          string                   `bun:"scope,notnull" json:"scope"`
	Reason         string                   `bun:"reason,notnull" json:"reason"`
	Counts         EnrollmentDeletionCounts `bun:"counts,type:jsonb,notnull" json:"counts"`
	DeletedAt      time.Time                `bun:"deleted_at,notnull,default:now()" json:"deleted_at"`
}

// EnrollmentDeletionCounts is the immutable deletion evidence stored with an
// enrollment tombstone. It deliberately lives in Audit so the ledger does not
// depend on the enrollment write model.
type EnrollmentDeletionCounts struct {
	Requests                  int `json:"requests"`
	RequestChildren           int `json:"request_children"`
	RequestChildOfferings     int `json:"request_child_offerings"`
	RequestGuardians          int `json:"request_guardians"`
	ChangeRequests            int `json:"change_requests"`
	ChangeRequestMessages     int `json:"change_request_messages"`
	LateInvites               int `json:"late_invites"`
	OfferingAdjustments       int `json:"offering_adjustments"`
	EmailOutbox               int `json:"email_outbox"`
	RolloverLinksCleared      int `json:"rollover_links_cleared"`
	StudentSourceLinksCleared int `json:"student_source_links_cleared"`
}

type EnrollmentDeletionRepository interface {
	Create(ctx context.Context, event *EnrollmentDeletion) error
}
