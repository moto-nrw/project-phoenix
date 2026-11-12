package application

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ApplySourcedRosterAttendance resolves and applies the complete occurrence set
// under its student/day locks. Reads and writes are independent of dates and children.
func (s *Service) ApplySourcedRosterAttendance(ctx context.Context, dates map[int64]string) error {
	if len(dates) == 0 {
		return nil
	}
	ids, from, to, err := sourcedAttendanceWindow(dates)
	if err != nil {
		return err
	}
	err = s.runWrite(ctx, "apply_sourced_roster_attendance", func(ctx context.Context) (ports.Stats, error) {
		return s.applySourcedRosterAttendance(ctx, ids, from, to, dates)
	})
	return sessionAttendanceError(err)
}

func sourcedAttendanceWindow(dates map[int64]string) ([]int64, string, string, error) {
	ids := make([]int64, 0, len(dates))
	from, to := "", ""
	for id, date := range dates {
		if id <= 0 {
			return nil, "", "", errors.New("student presence: invalid roster instance")
		}
		if _, err := calendar.ParseDate(date); err != nil {
			return nil, "", "", err
		}
		ids = append(ids, id)
		if from == "" || date < from {
			from = date
		}
		if date > to {
			to = date
		}
	}
	slices.Sort(ids)
	return ids, from, to, nil
}

func (s *Service) applySourcedRosterAttendance(ctx context.Context, ids []int64, from, to string, dates map[int64]string) (ports.Stats, error) {
	stats := ports.Stats{}
	roster, carePlan, _, err := s.attendanceRulePorts()
	if err != nil {
		return stats, err
	}
	participants, err := roster.ListPlannedParticipants(ctx, ports.PlannedRosterFilter{InstanceIDs: ids, ExcludeCancelled: true})
	if err != nil || len(participants) == 0 {
		return stats, err
	}
	days, err := sourcedStudentDays(participants, dates)
	if err != nil {
		return stats, err
	}
	locks, ok := s.careDays.(studentpresence.SessionRosterDayLocker)
	if !ok {
		return stats, errors.New("student presence: roster day batch locks are required")
	}
	if err := locks.LockRosterDays(ctx, days); err != nil {
		return stats, err
	}
	students := uniqueStudentIDs(participants)
	statuses, err := carePlan.ListStudentStatusDays(ctx, ports.StudentStatusDayFilter{StudentIDs: students, From: from, To: to, ActiveOnly: true, LatestOnly: true})
	if err != nil {
		return stats, err
	}
	exceptions, err := carePlan.ListPickupExceptions(ctx, ports.PickupExceptionFilter{StudentIDs: students, From: from, To: to})
	if err != nil {
		return stats, err
	}
	statusRows, partialRows := sourcedAttendanceChanges(participants, statuses, exceptions)
	statusStats, err := s.store.ApplyStatusDayAbsences(ctx, statusRows, time.Now().UTC())
	addStats(&stats, statusStats)
	if err != nil {
		return stats, err
	}
	partialStats, err := s.store.ApplyPartialAbsences(ctx, partialRows, time.Now().UTC())
	addStats(&stats, partialStats)
	return stats, err
}

func sourcedStudentDays(participants []ports.PlannedParticipant, dates map[int64]string) (map[int64][]string, error) {
	days := map[int64][]string{}
	for _, row := range participants {
		if dates[row.InstanceID] != row.Date {
			return nil, errors.New("student presence: roster instance date changed")
		}
		if !slices.Contains(days[row.StudentID], row.Date) {
			days[row.StudentID] = append(days[row.StudentID], row.Date)
		}
	}
	return days, nil
}

type attendanceDay struct {
	studentID int64
	date      string
}

func sourcedAttendanceChanges(participants []ports.PlannedParticipant, statuses []ports.StudentStatusDay, exceptions []ports.PickupException) ([]ports.StatusDayAbsence, []ports.PartialAbsence) {
	byStatus := map[attendanceDay]ports.StudentStatusDay{}
	byException := map[attendanceDay]ports.PickupException{}
	for _, row := range statuses {
		byStatus[attendanceDay{row.StudentID, row.Date}] = row
	}
	for _, row := range exceptions {
		if row.ExcusedFrom != nil {
			byException[attendanceDay{row.StudentID, row.ExceptionDate}] = row
		}
	}
	statusRows := []ports.StatusDayAbsence{}
	partialRows := []ports.PartialAbsence{}
	for _, row := range participants {
		day := attendanceDay{row.StudentID, row.Date}
		if status, ok := byStatus[day]; ok {
			substatus := ""
			if value := statusDaySubstatus(status.Status); value != nil {
				substatus = *value
			}
			statusRows = append(statusRows, ports.StatusDayAbsence{ParticipantID: row.ID, StatusDayID: status.ID, Substatus: substatus})
		}
		if exception, ok := byException[day]; ok && row.StartTime >= wallClock(*exception.ExcusedFrom) {
			partialRows = append(partialRows, ports.PartialAbsence{ParticipantID: row.ID, PickupExceptionID: exception.ID})
		}
	}
	return statusRows, partialRows
}
