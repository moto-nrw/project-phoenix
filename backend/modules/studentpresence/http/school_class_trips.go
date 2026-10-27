package studentpresencehttp

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan/absencerecords"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// SchoolClassTrips records the class trip a Lehrkraft reports through "moto
// schule" (#4799) with the status-day write of POST /students/status-days/bulk:
// the same locks, the same refusal of days a child already has a status or a
// partial absence on, the same live flags and the same after-commit fan-out.
// Only the authorization differs. The school portal has no users:update; the
// caller proved the class assignment, and the locked child must still be in
// that class (inClass), else ErrStudentStatusDayReassigned.
type SchoolClassTrips struct{ rs *Resource }

// NewSchoolClassTrips builds the write over the collaborators the student
// routes use: the status-day service, the People Directory, the broadcaster
// and the guardian wake.
func NewSchoolClassTrips(cfg ResourceConfig) SchoolClassTrips {
	return SchoolClassTrips{rs: NewResource(cfg)}
}

// RecordClassTrip writes the status "Klassenfahrt" for every child on every
// date inside the caller's transaction. inClass decides on the locked row
// whether the child still belongs to the reported class.
func (t SchoolClassTrips) RecordClassTrip(ctx context.Context, studentIDs []int64, dates []calendar.Date, reason string, inClass func(schoolClass string) bool) error {
	if t.rs == nil || t.rs.StudentStatusDayService == nil || t.rs.PeopleDirectory == nil {
		return errors.New("school class trips: status days not configured")
	}
	if inClass == nil {
		return errors.New("school class trips: class check is required")
	}
	wc := t.rs.schoolClassWriteContext(tenant.FromContext(ctx), absencerecords.StudentStatusDayClassTrip, inClass)
	return t.rs.StudentStatusDayService.BulkCreateForDates(ctx, wc, studentIDs, absencerecords.StudentStatusDayClassTrip, reason, dates)
}

// schoolClassWriteContext is the status-day write context of a write moto
// schule makes for a class (#4799, #4791): the locked child is authorized
// only for status and only while it is still in the class (inClass), and the
// after-commit fan-out is the OGS write's.
func (rs *Resource) schoolClassWriteContext(tenantID int64, status string, inClass func(schoolClass string) bool) studentpresence.StatusDayWriteContext {
	return studentpresence.StatusDayWriteContext{
		TenantID: tenantID,
		StudentService: newStatusDayStudents(rs.PeopleDirectory, func(_ context.Context, student *peopleModule.StudentState, locked string) bool {
			return locked == status && student != nil && inClass(student.SchoolClass)
		}),
		AfterCommit: func(studentID int64) {
			rs.broadcastStudentUpdated(tenantID, studentID)
			rs.wakeChildGuardians(tenantID, studentID)
		},
	}
}
