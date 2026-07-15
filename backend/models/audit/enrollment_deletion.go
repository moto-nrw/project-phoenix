package audit

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/enrollment"
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
	base.TenantModel
	RequestID      int64                     `bun:"request_id,notnull" json:"request_id"`
	ChildID        *int64                    `bun:"child_id" json:"child_id,omitempty"`
	ActorAccountID *int64                    `bun:"actor_account_id" json:"actor_account_id,omitempty"`
	ActorType      string                    `bun:"actor_type,notnull" json:"actor_type"`
	Scope          string                    `bun:"scope,notnull" json:"scope"`
	Reason         string                    `bun:"reason,notnull" json:"reason"`
	Counts         enrollment.DeletionCounts `bun:"counts,type:jsonb,notnull" json:"counts"`
	DeletedAt      time.Time                 `bun:"deleted_at,notnull,default:now()" json:"deleted_at"`
}

type EnrollmentDeletionRepository interface {
	Create(ctx context.Context, event *EnrollmentDeletion) error
}
