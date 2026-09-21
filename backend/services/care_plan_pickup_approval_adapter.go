package services

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

func (s *careScheduleRequestService) pickupApprovals() (carerequests.PickupApprovals, error) {
	adapter := pickupApprovalAdapter{s}
	exceptions, err := compose.NewPickupApprovalExceptions(s.requestRecords)
	if err != nil {
		return nil, err
	}
	service, err := compose.NewPickupApprovals(compose.PickupApprovalDependencies{
		People: adapter, Presence: adapter, Exceptions: exceptions, Excusal: adapter, Locker: s.dayLocker, Today: s.todayDate, Fingerprint: securityruntime.Fingerprint,
	})
	if err != nil {
		return nil, err
	}
	return service, nil
}

type pickupApprovalAdapter struct{ s *careScheduleRequestService }

func (a pickupApprovalAdapter) EnrolledUntil(ctx context.Context, id int64) (*timezone.Date, error) {
	student, err := a.s.people.FindStudentRecord(ctx, id)
	if err != nil {
		return nil, requestStudentReadError(err)
	}
	if student.EnrolledUntil == "" {
		return nil, nil
	}
	date := timezone.Date(student.EnrolledUntil)
	return &date, nil
}
func (a pickupApprovalAdapter) ActingStaffID(ctx context.Context) (int64, error) {
	staff, err := a.s.resolvePickupChangeStaff(ctx)
	if err != nil {
		return 0, err
	}
	return staff.ID, nil
}
func (a pickupApprovalAdapter) LockStudentAttendance(ctx context.Context, id int64) error {
	return a.s.attendance.LockStudentAttendance(ctx, id)
}
func (a pickupApprovalAdapter) AttendanceCompletion(ctx context.Context, id int64, date timezone.Date) (open, completed bool, err error) {
	rows, err := a.s.attendance.ListAttendance(ctx, studentpresence.AttendanceFilter{StudentIDs: []int64{id}, FromDate: date.String(), UntilDate: date.String()})
	if err != nil {
		return false, false, err
	}
	for _, row := range rows {
		if row.CheckOutTime == nil {
			open = true
		} else {
			completed = true
		}
	}
	return open, completed, nil
}
func (a pickupApprovalAdapter) Preview(ctx context.Context, id int64, date timezone.Date, pickup time.Time) ([]carerequests.Block, error) {
	return a.s.pickupAutoExcusal.Preview(ctx, id, date, pickup)
}
func (a pickupApprovalAdapter) Sync(ctx context.Context, id int64) error {
	_, err := a.s.pickupAutoExcusal.Sync(ctx, id)
	return err
}
