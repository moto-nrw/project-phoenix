package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOperator_GetID(t *testing.T) {
	t.Parallel()

	o := &Operator{}
	o.ID = 456
	assert.Equal(t, int64(456), o.GetID())
}

func TestOperator_GetCreatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	o := &Operator{}
	o.CreatedAt = now
	assert.Equal(t, now, o.GetCreatedAt())
}

func TestOperator_GetUpdatedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	o := &Operator{}
	o.UpdatedAt = now
	assert.Equal(t, now, o.GetUpdatedAt())
}
