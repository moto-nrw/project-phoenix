package careplan

// CareDayAttendance is the timetable's attendance provenance, without its
// persistence model. A plan-owned absence is one stamped by a status day or
// partial excusal; a manual decision always wins over that provenance.
type CareDayAttendance struct {
	Expected         bool
	NotScheduled     bool
	ManuallyDecided  bool
	PlanOwnedAbsence bool
}

// AttendanceRowCareDay maps one timetable attendance row onto the care-day
// verdict every reader groups, counts, and renders by (#1747 review). It is the
// single implementation behind the planner list (api/timetable), the operation
// roster, and the planned-now cards, so those three can never disagree about
// the same child.
//
// planVerdict is what the derivation says about that child on the instance's
// date; the empty string means "not resolved" and reads as unknown — a missing
// fact never excludes a child.
//
// On a completed instance the verdict is frozen: ending the block wrote the
// absences and stamped not_scheduled on the children it spared, so that column
// IS the verdict. Reading the current care plan there would let a later plan
// edit relabel a finished day.
//
// A row somebody set by hand (ManualStatusAt) reports unknown regardless of
// what the plan says: staff setting an unbooked slot back to 'expected' means
// "the plan is wrong, this child is coming", and the planner must show that
// instead of the derivation it overrides (#1747 review).
//
// A row that already carries a real attendance status tells its own story and
// reports unknown — with two exceptions that still count as non-bookings when
// the plan says the child was never expected:
//
//  1. An absence a broad day status (sick / excused / class trip) wrote and
//     still owns via student_status_day_id. ApplyStatusDay stamps every
//     expected row of the day, including days the child was never booked into
//     care (a check-in and a manual PATCH both clear that column).
//  2. An absence a partial-day excusal wrote and still owns via
//     pickup_exception_id. ApplyPartialAbsence does the same for blocks that
//     start at or after the cutoff; without recognizing that provenance the
//     planner treats the active absence as real and under-counts non-scheduled
//     children until session end.
//
// Until the block ends and MarkNotScheduled undoes it, such a row is a false
// absence, so it keeps the non-booking verdict. A cancelled care day WAS
// booked, so its absence is real and stays one, and a manual absence is never
// relabelled.
func AttendanceRowCareDay(instanceCompleted bool, row *CareDayAttendance, planVerdict CareDayStatus) CareDayStatus {
	if row == nil {
		return CareDayUnknown
	}
	if row.ManuallyDecided {
		return CareDayUnknown
	}
	if instanceCompleted {
		if row.NotScheduled && row.Expected {
			return CareDayNotScheduled
		}
		return CareDayUnknown
	}
	if !row.Expected {
		// Plan-owned absences on a not-scheduled day remain non-bookings while
		// the block is active. Manual status already returned above.
		if planVerdict == CareDayNotScheduled &&
			row.PlanOwnedAbsence {
			return CareDayNotScheduled
		}
		return CareDayUnknown
	}
	if planVerdict != "" {
		return planVerdict
	}
	return CareDayUnknown
}
