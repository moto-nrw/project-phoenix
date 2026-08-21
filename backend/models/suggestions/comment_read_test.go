package suggestions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommentRead_StructFields(t *testing.T) {
	t.Parallel()

	// Verify struct can be instantiated with all fields
	cr := &CommentRead{
		AccountID: 123,
		PostID:    456,
	}

	assert.Equal(t, int64(123), cr.AccountID)
	assert.Equal(t, int64(456), cr.PostID)
}
