package active

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStudentStatusDayModelAccessors(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entry := &StudentStatusDay{}
	entry.ID = 42
	entry.CreatedAt = now
	entry.UpdatedAt = now.Add(time.Minute)

	assert.Equal(t, int64(42), entry.GetID())
	assert.Equal(t, now, entry.GetCreatedAt())
	assert.Equal(t, now.Add(time.Minute), entry.GetUpdatedAt())
}
