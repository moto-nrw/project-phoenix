package peopledirectory

type GuardianProfileWithChildren struct {
	Profile      *GuardianProfileSnapshot
	PrimaryPhone string
	Children     []GuardianChildSummary
}
