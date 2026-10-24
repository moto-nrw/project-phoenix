package application

import "github.com/moto-nrw/project-phoenix/modules/communication"

// Group thread vocabulary (#4784) for the messaging adapters, which reach
// the public contract only through the application layer.
type (
	ParentGroupTarget           = communication.ParentGroupTarget
	ParentGroupThread           = communication.ParentGroupThread
	ParentGroupMessage          = communication.ParentGroupMessage
	ParentGroupRecipient        = communication.ParentGroupRecipient
	ParentGroupChildRead        = communication.ParentGroupChildRead
	ParentGroupThreadSummary    = communication.ParentGroupThreadSummary
	ParentGroupChild            = communication.ParentGroupChild
	ParentGroupThreadDetail     = communication.ParentGroupThreadDetail
	ParentGroupThreadReadCounts = communication.ParentGroupThreadReadCounts
	ParentGroupThreadInput      = communication.ParentGroupThreadInput
	ParentGroupAudienceChild    = communication.ParentGroupAudienceChild
	ParentGroupAudienceReads    = communication.ParentGroupAudienceReads
)

const (
	ParentGroupTargetClass         = communication.ParentGroupTargetClass
	ParentGroupTargetGroup         = communication.ParentGroupTargetGroup
	ParentGroupTargetActivityGroup = communication.ParentGroupTargetActivityGroup
)

var (
	ErrParentGroupThreadNotFound     = communication.ErrParentGroupThreadNotFound
	ErrParentGroupThreadClosed       = communication.ErrParentGroupThreadClosed
	ErrParentGroupThreadInvalid      = communication.ErrParentGroupThreadInvalid
	ErrParentGroupThreadNoRecipients = communication.ErrParentGroupThreadNoRecipients
	ErrParentGroupMessagingForbidden = communication.ErrParentMessagingForbidden
	ErrParentGroupMessagingDisabled  = communication.ErrParentMessagingDisabled
	ErrParentGroupMessageEmptyBody   = communication.ErrParentMessageEmptyBody
	ErrParentGroupMessageTooLong     = communication.ErrParentMessageBodyTooLong
)
