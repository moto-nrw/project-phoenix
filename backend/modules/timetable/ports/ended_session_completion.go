package ports

// EndedSessionCompletionDependencies wire the completion. CareDays is
// optional: without it no child is spared the absent stamp, which is the
// behaviour that predates #1747 — never a silent skip.
type EndedSessionCompletionDependencies struct {
	Instances    InstanceRows
	Participants ParticipantRows
	CareDays     CareDays
}
