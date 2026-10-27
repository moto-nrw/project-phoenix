package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The public demo opens the Anwesenheitsprotokoll and the Feedbackhistorie
// of a child (#5029); both are off by default for real schools.
func TestFullOperationProfileEnablesHistories(t *testing.T) {
	t.Parallel()
	settings := fullOperationProfileDefinition().Settings
	for _, key := range []string{profileSettingAttendanceLog, profileSettingFeedbackEnabled} {
		setting, ok := settings[key]
		require.True(t, ok, key)
		assert.JSONEq(t, `true`, string(setting.Value), key)
		assert.Equal(t, SettingManagedByTenant, setting.ManagedBy, key)
	}
}

// The seeded feedback (#5029) rates only days a child came, inside the
// window, with the day's rating at check-out and lunch after arrival.
func TestFeedbackHistoryPlanRatesPresentDays(t *testing.T) {
	t.Parallel()
	input, _ := presenceHistoryFixture()
	days := planPresenceHistory(input).days
	from := input.today.AddDays(-feedbackHistoryDays)
	entries := planFeedbackHistory(days, from)

	present := map[string]PastPresenceDay{}
	for _, day := range days {
		present[fmt.Sprintf("%s|%d", day.Date, day.StudentID)] = day
	}
	values := map[string]int{}
	mensa, students := 0, map[int64]bool{}
	for _, entry := range entries {
		day, ok := present[fmt.Sprintf("%s|%d", entry.Day, entry.StudentID)]
		require.True(t, ok, "feedback only on a present day: %+v", entry)
		require.False(t, day.Date.Before(from), "inside the window")
		values[entry.Value]++
		students[entry.StudentID] = true
		if entry.IsMensaFeedback {
			mensa++
			assert.Less(t, entry.Time, day.CheckOut.In(seedBerlinLocation()).Format("15:04:05"))
		} else {
			assert.Equal(t, day.CheckOut.In(seedBerlinLocation()).Format("15:04:05"), entry.Time)
		}
	}
	assert.Greater(t, mensa, 0, "lunch ratings fill the Mensa badge")
	assert.Greater(t, len(entries)-mensa, mensa, "more day ratings than lunch ratings")
	assert.Greater(t, len(students), 50, "most children have a history")
	assert.Greater(t, values["positive"], values["neutral"])
	assert.Greater(t, values["neutral"], values["negative"])
	assert.Positive(t, values["negative"])
	assert.Equal(t, entries, planFeedbackHistory(days, from), "the same day seeds the same feedback")
}

// The step writes the history as the school admin in one batch, inside the
// window it read, and leaves a school alone that already has one (#5029).
func TestSeedFeedbackHistoryStepWritesOneBatchOnce(t *testing.T) {
	t.Parallel()
	for _, existing := range []bool{false, true} {
		var start, end, batchAuth string
		var batch struct {
			Entries []feedbackHistoryEntry `json:"entries"`
		}
		srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
			w.Header().Set("Content-Type", "application/json")
			switch {
			case r.Method == http.MethodGet && (r.URL.Path == "/api/time-tracking/holidays" ||
				r.URL.Path == "/api/timetable/closing-days/" || r.URL.Path == "/api/timetable/periods/"):
				_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
			case r.Method == http.MethodGet && r.URL.Path == "/api/students":
				_, _ = w.Write([]byte(`{"status":"success","data":[` + feedbackHistoryStudentRows(20) + `],"pagination":{"total_pages":1}}`))
			case r.Method == http.MethodGet && r.URL.Path == "/api/feedback/date-range":
				start, end = r.URL.Query().Get("start_date"), r.URL.Query().Get("end_date")
				if existing {
					_, _ = w.Write([]byte(`{"status":"success","data":[{"id":1}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
			case r.Method == http.MethodPost && r.URL.Path == "/api/feedback/batch":
				batchAuth = r.Header.Get("Authorization")
				require.NoError(t, json.NewDecoder(r.Body).Decode(&batch))
				w.WriteHeader(http.StatusCreated)
				_, _ = fmt.Fprintf(w, `{"status":"success","data":{"count":%d}}`, len(batch.Entries))
			default:
				t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			}
		})
		runtime := &Runtime{
			Client:     newTestClient(srv.URL, false),
			TenantAuth: AuthRef{Kind: AuthBearer, Token: "tenant-token"},
		}
		step := seedFeedbackHistoryStep{seeder: &Seeder{definition: fullOperationProfileDefinition()}}

		require.NoError(t, step.Run(context.Background(), runtime))
		srv.Close()
		if existing {
			assert.Empty(t, batch.Entries, "a present history is not written twice")
			continue
		}
		assert.Equal(t, "Bearer tenant-token", batchAuth)
		require.NotEmpty(t, batch.Entries)
		mensa := 0
		for _, entry := range batch.Entries {
			assert.GreaterOrEqual(t, entry.Day, start)
			assert.LessOrEqual(t, entry.Day, end)
			if entry.IsMensaFeedback {
				mensa++
			}
		}
		assert.Positive(t, mensa, "the history holds Mensa feedback")
	}
}

// A profile that leaves feedback off sends no request: the API would refuse
// the entries.
func TestSeedFeedbackHistoryStepSkipsSchoolsWithoutFeedback(t *testing.T) {
	t.Parallel()
	srv := newSeedHTTPTestServer(func(_ seedHTTPResponseWriter, r *seedHTTPRequest) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
	})
	defer srv.Close()
	step := seedFeedbackHistoryStep{seeder: &Seeder{definition: manualProfileDefinition()}}
	require.NoError(t, step.Run(context.Background(), &Runtime{Client: newTestClient(srv.URL, false)}))
}

func feedbackHistoryStudentRows(count int) string {
	rows := make([]string, 0, count)
	for i := range count {
		rows = append(rows, fmt.Sprintf(`{"id":%d}`, 100+i))
	}
	return strings.Join(rows, ",")
}
