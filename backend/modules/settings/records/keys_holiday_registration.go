package records

// KeyHolidayRegistrationWishMode says what the holiday care wish of the
// online registration does in a holiday period that takes it into account
// (#4703): "priority" lets children without a wish wait behind the others,
// "wish_only" refuses a child whose parents said no holiday care in the
// online registration. Only the Träger sets it (LevelTraeger). Enrollment
// refuses any other value.
const KeyHolidayRegistrationWishMode = "holiday_registration.wish_mode"

// KeyHolidayRegistrationRetentionDays says how many days after the last
// week of a holiday period the Träger keeps its registrations (#6691, ADR
// 0118 addendum). The privacy text of the public registration names the
// value; a registration keeps the days it was submitted with, so a change
// reaches only later registrations. Only the Träger sets it (LevelTraeger).
// Enrollment refuses a value outside 92 to 1095.
const KeyHolidayRegistrationRetentionDays = "holiday_registration.retention_days"
