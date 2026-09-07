package usercontext

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroupVisitResponsePreservesHTTPFields(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	encoded, err := json.Marshal(groupVisitResponses([]studentpresence.Visit{{
		ID: 11, TenantID: 22, StudentID: 33, ActiveGroupID: 44,
		CreatedAt: at, UpdatedAt: at.Add(time.Minute), EntryTime: at.Add(-time.Hour),
	}}))
	require.NoError(t, err)
	assert.JSONEq(t, `[{
		"id":11,"tenant_id":22,"student_id":33,"active_group_id":44,
		"created_at":"2026-09-07T12:00:00Z","updated_at":"2026-09-07T12:01:00Z",
		"entry_time":"2026-09-07T11:00:00Z"
	}]`, string(encoded))
}

func TestGroupVisitResponsePreservesEmptyResult(t *testing.T) {
	t.Parallel()
	for _, visits := range [][]studentpresence.Visit{nil, {}} {
		encoded, err := json.Marshal(groupVisitResponses(visits))
		require.NoError(t, err)
		assert.Equal(t, "null", string(encoded))
	}
}
