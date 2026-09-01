package api

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedAttendanceCorrectionUsesCompletedInstanceFlow(t *testing.T) {
	t.Parallel()

	var paths []string
	var correction map[string]any
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/timetable/instances/71/students/21/correction" {
			require.NoError(t, json.NewDecoder(r.Body).Decode(&correction))
		}
		if r.URL.Path == "/api/timetable/instances" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"71"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	})
	defer srv.Close()

	fs := NewFixedSeeder(newTestClient(srv.URL, false), false, "")
	fs.studentIDByIndex = map[int]int64{0: 21}
	fs.staffIDs = map[string]int64{"Anna Müller": 26}
	fs.roomIDs = map[string]int64{"OGS-Raum 1": 24}

	err := seedAttendanceCorrection(&Runtime{FixedSeeder: fs, Client: fs.client})
	require.NoError(t, err)
	assert.Equal(t, []string{
		"/api/timetable/instances",
		"/api/timetable/instances/71/start",
		"/api/timetable/instances/71/complete",
		"/api/timetable/instances/71/students/21/correction",
	}, paths)
	assert.Equal(t, "Nachträglich ergänzt.", correction["note"])
	assert.Equal(t, "Dokumentation vervollständigt", correction["reason"])
}
