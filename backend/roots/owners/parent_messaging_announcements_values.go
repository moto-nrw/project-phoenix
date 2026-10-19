package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentMessagingFromOwnerAnnouncementFeedItem(v communication.PortalAnnouncementFeedItem) parentportal.AnnouncementFeedItem {
	return parentportal.AnnouncementFeedItem{
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
		Options: ParentMessagingSlice(v.Options, func(row *communication.PortalParentAnnouncementOption) *parentportal.ParentAnnouncementOption {
			return ParentMessagingPointer(row, parentMessagingFromOwnerParentAnnouncementOption)
		}),
		Children: ParentMessagingSlice(v.Children, func(row *communication.PortalAnnouncementPollChild) *parentportal.AnnouncementPollChild {
			return ParentMessagingPointer(row, parentMessagingFromOwnerAnnouncementPollChild)
		}),
		Declaration: ParentMessagingPointer(v.Declaration, parentMessagingFromOwnerAnnouncementFeedDeclaration),
		Poll:        v.Poll,
	}
}

func parentMessagingToOwnerAnnouncementFeedScope(v parentportal.AnnouncementFeedScope) communication.PortalAnnouncementFeedScope {
	return communication.PortalAnnouncementFeedScope{
		TenantIDs:           v.TenantIDs,
		SystemOnlyTenantIDs: v.SystemOnlyTenantIDs,
	}
}

func parentMessagingFromOwnerAnnouncementPollChild(v communication.PortalAnnouncementPollChild) parentportal.AnnouncementPollChild {
	return parentportal.AnnouncementPollChild{
		AnnouncementID:  v.AnnouncementID,
		StudentID:       v.StudentID,
		FirstName:       v.FirstName,
		LastName:        v.LastName,
		SelectedOptions: v.SelectedOptions,
	}
}

func parentMessagingFromOwnerParentAnnouncement(v communication.PortalParentAnnouncement) parentportal.ParentAnnouncement {
	return parentportal.ParentAnnouncement{
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
		Targets: ParentMessagingSlice(v.Targets, func(row *communication.PortalParentAnnouncementTarget) *parentportal.ParentAnnouncementTarget {
			return ParentMessagingPointer(row, parentMessagingFromOwnerParentAnnouncementTarget)
		}),
		Options: ParentMessagingSlice(v.Options, func(row *communication.PortalParentAnnouncementOption) *parentportal.ParentAnnouncementOption {
			return ParentMessagingPointer(row, parentMessagingFromOwnerParentAnnouncementOption)
		}),
		Declaration:     parentMessagingFromOwnerAnnouncementDeclarationSettings(v.Declaration),
		Poll:            v.Poll,
		DeclarationMode: v.DeclarationMode,
	}
}

func parentMessagingFromOwnerParentAnnouncementOption(v communication.PortalParentAnnouncementOption) parentportal.ParentAnnouncementOption {
	return parentportal.ParentAnnouncementOption{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		Label:          v.Label,
		Position:       v.Position,
		CreatedAt:      v.CreatedAt,
	}
}

func parentMessagingFromOwnerParentAnnouncementTarget(v communication.PortalParentAnnouncementTarget) parentportal.ParentAnnouncementTarget {
	return parentportal.ParentAnnouncementTarget{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		TargetType:     v.TargetType,
		TargetRefID:    v.TargetRefID,
		TargetRefText:  v.TargetRefText,
		CreatedAt:      v.CreatedAt,
	}
}
