package test

import (
	"github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/schedule"
)

// The retained School Structure suites (the tests of
// database/repositories/education, services/education and the owner's HTTP
// adapters) may not import the retained models (#2742). They name the rows,
// queries and values they arrange and assert through this test support.
// Every entry goes with the last suite that uses it.

type (
	EducationModel              = education.Model
	EducationGroup              = education.Group
	EducationGroupRoom          = education.GroupRoom
	EducationGroupListQuery     = education.GroupListQuery
	EducationGroupTeacher       = education.GroupTeacher
	EducationClassTeacher       = education.ClassTeacher
	EducationGroupSubstitution  = education.GroupSubstitution
	EducationSubstitutionStaff  = education.SubstitutionStaff
	EducationSubstitutionPerson = education.SubstitutionPerson
	EducationStaffGroupID       = education.StaffGroupID
	EducationTeacher            = education.Teacher
	EducationTeacherPerson      = education.TeacherPerson
	EducationSubstitutionChange = education.SubstitutionChange
	EducationGradeTransition    = education.GradeTransition
)

const (
	EducationGroupSubstitutionTypeGroupHandover = education.GroupSubstitutionTypeGroupHandover
	EducationGroupSubstitutionTypeLegacy        = education.GroupSubstitutionTypeLegacy
)

// AuditStammdatenSectionSchoolClasses is the Stammdaten audit section of a
// staff member's class assignments (#1772).
const AuditStammdatenSectionSchoolClasses = audit.StammdatenSectionSchoolClasses

// The roster statuses the grade transition suites arrange and assert.
const (
	ScheduleAttendanceStatusExpected = schedule.AttendanceStatusExpected
	ScheduleAttendanceStatusPresent  = schedule.AttendanceStatusPresent
	ScheduleAttendanceStatusAbsent   = schedule.AttendanceStatusAbsent
	ScheduleInstanceStatusCompleted  = schedule.InstanceStatusCompleted
)
