package announcement

import (
	values "github.com/moto-nrw/project-phoenix/modules/communication/internal/application"
)

func announcementAnnouncementPollResultsFromValue(v *values.AnnouncementPollResults) *AnnouncementPollResults {
	if v == nil {
		return nil
	}
	return &AnnouncementPollResults{
		ChildCount:       v.ChildCount,
		TargetChildCount: v.TargetChildCount,
		AnsweredCount:    v.AnsweredCount,
		Options: mapAnnouncementRows(values.AnnouncementPollOptionResultSliceFromAnnouncementRecords(v.Options), func(row *values.AnnouncementPollOptionResult) *AnnouncementPollOptionResult {
			return (*AnnouncementPollOptionResult)(row)
		}),
	}
}
func announcementDeclarationVersionToValue(v *DeclarationVersion) *values.DeclarationVersion {
	if v == nil {
		return nil
	}
	return &values.DeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments: values.DeclarationAttachmentDigestValuesToAnnouncementRecords(mapAnnouncementRows(v.Attachments, func(row DeclarationAttachmentDigest) values.DeclarationAttachmentDigest {
			return values.DeclarationAttachmentDigest(row)
		})),
		ContentHash: v.ContentHash,
		PublishedAt: v.PublishedAt,
	}
}
func announcementDeclarationVersionFromValue(v *values.DeclarationVersion) *DeclarationVersion {
	if v == nil {
		return nil
	}
	return &DeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments: mapAnnouncementRows(values.DeclarationAttachmentDigestValuesFromAnnouncementRecords(v.Attachments), func(row values.DeclarationAttachmentDigest) DeclarationAttachmentDigest {
			return DeclarationAttachmentDigest(row)
		}),
		ContentHash: v.ContentHash,
		PublishedAt: v.PublishedAt,
	}
}
func announcementParentAnnouncementToValue(v *ParentAnnouncement) *values.ParentAnnouncement {
	if v == nil {
		return nil
	}
	result := &values.ParentAnnouncement{
		ID:                      v.ID,
		CreatedAt:               v.CreatedAt,
		UpdatedAt:               v.UpdatedAt,
		TenantID:                v.TenantID,
		Title:                   v.Title,
		Body:                    v.Body,
		Priority:                v.Priority,
		LinkURL:                 v.LinkURL,
		RequiresAcknowledgement: v.RequiresAcknowledgement,
		SendEmail:               v.SendEmail,
		PublishedAt:             v.PublishedAt,
		ExpiresAt:               v.ExpiresAt,
		Active:                  v.Active,
		CreatedBy:               v.CreatedBy,
		ResponseType:            v.ResponseType,
		ResponseDeadline:        v.ResponseDeadline,
		DeliveryMode:            v.DeliveryMode,
		EmailAudience:           v.EmailAudience,
		SystemKind:              v.SystemKind,
		ReminderAt:              v.ReminderAt,
		ReminderText:            v.ReminderText,
		ReminderSentAt:          v.ReminderSentAt,
		Targets: values.ParentAnnouncementTargetSliceToAnnouncementRecords(mapAnnouncementRows(v.Targets, func(row *ParentAnnouncementTarget) *values.ParentAnnouncementTarget {
			return (*values.ParentAnnouncementTarget)(row)
		})),
		Options: values.ParentAnnouncementOptionSliceToAnnouncementRecords(mapAnnouncementRows(v.Options, func(row *ParentAnnouncementOption) *values.ParentAnnouncementOption {
			return (*values.ParentAnnouncementOption)(row)
		})),
		Declaration: *values.AnnouncementDeclarationSettingsToAnnouncementRecord((*values.AnnouncementDeclarationSettings)(&v.Declaration)),
	}
	result.Poll = values.AnnouncementIsPoll(result)
	result.DeclarationMode = values.AnnouncementIsDeclaration(result)
	return result
}
func announcementParentAnnouncementFromValue(v *values.ParentAnnouncement) *ParentAnnouncement {
	if v == nil {
		return nil
	}
	return &ParentAnnouncement{
		ID:                      v.ID,
		CreatedAt:               v.CreatedAt,
		UpdatedAt:               v.UpdatedAt,
		TenantID:                v.TenantID,
		Title:                   v.Title,
		Body:                    v.Body,
		Priority:                v.Priority,
		LinkURL:                 v.LinkURL,
		RequiresAcknowledgement: v.RequiresAcknowledgement,
		SendEmail:               v.SendEmail,
		PublishedAt:             v.PublishedAt,
		ExpiresAt:               v.ExpiresAt,
		Active:                  v.Active,
		CreatedBy:               v.CreatedBy,
		ResponseType:            v.ResponseType,
		ResponseDeadline:        v.ResponseDeadline,
		DeliveryMode:            v.DeliveryMode,
		EmailAudience:           v.EmailAudience,
		SystemKind:              v.SystemKind,
		ReminderAt:              v.ReminderAt,
		ReminderText:            v.ReminderText,
		ReminderSentAt:          v.ReminderSentAt,
		Targets: mapAnnouncementRows(values.ParentAnnouncementTargetSliceFromAnnouncementRecords(v.Targets), func(row *values.ParentAnnouncementTarget) *ParentAnnouncementTarget {
			return (*ParentAnnouncementTarget)(row)
		}),
		Options: mapAnnouncementRows(values.ParentAnnouncementOptionSliceFromAnnouncementRecords(v.Options), func(row *values.ParentAnnouncementOption) *ParentAnnouncementOption {
			return (*ParentAnnouncementOption)(row)
		}),
		Declaration: AnnouncementDeclarationSettings(*values.AnnouncementDeclarationSettingsFromAnnouncementRecord(&v.Declaration)),
	}
}
