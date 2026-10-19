package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/care"

func carePlanToAppCourseCatalog(value CourseCatalogSnapshot) care.CourseCatalog {
	return care.CourseCatalog{
		Enabled:                value.Enabled,
		DisabledReason:         value.DisabledReason,
		PhaseName:              value.PhaseName,
		EffectiveFrom:          value.EffectiveFrom,
		PendingRequestID:       value.PendingRequestID,
		PendingSubmittedBySelf: value.PendingSubmittedBySelf,
		OtherRequestPending:    value.OtherRequestPending,
		CanRequest:             value.CanRequest,
		ReasonRequired:         value.ReasonRequired,
		Items:                  carePlanValueSlice(value.Items, carePlanToAppCourseCatalogItem),
	}
}
func carePlanFromAppCourseCatalog(value care.CourseCatalog) CourseCatalogSnapshot {
	return CourseCatalogSnapshot{
		Enabled:                value.Enabled,
		DisabledReason:         value.DisabledReason,
		PhaseName:              value.PhaseName,
		EffectiveFrom:          value.EffectiveFrom,
		PendingRequestID:       value.PendingRequestID,
		PendingSubmittedBySelf: value.PendingSubmittedBySelf,
		OtherRequestPending:    value.OtherRequestPending,
		CanRequest:             value.CanRequest,
		ReasonRequired:         value.ReasonRequired,
		Items:                  carePlanValueSlice(value.Items, carePlanFromAppCourseCatalogItem),
	}
}
func carePlanToAppCourseCatalogItem(value CourseCatalogItemSnapshot) care.CourseCatalogItem {
	return care.CourseCatalogItem{
		OfferingID:       value.OfferingID,
		ActivityGroupID:  value.ActivityGroupID,
		Name:             value.Name,
		Description:      value.Description,
		AvailableDays:    value.AvailableDays,
		Capacity:         value.Capacity,
		FreeSlots:        value.FreeSlots,
		Booked:           value.Booked,
		Requested:        value.Requested,
		Waitlisted:       value.Waitlisted,
		WaitlistPosition: value.WaitlistPosition,
	}
}
func carePlanFromAppCourseCatalogItem(value care.CourseCatalogItem) CourseCatalogItemSnapshot {
	return CourseCatalogItemSnapshot{
		OfferingID:       value.OfferingID,
		ActivityGroupID:  value.ActivityGroupID,
		Name:             value.Name,
		Description:      value.Description,
		AvailableDays:    value.AvailableDays,
		Capacity:         value.Capacity,
		FreeSlots:        value.FreeSlots,
		Booked:           value.Booked,
		Requested:        value.Requested,
		Waitlisted:       value.Waitlisted,
		WaitlistPosition: value.WaitlistPosition,
	}
}

func carePlanFromAppCreateCourseRequestInput(value care.CreateCourseRequestInput) CreateCourseRequestInputSnapshot {
	return CreateCourseRequestInputSnapshot{
		StudentID:  value.StudentID,
		AccountID:  value.AccountID,
		OfferingID: value.OfferingID,
		Note:       value.Note,
	}
}

func carePlanFromAppCreateOfferingChangeInput(value care.CreateOfferingChangeInput) CreateOfferingChangeInputSnapshot {
	return CreateOfferingChangeInputSnapshot{
		StudentID:                   value.StudentID,
		AccountID:                   value.AccountID,
		Selections:                  carePlanValueSlice(value.Selections, carePlanFromAppOfferingChangeSelection),
		EffectiveFrom:               value.EffectiveFrom,
		Note:                        value.Note,
		CompleteWithdrawalConfirmed: value.CompleteWithdrawalConfirmed,
	}
}
func carePlanToAppOfferingChangeCatalog(value OfferingChangeCatalogSnapshot) care.OfferingChangeCatalog {
	return care.OfferingChangeCatalog{
		PhaseID:               value.PhaseID,
		PhaseName:             value.PhaseName,
		SelectionMode:         value.SelectionMode,
		EarliestEffectiveFrom: value.EarliestEffectiveFrom,
		LatestEffectiveFrom:   value.LatestEffectiveFrom,
		TargetGradeLevel:      value.TargetGradeLevel,
		TargetSchoolClass:     value.TargetSchoolClass,
		CourseCapacityUntil:   value.CourseCapacityUntil,
		Items:                 carePlanValueSlice(value.Items, carePlanToAppOfferingChangeCatalogItem),
	}
}
func carePlanFromAppOfferingChangeCatalog(value care.OfferingChangeCatalog) OfferingChangeCatalogSnapshot {
	return OfferingChangeCatalogSnapshot{
		PhaseID:               value.PhaseID,
		PhaseName:             value.PhaseName,
		SelectionMode:         value.SelectionMode,
		EarliestEffectiveFrom: value.EarliestEffectiveFrom,
		LatestEffectiveFrom:   value.LatestEffectiveFrom,
		TargetGradeLevel:      value.TargetGradeLevel,
		TargetSchoolClass:     value.TargetSchoolClass,
		CourseCapacityUntil:   value.CourseCapacityUntil,
		Items:                 carePlanValueSlice(value.Items, carePlanFromAppOfferingChangeCatalogItem),
	}
}
func carePlanToAppOfferingChangeCatalogItem(value OfferingChangeCatalogItemSnapshot) care.OfferingChangeCatalogItem {
	return care.OfferingChangeCatalogItem{
		OfferingID:      value.OfferingID,
		Name:            value.Name,
		Description:     value.Description,
		DaysOfWeekMode:  value.DaysOfWeekMode,
		AvailableDays:   value.AvailableDays,
		SelectionGroup:  value.SelectionGroup,
		SelectionRule:   value.SelectionRule,
		IsRequired:      value.IsRequired,
		PriceCents:      value.PriceCents,
		IncludesLunch:   value.IncludesLunch,
		IncludesHoliday: value.IncludesHoliday,
		CountsAsCare:    value.CountsAsCare,
		PickupTimes:     value.PickupTimes,
		Selected:        value.Selected,
		SelectedDays:    value.SelectedDays,
		Automatic:       value.Automatic,
		IsActive:        value.IsActive,
		Capacity:        value.Capacity,
		FreeSlots:       value.FreeSlots,
		ActivityGroupID: value.ActivityGroupID,
	}
}
func carePlanFromAppOfferingChangeCatalogItem(value care.OfferingChangeCatalogItem) OfferingChangeCatalogItemSnapshot {
	return OfferingChangeCatalogItemSnapshot{
		OfferingID:      value.OfferingID,
		Name:            value.Name,
		Description:     value.Description,
		DaysOfWeekMode:  value.DaysOfWeekMode,
		AvailableDays:   value.AvailableDays,
		SelectionGroup:  value.SelectionGroup,
		SelectionRule:   value.SelectionRule,
		IsRequired:      value.IsRequired,
		PriceCents:      value.PriceCents,
		IncludesLunch:   value.IncludesLunch,
		IncludesHoliday: value.IncludesHoliday,
		CountsAsCare:    value.CountsAsCare,
		PickupTimes:     value.PickupTimes,
		Selected:        value.Selected,
		SelectedDays:    value.SelectedDays,
		Automatic:       value.Automatic,
		IsActive:        value.IsActive,
		Capacity:        value.Capacity,
		FreeSlots:       value.FreeSlots,
		ActivityGroupID: value.ActivityGroupID,
	}
}
func carePlanToAppOfferingChangeDecision(value OfferingChangeDecisionSnapshot) care.OfferingChangeDecision {
	return care.OfferingChangeDecision{
		ID:                  value.ID,
		SubmittedBy:         value.SubmittedBy,
		SubmittedBySelf:     value.SubmittedBySelf,
		Status:              value.Status,
		CompleteWithdrawal:  value.CompleteWithdrawal,
		DecidedAt:           value.DecidedAt,
		EffectiveFrom:       value.EffectiveFrom,
		Reason:              value.Reason,
		Requested:           carePlanValueSlice(value.Requested, carePlanToAppOfferingChangeRequestedItem),
		AppliedDiff:         carePlanValueSlice(value.AppliedDiff, carePlanToAppOfferingChangeDiffEntry),
		OverriddenOfferings: carePlanValueSlice(value.OverriddenOfferings, carePlanToAppOfferingOverride),
	}
}
func carePlanFromAppOfferingChangeDecision(value care.OfferingChangeDecision) OfferingChangeDecisionSnapshot {
	return OfferingChangeDecisionSnapshot{
		ID:                  value.ID,
		SubmittedBy:         value.SubmittedBy,
		SubmittedBySelf:     value.SubmittedBySelf,
		Status:              value.Status,
		CompleteWithdrawal:  value.CompleteWithdrawal,
		DecidedAt:           value.DecidedAt,
		EffectiveFrom:       value.EffectiveFrom,
		Reason:              value.Reason,
		Requested:           carePlanValueSlice(value.Requested, carePlanFromAppOfferingChangeRequestedItem),
		AppliedDiff:         carePlanValueSlice(value.AppliedDiff, carePlanFromAppOfferingChangeDiffEntry),
		OverriddenOfferings: carePlanValueSlice(value.OverriddenOfferings, carePlanFromAppOfferingOverride),
	}
}
func carePlanToAppOfferingChangeDiffEntry(value OfferingChangeDiffEntrySnapshot) care.OfferingChangeDiffEntry {
	return care.OfferingChangeDiffEntry{
		OfferingID:          value.OfferingID,
		Label:               value.Label,
		OldState:            value.OldState,
		OldDays:             value.OldDays,
		NewState:            value.NewState,
		NewDays:             value.NewDays,
		NewAutomaticDays:    value.NewAutomaticDays,
		NewRuleDays:         value.NewRuleDays,
		NewDaysWithoutRules: value.NewDaysWithoutRules,
		AutoTriggerIDs:      value.AutoTriggerIDs,
		AutoTriggerNames:    value.AutoTriggerNames,
		IsCourse:            value.IsCourse,
	}
}
func carePlanFromAppOfferingChangeDiffEntry(value care.OfferingChangeDiffEntry) OfferingChangeDiffEntrySnapshot {
	return OfferingChangeDiffEntrySnapshot{
		OfferingID:          value.OfferingID,
		Label:               value.Label,
		OldState:            value.OldState,
		OldDays:             value.OldDays,
		NewState:            value.NewState,
		NewDays:             value.NewDays,
		NewAutomaticDays:    value.NewAutomaticDays,
		NewRuleDays:         value.NewRuleDays,
		NewDaysWithoutRules: value.NewDaysWithoutRules,
		AutoTriggerIDs:      value.AutoTriggerIDs,
		AutoTriggerNames:    value.AutoTriggerNames,
		IsCourse:            value.IsCourse,
	}
}
func carePlanToAppOfferingChangeRequest(value OfferingChangeRequestSnapshot) care.OfferingChangeRequest {
	return care.OfferingChangeRequest{
		ID:                          value.ID,
		TenantID:                    value.TenantID,
		CreatedAt:                   value.CreatedAt,
		UpdatedAt:                   value.UpdatedAt,
		StudentID:                   value.StudentID,
		RequestChildID:              value.RequestChildID,
		SubmittedBy:                 value.SubmittedBy,
		CompleteWithdrawalConfirmed: value.CompleteWithdrawalConfirmed,
		WithdrawalConfirmedBy:       value.WithdrawalConfirmedBy,
		WithdrawalConfirmedAt:       value.WithdrawalConfirmedAt,
		ApprovedCompleteWithdrawal:  value.ApprovedCompleteWithdrawal,
		Payload:                     value.Payload,
		EffectiveFrom:               value.EffectiveFrom,
		ParentNote:                  value.ParentNote,
		Status:                      value.Status,
		DecisionReason:              value.DecisionReason,
		DecisionSnapshot:            value.DecisionSnapshot,
		ReviewedBy:                  value.ReviewedBy,
		ReviewedAt:                  value.ReviewedAt,
		AppliedAt:                   value.AppliedAt,
	}
}

func carePlanToAppOfferingChangeRequestedItem(value OfferingChangeRequestedItemSnapshot) care.OfferingChangeRequestedItem {
	return care.OfferingChangeRequestedItem{
		OfferingID: value.OfferingID,
		Name:       value.Name,
		Days:       value.Days,
	}
}
func carePlanFromAppOfferingChangeRequestedItem(value care.OfferingChangeRequestedItem) OfferingChangeRequestedItemSnapshot {
	return OfferingChangeRequestedItemSnapshot{
		OfferingID: value.OfferingID,
		Name:       value.Name,
		Days:       value.Days,
	}
}
func carePlanToAppOfferingChangeSelection(value OfferingChangeSelectionSnapshot) care.OfferingChangeSelection {
	return care.OfferingChangeSelection{
		OfferingID:   value.OfferingID,
		SelectedDays: value.SelectedDays,
	}
}
func carePlanFromAppOfferingChangeSelection(value care.OfferingChangeSelection) OfferingChangeSelectionSnapshot {
	return OfferingChangeSelectionSnapshot{
		OfferingID:   value.OfferingID,
		SelectedDays: value.SelectedDays,
	}
}
func carePlanToAppOfferingChangeView(value OfferingChangeViewSnapshot) care.OfferingChangeView {
	return care.OfferingChangeView{
		Request:      carePlanValuePointer(value.Request, carePlanToAppOfferingChangeRequest),
		Diff:         carePlanValueSlice(value.Diff, carePlanToAppOfferingChangeDiffEntry),
		LastDecision: carePlanValuePointer(value.LastDecision, carePlanToAppOfferingChangeDecision),
	}
}

func carePlanToAppOfferingOverride(value OfferingOverrideSnapshot) care.OfferingOverride {
	return care.OfferingOverride{
		OfferingID: value.OfferingID,
		Name:       value.Name,
	}
}
func carePlanFromAppOfferingOverride(value care.OfferingOverride) OfferingOverrideSnapshot {
	return OfferingOverrideSnapshot{
		OfferingID: value.OfferingID,
		Name:       value.Name,
	}
}
