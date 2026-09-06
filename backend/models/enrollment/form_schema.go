package enrollment

import (
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type FormFieldType = capability.FormFieldType
type FormFieldOption = capability.FormFieldOption
type FormFieldValidation = capability.FormFieldValidation
type VisibilityCondition = capability.VisibilityCondition
type FormField = capability.FormField
type CoreRequirements = capability.CoreRequirements
type ReservedTarget = capability.ReservedTarget
type PhoneEntry = capability.PhoneEntry
type WeekdaySchedule = capability.WeekdaySchedule
type WeekdayBoolean = capability.WeekdayBoolean
type WeekdayMode = capability.WeekdayMode
type WeekdayMultiMode = capability.WeekdayMultiMode
type ContactEntry = capability.ContactEntry
type FormLegalBlock = capability.FormLegalBlock

const (
	FormFieldBoolean                    = capability.FormFieldBoolean
	FormFieldNumber                     = capability.FormFieldNumber
	FormFieldText                       = capability.FormFieldText
	FormFieldTextarea                   = capability.FormFieldTextarea
	FormFieldDate                       = capability.FormFieldDate
	FormFieldSelect                     = capability.FormFieldSelect
	FormFieldInfo                       = capability.FormFieldInfo
	FormFieldPhoneList                  = capability.FormFieldPhoneList
	FormFieldWeekdaySchedule            = capability.FormFieldWeekdaySchedule
	FormFieldWeekdayBoolean             = capability.FormFieldWeekdayBoolean
	FormFieldWeekdayMode                = capability.FormFieldWeekdayMode
	FormFieldWeekdayMultiMode           = capability.FormFieldWeekdayMultiMode
	FormFieldContactList                = capability.FormFieldContactList
	ConditionSourceField                = capability.ConditionSourceField
	ConditionSourceGradeLevel           = capability.ConditionSourceGradeLevel
	ConditionSourceCareOffering         = capability.ConditionSourceCareOffering
	ConditionOpEquals                   = capability.ConditionOpEquals
	ConditionOpNotEquals                = capability.ConditionOpNotEquals
	ConditionOpNotEmpty                 = capability.ConditionOpNotEmpty
	ConditionOpIncludes                 = capability.ConditionOpIncludes
	CoreRequirementGuardianPhone        = capability.CoreRequirementGuardianPhone
	TargetStudentHealthInfo             = capability.TargetStudentHealthInfo
	TargetStudentExtraInfo              = capability.TargetStudentExtraInfo
	TargetStudentDeparture              = capability.TargetStudentDeparture
	TargetStudentAllowedDepartureModes  = capability.TargetStudentAllowedDepartureModes
	TargetStudentBusDays                = capability.TargetStudentBusDays
	TargetStudentBus                    = capability.TargetStudentBus
	TargetStudentPickupStatus           = capability.TargetStudentPickupStatus
	TargetStudentDepartureCompanionNote = capability.TargetStudentDepartureCompanionNote
	TargetSchedulePickup                = capability.TargetSchedulePickup
	TargetScheduleArrival               = capability.TargetScheduleArrival
	TargetStudentContacts               = capability.TargetStudentContacts
	WeekdayModeAlone                    = capability.WeekdayModeAlone
	WeekdayModeBus                      = capability.WeekdayModeBus
	WeekdayModePickup                   = capability.WeekdayModePickup
	WeekdayModeAccompanied              = capability.WeekdayModeAccompanied
	LegalBlockKindTerms                 = capability.LegalBlockKindTerms
	LegalBlockKindPrivacyNotice         = capability.LegalBlockKindPrivacyNotice
	LegalBlockKindNotice                = capability.LegalBlockKindNotice
	LegalBlockKindConsent               = capability.LegalBlockKindConsent
	LegalBlockSourceStandard            = capability.LegalBlockSourceStandard
	LegalBlockSourceCustom              = capability.LegalBlockSourceCustom
	LegalBlockDisplayModeText           = capability.LegalBlockDisplayModeText
	LegalBlockDisplayModePDF            = capability.LegalBlockDisplayModePDF
)

var (
	ReservedTargets   = capability.ReservedTargets
	CoreFieldKeys     = capability.CoreFieldKeys
	ValidPhoneTypes   = capability.ValidPhoneTypes
	ValidWeekdayModes = capability.ValidWeekdayModes
	ValidWeekdays     = capability.ValidWeekdays
)
