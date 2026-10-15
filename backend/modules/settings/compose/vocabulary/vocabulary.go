// Package vocabulary names the Settings Platform records, repositories and constants
// the legacy composition still binds (#2747). The owner's composition
// re-exports them under its own path, so the legacy composition depends on the
// composition instead of the domain package. Shrink it as consumers move to the
// owner's public contract; never add a name nothing uses.
package vocabulary

import upstream "github.com/moto-nrw/project-phoenix/models/config"

type (
	CalendarDate                = upstream.CalendarDate
	SettingAuditRepository      = upstream.SettingAuditRepository
	SettingValue                = upstream.SettingValue
	SettingValueRepository      = upstream.SettingValueRepository
	StaffWorkSchedule           = upstream.StaffWorkSchedule
	StaffWorkScheduleRepository = upstream.StaffWorkScheduleRepository
	WorkTimeModel               = upstream.WorkTimeModel
	WorkTimeModelEntry          = upstream.WorkTimeModelEntry
	WorkTimeModelRepository     = upstream.WorkTimeModelRepository
)

const (
	EnrollmentDuplicateHandlingWarn                     = upstream.EnrollmentDuplicateHandlingWarn
	EnrollmentNotifyPerDecisionImmediate                = upstream.EnrollmentNotifyPerDecisionImmediate
	GroupModeFixedGroups                                = upstream.GroupModeFixedGroups
	GroupModeOpenCare                                   = upstream.GroupModeOpenCare
	KeyAccountLockoutDurationMinutes                    = upstream.KeyAccountLockoutDurationMinutes
	KeyAccountLockoutThreshold                          = upstream.KeyAccountLockoutThreshold
	KeyAttendanceEditScope                              = upstream.KeyAttendanceEditScope
	KeyAttendanceNFCEnabled                             = upstream.KeyAttendanceNFCEnabled
	KeyBirthdayDisplayEnabled                           = upstream.KeyBirthdayDisplayEnabled
	KeyBirthdayDisplayIncludeStaff                      = upstream.KeyBirthdayDisplayIncludeStaff
	KeyBlockCompleteScope                               = upstream.KeyBlockCompleteScope
	KeyBlockStartScope                                  = upstream.KeyBlockStartScope
	KeyCalendarCalDAVEnabled                            = upstream.KeyCalendarCalDAVEnabled
	KeyDeviceOnlineWindowMinutes                        = upstream.KeyDeviceOnlineWindowMinutes
	KeyEmailReplyToAddress                              = upstream.KeyEmailReplyToAddress
	KeyEnrollmentAllowSubmissionEdit                    = upstream.KeyEnrollmentAllowSubmissionEdit
	KeyEnrollmentAutoInviteGuardianOnApprove            = upstream.KeyEnrollmentAutoInviteGuardianOnApprove
	KeyEnrollmentBookingsAuthoritative                  = upstream.KeyEnrollmentBookingsAuthoritative
	KeyEnrollmentCaptchaSecretKey                       = upstream.KeyEnrollmentCaptchaSecretKey
	KeyEnrollmentCaptchaSiteKey                         = upstream.KeyEnrollmentCaptchaSiteKey
	KeyEnrollmentCareOfferingsEnabled                   = upstream.KeyEnrollmentCareOfferingsEnabled
	KeyEnrollmentChangeRequestEmailNotificationsEnabled = upstream.KeyEnrollmentChangeRequestEmailNotificationsEnabled
	KeyEnrollmentCollectGradeLevel                      = upstream.KeyEnrollmentCollectGradeLevel
	KeyEnrollmentCollectSchoolClass                     = upstream.KeyEnrollmentCollectSchoolClass
	KeyEnrollmentDefaultActivationMode                  = upstream.KeyEnrollmentDefaultActivationMode
	KeyEnrollmentDuplicateHandling                      = upstream.KeyEnrollmentDuplicateHandling
	KeyEnrollmentEnabled                                = upstream.KeyEnrollmentEnabled
	KeyEnrollmentGradeLevelMax                          = upstream.KeyEnrollmentGradeLevelMax
	KeyEnrollmentLegalAGBDisplayMode                    = upstream.KeyEnrollmentLegalAGBDisplayMode
	KeyEnrollmentLegalAGBDocumentURL                    = upstream.KeyEnrollmentLegalAGBDocumentURL
	KeyEnrollmentLegalAGBText                           = upstream.KeyEnrollmentLegalAGBText
	KeyEnrollmentLegalDSGVOEnabled                      = upstream.KeyEnrollmentLegalDSGVOEnabled
	KeyEnrollmentLegalDSGVOText                         = upstream.KeyEnrollmentLegalDSGVOText
	KeyEnrollmentLegalEmailContactEnabled               = upstream.KeyEnrollmentLegalEmailContactEnabled
	KeyEnrollmentLegalEmailContactText                  = upstream.KeyEnrollmentLegalEmailContactText
	KeyEnrollmentLegalPhotoEnabled                      = upstream.KeyEnrollmentLegalPhotoEnabled
	KeyEnrollmentLegalPhotoText                         = upstream.KeyEnrollmentLegalPhotoText
	KeyEnrollmentLegalTermsEnabled                      = upstream.KeyEnrollmentLegalTermsEnabled
	KeyEnrollmentNotificationEmails                     = upstream.KeyEnrollmentNotificationEmails
	KeyEnrollmentNotifyPerDecision                      = upstream.KeyEnrollmentNotifyPerDecision
	KeyEnrollmentOfferingChangesEnabled                 = upstream.KeyEnrollmentOfferingChangesEnabled
	KeyEnrollmentOfferingChangesLeadDays                = upstream.KeyEnrollmentOfferingChangesLeadDays
	KeyEnrollmentParentCourseRequestsEnabled            = upstream.KeyEnrollmentParentCourseRequestsEnabled
	KeyEnrollmentRejectedRetentionDays                  = upstream.KeyEnrollmentRejectedRetentionDays
	KeyGDPRWithdrawnEnrollmentRetentionDays             = upstream.KeyGDPRWithdrawnEnrollmentRetentionDays
	KeyEnrollmentRequireCaptcha                         = upstream.KeyEnrollmentRequireCaptcha
	KeyEnrollmentStatusTokenTTLDays                     = upstream.KeyEnrollmentStatusTokenTTLDays
	KeyEnrollmentWaitlistEnabled                        = upstream.KeyEnrollmentWaitlistEnabled
	KeyExcusedClearMode                                 = upstream.KeyExcusedClearMode
	KeyFederalState                                     = upstream.KeyFederalState
	KeyFeedbackDataRetentionDays                        = upstream.KeyFeedbackDataRetentionDays
	KeyFeedbackEnabled                                  = upstream.KeyFeedbackEnabled
	KeyFilesMaxStorageMB                                = upstream.KeyFilesMaxStorageMB
	KeyFilesStaffUploadEnabled                          = upstream.KeyFilesStaffUploadEnabled
	KeyGDPRStudentChangeLogRetentionDays                = upstream.KeyGDPRStudentChangeLogRetentionDays
	KeyGDPRTimeTrackingRetentionDays                    = upstream.KeyGDPRTimeTrackingRetentionDays
	KeyGDPRTimetableRetentionDays                       = upstream.KeyGDPRTimetableRetentionDays
	KeyGroupMode                                        = upstream.KeyGroupMode
	KeyGuardianInvitationTokenExpiryHours               = upstream.KeyGuardianInvitationTokenExpiryHours
	KeyMFAMode                                          = upstream.KeyMFAMode
	KeyMFATrustedDeviceDays                             = upstream.KeyMFATrustedDeviceDays
	KeyMFATrustedDeviceEnabled                          = upstream.KeyMFATrustedDeviceEnabled
	KeyMealPlanEnabled                                  = upstream.KeyMealPlanEnabled
	KeyMealRegistrationCutoffTime                       = upstream.KeyMealRegistrationCutoffTime
	KeyMealRegistrationEnabled                          = upstream.KeyMealRegistrationEnabled
	KeyNotificationsAbsenceApprovalEmail                = upstream.KeyNotificationsAbsenceApprovalEmail
	KeyOperationalOverviewScope                         = upstream.KeyOperationalOverviewScope
	KeyParentAbsenceReviewScope                         = upstream.KeyParentAbsenceReviewScope
	KeyParentRequestGroupLeaderReviewEnabled            = upstream.KeyParentRequestGroupLeaderReviewEnabled
	KeyParentRequestReviewScope                         = upstream.KeyParentRequestReviewScope
	KeyPresenceMode                                     = upstream.KeyPresenceMode
	KeyPrivacyConsentRetentionDays                      = upstream.KeyPrivacyConsentRetentionDays
	KeyRemindersActivityOverdueEnabled                  = upstream.KeyRemindersActivityOverdueEnabled
	KeyRemindersActivityStartEnabled                    = upstream.KeyRemindersActivityStartEnabled
	KeyRemindersActivityStartLeadMinutes                = upstream.KeyRemindersActivityStartLeadMinutes
	KeyRemindersPickupOverdueEnabled                    = upstream.KeyRemindersPickupOverdueEnabled
	KeyRemindersPickupUpcomingEnabled                   = upstream.KeyRemindersPickupUpcomingEnabled
	KeyRemindersPickupUpcomingLeadMinutes               = upstream.KeyRemindersPickupUpcomingLeadMinutes
	KeyRequirePickupOfferingReview                      = upstream.KeyRequirePickupOfferingReview
	KeySessionInactivityTimeoutMin                      = upstream.KeySessionInactivityTimeoutMin
	KeySickClearMode                                    = upstream.KeySickClearMode
	KeyStudentAbsenceEditScope                          = upstream.KeyStudentAbsenceEditScope
	KeyStudentPhotosEnabled                             = upstream.KeyStudentPhotosEnabled
	KeyTimeTrackingAccountStartDate                     = upstream.KeyTimeTrackingAccountStartDate
	KeyTimeTrackingDeviationToleranceMinutes            = upstream.KeyTimeTrackingDeviationToleranceMinutes
	KeyTimeTrackingEnforcePlannedStart                  = upstream.KeyTimeTrackingEnforcePlannedStart
	KeyTimeTrackingPlannedStartToleranceMinutes         = upstream.KeyTimeTrackingPlannedStartToleranceMinutes
	KeyTimeTrackingRequireDeviationReason               = upstream.KeyTimeTrackingRequireDeviationReason
	KeyTimetableCompleteLeadMinutes                     = upstream.KeyTimetableCompleteLeadMinutes
	KeyTimetableEnforcePlannedEnd                       = upstream.KeyTimetableEnforcePlannedEnd
	KeyTimetableOverdueThresholdMinutes                 = upstream.KeyTimetableOverdueThresholdMinutes
	KeyTimetableStartLeadMinutes                        = upstream.KeyTimetableStartLeadMinutes
	KeyWebExceedParticipantLimit                        = upstream.KeyWebExceedParticipantLimit
	MFAModeOff                                          = upstream.MFAModeOff
	OverviewScopeAllStaff                               = upstream.OverviewScopeAllStaff
	PresenceModeBinary                                  = upstream.PresenceModeBinary
	PresenceModeDetailed                                = upstream.PresenceModeDetailed
	StudentAbsenceEditScopeAllStaff                     = upstream.StudentAbsenceEditScopeAllStaff
)

var (
	DailyTargetFromModel    = upstream.DailyTargetFromModel
	DailyTargetFromSchedule = upstream.DailyTargetFromSchedule
)
