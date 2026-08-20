package audit

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEnrollmentOfferingAdjustmentAccessors(t *testing.T) {
	t.Parallel()

	changedAt := time.Date(2026, time.June, 20, 10, 30, 0, 0, time.UTC)
	entry := &EnrollmentOfferingAdjustment{
		ID:        42,
		Before:    json.RawMessage(`[]`),
		After:     json.RawMessage(`[{"offering":"Randstunde"}]`),
		ChangedAt: changedAt,
	}

	assert.Equal(t, int64(42), entry.GetID())
	assert.Equal(t, changedAt, entry.GetCreatedAt())
	assert.Equal(t, changedAt, entry.GetUpdatedAt())
}
