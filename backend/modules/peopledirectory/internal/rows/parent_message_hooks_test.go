package users

// Tests for the Model accessors surfaced on the parent-message models:
// the embedded id/timestamp getters must return the embedded fields. No DB
// connection is needed.

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBaseModel_Accessors(t *testing.T) {
	t.Parallel()

	now := time.Now()
	m := Model{ID: 42, CreatedAt: now, UpdatedAt: now.Add(time.Hour)}
	assert.Equal(t, int64(42), m.GetID())
	assert.Equal(t, now, m.GetCreatedAt())
	assert.Equal(t, now.Add(time.Hour), m.GetUpdatedAt())
}
