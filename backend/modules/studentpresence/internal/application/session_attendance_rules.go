package application

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
)

var (
	ErrPlannedRosterUnbound     = errors.New("student presence: planned roster is not bound")
	ErrCarePlanDirectoryUnbound = errors.New("student presence: care plan directory is not bound")
)

func (s *Service) plannedRoster() (ports.PlannedRoster, error) {
	if s.roster == nil {
		return nil, ErrPlannedRosterUnbound
	}
	return s.roster, nil
}

func (s *Service) carePlanRules() (ports.CarePlanDirectory, ports.CareDayLocker, error) {
	if s.carePlan == nil || s.careDays == nil {
		return nil, nil, ErrCarePlanDirectoryUnbound
	}
	return s.carePlan, s.careDays, nil
}

// completedInstanceIDs returns which of the instances have a completed
// session; the attendance rules leave a finished block alone.
func (s *Service) completedInstanceIDs(ctx context.Context, instanceIDs []int64, stats *ports.Stats) (map[int64]bool, error) {
	result := map[int64]bool{}
	if len(instanceIDs) == 0 {
		return result, nil
	}
	sessions, sessionStats, err := s.store.ListActivitySessions(ctx, ports.ActivitySessionFilter{InstanceIDs: instanceIDs, Status: "completed"})
	addStats(stats, sessionStats)
	if err != nil {
		return nil, err
	}
	for _, session := range sessions {
		result[session.InstanceID] = true
	}
	return result, nil
}

func addStats(total *ports.Stats, part ports.Stats) {
	total.Queries += part.Queries
	total.StatementDuration += part.StatementDuration
}

func participantIDs(participants []ports.PlannedParticipant) []int64 {
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.ID)
	}
	return ids
}

func instanceIDsOf(participants []ports.PlannedParticipant) []int64 {
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.InstanceID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func statusDaySubstatus(status string) *string {
	var value string
	switch status {
	case "sick":
		value = "sick"
	case "excused":
		value = "excused"
	case "class_trip":
		value = "field_trip"
	default:
		return nil
	}
	return &value
}

func wallClock(value time.Time) string { return value.Format("15:04:05") }

func latestActiveStatusDay(ctx context.Context, carePlan ports.CarePlanDirectory, studentID int64, date string) (*ports.StudentStatusDay, error) {
	values, err := carePlan.ListStudentStatusDays(ctx, ports.StudentStatusDayFilter{
		StudentIDs: []int64{studentID}, Date: date, ActiveOnly: true, LatestOnly: true,
	})
	if err != nil || len(values) == 0 {
		return nil, err
	}
	return &values[0], nil
}

// ApplyStatusDay marks the student's planned participants of the day absent
// under the reported day status, when that status is the latest active one.
func (s *Service) ApplyStatusDay(ctx context.Context, studentID int64, date string, statusDayID int64, substatus string) (rows int, err error) {
	err = s.runWrite(ctx, "apply_status_day", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if studentID <= 0 || statusDayID <= 0 || date == "" {
			return stats, errors.New("student presence: invalid status day application")
		}
		roster, carePlan, locks, err := s.attendanceRulePorts()
		if err != nil {
			return stats, err
		}
		if err := locks.LockStudentAndExceptionDay(txCtx, studentID, date); err != nil {
			return stats, err
		}
		applies, err := statusDayApplies(txCtx, carePlan, studentID, date, statusDayID)
		if err != nil || !applies {
			return stats, err
		}
		participants, err := roster.ListPlannedParticipants(txCtx, ports.PlannedRosterFilter{StudentIDs: []int64{studentID}, Date: date, ExcludeCancelled: true})
		if err != nil {
			return stats, err
		}
		applyStats, err := s.store.ApplyStatusDayAbsences(txCtx, statusDayAbsencesFor(participants, statusDayID, substatus), time.Now().UTC())
		addStats(&stats, applyStats)
		rows = int(applyStats.Rows)
		return stats, err
	})
	return rows, err
}

// attendanceRulePorts binds the three ports every attendance rule needs.
func (s *Service) attendanceRulePorts() (ports.PlannedRoster, ports.CarePlanDirectory, ports.CareDayLocker, error) {
	roster, err := s.plannedRoster()
	if err != nil {
		return nil, nil, nil, err
	}
	carePlan, locks, err := s.carePlanRules()
	if err != nil {
		return nil, nil, nil, err
	}
	return roster, carePlan, locks, nil
}

// statusDayApplies reports whether the reported day status is the child's
// latest active one on the day, the only one that owns absences.
func statusDayApplies(ctx context.Context, carePlan ports.CarePlanDirectory, studentID int64, date string, statusDayID int64) (bool, error) {
	incoming, err := carePlan.FindStudentStatusDay(ctx, statusDayID, true)
	if err != nil {
		return false, err
	}
	latest, err := latestActiveStatusDay(ctx, carePlan, studentID, date)
	if err != nil {
		return false, err
	}
	return incoming != nil && latest != nil && incoming.StudentID == studentID && incoming.Date == date && latest.ID == statusDayID, nil
}

func statusDayAbsencesFor(participants []ports.PlannedParticipant, statusDayID int64, substatus string) []ports.StatusDayAbsence {
	absences := make([]ports.StatusDayAbsence, 0, len(participants))
	for _, participant := range participants {
		absences = append(absences, ports.StatusDayAbsence{ParticipantID: participant.ID, StatusDayID: statusDayID, Substatus: substatus})
	}
	return absences
}

// ReleaseStatusDay returns the participants the day status owned to the next
// active day status, or to expected; a block that already ended keeps the
// absence.
func (s *Service) ReleaseStatusDay(ctx context.Context, statusDayID int64) (rows int, err error) {
	err = s.runWrite(ctx, "release_status_day", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if statusDayID <= 0 {
			return stats, errors.New("student presence: invalid status day ID")
		}
		carePlan, locks, err := s.carePlanRules()
		if err != nil {
			return stats, err
		}
		released, err := lockedStatusDay(txCtx, carePlan, locks, statusDayID)
		if err != nil || released == nil {
			return stats, err
		}
		replacement, err := latestActiveStatusDay(txCtx, carePlan, released.StudentID, released.Date)
		if err != nil {
			return stats, err
		}
		released_, err := s.releaseStatusDayRows(txCtx, statusDayID, replacement, &stats)
		if err != nil {
			return stats, err
		}
		rows = int(released_)
		return stats, s.replayPartialAbsences(txCtx, carePlan, released.StudentID, released.Date, &stats)
	})
	return rows, err
}

// lockedStatusDay locks the child's exception day and re-reads the day
// status under the lock; nil when it does not exist.
func lockedStatusDay(ctx context.Context, carePlan ports.CarePlanDirectory, locks ports.CareDayLocker, statusDayID int64) (*ports.StudentStatusDay, error) {
	released, err := carePlan.FindStudentStatusDay(ctx, statusDayID, false)
	if err != nil || released == nil {
		return nil, err
	}
	if err := locks.LockStudentAndExceptionDay(ctx, released.StudentID, released.Date); err != nil {
		return nil, err
	}
	return carePlan.FindStudentStatusDay(ctx, statusDayID, false)
}

// releaseStatusDayRows moves the participants the day status owns to their
// next state and returns how many rows changed.
func (s *Service) releaseStatusDayRows(ctx context.Context, statusDayID int64, replacement *ports.StudentStatusDay, stats *ports.Stats) (int64, error) {
	owned, listStats, err := s.store.ListSessionAttendanceByStatusDay(ctx, statusDayID)
	addStats(stats, listStats)
	if err != nil {
		return 0, err
	}
	releases, err := s.statusDayReleases(ctx, owned, replacement, true, stats)
	if err != nil {
		return 0, err
	}
	releaseStats, err := s.store.ReleaseStatusDay(ctx, statusDayID, releases, time.Now().UTC())
	addStats(stats, releaseStats)
	return releaseStats.Rows, err
}

// statusDayReleases decides the next state of every owned participant. With
// keepCompleted, a participant of a finished block stays absent even without
// a replacement, which is what the old release did for reported day statuses.
func (s *Service) statusDayReleases(ctx context.Context, owned []ports.SessionAttendance, replacement *ports.StudentStatusDay, keepCompleted bool, stats *ports.Stats) ([]ports.StatusDayRelease, error) {
	if len(owned) == 0 {
		return nil, nil
	}
	roster, err := s.plannedRoster()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(owned))
	for _, row := range owned {
		ids = append(ids, row.ParticipantID)
	}
	participants, err := roster.ListPlannedParticipants(ctx, ports.PlannedRosterFilter{IDs: ids})
	if err != nil {
		return nil, err
	}
	completed, err := s.completedInstanceIDs(ctx, instanceIDsOf(participants), stats)
	if err != nil {
		return nil, err
	}
	instanceByParticipant := make(map[int64]int64, len(participants))
	for _, participant := range participants {
		instanceByParticipant[participant.ID] = participant.InstanceID
	}
	releases := make([]ports.StatusDayRelease, 0, len(owned))
	for _, row := range owned {
		instanceID, known := instanceByParticipant[row.ParticipantID]
		if !known {
			continue
		}
		if release, ok := statusDayRelease(row.ParticipantID, completed[instanceID], keepCompleted, replacement); ok {
			releases = append(releases, release)
		}
	}
	return releases, nil
}

// statusDayRelease decides the next state of one owned participant: absent
// under the replacement day status, absent because the block ended and the
// caller keeps those, or expected. A participant of a finished block the
// caller does not keep stays as it is.
func statusDayRelease(participantID int64, completed, keepCompleted bool, replacement *ports.StudentStatusDay) (ports.StatusDayRelease, bool) {
	if !keepCompleted && completed {
		return ports.StatusDayRelease{}, false
	}
	release := ports.StatusDayRelease{ParticipantID: participantID, Status: "expected"}
	if replacement != nil {
		release.ReplacementDayID = &replacement.ID
		release.Substatus = statusDaySubstatus(replacement.Status)
	}
	if replacement != nil || (keepCompleted && completed) {
		release.Status = "absent"
	}
	return release, true
}

func (s *Service) replayPartialAbsences(ctx context.Context, carePlan ports.CarePlanDirectory, studentID int64, date string, stats *ports.Stats) error {
	exceptions, err := carePlan.ListPickupExceptions(ctx, ports.PickupExceptionFilter{StudentIDs: []int64{studentID}, Date: date})
	if err != nil {
		return err
	}
	for _, exception := range exceptions {
		if _, err := s.applyPartialAbsence(ctx, exception, false, stats); err != nil {
			return err
		}
	}
	return nil
}

// applyPartialAbsence marks the student's planned participants of blocks
// starting at or after the excusal absent and excused. Finished blocks are
// only included when the caller says so.
func (s *Service) applyPartialAbsence(ctx context.Context, exception ports.PickupException, includeCompleted bool, stats *ports.Stats) (int64, error) {
	if exception.ExcusedFrom == nil {
		return 0, nil
	}
	roster, err := s.plannedRoster()
	if err != nil {
		return 0, err
	}
	participants, err := roster.ListPlannedParticipants(ctx, ports.PlannedRosterFilter{
		StudentIDs: []int64{exception.StudentID}, Date: exception.ExceptionDate,
		FromClock: wallClock(*exception.ExcusedFrom), ExcludeCancelled: true,
	})
	if err != nil {
		return 0, err
	}
	completed := map[int64]bool{}
	if !includeCompleted {
		if completed, err = s.completedInstanceIDs(ctx, instanceIDsOf(participants), stats); err != nil {
			return 0, err
		}
	}
	absences := make([]ports.PartialAbsence, 0, len(participants))
	for _, participant := range participants {
		if completed[participant.InstanceID] {
			continue
		}
		absences = append(absences, ports.PartialAbsence{ParticipantID: participant.ID, PickupExceptionID: exception.ID})
	}
	applyStats, err := s.store.ApplyPartialAbsences(ctx, absences, time.Now().UTC())
	addStats(stats, applyStats)
	return applyStats.Rows, err
}

func (s *Service) ApplyActiveStatusDaysForInstance(ctx context.Context, instanceID int64, date string) (rows int, err error) {
	err = s.runWrite(ctx, "apply_active_status_days_for_instance", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if instanceID <= 0 || date == "" {
			return stats, errors.New("student presence: invalid instance status day application")
		}
		roster, carePlan, locks, err := s.attendanceRulePorts()
		if err != nil {
			return stats, err
		}
		participants, err := roster.ListPlannedParticipants(txCtx, ports.PlannedRosterFilter{InstanceIDs: []int64{instanceID}})
		if err != nil {
			return stats, err
		}
		if err := lockStudentDays(txCtx, locks, uniqueStudentIDs(participants), date); err != nil {
			return stats, err
		}
		statuses, err := carePlan.ListStudentStatusDays(txCtx, ports.StudentStatusDayFilter{Date: date, ActiveOnly: true, LatestOnly: true})
		if err != nil {
			return stats, err
		}
		applyStats, err := s.store.ApplyStatusDayAbsences(txCtx, statusDayAbsencesByStudent(participants, statuses), time.Now().UTC())
		addStats(&stats, applyStats)
		rows = int(applyStats.Rows)
		return stats, err
	})
	return rows, err
}

func lockStudentDays(ctx context.Context, locks ports.CareDayLocker, studentIDs []int64, date string) error {
	for _, studentID := range studentIDs {
		if err := locks.LockStudentAndExceptionDay(ctx, studentID, date); err != nil {
			return err
		}
	}
	return nil
}

// statusDayAbsencesByStudent pairs every participant with the latest active
// day status of its child; children without one are left alone.
func statusDayAbsencesByStudent(participants []ports.PlannedParticipant, statuses []ports.StudentStatusDay) []ports.StatusDayAbsence {
	byStudent := make(map[int64]ports.StudentStatusDay, len(statuses))
	for _, status := range statuses {
		byStudent[status.StudentID] = status
	}
	absences := make([]ports.StatusDayAbsence, 0, len(participants))
	for _, participant := range participants {
		status, ok := byStudent[participant.StudentID]
		if !ok {
			continue
		}
		substatus := ""
		if value := statusDaySubstatus(status.Status); value != nil {
			substatus = *value
		}
		absences = append(absences, ports.StatusDayAbsence{ParticipantID: participant.ID, StatusDayID: status.ID, Substatus: substatus})
	}
	return absences
}

func uniqueStudentIDs(participants []ports.PlannedParticipant) []int64 {
	ids := make([]int64, 0, len(participants))
	for _, participant := range participants {
		ids = append(ids, participant.StudentID)
	}
	slices.Sort(ids)
	return slices.Compact(ids)
}

func (s *Service) ApplyPartialAbsence(ctx context.Context, pickupExceptionID int64) (rows int, err error) {
	err = s.runWrite(ctx, "apply_partial_absence", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if pickupExceptionID <= 0 {
			return stats, errors.New("student presence: invalid pickup exception ID")
		}
		carePlan, _, err := s.carePlanRules()
		if err != nil {
			return stats, err
		}
		exception, err := carePlan.FindPickupException(txCtx, pickupExceptionID)
		if err != nil || exception == nil || exception.ExcusedFrom == nil {
			return stats, err
		}
		updated, err := s.applyPartialAbsence(txCtx, *exception, false, &stats)
		rows = int(updated)
		return stats, err
	})
	return rows, err
}

func (s *Service) ReleasePartialAbsence(ctx context.Context, pickupExceptionID int64) (rows int, err error) {
	err = s.runWrite(ctx, "release_partial_absence", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if pickupExceptionID <= 0 {
			return stats, errors.New("student presence: invalid pickup exception ID")
		}
		carePlan, locks, err := s.carePlanRules()
		if err != nil {
			return stats, err
		}
		exception, err := lockedPickupException(txCtx, carePlan, locks, pickupExceptionID)
		if err != nil || exception == nil {
			return stats, err
		}
		replacement, err := latestActiveStatusDay(txCtx, carePlan, exception.StudentID, exception.ExceptionDate)
		if err != nil {
			return stats, err
		}
		released, err := s.releasePartialAbsenceRows(txCtx, pickupExceptionID, replacement, &stats)
		rows = int(released)
		return stats, err
	})
	return rows, err
}

// lockedPickupException locks the child's exception day and re-reads the
// excusal under the lock; nil when it does not exist.
func lockedPickupException(ctx context.Context, carePlan ports.CarePlanDirectory, locks ports.CareDayLocker, pickupExceptionID int64) (*ports.PickupException, error) {
	exception, err := carePlan.FindPickupException(ctx, pickupExceptionID)
	if err != nil || exception == nil {
		return nil, err
	}
	if err := locks.LockStudentAndExceptionDay(ctx, exception.StudentID, exception.ExceptionDate); err != nil {
		return nil, err
	}
	return carePlan.FindPickupException(ctx, pickupExceptionID)
}

// releasePartialAbsenceRows moves the participants the excusal owns to their
// next state; participants of finished blocks keep the absence.
func (s *Service) releasePartialAbsenceRows(ctx context.Context, pickupExceptionID int64, replacement *ports.StudentStatusDay, stats *ports.Stats) (int64, error) {
	owned, listStats, err := s.store.ListSessionAttendanceByPickupException(ctx, pickupExceptionID)
	addStats(stats, listStats)
	if err != nil {
		return 0, err
	}
	releases, err := s.statusDayReleases(ctx, owned, replacement, false, stats)
	if err != nil {
		return 0, err
	}
	releaseStats, err := s.store.ReleasePartialAbsence(ctx, pickupExceptionID, releases, time.Now().UTC())
	addStats(stats, releaseStats)
	return releaseStats.Rows, err
}

func (s *Service) ApplyActivePartialAbsencesForInstance(ctx context.Context, instanceID int64, date string) (rows int, err error) {
	err = s.runWrite(ctx, "apply_active_partial_absences_for_instance", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if instanceID <= 0 || date == "" {
			return stats, errors.New("student presence: invalid instance partial absence application")
		}
		roster, carePlan, locks, err := s.attendanceRulePorts()
		if err != nil {
			return stats, err
		}
		exceptions, err := carePlan.ListPickupExceptions(txCtx, ports.PickupExceptionFilter{Date: date})
		if err != nil {
			return stats, err
		}
		cutoffs := excusalCutoffs(exceptions)
		if len(cutoffs) == 0 {
			return stats, nil
		}
		participants, err := lockedPlannedParticipants(txCtx, roster, locks, instanceID, date, sortedKeys(cutoffs))
		if err != nil {
			return stats, err
		}
		applyStats, err := s.store.ApplyPartialAbsences(txCtx, partialAbsencesFor(participants, cutoffs), time.Now().UTC())
		addStats(&stats, applyStats)
		rows = int(applyStats.Rows)
		return stats, err
	})
	return rows, err
}

// excusalCutoffs indexes the excusals of the day that name a cutoff by
// child.
func excusalCutoffs(exceptions []ports.PickupException) map[int64]ports.PickupException {
	cutoffs := make(map[int64]ports.PickupException, len(exceptions))
	for _, exception := range exceptions {
		if exception.ExcusedFrom != nil {
			cutoffs[exception.StudentID] = exception
		}
	}
	return cutoffs
}

// lockedPlannedParticipants locks the exception days of the children and
// lists the planned participants of the block under those locks.
func lockedPlannedParticipants(ctx context.Context, roster ports.PlannedRoster, locks ports.CareDayLocker, instanceID int64, date string, studentIDs []int64) ([]ports.PlannedParticipant, error) {
	for _, studentID := range studentIDs {
		if err := locks.LockExceptionDay(ctx, studentID, date); err != nil {
			return nil, err
		}
	}
	return roster.ListPlannedParticipants(ctx, ports.PlannedRosterFilter{InstanceIDs: []int64{instanceID}, Date: date, ExcludeCancelled: true})
}

// partialAbsencesFor marks the participants whose block starts at or after
// their child's cutoff.
func partialAbsencesFor(participants []ports.PlannedParticipant, cutoffs map[int64]ports.PickupException) []ports.PartialAbsence {
	absences := make([]ports.PartialAbsence, 0, len(participants))
	for _, participant := range participants {
		exception, ok := cutoffs[participant.StudentID]
		if !ok || participant.StartTime < wallClock(*exception.ExcusedFrom) {
			continue
		}
		absences = append(absences, ports.PartialAbsence{ParticipantID: participant.ID, PickupExceptionID: exception.ID})
	}
	return absences
}

func sortedKeys(values map[int64]ports.PickupException) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// MarkExpectedAbsentByActiveGroupIDs closes the expectation of the blocks a
// live group carried: every still expected participant of a running session
// on those groups is marked absent, except the excluded (student, instance)
// pairs the caller spared as not booked.
func (s *Service) MarkExpectedAbsentByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, at time.Time, exclusions []ports.PlannedParticipant) error {
	return s.runWrite(ctx, "mark_expected_absent_by_active_groups", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if !validIDs(activeGroupIDs) || at.IsZero() {
			return stats, errors.New("student presence: invalid session end")
		}
		participants, err := s.groupParticipants(txCtx, activeGroupIDs, "active", &stats)
		if err != nil {
			return stats, err
		}
		excluded := make(map[[2]int64]bool, len(exclusions))
		for _, exclusion := range exclusions {
			excluded[[2]int64{exclusion.StudentID, exclusion.InstanceID}] = true
		}
		ids := make([]int64, 0, len(participants))
		for _, participant := range participants {
			if excluded[[2]int64{participant.StudentID, participant.InstanceID}] {
				continue
			}
			ids = append(ids, participant.ID)
		}
		transitionStats, err := s.store.TransitionParticipants(txCtx, ids, "expected", "absent", &at)
		addStats(&stats, transitionStats)
		return stats, err
	})
}

func (s *Service) CloseOpenCheckoutsByActiveGroupIDs(ctx context.Context, activeGroupIDs []int64, at time.Time) (rows int, err error) {
	err = s.runWrite(ctx, "close_open_checkouts_by_active_groups", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		if !validIDs(activeGroupIDs) || at.IsZero() {
			return stats, errors.New("student presence: invalid session end")
		}
		participants, err := s.groupParticipants(txCtx, activeGroupIDs, "", &stats)
		if err != nil {
			return stats, err
		}
		closeStats, err := s.store.CloseOpenParticipants(txCtx, participantIDs(participants), at)
		addStats(&stats, closeStats)
		rows = int(closeStats.Rows)
		return stats, err
	})
	return rows, err
}

// groupParticipants resolves the planned participants of the instances whose
// sessions run (or ran) on the live groups.
func (s *Service) groupParticipants(ctx context.Context, activeGroupIDs []int64, status string, stats *ports.Stats) ([]ports.PlannedParticipant, error) {
	if len(activeGroupIDs) == 0 {
		return nil, nil
	}
	roster, err := s.plannedRoster()
	if err != nil {
		return nil, err
	}
	sessions, sessionStats, err := s.store.ListActivitySessions(ctx, ports.ActivitySessionFilter{ActiveGroupIDs: activeGroupIDs, Status: status})
	addStats(stats, sessionStats)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	instanceIDs := make([]int64, 0, len(sessions))
	for _, session := range sessions {
		instanceIDs = append(instanceIDs, session.InstanceID)
	}
	return roster.ListPlannedParticipants(ctx, ports.PlannedRosterFilter{InstanceIDs: instanceIDs})
}
