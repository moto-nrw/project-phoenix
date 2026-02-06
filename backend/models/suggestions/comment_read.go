package suggestions

import (
	"time"

	"github.com/uptrace/bun"
)

// tableCommentReads is the schema-qualified table name
const tableCommentReads = "suggestions.comment_reads"

// CommentRead tracks when a user last read comments on a post
type CommentRead struct {
	bun.BaseModel `bun:"table:suggestions.comment_reads,alias:cr"`
	AccountID     int64     `bun:"account_id,pk"`
	PostID        int64     `bun:"post_id,pk"`
	LastReadAt    time.Time `bun:"last_read_at,notnull"`
}

// TableName returns the database table name
func (cr *CommentRead) TableName() string {
	return tableCommentReads
}
