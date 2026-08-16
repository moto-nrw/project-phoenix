package enrollment

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// OfferingPickupTimeService materializes Angebots-Gehzeiten
// (care_offerings.pickup_times) into schedule.student_pickup_schedules rows
// (#2290, ADR 0001). The decision service implements it: it already owns the
// offering, link, and pickup-schedule repositories on both write paths
// (admin rollout + approval fan-out).
type OfferingPickupTimeService interface {
	// PreviewOfferingPickupRollout computes what a rollout of the offering's
	// current pickup_times would change, without writing anything. The admin
	// confirmation dialog renders it.
	PreviewOfferingPickupRollout(ctx context.Context, offeringID int64) (*OfferingPickupRolloutPreview, error)
	// RolloutOfferingPickupTimes reconciles every student booked into the
	// offering. Staff-maintained rows are overwritten only where this offering
	// supplies the winning Gehzeit, unless the student is listed in
	// skipStudentIDs — the dialog's per-child opt-out. reviewedBy is the acting
	// account id.
	RolloutOfferingPickupTimes(ctx context.Context, offeringID int64, skipStudentIDs []int64, reviewedBy int64) (*OfferingPickupRolloutResult, error)
	// ReconcileOfferingPickupForStudents recomputes the offering-sourced rows
	// of the given students without touching staff-maintained rows. Booking
	// changes and approvals call it. createdByStaffID may be 0: then only
	// updates and deletes run, inserts are skipped.
	ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64, createdByStaffID int64) error
	// ResetStudentPickupDayToOffering restores one student weekday to the
	// Angebots-Gehzeit. Without an offering time for that weekday the row is
	// deleted; the returned row is nil then.
	ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, weekday int, reviewedBy int64) (*scheduleModels.StudentPickupSchedule, error)
}

// OfferingPickupConflict is one staff-maintained row a rollout would
// overwrite: the dialog lists these per child with an opt-out.
type OfferingPickupConflict struct {
	StudentID   int64  `json:"student_id"`
	StudentName string `json:"student_name"`
	Weekday     int    `json:"weekday"`
	CurrentTime string `json:"current_time"`
	NewTime     string `json:"new_time"`
}

// OfferingPickupRolloutPreview summarizes a dry-run rollout.
type OfferingPickupRolloutPreview struct {
	AffectedStudents int                      `json:"affected_students"`
	NewRows          int                      `json:"new_rows"`
	UpdatedRows      int                      `json:"updated_rows"`
	RemovedRows      int                      `json:"removed_rows"`
	Conflicts        []OfferingPickupConflict `json:"conflicts"`
}

// OfferingPickupRolloutResult counts what an executed rollout wrote.
type OfferingPickupRolloutResult struct {
	CreatedRows     int `json:"created_rows"`
	UpdatedRows     int `json:"updated_rows"`
	DeletedRows     int `json:"deleted_rows"`
	SkippedStudents int `json:"skipped_students"`
}

// offeringPickupDesired is the wanted Gehzeit of one student weekday: the
// latest time across the day's booked offerings, with the offering that
// contributed it.
type offeringPickupDesired struct {
	hhmm       string
	offeringID int64
}

func (s *decisionService) PreviewOfferingPickupRollout(ctx context.Context, offeringID int64) (*OfferingPickupRolloutPreview, error) {
	if !s.hasOfferingPickupDependencies() {
		return nil, fmt.Errorf("offering pickup rollout: repositories not configured")
	}
	preview := &OfferingPickupRolloutPreview{Conflicts: []OfferingPickupConflict{}}
	active, err := s.offeringPickupRolloutActiveToday(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if !active {
		return preview, nil
	}
	studentIDs, err := s.offeringPickupAffectedStudents(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if len(studentIDs) == 0 {
		return preview, nil
	}
	desired, err := s.desiredOfferingPickupTimes(ctx, studentIDs, timezone.TodayDate())
	if err != nil {
		return nil, err
	}
	existing, err := s.existingPickupRowsByStudent(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	names, err := s.studentDisplayNames(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	affected := map[int64]bool{}
	for _, studentID := range studentIDs {
		for weekday := scheduleModels.WeekdayMonday; weekday <= scheduleModels.WeekdayFriday; weekday++ {
			want, hasWant := desired[studentID][weekday]
			row := existing[studentID][weekday]
			switch {
			case hasWant && row == nil:
				preview.NewRows++
				affected[studentID] = true
			case hasWant && row.Source == scheduleModels.PickupScheduleSourceCareOffering:
				affected[studentID] = true
				if !pickupRowMatchesDesired(row, want) {
					preview.UpdatedRows++
				}
			case hasWant && row.Source == scheduleModels.PickupScheduleSourceStaff:
				if want.offeringID != offeringID {
					continue
				}
				affected[studentID] = true
				if row.PickupTime.Format("15:04") != want.hhmm {
					preview.Conflicts = append(preview.Conflicts, OfferingPickupConflict{
						StudentID:   studentID,
						StudentName: names[studentID],
						Weekday:     weekday,
						CurrentTime: row.PickupTime.Format("15:04"),
						NewTime:     want.hhmm,
					})
				}
			case !hasWant && row != nil && row.Source == scheduleModels.PickupScheduleSourceCareOffering:
				preview.RemovedRows++
				affected[studentID] = true
			}
		}
	}
	preview.AffectedStudents = len(affected)
	sort.Slice(preview.Conflicts, func(i, j int) bool {
		if preview.Conflicts[i].StudentName != preview.Conflicts[j].StudentName {
			return preview.Conflicts[i].StudentName < preview.Conflicts[j].StudentName
		}
		return preview.Conflicts[i].Weekday < preview.Conflicts[j].Weekday
	})
	return preview, nil
}

func (s *decisionService) RolloutOfferingPickupTimes(ctx context.Context, offeringID int64, skipStudentIDs []int64, reviewedBy int64) (*OfferingPickupRolloutResult, error) {
	if !s.hasOfferingPickupDependencies() {
		return nil, fmt.Errorf("offering pickup rollout: repositories not configured")
	}
	result := &OfferingPickupRolloutResult{}
	active, err := s.offeringPickupRolloutActiveToday(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if !active {
		return result, nil
	}
	studentIDs, err := s.offeringPickupAffectedStudents(ctx, offeringID)
	if err != nil {
		return nil, err
	}
	if len(studentIDs) == 0 {
		return result, nil
	}
	createdBy, err := s.resolveReviewerStaffID(ctx, reviewedBy)
	if err != nil {
		return nil, fmt.Errorf("rollout requires an acting staff member: %w", err)
	}
	skip := make(map[int64]bool, len(skipStudentIDs))
	for _, id := range skipStudentIDs {
		skip[id] = true
	}
	stats, err := s.reconcileOfferingPickupRows(ctx, studentIDs, offeringPickupReconcileOptions{
		overwriteStaff:           true,
		overwriteStaffOfferingID: offeringID,
		skipStudents:             skip,
		createdBy:                createdBy,
		onDate:                   timezone.TodayDate(),
	})
	if err != nil {
		return nil, err
	}
	result.CreatedRows = stats.created
	result.UpdatedRows = stats.updated
	result.DeletedRows = stats.deleted
	result.SkippedStudents = stats.skippedStudents
	s.Logger.Info("offering pickup rollout executed",
		slog.Int64("care_offering_id", offeringID),
		slog.Int("created_rows", result.CreatedRows),
		slog.Int("updated_rows", result.UpdatedRows),
		slog.Int("deleted_rows", result.DeletedRows),
		slog.Int("skipped_students", result.SkippedStudents),
	)
	return result, nil
}

func (s *decisionService) offeringPickupRolloutActiveToday(ctx context.Context, offeringID int64) (bool, error) {
	if offeringID <= 0 {
		return false, fmt.Errorf("%w: offering id is required", ErrCareOfferingInvalid)
	}
	offering, err := s.CareOfferingRepo.FindByID(ctx, offeringID)
	if err != nil || offering == nil {
		return false, ErrCareOfferingNotFound
	}
	phase, err := s.PhaseRepo.FindByID(ctx, offering.PhaseID)
	if err != nil {
		return false, fmt.Errorf("load offering phase for pickup rollout: %w", err)
	}
	if phase == nil {
		return false, fmt.Errorf("load offering phase for pickup rollout: phase not found")
	}
	today := timezone.TodayDate()
	return !phase.ServiceStartDate.After(today) && !phase.ServiceEndDate.Before(today), nil
}

func (s *decisionService) ReconcileOfferingPickupForStudents(ctx context.Context, studentIDs []int64, createdByStaffID int64) error {
	if len(studentIDs) == 0 || !s.hasOfferingPickupDependencies() {
		return nil
	}
	_, err := s.reconcileOfferingPickupRows(ctx, studentIDs, offeringPickupReconcileOptions{
		overwriteStaff: false,
		createdBy:      createdByStaffID,
	})
	return err
}

func (s *decisionService) ResetStudentPickupDayToOffering(ctx context.Context, studentID int64, weekday int, reviewedBy int64) (*scheduleModels.StudentPickupSchedule, error) {
	if !s.hasOfferingPickupDependencies() {
		return nil, fmt.Errorf("offering pickup reset: repositories not configured")
	}
	if weekday < scheduleModels.WeekdayMonday || weekday > scheduleModels.WeekdayFriday {
		return nil, fmt.Errorf("weekday must be between 1 (Monday) and 5 (Friday)")
	}
	desired, err := s.desiredOfferingPickupTimes(ctx, []int64{studentID}, timezone.TodayDate())
	if err != nil {
		return nil, err
	}
	existingRows, err := s.PickupScheduleRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("load pickup schedules: %w", err)
	}
	var existing *scheduleModels.StudentPickupSchedule
	for _, row := range existingRows {
		if row.Weekday == weekday {
			existing = row
			break
		}
	}
	want, hasWant := desired[studentID][weekday]
	if !hasWant {
		if existing != nil {
			if err := s.PickupScheduleRepo.Delete(ctx, existing.ID); err != nil {
				return nil, fmt.Errorf("delete pickup schedule: %w", err)
			}
		}
		return nil, nil
	}
	createdBy, err := s.resolveReviewerStaffID(ctx, reviewedBy)
	if err != nil {
		if existing == nil {
			return nil, fmt.Errorf("reset requires an acting staff member: %w", err)
		}
		createdBy = existing.CreatedBy
	}
	row := &scheduleModels.StudentPickupSchedule{
		StudentID:      studentID,
		Weekday:        weekday,
		PickupTime:     mustParseWallClock(want.hhmm),
		CreatedBy:      createdBy,
		Source:         scheduleModels.PickupScheduleSourceCareOffering,
		CareOfferingID: &want.offeringID,
	}
	if existing != nil {
		row.Notes = existing.Notes
	}
	if err := s.PickupScheduleRepo.UpsertSchedule(ctx, row); err != nil {
		return nil, fmt.Errorf("upsert pickup schedule: %w", err)
	}
	return row, nil
}

type offeringPickupReconcileOptions struct {
	overwriteStaff           bool
	overwriteStaffOfferingID int64
	skipStudents             map[int64]bool
	createdBy                int64
	onDate                   timezone.Date
}

type offeringPickupReconcileStats struct {
	created, updated, deleted int
	skippedStudents           int
	changedStudents           map[int64]bool
}

func (s *offeringPickupReconcileStats) markChanged(studentID int64) {
	if s.changedStudents == nil {
		s.changedStudents = make(map[int64]bool)
	}
	s.changedStudents[studentID] = true
}

func (s *offeringPickupReconcileStats) changedStudentIDs() []int64 {
	ids := make([]int64, 0, len(s.changedStudents))
	for id := range s.changedStudents {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

// reconcileOfferingPickupRows is the core reconciler: it aligns the
// offering-sourced pickup rows of the given students with the desired state
// derived from their currently valid approved offerings. Staff rows are only
// touched when overwriteStaff is set and the student is not skipped.
func (s *decisionService) reconcileOfferingPickupRows(ctx context.Context, studentIDs []int64, opts offeringPickupReconcileOptions) (*offeringPickupReconcileStats, error) {
	if opts.onDate.IsZero() {
		opts.onDate = timezone.TodayDate()
	}
	desired, err := s.desiredOfferingPickupTimes(ctx, studentIDs, opts.onDate)
	if err != nil {
		return nil, err
	}
	existing, err := s.existingPickupRowsByStudent(ctx, studentIDs)
	if err != nil {
		return nil, err
	}
	stats := &offeringPickupReconcileStats{}
	for _, studentID := range studentIDs {
		if opts.skipStudents[studentID] {
			stats.skippedStudents++
			continue
		}
		for weekday := scheduleModels.WeekdayMonday; weekday <= scheduleModels.WeekdayFriday; weekday++ {
			want, hasWant := desired[studentID][weekday]
			row := existing[studentID][weekday]
			switch {
			case hasWant && row == nil:
				if err := s.upsertOfferingPickupRow(ctx, studentID, weekday, want, opts.createdBy, nil); err != nil {
					return nil, err
				}
				stats.created++
				stats.markChanged(studentID)
			case hasWant && row.Source == scheduleModels.PickupScheduleSourceCareOffering:
				if pickupRowMatchesDesired(row, want) {
					continue
				}
				if err := s.upsertOfferingPickupRow(ctx, studentID, weekday, want, pickupRowAuthor(row, opts.createdBy), row); err != nil {
					return nil, err
				}
				stats.updated++
				stats.markChanged(studentID)
			case hasWant && row.Source == scheduleModels.PickupScheduleSourceStaff:
				if !opts.overwriteStaff || want.offeringID != opts.overwriteStaffOfferingID || row.PickupTime.Format("15:04") == want.hhmm {
					continue
				}
				if err := s.upsertOfferingPickupRow(ctx, studentID, weekday, want, pickupRowAuthor(row, opts.createdBy), row); err != nil {
					return nil, err
				}
				stats.updated++
				stats.markChanged(studentID)
			case !hasWant && row != nil && row.Source == scheduleModels.PickupScheduleSourceCareOffering:
				if err := s.PickupScheduleRepo.Delete(ctx, row.ID); err != nil {
					return nil, fmt.Errorf("delete stale offering pickup row: %w", err)
				}
				stats.deleted++
				stats.markChanged(studentID)
			}
		}
	}
	s.deferOfferingPickupBroadcasts(ctx, stats.changedStudentIDs())
	return stats, nil
}

func (s *decisionService) deferOfferingPickupBroadcasts(ctx context.Context, studentIDs []int64) {
	if len(studentIDs) == 0 || (s.Broadcaster == nil && s.PickupGuardianNotifier == nil) {
		return
	}
	tenantID := tenant.FromContext(ctx)
	tenant.RegisterAfterCommit(ctx, func() {
		if tenantID <= 0 {
			return
		}
		if s.Broadcaster != nil {
			source := "offering_pickup_reconcile"
			event := realtime.NewEvent(realtime.EventPickupScheduleChanged, "", realtime.EventData{Source: &source})
			if err := s.Broadcaster.BroadcastToTenant(tenantID, event); err != nil {
				s.Logger.Warn("offering pickup reconcile: failed to broadcast schedule change",
					slog.Int64("tenant_id", tenantID),
					slog.String("error", err.Error()),
				)
			}
		}
		if s.PickupGuardianNotifier != nil {
			for _, studentID := range studentIDs {
				s.PickupGuardianNotifier.BroadcastChildUpdateToGuardians(tenantID, studentID)
			}
		}
	})
}

func (s *decisionService) upsertOfferingPickupRow(ctx context.Context, studentID int64, weekday int, want offeringPickupDesired, createdBy int64, previous *scheduleModels.StudentPickupSchedule) error {
	row := &scheduleModels.StudentPickupSchedule{
		StudentID:      studentID,
		Weekday:        weekday,
		PickupTime:     mustParseWallClock(want.hhmm),
		CreatedBy:      createdBy,
		Source:         scheduleModels.PickupScheduleSourceCareOffering,
		CareOfferingID: &want.offeringID,
	}
	if previous != nil {
		row.Notes = previous.Notes
	}
	if err := s.PickupScheduleRepo.UpsertSchedule(ctx, row); err != nil {
		return fmt.Errorf("upsert offering pickup row: %w", err)
	}
	return nil
}

// offeringPickupAffectedStudents lists the students holding a current or
// scheduled approved booking of the offering — the rollout scope.
func (s *decisionService) offeringPickupAffectedStudents(ctx context.Context, offeringID int64) ([]int64, error) {
	if offeringID <= 0 {
		return nil, fmt.Errorf("%w: offering id is required", ErrCareOfferingInvalid)
	}
	if _, err := s.CareOfferingRepo.FindByID(ctx, offeringID); err != nil {
		return nil, ErrCareOfferingNotFound
	}
	children, err := s.RequestChildOfferingRepo.ListApprovedChildrenByCareOfferingIDs(ctx, []int64{offeringID}, timezone.TodayDate())
	if err != nil {
		return nil, fmt.Errorf("list approved offering children: %w", err)
	}
	seen := make(map[int64]bool, len(children))
	studentIDs := make([]int64, 0, len(children))
	for _, child := range children {
		if child == nil || child.StudentID <= 0 || seen[child.StudentID] {
			continue
		}
		seen[child.StudentID] = true
		studentIDs = append(studentIDs, child.StudentID)
	}
	slices.Sort(studentIDs)
	return studentIDs, nil
}

// desiredOfferingPickupTimes computes each student's wanted Gehzeit per
// weekday: the latest pickup_times entry across the offerings the student
// holds on the reference date, on the days they actually booked (falling back to the
// offering's fixed available_days when the link stores none — the same
// fallback careUsageRow applies).
func (s *decisionService) desiredOfferingPickupTimes(ctx context.Context, studentIDs []int64, onDate timezone.Date) (map[int64]map[int]offeringPickupDesired, error) {
	out := make(map[int64]map[int]offeringPickupDesired, len(studentIDs))
	links, err := s.RequestChildOfferingRepo.ListApprovedByStudentIDsOnDate(ctx, studentIDs, onDate)
	if err != nil {
		return nil, fmt.Errorf("list approved offering links: %w", err)
	}
	if len(links) == 0 {
		return out, nil
	}
	offeringIDs := make([]int64, 0, len(links))
	seen := map[int64]bool{}
	for _, link := range links {
		if link.Link != nil && !seen[link.Link.CareOfferingID] {
			seen[link.Link.CareOfferingID] = true
			offeringIDs = append(offeringIDs, link.Link.CareOfferingID)
		}
	}
	offerings, err := s.CareOfferingRepo.ListByIDs(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("load offerings: %w", err)
	}
	offeringByID := make(map[int64]*enrollmentModels.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[offering.ID] = offering
	}
	for _, entry := range links {
		if entry.Link == nil {
			continue
		}
		offering := offeringByID[entry.Link.CareOfferingID]
		if offering == nil || len(offering.PickupTimes) == 0 {
			continue
		}
		days := entry.Link.SelectedDays
		if len(days) == 0 && offering.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeFixed {
			days = offering.AvailableDays
		}
		for _, day := range days {
			hhmm := offering.PickupTimes[strings.ToLower(strings.TrimSpace(day))]
			if hhmm == "" {
				continue
			}
			weekday, ok := enrollmentModels.CanonicalDayToISOWeekday(day)
			if !ok || weekday > scheduleModels.WeekdayFriday {
				continue
			}
			if out[entry.StudentID] == nil {
				out[entry.StudentID] = make(map[int]offeringPickupDesired)
			}
			// Latest wins: HH:MM strings are zero-padded (validated), so
			// lexicographic comparison is chronological.
			if current, ok := out[entry.StudentID][weekday]; !ok || hhmm > current.hhmm {
				out[entry.StudentID][weekday] = offeringPickupDesired{hhmm: hhmm, offeringID: offering.ID}
			}
		}
	}
	return out, nil
}

func (s *decisionService) existingPickupRowsByStudent(ctx context.Context, studentIDs []int64) (map[int64]map[int]*scheduleModels.StudentPickupSchedule, error) {
	rows, err := s.PickupScheduleRepo.FindByStudentIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("load pickup schedules: %w", err)
	}
	out := make(map[int64]map[int]*scheduleModels.StudentPickupSchedule, len(studentIDs))
	for _, row := range rows {
		if out[row.StudentID] == nil {
			out[row.StudentID] = make(map[int]*scheduleModels.StudentPickupSchedule)
		}
		out[row.StudentID][row.Weekday] = row
	}
	return out, nil
}

func (s *decisionService) studentDisplayNames(ctx context.Context, studentIDs []int64) (map[int64]string, error) {
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("load students: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, fmt.Errorf("load persons: %w", err)
	}
	out := make(map[int64]string, len(students))
	for id, student := range students {
		if person := persons[student.PersonID]; person != nil {
			out[id] = strings.TrimSpace(person.FirstName + " " + person.LastName)
		}
	}
	return out, nil
}

func pickupRowMatchesDesired(row *scheduleModels.StudentPickupSchedule, want offeringPickupDesired) bool {
	if row.PickupTime.Format("15:04") != want.hhmm {
		return false
	}
	return row.CareOfferingID != nil && *row.CareOfferingID == want.offeringID
}

func pickupRowAuthor(row *scheduleModels.StudentPickupSchedule, fallback int64) int64 {
	if row != nil && row.CreatedBy > 0 {
		return row.CreatedBy
	}
	return fallback
}

// mustParseWallClock converts a validated HH:MM string into the wall-clock
// time.Time shape TIME columns use. The input is validated at the model
// boundary; a parse failure here would be a programming error, so it maps to
// a zero time the row validation rejects loudly.
func mustParseWallClock(hhmm string) time.Time {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}
	}
	return timezone.WallClock(t)
}

// materializeOfferingPickupAfterApproval writes the approved child's
// Angebots-Gehzeiten (#2290). Runs after the status flip inside the approval
// transaction. Automatically approved rows have no acting staff author;
// their source and care_offering_id retain the relevant provenance.
func (s *decisionService) materializeOfferingPickupAfterApproval(ctx context.Context, child *enrollmentModels.RequestChild, reviewedBy int64, onDate timezone.Date) error {
	if !s.hasOfferingPickupDependencies() {
		return nil
	}
	studentID := int64(0)
	switch {
	case child == nil:
		return nil
	case child.CreatedStudentID != nil && *child.CreatedStudentID > 0:
		studentID = *child.CreatedStudentID
	case child.MatchedStudentID != nil && *child.MatchedStudentID > 0:
		studentID = *child.MatchedStudentID
	default:
		return nil
	}
	createdBy, err := s.resolveReviewerStaffID(ctx, reviewedBy)
	if err != nil {
		s.Logger.Debug("offering pickup materialization: no acting staff; automatic rows have no creator",
			slog.Int64("reviewed_by", reviewedBy),
			slog.String("error", err.Error()),
		)
		createdBy = 0
	}
	_, err = s.reconcileOfferingPickupRows(ctx, []int64{studentID}, offeringPickupReconcileOptions{
		createdBy: createdBy,
		onDate:    onDate,
	})
	return err
}

// ReconcileOfferingPickupForStudentsByAccount is the account-facing form of
// the reconcile: it resolves the acting account to its staff row for new-row
// authorship and degrades to a no-insert reconcile when the account has no
// staff (mirroring the schedule dispatch tolerance).
func (s *decisionService) ReconcileOfferingPickupForStudentsByAccount(ctx context.Context, studentIDs []int64, accountID int64) error {
	createdBy, err := s.resolveReviewerStaffID(ctx, accountID)
	if err != nil {
		s.Logger.Warn("offering pickup reconcile: account has no staff, inserts skipped",
			slog.Int64("account_id", accountID),
			slog.String("error", err.Error()),
		)
		createdBy = 0
	}
	return s.ReconcileOfferingPickupForStudents(ctx, studentIDs, createdBy)
}

// hasOfferingPickupDependencies guards the reconciler against partial
// wirings. Production always injects the full repository set (factory.go
// wires one decision service everywhere, including the rollover worker);
// only slim test wirings omit the pickup repositories. A missing repo means
// "this wiring does not materialize Gehzeiten", not an error.
func (s *decisionService) hasOfferingPickupDependencies() bool {
	return s.PickupScheduleRepo != nil &&
		s.RequestChildOfferingRepo != nil &&
		s.CareOfferingRepo != nil
}
