package suggestions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommentRead_TableName(t *testing.T) {
	cr := &CommentRead{}
	assert.Equal(t, "suggestions.comment_reads", cr.TableName())
}

func TestCommentRead_StructFields(t *testing.T) {
	// Verify struct can be instantiated with all fields
	cr := &CommentRead{
		AccountID: 123,
		PostID:    456,
	}

	assert.Equal(t, int64(123), cr.AccountID)
	assert.Equal(t, int64(456), cr.PostID)
}
