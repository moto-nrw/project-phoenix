package timetablehttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The slim view (#4996) drops only the per-child attendance rows: the
// Betreuungsplan grid renders counts and IDs, and the block detail loads its
// day in the full view when it opens.
func TestListInstances_SlimViewDropsOnlyTheAttendanceRows(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)
	inst := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "13:00", EndHHMM: "14:00", Title: "Lernzeit-Slim-Test",
	})
	suffix := time.Now().UnixNano()
	student1 := testpkg.CreateTestStudent(t, s.db, "Anna", fmt.Sprintf("Slim-%d-A", suffix), "1a")
	student2 := testpkg.CreateTestStudent(t, s.db, "Ben", fmt.Sprintf("Slim-%d-B", suffix), "1a")
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student1.ID, timetable.SlotAttendanceExpected)
	testpkg.CreateTestInstanceStudent(t, s.db, inst.ID, student2.ID, timetable.SlotAttendancePresent)

	router := listRouter(s.ctx, s.res)
	window := fmt.Sprintf("/instances?from=%s&to=%s", from, to)
	fullRR := doList(t, router, window)
	require.Equal(t, http.StatusOK, fullRR.Code, "body=%s", fullRR.Body.String())
	slimRR := doList(t, router, window+"&view=slim")
	require.Equal(t, http.StatusOK, slimRR.Code, "body=%s", slimRR.Body.String())

	full := decodeList(t, fullRR)
	slim := decodeList(t, slimRR)
	require.Len(t, full.Instances, 1)
	require.Len(t, slim.Instances, 1)
	require.Len(t, full.Instances[0].Students, 2, "the default view keeps the attendance rows")
	assert.Empty(t, slim.Instances[0].Students)

	// Everything but the rows is identical, the counts included.
	fullItem, slimItem := full.Instances[0], slim.Instances[0]
	fullItem.Students = nil
	assert.Equal(t, fullItem, slimItem)
	assert.ElementsMatch(t, []int64{student1.ID, student2.ID}, slimItem.StudentIDs)
	assert.Equal(t, 1, slimItem.ExpectedStudentsCount)
	assert.Equal(t, 1, slimItem.PresentStudentsCount)

	var raw struct {
		Data struct {
			Instances []map[string]json.RawMessage `json:"instances"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(slimRR.Body.Bytes(), &raw))
	assert.JSONEq(t, "null", string(raw.Data.Instances[0]["students"]))
	explicitFull := doList(t, router, window+"&view=full")
	require.Equal(t, http.StatusOK, explicitFull.Code, "body=%s", explicitFull.Body.String())
	assert.Len(t, decodeList(t, explicitFull).Instances[0].Students, 2)
}

func TestListInstances_RejectsAnUnknownView(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()
	from, _ := listFutureDate(1)
	to, _ := listFutureDate(7)

	w := doList(t, listRouter(s.ctx, s.res), fmt.Sprintf("/instances?from=%s&to=%s&view=compact", from, to))
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}
