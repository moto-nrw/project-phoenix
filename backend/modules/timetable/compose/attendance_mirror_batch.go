package compose

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	scheduleModel "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

// The roomless and batch variants of the attendance mirror. The batch calls
// resolve many students at one shared instant with one read and one guarded
// UPDATE (review #2372) instead of two round trips per student; bulk presence
// events carry no attendance enrichment by design (#848), so they return no
// snapshots.

// instanceLister is the list read the retained instance repository serves
// beside its model interface.
type instanceLister interface {
	List(ctx context.Context, options *modelBase.QueryOptions) ([]*scheduleModel.ActivityInstance, error)
}

// MirrorCheckOutAt closes the latest open slot attendance for roomless binary
// mode. It never changes another slot's status or history.
func (s *attendanceMirror) MirrorCheckOutAt(ctx context.Context, studentID int64, at time.Time) (err error) {
	defer s.recoverMirrorPanic(ctx, "roomless attendance checkout mirror panic", "MirrorCheckOutAt panic", &err)

	day := scheduleModel.Date(timezone.DateFromTime(at))
	rows, err := s.instanceStudentRepo.FindByStudentAndDateRange(ctx, studentID, day, day)
	if err != nil {
		s.getLogger().Warn("roomless attendance checkout mirror: slot lookup failed",
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("MirrorCheckOutAt: read slot attendance: %w", err)
	}
	latest := latestOpenRow(rows)
	if latest == nil {
		return nil
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckout(ctx, latest.InstanceID, studentID, at); err != nil {
		s.getLogger().Error("roomless attendance checkout mirror UPDATE failed",
			slog.Int64("instance_id", latest.InstanceID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("MirrorCheckOutAt: write slot checkout: %w", err)
	}
	return nil
}

// latestOpenRow picks the most recently checked-in present row that is still
// open.
func latestOpenRow(rows []*scheduleModel.InstanceStudent) *scheduleModel.InstanceStudent {
	var latest *scheduleModel.InstanceStudent
	for _, row := range rows {
		if isOpenPresence(row) && (latest == nil || row.CheckedInAt.After(*latest.CheckedInAt)) {
			latest = row
		}
	}
	return latest
}

func isOpenPresence(row *scheduleModel.InstanceStudent) bool {
	return row.Status == scheduleModel.AttendanceStatusPresent && row.CheckedInAt != nil && row.CheckedOutAt == nil
}

// latestOpenPresence picks, per student, the most recently checked-in present
// row that is still open.
func latestOpenPresence(rows []*scheduleModel.InstanceStudent) map[int64]*scheduleModel.InstanceStudent {
	latestByStudent := make(map[int64]*scheduleModel.InstanceStudent)
	for _, row := range rows {
		if !isOpenPresence(row) {
			continue
		}
		latest := latestByStudent[row.StudentID]
		if latest == nil || row.CheckedInAt.After(*latest.CheckedInAt) {
			latestByStudent[row.StudentID] = row
		}
	}
	return latestByStudent
}

// MirrorCheckInAtBatch resolves many roomless check-ins at one shared instant
// with one candidate query and one guarded UPDATE (review #2372). Per student
// the same rule as MirrorCheckInAt applies: exactly one currently-running
// booked slot, ambiguous or unbooked students stay unassigned, and rows in a
// manual or already-open state are preserved.
func (s *attendanceMirror) MirrorCheckInAtBatch(ctx context.Context, studentIDs []int64, at time.Time) (err error) {
	defer s.recoverMirrorPanic(ctx, "roomless attendance batch mirror panic", "batch check-in sync panic", &err)
	if len(studentIDs) == 0 {
		return nil
	}

	rows, err := s.instanceStudentRepo.FindCurrentCandidatesByStudentIDs(
		ctx, studentIDs, scheduleModel.DateFromTime(at), at,
	)
	if err != nil {
		s.getLogger().Warn("roomless attendance batch mirror: candidate lookup failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("batch check-in sync: find candidates: %w", err)
	}

	keys := s.uniqueCheckinKeys(rows, len(studentIDs))
	if len(keys) == 0 {
		return nil
	}
	if err := s.instanceStudentRepo.UpdateAttendanceFromCheckinBatch(ctx, keys, at); err != nil {
		s.getLogger().Error("roomless attendance batch mirror UPDATE failed",
			slog.Int("row_count", len(keys)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("batch check-in sync: update slots: %w", err)
	}
	return nil
}

// uniqueCheckinKeys keeps the students with exactly one candidate slot whose
// state a check-in may replace.
func (s *attendanceMirror) uniqueCheckinKeys(rows []*scheduleModel.InstanceStudent, students int) []scheduleModel.InstanceStudentKey {
	byStudent := make(map[int64][]*scheduleModel.InstanceStudent, students)
	for _, row := range rows {
		byStudent[row.StudentID] = append(byStudent[row.StudentID], row)
	}
	keys := make([]scheduleModel.InstanceStudentKey, 0, len(byStudent))
	for studentID, candidates := range byStudent {
		if len(candidates) != 1 {
			s.getLogger().Debug("roomless attendance batch mirror: slot assignment is not unique",
				slog.Int64("student_id", studentID),
				slog.Int("candidate_count", len(candidates)),
			)
			continue
		}
		if shouldPreserveAttendanceOnCheckin(candidates[0]) {
			continue
		}
		keys = append(keys, scheduleModel.InstanceStudentKey{
			InstanceID: candidates[0].InstanceID,
			StudentID:  studentID,
		})
	}
	return keys
}

// MirrorCheckOutAtBatch closes many students' latest open slot attendance at
// one shared instant: one slot query and one guarded UPDATE (review #2372).
// Per student the same rule as MirrorCheckOutAt applies — only the most
// recently checked-in open present row of the day closes.
func (s *attendanceMirror) MirrorCheckOutAtBatch(ctx context.Context, studentIDs []int64, at time.Time) (err error) {
	defer s.recoverMirrorPanic(ctx, "roomless attendance batch checkout mirror panic", "MirrorCheckOutAtBatch panic", &err)
	if len(studentIDs) == 0 {
		return nil
	}

	day := timezone.DateFromTime(at)
	rows, err := s.instanceStudentRepo.FindByStudentIDsAndDate(ctx, studentIDs, scheduleModel.Date(day))
	if err != nil {
		s.getLogger().Warn("roomless attendance batch checkout mirror: slot lookup failed",
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("MirrorCheckOutAtBatch: read slot attendance: %w", err)
	}

	latestByStudent := latestOpenPresence(rows)
	keys := make([]scheduleModel.InstanceStudentKey, 0, len(latestByStudent))
	for studentID, row := range latestByStudent {
		keys = append(keys, scheduleModel.InstanceStudentKey{InstanceID: row.InstanceID, StudentID: studentID})
	}
	if len(keys) == 0 {
		return nil
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckoutBatch(ctx, keys, at); err != nil {
		s.getLogger().Error("roomless attendance batch checkout mirror UPDATE failed",
			slog.Int("row_count", len(keys)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("MirrorCheckOutAtBatch: write slot checkout: %w", err)
	}
	return nil
}

// MirrorCheckOutForVisits stamps the slot checkout for many visits ended at
// one shared instant. Instances are resolved once per distinct active group
// and all slot checkouts land in one guarded UPDATE (review #2372) — the
// per-visit read of MirrorCheckOutForVisit exists only for its SSE snapshot,
// which bulk events do not carry (#848), so the batch skips it and relies on
// the UPDATE's own guards.
func (s *attendanceMirror) MirrorCheckOutForVisits(ctx context.Context, visits []timetable.AttendanceVisit, at time.Time) (err error) {
	defer s.recoverMirrorPanic(ctx, "attendance batch visit checkout mirror panic", "visit batch checkout sync panic", &err)
	groupIDs := visitActiveGroupIDs(visits)
	if len(groupIDs) == 0 {
		return nil
	}
	instanceByGroup, err := s.instancesByActiveGroup(ctx, groupIDs)
	if err != nil {
		s.getLogger().Warn("attendance batch mirror: find instances by active_group_id failed", slog.String("error", err.Error()))
		return fmt.Errorf("visit batch checkout sync: find instances: %w", err)
	}
	keys := make([]scheduleModel.InstanceStudentKey, 0, len(visits))
	for _, visit := range visits {
		instance := instanceByGroup[visit.ActiveGroupID]
		if visit.ActiveGroupID <= 0 || instance == nil {
			continue // walk-in / no bridged instance — nothing to mirror
		}
		keys = append(keys, scheduleModel.InstanceStudentKey{InstanceID: instance.ID, StudentID: visit.StudentID})
	}
	if len(keys) == 0 {
		return nil
	}
	if err := s.instanceStudentRepo.UpdateAttendanceCheckoutBatch(ctx, keys, at); err != nil {
		s.getLogger().Error("attendance batch visit checkout mirror UPDATE failed",
			slog.Int("row_count", len(keys)),
			slog.String("error", err.Error()),
		)
		return fmt.Errorf("visit batch checkout sync: update slots: %w", err)
	}
	return nil
}

// visitActiveGroupIDs lists the distinct sessions of the visits.
func visitActiveGroupIDs(visits []timetable.AttendanceVisit) []int64 {
	groupIDs := make([]int64, 0, len(visits))
	seenGroups := make(map[int64]bool, len(visits))
	for _, visit := range visits {
		if visit.ActiveGroupID > 0 && !seenGroups[visit.ActiveGroupID] {
			seenGroups[visit.ActiveGroupID] = true
			groupIDs = append(groupIDs, visit.ActiveGroupID)
		}
	}
	return groupIDs
}

// instancesByActiveGroup resolves the blocks bridged to the sessions in one
// read.
func (s *attendanceMirror) instancesByActiveGroup(ctx context.Context, groupIDs []int64) (map[int64]*scheduleModel.ActivityInstance, error) {
	lister, ok := s.instanceRepo.(instanceLister)
	if !ok {
		return nil, fmt.Errorf("legacy list capability is not configured for %T", s.instanceRepo)
	}
	args := make([]any, len(groupIDs))
	for i, id := range groupIDs {
		args[i] = id
	}
	instances, err := lister.List(ctx, &modelBase.QueryOptions{
		Filter: modelBase.NewFilter().In("active_group_id", args...),
	})
	if err != nil {
		return nil, err
	}
	instanceByGroup := make(map[int64]*scheduleModel.ActivityInstance, len(instances))
	for _, instance := range instances {
		if instance.ActiveGroupID != nil {
			instanceByGroup[*instance.ActiveGroupID] = instance
		}
	}
	return instanceByGroup, nil
}
