package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) PreviewAllowanceBooking(ctx context.Context, staffID, absenceTypeID int64, candidate domain.StaffAbsence, firstYear, lastYear int) ([]domain.AbsenceTypeAllowanceSummary, error) {
	if absenceTypeID <= 0 {
		return nil, domain.ErrAbsenceTypeNotFound
	}
	absenceType, err := s.FindStaffAbsenceType(ctx, absenceTypeID)
	if err != nil {
		if errors.Is(err, domain.ErrAbsenceTypeNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("find absence type: database error during find by id: %w", err)
	}
	if !absenceType.AllowanceEnabled {
		return nil, nil
	}
	previews := make([]domain.AbsenceTypeAllowanceSummary, 0, lastYear-firstYear+1)
	blocked := false
	for year := firstYear; year <= lastYear; year++ {
		summary, err := s.AllowanceSummary(ctx, staffID, absenceTypeID, year)
		if err != nil {
			return nil, err
		}
		existing, err := s.ListStaffAbsences(ctx, domain.StaffAbsenceFilter{
			StaffID: staffID, OverlapFrom: fmt.Sprintf("%04d-01-01", year), OverlapTo: fmt.Sprintf("%04d-12-31", year),
			Order: []domain.StaffAbsenceOrder{{Field: domain.StaffAbsenceOrderDateStart}},
		})
		if err != nil {
			return nil, fmt.Errorf("load overlapping absences for allowance preview: database error during get absences by staff and date range: %w", err)
		}
		blocking := make([]domain.StaffAbsence, 0, len(existing))
		for _, absence := range existing {
			if absence.AbsenceTypeID == nil || *absence.AbsenceTypeID != absenceTypeID {
				continue
			}
			switch absence.Status {
			case domain.AbsenceStatusReported, domain.AbsenceStatusApproved, domain.AbsenceStatusRequested, domain.AbsenceStatusQuestion:
				blocking = append(blocking, absence)
			}
		}
		days, err := s.additionalAbsenceDays(candidate, blocking, year)
		if err != nil {
			return nil, err
		}
		summary.TakenDays += days
		summary.RemainingDays -= days
		previews = append(previews, summary)
		// Kontingente never go negative (#3256); only the Stundenkonto may.
		if summary.RemainingDays < 0 {
			blocked = true
		}
	}
	if blocked {
		return previews, domain.ErrAbsenceTypeAllowanceExceeded
	}
	return previews, nil
}

func (s *Service) AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (domain.AbsenceTypeAllowanceSummary, error) {
	if staffID <= 0 || year < 2000 || year > 2100 {
		return domain.AbsenceTypeAllowanceSummary{}, domain.ErrAbsenceTypeAllowanceInvalid
	}
	if absenceTypeID <= 0 {
		return domain.AbsenceTypeAllowanceSummary{}, domain.ErrAbsenceTypeNotFound
	}
	if _, err := s.FindStaffAbsenceType(ctx, absenceTypeID); err != nil {
		if errors.Is(err, domain.ErrAbsenceTypeNotFound) {
			return domain.AbsenceTypeAllowanceSummary{}, err
		}
		return domain.AbsenceTypeAllowanceSummary{}, fmt.Errorf("find absence type: database error during find by id: %w", err)
	}
	var entitled float64
	err := s.run("staff_absence_type_entitlement", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var err error
		entitled, _, queryStats, err = s.store.StaffAbsenceTypeEntitlement(ctx, staffID, absenceTypeID, year)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, fmt.Errorf("load absence type allowance: database error during list with options: %w", err)
	}
	absences, err := s.ListStaffAbsences(ctx, domain.StaffAbsenceFilter{
		StaffID: staffID, OverlapFrom: fmt.Sprintf("%04d-01-01", year), OverlapTo: fmt.Sprintf("%04d-12-31", year),
		Order: []domain.StaffAbsenceOrder{{Field: domain.StaffAbsenceOrderDateStart}},
	})
	if err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, fmt.Errorf("load absences for allowance: database error during get absences by staff and date range: %w", err)
	}
	result := domain.AbsenceTypeAllowanceSummary{StaffID: staffID, AbsenceTypeID: absenceTypeID, Year: year, EntitledDays: entitled}
	for _, absence := range absences {
		if absence.AbsenceTypeID == nil || *absence.AbsenceTypeID != absenceTypeID {
			continue
		}
		days, err := s.absenceDays(absence, year)
		if err != nil {
			return domain.AbsenceTypeAllowanceSummary{}, err
		}
		switch absence.Status {
		case domain.AbsenceStatusReported, domain.AbsenceStatusApproved:
			result.TakenDays += days
		case domain.AbsenceStatusRequested, domain.AbsenceStatusQuestion:
			result.ReservedDays += days
		}
	}
	result.RemainingDays = result.EntitledDays - result.TakenDays - result.ReservedDays
	return result, nil
}

func formatAllowanceDays(days float64) string {
	value := strings.Replace(strconv.FormatFloat(days, 'f', -1, 64), ".", ",", 1)
	if days == 1 {
		return value + " Tag"
	}
	return value + " Tage"
}

func (s *Service) SetAllowance(ctx context.Context, input domain.SetAbsenceTypeAllowance) (domain.AbsenceTypeAllowanceSummary, error) {
	if input.AbsenceTypeID <= 0 {
		return domain.AbsenceTypeAllowanceSummary{}, domain.ErrAbsenceTypeNotFound
	}
	absenceType, err := s.FindStaffAbsenceType(ctx, input.AbsenceTypeID)
	if err != nil {
		if errors.Is(err, domain.ErrAbsenceTypeNotFound) {
			return domain.AbsenceTypeAllowanceSummary{}, err
		}
		return domain.AbsenceTypeAllowanceSummary{}, fmt.Errorf("find absence type: database error during find by id: %w", err)
	}
	if !absenceType.AllowanceEnabled {
		return domain.AbsenceTypeAllowanceSummary{}, &domain.InvalidAbsenceAllowanceError{Reason: "diese Abwesenheitsart hat kein Kontingent"}
	}
	if err := input.Validate(); err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, err
	}
	var result domain.AbsenceTypeAllowanceSummary
	err = s.run("set_staff_absence_type_allowance", func(stats *domain.OperationStats) error {
		return s.transaction.RunWrite(ctx, func(txCtx context.Context) error {
			if err := s.LockStaffAbsenceWrites(txCtx, input.StaffID); err != nil {
				return fmt.Errorf("lock staff allowance writes: %w", err)
			}
			previous, found, queryStats, err := s.store.StaffAbsenceTypeEntitlement(txCtx, input.StaffID, input.AbsenceTypeID, input.Year)
			stats.Add(queryStats)
			if err != nil {
				return fmt.Errorf("load existing absence type allowance: database error during list with options: %w", err)
			}
			current, err := s.AllowanceSummary(txCtx, input.StaffID, input.AbsenceTypeID, input.Year)
			if err != nil {
				return err
			}
			// Kontingente never go negative (#3256): taken and requested days
			// have to be removed before the claim can shrink below them.
			if used := current.TakenDays + current.ReservedDays; input.EntitledDays < used {
				return &domain.InvalidAbsenceAllowanceError{Reason: fmt.Sprintf(
					"Der Anspruch kann nicht unter %s liegen, so viele Tage sind schon eingetragen oder beantragt.",
					formatAllowanceDays(used),
				)}
			}
			var oldDays *float64
			if found {
				oldDays = &previous
			}
			writeStats, err := s.store.UpsertStaffAbsenceTypeAllowance(txCtx, input)
			stats.Add(writeStats)
			if err != nil {
				return fmt.Errorf("save absence type allowance: database error during upsert staff absence type allowance: %w", err)
			}
			auditStats, err := s.store.RecordStaffAbsenceTypeAllowanceChange(txCtx, input, oldDays)
			stats.Add(auditStats)
			if err != nil {
				return fmt.Errorf("audit absence type allowance: database error during create: %w", err)
			}
			result, err = s.AllowanceSummary(txCtx, input.StaffID, input.AbsenceTypeID, input.Year)
			return err
		})
	})
	if err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, err
	}
	return result, nil
}
