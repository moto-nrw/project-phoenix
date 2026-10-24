package peopledirectory

type GuardianEmergencyContactRow struct {
	StudentID         int64
	GuardianProfileID int64
	FirstName         *string
	LastName          *string
	Email             *string
	PhoneNumber       *string
	// RelationshipType, CanPickup and IsEmergencyContact carry the role of the
	// relationship (#2527). The emergency list itself ignores them — it wants
	// every reachable adult — but the supervision sheet must tell "darf
	// abholen" apart from "im Notfall anrufen", and both answers come from the
	// same join.
	RelationshipType   *string
	PickupNotes        *string
	CanPickup          bool
	IsEmergencyContact bool
}
