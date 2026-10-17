// Package vocabulary names the People Directory records, repository contracts
// and constants the legacy composition still binds (#2747, #3990) under the
// owner's composition path. Shrink it as consumers move to the owner's public
// contract; never add a name nothing uses.
package vocabulary

import (
	upstream "github.com/moto-nrw/project-phoenix/models/users"
)

type (
	ActiveCaregiver                    = upstream.ActiveCaregiver
	AllowedDepartureModes              = upstream.AllowedDepartureModes
	BirthdayEntry                      = upstream.BirthdayEntry
	BlockerGroup                       = upstream.BlockerGroup
	BlockerSubstitution                = upstream.BlockerSubstitution
	BlockerSupervision                 = upstream.BlockerSupervision
	CalendarDate                       = upstream.CalendarDate
	CareExitSourceOffering             = upstream.CareExitSourceOffering
	CareWithdrawalCompletion           = upstream.CareWithdrawalCompletion
	CareWithdrawalCompletionFilter     = upstream.CareWithdrawalCompletionFilter
	CareWithdrawalCompletionRepository = upstream.CareWithdrawalCompletionRepository
	CompanionChangeRecorder            = upstream.CompanionChangeRecorder
	CompanionLink                      = upstream.CompanionLink
	DepartureMode                      = upstream.DepartureMode
	DeparturePlanSnapshot              = upstream.DeparturePlanSnapshot
	FamilyProtectionEventRepository    = upstream.FamilyProtectionEventRepository
	GuardianEmergencyContactRow        = upstream.GuardianEmergencyContactRow
	GuardianFinancialDataRepository    = upstream.GuardianFinancialDataRepository
	GuardianPhoneNumber                = upstream.GuardianPhoneNumber
	GuardianProfile                    = upstream.GuardianProfile
	Guest                              = upstream.Guest
	MonthDay                           = upstream.MonthDay
	ParentAnnouncementRepository       = upstream.ParentAnnouncementRepository
	ParentMessageReadRepository        = upstream.ParentMessageReadRepository
	ParentMessageRepository            = upstream.ParentMessageRepository
	ParentMessageThreadRepository      = upstream.ParentMessageThreadRepository
	ParentRequestEvent                 = upstream.ParentRequestEvent
	ParentRequestEventRepository       = upstream.ParentRequestEventRepository
	ParentRequestShareEventRepository  = upstream.ParentRequestShareEventRepository
	Person                             = upstream.Person
	PersonAccount                      = upstream.PersonAccount
	PhoneType                          = upstream.PhoneType
	Staff                              = upstream.Staff
	StaffMessageReadRepository         = upstream.StaffMessageReadRepository
	StaffMessageRepository             = upstream.StaffMessageRepository
	StaffMessageThreadRepository       = upstream.StaffMessageThreadRepository
	StaffWithRoleInfo                  = upstream.StaffWithRoleInfo
	Student                            = upstream.Student
	StudentCompanion                   = upstream.StudentCompanion
	StudentConsentState                = upstream.StudentConsentState
	StudentDataChangeRequest           = upstream.StudentDataChangeRequest
	StudentGuardian                    = upstream.StudentGuardian
	StudentStatus                      = upstream.StudentStatus
	StudentWithGroupInfo               = upstream.StudentWithGroupInfo
	Teacher                            = upstream.Teacher
)

const (
	BirthdayKindStaff                  = upstream.BirthdayKindStaff
	CareWithdrawalStatePending         = upstream.CareWithdrawalStatePending
	DataChangeStatusApproved           = upstream.DataChangeStatusApproved
	DataChangeStatusAutoApplied        = upstream.DataChangeStatusAutoApplied
	DataChangeStatusPending            = upstream.DataChangeStatusPending
	DataChangeStatusRejected           = upstream.DataChangeStatusRejected
	ParentMessageEventRequestStatus    = upstream.ParentMessageEventRequestStatus
	ParentMessageRequestCareSchedule   = upstream.ParentMessageRequestCareSchedule
	ParentMessageRequestPickupChange   = upstream.ParentMessageRequestPickupChange
	ParentMessageRequestStatusDone     = upstream.ParentMessageRequestStatusDone
	ParentMessageRequestStatusOpen     = upstream.ParentMessageRequestStatusOpen
	ParentMessageRequestStatusRejected = upstream.ParentMessageRequestStatusRejected
	ParentMessageSenderGuardian        = upstream.ParentMessageSenderGuardian
	ParentMessageSenderStaff           = upstream.ParentMessageSenderStaff
	ParentRequestEventCorrected        = upstream.ParentRequestEventCorrected
	ParentRequestEventDecided          = upstream.ParentRequestEventDecided
	ParentRequestEventGuardianEdit     = upstream.ParentRequestEventGuardianEdit
	ParentRequestEventMarkedDone       = upstream.ParentRequestEventMarkedDone
	ParentRequestEventSubmitted        = upstream.ParentRequestEventSubmitted
	ParentRequestTypeCareSchedule      = upstream.ParentRequestTypeCareSchedule
	ParentRequestTypePickupChange      = upstream.ParentRequestTypePickupChange
	StudentStatusActive                = upstream.StudentStatusActive
	StudentStatusAlumnus               = upstream.StudentStatusAlumnus
	StudentStatusInactive              = upstream.StudentStatusInactive
	StudentStatusPending               = upstream.StudentStatusPending
)

var (
	CompanionWeekdayKeys               = upstream.CompanionWeekdayKeys
	ErrChangeRequestNotDecided         = upstream.ErrChangeRequestNotDecided
	ErrChangeRequestNotFound           = upstream.ErrChangeRequestNotFound
	ErrChangeRequestNotPending         = upstream.ErrChangeRequestNotPending
	ErrCompanionInvalidWeekday         = upstream.ErrCompanionInvalidWeekday
	ErrCompanionLockBusy               = upstream.ErrCompanionLockBusy
	ErrCompanionSelfLink               = upstream.ErrCompanionSelfLink
	ErrCompanionStudentIDRequired      = upstream.ErrCompanionStudentIDRequired
	ErrCompanionWouldLoseDeparture     = upstream.ErrCompanionWouldLoseDeparture
	ErrGuardianAccountConflict         = upstream.ErrGuardianAccountConflict
	ErrGuardianProfileNotFound         = upstream.ErrGuardianProfileNotFound
	ErrPersonnelNumberConflict         = upstream.ErrPersonnelNumberConflict
	ErrStudentGuardianNotFound         = upstream.ErrStudentGuardianNotFound
	ErrStudentRowMissing               = upstream.ErrStudentRowMissing
	AccompaniedWeekdays                = upstream.AccompaniedWeekdays
	ContextWithCompanionChangeRecorder = upstream.ContextWithCompanionChangeRecorder
	EnrolledOn                         = upstream.EnrolledOn
	MonthDayOf                         = upstream.MonthDayOf
	OptionalCalendarDate               = upstream.OptionalCalendarDate
	RecordCompanionChange              = upstream.RecordCompanionChange
	RenderCalendarDate                 = upstream.RenderCalendarDate
	WithAccompaniedDays                = upstream.WithAccompaniedDays
)
