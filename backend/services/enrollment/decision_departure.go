package enrollment

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// DecisionDepartureCompanions is backed by the existing Care Plan adapter.
// Enrollment coordinates the graph; only the two owners persist their rows.
type DecisionDepartureCompanions interface {
	ListForStudent(context.Context, int64) ([]*users.StudentCompanion, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
}

func (s *decisionService) applyEnrollmentDeparture(ctx context.Context, before, student *users.Student) error {
	if s.DepartureCompanions == nil || s.DeleteDepartureCompanions == nil {
		return errors.New("decision: departure companion capability is required")
	}
	// The owner takes the shared class gate before locking the subject row.
	current, err := s.readEnrollmentStudent(ctx, student.ID, "update")
	if err != nil {
		return err
	}
	enrollmentRebaseUntouchedDeparturePlan(student, current)
	allowed := enrollmentResolveAllowedDepartureModes(student, current)
	student.AllowedDepartureModes = allowed
	student.DepartureDays = allowed.DepartureDays()
	student.BusDays = allowed.BusDays()
	student.PickupDays = allowed.PickupDays()
	edges, err := s.DepartureCompanions.ListForStudent(ctx, student.ID)
	if err != nil {
		return err
	}
	accompanied := users.AccompaniedWeekdays(allowed, student.DepartureDays)
	removedDays := map[int64][]string{}
	var dropIDs []int64
	for _, edge := range edges {
		far, ok := edge.Other(student.ID)
		if !ok {
			continue
		}
		day := users.CompanionWeekdayKeys[edge.Weekday]
		if accompanied[day] {
			student.MarkDepartureCompanionDays(day)
		} else {
			dropIDs = append(dropIDs, edge.ID)
			removedDays[far] = append(removedDays[far], day)
		}
	}
	if err := s.checkEnrollmentCompanionRemoval(ctx, student.ID, removedDays); err != nil {
		return err
	}
	if err := student.Validate(); err != nil {
		return err
	}
	if !allowed.HasMode(users.DepartureAccompanied) {
		student.DepartureCompanionNote = nil
	}
	patch := enrollmentProfilePatch(before, student)
	patch.DepartureSet = true
	patch.DepartureCompanionNote = student.DepartureCompanionNote
	patch.DepartureCompanionDays = student.DepartureCompanionDays
	patch.AllowedDepartureModes = map[string][]string{}
	for day, modes := range allowed {
		for _, mode := range modes {
			patch.AllowedDepartureModes[day] = append(patch.AllowedDepartureModes[day], string(mode))
		}
	}
	if err := s.StudentEnrollment.ApplyEnrollmentProfile(ctx, student.ID, patch); err != nil {
		return err
	}
	if len(dropIDs) > 0 {
		if err := s.DeleteDepartureCompanions(ctx, dropIDs); err != nil {
			return err
		}
		users.RecordCompanionChange(ctx)
	}
	status := allowed.LegacyPickupStatus()
	student.PickupStatus = &status
	student.SnapshotDeparturePlan()
	return nil
}

func (s *decisionService) checkEnrollmentCompanionRemoval(ctx context.Context, subjectID int64, removedDays map[int64][]string) error {
	if len(removedDays) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(removedDays))
	for id := range removedDays {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	farEnds := make(map[int64]*users.Student, len(ids))
	for _, id := range ids {
		var student *users.Student
		var err error
		if id < subjectID {
			student, err = s.readEnrollmentStudent(ctx, id, "nowait")
		} else {
			student, err = s.readEnrollmentStudent(ctx, id, "update")
		}
		switch {
		case errors.Is(err, sql.ErrNoRows):
			continue
		case modelBase.IsLockNotAvailable(err):
			return users.ErrCompanionLockBusy
		case err != nil:
			return err
		}
		farEnds[id] = student
	}
	if batch := users.CompanionStrandingBatchFromContext(ctx); batch != nil {
		for _, id := range ids {
			batch.Defer(id, removedDays[id])
		}
		return nil
	}
	covered, err := s.DepartureCompanions.CompanionDaysCoveredExcluding(ctx, ids, subjectID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		student := farEnds[id]
		if student == nil || (student.DepartureCompanionNote != nil && strings.TrimSpace(*student.DepartureCompanionNote) != "") {
			continue
		}
		accompanied := users.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)
		for _, day := range removedDays[id] {
			if accompanied[day] && !covered[id][day] {
				return users.ErrCompanionWouldLoseDeparture
			}
		}
	}
	return nil
}
