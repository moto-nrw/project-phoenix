package repositories

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
)

type studentOwners struct {
	membership schoolmembership.Capability
	care       careplan.StudentProfileCommands
}

func NewStudentOwners(membership schoolmembership.Capability, care careplan.StudentProfileCommands) peopleCompose.StudentOwners {
	return studentOwners{membership: membership, care: care}
}

func (s studentOwners) LockClassWrites(ctx context.Context, exclusive bool) error {
	return s.membership.LockStudentClassWrites(ctx, exclusive)
}

func enrollmentOf(record peopledirectory.StudentRecord) schoolmembership.StudentEnrollment {
	return schoolmembership.StudentEnrollment{
		StudentID: record.ID, SchoolClass: record.SchoolClass, Status: record.Status,
		GroupID:      record.GroupID,
		EnrolledFrom: record.EnrolledFrom, EnrolledUntil: record.EnrolledUntil,
	}
}

func (s studentOwners) Enroll(ctx context.Context, record peopledirectory.StudentRecord) (int64, error) {
	return s.membership.Enroll(ctx, enrollmentOf(record))
}

func (s studentOwners) Renew(ctx context.Context, record peopledirectory.StudentRecord) (int64, error) {
	id, err := s.membership.RenewEnrollment(ctx, enrollmentOf(record))
	if err != nil {
		return 0, err
	}
	if id == 0 {
		return 0, peopledirectory.ErrStudentNotFound
	}
	return id, err
}

func (s studentOwners) SaveCare(ctx context.Context, membershipID int64, record peopledirectory.StudentRecord, plan peopledirectory.StudentPlan, note *string, touched bool) error {
	modes := make(map[string][]string, len(plan.AllowedDepartureModes))
	for day, values := range plan.AllowedDepartureModes {
		for _, value := range values {
			modes[day] = append(modes[day], string(value))
		}
	}
	days := make(map[string]string, len(plan.DepartureDays))
	for day, value := range plan.DepartureDays {
		days[day] = string(value)
	}
	return s.care.SaveStudentCareProfile(ctx, careplan.StudentCareProfile{
		MembershipID: membershipID, SupervisorNotes: record.SupervisorNotes, HealthInfo: record.HealthInfo,
		PickupStatus: record.PickupStatus, Sick: record.Sick != nil && *record.Sick,
		SickSince: record.SickSince, Excused: record.Excused != nil && *record.Excused, ExcusedSince: record.ExcusedSince,
	}, &careplan.StudentDeparturePlan{
		MembershipID: membershipID, PickupStatus: plan.AllowedDepartureModes.LegacyPickupStatus(),
		AllowedDepartureModes: modes, DepartureDays: days,
		BusDays: plan.BusDays, PickupDays: plan.PickupDays, CompanionNote: note, PlanTouched: touched,
	})
}
