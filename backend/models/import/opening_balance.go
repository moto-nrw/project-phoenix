package importpkg

// OpeningBalanceImportRow is one line of the go-live opening balance import
// (#2132): Stundenkonto and vacation takeover values for one staff member.
// The raw string fields mirror the file columns; the typed fields are
// resolved during validation. Every value column is optional — an empty cell
// skips that side for the row.
type OpeningBalanceImportRow struct {
	PersonnelNumber   string `json:"personnel_number"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	HoursBalance      string `json:"hours_balance"`
	VacationEntitled  string `json:"vacation_entitled"`
	VacationCarryover string `json:"vacation_carryover"`
	VacationRemaining string `json:"vacation_remaining"`

	// Resolved during validation, not part of the file.
	StaffID               int64    `json:"staff_id,omitempty"`
	HoursBalanceMinutes   *int     `json:"hours_balance_minutes,omitempty"`
	VacationEntitledDays  *float64 `json:"vacation_entitled_days,omitempty"`
	VacationCarryoverDays *float64 `json:"vacation_carryover_days,omitempty"`
	VacationRemainingDays *float64 `json:"vacation_remaining_days,omitempty"`
}
