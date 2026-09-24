package compose

import (
	"context"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// rosterWarnings flags the roster's children: a late or missing expected
// arrival, a class-wide arrival exception (#2962), and a child outside the
// class groups the template addresses. A failed arrival read only drops the
// arrival warnings.
func (s *operations) rosterWarnings(ctx context.Context, src rosterSources) map[int64][]timetable.OperationRosterWarning {
	warnings := make(map[int64][]timetable.OperationRosterWarning)
	if len(src.studentIDs) == 0 {
		return warnings
	}
	arrivals, err := s.deps.Arrivals.EffectiveArrivals(ctx, src.studentIDs, timezone.Date(src.inst.Date))
	if err != nil {
		s.logger().WarnContext(
			ctx,
			"could not load arrival times for timetable roster warnings",
			slog.String("error", err.Error()),
			slog.Int64("instance_id", src.inst.ID),
		)
	} else {
		appendArrivalWarnings(warnings, arrivals, src.inst)
	}
	appendClassMismatchWarnings(warnings, src)
	return warnings
}

// appendClassMismatchWarnings flags the children whose education group is
// not one the template addresses; with exactly one addressed group the
// warning names it.
func appendClassMismatchWarnings(warnings map[int64][]timetable.OperationRosterWarning, src rosterSources) {
	expectedGroupIDs := rosterMismatchExpectedGroupIDs(src.template)
	if len(expectedGroupIDs) == 0 {
		return
	}
	var expectedGroupID *int64
	var expectedGroupName *string
	if len(expectedGroupIDs) == 1 {
		for groupID := range expectedGroupIDs {
			expectedGroupID = &groupID
			if name, ok := src.groupNames[groupID]; ok {
				expectedGroupName = &name
			}
		}
	}
	for _, studentID := range src.studentIDs {
		st := src.students[studentID]
		if st == nil {
			continue
		}
		if st.GroupID != nil {
			if _, matches := expectedGroupIDs[*st.GroupID]; matches {
				continue
			}
		}
		warnings[studentID] = append(warnings[studentID], timetable.OperationRosterWarning{
			Kind:                  "template_class_mismatch",
			Message:               "Kind passt nicht zur Klassengruppe der Betreuungsplan-Vorlage.",
			ExpectedGroupID:       expectedGroupID,
			ExpectedGroupName:     expectedGroupName,
			CurrentEducationGroup: st.GroupID,
		})
	}
}

func rosterMismatchExpectedGroupIDs(group *rosterTemplateGroup) map[int64]struct{} {
	if group == nil {
		return nil
	}
	if len(group.Targets) == 0 {
		if group.EducationGroupID == nil {
			return nil
		}
		return map[int64]struct{}{*group.EducationGroupID: {}}
	}
	expected := make(map[int64]struct{}, len(group.Targets))
	for _, target := range group.Targets {
		if target == nil || target.TargetGroupType != activitiesModels.TargetGroupTypeGruppe || target.EducationGroupID == nil {
			continue
		}
		expected[*target.EducationGroupID] = struct{}{}
	}
	return expected
}

func appendArrivalWarnings(warnings map[int64][]timetable.OperationRosterWarning, arrivals map[int64]*ExpectedArrival, inst *scheduleModels.ActivityInstance) {
	slotStart := inst.StartTime.Format("15:04")
	slotStartClock := timezone.NormalizeWallClock(inst.StartTime)
	for studentID, arrival := range arrivals {
		if arrival == nil {
			continue
		}
		if arrival.ArrivalTime == nil {
			if !arrival.IsException {
				warnings[studentID] = append(warnings[studentID], timetable.OperationRosterWarning{
					Kind:      "missing_arrival_schedule",
					Message:   "Für diesen Tag ist keine erwartete Ankunft hinterlegt.",
					SlotStart: &slotStart,
				})
			}
			continue
		}
		arrivesLate := timezone.NormalizeWallClock(*arrival.ArrivalTime).After(slotStartClock)
		if arrivesLate {
			expectedArrival := arrival.ArrivalTime.Format("15:04")
			warnings[studentID] = append(warnings[studentID], timetable.OperationRosterWarning{
				Kind:            "arrival_after_slot_start",
				Message:         "Erwartete Ankunft liegt nach dem Start dieser Betreuung.",
				ExpectedArrival: &expectedArrival,
				SlotStart:       &slotStart,
			})
		}
		appendClassArrivalNotice(warnings, studentID, arrival.ClassException, arrivesLate, slotStart)
	}
}

// appendClassArrivalNotice explains a class-wide day exception (#2962): it
// is information, not a warning. When the row already says "Kommt um HH:MM
// Uhr" from the late-arrival warning, the line adds only the reason.
func appendClassArrivalNotice(warnings map[int64][]timetable.OperationRosterWarning, studentID int64, notice *ClassArrivalNotice, arrivesLate bool, slotStart string) {
	if notice == nil {
		return
	}
	expectedArrival := notice.ArrivalTime
	message := "Kommt heute um " + expectedArrival + " Uhr (" + notice.Label + ")"
	if arrivesLate {
		message = notice.Label
	}
	warnings[studentID] = append(warnings[studentID], timetable.OperationRosterWarning{
		Kind:            "class_arrival_exception",
		Message:         message,
		ExpectedArrival: &expectedArrival,
		SlotStart:       &slotStart,
	})
}
