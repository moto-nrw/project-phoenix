package domain

// SchoolVacationRecord is one vacation window of a federal state as the sync
// stores it (#5345). Region is the ISO code ("DE-NW"); StartDate and EndDate
// are calendar days in YYYY-MM-DD, both part of the vacation. It is the same
// struct type the OpenHolidays adapter returns, so the composition binds the
// adapter without a translation.
type SchoolVacationRecord = struct {
	Region    string
	Name      string
	StartDate string
	EndDate   string
}
