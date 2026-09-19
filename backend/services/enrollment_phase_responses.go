package services

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/services/enrollment"
)

// phaseResponseRoster feeds the response overview (#3379) with the children
// the school cares for today. Only active children are asked to re-enroll: a
// pending child has not started yet and its own enrollment is still running.
type phaseResponseRoster struct {
	query peopledirectory.StudentDirectoryQuery
}

func (d phaseResponseRoster) ListRunningStudents(ctx context.Context, today timezone.Date) ([]enrollment.PhaseResponseStudent, error) {
	entries, err := d.query.ListStudentRoster(ctx, today.String())
	if err != nil {
		return nil, err
	}
	result := make([]enrollment.PhaseResponseStudent, 0, len(entries))
	for _, entry := range entries {
		if entry.Record.Status != peopledirectory.StudentStatusActive {
			continue
		}
		result = append(result, enrollment.PhaseResponseStudent{
			ID: entry.Record.ID, FirstName: entry.FirstName, LastName: entry.LastName,
			SchoolClass: entry.Record.SchoolClass,
		})
	}
	return result, nil
}

type phaseResponseCareExits struct{ query careplan.CareRecordsQuery }

func (d phaseResponseCareExits) StudentsWithCareExit(ctx context.Context, studentIDs []int64) (map[int64]bool, error) {
	exits, err := d.query.FindCareExits(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]bool, len(exits))
	for studentID := range exits {
		result[studentID] = true
	}
	return result, nil
}

func newPhaseResponseSources(
	children enrollment.PhaseResponseChildren,
	persons peopledirectory.Capability,
	carePlan careplan.CareRecordsQuery,
) *enrollment.PhaseResponseSources {
	return &enrollment.PhaseResponseSources{
		Children:       children,
		Roster:         phaseResponseRoster{query: persons},
		CareExits:      phaseResponseCareExits{query: carePlan},
		PortalAccounts: persons,
	}
}
