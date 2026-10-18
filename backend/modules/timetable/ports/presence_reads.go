package ports

// LegacyInstanceFilter selects activity instances the way the retained
// repositories ask for them. Status uses the legacy vocabulary: planned is a
// plan nobody started, active and completed are session states, cancelled is
// a plan state.
type LegacyInstanceFilter struct {
	IDs                 []int64
	Date                *string
	Dates               []string
	FromDate            *string
	ToDate              *string
	ActivityGroupID     *int64
	ActivityGroupIDs    []int64
	ActiveGroupIDs      []int64
	Status              string
	IsSpontaneous       *bool
	IdempotencyKey      string
	MaterializedPlanned bool
	OrderByDateAndTime  bool
	Limit               int
	Offset              int
}

// LegacyParticipantFilter selects participants with their attendance the way
// the retained repositories ask for them. Status filters the attendance
// status; ExcludeFinished drops participants of cancelled or ended blocks.
type LegacyParticipantFilter struct {
	IDs                        []int64
	InstanceIDs                []int64
	StudentIDs                 []int64
	Status                     *string
	Date                       *string
	FromDate                   *string
	ToDate                     *string
	CurrentTime                *string
	ExcludeFinished            bool
	OrderByCreated             bool
	OrderByInstanceStudent     bool
	OrderByStudentActivityTime bool
	OrderByActivityDateTime    bool
	Limit                      int
	Offset                     int
}
