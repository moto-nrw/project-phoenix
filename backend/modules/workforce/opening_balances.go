package workforce

import "context"

// OpeningBalanceBookings is the go-live hours ledger capability. Preview
// validation is read-only; booking repeats its guards under the ledger lock.
// Calendar dates use YYYY-MM-DD, as in the time-tracking contract.
type OpeningBalanceBookings interface {
	ValidateOpeningBalance(context.Context, int64, int64, string, int, string) error
	CreateOpeningBalance(context.Context, int64, int64, string, int, string) (*StaffBalanceAdjustment, error)
}

// VacationTakeovers supplies the quota and opening commands in booking order:
// apply any quota changes before deriving days already taken from the opening.
type VacationTakeovers interface {
	VacationQuotaSummary(context.Context, int64, int) (*VacationQuotaSummary, error)
	UpsertVacationQuota(context.Context, int64, int, float64, float64) error
	SetVacationOpening(context.Context, int64, int64, SetVacationOpeningRequest) (*StaffVacationOpening, error)
	ValidateVacationOpeningAbsencesBefore(context.Context, int64, string) error
}
