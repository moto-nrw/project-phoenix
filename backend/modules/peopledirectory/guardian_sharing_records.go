package peopledirectory

import (
	"context"
	"time"
)

// Complete native values retain source metadata, nullable fields and JSON presence.
type GuardianFamilyProtectionRecord struct {
	ID             int64     `json:"id"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	TenantID       int64     `json:"tenant_id"`
	StudentID      int64     `json:"student_id"`
	Enabled        bool      `json:"enabled"`
	Reason         string    `json:"reason"`
	ActorAccountID int64     `json:"actor_account_id"`
}
type ParentRequestShareRecord struct {
	ID                  int64     `json:"id"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
	TenantID            int64     `json:"tenant_id"`
	StudentID           int64     `json:"student_id"`
	RequestType         string    `json:"request_type"`
	RequestID           int64     `json:"request_id"`
	AuthorAccountID     int64     `json:"author_account_id"`
	RecipientAccountIDs []int64   `json:"recipient_account_ids"`
}

type GuardianFamilyProtectionReads interface {
	GuardianFamilyProtectionRecords(context.Context, []int64) (map[int64]*GuardianFamilyProtectionRecord, error)
}
type ParentRequestShares interface {
	AppendParentRequestShare(context.Context, *ParentRequestShareRecord) error
	ParentRequestSharesForStudent(context.Context, int64) ([]*ParentRequestShareRecord, error)
}
