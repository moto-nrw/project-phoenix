package enrollment

import (
	"time"
)

type ChangeRequestMessage struct {
	ID              int64     `json:"id"`
	TenantID        int64     `json:"tenant_id"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	ChangeRequestID int64     `json:"change_request_id"`
	AuthorType      string    `json:"author_type"`
	AuthorAccountID *int64    `json:"author_account_id,omitempty"`
	Body            string    `json:"body"`
	InternalOnly    bool      `json:"internal_only"`
}
