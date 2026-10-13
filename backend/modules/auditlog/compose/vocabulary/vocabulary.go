// Package vocabulary names the Audit Platform records, repositories and constants
// the legacy composition still binds (#2747). The owner's composition
// re-exports them under its own path, so the legacy composition depends on the
// composition instead of the domain package. Shrink it as consumers move to the
// owner's public contract; never add a name nothing uses.
package vocabulary

import upstream "github.com/moto-nrw/project-phoenix/models/audit"

type (
	AppendStore                            = upstream.AppendStore
	AttendanceCorrection                   = upstream.AttendanceCorrection
	AttendanceCorrectionRepository         = upstream.AttendanceCorrectionRepository
	AuthEvent                              = upstream.AuthEvent
	AuthEventRepository                    = upstream.AuthEventRepository
	BookingConsistencyReport               = upstream.BookingConsistencyReport
	Command                                = upstream.Command
	DataAccessLog                          = upstream.DataAccessLog
	DataAccessLogRepository                = upstream.DataAccessLogRepository
	DataDeletion                           = upstream.DataDeletion
	DataDeletionRepository                 = upstream.DataDeletionRepository
	Date                                   = upstream.Date
	DeviationEvent                         = upstream.DeviationEvent
	DeviationEventRepository               = upstream.DeviationEventRepository
	EnrollmentDeletion                     = upstream.EnrollmentDeletion
	EnrollmentDeletionCounts               = upstream.EnrollmentDeletionCounts
	EnrollmentDeletionRepository           = upstream.EnrollmentDeletionRepository
	EnrollmentOfferingAdjustment           = upstream.EnrollmentOfferingAdjustment
	EnrollmentOfferingAdjustmentRepository = upstream.EnrollmentOfferingAdjustmentRepository
	EnrollmentRestoration                  = upstream.EnrollmentRestoration
	EnrollmentRestorationRepository        = upstream.EnrollmentRestorationRepository
	FileEvent                              = upstream.FileEvent
	FileEventRepository                    = upstream.FileEventRepository
	GuardianFinancialChange                = upstream.GuardianFinancialChange
	GuardianFinancialChangeCreator         = upstream.GuardianFinancialChangeCreator
	Model                                  = upstream.Model
	OperatorAuditEntry                     = upstream.OperatorAuditEntry
	OperatorAuditLogRepository             = upstream.OperatorAuditLogRepository
	PendingAccountWideWipe                 = upstream.PendingAccountWideWipe
	PersonnelNumberChange                  = upstream.PersonnelNumberChange
	PersonnelNumberChangeCreator           = upstream.PersonnelNumberChangeCreator
	RecentDeletionSummary                  = upstream.RecentDeletionSummary
	StaffMasterDataChange                  = upstream.StaffMasterDataChange
	StaffMasterDataChangeCreator           = upstream.StaffMasterDataChangeCreator
	StudentConsentChange                   = upstream.StudentConsentChange
	StudentConsentChangeRepository         = upstream.StudentConsentChangeRepository
	StudentDeletionRepository              = upstream.StudentDeletionRepository
	StudentFieldEdit                       = upstream.StudentFieldEdit
	StudentFieldEditRepository             = upstream.StudentFieldEditRepository
	SubstitutionChange                     = upstream.SubstitutionChange
	SubstitutionChangeCreator              = upstream.SubstitutionChangeCreator
	TimeTrackingAuditLogCursor             = upstream.TimeTrackingAuditLogCursor
	TimeTrackingAuditLogFilter             = upstream.TimeTrackingAuditLogFilter
	TimeTrackingAuditLogRepository         = upstream.TimeTrackingAuditLogRepository
	TimeTrackingDeletion                   = upstream.TimeTrackingDeletion
	TimeTrackingDeletionRepository         = upstream.TimeTrackingDeletionRepository
	UnregisteredTagScan                    = upstream.UnregisteredTagScan
	UnregisteredTagScanRepository          = upstream.UnregisteredTagScanRepository
	WorkSessionEdit                        = upstream.WorkSessionEdit
	WorkSessionEditRepository              = upstream.WorkSessionEditRepository
)

const (
	AttendanceFieldNote                   = upstream.AttendanceFieldNote
	AttendanceFieldStatus                 = upstream.AttendanceFieldStatus
	AttendanceFieldSubstatus              = upstream.AttendanceFieldSubstatus
	CorrectionReasonMaxLength             = upstream.CorrectionReasonMaxLength
	DeletionTypeManual                    = upstream.DeletionTypeManual
	DeletionTypeStudentChangeLogRetention = upstream.DeletionTypeStudentChangeLogRetention
	DeletionTypeTimetableRetention        = upstream.DeletionTypeTimetableRetention
	DeviationEventAbsence                 = upstream.DeviationEventAbsence
	DeviationEventReturnToPresence        = upstream.DeviationEventReturnToPresence
	DeviationEventSickCleared             = upstream.DeviationEventSickCleared
	DeviationEventSickReported            = upstream.DeviationEventSickReported
	DeviationEventSubstituteRemoved       = upstream.DeviationEventSubstituteRemoved
	DeviationEventSubstitution            = upstream.DeviationEventSubstitution
	EnrollmentDeletionActorAdmin          = upstream.EnrollmentDeletionActorAdmin
	EnrollmentDeletionActorSystem         = upstream.EnrollmentDeletionActorSystem
	EnrollmentDeletionScopeChild          = upstream.EnrollmentDeletionScopeChild
	EnrollmentDeletionScopeRequest        = upstream.EnrollmentDeletionScopeRequest
	EventTypeCaregiverCapabilityDisabled  = upstream.EventTypeCaregiverCapabilityDisabled
	EventTypeCaregiverCapabilityEnabled   = upstream.EventTypeCaregiverCapabilityEnabled
	EventTypeStaffPreviewEnded            = upstream.EventTypeStaffPreviewEnded
	EventTypeStaffPreviewStarted          = upstream.EventTypeStaffPreviewStarted
	EventTypeTenantAccessGranted          = upstream.EventTypeTenantAccessGranted
	EventTypeTenantAccessRevoked          = upstream.EventTypeTenantAccessRevoked
	EventTypeTenantRoleChanged            = upstream.EventTypeTenantRoleChanged
	GuardianPaymentFieldAccountHolder     = upstream.GuardianPaymentFieldAccountHolder
	GuardianPaymentFieldIBAN              = upstream.GuardianPaymentFieldIBAN
	GuardianPaymentFieldIsPayer           = upstream.GuardianPaymentFieldIsPayer
	ResourceTypeAttendanceStatistics      = upstream.ResourceTypeAttendanceStatistics
	ResourceTypeClassDayView              = upstream.ResourceTypeClassDayView
	ResourceTypeEnrollmentPhaseExport     = upstream.ResourceTypeEnrollmentPhaseExport
	ResourceTypeEnrollmentStudentExport   = upstream.ResourceTypeEnrollmentStudentExport
	ResourceTypeGuardianFinancialReveal   = upstream.ResourceTypeGuardianFinancialReveal
	ResourceTypeGuardianFinancialView     = upstream.ResourceTypeGuardianFinancialView
	ResourceTypeGuardianPaymentExport     = upstream.ResourceTypeGuardianPaymentExport
	ResourceTypeGuardianPaymentOverview   = upstream.ResourceTypeGuardianPaymentOverview
	ResourceTypeStaffDocumentDownload     = upstream.ResourceTypeStaffDocumentDownload
	ResourceTypeStaffFinancialReveal      = upstream.ResourceTypeStaffFinancialReveal
	ResourceTypeStaffFinancialView        = upstream.ResourceTypeStaffFinancialView
	ResourceTypeStudentDocumentDownload   = upstream.ResourceTypeStudentDocumentDownload
	ResourceTypeSupervisionStudentSheet   = upstream.ResourceTypeSupervisionStudentSheet
	StammdatenSectionArbeitsvertrag       = upstream.StammdatenSectionArbeitsvertrag
	StammdatenSectionBankSteuer           = upstream.StammdatenSectionBankSteuer
	StammdatenSectionDokumente            = upstream.StammdatenSectionDokumente
	StammdatenSectionKontakt              = upstream.StammdatenSectionKontakt
	StammdatenSectionPerson               = upstream.StammdatenSectionPerson
	StammdatenSectionQualifikation        = upstream.StammdatenSectionQualifikation
	StammdatenSectionSchoolClasses        = upstream.StammdatenSectionSchoolClasses
	StudentConsentAGB                     = upstream.StudentConsentAGB
	StudentConsentDataProcessing          = upstream.StudentConsentDataProcessing
	StudentConsentEmailContact            = upstream.StudentConsentEmailContact
	StudentConsentGranted                 = upstream.StudentConsentGranted
	StudentConsentPhoto                   = upstream.StudentConsentPhoto
	StudentConsentSourceEnrollment        = upstream.StudentConsentSourceEnrollment
	StudentConsentSourceTenantPortal      = upstream.StudentConsentSourceTenantPortal
	StudentConsentWithdrawn               = upstream.StudentConsentWithdrawn
	SubstitutionAssigned                  = upstream.SubstitutionAssigned
	SubstitutionEnded                     = upstream.SubstitutionEnded
)

var (
	ValidAuditLogSources   = upstream.ValidAuditLogSources
	NewAuthEvent           = upstream.NewAuthEvent
	NewDataDeletion        = upstream.NewDataDeletion
	NewDate                = upstream.NewDate
	StudentDocumentField   = upstream.StudentDocumentField
	TenantIDFromContext    = upstream.TenantIDFromContext
	TransactionFromContext = upstream.TransactionFromContext
	WithTenantID           = upstream.WithTenantID
	WithTransaction        = upstream.WithTransaction
)
