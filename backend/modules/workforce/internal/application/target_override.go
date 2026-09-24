package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// StaffTargetOverrides loads the Sonderarbeitszeiten of the queried staff
// members. With query.Days it also expands them into the days they set a
// target on: Monday to Friday inside the window, never a statutory holiday.
// Every Soll reader uses that expansion, so the rule lives in one place.
func (s *Service) StaffTargetOverrides(ctx context.Context, query domain.TargetOverrideQuery) (rows []domain.StaffTargetOverride, days domain.TargetOverrideDays, err error) {
	if validationErr := validateTargetOverrideWindow(query); validationErr != nil {
		return nil, nil, validationErr
	}
	if len(query.StaffIDs) == 0 {
		return nil, domain.TargetOverrideDays{}, nil
	}
	err = s.run("staff_target_overrides", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		rows, queryStats, err = s.store.ListStaffTargetOverrides(ctx, query)
		stats.Add(queryStats)
		if err != nil || !query.Days {
			return err
		}
		days, err = s.expandTargetOverrides(ctx, rows, query.From, query.To)
		return err
	})
	return rows, days, err
}

func validateTargetOverrideWindow(query domain.TargetOverrideQuery) error {
	if err := domain.ValidateDate(query.From, "from"); err != nil {
		return err
	}
	if err := domain.ValidateDate(query.To, "to"); err != nil {
		return err
	}
	if (query.From == "") != (query.To == "") || query.Days && query.From == "" {
		return &domain.TargetOverrideError{Kind: domain.ErrInvalidStaffTargetOverride, Reason: "target override window needs both from and to"}
	}
	return nil
}

// expandTargetOverrides asks the School Calendar for the statutory holidays of
// only the span the loaded ranges cover, so a Soll read without any
// Sonderarbeitszeit costs no calendar lookup at all.
func (s *Service) expandTargetOverrides(ctx context.Context, rows []domain.StaffTargetOverride, from, to string) (domain.TargetOverrideDays, error) {
	if len(rows) == 0 {
		return domain.TargetOverrideDays{}, nil
	}
	var holidays map[string]bool
	if s.holidays != nil {
		spanFrom, spanTo := rows[0].StartDate, rows[0].EndDate
		for _, row := range rows[1:] {
			spanFrom, spanTo = min(spanFrom, row.StartDate), max(spanTo, row.EndDate)
		}
		var err error
		if holidays, err = s.holidays(ctx, max(spanFrom, from), min(spanTo, to)); err != nil {
			return nil, fmt.Errorf("load statutory holidays for target overrides: %w", err)
		}
	}
	return domain.ExpandTargetOverrides(rows, from, to, holidays), nil
}

// CreateStaffTargetOverride stores a new Sonderarbeitszeit. The staff balance
// lock serializes it with every other Soll writer, so the overlap check and
// the closed-month check see a stable picture.
func (s *Service) CreateStaffTargetOverride(ctx context.Context, staffID int64, fields domain.StaffTargetOverrideFields, createdBy *int64) (result domain.StaffTargetOverride, err error) {
	if validationErr := domain.ValidateStaffTargetOverrideFields(fields); validationErr != nil {
		return result, validationErr
	}
	err = s.run("create_staff_target_override", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if err := s.guardTargetOverrideWrite(txCtx, staffID, stats, fields); err != nil {
				return err
			}
			var writeStats domain.OperationStats
			result, writeStats, err = s.store.CreateStaffTargetOverride(txCtx, domain.StaffTargetOverride{
				StaffID: staffID, StartDate: fields.StartDate, EndDate: fields.EndDate,
				DailyMinutes: fields.DailyMinutes, CreatedBy: createdBy,
			})
			stats.Add(writeStats)
			return err
		})
	})
	return result, err
}

// DeleteStaffTargetOverride removes a Sonderarbeitszeit outside closed months.
func (s *Service) DeleteStaffTargetOverride(ctx context.Context, staffID, id int64) error {
	return s.run("delete_staff_target_override", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			current, err := s.lockedTargetOverride(txCtx, staffID, id, stats)
			if err != nil {
				return err
			}
			if err := s.rejectClosedTargetOverrideMonths(txCtx, staffID, stats, domain.StaffTargetOverrideFields{StartDate: current.StartDate, EndDate: current.EndDate}); err != nil {
				return err
			}
			found, deleteStats, err := s.store.DeleteStaffTargetOverride(txCtx, staffID, id)
			stats.Add(deleteStats)
			if err == nil && !found {
				return domain.ErrStaffTargetOverrideNotFound
			}
			return err
		})
	})
}

// lockedTargetOverride takes the staff balance lock and loads the row the
// write changes. The row belongs to the staff member in the URL or it does
// not exist for this caller.
func (s *Service) lockedTargetOverride(ctx context.Context, staffID, id int64, stats *domain.OperationStats) (domain.StaffTargetOverride, error) {
	if err := s.transaction.LockStaffBalance(ctx, staffID); err != nil {
		return domain.StaffTargetOverride{}, err
	}
	current, found, findStats, err := s.store.FindStaffTargetOverride(ctx, staffID, id)
	stats.Add(findStats)
	if err != nil {
		return domain.StaffTargetOverride{}, err
	}
	if !found {
		return domain.StaffTargetOverride{}, domain.ErrStaffTargetOverrideNotFound
	}
	return current, nil
}

// guardTargetOverrideWrite takes the staff balance lock and rejects a new
// range that overlaps another one of the same staff member or touches a
// closed month.
func (s *Service) guardTargetOverrideWrite(ctx context.Context, staffID int64, stats *domain.OperationStats, fields domain.StaffTargetOverrideFields) error {
	if err := s.transaction.LockStaffBalance(ctx, staffID); err != nil {
		return err
	}
	existing, listStats, err := s.store.ListStaffTargetOverrides(ctx, domain.TargetOverrideQuery{StaffIDs: []int64{staffID}})
	stats.Add(listStats)
	if err != nil {
		return err
	}
	if err := domain.RejectTargetOverrideOverlap(existing, fields); err != nil {
		return err
	}
	return s.rejectClosedTargetOverrideMonths(ctx, staffID, stats, fields)
}

func (s *Service) rejectClosedTargetOverrideMonths(ctx context.Context, staffID int64, stats *domain.OperationStats, fields domain.StaffTargetOverrideFields) error {
	closed, snapshotStats, err := s.store.ClosedMonthSnapshotsForStaff(ctx, []int64{staffID})
	stats.Add(snapshotStats)
	if err != nil {
		return err
	}
	return domain.RejectClosedTargetOverrideMonths(closed, fields)
}
