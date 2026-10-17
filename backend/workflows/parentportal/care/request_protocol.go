package care

// Values are the unchanged request/ledger storage and response protocol.
const (
	DataChangeStatusAutoApplied     = "auto_applied"
	DataChangeStatusPending         = "pending"
	DataChangeTargetDeparture       = "departure"
	DataChangeTargetGuardianPhone   = "guardian_phone"
	DataChangeTargetGuardianProfile = "guardian_profile"
	DataChangeTargetPerson          = "person"
	DataChangeTargetStudent         = "student"
	ParentMessageRequestMasterData  = "master_data"
	ParentMessageRequestStatusOpen  = "offen"
	ParentMessageSenderGuardian     = "guardian"
	ParentRequestEventGuardianEdit  = "guardian_edited"
	ParentRequestEventSubmitted     = "submitted"
	ParentRequestTypeMasterData     = "master_data"
)
