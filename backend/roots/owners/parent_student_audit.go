package owners

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func NewParentStudentAudit(source peopledirectory.StudentAuditActorCommand) parentportal.NativeStudentAudit {
	return parentportal.NewNativeStudentAudit(func(ctx context.Context, before, after *parentportal.StudentSnapshot, actor int64) error {
		return source.RecordStudentChangesForActor(ctx, parentStudentAuditSnapshot(before), parentStudentAuditSnapshot(after), actor)
	})
}

func parentStudentAuditSnapshot(value *parentportal.StudentSnapshot) *peopledirectory.StudentAuditSnapshot {
	if value == nil {
		return nil
	}
	snapshot := &peopledirectory.StudentAuditSnapshot{StudentID: value.ID, Status: value.Status, ExtraInfo: value.ExtraInfo, HealthInfo: value.HealthInfo, PickupStatus: value.PickupStatus, DepartureCompanionNote: value.DepartureCompanionNote}
	if value.EnrolledUntil != nil {
		snapshot.CareEnd = value.EnrolledUntil.String()
	}
	if value.AllowedDepartureModes != nil {
		snapshot.AllowedDepartureModes = make(peopledirectory.AllowedDepartureModes, len(value.AllowedDepartureModes))
		for day, modes := range value.AllowedDepartureModes {
			if modes == nil {
				snapshot.AllowedDepartureModes[day] = nil
				continue
			}
			values := make([]peopledirectory.DepartureMode, len(modes))
			for i, mode := range modes {
				values[i] = peopledirectory.DepartureMode(mode)
			}
			snapshot.AllowedDepartureModes[day] = values
		}
	}
	if value.DepartureDays != nil {
		snapshot.DepartureDays = make(peopledirectory.DepartureDays, len(value.DepartureDays))
		for day, mode := range value.DepartureDays {
			snapshot.DepartureDays[day] = peopledirectory.DepartureMode(mode)
		}
	}
	return snapshot
}
