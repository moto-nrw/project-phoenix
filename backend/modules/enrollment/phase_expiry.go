package enrollment

type PhaseExpiryInput struct {
	AsOf            Date
	WarningThrough  Date
	OfferingsJSON   string
	BookingsJSON    string
	StudentIDs      []int64
	StudentStatuses []string
	EnrolledFrom    []string
	EnrolledUntil   []string
}

type PhaseExpirySnapshot struct {
	SourcePhaseID      int64
	SourcePhaseName    string
	SuccessorPhaseID   *int64
	SuccessorPhaseName *string
	FirstAffectedDate  Date
	AffectedChildren   int
	UnresolvedChildren int
}
