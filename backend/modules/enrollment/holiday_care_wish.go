package enrollment

// Holiday care wish of the online registration (#4703): a form template
// can ask the parents per child whether the child needs care in the
// holidays. The answer is no free custom field: it has its own column on
// the request child and counts once the child is approved, for the school
// year of the phase. The holiday registration of the Träger reads it, only
// for a parent with the relationship right (ADR 0113), and gives such
// children precedence when the period asks for it.

// CoreRequirementHolidayCareWish makes a form template ask, and require,
// the holiday care wish of every child.
const CoreRequirementHolidayCareWish = "holiday_care_wish"
