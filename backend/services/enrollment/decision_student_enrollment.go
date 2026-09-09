package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/models/users"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
)

// DecisionStudentEnrollment is the bounded student owner seam used by approval,
// renewal and accepted form changes. It cannot update attendance.
type DecisionStudentEnrollment interface {
	ReadEnrollmentStudent(context.Context, int64, string) (peopleEnrollment.Record, error)
	LockEnrollmentClassWrites(context.Context) error
	ApplyEnrollmentProfile(context.Context, int64, peopleEnrollment.ProfilePatch) error
	CreateEnrollmentStudent(context.Context, peopleEnrollment.Input) (peopleEnrollment.CreatedStudent, error)
	RenewEnrollmentStudent(context.Context, int64, peopleEnrollment.Input) error
}

func enrollmentProfilePatch(before, after *users.Student) peopleEnrollment.ProfilePatch {
	return peopleEnrollment.ProfilePatch{
		HealthInfoSet: enrollmentValueChanged(before.HealthInfo, after.HealthInfo), HealthInfo: after.HealthInfo,
		ExtraInfoSet: enrollmentValueChanged(before.ExtraInfo, after.ExtraInfo), ExtraInfo: after.ExtraInfo,
		PhotoConsentGivenAtSet: enrollmentValueChanged(before.PhotoConsentGivenAt, after.PhotoConsentGivenAt), PhotoConsentGivenAt: after.PhotoConsentGivenAt,
		PhotoConsentGivenBySet: enrollmentValueChanged(before.PhotoConsentGivenBy, after.PhotoConsentGivenBy), PhotoConsentGivenBy: after.PhotoConsentGivenBy,
		AGBAcceptedAtSet: enrollmentValueChanged(before.AGBAcceptedAt, after.AGBAcceptedAt), AGBAcceptedAt: after.AGBAcceptedAt,
		DataProcessingAcceptedAtSet: enrollmentValueChanged(before.DataProcessingAcceptedAt, after.DataProcessingAcceptedAt), DataProcessingAcceptedAt: after.DataProcessingAcceptedAt,
		EmailContactAcceptedAtSet: enrollmentValueChanged(before.EmailContactAcceptedAt, after.EmailContactAcceptedAt), EmailContactAcceptedAt: after.EmailContactAcceptedAt,
	}
}

func enrollmentValueChanged[T comparable](before, after *T) bool {
	if before == nil || after == nil {
		return before != after
	}
	return *before != *after
}

func enrollmentStudentInput(student *users.Student) peopleEnrollment.Input {
	input := peopleEnrollment.Input{
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: string(student.Status),
		GuardianEmail: student.GuardianEmail, GuardianPhone: student.GuardianPhone,
	}
	if student.EnrolledFrom != nil {
		input.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		input.EnrolledUntil = student.EnrolledUntil.String()
	}
	return input
}
