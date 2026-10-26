package owners

import (
	"context"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

type planExportFakeGroupNames map[int64]string

func (n planExportFakeGroupNames) EducationGroupNames(_ context.Context, ids []int64) (map[int64]string, error) {
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		if name, ok := n[id]; ok {
			out[id] = name
		}
	}
	return out, nil
}

// #4766, #4768: the root binds the export's filter to the planner's display
// projection, so a group, person or room filter prints the same blocks the
// Betreuungsplan shows.
func TestPlanExportFiltersBetreuungsplanByGroupAndPerson(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	room := testpkg.CreateTestRoom(t, db, "PlanFilter")
	groupBlock := testpkg.CreateTestActivityInstance(t, db, planExportMonday, room.ID, testpkg.ActivityInstanceOpts{Title: "Gruppenblock", StartHHMM: "12:00", EndHHMM: "13:00"})
	otherRoom := testpkg.CreateTestRoom(t, db, "PlanFilterRaum")
	staffBlock := testpkg.CreateTestActivityInstance(t, db, planExportMonday, otherRoom.ID, testpkg.ActivityInstanceOpts{Title: "Personalblock", StartHHMM: "14:00", EndHHMM: "15:00"})
	group := testpkg.CreateTestEducationGroup(t, db, "Sonnen")
	child := testpkg.CreateTestStudent(t, db, "Mia", "Filter", "2a")
	testpkg.AssignStudentToGroup(t, db, child.ID, group.ID)
	testpkg.CreateTestInstanceStudent(t, db, groupBlock.ID, child.ID, "")
	staff := testpkg.CreateTestStaff(t, db, "Sina", "Vertretung")
	testpkg.CreateTestInstanceStaff(t, db, staffBlock.ID, staff.ID, testpkg.InstanceStaffOpts{IsSubstitute: true})

	// export renders the sheet and returns the printed block titles, then
	// the sheet's filter line. A title carries the style prefix Document
	// Rendering owns, so it is matched by its ending, like assertPlanCell.
	export := func(filter planexport.BlockFilter) (titles, filters []string) {
		t.Helper()
		renderer := &planExportCaptureRenderer{}
		params, err := planexport.ParseParams(planExportMonday.String(), planExportMonday.String(), string(planexport.TemplateByOffering), "", "")
		require.NoError(t, err)
		params.Filter = filter
		require.NoError(t, testpkg.WithTenantTx(t, ctx, db, testpkg.Tenant(t), func(txCtx context.Context, tx bun.Tx) error {
			sources := planExportBetreuungsplanSources(t, db, tx, planExportFixtureStaff(&planexport.StaffMember{ID: staff.ID, FirstName: "Sina", LastName: "Vertretung"}), renderer)
			sources.BlockGroups = MustNewUnobservedTimetable(testpkg.MustDatabase(db))
			sources.EducationGroupNames = planExportFakeGroupNames{group.ID: "Sonnen"}
			_, err := newPlanExport(sources).ExportBetreuungsplan(txCtx, params)
			return err
		}))
		for _, row := range renderer.doc.Rows {
			for _, title := range []string{"Gruppenblock", "Personalblock"} {
				if strings.HasSuffix(row.Values[lists.ColumnPlanRowLabel], title) {
					titles = append(titles, title)
				}
			}
		}
		return titles, renderer.doc.Filters
	}

	titles, _ := export(planexport.BlockFilter{})
	require.ElementsMatch(t, []string{"Gruppenblock", "Personalblock"}, titles)
	titles, filters := export(planexport.BlockFilter{EducationGroupID: &group.ID})
	require.Equal(t, []string{"Gruppenblock"}, titles)
	require.Contains(t, filters, "Gruppe: Sonnen")
	titles, filters = export(planexport.BlockFilter{StaffID: &staff.ID})
	require.Equal(t, []string{"Personalblock"}, titles)
	require.Contains(t, filters, "Person: Vertretung, S.")
	titles, filters = export(planexport.BlockFilter{RoomID: &otherRoom.ID})
	require.Equal(t, []string{"Personalblock"}, titles)
	require.Contains(t, filters, "Raum: "+otherRoom.Name)
}
