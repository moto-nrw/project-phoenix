package active_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/active"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
)

func TestStudentStatusDayModelAccessors(t *testing.T) {
	now := time.Now()
	entry := &active.StudentStatusDay{
		Model: modelBase.Model{
			ID:        42,
			CreatedAt: now,
			UpdatedAt: now.Add(time.Minute),
		},
	}

	assert.Equal(t, int64(42), entry.GetID())
	assert.Equal(t, now, entry.GetCreatedAt())
	assert.Equal(t, now.Add(time.Minute), entry.GetUpdatedAt())
}
