package application

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// withdrawalPeriods dates withdrawal periods over the statutory holidays at
// the school of each contract (§ 193 BGB, #6350): Fronleichnam moves the
// last day at a school in Nordrhein-Westfalen, not at one in Berlin.
type withdrawalPeriods struct {
	holidays carecontract.SchoolHolidays
}

// withdrawalSubject is one contract whose period is asked for.
type withdrawalSubject struct {
	schoolID   int64
	concluded  *carecontract.Concluded
	informedAt *time.Time
}

// until is the last day of one contract's period, nil when it has none.
func (w withdrawalPeriods) until(ctx context.Context, schoolID int64, concluded *carecontract.Concluded, informedAt *time.Time) (*calendar.Date, error) {
	days, err := w.untilEach(ctx, []withdrawalSubject{{schoolID: schoolID, concluded: concluded, informedAt: informedAt}})
	if err != nil {
		return nil, err
	}
	return days[0], nil
}

// untilEach dates the periods of several contracts with one read of the
// holidays of their schools; a contract without a period reads none.
func (w withdrawalPeriods) untilEach(ctx context.Context, subjects []withdrawalSubject) ([]*calendar.Date, error) {
	result := make([]*calendar.Date, len(subjects))
	var from, to calendar.Date
	var schoolIDs []int64
	seen := map[int64]bool{}
	for _, subject := range subjects {
		if !hasWithdrawalPeriod(subject.concluded, subject.informedAt) {
			continue
		}
		first, last := withdrawalWindow(subject.concluded.At)
		if from.IsZero() || first.Before(from) {
			from = first
		}
		if to.IsZero() || last.After(to) {
			to = last
		}
		if !seen[subject.schoolID] {
			seen[subject.schoolID] = true
			schoolIDs = append(schoolIDs, subject.schoolID)
		}
	}
	if len(schoolIDs) == 0 {
		return result, nil
	}
	if w.holidays == nil {
		return nil, errors.New("care contract: the school holidays are not configured")
	}
	holidays, err := w.holidays.SchoolHolidayDates(ctx, schoolIDs, from, to)
	if err != nil {
		return nil, fmt.Errorf("care contract: read the school holidays: %w", err)
	}
	for i, subject := range subjects {
		if !hasWithdrawalPeriod(subject.concluded, subject.informedAt) {
			continue
		}
		days, ok := holidays[subject.schoolID]
		if !ok {
			return nil, fmt.Errorf("care contract: no holidays for school %d", subject.schoolID)
		}
		result[i] = withdrawalUntil(subject.concluded, subject.informedAt, days)
	}
	return result, nil
}

// withdrawalEnd is the last day of the 14-day withdrawal period that starts
// with the signature of a distance contract (§ 355 Abs. 2 BGB), moved past
// Saturday, Sunday and the holidays at the school (§ 193 BGB); the
// withdrawal and termination page names the same day (#4682, #6350).
func (w withdrawalPeriods) withdrawalEnd(ctx context.Context, schoolID int64, signedAt time.Time) (calendar.Date, error) {
	until, err := w.until(ctx, schoolID, &carecontract.Concluded{Method: carecontract.MethodElectronic, At: signedAt}, &signedAt)
	if err != nil {
		return "", err
	}
	return *until, nil
}
