package test

import (
	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/models/activities"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// The retained People Directory repository suites (database/repositories/users)
// may import neither the retained models nor the authorization runtime
// (#2727). They name the rows, values and errors they arrange and assert
// through this test support. Every entry goes with the last suite that uses
// it.

type (
	AnnouncementDeliveryRecipient  = users.AnnouncementDeliveryRecipient
	AnnouncementFeedItem           = users.AnnouncementFeedItem
	AnnouncementFeedScope          = users.AnnouncementFeedScope
	AnnouncementLetterChildStatus  = users.AnnouncementLetterChildStatus
	AnnouncementRecipientStatus    = users.AnnouncementRecipientStatus
	AnnouncementStats              = users.AnnouncementStats
	CareWithdrawalCompletion       = users.CareWithdrawalCompletion
	CareWithdrawalCompletionFilter = users.CareWithdrawalCompletionFilter
	CaregiverBindingLocker         = users.CaregiverBindingLocker
	FamilyProtectionEvent          = users.FamilyProtectionEvent
	GuardianPhoneNumber            = users.GuardianPhoneNumber
	GuardianProfile                = users.GuardianProfile
	GuardianProfileRepository      = users.GuardianProfileRepository
	Guest                          = users.Guest
	ParentAnnouncement             = users.ParentAnnouncement
	ParentAnnouncementOption       = users.ParentAnnouncementOption
	ParentAnnouncementRepository   = users.ParentAnnouncementRepository
	ParentAnnouncementTarget       = users.ParentAnnouncementTarget
	ParentMessage                  = users.ParentMessage
	ParentMessageThread            = users.ParentMessageThread
	ParentRequestShareEvent        = users.ParentRequestShareEvent
	Person                         = users.Person
	Staff                          = users.Staff
	Student                        = users.Student
	StudentCompanion               = users.StudentCompanion
	StudentDataChangeRequest       = users.StudentDataChangeRequest
	StudentGuardian                = users.StudentGuardian
	StudentGuardianRepository      = users.StudentGuardianRepository
	StudentStatus                  = users.StudentStatus
	StudentWithGroupInfo           = users.StudentWithGroupInfo
	Teacher                        = users.Teacher
	QueryOptions                   = users.QueryOptions
	QueryFilter                    = users.QueryFilter
	RequestQueueFilters            = users.RequestQueueFilters
	StudentEnrollment              = activities.StudentEnrollment
	ActivityDate                   = activities.Date
)

const (
	AnnouncementTargetActivityGroup        = users.AnnouncementTargetActivityGroup
	AnnouncementTargetClass                = users.AnnouncementTargetClass
	AnnouncementTargetPendingEnrollment    = users.AnnouncementTargetPendingEnrollment
	AnnouncementTargetSchoolAll            = users.AnnouncementTargetSchoolAll
	AnnouncementTargetStudent              = users.AnnouncementTargetStudent
	CareWithdrawalTriggerBookingExpired    = users.CareWithdrawalTriggerBookingExpired
	CareWithdrawalTriggerDirectSchool      = users.CareWithdrawalTriggerDirectSchool
	DataChangeStatusApproved               = users.DataChangeStatusApproved
	DataChangeStatusPending                = users.DataChangeStatusPending
	DataChangeStatusRejected               = users.DataChangeStatusRejected
	DataChangeTargetPerson                 = users.DataChangeTargetPerson
	ParentAnnouncementPriorityInfo         = users.ParentAnnouncementPriorityInfo
	ParentAnnouncementResponseMultiChoice  = users.ParentAnnouncementResponseMultiChoice
	ParentAnnouncementResponseSingleChoice = users.ParentAnnouncementResponseSingleChoice
	ParentMessageKindEvent                 = users.ParentMessageKindEvent
	ParentMessageSenderGuardian            = users.ParentMessageSenderGuardian
	ParentMessageSenderStaff               = users.ParentMessageSenderStaff
	ParentMessageSenderSystem              = users.ParentMessageSenderSystem
	PhoneTypeHome                          = users.PhoneTypeHome
	PhoneTypeMobile                        = users.PhoneTypeMobile
	PhoneTypeWork                          = users.PhoneTypeWork
	StudentStatusActive                    = users.StudentStatusActive
	StudentStatusAlumnus                   = users.StudentStatusAlumnus
	StudentStatusInactive                  = users.StudentStatusInactive
	StudentStatusPending                   = users.StudentStatusPending
	GuardianPermissionEnrollmentSubmit     = authorize.GuardianPermissionEnrollmentSubmit
	GuardianPermissionPollResponse         = authorize.GuardianPermissionPollResponse
	GuardianPermissionPortalAccess         = authorize.GuardianPermissionPortalAccess
	GuardianRoleLegalGuardian              = authorize.GuardianRoleLegalGuardian
	GuardianRolePickupOnly                 = authorize.GuardianRolePickupOnly
	GuardianRoleSocialWorker               = authorize.GuardianRoleSocialWorker
	WeekdaySunday                          = activities.WeekdaySunday
)

var (
	ErrAnnouncementPublished     = users.ErrAnnouncementPublished
	ErrChangeRequestNotFound     = users.ErrChangeRequestNotFound
	ErrChangeRequestNotPending   = users.ErrChangeRequestNotPending
	ErrGuardianAccountConflict   = users.ErrGuardianAccountConflict
	ErrGuardianProfileNotFound   = users.ErrGuardianProfileNotFound
	ErrStudentGuardianNotFound   = users.ErrStudentGuardianNotFound
	EnrolledOn                   = users.EnrolledOn
	WithAccompaniedDays          = users.WithAccompaniedDays
	NewQueryOptions              = users.NewQueryOptions
	NewQueryFilter               = users.NewQueryFilter
	ApplyStudentGuardianRole     = authorize.ApplyStudentGuardianRole
	StudentGuardianHasPermission = authorize.StudentGuardianHasPermission
)
