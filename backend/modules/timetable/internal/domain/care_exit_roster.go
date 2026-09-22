package domain

// CareExitRosterRow is one planned participant a care exit removes or
// restores: the planning facts only. The attendance the participant carried
// is Student Presence's and travels beside the row in the caller's ledger.
type CareExitRosterRow struct {
	ParticipantID int64  `json:"participant_id"`
	TenantID      int64  `json:"tenant_id"`
	StudentID     int64  `json:"student_id"`
	InstanceID    int64  `json:"instance_id"`
	RoomID        *int64 `json:"room_id"`
}
