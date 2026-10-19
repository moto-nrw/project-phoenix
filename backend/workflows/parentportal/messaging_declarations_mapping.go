package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

func messagingToAppAnnouncementDeclarationSettings(v AnnouncementDeclarationSettings) messaging.AnnouncementDeclarationSettings {
	return messaging.AnnouncementDeclarationSettings{
		Kind:             v.Kind,
		Signers:          v.Signers,
		Revocable:        v.Revocable,
		RequiresPassword: v.RequiresPassword,
	}
}

func messagingToAppAnnouncementFeedDeclaration(v AnnouncementFeedDeclaration) messaging.AnnouncementFeedDeclaration {
	return messaging.AnnouncementFeedDeclaration{
		Kind:             v.Kind,
		Signers:          v.Signers,
		Revocable:        v.Revocable,
		RequiresPassword: v.RequiresPassword,
		Deadline:         v.Deadline,
		Closed:           v.Closed,
		Version:          messagingValuePointer(v.Version, messagingToAppDeclarationVersion),
		Children: messagingValueSlice(v.Children, func(row *AnnouncementFeedDeclarationChild) *messaging.AnnouncementFeedDeclarationChild {
			return messagingValuePointer(row, messagingToAppAnnouncementFeedDeclarationChild)
		}),
	}
}
func messagingFromAppAnnouncementFeedDeclaration(v messaging.AnnouncementFeedDeclaration) AnnouncementFeedDeclaration {
	return AnnouncementFeedDeclaration{
		Kind:             v.Kind,
		Signers:          v.Signers,
		Revocable:        v.Revocable,
		RequiresPassword: v.RequiresPassword,
		Deadline:         v.Deadline,
		Closed:           v.Closed,
		Version:          messagingValuePointer(v.Version, messagingFromAppDeclarationVersion),
		Children: messagingValueSlice(v.Children, func(row *messaging.AnnouncementFeedDeclarationChild) *AnnouncementFeedDeclarationChild {
			return messagingValuePointer(row, messagingFromAppAnnouncementFeedDeclarationChild)
		}),
	}
}
func messagingToAppAnnouncementFeedDeclarationChild(v AnnouncementFeedDeclarationChild) messaging.AnnouncementFeedDeclarationChild {
	return messaging.AnnouncementFeedDeclarationChild{
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		CanSubmit:      v.CanSubmit,
		State:          v.State,
		MyAction:       v.MyAction,
		MySubmittedAt:  v.MySubmittedAt,
		AllowedActions: v.AllowedActions,
		OtherSigners:   messagingValueSlice(v.OtherSigners, messagingToAppDeclarationSignerState),
	}
}
func messagingFromAppAnnouncementFeedDeclarationChild(v messaging.AnnouncementFeedDeclarationChild) AnnouncementFeedDeclarationChild {
	return AnnouncementFeedDeclarationChild{
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		CanSubmit:      v.CanSubmit,
		State:          v.State,
		MyAction:       v.MyAction,
		MySubmittedAt:  v.MySubmittedAt,
		AllowedActions: v.AllowedActions,
		OtherSigners:   messagingValueSlice(v.OtherSigners, messagingFromAppDeclarationSignerState),
	}
}
func messagingToAppDeclarationAttachmentDigest(v DeclarationAttachmentDigest) messaging.DeclarationAttachmentDigest {
	return messaging.DeclarationAttachmentDigest{
		AttachmentID: v.AttachmentID,
		Filename:     v.Filename,
		ContentType:  v.ContentType,
		SizeBytes:    v.SizeBytes,
		SHA256:       v.SHA256,
	}
}
func messagingFromAppDeclarationAttachmentDigest(v messaging.DeclarationAttachmentDigest) DeclarationAttachmentDigest {
	return DeclarationAttachmentDigest{
		AttachmentID: v.AttachmentID,
		Filename:     v.Filename,
		ContentType:  v.ContentType,
		SizeBytes:    v.SizeBytes,
		SHA256:       v.SHA256,
	}
}
func messagingToAppDeclarationChild(v DeclarationChild) messaging.DeclarationChild {
	return messaging.DeclarationChild{
		AnnouncementID: v.AnnouncementID,
		StudentID:      v.StudentID,
		FirstName:      v.FirstName,
		LastName:       v.LastName,
		SchoolClass:    v.SchoolClass,
	}
}

func messagingToAppDeclarationSigner(v DeclarationSigner) messaging.DeclarationSigner {
	return messaging.DeclarationSigner{
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

func messagingToAppDeclarationSignerContext(v DeclarationSignerContext) messaging.DeclarationSignerContext {
	return messaging.DeclarationSignerContext{
		GuardianProfileID: v.GuardianProfileID,
		FirstName:         v.FirstName,
		LastName:          v.LastName,
		GuardianRole:      v.GuardianRole,
	}
}

func messagingToAppDeclarationSignerState(v DeclarationSignerState) messaging.DeclarationSignerState {
	return messaging.DeclarationSignerState{
		AccountID:   v.AccountID,
		FirstName:   v.FirstName,
		LastName:    v.LastName,
		Action:      v.Action,
		SubmittedAt: v.SubmittedAt,
	}
}
func messagingFromAppDeclarationSignerState(v messaging.DeclarationSignerState) DeclarationSignerState {
	return DeclarationSignerState{
		AccountID:   v.AccountID,
		FirstName:   v.FirstName,
		LastName:    v.LastName,
		Action:      v.Action,
		SubmittedAt: v.SubmittedAt,
	}
}
func messagingToAppDeclarationSubmission(v DeclarationSubmission) messaging.DeclarationSubmission {
	return messaging.DeclarationSubmission{
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
func messagingFromAppDeclarationSubmission(v messaging.DeclarationSubmission) DeclarationSubmission {
	return DeclarationSubmission{
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

func messagingToAppDeclarationVersion(v DeclarationVersion) messaging.DeclarationVersion {
	return messaging.DeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments:    messagingValueSlice(v.Attachments, messagingToAppDeclarationAttachmentDigest),
		ContentHash:    v.ContentHash,
		PublishedAt:    v.PublishedAt,
	}
}
func messagingFromAppDeclarationVersion(v messaging.DeclarationVersion) DeclarationVersion {
	return DeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments:    messagingValueSlice(v.Attachments, messagingFromAppDeclarationAttachmentDigest),
		ContentHash:    v.ContentHash,
		PublishedAt:    v.PublishedAt,
	}
}
