package domain

import (
	"math"
	"strings"
)

type InvalidAbsenceAllowanceError struct{ Reason string }

func (e *InvalidAbsenceAllowanceError) Error() string { return e.Reason }
func (e *InvalidAbsenceAllowanceError) Unwrap() error { return ErrAbsenceTypeAllowanceInvalid }

type SetAbsenceTypeAllowance struct {
	StaffID, AbsenceTypeID int64
	Year                   int
	EntitledDays           float64
	Reason                 string
	ChangedBy              int64
}

func (input SetAbsenceTypeAllowance) Validate() error {
	var reason string
	switch {
	case input.StaffID <= 0:
		reason = "staff_id is required"
	case input.AbsenceTypeID <= 0:
		reason = "absence_type_id is required"
	case input.Year < 2000 || input.Year > 2100:
		reason = "year out of range"
	case input.EntitledDays < 0 || input.EntitledDays > 366:
		reason = "entitled_days out of range"
	case math.Abs(input.EntitledDays*2-math.Round(input.EntitledDays*2)) > 0.000001:
		reason = "entitled_days must use whole or half days"
	case strings.TrimSpace(input.Reason) == "" || input.ChangedBy <= 0:
		reason = "Begründung und ändernde Person sind erforderlich"
	default:
		return nil
	}
	return &InvalidAbsenceAllowanceError{Reason: reason}
}

type AbsenceTypeAllowanceSummary struct {
	StaffID, AbsenceTypeID                               int64
	Year                                                 int
	EntitledDays, TakenDays, ReservedDays, RemainingDays float64
}
