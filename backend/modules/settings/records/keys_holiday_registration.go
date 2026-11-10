package records

// KeyHolidayRegistrationWishMode says what the holiday care wish of the
// online registration does in a holiday period that takes it into account
// (#4703): "priority" lets children without a wish wait behind the others,
// "wish_only" refuses a child whose parents said no holiday care in the
// online registration. Only the Träger sets it (LevelTraeger). Enrollment
// refuses any other value.
const KeyHolidayRegistrationWishMode = "holiday_registration.wish_mode"
