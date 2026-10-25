package records

// Values of the meal ordering settings (#3873). The Meal Plan reads them as
// plain strings and owns their meaning.
const (
	// MealRegistrationModelSingle: parents register and cancel until one
	// deadline per meal day. The default, and the behaviour before #3873.
	MealRegistrationModelSingle = "ongoing_single"
	// MealRegistrationModelSplit: registering ends at the registration
	// deadline, cancelling at a separate cancellation deadline.
	MealRegistrationModelSplit = "ongoing_split"
	// MealRegistrationModelAnnual: parents register once for the school year
	// until a fixed date; afterwards only the school registers, and parents
	// cancel if the school allows it.
	MealRegistrationModelAnnual = "annual"

	// MealDeadlineDaysBefore: a number of days before the meal day.
	MealDeadlineDaysBefore = "days_before"
	// MealDeadlinePreviousWeek: a weekday of the week before the meal day.
	MealDeadlinePreviousWeek = "previous_week"

	MealWeekdayMonday    = "monday"
	MealWeekdayTuesday   = "tuesday"
	MealWeekdayWednesday = "wednesday"
	MealWeekdayThursday  = "thursday"
	MealWeekdayFriday    = "friday"

	// MealAnnualChangesParentsCancel: after the annual deadline parents may
	// still cancel days or weeks until the cancellation deadline.
	MealAnnualChangesParentsCancel = "parents_cancel"
	// MealAnnualChangesSchoolOnly: after the annual deadline only the school
	// changes a registration.
	MealAnnualChangesSchoolOnly = "school_only"

	// MealSickStaysOrdered: an absence reported after the cancellation
	// deadline does not cancel the meal. The default.
	MealSickStaysOrdered = "stays_ordered"
	// MealSickCancelled: an absence reported up to the meal day itself
	// cancels the meal; after the order it reaches the caterer as a change.
	MealSickCancelled = "cancelled"

	MealOrderRhythmDaily  = "daily"
	MealOrderRhythmWeekly = "weekly"

	// MealOrderDetailTotal: the order carries the number of meals per day.
	MealOrderDetailTotal = "total"
	// MealOrderDetailByClass: the order also breaks the number down by class.
	MealOrderDetailByClass = "by_class"
)
