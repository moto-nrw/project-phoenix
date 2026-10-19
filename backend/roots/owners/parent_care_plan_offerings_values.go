package owners

import (
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

func parentCarePlanFromOwnerCourseCatalog(value careplan.CourseCatalog) parentportal.CourseCatalogSnapshot {
	return parentportal.CourseCatalogSnapshot{
		Enabled:                value.Enabled,
		DisabledReason:         value.DisabledReason,
		PhaseName:              value.PhaseName,
		EffectiveFrom:          value.EffectiveFrom,
		PendingRequestID:       value.PendingRequestID,
		PendingSubmittedBySelf: value.PendingSubmittedBySelf,
		OtherRequestPending:    value.OtherRequestPending,
		CanRequest:             value.CanRequest,
		ReasonRequired:         value.ReasonRequired,
		Items:                  parentCarePlanSlice(value.Items, parentCarePlanFromOwnerCourseCatalogItem),
	}
}

func parentCarePlanFromOwnerCourseCatalogItem(value careplan.CourseCatalogItem) parentportal.CourseCatalogItemSnapshot {
	return parentportal.CourseCatalogItemSnapshot{
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

func parentCarePlanToOwnerCreateCourseRequestInput(value parentportal.CreateCourseRequestInputSnapshot) careplan.CreateCourseRequestInput {
	return careplan.CreateCourseRequestInput{
		StudentID:  value.StudentID,
		AccountID:  value.AccountID,
		OfferingID: value.OfferingID,
		Note:       value.Note,
	}
}

func parentCarePlanToOwnerCreateOfferingChangeInput(value parentportal.CreateOfferingChangeInputSnapshot) careplan.CreateOfferingChangeInput {
	return careplan.CreateOfferingChangeInput{
		StudentID:                   value.StudentID,
		AccountID:                   value.AccountID,
		Selections:                  parentCarePlanSlice(value.Selections, parentCarePlanToOwnerOfferingChangeSelection),
		EffectiveFrom:               value.EffectiveFrom,
		Note:                        value.Note,
		CompleteWithdrawalConfirmed: value.CompleteWithdrawalConfirmed,
	}
}

func parentCarePlanFromOwnerOfferingChangeCatalog(value careplan.OfferingChangeCatalog) parentportal.OfferingChangeCatalogSnapshot {
	return parentportal.OfferingChangeCatalogSnapshot{
		PhaseID:               value.PhaseID,
		PhaseName:             value.PhaseName,
		SelectionMode:         value.SelectionMode,
		EarliestEffectiveFrom: value.EarliestEffectiveFrom,
		LatestEffectiveFrom:   value.LatestEffectiveFrom,
		TargetGradeLevel:      value.TargetGradeLevel,
		TargetSchoolClass:     value.TargetSchoolClass,
		CourseCapacityUntil:   value.CourseCapacityUntil,
		Items:                 parentCarePlanSlice(value.Items, parentCarePlanFromOwnerOfferingChangeCatalogItem),
	}
}

func parentCarePlanFromOwnerOfferingChangeCatalogItem(value careplan.OfferingChangeCatalogItem) parentportal.OfferingChangeCatalogItemSnapshot {
	return parentportal.OfferingChangeCatalogItemSnapshot{
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

func parentCarePlanFromOwnerOfferingChangeDecision(value careplan.OfferingChangeDecision) parentportal.OfferingChangeDecisionSnapshot {
	return parentportal.OfferingChangeDecisionSnapshot{
		ID:                  value.ID,
		SubmittedBy:         value.SubmittedBy,
		SubmittedBySelf:     value.SubmittedBySelf,
		Status:              value.Status,
		CompleteWithdrawal:  value.CompleteWithdrawal,
		DecidedAt:           value.DecidedAt,
		EffectiveFrom:       value.EffectiveFrom,
		Reason:              value.Reason,
		Requested:           parentCarePlanSlice(value.Requested, parentCarePlanFromOwnerOfferingChangeRequestedItem),
		AppliedDiff:         parentCarePlanSlice(value.AppliedDiff, parentCarePlanFromOwnerOfferingChangeDiffEntry),
		OverriddenOfferings: parentCarePlanSlice(value.OverriddenOfferings, parentCarePlanFromOwnerOfferingOverride),
	}
}

func parentCarePlanFromOwnerOfferingChangeDiffEntry(value careplan.OfferingChangeDiffEntry) parentportal.OfferingChangeDiffEntrySnapshot {
	return parentportal.OfferingChangeDiffEntrySnapshot{
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

func parentCarePlanFromOwnerOfferingChangeRequest(value careplan.OfferingChangeRequest) parentportal.OfferingChangeRequestSnapshot {
	return parentportal.OfferingChangeRequestSnapshot{
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

func parentCarePlanFromOwnerOfferingChangeRequestedItem(value careplan.OfferingChangeRequestedItem) parentportal.OfferingChangeRequestedItemSnapshot {
	return parentportal.OfferingChangeRequestedItemSnapshot{
		OfferingID: value.OfferingID,
		Name:       value.Name,
		Days:       value.Days,
	}
}

func parentCarePlanToOwnerOfferingChangeSelection(value parentportal.OfferingChangeSelectionSnapshot) careplan.OfferingChangeSelection {
	return careplan.OfferingChangeSelection{
		OfferingID:   value.OfferingID,
		SelectedDays: value.SelectedDays,
	}
}

func parentCarePlanFromOwnerOfferingChangeView(value careplan.OfferingChangeView) parentportal.OfferingChangeViewSnapshot {
	return parentportal.OfferingChangeViewSnapshot{
		Request:      parentCarePlanPointer(value.Request, parentCarePlanFromOwnerOfferingChangeRequest),
		Diff:         parentCarePlanSlice(value.Diff, parentCarePlanFromOwnerOfferingChangeDiffEntry),
		LastDecision: parentCarePlanPointer(value.LastDecision, parentCarePlanFromOwnerOfferingChangeDecision),
	}
}

func parentCarePlanFromOwnerOfferingOverride(value careplan.OfferingOverride) parentportal.OfferingOverrideSnapshot {
	return parentportal.OfferingOverrideSnapshot{
		OfferingID: value.OfferingID,
		Name:       value.Name,
	}
}
