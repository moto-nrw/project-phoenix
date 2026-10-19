package application

import (
	"time"
)

func (a *StaffParentAnnouncement) IsPublished() bool {
	return AnnouncementIsPublished(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) IsLetter() bool {
	return AnnouncementIsLetter(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) ReachesContactsWithoutPortal() bool {
	return AnnouncementReachesContactsWithoutPortal(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) IsSystem() bool {
	return AnnouncementIsSystem(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) IsPoll() bool {
	return AnnouncementIsPoll(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) HasReminder() bool {
	return AnnouncementHasReminder(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) ReminderSent() bool {
	return AnnouncementReminderSent(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) ReminderBody() string {
	return AnnouncementReminderBody(StaffAnnouncementParentAnnouncementToValue(a))
}
func (a *StaffParentAnnouncement) AcceptsResponsesAt(t time.Time) bool {
	return AnnouncementAcceptsResponsesAt(StaffAnnouncementParentAnnouncementToValue(a), t)
}
func (a *StaffParentAnnouncement) IsDeclaration() bool {
	return AnnouncementIsDeclaration(StaffAnnouncementParentAnnouncementToValue(a))
}
func (v *StaffDeclarationVersion) CanonicalContent() []byte {
	return CanonicalDeclarationContent(StaffAnnouncementDeclarationVersionToValue(v))
}
func (v *StaffDeclarationSubmission) CanonicalRecord() []byte {
	return CanonicalDeclarationRecord((*DeclarationSubmission)(v))
}

const (
	StaffParentAnnouncementPriorityInfo               = ParentAnnouncementPriorityInfo
	StaffParentAnnouncementPriorityImportant          = ParentAnnouncementPriorityImportant
	StaffParentAnnouncementResponseNone               = ParentAnnouncementResponseNone
	StaffParentAnnouncementResponseSingleChoice       = ParentAnnouncementResponseSingleChoice
	StaffParentAnnouncementResponseMultiChoice        = ParentAnnouncementResponseMultiChoice
	StaffParentAnnouncementDeliveryStandard           = ParentAnnouncementDeliveryStandard
	StaffParentAnnouncementDeliveryLetter             = ParentAnnouncementDeliveryLetter
	StaffEmailAudiencePortalOnly                      = EmailAudiencePortalOnly
	StaffEmailAudienceAllContacts                     = EmailAudienceAllContacts
	StaffAnnouncementTargetSchoolAll                  = AnnouncementTargetSchoolAll
	StaffAnnouncementTargetClass                      = AnnouncementTargetClass
	StaffAnnouncementTargetGroup                      = AnnouncementTargetGroup
	StaffAnnouncementTargetActivityGroup              = AnnouncementTargetActivityGroup
	StaffAnnouncementTargetStudent                    = AnnouncementTargetStudent
	StaffAnnouncementTargetPendingEnrollment          = AnnouncementTargetPendingEnrollment
	StaffParentAnnouncementSystemKindCareCancellation = ParentAnnouncementSystemKindCareCancellation
	StaffDeclarationSignersAny                        = DeclarationSignersAny
	StaffDeclarationSignersAll                        = DeclarationSignersAll
	StaffDeclarationActionAgreed                      = DeclarationActionAgreed
	StaffDeclarationActionDeclined                    = DeclarationActionDeclined
	StaffDeclarationActionRevoked                     = DeclarationActionRevoked
	StaffDeclarationStateOpen                         = DeclarationStateOpen
	StaffDeclarationStatePartial                      = DeclarationStatePartial
	StaffDeclarationStateAgreed                       = DeclarationStateAgreed
	StaffDeclarationStateDeclined                     = DeclarationStateDeclined
	StaffDeclarationStateRevoked                      = DeclarationStateRevoked
	StaffDeclarationStateNoSigner                     = DeclarationStateNoSigner
	StaffDeclarationStateExpired                      = DeclarationStateExpired
)

func StaffValidAnnouncementResponseType(v string) bool  { return ValidAnnouncementResponseType(v) }
func StaffValidAnnouncementDeliveryMode(v string) bool  { return ValidAnnouncementDeliveryMode(v) }
func StaffValidAnnouncementEmailAudience(v string) bool { return ValidAnnouncementEmailAudience(v) }
func StaffValidAnnouncementPriority(v string) bool      { return ValidAnnouncementPriority(v) }
func StaffValidAnnouncementTargetType(v string) bool    { return ValidAnnouncementTargetType(v) }

const (
	StaffParentAnnouncementDeliveryDeclaration = ParentAnnouncementDeliveryDeclaration
	StaffDeclarationKindConsent                = DeclarationKindConsent
	StaffDeclarationMethodSimpleElectronic     = DeclarationMethodSimpleElectronic
)

func (c *StaffAnnouncementLetterChildStatus) Fulfilled() bool {
	return AnnouncementLetterFulfilled((*AnnouncementLetterChildStatus)(c))
}
func (c *StaffAnnouncementLetterChildStatus) Outstanding() bool {
	return AnnouncementLetterOutstanding((*AnnouncementLetterChildStatus)(c))
}
