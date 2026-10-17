package parentstore

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func portalFromRetainedAnnouncementFeedItem(v models.AnnouncementFeedItem) communication.PortalAnnouncementFeedItem {
	return communication.PortalAnnouncementFeedItem{
		ID:                      v.ID,
		TenantID:                v.TenantID,
		Title:                   v.Title,
		Body:                    v.Body,
		Priority:                v.Priority,
		LinkURL:                 v.LinkURL,
		RequiresAcknowledgement: v.RequiresAcknowledgement,
		PublishedAt:             v.PublishedAt,
		ExpiresAt:               v.ExpiresAt,
		ResponseType:            v.ResponseType,
		DeliveryMode:            v.DeliveryMode,
		ResponseDeadline:        v.ResponseDeadline,
		SystemKind:              v.SystemKind,
		SchoolName:              v.SchoolName,
		ReadAt:                  v.ReadAt,
		AcknowledgedAt:          v.AcknowledgedAt,
		ReminderSentAt:          v.ReminderSentAt,
		ReminderText:            v.ReminderText,
		Options: portalRecordSlice(v.Options, func(row *models.ParentAnnouncementOption) *communication.PortalParentAnnouncementOption {
			return portalRecordPointer(row, portalFromRetainedParentAnnouncementOption)
		}),
		Children: portalRecordSlice(v.Children, func(row *models.AnnouncementPollChild) *communication.PortalAnnouncementPollChild {
			return portalRecordPointer(row, portalFromRetainedAnnouncementPollChild)
		}),
		Declaration: portalRecordPointer(v.Declaration, portalFromRetainedAnnouncementFeedDeclaration),
		Poll:        v.IsPoll(),
	}
}

func portalToRetainedAnnouncementFeedScope(v communication.PortalAnnouncementFeedScope) models.AnnouncementFeedScope {
	return models.AnnouncementFeedScope{
		TenantIDs:           v.TenantIDs,
		SystemOnlyTenantIDs: v.SystemOnlyTenantIDs,
	}
}
func portalFromRetainedAnnouncementPollChild(v models.AnnouncementPollChild) communication.PortalAnnouncementPollChild {
	return communication.PortalAnnouncementPollChild{
		AnnouncementID:  v.AnnouncementID,
		StudentID:       v.StudentID,
		FirstName:       v.FirstName,
		LastName:        v.LastName,
		SelectedOptions: v.SelectedOptions,
	}
}

func portalFromRetainedParentAnnouncement(v models.ParentAnnouncement) communication.PortalParentAnnouncement {
	return communication.PortalParentAnnouncement{
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
		Targets: portalRecordSlice(v.Targets, func(row *models.ParentAnnouncementTarget) *communication.PortalParentAnnouncementTarget {
			return portalRecordPointer(row, portalFromRetainedParentAnnouncementTarget)
		}),
		Options: portalRecordSlice(v.Options, func(row *models.ParentAnnouncementOption) *communication.PortalParentAnnouncementOption {
			return portalRecordPointer(row, portalFromRetainedParentAnnouncementOption)
		}),
		Declaration:     portalFromRetainedAnnouncementDeclarationSettings(v.Declaration),
		Poll:            v.IsPoll(),
		DeclarationMode: v.IsDeclaration(),
	}
}

func portalFromRetainedParentAnnouncementOption(v models.ParentAnnouncementOption) communication.PortalParentAnnouncementOption {
	return communication.PortalParentAnnouncementOption{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		Label:          v.Label,
		Position:       v.Position,
		CreatedAt:      v.CreatedAt,
	}
}

func portalFromRetainedParentAnnouncementTarget(v models.ParentAnnouncementTarget) communication.PortalParentAnnouncementTarget {
	return communication.PortalParentAnnouncementTarget{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		TargetType:     v.TargetType,
		TargetRefID:    v.TargetRefID,
		TargetRefText:  v.TargetRefText,
		CreatedAt:      v.CreatedAt,
	}
}
