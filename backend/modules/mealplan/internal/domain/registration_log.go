package domain

// RegistrationChangeKind is what an entry of the registration log changed.
type RegistrationChangeKind string

const (
	// RegistrationDay registers or cancels one day.
	RegistrationDay RegistrationChangeKind = "day"
	// RegistrationRegular returns one day to the regular days.
	RegistrationRegular RegistrationChangeKind = "regular"
	// RegistrationSchedule sets the regular days from a day on.
	RegistrationSchedule RegistrationChangeKind = "schedule"
)

// RegistrationChange is one entry of a child's registration log (#3873):
// who changed what, as the school or as a parent. Date is the day, or the
// first day of new regular days. The log only grows.
type RegistrationChange struct {
	StudentID     int64
	AccountID     int64
	BySchool      bool
	Kind          RegistrationChangeKind
	Date          Date
	Participating *bool
	Weekdays      []Weekday
}
