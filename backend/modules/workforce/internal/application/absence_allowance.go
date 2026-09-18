package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// previewAbsenceID marks the not yet stored booking inside a preview ledger;
// stored absences always have a positive ID.
const previewAbsenceID int64 = 0

// allowanceAccounts is everything the ledger of one person and one type
// needs for the years firstYear..lastYear.
type allowanceAccounts struct {
	absenceType  domain.StaffAbsenceType
	entitlements map[int]float64
	uses         []domain.AllowanceUse
}

func (a allowanceAccounts) ledger(today string, extra ...domain.AllowanceUse) domain.AllowanceLedger {
	return domain.BuildAllowanceLedger(a.entitlements, a.absenceType.CarryoverUntil, today, append(slices.Clone(a.uses), extra...))
}

func (s *Service) findAllowanceType(ctx context.Context, absenceTypeID int64) (domain.StaffAbsenceType, error) {
	if absenceTypeID <= 0 {
		return domain.StaffAbsenceType{}, domain.ErrAbsenceTypeNotFound
	}
	absenceType, err := s.FindStaffAbsenceType(ctx, absenceTypeID)
	if err != nil {
		if errors.Is(err, domain.ErrAbsenceTypeNotFound) {
			return domain.StaffAbsenceType{}, err
		}
		return domain.StaffAbsenceType{}, fmt.Errorf("find absence type: database error during find by id: %w", err)
	}
	return absenceType, nil
}

// loadAllowanceAccounts reads the claims and bookings behind the accounts of
// firstYear..lastYear. With a carryover the chain starts at the oldest claim,
// because an older rest decides how much of each later year is still open.
func (s *Service) loadAllowanceAccounts(ctx context.Context, staffID int64, absenceType domain.StaffAbsenceType, firstYear, lastYear int) (allowanceAccounts, error) {
	accounts := allowanceAccounts{absenceType: absenceType}
	err := s.run("staff_absence_type_entitlements", func(stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var err error
		accounts.entitlements, queryStats, err = s.store.StaffAbsenceTypeEntitlements(ctx, staffID, absenceType.ID)
		stats.Add(queryStats)
		return err
	})
	if err != nil {
		return allowanceAccounts{}, fmt.Errorf("load absence type allowance: database error during list with options: %w", err)
	}
	from := firstYear
	if absenceType.CarryoverUntil != "" {
		from = firstYear - 1
		for year := range accounts.entitlements {
			from = min(from, year)
		}
	}
	absences, err := s.ListStaffAbsences(ctx, domain.StaffAbsenceFilter{
		StaffID:     staffID,
		OverlapFrom: fmt.Sprintf("%04d-01-01", from),
		OverlapTo:   domain.AllowanceExpiresOn(lastYear, absenceType.CarryoverUntil),
		Order:       []domain.StaffAbsenceOrder{{Field: domain.StaffAbsenceOrderDateStart}},
	})
	if err != nil {
		return allowanceAccounts{}, fmt.Errorf("load absences for allowance: database error during get absences by staff and date range: %w", err)
	}
	for _, absence := range absences {
		if absence.AbsenceTypeID == nil || *absence.AbsenceTypeID != absenceType.ID {
			continue
		}
		switch absence.Status {
		case domain.AbsenceStatusReported, domain.AbsenceStatusApproved, domain.AbsenceStatusRequested, domain.AbsenceStatusQuestion:
		default:
			continue
		}
		uses, err := s.allowanceUses(absence)
		if err != nil {
			return allowanceAccounts{}, err
		}
		accounts.uses = append(accounts.uses, uses...)
	}
	return accounts, nil
}

func (s *Service) PreviewAllowanceBooking(ctx context.Context, staffID, absenceTypeID int64, candidate domain.StaffAbsence, firstYear, lastYear int) ([]domain.AbsenceTypeAllowanceSummary, error) {
	absenceType, err := s.findAllowanceType(ctx, absenceTypeID)
	if err != nil {
		return nil, err
	}
	if !absenceType.AllowanceEnabled {
		return nil, nil
	}
	accounts, err := s.loadAllowanceAccounts(ctx, staffID, absenceType, firstYear, lastYear)
	if err != nil {
		return nil, err
	}
	candidate.ID, candidate.StaffID, candidate.Status = previewAbsenceID, staffID, domain.AbsenceStatusReported
	candidate.CreatedAt = s.clock.Now()
	requested, err := s.allowanceUses(candidate)
	if err != nil {
		return nil, err
	}
	// Overlapping bookings are merged by the writer, so only coverage beyond
	// what the day already holds is new.
	covered := make(map[string]float64)
	for _, use := range accounts.uses {
		covered[use.Day] = max(covered[use.Day], use.Days)
	}
	additional := make([]domain.AllowanceUse, 0, len(requested))
	for _, use := range requested {
		use.Days = max(0, use.Days-covered[use.Day])
		additional = append(additional, use)
	}
	return s.allowancePreviews(accounts, staffID, absenceTypeID, firstYear, lastYear, []int64{previewAbsenceID}, additional)
}

// PreviewAllowanceRebooking shows what moving stored absences into the
// allowance of absenceTypeID would take (#3258). Each absence keeps its entry
// date, so it may use an older rest exactly when its original entry could
// have. With ErrAbsenceTypeAllowanceExceeded the previews are still set.
func (s *Service) PreviewAllowanceRebooking(ctx context.Context, staffID, absenceTypeID int64, absenceIDs []int64) ([]domain.AbsenceTypeAllowanceSummary, error) {
	absenceType, err := s.findAllowanceType(ctx, absenceTypeID)
	if err != nil {
		return nil, err
	}
	if !absenceType.AllowanceEnabled || len(absenceIDs) == 0 {
		return nil, nil
	}
	candidates := make([]domain.StaffAbsence, 0, len(absenceIDs))
	firstYear, lastYear := 0, 0
	for _, id := range absenceIDs {
		absence, err := s.FindStaffAbsence(ctx, id)
		if err != nil {
			return nil, err
		}
		if absence.StaffID != staffID {
			return nil, domain.ErrStaffAbsenceNotFound
		}
		start, end, err := absenceYears(absence)
		if err != nil {
			return nil, err
		}
		if firstYear == 0 || start < firstYear {
			firstYear = start
		}
		lastYear = max(lastYear, end)
		candidates = append(candidates, absence)
	}
	accounts, err := s.loadAllowanceAccounts(ctx, staffID, absenceType, firstYear, lastYear)
	if err != nil {
		return nil, err
	}
	accounts.uses = slices.DeleteFunc(accounts.uses, func(use domain.AllowanceUse) bool {
		return slices.Contains(absenceIDs, use.AbsenceID)
	})
	var additional []domain.AllowanceUse
	for _, candidate := range candidates {
		uses, err := s.allowanceUses(candidate)
		if err != nil {
			return nil, err
		}
		additional = append(additional, uses...)
	}
	return s.allowancePreviews(accounts, staffID, absenceTypeID, firstYear, lastYear, absenceIDs, additional)
}

func absenceYears(absence domain.StaffAbsence) (int, int, error) {
	if len(absence.DateStart) < 4 || len(absence.DateEnd) < 4 {
		return 0, 0, domain.ErrAbsenceTypeAllowanceInvalid
	}
	start, err := strconv.Atoi(absence.DateStart[:4])
	if err != nil {
		return 0, 0, domain.ErrAbsenceTypeAllowanceInvalid
	}
	end, err := strconv.Atoi(absence.DateEnd[:4])
	if err != nil {
		return 0, 0, domain.ErrAbsenceTypeAllowanceInvalid
	}
	return start, end, nil
}

// allowancePreviews books additional onto the accounts and reports every year
// the bookings in bookingIDs draw on, plus firstYear..lastYear.
func (s *Service) allowancePreviews(
	accounts allowanceAccounts,
	staffID, absenceTypeID int64,
	firstYear, lastYear int,
	bookingIDs []int64,
	additional []domain.AllowanceUse,
) ([]domain.AbsenceTypeAllowanceSummary, error) {
	today := s.clock.Today()
	before := accounts.ledger(today)
	after := accounts.ledger(today, additional...)

	var years []int
	for _, id := range bookingIDs {
		for _, year := range after.BookedYears(id) {
			if !slices.Contains(years, year) {
				years = append(years, year)
			}
		}
	}
	for year := firstYear; year <= lastYear; year++ {
		if !slices.Contains(years, year) {
			years = append(years, year)
		}
	}
	slices.Sort(years)
	previews := make([]domain.AbsenceTypeAllowanceSummary, 0, len(years))
	for _, year := range years {
		summary := after.Summary(staffID, absenceTypeID, year)
		for _, id := range bookingIDs {
			summary.BookingDays += after.Booked(id, year)
		}
		previews = append(previews, summary)
	}
	// Kontingente never go negative (#3256); only the Stundenkonto may.
	if len(after.Overdrawn(before)) > 0 {
		return previews, domain.ErrAbsenceTypeAllowanceExceeded
	}
	return previews, nil
}

func (s *Service) AllowanceSummary(ctx context.Context, staffID, absenceTypeID int64, year int) (domain.AbsenceTypeAllowanceSummary, error) {
	if staffID <= 0 || year < 2000 || year > 2100 {
		return domain.AbsenceTypeAllowanceSummary{}, domain.ErrAbsenceTypeAllowanceInvalid
	}
	absenceType, err := s.findAllowanceType(ctx, absenceTypeID)
	if err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, err
	}
	accounts, err := s.loadAllowanceAccounts(ctx, staffID, absenceType, year, year)
	if err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, err
	}
	return accounts.ledger(s.clock.Today()).Summary(staffID, absenceTypeID, year), nil
}

func formatAllowanceDays(days float64) string {
	value := strings.Replace(strconv.FormatFloat(days, 'f', -1, 64), ".", ",", 1)
	if days == 1 {
		return value + " Tag"
	}
	return value + " Tage"
}

func (s *Service) SetAllowance(ctx context.Context, input domain.SetAbsenceTypeAllowance) (domain.AbsenceTypeAllowanceSummary, error) {
	absenceType, err := s.findAllowanceType(ctx, input.AbsenceTypeID)
	if err != nil {
		return domain.AbsenceTypeAllowanceSummary{}, err
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
			// A claim also feeds the carryover into the following year.
			accounts, err := s.loadAllowanceAccounts(txCtx, input.StaffID, absenceType, input.Year, input.Year+1)
			if err != nil {
				return err
			}
			if err := s.ensureAllowanceClaimCovers(accounts, input); err != nil {
				return err
			}
			var oldDays *float64
			if previous, found := accounts.entitlements[input.Year]; found {
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

// ensureAllowanceClaimCovers keeps Kontingente from going negative (#3256):
// taken and requested days have to be removed before a claim can shrink
// below them, including days of the following year that use this rest.
func (s *Service) ensureAllowanceClaimCovers(accounts allowanceAccounts, input domain.SetAbsenceTypeAllowance) error {
	today := s.clock.Today()
	before := accounts.ledger(today)
	changed := accounts
	changed.entitlements = make(map[int]float64, len(accounts.entitlements)+1)
	for year, days := range accounts.entitlements {
		changed.entitlements[year] = days
	}
	changed.entitlements[input.Year] = input.EntitledDays
	after := changed.ledger(today)
	overdrawn := after.Overdrawn(before)
	if len(overdrawn) == 0 {
		return nil
	}
	if slices.Contains(overdrawn, input.Year) {
		summary := after.Summary(input.StaffID, input.AbsenceTypeID, input.Year)
		return &domain.InvalidAbsenceAllowanceError{Reason: fmt.Sprintf(
			"Der Anspruch kann nicht unter %s liegen, so viele Tage sind schon eingetragen oder beantragt.",
			formatAllowanceDays(summary.TakenDays+summary.ReservedDays),
		)}
	}
	return &domain.InvalidAbsenceAllowanceError{Reason: fmt.Sprintf(
		"Der Anspruch kann nicht so stark sinken: Eingetragene Tage bis %s nutzen den Rest aus %d, und für %d reichen die Tage dann nicht mehr.",
		formatGermanDate(domain.AllowanceExpiresOn(input.Year, accounts.absenceType.CarryoverUntil)), input.Year, overdrawn[0],
	)}
}

// formatGermanDate renders a strict YYYY-MM-DD day as DD.MM.YYYY.
func formatGermanDate(day string) string {
	if len(day) != 10 {
		return day
	}
	return day[8:10] + "." + day[5:7] + "." + day[0:4]
}
