package timetracking

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpeningBalanceRequest_RequiresBalanceMinutes(t *testing.T) {
	t.Parallel()

	var request openingBalanceRequest
	require.NoError(t, json.Unmarshal([]byte(`{"effective_date":"2026-03-01"}`), &request))
	assert.Nil(t, request.BalanceMinutes)

	require.NoError(t, json.Unmarshal([]byte(`{"effective_date":"2026-03-01","balance_minutes":0}`), &request))
	require.NotNil(t, request.BalanceMinutes)
	assert.Zero(t, *request.BalanceMinutes)
}

func TestVacationOpeningRequest_RequiresRemainingDays(t *testing.T) {
	t.Parallel()

	var request setVacationOpeningRequest
	require.NoError(t, json.Unmarshal([]byte(`{"effective_date":"2026-03-01"}`), &request))
	assert.Nil(t, request.RemainingDays)

	require.NoError(t, json.Unmarshal([]byte(`{"effective_date":"2026-03-01","remaining_days":0}`), &request))
	require.NotNil(t, request.RemainingDays)
	assert.Zero(t, *request.RemainingDays)
}
