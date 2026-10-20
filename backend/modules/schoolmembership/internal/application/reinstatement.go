package application

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership/internal/domain"
)

// ReinstateStaff continues a retired staff membership, the Wiederaufnahme of
// ADR 0021. Membership and employment profile come back under the same ID, so
// every record keyed by it belongs to the resumed employment again.
func (s *Service) ReinstateStaff(ctx context.Context, id int64) (result schoolmembership.Staff, err error) {
	err = s.runStaffWrite(ctx, "reinstate_staff", func(txCtx context.Context, stats *domain.OperationStats) error {
		retired, err := s.lockRetiredStaff(txCtx, id, stats)
		if err != nil {
			return err
		}
		employment := staffEmployment(retired)
		number := employment.PersonnelNumber
		// The personnel-number trigger fires on profile writes, not on the
		// membership's deleted_at. Park the number while the membership is
		// retired and write it back once it is live, so the trigger checks it.
		if number != nil {
			employment.PersonnelNumber = nil
			if err := s.saveEmploymentProfile(txCtx, employment, stats); err != nil {
				return err
			}
		}
		var reinstateStats domain.OperationStats
		result, reinstateStats, err = s.store.ReinstateStaff(txCtx, id)
		stats.Add(reinstateStats)
		if err != nil {
			return err
		}
		if number != nil {
			if employment, err = s.restorePersonnelNumber(txCtx, employment, number, stats); err != nil {
				return err
			}
		}
		result = staffWithEmployment(result, employment)
		return nil
	})
	return result, err
}

// lockRetiredStaff locks a retired membership and composes its employment.
func (s *Service) lockRetiredStaff(ctx context.Context, id int64, stats *domain.OperationStats) (schoolmembership.Staff, error) {
	retired, found, queryStats, err := s.store.FindStaff(ctx, id, "UPDATE", true)
	stats.Add(queryStats)
	if err != nil {
		return schoolmembership.Staff{}, err
	}
	if !found || !retired.IsDeleted() {
		return schoolmembership.Staff{}, schoolmembership.ErrStaffNotFound
	}
	return s.withEmployment(ctx, retired, stats)
}

func (s *Service) saveEmploymentProfile(ctx context.Context, employment domain.StaffEmployment, stats *domain.OperationStats) error {
	stats.Queries++
	return s.employment.SaveStaffEmployment(ctx, employment)
}

// restorePersonnelNumber writes the parked number back to the live
// membership. A number somebody else holds by now stays cleared instead of
// failing the reinstatement (ADR 0021, decision 6); the savepoint keeps the
// refused write from aborting the caller's transaction.
func (s *Service) restorePersonnelNumber(ctx context.Context, employment domain.StaffEmployment, number *string, stats *domain.OperationStats) (domain.StaffEmployment, error) {
	restored := employment
	restored.PersonnelNumber = number
	err := s.tx.RunSavepoint(ctx, func(spCtx context.Context) error {
		return s.saveEmploymentProfile(spCtx, restored, stats)
	})
	// The Workforce port reports the conflict in the public vocabulary.
	if errors.Is(err, schoolmembership.ErrPersonnelNumberConflict) {
		return employment, nil
	}
	if err != nil {
		return domain.StaffEmployment{}, err
	}
	return restored, nil
}

// ReinstateTeacher revives a retired caregiver profile of a live staff
// membership instead of growing a second one next to it (ADR 0021).
func (s *Service) ReinstateTeacher(ctx context.Context, id int64) (result schoolmembership.Teacher, err error) {
	err = s.runWrite(ctx, "reinstate_teacher", func(txCtx context.Context, stats *domain.OperationStats) error {
		retired, queryStats, err := s.store.ListTeachers(txCtx, schoolmembership.TeacherFilter{IDs: []int64{id}, IncludeDeleted: true})
		stats.Add(queryStats)
		if err != nil {
			return err
		}
		if len(retired) != 1 || !retired[0].IsDeleted() {
			return schoolmembership.ErrTeacherNotFound
		}
		// Staff before teacher, the order every teacher write locks in.
		if err := s.requireStaffLocked(txCtx, retired[0].StaffID, stats); err != nil {
			return err
		}
		var reinstateStats domain.OperationStats
		result, reinstateStats, err = s.store.ReinstateTeacher(txCtx, id)
		stats.Add(reinstateStats)
		return err
	})
	return result, err
}
