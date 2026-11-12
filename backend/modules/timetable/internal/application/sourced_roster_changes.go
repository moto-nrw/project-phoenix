package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain"
)

var _ timetable.SourcedRosterCommands = (*Service)(nil)
var _ timetable.SourcedInstanceStudents = (*Service)(nil)

func (s *Service) SaveSourcedRosterChanges(ctx context.Context, changes timetable.SourcedRosterChanges) error {
	for _, row := range changes.Create {
		if !validStudentEnrollment(row) {
			return s.reject("save_sourced_roster_changes", timetable.ErrInvalidStudentEnrollment)
		}
	}
	if hasInvalidID(changes.Delete) {
		return s.reject("save_sourced_roster_changes", timetable.ErrInvalidStudentEnrollment)
	}
	for id, end := range changes.Ends {
		if id <= 0 || !timetable.ValidDate(end) {
			return s.reject("save_sourced_roster_changes", timetable.ErrInvalidStudentEnrollment)
		}
	}
	if len(changes.Create)+len(changes.Delete)+len(changes.Ends) == 0 {
		return nil
	}
	return s.runWrite(ctx, "save_sourced_roster_changes", true, func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SaveSourcedRosterChanges(ctx, changes)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) UpdateGroupOfferingSources(ctx context.Context, changes map[int64]timetable.OfferingSourceInput) error {
	for id, fields := range changes {
		source := timetable.GroupInput{TargetGroupType: timetable.TargetGroupTypeOffering, SourceCareOfferingIDs: fields.CareOfferingIDs, SourceGradeLevels: fields.GradeLevels, SourceSchoolClasses: fields.SchoolClasses}
		if id <= 0 || !normalizeOfferingSource(&source) {
			return s.reject("update_group_offering_sources", timetable.ErrInvalidGroup)
		}
		changes[id] = timetable.OfferingSourceInput{CareOfferingIDs: source.SourceCareOfferingIDs, GradeLevels: source.SourceGradeLevels, SchoolClasses: source.SourceSchoolClasses}
	}
	if len(changes) == 0 {
		return nil
	}
	return s.runWrite(ctx, "update_group_offering_sources", true, func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.UpdateGroupOfferingSources(ctx, changes)
		stats.Add(queryStats)
		return err
	})
}

func (s *Service) SaveSourcedInstanceStudents(ctx context.Context, creates []timetable.InstanceStudentInput, deletes []int64) error {
	for _, row := range creates {
		if !validInstanceStudent(row) {
			return s.reject("save_sourced_instance_students", timetable.ErrInvalidInstanceStudent)
		}
	}
	if hasInvalidID(deletes) {
		return s.reject("save_sourced_instance_students", timetable.ErrInvalidInstanceStudent)
	}
	if len(creates)+len(deletes) == 0 {
		return nil
	}
	return s.runWrite(ctx, "save_sourced_instance_students", true, func(ctx context.Context, stats *domain.OperationStats) error {
		queryStats, err := s.store.SaveSourcedInstanceStudents(ctx, creates, deletes)
		stats.Add(queryStats)
		return err
	})
}
