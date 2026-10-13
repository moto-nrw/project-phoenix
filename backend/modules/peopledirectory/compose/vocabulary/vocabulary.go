// Package vocabulary names the People Directory records, repositories and constants
// the legacy composition still binds (#2747). The owner's composition
// re-exports them under its own path, so the legacy composition depends on the
// composition instead of the domain package. Shrink it as consumers move to the
// owner's public contract; never add a name nothing uses.
package vocabulary

import upstream "github.com/moto-nrw/project-phoenix/models/users"

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
	CaregiverBindingLocker             = upstream.CaregiverBindingLocker
	CompanionChangeRecorder            = upstream.CompanionChangeRecorder
	CompanionLink                      = upstream.CompanionLink
	DatabaseError                      = upstream.DatabaseError
	DepartureMode                      = upstream.DepartureMode
	DeparturePlanSnapshot              = upstream.DeparturePlanSnapshot
	FamilyProtectionEventRepository    = upstream.FamilyProtectionEventRepository
	GuardianFinancialDataRepository    = upstream.GuardianFinancialDataRepository
	GuardianPhoneNumber                = upstream.GuardianPhoneNumber
	GuardianPhoneNumberRepository      = upstream.GuardianPhoneNumberRepository
	GuardianProfile                    = upstream.GuardianProfile
	GuardianProfileRepository          = upstream.GuardianProfileRepository
	Guest                              = upstream.Guest
	GuestRepository                    = upstream.GuestRepository
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
	PersonRepository                   = upstream.PersonRepository
	PhoneType                          = upstream.PhoneType
	QueryFilter                        = upstream.QueryFilter
	QueryOptions                       = upstream.QueryOptions
	RequestQueueFilters                = upstream.RequestQueueFilters
	Staff                              = upstream.Staff
	StaffFinancialDataRepository       = upstream.StaffFinancialDataRepository
	StaffMasterDataRepository          = upstream.StaffMasterDataRepository
	StaffMessageReadRepository         = upstream.StaffMessageReadRepository
	StaffMessageRepository             = upstream.StaffMessageRepository
	StaffMessageThreadRepository       = upstream.StaffMessageThreadRepository
	StaffQualificationRepository       = upstream.StaffQualificationRepository
	StaffRepository                    = upstream.StaffRepository
	StaffWithRoleInfo                  = upstream.StaffWithRoleInfo
	Student                            = upstream.Student
	StudentCompanion                   = upstream.StudentCompanion
	StudentCompanionRepository         = upstream.StudentCompanionRepository
	StudentConsentState                = upstream.StudentConsentState
	StudentDataChangeRequest           = upstream.StudentDataChangeRequest
	StudentDataChangeRequestRepository = upstream.StudentDataChangeRequestRepository
	StudentGuardian                    = upstream.StudentGuardian
	StudentGuardianRepository          = upstream.StudentGuardianRepository
	StudentRepository                  = upstream.StudentRepository
	StudentStatus                      = upstream.StudentStatus
	StudentWithGroupInfo               = upstream.StudentWithGroupInfo
	Teacher                            = upstream.Teacher
	TeacherRepository                  = upstream.TeacherRepository
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
	SortDesc                           = upstream.SortDesc
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
	ErrRepositoryNotFound              = upstream.ErrRepositoryNotFound
	ErrStudentGuardianNotFound         = upstream.ErrStudentGuardianNotFound
	ErrStudentRowMissing               = upstream.ErrStudentRowMissing
	IsRepositoryNoRows                 = upstream.IsRepositoryNoRows
	IsUniqueViolation                  = upstream.IsUniqueViolation
	IsUniqueViolationOn                = upstream.IsUniqueViolationOn
	NewQueryFilter                     = upstream.NewQueryFilter
	NewQueryOptions                    = upstream.NewQueryOptions
	AccompaniedWeekdays                = upstream.AccompaniedWeekdays
	ContextWithCompanionChangeRecorder = upstream.ContextWithCompanionChangeRecorder
	EnrolledOn                         = upstream.EnrolledOn
	MissingStudentError                = upstream.MissingStudentError
	MonthDayOf                         = upstream.MonthDayOf
	OptionalCalendarDate               = upstream.OptionalCalendarDate
	RecordCompanionChange              = upstream.RecordCompanionChange
	RenderCalendarDate                 = upstream.RenderCalendarDate
	RowMissing                         = upstream.RowMissing
	WithAccompaniedDays                = upstream.WithAccompaniedDays
)
