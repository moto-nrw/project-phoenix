package presenceprojection

// The projection answers in its own vocabulary (ADR 0058); the Timetable
// composition maps the rows onto the retained schedule DTOs.

// The planning and execution states the legacy readers compare against.
const (
	statusPlanned   = "planned"
	statusActive    = "active"
	statusCompleted = "completed"
	statusCancelled = "cancelled"
)

// careGroupType is the activities.groups type of a care block template.
const careGroupType = "care"

// presenceContract.Instance is one activity instance with its execution folded onto the
// legacy status vocabulary: a started block that is not cancelled carries
// the session's status and execution fields.

// presenceContract.Participant is one planned participant of a block with its attendance; a
// participant without an attendance row is expected.

// presenceContract.ScheduledInstance is one planned block of a child with the attendance the
// child's participant row carries.

// presenceContract.ParallelPresence is another running block of the day a child is still
// present in.

// presenceContract.PartialAbsenceBlock is one block an earlier pickup can still excuse.
