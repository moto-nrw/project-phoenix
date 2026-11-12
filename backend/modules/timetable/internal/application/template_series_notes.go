package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-ogs/project-phoenix/modules/timetable/ports"
)

// propagateNotesToPredecessors keeps the durable Wochennotiz uniform across a
// split series without changing a predecessor's recurrence or roster. It uses
// the predecessor's stored fields when reusing the regular template update
// capability, so only Notes differs from that segment's current state.
func (s *TemplateService) propagateNotesToPredecessors(ctx context.Context, in TemplateUpdateInput) error {
	if !in.PropagateNotesToPredecessors {
		return nil
	}
	segments, err := loadTemplateSeriesSegments(
		ctx,
		s.deps.ActivityGroupRepo,
		s.deps.ActivityScheduleRepo,
		s.getLogger(),
		in.TemplateID,
	)
	if err != nil {
		return err
	}
	for _, segment := range segments {
		if segment.Group == nil || segment.Group.ID == in.TemplateID || segment.ValidUntil == nil {
			continue
		}
		updated, err := s.deps.ActivityGroupRepo.UpdateTemplate(
			ctx,
			segment.Group.ID,
			templateUpdateWithNotes(segment.Group, in.Fields.Notes),
		)
		if err != nil {
			return &ports.ScheduleError{Op: "update template: propagate notes to predecessors", Err: err}
		}
		if updated != 1 {
			return &ports.ScheduleError{
				Op:  "update template: propagate notes to predecessors",
				Err: fmt.Errorf("expected predecessor template %d to change, got %d", segment.Group.ID, updated),
			}
		}
	}
	return nil
}

func templateUpdateWithNotes(group *activityContract.Group, notes *string) timetable.TemplateUpdate {
	roomID := int64(0)
	if group.PlannedRoomID != nil {
		roomID = *group.PlannedRoomID
	}
	includeClosingDays := group.IncludeClosingDays
	return timetable.TemplateUpdate{
		Name:                    group.Name,
		Type:                    group.Type,
		CategoryID:              group.CategoryID,
		PlanningTrackID:         group.PlanningTrackID,
		PlanningTrackIDProvided: true,
		RoomID:                  roomID,
		EducationGroupID:        group.EducationGroupID,
		MaxParticipants:         group.MaxParticipants,
		MaxParticipantsProvided: true,
		RequiredStaff:           group.RequiredStaff,
		CalendarPeriodID:        group.CalendarPeriodID,
		TargetGroupType:         group.TargetGroupType,
		TargetGradeLevel:        group.TargetGradeLevel,
		TargetSchoolClass:       group.TargetSchoolClass,
		ListKind:                group.ListKind,
		Notes:                   notes,
		SourceCareOfferingIDs:   group.SourceCareOfferingIDs,
		SourceGradeLevels:       group.SourceGradeLevels,
		SourceSchoolClasses:     group.SourceSchoolClasses,
		IncludeClosingDays:      &includeClosingDays,
	}
}
