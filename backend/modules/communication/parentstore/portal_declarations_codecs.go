package parentstore

import (
	models "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/modules/communication"
)

func portalFromRetainedAnnouncementDeclarationSettings(v models.AnnouncementDeclarationSettings) communication.PortalAnnouncementDeclarationSettings {
	return communication.PortalAnnouncementDeclarationSettings{
		Kind:             v.Kind,
		Signers:          v.Signers,
		Revocable:        v.Revocable,
		RequiresPassword: v.RequiresPassword,
	}
}

func portalFromRetainedAnnouncementFeedDeclaration(v models.AnnouncementFeedDeclaration) communication.PortalAnnouncementFeedDeclaration {
	return communication.PortalAnnouncementFeedDeclaration{
		Kind:             v.Kind,
		Signers:          v.Signers,
		Revocable:        v.Revocable,
		RequiresPassword: v.RequiresPassword,
		Deadline:         v.Deadline,
		Closed:           v.Closed,
		Version:          portalRecordPointer(v.Version, portalFromRetainedDeclarationVersion),
		Children: portalRecordSlice(v.Children, func(row *models.AnnouncementFeedDeclarationChild) *communication.PortalAnnouncementFeedDeclarationChild {
			return portalRecordPointer(row, portalFromRetainedAnnouncementFeedDeclarationChild)
		}),
	}
}

func portalFromRetainedAnnouncementFeedDeclarationChild(v models.AnnouncementFeedDeclarationChild) communication.PortalAnnouncementFeedDeclarationChild {
	return communication.PortalAnnouncementFeedDeclarationChild{
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		CanSubmit:      v.CanSubmit,
		State:          v.State,
		MyAction:       v.MyAction,
		MySubmittedAt:  v.MySubmittedAt,
		AllowedActions: v.AllowedActions,
		OtherSigners:   portalRecordSlice(v.OtherSigners, portalFromRetainedDeclarationSignerState),
	}
}

func portalFromRetainedDeclarationAttachmentDigest(v models.DeclarationAttachmentDigest) communication.PortalDeclarationAttachmentDigest {
	return communication.PortalDeclarationAttachmentDigest{
		AttachmentID: v.AttachmentID,
		Filename:     v.Filename,
		ContentType:  v.ContentType,
		SizeBytes:    v.SizeBytes,
		SHA256:       v.SHA256,
	}
}
func portalToRetainedDeclarationAttachmentDigest(v communication.PortalDeclarationAttachmentDigest) models.DeclarationAttachmentDigest {
	return models.DeclarationAttachmentDigest{
		AttachmentID: v.AttachmentID,
		Filename:     v.Filename,
		ContentType:  v.ContentType,
		SizeBytes:    v.SizeBytes,
		SHA256:       v.SHA256,
	}
}
func portalFromRetainedDeclarationChild(v models.DeclarationChild) communication.PortalDeclarationChild {
	return communication.PortalDeclarationChild{
		AnnouncementID: v.AnnouncementID,
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		SchoolClass:    v.SchoolClass,
	}
}

func portalFromRetainedDeclarationSigner(v models.DeclarationSigner) communication.PortalDeclarationSigner {
	return communication.PortalDeclarationSigner{
		StudentID:         v.StudentID,
		GuardianProfileID: v.GuardianProfileID,
		AccountID:         v.AccountID,
		FirstName:         v.FirstName,
		LastName:          v.LastName,
		Email:             v.Email,
		PortalLocale:      v.PortalLocale,
		GuardianRole:      v.GuardianRole,
	}
}

func portalFromRetainedDeclarationSignerContext(v models.DeclarationSignerContext) communication.PortalDeclarationSignerContext {
	return communication.PortalDeclarationSignerContext{
		GuardianProfileID: v.GuardianProfileID,
		FirstName:         v.FirstName,
		LastName:          v.LastName,
		GuardianRole:      v.GuardianRole,
	}
}

func portalFromRetainedDeclarationSignerState(v models.DeclarationSignerState) communication.PortalDeclarationSignerState {
	return communication.PortalDeclarationSignerState{
		AccountID:   v.AccountID,
		FirstName:   v.FirstName,
		LastName:    v.LastName,
		Action:      v.Action,
		SubmittedAt: v.SubmittedAt,
	}
}

func portalFromRetainedDeclarationSubmission(v models.DeclarationSubmission) communication.PortalDeclarationSubmission {
	return communication.PortalDeclarationSubmission{
		ID:                v.ID,
		TenantID:          v.TenantID,
		AnnouncementID:    v.AnnouncementID,
		VersionID:         v.VersionID,
		VersionNo:         v.VersionNo,
		StudentID:         v.StudentID,
		AccountID:         v.AccountID,
		GuardianProfileID: v.GuardianProfileID,
		SignerName:        v.SignerName,
		GuardianRole:      v.GuardianRole,
		Action:            v.Action,
		Method:            v.Method,
		PasswordConfirmed: v.PasswordConfirmed,
		ContentHash:       v.ContentHash,
		RecordHash:        v.RecordHash,
		SubmittedAt:       v.SubmittedAt,
	}
}
func portalToRetainedDeclarationSubmission(v communication.PortalDeclarationSubmission) models.DeclarationSubmission {
	return models.DeclarationSubmission{
		ID:                v.ID,
		TenantID:          v.TenantID,
		AnnouncementID:    v.AnnouncementID,
		VersionID:         v.VersionID,
		VersionNo:         v.VersionNo,
		StudentID:         v.StudentID,
		AccountID:         v.AccountID,
		GuardianProfileID: v.GuardianProfileID,
		SignerName:        v.SignerName,
		GuardianRole:      v.GuardianRole,
		Action:            v.Action,
		Method:            v.Method,
		PasswordConfirmed: v.PasswordConfirmed,
		ContentHash:       v.ContentHash,
		RecordHash:        v.RecordHash,
		SubmittedAt:       v.SubmittedAt,
	}
}
func portalFromRetainedDeclarationVersion(v models.DeclarationVersion) communication.PortalDeclarationVersion {
	return communication.PortalDeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments:    portalRecordSlice(v.Attachments, portalFromRetainedDeclarationAttachmentDigest),
		ContentHash:    v.ContentHash,
		PublishedAt:    v.PublishedAt,
	}
}
func portalToRetainedDeclarationVersion(v communication.PortalDeclarationVersion) models.DeclarationVersion {
	return models.DeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments:    portalRecordSlice(v.Attachments, portalToRetainedDeclarationAttachmentDigest),
		ContentHash:    v.ContentHash,
		PublishedAt:    v.PublishedAt,
	}
}
