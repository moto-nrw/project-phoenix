package domain

import (
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/workforce/openingbalance"
)

var ErrInvalidWorkSession = errors.New("invalid work session input")

type InvalidWorkSessionError struct{ Reason string }

func (e *InvalidWorkSessionError) Error() string { return e.Reason }
func (e *InvalidWorkSessionError) Unwrap() error { return ErrInvalidWorkSession }
func invalidWorkSession(reason string) error     { return &InvalidWorkSessionError{Reason: reason} }

type StaffVacationOpening struct {
	ID                   int64
	TenantID             int64
	StaffID              int64
	Year                 int
	EffectiveDate        string
	TakenBeforeDays      float64
	EnteredRemainingDays float64
	Note                 string
	DecidedBy            int64
	DecidedAt            time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

func (o StaffVacationOpening) Validate() error {
	value := openingbalance.Vacation{StaffID: o.StaffID, Year: o.Year, EffectiveDate: o.EffectiveDate,
		TakenBeforeDays: o.TakenBeforeDays, EnteredRemainingDays: o.EnteredRemainingDays, DecidedBy: o.DecidedBy}
	if err := value.Validate(); err != nil {
		return invalidWorkSession(err.Error())
	}
	return nil
}
