package suggestions

import (
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

// CommentRead tracks when a user last read comments on a post
type CommentRead struct {
	bun.BaseModel `bun:"table:suggestions.comment_reads,alias:cr"`
	base.TenantModel
	AccountID  int64     `bun:"account_id,pk"`
	PostID     int64     `bun:"post_id,pk"`
	ReaderType string    `bun:"reader_type,pk"`
	LastReadAt time.Time `bun:"last_read_at,notnull"`
}
