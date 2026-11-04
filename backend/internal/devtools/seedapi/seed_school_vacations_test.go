package api

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSeedSchoolVacationsStepUsesTheLocalSeedRoute(t *testing.T) {
	t.Parallel()

	var seedHeader string
	var body struct {
		Vacations []map[string]string `json:"vacations"`
	}
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/timetable/school-vacations/seed", r.URL.Path)
		seedHeader = r.Header.Get(seedTokenHeader)
		assert.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"stored":64}}`))
	})
	defer srv.Close()

	client := newTestClient(srv.URL, false)
	runtime := &Runtime{Client: client, TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"}}

	require.NoError(t, (seedSchoolVacationsStep{}).Run(context.Background(), runtime))
	assert.Equal(t, "true", seedHeader)
	assert.Len(t, body.Vacations, 4)
}

func TestSeedSchoolVacationWindowsStartNextWeek(t *testing.T) {
	t.Parallel()
	// Wednesday: the first vacation runs from next Monday to Friday.
	today := seedDate{Time: time.Date(2026, time.October, 28, 0, 0, 0, 0, time.UTC)}

	windows := seedSchoolVacationWindows(today)

	require.Len(t, windows, 4)
	assert.Equal(t, map[string]string{"name": "Herbstferien", "start_date": "2026-11-02", "end_date": "2026-11-06"}, windows[0])
	assert.Equal(t, map[string]string{"name": "Weihnachtsferien", "start_date": "2026-12-23", "end_date": "2027-01-06"}, windows[1],
		"the Christmas vacation crosses the year")
	assert.Equal(t, "2027-07-12", windows[3]["start_date"])

	december := seedSchoolVacationWindows(seedDate{Time: time.Date(2026, time.December, 13, 0, 0, 0, 0, time.UTC)})
	assert.Equal(t, "2026-12-23", december[1]["start_date"], "a first vacation before Christmas keeps this Christmas")
	late := seedSchoolVacationWindows(seedDate{Time: time.Date(2026, time.December, 20, 0, 0, 0, 0, time.UTC)})
	assert.Equal(t, "2027-12-23", late[1]["start_date"], "a first vacation after Christmas moves the year on")
}
