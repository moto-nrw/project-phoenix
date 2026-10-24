package api

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFixedSeederSeedGroupHandover(t *testing.T) {
	t.Parallel()

	var method, path string
	var decodeErr error
	var request struct {
		Type          string `json:"type"`
		GroupHandover struct {
			GroupID       int64  `json:"group_id"`
			TargetStaffID int64  `json:"target_staff_id"`
			StartDate     string `json:"start_date"`
			EndDate       string `json:"end_date"`
		} `json:"group_handover"`
	}
	server := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		method, path = r.Method, r.URL.Path
		decodeErr = json.NewDecoder(r.Body).Decode(&request)
		w.WriteHeader(seedHTTPStatusCreated)
	})
	defer server.Close()

	seeder := NewFixedSeeder(newTestClient(server.URL, false), false, "")
	seeder.groupIDs["sternengruppe"] = 12
	seeder.staffIDs["Birgit Braun"] = 34
	require.NoError(t, seeder.seedGroupHandover(context.Background()))

	require.NoError(t, decodeErr)
	require.Equal(t, seedHTTPMethodPost, method)
	require.Equal(t, "/api/substitutions", path)
	require.Equal(t, "group_handover", request.Type)
	require.Equal(t, int64(12), request.GroupHandover.GroupID)
	require.Equal(t, int64(34), request.GroupHandover.TargetStaffID)
	start, err := parseSeedDate(request.GroupHandover.StartDate)
	require.NoError(t, err)
	end, err := parseSeedDate(request.GroupHandover.EndDate)
	require.NoError(t, err)
	require.Equal(t, start.AddDays(2), end)
}

func TestFixedSeederSeedGroupHandoverRequiresSeedReferences(t *testing.T) {
	t.Parallel()

	seeder := NewFixedSeeder(nil, false, "")
	require.ErrorContains(t, seeder.seedGroupHandover(context.Background()), "group not found")

	seeder.groupIDs["sternengruppe"] = 12
	require.ErrorContains(t, seeder.seedGroupHandover(context.Background()), "staff not found")
}
