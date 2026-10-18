package users

// GuardianEmergencyContactRow is one (guardian, phone number) projection row
// for the emergency contact list; the consumer aggregates rows per student.
type GuardianEmergencyContactRow struct {
	StudentID         int64   `bun:"student_id"`
	GuardianProfileID int64   `bun:"guardian_profile_id"`
	FirstName         *string `bun:"first_name"`
	LastName          *string `bun:"last_name"`
	Email             *string `bun:"email"`
	PhoneNumber       *string `bun:"phone_number"`
	// RelationshipType, CanPickup and IsEmergencyContact carry the role of the
	// relationship (#2527). The emergency list itself ignores them — it wants
	// every reachable adult — but the supervision sheet must tell "darf
	// abholen" apart from "im Notfall anrufen", and both answers come from the
	// same join.
	RelationshipType   *string `bun:"relationship_type"`
	PickupNotes        *string `bun:"pickup_notes"`
	CanPickup          bool    `bun:"can_pickup"`
	IsEmergencyContact bool    `bun:"is_emergency_contact"`
}
