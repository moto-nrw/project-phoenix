package application

func StaffAnnouncementAnnouncementPollResultsFromValue(v *AnnouncementPollResults) *StaffAnnouncementPollResults {
	if v == nil {
		return nil
	}
	return &StaffAnnouncementPollResults{
		ChildCount:       v.ChildCount,
		TargetChildCount: v.TargetChildCount,
		AnsweredCount:    v.AnsweredCount,
		Options: MapAnnouncementRows(AnnouncementPollOptionResultSliceFromAnnouncementRecords(v.Options), func(row *AnnouncementPollOptionResult) *StaffAnnouncementPollOptionResult {
			return (*StaffAnnouncementPollOptionResult)(row)
		}),
	}
}
func StaffAnnouncementDeclarationVersionToValue(v *StaffDeclarationVersion) *DeclarationVersion {
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
		Attachments: DeclarationAttachmentDigestValuesToAnnouncementRecords(MapAnnouncementRows(v.Attachments, func(row StaffDeclarationAttachmentDigest) DeclarationAttachmentDigest {
			return DeclarationAttachmentDigest(row)
		})),
		ContentHash: v.ContentHash,
		PublishedAt: v.PublishedAt,
	}
}
func StaffAnnouncementDeclarationVersionFromValue(v *DeclarationVersion) *StaffDeclarationVersion {
	if v == nil {
		return nil
	}
	return &StaffDeclarationVersion{
		ID:             v.ID,
		TenantID:       v.TenantID,
		AnnouncementID: v.AnnouncementID,
		VersionNo:      v.VersionNo,
		Title:          v.Title,
		Body:           v.Body,
		Kind:           v.Kind,
		Attachments: MapAnnouncementRows(DeclarationAttachmentDigestValuesFromAnnouncementRecords(v.Attachments), func(row DeclarationAttachmentDigest) StaffDeclarationAttachmentDigest {
			return StaffDeclarationAttachmentDigest(row)
		}),
		ContentHash: v.ContentHash,
		PublishedAt: v.PublishedAt,
	}
}
func StaffAnnouncementParentAnnouncementToValue(v *StaffParentAnnouncement) *ParentAnnouncement {
	if v == nil {
		return nil
	}
	result := &ParentAnnouncement{
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
		SenderName:              v.SenderName,
		SenderSchoolClass:       v.SenderSchoolClass,
		ReminderAt:              v.ReminderAt,
		ReminderText:            v.ReminderText,
		ReminderSentAt:          v.ReminderSentAt,
		Targets: ParentAnnouncementTargetSliceToAnnouncementRecords(MapAnnouncementRows(v.Targets, func(row *StaffParentAnnouncementTarget) *ParentAnnouncementTarget {
			return (*ParentAnnouncementTarget)(row)
		})),
		Options: ParentAnnouncementOptionSliceToAnnouncementRecords(MapAnnouncementRows(v.Options, func(row *StaffParentAnnouncementOption) *ParentAnnouncementOption {
			return (*ParentAnnouncementOption)(row)
		})),
		Declaration: *AnnouncementDeclarationSettingsToAnnouncementRecord((*AnnouncementDeclarationSettings)(&v.Declaration)),
	}
	result.Poll = AnnouncementIsPoll(result)
	result.DeclarationMode = AnnouncementIsDeclaration(result)
	return result
}
func StaffAnnouncementParentAnnouncementFromValue(v *ParentAnnouncement) *StaffParentAnnouncement {
	if v == nil {
		return nil
	}
	return &StaffParentAnnouncement{
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
		SenderName:              v.SenderName,
		SenderSchoolClass:       v.SenderSchoolClass,
		ReminderAt:              v.ReminderAt,
		ReminderText:            v.ReminderText,
		ReminderSentAt:          v.ReminderSentAt,
		Targets: MapAnnouncementRows(ParentAnnouncementTargetSliceFromAnnouncementRecords(v.Targets), func(row *ParentAnnouncementTarget) *StaffParentAnnouncementTarget {
			return (*StaffParentAnnouncementTarget)(row)
		}),
		Options: MapAnnouncementRows(ParentAnnouncementOptionSliceFromAnnouncementRecords(v.Options), func(row *ParentAnnouncementOption) *StaffParentAnnouncementOption {
			return (*StaffParentAnnouncementOption)(row)
		}),
		Declaration: StaffAnnouncementDeclarationSettings(*AnnouncementDeclarationSettingsFromAnnouncementRecord(&v.Declaration)),
	}
}

func MapAnnouncementRows[A, B any](rows []A, mapRow func(A) B) []B {
	if rows == nil {
		return nil
	}
	out := make([]B, len(rows))
	for i, row := range rows {
		out[i] = mapRow(row)
	}
	return out
}
