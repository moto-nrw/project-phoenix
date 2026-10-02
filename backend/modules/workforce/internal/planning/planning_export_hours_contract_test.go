package planning_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	usersModel "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/planexport"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	planning "github.com/moto-nrw/project-phoenix/modules/workforce/internal/planning"
	"github.com/moto-nrw/project-phoenix/services/listexport"
	"github.com/stretchr/testify/require"
)

// hoursExport records what the hours sheet received from the facade.
type hoursExport struct {
	planexport.Service
	params planexport.Params
	hours  planexport.WeeklyHours
}

func (f *hoursExport) ExportDienstplanHours(ctx context.Context, params planexport.Params, reader planexport.WeeklyHoursReader) (listexport.File, error) {
	f.params = params
	hours, err := reader.WeeklyHours(ctx, "2026-09-21", "2026-09-27")
	if err != nil {
		return listexport.File{}, err
	}
	f.hours = hours
	return listexport.File{Filename: "Stundenuebersicht.xlsx", Data: []byte("sheet")}, nil
}

// The hours template (#3819) is served from the Dienstplan's own weekly
// summaries: the facade hands the export a reader over the overview, which
// carries the per-Schichtart split and the target unchanged.
func TestPlanningExportsTheHoursSheetFromTheWeeklySummaries(t *testing.T) {
	t.Parallel()
	monday := timezone.NewDate(2026, 9, 21)
	typeID := int64(4)
	target, delta := 1320, -30
	anna := &usersModel.Staff{Person: &usersModel.Person{FirstName: "Anna", LastName: "Müller"}}
	anna.ID = 7
	export := &hoursExport{}
	p := newPlanningFacade(planning.PlanningDependencies{
		Overview: planningOverview(func(_ context.Context, from, to timezone.Date) (*planning.StaffScheduleOverview, error) {
			require.Equal(t, monday, from)
			require.Equal(t, monday.AddDays(6), to)
			return &planning.StaffScheduleOverview{
				Staff: []*usersModel.Staff{anna, nil},
				WeeklySummaries: []planning.StaffWeeklySummary{{
					StaffID: 7, WeekStart: monday, PlannedMinutes: 1290, TargetMinutes: &target, DeltaMinutes: &delta,
					ByShiftType: []planning.ShiftTypeMinutes{{ShiftTypeID: &typeID, Minutes: 1200}, {Minutes: 90}},
				}},
			}, nil
		}),
		PlanExport: export,
	})

	file, err := p.ExportPlan(context.Background(), workforce.PlanExportRequest{
		From: "2026-09-23", To: "2026-09-23", Template: "hours", Format: "xlsx",
	})
	require.NoError(t, err)
	require.Equal(t, "Stundenuebersicht.xlsx", file.Filename)
	require.Equal(t, planexport.TemplateByHours, export.params.Template)
	require.Equal(t, []*planexport.StaffMember{{ID: 7, FirstName: "Anna", LastName: "Müller"}}, export.hours.Staff)
	require.Equal(t, []planexport.WeeklySummary{{
		StaffID: 7, WeekStart: "2026-09-21", PlannedMinutes: 1290, TargetMinutes: &target, DeltaMinutes: &delta,
		ByShiftType: []planexport.ShiftTypeMinutes{{ShiftTypeID: &typeID, Minutes: 1200}, {Minutes: 90}},
	}}, export.hours.Summaries)
}
