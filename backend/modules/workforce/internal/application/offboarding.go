package application

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) PreviewStaffOffboarding(ctx context.Context, staffID int64, from string) (result domain.OffboardingPreview, err error) {
	err = s.run("preview_staff_offboarding", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			_, result, err = s.offboardingSnapshot(txCtx, staffID, from, stats)
			return err
		})
	})
	return result, err
}

func (s *Service) ExecuteStaffOffboarding(ctx context.Context, staffID, actorID int64, from, revision string, appendDeletion func(context.Context, domain.StaffAbsence, int64) error) (result domain.OffboardingCounts, err error) {
	err = s.run("execute_staff_offboarding", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			snapshot, preview, err := s.offboardingSnapshot(txCtx, staffID, from, stats)
			if err != nil {
				return err
			}
			if preview.Counts == (domain.OffboardingCounts{}) {
				return nil
			}
			if preview.Revision != revision {
				return domain.ErrOffboardingConflict
			}
			if preview.Blocked {
				return domain.ErrOffboardingInUse
			}
			for _, absence := range snapshot.Absences {
				if err := appendDeletion(txCtx, absence, actorID); err != nil {
					return fmt.Errorf("archive absence before offboarding: %w", err)
				}
			}
			var changed domain.OperationStats
			result.Substitutions, changed, err = s.store.DeleteGroupSubstitutionsForStaff(txCtx, staffID, from)
			stats.Add(changed)
			if err != nil {
				return err
			}
			result.Shifts, changed, err = s.store.DeleteUpcomingStaffShifts(txCtx, staffID, from)
			stats.Add(changed)
			if err != nil {
				return err
			}
			for _, series := range snapshot.Series {
				changed, err = s.store.CapStaffShiftSeries(txCtx, series.ID, from)
				stats.Add(changed)
				if err != nil {
					return err
				}
				result.Series += changed.Rows
			}
			result.Absences, changed, err = s.store.DeleteNonHistoricalStaffAbsences(txCtx, staffID, from)
			stats.Add(changed)
			if err != nil {
				return err
			}
			if result != preview.Counts {
				return domain.ErrOffboardingConflict
			}
			return nil
		})
	})
	if err != nil {
		return domain.OffboardingCounts{}, err
	}
	return result, nil
}

func (s *Service) LockStaffOffboarding(ctx context.Context, staffID int64) error {
	if err := s.LockStaffAbsenceWrites(ctx, staffID); err != nil {
		return err
	}
	return s.transaction.LockStaffShifts(ctx, staffID)
}

func (s *Service) offboardingSnapshot(ctx context.Context, staffID int64, from string, stats *domain.OperationStats) (domain.OffboardingSnapshot, domain.OffboardingPreview, error) {
	if err := s.LockStaffOffboarding(ctx, staffID); err != nil {
		return domain.OffboardingSnapshot{}, domain.OffboardingPreview{}, err
	}
	snapshot, readStats, err := s.store.PreviewStaffOffboarding(ctx, staffID, from)
	stats.Add(readStats)
	if err != nil {
		return snapshot, domain.OffboardingPreview{}, err
	}
	preview := domain.OffboardingPreview{Counts: domain.OffboardingCounts{
		Absences: int64(len(snapshot.Absences)), Shifts: int64(len(snapshot.Shifts)),
		Series: int64(len(snapshot.Series)), Substitutions: int64(len(snapshot.Substitutions)),
	}}
	// Revisions contain only row identities and versions, not absence notes.
	type rowVersion struct {
		ID        int64
		UpdatedAt time.Time
	}
	versions := struct {
		StaffID                                 int64
		From                                    string
		Absences, Shifts, Series, Substitutions []rowVersion
	}{StaffID: staffID, From: from}
	for _, row := range snapshot.Absences {
		versions.Absences = append(versions.Absences, rowVersion{row.ID, row.UpdatedAt})
	}
	for _, row := range snapshot.Shifts {
		versions.Shifts = append(versions.Shifts, rowVersion{row.ID, row.UpdatedAt})
	}
	for _, row := range snapshot.Series {
		versions.Series = append(versions.Series, rowVersion{row.ID, row.UpdatedAt})
	}
	for _, row := range snapshot.Substitutions {
		versions.Substitutions = append(versions.Substitutions, rowVersion{row.ID, row.UpdatedAt})
		if row.TargetType == domain.GroupSubstitutionTypeGroupHandover && row.SubstituteStaffID == staffID {
			preview.Blocked = true
		}
	}
	payload, err := json.Marshal(versions)
	preview.Revision = string(payload)
	return snapshot, preview, err
}
