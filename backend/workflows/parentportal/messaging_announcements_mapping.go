package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

func messagingToAppAnnouncementFeedItem(v AnnouncementFeedItem) messaging.AnnouncementFeedItem {
	return messaging.AnnouncementFeedItem{
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
		Options: messagingValueSlice(v.Options, func(row *ParentAnnouncementOption) *messaging.ParentAnnouncementOption {
			return messagingValuePointer(row, messagingToAppParentAnnouncementOption)
		}),
		Children: messagingValueSlice(v.Children, func(row *AnnouncementPollChild) *messaging.AnnouncementPollChild {
			return messagingValuePointer(row, messagingToAppAnnouncementPollChild)
		}),
		Declaration: messagingValuePointer(v.Declaration, messagingToAppAnnouncementFeedDeclaration),
		Poll:        v.Poll,
	}
}
func messagingFromAppAnnouncementFeedItem(v messaging.AnnouncementFeedItem) AnnouncementFeedItem {
	return AnnouncementFeedItem{
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
		Options: messagingValueSlice(v.Options, func(row *messaging.ParentAnnouncementOption) *ParentAnnouncementOption {
			return messagingValuePointer(row, messagingFromAppParentAnnouncementOption)
		}),
		Children: messagingValueSlice(v.Children, func(row *messaging.AnnouncementPollChild) *AnnouncementPollChild {
			return messagingValuePointer(row, messagingFromAppAnnouncementPollChild)
		}),
		Declaration: messagingValuePointer(v.Declaration, messagingFromAppAnnouncementFeedDeclaration),
		Poll:        v.Poll,
	}
}

func messagingFromAppAnnouncementFeedScope(v messaging.AnnouncementFeedScope) AnnouncementFeedScope {
	return AnnouncementFeedScope{
		TenantIDs:           v.TenantIDs,
		SystemOnlyTenantIDs: v.SystemOnlyTenantIDs,
	}
}
func messagingToAppAnnouncementPollChild(v AnnouncementPollChild) messaging.AnnouncementPollChild {
	return messaging.AnnouncementPollChild{
		AnnouncementID:  v.AnnouncementID,
		StudentID:       v.StudentID,
		FirstName:       v.FirstName,
		LastName:        v.LastName,
		SelectedOptions: v.SelectedOptions,
	}
}
func messagingFromAppAnnouncementPollChild(v messaging.AnnouncementPollChild) AnnouncementPollChild {
	return AnnouncementPollChild{
		AnnouncementID:  v.AnnouncementID,
		StudentID:       v.StudentID,
		FirstName:       v.FirstName,
		LastName:        v.LastName,
		SelectedOptions: v.SelectedOptions,
	}
}
func messagingToAppParentAnnouncement(v ParentAnnouncement) messaging.ParentAnnouncement {
	return messaging.ParentAnnouncement{
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
		Targets: messagingValueSlice(v.Targets, func(row *ParentAnnouncementTarget) *messaging.ParentAnnouncementTarget {
			return messagingValuePointer(row, messagingToAppParentAnnouncementTarget)
		}),
		Options: messagingValueSlice(v.Options, func(row *ParentAnnouncementOption) *messaging.ParentAnnouncementOption {
			return messagingValuePointer(row, messagingToAppParentAnnouncementOption)
		}),
		Declaration:     messagingToAppAnnouncementDeclarationSettings(v.Declaration),
		Poll:            v.Poll,
		DeclarationMode: v.DeclarationMode,
	}
}

func messagingToAppParentAnnouncementOption(v ParentAnnouncementOption) messaging.ParentAnnouncementOption {
	return messaging.ParentAnnouncementOption{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		Label:          v.Label,
		Position:       v.Position,
		CreatedAt:      v.CreatedAt,
	}
}
func messagingFromAppParentAnnouncementOption(v messaging.ParentAnnouncementOption) ParentAnnouncementOption {
	return ParentAnnouncementOption{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		Label:          v.Label,
		Position:       v.Position,
		CreatedAt:      v.CreatedAt,
	}
}
func messagingToAppParentAnnouncementTarget(v ParentAnnouncementTarget) messaging.ParentAnnouncementTarget {
	return messaging.ParentAnnouncementTarget{
		TenantID:       v.TenantID,
		ID:             v.ID,
		AnnouncementID: v.AnnouncementID,
		TargetType:     v.TargetType,
		TargetRefID:    v.TargetRefID,
		TargetRefText:  v.TargetRefText,
		CreatedAt:      v.CreatedAt,
	}
}
