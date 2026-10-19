//go:build testhooks

package compose

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// CareScheduleRepositories are the retained repositories served by Care Plan.
type CareScheduleRepositories struct {
	StudentArrivalSchedule    ports.StudentArrivalScheduleRecords
	StudentArrivalException   ports.StudentArrivalExceptionRecords
	StudentArrivalNote        ports.StudentArrivalNoteRecords
	StudentPickupSchedule     ports.StudentPickupScheduleRecords
	StudentPickupException    ports.StudentPickupExceptionRecords
	StudentPickupNote         ports.StudentPickupNoteRecords
	CareScheduleChangeRequest ports.CareScheduleChangeRequestRecords
}

// NewCareScheduleRepositories builds the retained repositories over Care Plan records.
func NewCareScheduleRepositories(rows ports.CarePlanRows) CareScheduleRepositories {
	return CareScheduleRepositories{
		StudentArrivalSchedule:    application.NewCareArrivalScheduleRows(rows),
		StudentArrivalException:   application.NewCareArrivalExceptionRows(rows),
		StudentArrivalNote:        application.NewCareArrivalNoteRows(rows),
		StudentPickupSchedule:     application.NewCarePickupScheduleRows(rows),
		StudentPickupException:    application.NewCarePickupExceptionRows(rows),
		StudentPickupNote:         application.NewCarePickupNoteRows(rows),
		CareScheduleChangeRequest: application.NewCareScheduleRequestRows(rows),
	}
}

// EnrollmentPickupSchedules binds Enrollment's weekly pickup port to the
// pickup schedules.
func (r CareScheduleRepositories) EnrollmentPickupSchedules() application.EnrollmentPickupSchedules {
	return application.NewEnrollmentPickupSchedules(r.StudentPickupSchedule)
}

// EnrollmentArrivalSchedules binds Enrollment's weekly arrival port to the
// arrival schedules.
func (r CareScheduleRepositories) EnrollmentArrivalSchedules() application.EnrollmentArrivalSchedules {
	return application.NewEnrollmentArrivalSchedules(r.StudentArrivalSchedule)
}

// NewCarePickupNoteRows serves the pickup notes from Care Plan.
func NewCarePickupNoteRows(rows ports.CarePlanRows) ports.CarePickupNoteRows {
	return application.NewCarePickupNoteRows(rows)
}

// NewCalendarPeriodUsageRepository counts the rows that reference each
// calendar period.
func NewCalendarPeriodUsageRepository(enrollment application.EnrollmentPhaseQueries, countReferences func(context.Context) (map[int64]ports.CalendarPeriodReferences, error)) ports.CalendarPeriodUsageRepository {
	return application.NewCalendarPeriodUsageRepository(enrollment, countReferences)
}
