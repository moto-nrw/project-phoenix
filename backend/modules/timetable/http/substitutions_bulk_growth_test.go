package timetablehttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-ogs/project-phoenix/test"
)

// bulkGrowthDay is one selected date of the growth fixture: a block A runs
// alone, a block A shares with B, and (with a substitute) a block that was
// acknowledged as understaffed.
type bulkGrowthDay struct {
	date   calendar.Date
	alone  int64
	shared int64
	acked  int64
}

// TestBulkSubstitutionPlanningGrowth saves N3 and N8 selected dates with and
// without a substitute and checks the complete written state, the response
// order and the cleared acknowledgements (#5808). The planning reads run once
// for all selected dates under the held day locks; only the per-day writes
// grow with the days.
func TestBulkSubstitutionPlanningGrowth(t *testing.T) {
	t.Parallel()
	testpkg.SetupIsolatedTestDB(t)
	s := buildDevModule(t)
	router := bulkRouter(s.ctx, s.res, s.db)
	counter := testpkg.CaptureQueries(t, s.db)

	offset := 1
	newDays := func(n int, withAck bool) ([]bulkGrowthDay, []string) {
		days := make([]bulkGrowthDay, 0, n)
		// A selected date without any assignment classifies to an empty day;
		// it is the earliest date of the save.
		empty, _ := futureSubDate(offset)
		offset++
		dates := []string{empty}
		for i := range n {
			text, date := futureSubDate(offset)
			offset++
			day := bulkGrowthDay{date: date}
			day.alone = testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: fmt.Sprintf("Allein %d", i), StartHHMM: "08:00", EndHHMM: "09:00"}).ID
			day.shared = testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: fmt.Sprintf("Geteilt %d", i), StartHHMM: "10:00", EndHHMM: "11:00"}).ID
			testpkg.CreateTestInstanceStaff(t, s.db, day.alone, s.staffA, testpkg.InstanceStaffOpts{})
			testpkg.CreateTestInstanceStaff(t, s.db, day.shared, s.staffB, testpkg.InstanceStaffOpts{})
			testpkg.CreateTestInstanceStaff(t, s.db, day.shared, s.staffA, testpkg.InstanceStaffOpts{})
			if withAck {
				day.acked = testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{Title: fmt.Sprintf("Quittiert %d", i), StartHHMM: "12:00", EndHHMM: "13:00"}).ID
				testpkg.CreateTestInstanceStaff(t, s.db, day.acked, s.staffA, testpkg.InstanceStaffOpts{})
				setUnderstaffedAckFixture(t, s.db, s.ctx, day.acked)
			}
			days = append(days, day)
			dates = append(dates, text)
		}
		return days, dates
	}

	for _, withSubstitute := range []bool{false, true} {
		statements := map[int]int{}
		for _, n := range []int{3, 8} {
			days, dates := newDays(n, withSubstitute)
			body := map[string]any{"absent_staff_id": s.staffA, "dates": dates, "reason": "krank"}
			if withSubstitute {
				body["substitute_staff_id"] = s.staffY
			}
			counter.Reset()
			w := doBulk(t, router, body)
			require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
			statements[n] = counter.Total()

			var resp struct {
				Data BulkSubstitutionResponse `json:"data"`
			}
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Len(t, resp.Data.Days, n+1)
			assert.Empty(t, resp.Data.Days[0].AffectedInstances, "the empty date stays a no-op")
			perDay := 2
			if withSubstitute {
				perDay = 3
			}
			assert.Equal(t, perDay*n, resp.Data.TotalAffected)
			for i, day := range days {
				got := resp.Data.Days[i+1]
				assert.Equal(t, day.date.String(), got.Date)
				ids := make([]int64, 0, len(got.AffectedInstances))
				for _, affected := range got.AffectedInstances {
					ids = append(ids, affected.InstanceID)
				}
				want := []int64{day.alone, day.shared}
				if withSubstitute {
					want = append(want, day.acked)
				}
				assert.Equal(t, want, ids, "affected blocks keep their time order on %s", day.date)
				t.Logf("day %s actions: %s", day.date, affectedActions(got.AffectedInstances))
				assertBulkGrowthDay(t, s, day, withSubstitute)
			}
		}
		t.Logf("substitute=%t statements: N3=%d N8=%d", withSubstitute, statements[3], statements[8])
	}
}

func affectedActions(values []AffectedInstance) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, value.Action)
	}
	return strings.Join(parts, ",")
}

func assertBulkGrowthDay(t *testing.T, s *devSetup, day bulkGrowthDay, withSubstitute bool) {
	t.Helper()
	blocks := []int64{day.alone, day.shared}
	if withSubstitute {
		blocks = append(blocks, day.acked)
	}
	for _, instanceID := range blocks {
		var absentA, presentB bool
		substitutes := 0
		for _, row := range devInstanceStaff(t, s.db, s.ctx, instanceID) {
			switch {
			case row.StaffID == s.staffA && !row.IsSubstitute:
				absentA = row.IsAbsent
				if row.IsAbsent {
					assert.NotNil(t, row.AbsenceReason, "absence keeps its reason")
				}
			case row.StaffID == s.staffB:
				presentB = !row.IsAbsent
			case row.StaffID == s.staffY && row.IsSubstitute:
				assert.False(t, row.IsAbsent)
				substitutes++
			default:
				t.Errorf("unexpected staff row %+v on instance %d", row, instanceID)
			}
		}
		assert.True(t, absentA, "A absent on instance %d", instanceID)
		if instanceID == day.shared {
			assert.True(t, presentB, "B stays on the shared block")
		}
		if withSubstitute {
			assert.Equal(t, 1, substitutes, "exactly one cover row on instance %d", instanceID)
		} else {
			assert.Zero(t, substitutes, "absence-only creates no cover on instance %d", instanceID)
		}
	}
	if withSubstitute {
		instance := readActivityInstanceAck(t, s, day.acked)
		assert.False(t, instance, "the covered block loses its stale understaffed acknowledgement")
	}
}

func readActivityInstanceAck(t *testing.T, s *devSetup, instanceID int64) bool {
	t.Helper()
	var ack bool
	require.NoError(t, s.db.NewSelect().TableExpr(`schedule.activity_instances`).Column("understaffed_ack").
		Where("id = ?", instanceID).Scan(s.ctx, &ack))
	return ack
}
