package owners

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (s *CareScheduleRequestService) pickupApprovals() (carerequests.PickupApprovals, error) {
	adapter := pickupApprovalAdapter{s}
	exceptions, err := compose.NewPickupApprovalExceptions(s.RequestRecords)
	if err != nil {
		return nil, err
	}
	service, err := compose.NewPickupApprovals(compose.PickupApprovalDependencies{
		People: adapter, Presence: adapter, Exceptions: exceptions, Excusal: adapter, Locker: s.DayLocker, Today: s.todayDate, Fingerprint: securityruntime.Fingerprint,
	})
	if err != nil {
		return nil, err
	}
	return service, nil
}

type pickupApprovalAdapter struct{ s *CareScheduleRequestService }

func (a pickupApprovalAdapter) EnrolledUntil(ctx context.Context, id int64) (*timezone.Date, error) {
	student, err := a.s.People.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, requestStudentReadError(err)
	}
	if student.EnrolledUntil == "" {
		return nil, nil
	}
	date := timezone.Date(student.EnrolledUntil)
	return &date, nil
}

func (a pickupApprovalAdapter) CurrentStaffID(ctx context.Context) (int64, bool, error) {
	return a.s.UserContext.CurrentStaffID(ctx)
}

func (a pickupApprovalAdapter) ValidatePickupPerson(ctx context.Context, studentID, guardianProfileID int64) error {
	if a.s.PickupPermissionApprovals == nil {
		return errors.New("schedule: pickup permission approvals not configured")
	}
	return a.s.ValidatePickupPerson(ctx, studentID, guardianProfileID)
}

func (a pickupApprovalAdapter) LockStudentAttendance(ctx context.Context, id int64) error {
	return a.s.Attendance.LockStudentAttendance(ctx, id)
}

func (a pickupApprovalAdapter) AttendanceCompletion(ctx context.Context, id int64, date timezone.Date) (open, completed bool, err error) {
	rows, err := a.s.Attendance.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{id}, FromDate: date.String(), UntilDate: date.String()})
	if err != nil {
		return false, false, err
	}
	open, completed = studentpresence.AttendanceCompletion(rows)
	return open, completed, nil
}

func (a pickupApprovalAdapter) Preview(ctx context.Context, id int64, date timezone.Date, pickup time.Time) ([]carerequests.Block, error) {
	return a.s.PickupAutoExcusal.Preview(ctx, id, date, pickup)
}

func (a pickupApprovalAdapter) Sync(ctx context.Context, id int64) error {
	_, err := a.s.PickupAutoExcusal.Sync(ctx, id)
	return err
}
