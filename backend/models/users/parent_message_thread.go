package users

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const tableUsersParentMessageThreads = "users.parent_message_threads"

// Sender kinds shared by threads and messages. A message originates either
// from a guardian (parents portal) or from staff (tenant portal).
const (
	ParentMessageSenderGuardian = "guardian"
	ParentMessageSenderStaff    = "staff"
)

// ParentMessageThread is one conversation between the OGS team and a SINGLE
// guardian about one child — like an email thread, identified by a subject.
// A guardian can have several threads about the same child. LastSenderKind
// lets the inbox flag "awaiting OGS reply" without scanning the messages.
type ParentMessageThread struct {
	base.Model `bun:"schema:users,table:parent_message_threads"`
	base.TenantModel
	StudentID         int64      `bun:"student_id,notnull" json:"student_id"`
	GuardianAccountID int64      `bun:"guardian_account_id,notnull" json:"guardian_account_id"`
	Subject           string     `bun:"subject,notnull" json:"subject"`
	LastMessageAt     *time.Time `bun:"last_message_at" json:"last_message_at,omitempty"`
	LastSenderKind    *string    `bun:"last_sender_kind" json:"last_sender_kind,omitempty"`
}

func (t *ParentMessageThread) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(tableUsersParentMessageThreads)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(tableUsersParentMessageThreads)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(tableUsersParentMessageThreads)
	}
	return nil
}

func (t *ParentMessageThread) GetID() any              { return t.ID }
func (t *ParentMessageThread) GetCreatedAt() time.Time { return t.CreatedAt }
func (t *ParentMessageThread) GetUpdatedAt() time.Time { return t.UpdatedAt }
func (t *ParentMessageThread) TableName() string       { return tableUsersParentMessageThreads }

// ParentMessageThreadRepository is the tenant-scoped data-access contract for
// message threads. All methods must run inside a tenant transaction.
type ParentMessageThreadRepository interface {
	Create(ctx context.Context, thread *ParentMessageThread) error
	Update(ctx context.Context, thread *ParentMessageThread) error
	// FindByID returns the thread by id (tenant-scoped), or nil when absent.
	FindByID(ctx context.Context, id int64) (*ParentMessageThread, error)
	// ListGuardiansForStudent returns the child's guardians that have a portal
	// account (i.e. can receive messages), primary first. Powers the staff
	// "new conversation" recipient picker.
	ListGuardiansForStudent(ctx context.Context, studentID int64) ([]*MessageableGuardian, error)
}

// MessageableGuardian is one guardian of a child who has a parent-portal
// account and can therefore be the recipient of a thread.
type MessageableGuardian struct {
	AccountID        int64  `bun:"account_id" json:"account_id"`
	Name             string `bun:"name" json:"name"`
	RelationshipType string `bun:"relationship_type" json:"relationship_type"`
	IsPrimary        bool   `bun:"is_primary" json:"is_primary"`
}
