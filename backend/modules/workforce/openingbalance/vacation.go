// Package openingbalance defines validated Workforce takeover values.
package openingbalance

import (
	"errors"
	"math"
	"time"
)

// Vacation contains the values needed to validate a vacation takeover.
// EffectiveDate is the ISO calendar date, never an instant.
type Vacation struct {
	StaffID                               int64
	Year                                  int
	EffectiveDate                         string
	TakenBeforeDays, EnteredRemainingDays float64
	DecidedBy                             int64
}

func (o Vacation) Validate() error {
	if o.StaffID <= 0 {
		return errors.New("staff_id is required")
	}
	if o.Year < 2000 || o.Year > 2100 {
		return errors.New("year out of range")
	}
	if o.EffectiveDate == "" {
		return errors.New("effective_date is required")
	}
	const layout = "2006-01-02"
	effective, err := time.Parse(layout, o.EffectiveDate)
	if err != nil || effective.Format(layout) != o.EffectiveDate {
		return errors.New("effective_date must be a " + layout + " date")
	}
	if effective.Year() != o.Year {
		return errors.New("effective_date must lie in the opening year")
	}
	if o.TakenBeforeDays < -999 || o.TakenBeforeDays > 999 {
		return errors.New("taken_before_days out of range")
	}
	if o.EnteredRemainingDays < -999 || o.EnteredRemainingDays > 999 {
		return errors.New("entered_remaining_days out of range")
	}
	for _, value := range []float64{o.TakenBeforeDays, o.EnteredRemainingDays} {
		if !(math.Abs(value*10-math.Round(value*10)) < 1e-9) {
			return errors.New("vacation opening days must have at most one decimal place")
		}
	}
	if o.DecidedBy <= 0 {
		return errors.New("decided_by is required")
	}
	return nil
}
