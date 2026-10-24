package users

import (
	"context"
)

// ParentRequestShareEvent freezes one guardian's recipient choice for one
// request. The newest event is effective; older choices remain auditable.
type ParentRequestShareEvent struct {
	Model `bun:"schema:users,table:parent_request_share_events"`
	TenantModel
	StudentID           int64   `bun:"student_id,notnull" json:"student_id"`
	RequestType         string  `bun:"request_type,notnull" json:"request_type"`
	RequestID           int64   `bun:"request_id,notnull" json:"request_id"`
	AuthorAccountID     int64   `bun:"author_account_id,notnull" json:"author_account_id"`
	RecipientAccountIDs []int64 `bun:"recipient_account_ids,array,notnull" json:"recipient_account_ids"`
}

type ParentRequestShareEventRepository interface {
	Create(context.Context, *ParentRequestShareEvent) error
	CurrentForStudent(context.Context, int64) ([]*ParentRequestShareEvent, error)
}
