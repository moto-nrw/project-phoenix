package records

// The thresholds of the hints on the Träger dashboard (#4675): from which
// value the dashboard marks a school. Only the Träger sets them
// (LevelTraeger); 0 switches the hint off.
//
//   - KeyTraegerAlertChildrenPerStaff: more present children per present
//     staff member than this marks too little staff. Present children with
//     no present staff member mark it too.
//   - KeyTraegerAlertSickPercent: at least this share of the children, in
//     percent, sick today.
//   - KeyTraegerAlertOpenEnrollments: at least this many children of the
//     online enrollment waiting for a decision.
const (
	KeyTraegerAlertChildrenPerStaff = "traeger_dashboard.alert_children_per_staff"
	KeyTraegerAlertSickPercent      = "traeger_dashboard.alert_sick_percent"
	KeyTraegerAlertOpenEnrollments  = "traeger_dashboard.alert_open_enrollments"
)
