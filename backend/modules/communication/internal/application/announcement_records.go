package application

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

type AnnouncementDeclarationSettings communication.PortalAnnouncementDeclarationSettings
type AnnouncementDeliveryRecipient communication.PortalAnnouncementDeliveryRecipient
type AnnouncementLetterChildStatus communication.PortalAnnouncementLetterChildStatus
type AnnouncementPollChildStatus communication.PortalAnnouncementPollChildStatus
type AnnouncementPollOptionResult communication.PortalAnnouncementPollOptionResult
type AnnouncementPollReminderRecipient communication.PortalAnnouncementPollReminderRecipient
type AnnouncementPollResults communication.PortalAnnouncementPollResults
type AnnouncementRecipient communication.PortalAnnouncementRecipient
type AnnouncementRecipientStatus communication.PortalAnnouncementRecipientStatus
type AnnouncementStats communication.PortalAnnouncementStats
type DeclarationAttachmentDigest communication.PortalDeclarationAttachmentDigest
type DeclarationChild communication.PortalDeclarationChild
type DeclarationSigner communication.PortalDeclarationSigner
type DeclarationSubmission communication.PortalDeclarationSubmission
type DeclarationVersion communication.PortalDeclarationVersion
type ParentAnnouncement communication.PortalParentAnnouncement
type ParentAnnouncementOption communication.PortalParentAnnouncementOption
type ParentAnnouncementTarget communication.PortalParentAnnouncementTarget
type StaffAnnouncementSource interface {
	communication.StaffAnnouncementRecords
}

func AnnouncementPollResultsFromAnnouncementRecord(v *communication.PortalAnnouncementPollResults) *AnnouncementPollResults {
	return (*AnnouncementPollResults)(v)
}

func AnnouncementStatsFromAnnouncementRecord(v *communication.PortalAnnouncementStats) *AnnouncementStats {
	return (*AnnouncementStats)(v)
}

func DeclarationVersionFromAnnouncementRecord(v *communication.PortalDeclarationVersion) *DeclarationVersion {
	return (*DeclarationVersion)(v)
}
func ParentAnnouncementFromAnnouncementRecord(v *communication.PortalParentAnnouncement) *ParentAnnouncement {
	return (*ParentAnnouncement)(v)
}

func DeclarationVersionToAnnouncementRecord(v *DeclarationVersion) *communication.PortalDeclarationVersion {
	return (*communication.PortalDeclarationVersion)(v)
}
func ParentAnnouncementToAnnouncementRecord(v *ParentAnnouncement) *communication.PortalParentAnnouncement {
	return (*communication.PortalParentAnnouncement)(v)
}

func DeclarationSubmissionSliceFromAnnouncementRecords(rows []*communication.PortalDeclarationSubmission) []*DeclarationSubmission {
	if rows == nil {
		return nil
	}
	out := make([]*DeclarationSubmission, len(rows))
	for i, row := range rows {
		out[i] = (*DeclarationSubmission)(row)
	}
	return out
}
func ParentAnnouncementSliceFromAnnouncementRecords(rows []*communication.PortalParentAnnouncement) []*ParentAnnouncement {
	if rows == nil {
		return nil
	}
	out := make([]*ParentAnnouncement, len(rows))
	for i, row := range rows {
		out[i] = (*ParentAnnouncement)(row)
	}
	return out
}
func AnnouncementRecipientStatusSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementRecipientStatus) []*AnnouncementRecipientStatus {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementRecipientStatus, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementRecipientStatus)(row)
	}
	return out
}
func ParentAnnouncementOptionSliceToAnnouncementRecords(rows []*ParentAnnouncementOption) []*communication.PortalParentAnnouncementOption {
	if rows == nil {
		return nil
	}
	out := make([]*communication.PortalParentAnnouncementOption, len(rows))
	for i, row := range rows {
		out[i] = (*communication.PortalParentAnnouncementOption)(row)
	}
	return out
}
func ParentAnnouncementTargetSliceToAnnouncementRecords(rows []*ParentAnnouncementTarget) []*communication.PortalParentAnnouncementTarget {
	if rows == nil {
		return nil
	}
	out := make([]*communication.PortalParentAnnouncementTarget, len(rows))
	for i, row := range rows {
		out[i] = (*communication.PortalParentAnnouncementTarget)(row)
	}
	return out
}
func DeclarationSignerSliceFromAnnouncementRecords(rows []*communication.PortalDeclarationSigner) []*DeclarationSigner {
	if rows == nil {
		return nil
	}
	out := make([]*DeclarationSigner, len(rows))
	for i, row := range rows {
		out[i] = (*DeclarationSigner)(row)
	}
	return out
}
func AnnouncementLetterChildStatusSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementLetterChildStatus) []*AnnouncementLetterChildStatus {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementLetterChildStatus, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementLetterChildStatus)(row)
	}
	return out
}
func DeclarationChildSliceFromAnnouncementRecords(rows []*communication.PortalDeclarationChild) []*DeclarationChild {
	if rows == nil {
		return nil
	}
	out := make([]*DeclarationChild, len(rows))
	for i, row := range rows {
		out[i] = (*DeclarationChild)(row)
	}
	return out
}
func ParentAnnouncementOptionSliceFromAnnouncementRecords(rows []*communication.PortalParentAnnouncementOption) []*ParentAnnouncementOption {
	if rows == nil {
		return nil
	}
	out := make([]*ParentAnnouncementOption, len(rows))
	for i, row := range rows {
		out[i] = (*ParentAnnouncementOption)(row)
	}
	return out
}
func DeclarationVersionSliceFromAnnouncementRecords(rows []*communication.PortalDeclarationVersion) []*DeclarationVersion {
	if rows == nil {
		return nil
	}
	out := make([]*DeclarationVersion, len(rows))
	for i, row := range rows {
		out[i] = (*DeclarationVersion)(row)
	}
	return out
}
func AnnouncementPollReminderRecipientSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementPollReminderRecipient) []*AnnouncementPollReminderRecipient {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementPollReminderRecipient, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementPollReminderRecipient)(row)
	}
	return out
}
func AnnouncementRecipientSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementRecipient) []*AnnouncementRecipient {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementRecipient, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementRecipient)(row)
	}
	return out
}
func ParentAnnouncementTargetSliceFromAnnouncementRecords(rows []*communication.PortalParentAnnouncementTarget) []*ParentAnnouncementTarget {
	if rows == nil {
		return nil
	}
	out := make([]*ParentAnnouncementTarget, len(rows))
	for i, row := range rows {
		out[i] = (*ParentAnnouncementTarget)(row)
	}
	return out
}
func AnnouncementPollChildStatusSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementPollChildStatus) []*AnnouncementPollChildStatus {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementPollChildStatus, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementPollChildStatus)(row)
	}
	return out
}
func AnnouncementDeliveryRecipientSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementDeliveryRecipient) []*AnnouncementDeliveryRecipient {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementDeliveryRecipient, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementDeliveryRecipient)(row)
	}
	return out
}

type AnnouncementFeedItem communication.PortalAnnouncementFeedItem
type AnnouncementFeedScope communication.PortalAnnouncementFeedScope
type AnnouncementPollChild communication.PortalAnnouncementPollChild

func AnnouncementPollOptionResultSliceFromAnnouncementRecords(rows []*communication.PortalAnnouncementPollOptionResult) []*AnnouncementPollOptionResult {
	if rows == nil {
		return nil
	}
	out := make([]*AnnouncementPollOptionResult, len(rows))
	for i, row := range rows {
		out[i] = (*AnnouncementPollOptionResult)(row)
	}
	return out
}

func DeclarationAttachmentDigestValuesToAnnouncementRecords(rows []DeclarationAttachmentDigest) []communication.PortalDeclarationAttachmentDigest {
	if rows == nil {
		return nil
	}
	out := make([]communication.PortalDeclarationAttachmentDigest, len(rows))
	for i, row := range rows {
		out[i] = communication.PortalDeclarationAttachmentDigest(row)
	}
	return out
}

func DeclarationAttachmentDigestValuesFromAnnouncementRecords(rows []communication.PortalDeclarationAttachmentDigest) []DeclarationAttachmentDigest {
	if rows == nil {
		return nil
	}
	out := make([]DeclarationAttachmentDigest, len(rows))
	for i, row := range rows {
		out[i] = DeclarationAttachmentDigest(row)
	}
	return out
}
func AnnouncementDeclarationSettingsToAnnouncementRecord(v *AnnouncementDeclarationSettings) *communication.PortalAnnouncementDeclarationSettings {
	return (*communication.PortalAnnouncementDeclarationSettings)(v)
}
func AnnouncementDeclarationSettingsFromAnnouncementRecord(v *communication.PortalAnnouncementDeclarationSettings) *AnnouncementDeclarationSettings {
	return (*AnnouncementDeclarationSettings)(v)
}
