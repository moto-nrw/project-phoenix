package timetablehttp

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #4766: GET /instances narrows the window to one OGS group and/or one
// staff member; without a filter it still returns the whole plan.
func TestListInstances_FiltersByGroupAndStaff(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()
	rows, err := RootCompositions.Rows(s.db)
	require.NoError(t, err)
	s.res.BlockGroups = rows.Timetable
	router := listRouter(s.ctx, s.res)

	from, fromDate := listFutureDate(1)
	to, _ := listFutureDate(7)
	suffix := time.Now().UnixNano()

	staffBlock := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "12:00", EndHHMM: "13:00", Title: "Vertretung",
	})
	substitute := testpkg.CreateTestStaff(t, s.db, "Sina", fmt.Sprintf("Vertretung-%d", suffix))
	testpkg.CreateTestInstanceStaff(t, s.db, staffBlock.ID, substitute.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})

	groupBlock := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "14:00", EndHHMM: "15:00", Title: "Gruppenblock",
	})
	group := testpkg.CreateTestEducationGroup(t, s.db, fmt.Sprintf("Sonnen-%d", suffix))
	child := testpkg.CreateTestStudent(t, s.db, "Mia", fmt.Sprintf("Gruppe-%d", suffix), "2a")
	testpkg.AssignStudentToGroup(t, s.db, child.ID, group.ID)
	testpkg.CreateTestInstanceStudent(t, s.db, groupBlock.ID, child.ID, timetable.SlotAttendanceExpected)

	otherBlock := testpkg.CreateTestActivityInstance(t, s.db, fromDate, s.roomID, testpkg.ActivityInstanceOpts{
		StartHHMM: "16:00", EndHHMM: "17:00", Title: "Ohne Bezug",
	})

	ids := func(path string) []int64 {
		t.Helper()
		w := doList(t, router, path)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		got := decodeList(t, w)
		out := make([]int64, 0, len(got.Instances))
		for _, item := range got.Instances {
			out = append(out, item.ID)
		}
		return out
	}
	window := fmt.Sprintf("/instances?from=%s&to=%s", from, to)

	assert.ElementsMatch(t, []int64{staffBlock.ID, groupBlock.ID, otherBlock.ID}, ids(window))
	assert.Equal(t, []int64{staffBlock.ID}, ids(fmt.Sprintf("%s&staff_id=%d", window, substitute.ID)))
	assert.Equal(t, []int64{groupBlock.ID}, ids(fmt.Sprintf("%s&group_id=%d", window, group.ID)))
	assert.Equal(t, []int64{}, ids(fmt.Sprintf("%s&group_id=%d&staff_id=%d", window, group.ID, substitute.ID)))
}

func TestListInstances_RejectsInvalidFilterIDs(t *testing.T) {
	t.Parallel()

	s := buildListSetup(t)
	defer s.cleanupFn()
	router := listRouter(s.ctx, s.res)
	from, _ := listFutureDate(1)
	to, _ := listFutureDate(7)

	for _, query := range []string{"group_id=abc", "group_id=0", "staff_id=-3"} {
		t.Run(query, func(t *testing.T) {
			w := doList(t, router, fmt.Sprintf("/instances?from=%s&to=%s&%s", from, to, query))
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

// A block belongs to a group its template takes as participant source even
// before any child of the group is planned on it (#4766).
func TestListInstances_GroupFilterKeepsBlocksOfTheTemplateSourceGroup(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	rows, err := RootCompositions.Rows(s.db)
	require.NoError(t, err)
	s.res.BlockGroups = rows.Timetable
	period := createTemplateTestPeriod(t, s.db, "Tpl-Group-Filter-Period")
	roomID := s.roomID
	suffix := time.Now().UnixNano()
	ownGroup := testpkg.CreateTestEducationGroup(t, s.db, fmt.Sprintf("Quelle-%d", suffix))
	secondGroup := testpkg.CreateTestEducationGroup(t, s.db, fmt.Sprintf("Zweite-%d", suffix))
	otherGroup := testpkg.CreateTestEducationGroup(t, s.db, fmt.Sprintf("Andere-%d", suffix))
	repoFactory := mustTimetableTestRepositories(s.db)
	template, err := repoFactory.Timetable.CreateGroup(s.ctx, timetable.GroupInput{
		Name:             fmt.Sprintf("Tpl-Group-Filter-%d", suffix),
		MaxParticipants:  25,
		IsOpen:           true,
		IsTemplate:       true,
		CategoryID:       s.category.ID,
		CreatedBy:        &s.staffA,
		PlannedRoomID:    &roomID,
		Type:             timetable.GroupTypeCare,
		CalendarPeriodID: &period.ID,
		TargetGroupType:  timetable.TargetGroupTypeEducationGroup,
		EducationGroupID: &ownGroup.ID,
	})
	require.NoError(t, err)
	require.NoError(t, repoFactory.Timetable.ReplaceGroupTargets(s.ctx, template.ID, []timetable.GroupTargetInput{
		{TargetGroupType: timetable.TargetGroupTypeEducationGroup, EducationGroupID: &ownGroup.ID},
		{TargetGroupType: timetable.TargetGroupTypeEducationGroup, EducationGroupID: &secondGroup.ID},
	}))

	date := calendar.NewDate(2026, 8, 10)
	block := testpkg.CreateTestActivityInstance(t, s.db, date, s.roomID, testpkg.ActivityInstanceOpts{
		Title: template.Name, ActivityGroupID: &template.ID, CalendarPeriodID: &period.ID,
	})

	router := conversionRouter(s.ctx, s.res)
	ids := func(groupID int64) []int64 {
		t.Helper()
		w := doTemplateJSON(t, router, http.MethodGet,
			fmt.Sprintf("/instances?from=%s&to=%s&group_id=%d", date, date, groupID), nil)
		require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
		got := decodeTemplateData[weeklyInstancesResponse](t, w)
		out := make([]int64, 0, len(got.Instances))
		for _, item := range got.Instances {
			out = append(out, item.ID)
		}
		return out
	}
	assert.Equal(t, []int64{block.ID}, ids(ownGroup.ID), "the template's target group")
	assert.Equal(t, []int64{block.ID}, ids(secondGroup.ID), "a further participant source group")
	assert.Equal(t, []int64{}, ids(otherGroup.ID))
}
