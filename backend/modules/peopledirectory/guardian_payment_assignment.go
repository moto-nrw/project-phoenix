package peopledirectory

type GuardianPaymentAssignment struct {
	StudentID           int64
	StudentFirstName    string
	StudentLastName     string
	SchoolClass         string
	GuardianProfileID   *int64
	GuardianFirstName   string
	GuardianLastName    string
	RelationshipTypeRaw string
}
