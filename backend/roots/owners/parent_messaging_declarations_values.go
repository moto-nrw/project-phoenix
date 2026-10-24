package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentMessagingFromOwnerAnnouncementDeclarationSettings(v communication.PortalAnnouncementDeclarationSettings) parentportal.AnnouncementDeclarationSettings {
	return parentportal.AnnouncementDeclarationSettings{
		Kind:              v.Kind,
		Signers:           v.Signers,
		Revocable:         v.Revocable,
		RequiresPassword:  v.RequiresPassword,
		RequiresSignature: v.RequiresSignature,
	}
}

func parentMessagingFromOwnerAnnouncementFeedDeclaration(v communication.PortalAnnouncementFeedDeclaration) parentportal.AnnouncementFeedDeclaration {
	return parentportal.AnnouncementFeedDeclaration{
		Kind:              v.Kind,
		Signers:           v.Signers,
		Revocable:         v.Revocable,
		RequiresPassword:  v.RequiresPassword,
		RequiresSignature: v.RequiresSignature,
		Deadline:          v.Deadline,
		Closed:            v.Closed,
		Version:           ParentMessagingPointer(v.Version, ParentMessagingFromOwnerDeclarationVersion),
		Children: ParentMessagingSlice(v.Children, func(row *communication.PortalAnnouncementFeedDeclarationChild) *parentportal.AnnouncementFeedDeclarationChild {
			return ParentMessagingPointer(row, parentMessagingFromOwnerAnnouncementFeedDeclarationChild)
		}),
	}
}

func parentMessagingFromOwnerAnnouncementFeedDeclarationChild(v communication.PortalAnnouncementFeedDeclarationChild) parentportal.AnnouncementFeedDeclarationChild {
	return parentportal.AnnouncementFeedDeclarationChild{
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		CanSubmit:      v.CanSubmit,
		State:          v.State,
		MyAction:       v.MyAction,
		MySubmittedAt:  v.MySubmittedAt,
		AllowedActions: v.AllowedActions,
		OtherSigners:   ParentMessagingSlice(v.OtherSigners, parentMessagingFromOwnerDeclarationSignerState),
	}
}

func parentMessagingToOwnerDeclarationAttachmentDigest(v parentportal.DeclarationAttachmentDigest) communication.PortalDeclarationAttachmentDigest {
	return communication.PortalDeclarationAttachmentDigest{
		AttachmentID: v.AttachmentID,
		Filename:     v.Filename,
		ContentType:  v.ContentType,
		SizeBytes:    v.SizeBytes,
		SHA256:       v.SHA256,
	}
}

func parentMessagingFromOwnerDeclarationAttachmentDigest(v communication.PortalDeclarationAttachmentDigest) parentportal.DeclarationAttachmentDigest {
	return parentportal.DeclarationAttachmentDigest{
		AttachmentID: v.AttachmentID,
		Filename:     v.Filename,
		ContentType:  v.ContentType,
		SizeBytes:    v.SizeBytes,
		SHA256:       v.SHA256,
	}
}

func parentMessagingFromOwnerDeclarationChild(v communication.PortalDeclarationChild) parentportal.DeclarationChild {
	return parentportal.DeclarationChild{
		AnnouncementID: v.AnnouncementID,
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		SchoolClass:    v.SchoolClass,
	}
}

func parentMessagingFromOwnerDeclarationSigner(v communication.PortalDeclarationSigner) parentportal.DeclarationSigner {
	return parentportal.DeclarationSigner{
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

func parentMessagingFromOwnerDeclarationSignerContext(v communication.PortalDeclarationSignerContext) parentportal.DeclarationSignerContext {
	return parentportal.DeclarationSignerContext{
		GuardianProfileID: v.GuardianProfileID,
		FirstName:         v.FirstName,
		LastName:          v.LastName,
		GuardianRole:      v.GuardianRole,
	}
}

func parentMessagingFromOwnerDeclarationSignerState(v communication.PortalDeclarationSignerState) parentportal.DeclarationSignerState {
	return parentportal.DeclarationSignerState{
		AccountID:   v.AccountID,
		FirstName:   v.FirstName,
		LastName:    v.LastName,
		Action:      v.Action,
		SubmittedAt: v.SubmittedAt,
	}
}

func ParentMessagingToOwnerDeclarationSubmission(v parentportal.DeclarationSubmission) communication.PortalDeclarationSubmission {
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
		SignatureID:       v.SignatureID,
	}
}

func ParentMessagingFromOwnerDeclarationSubmission(v communication.PortalDeclarationSubmission) parentportal.DeclarationSubmission {
	return parentportal.DeclarationSubmission{
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
		SignatureID:       v.SignatureID,
	}
}

func ParentMessagingToOwnerDeclarationVersion(v parentportal.DeclarationVersion) communication.PortalDeclarationVersion {
	return communication.PortalDeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments:    ParentMessagingSlice(v.Attachments, parentMessagingToOwnerDeclarationAttachmentDigest),
		ContentHash:    v.ContentHash,
		PublishedAt:    v.PublishedAt,
	}
}

func ParentMessagingFromOwnerDeclarationVersion(v communication.PortalDeclarationVersion) parentportal.DeclarationVersion {
	return parentportal.DeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments:    ParentMessagingSlice(v.Attachments, parentMessagingFromOwnerDeclarationAttachmentDigest),
		ContentHash:    v.ContentHash,
		PublishedAt:    v.PublishedAt,
	}
}
