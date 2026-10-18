package announcement

import (
	"time"

	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func (a *ParentAnnouncement) IsPublished() bool {
	return values.AnnouncementIsPublished(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) IsLetter() bool {
	return values.AnnouncementIsLetter(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) ReachesContactsWithoutPortal() bool {
	return values.AnnouncementReachesContactsWithoutPortal(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) IsSystem() bool {
	return values.AnnouncementIsSystem(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) IsPoll() bool {
	return values.AnnouncementIsPoll(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) HasReminder() bool {
	return values.AnnouncementHasReminder(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) ReminderSent() bool {
	return values.AnnouncementReminderSent(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) ReminderBody() string {
	return values.AnnouncementReminderBody(announcementParentAnnouncementToValue(a))
}
func (a *ParentAnnouncement) AcceptsResponsesAt(t time.Time) bool {
	return values.AnnouncementAcceptsResponsesAt(announcementParentAnnouncementToValue(a), t)
}
func (a *ParentAnnouncement) IsDeclaration() bool {
	return values.AnnouncementIsDeclaration(announcementParentAnnouncementToValue(a))
}
func (v *DeclarationVersion) CanonicalContent() []byte {
	return values.CanonicalDeclarationContent(announcementDeclarationVersionToValue(v))
}
func (v *DeclarationSubmission) CanonicalRecord() []byte {
	return values.CanonicalDeclarationRecord((*values.DeclarationSubmission)(v))
}

const (
	ParentAnnouncementPriorityInfo               = values.ParentAnnouncementPriorityInfo
	ParentAnnouncementPriorityImportant          = values.ParentAnnouncementPriorityImportant
	ParentAnnouncementResponseNone               = values.ParentAnnouncementResponseNone
	ParentAnnouncementResponseSingleChoice       = values.ParentAnnouncementResponseSingleChoice
	ParentAnnouncementResponseMultiChoice        = values.ParentAnnouncementResponseMultiChoice
	ParentAnnouncementDeliveryStandard           = values.ParentAnnouncementDeliveryStandard
	ParentAnnouncementDeliveryLetter             = values.ParentAnnouncementDeliveryLetter
	EmailAudiencePortalOnly                      = values.EmailAudiencePortalOnly
	EmailAudienceAllContacts                     = values.EmailAudienceAllContacts
	AnnouncementTargetSchoolAll                  = values.AnnouncementTargetSchoolAll
	AnnouncementTargetClass                      = values.AnnouncementTargetClass
	AnnouncementTargetGroup                      = values.AnnouncementTargetGroup
	AnnouncementTargetActivityGroup              = values.AnnouncementTargetActivityGroup
	AnnouncementTargetStudent                    = values.AnnouncementTargetStudent
	AnnouncementTargetPendingEnrollment          = values.AnnouncementTargetPendingEnrollment
	ParentAnnouncementSystemKindCareCancellation = values.ParentAnnouncementSystemKindCareCancellation
	DeclarationSignersAny                        = values.DeclarationSignersAny
	DeclarationSignersAll                        = values.DeclarationSignersAll
	DeclarationActionAgreed                      = values.DeclarationActionAgreed
	DeclarationActionDeclined                    = values.DeclarationActionDeclined
	DeclarationActionRevoked                     = values.DeclarationActionRevoked
	DeclarationStateOpen                         = values.DeclarationStateOpen
	DeclarationStatePartial                      = values.DeclarationStatePartial
	DeclarationStateAgreed                       = values.DeclarationStateAgreed
	DeclarationStateDeclined                     = values.DeclarationStateDeclined
	DeclarationStateRevoked                      = values.DeclarationStateRevoked
	DeclarationStateNoSigner                     = values.DeclarationStateNoSigner
	DeclarationStateExpired                      = values.DeclarationStateExpired
)

func ValidAnnouncementResponseType(v string) bool  { return values.ValidAnnouncementResponseType(v) }
func ValidAnnouncementDeliveryMode(v string) bool  { return values.ValidAnnouncementDeliveryMode(v) }
func ValidAnnouncementEmailAudience(v string) bool { return values.ValidAnnouncementEmailAudience(v) }
func ValidAnnouncementPriority(v string) bool      { return values.ValidAnnouncementPriority(v) }
func ValidAnnouncementTargetType(v string) bool    { return values.ValidAnnouncementTargetType(v) }

const (
	ParentAnnouncementDeliveryDeclaration = values.ParentAnnouncementDeliveryDeclaration
	DeclarationKindConsent                = values.DeclarationKindConsent
	DeclarationMethodSimpleElectronic     = values.DeclarationMethodSimpleElectronic
)

func (c *AnnouncementLetterChildStatus) Fulfilled() bool {
	return values.AnnouncementLetterFulfilled((*values.AnnouncementLetterChildStatus)(c))
}
func (c *AnnouncementLetterChildStatus) Outstanding() bool {
	return values.AnnouncementLetterOutstanding((*values.AnnouncementLetterChildStatus)(c))
}
