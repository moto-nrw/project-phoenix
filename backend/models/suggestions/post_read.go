package suggestions

import (
	"time"

	"github.com/uptrace/bun"
)

// PostRead tracks when an operator viewed a post
type PostRead struct {
	bun.BaseModel `bun:"table:suggestions.post_reads"`

	AccountID  int64     `bun:"account_id,pk"`
	PostID     int64     `bun:"post_id,pk"`
	ReaderType string    `bun:"reader_type,pk"`
	ViewedAt   time.Time `bun:"viewed_at,default:now()"`
}
