package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// fakeDirectories names nobody: no staff member has a person, no child is
// an alumnus or has a name.
func fakeDirectories() ports.RepositoryDirectories {
	return ports.RepositoryDirectories{
		StaffPersons: func(context.Context, []int64, bool) (map[int64]int64, error) { return map[int64]int64{}, nil },
		PersonNames: func(context.Context, []int64) (map[int64]ports.PersonName, error) {
			return map[int64]ports.PersonName{}, nil
		},
		StudentAlumni: func(context.Context, []int64) (map[int64]bool, error) { return map[int64]bool{}, nil },
		StudentNames: func(context.Context, []int64) (map[int64]ports.PersonName, error) {
			return map[int64]ports.PersonName{}, nil
		},
	}
}

// fakeEnrollments are the owner's enrollments of retained fake rows.
func fakeEnrollments(rows []*activityContract.StudentEnrollment) []timetable.StudentEnrollment {
	if rows == nil {
		return nil
	}
	values := make([]timetable.StudentEnrollment, 0, len(rows))
	for _, row := range rows {
		input := publicStudentEnrollmentInput(row)
		values = append(values, timetable.StudentEnrollment{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			StudentID: input.StudentID, ActivityGroupID: input.ActivityGroupID, ValidFrom: input.ValidFrom,
			ValidUntil: input.ValidUntil, CalendarPeriodID: input.CalendarPeriodID,
			EnrollmentRequestChildID: input.EnrollmentRequestChildID, SelectedWeekdays: input.SelectedWeekdays,
			AttendanceStatus: input.AttendanceStatus, Weekday: input.Weekday,
		})
	}
	return values
}

// fakeSupervisors are the owner's planned supervisors of retained fake rows.
func fakeSupervisors(rows []*activityContract.SupervisorPlanned) []timetable.PlannedSupervisor {
	if rows == nil {
		return nil
	}
	values := make([]timetable.PlannedSupervisor, 0, len(rows))
	for _, row := range rows {
		input := publicPlannedSupervisorInput(row)
		values = append(values, timetable.PlannedSupervisor{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			StaffID: input.StaffID, GroupID: input.GroupID, IsPrimary: input.IsPrimary, ValidFrom: input.ValidFrom,
			ValidUntil: input.ValidUntil, CalendarPeriodID: input.CalendarPeriodID, Weekday: input.Weekday,
		})
	}
	return values
}

// fakeEnrollmentRow is the retained row of an enrollment write.
func fakeEnrollmentRow(input timetable.StudentEnrollmentInput) *activityContract.StudentEnrollment {
	return legacyStudentEnrollment(timetable.StudentEnrollment{
		StudentID: input.StudentID, ActivityGroupID: input.ActivityGroupID, ValidFrom: input.ValidFrom,
		ValidUntil: input.ValidUntil, CalendarPeriodID: input.CalendarPeriodID,
		EnrollmentRequestChildID: input.EnrollmentRequestChildID, SelectedWeekdays: input.SelectedWeekdays,
		AttendanceStatus: input.AttendanceStatus, Weekday: input.Weekday,
	})
}

// fakeSupervisorRow is the retained row of a planned supervisor write.
func fakeSupervisorRow(input timetable.PlannedSupervisorInput) *activityContract.SupervisorPlanned {
	return legacyPlannedSupervisor(timetable.PlannedSupervisor{
		StaffID: input.StaffID, GroupID: input.GroupID, IsPrimary: input.IsPrimary, ValidFrom: input.ValidFrom,
		ValidUntil: input.ValidUntil, CalendarPeriodID: input.CalendarPeriodID, Weekday: input.Weekday,
	})
}

// fakeInstanceStaff are the owner's assignments of retained fake rows.
func fakeInstanceStaff(rows []*scheduleModels.InstanceStaff) []timetable.InstanceStaff {
	if rows == nil {
		return nil
	}
	values := make([]timetable.InstanceStaff, 0, len(rows))
	for _, row := range rows {
		values = append(values, timetable.InstanceStaff{
			ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
			InstanceID: row.InstanceID, StaffID: row.StaffID, RoomID: row.RoomID, IsPrimary: row.IsPrimary,
			IsSubstitute: row.IsSubstitute, IsAbsent: row.IsAbsent, AbsenceReason: row.AbsenceReason, SickAbsenceID: row.SickAbsenceID,
		})
	}
	return values
}
