package application

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
	"github.com/moto-nrw/project-phoenix/tenant"
)

func (s *Service) PreviewStaffOffboarding(ctx context.Context, staffID int64, from string) (result workforce.OffboardingPreview, err error) {
	err = s.run(ctx, "preview_staff_offboarding", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			_, result, err = s.offboardingSnapshot(txCtx, staffID, from, stats)
			return err
		})
	})
	return result, MapError(err)
}

func (s *Service) ExecuteStaffOffboarding(ctx context.Context, staffID, actorID int64, from, revision string, appendDeletion func(context.Context, workforce.StaffAbsence, int64) error) (result workforce.OffboardingCounts, err error) {
	err = s.run(ctx, "execute_staff_offboarding", func(ctx context.Context, stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			snapshot, preview, err := s.offboardingSnapshot(txCtx, staffID, from, stats)
			if err != nil {
				return err
			}
			if preview.Counts == (workforce.OffboardingCounts{}) {
				return nil
			}
			removal := offboardingRemoval{
				staffID: staffID, actorID: actorID, from: from,
				snapshot: snapshot, appendDeletion: appendDeletion,
			}
			return s.removeOffboardedRows(txCtx, removal, preview, revision, stats, &result)
		})
	})
	if err != nil {
		return workforce.OffboardingCounts{}, MapError(err)
	}
	return result, nil
}

// offboardingRemoval is the locked snapshot an offboarding removes, with the
// callback that archives each absence first.
type offboardingRemoval struct {
	staffID, actorID int64
	from             string
	snapshot         domain.OffboardingSnapshot
	appendDeletion   func(context.Context, workforce.StaffAbsence, int64) error
}

// removeOffboardedRows checks the preview revision, archives the absences and
// removes the snapshot's rows, recording the counts in result. A count that
// differs from the preview is a conflict.
func (s *Service) removeOffboardedRows(
	ctx context.Context,
	removal offboardingRemoval,
	preview workforce.OffboardingPreview,
	revision string,
	stats *domain.OperationStats,
	result *workforce.OffboardingCounts,
) error {
	if err := checkOffboardingPreview(preview, revision); err != nil {
		return err
	}
	for _, absence := range removal.snapshot.Absences {
		if err := removal.appendDeletion(ctx, AbsenceToPublic(absence), removal.actorID); err != nil {
			return fmt.Errorf("archive absence before offboarding: %w", err)
		}
	}
	if err := s.deleteOffboardedRows(ctx, removal.staffID, removal.from, removal.snapshot.Series, stats, result); err != nil {
		return err
	}
	if *result != preview.Counts {
		return workforce.ErrOffboardingConflict
	}
	return nil
}

// checkOffboardingPreview rejects an execution whose preview revision is stale
// or whose rows are still in use.
func checkOffboardingPreview(preview workforce.OffboardingPreview, revision string) error {
	if preview.Revision != revision {
		return workforce.ErrOffboardingConflict
	}
	if preview.Blocked {
		return workforce.ErrOffboardingInUse
	}
	return nil
}

// deleteOffboardedRows removes the staff member's substitutions, upcoming
// shifts and non-historical absences from from on and caps their series,
// recording the affected row counts in result.
func (s *Service) deleteOffboardedRows(ctx context.Context, staffID int64, from string, series []domain.StaffShiftSeries, stats *domain.OperationStats, result *workforce.OffboardingCounts) error {
	var changed domain.OperationStats
	var err error
	result.Substitutions, changed, err = s.store.DeleteGroupSubstitutionsForStaff(ctx, staffID, from)
	stats.Add(changed)
	if err != nil {
		return err
	}
	result.Shifts, changed, err = s.store.DeleteUpcomingStaffShifts(ctx, staffID, from)
	stats.Add(changed)
	if err != nil {
		return err
	}
	for _, row := range series {
		changed, err = s.store.CapStaffShiftSeries(ctx, row.ID, from)
		stats.Add(changed)
		if err != nil {
			return err
		}
		result.Series += changed.Rows
	}
	result.Absences, changed, err = s.store.DeleteNonHistoricalStaffAbsences(ctx, staffID, from)
	stats.Add(changed)
	return err
}

func (s *Service) LockStaffOffboarding(ctx context.Context, staffID int64) error {
	if _, ok := tenant.TransactionFromContext(ctx); !ok {
		return errors.New("workforce: offboarding lock requires a transaction")
	}
	if err := s.LockStaffAbsenceWrites(ctx, staffID); err != nil {
		return err
	}
	return MapError(s.transaction.LockStaffShifts(ctx, staffID))
}

func (s *Service) offboardingSnapshot(ctx context.Context, staffID int64, from string, stats *domain.OperationStats) (domain.OffboardingSnapshot, workforce.OffboardingPreview, error) {
	if err := s.LockStaffOffboarding(ctx, staffID); err != nil {
		return domain.OffboardingSnapshot{}, workforce.OffboardingPreview{}, err
	}
	snapshot, readStats, err := s.store.PreviewStaffOffboarding(ctx, staffID, from)
	stats.Add(readStats)
	if err != nil {
		return snapshot, workforce.OffboardingPreview{}, err
	}
	preview := workforce.OffboardingPreview{Counts: workforce.OffboardingCounts{
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
