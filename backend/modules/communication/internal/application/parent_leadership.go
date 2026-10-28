package application

import "github.com/moto-nrw/project-phoenix/modules/communication"

// The leadership conversation vocabulary (#5200) the staff side uses through
// this package; the values are Communication's public contract.
type (
	StaffReach                   = communication.ParentMessageStaffReach
	ParentMessageLeader          = communication.ParentMessageLeader
	ParentMessageLeaderDirectory = communication.ParentMessageLeaderDirectory
	ParentMessageReaderPreview   = communication.ParentMessageReaderPreview
	ParentMessageReaders         = communication.ParentMessageReaders
	ParentMessageGuardianReader  = communication.ParentMessageGuardianReader
	ParentMessageSchoolReaders   = communication.ParentMessageSchoolReaders
)

const (
	ParentAudienceTeam       = communication.ParentMessageAudienceTeam
	ParentAudienceLeadership = communication.ParentMessageAudienceLeadership
)

var (
	ErrParentMessageInvalidAudience       = communication.ErrParentMessageInvalidAudience
	ErrParentMessageLeadershipUnavailable = communication.ErrParentMessageLeadershipUnavailable

	NormalizeParentMessageAudience       = communication.NormalizeParentMessageAudience
	ParentMessageSchoolReadersKind       = communication.ParentMessageSchoolReadersKind
	ParentMessageLeaderShortNames        = communication.ParentMessageLeaderShortNames
	ResolveParentMessageLeadershipChoice = communication.ResolveParentMessageLeadershipChoice
)
