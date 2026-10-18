package studentpresencehttp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func partialAbsenceResponseID(t *testing.T, body []byte) int64 {
	t.Helper()
	var response struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(body, &response))
	require.Positive(t, response.Data.ID)
	return response.Data.ID
}
