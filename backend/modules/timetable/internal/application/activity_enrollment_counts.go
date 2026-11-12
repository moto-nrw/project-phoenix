package application

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/modules/timetable"
	activityContract "github.com/moto-ogs/project-phoenix/modules/timetable/activitycontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// GetEnrollmentCounts batches the roster reads, not a raw enrollment COUNT:
// retained history rows count once each, but missing, graduated and departed
// children do not. The directory reads remain observable even though their
// display names and alumni annotations are not needed by the HTTP response.
func (s *ActivityService) GetEnrollmentCounts(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	if len(groupIDs) == 0 {
		return map[int64]int{}, nil
	}
	counts, err := s.batchEnrollmentCounts(ctx, groupIDs)
	if err == nil {
		return counts, nil
	}
	var sqlError interface{ Field(byte) string }
	if errors.As(err, &sqlError) {
		// These reads use the same relations in each group. A statement-level
		// database failure also fails the first applicable scalar read. Do not
		// issue another statement in its now-aborted ambient transaction.
		return nil, err
	}
	// A later group's decoder failure must not replace an earlier group's
	// error. Decode failures leave the transaction usable, so resolve the
	// original error in the original group/read order. No success is cached.
	for _, groupID := range groupIDs {
		if _, firstErr := s.GetEnrolledStudents(ctx, groupID); firstErr != nil {
			return nil, firstErr
		}
	}
	return nil, err
}

func (s *ActivityService) batchEnrollmentCounts(ctx context.Context, groupIDs []int64) (map[int64]int, error) {
	rows, err := listLegacyStudentEnrollments(ctx, s.timetable, timetable.StudentEnrollmentFilter{
		ActivityGroupIDs: groupIDs, OrderByValidFrom: true,
	}, "find by group ID")
	if err != nil {
		return nil, enrolledStudentsError(err)
	}
	ids := make([]int64, 0, len(rows))
	populated := make(map[int64]bool, len(groupIDs))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
		populated[row.ActivityGroupID] = true
	}
	if err := s.readEmptyCountGroups(ctx, groupIDs, populated); err != nil {
		return nil, enrolledStudentsError(err)
	}
	counts := make(map[int64]int, len(groupIDs))
	if len(rows) == 0 {
		return counts, nil
	}
	if err := s.readCountDirectories(ctx, ids); err != nil {
		return nil, enrolledStudentsError(err)
	}
	students, err := s.studentRepo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, enrolledStudentsError(err)
	}
	for _, row := range rows {
		student := students[row.StudentID]
		if student != nil && !student.IsAlumnus() && !student.CareEndedOn(calendar.TodayDate()) {
			counts[row.ActivityGroupID]++
		}
	}
	return counts, nil
}

func (s *ActivityService) readEmptyCountGroups(ctx context.Context, groupIDs []int64, populated map[int64]bool) error {
	for _, id := range groupIDs {
		if !populated[id] {
			// The original scalar path still calls this port for an empty
			// roster. Its production binding returns without issuing SQL.
			if _, err := s.studentRepo.FindByIDs(ctx, []int64{}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *ActivityService) readCountDirectories(ctx context.Context, ids []int64) error {
	if s.directories.StudentAlumni == nil || s.directories.StudentNames == nil {
		return errors.New("student enrollment repository: people directory is required")
	}
	if _, err := s.directories.StudentAlumni(ctx, ids); err != nil {
		return err
	}
	_, err := s.directories.StudentNames(ctx, ids)
	return err
}

func enrolledStudentsError(err error) error {
	return &activityContract.ActivityError{Op: "get enrolled students", Err: err}
}
