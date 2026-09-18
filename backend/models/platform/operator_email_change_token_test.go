package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOperatorEmailChangeToken_Accessors(t *testing.T) {
	t.Parallel()

	now := time.Now()
	token := &OperatorEmailChangeToken{}
	token.ID = 42
	token.CreatedAt = now
	token.UpdatedAt = now.Add(time.Hour)

	assert.Equal(t, int64(42), token.GetID())
	assert.Equal(t, now, token.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), token.GetUpdatedAt())
}
