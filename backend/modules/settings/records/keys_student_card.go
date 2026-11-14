package records

// What the child cards show besides name and status (Betrieb › Kinderkarte
// und Fotos).
//
//   - KeyStudentPhotosEnabled: the child's photo on the card.
//   - KeyStudentCardActivitiesEnabled: today's planned offerings on the
//     child cards under "Gruppen" (#6487).
const (
	KeyStudentPhotosEnabled         = "operations.student_photos_enabled"
	KeyStudentCardActivitiesEnabled = "operations.student_card_activities_enabled"
)
