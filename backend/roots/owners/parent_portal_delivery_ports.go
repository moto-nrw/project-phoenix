package owners

import (
	"context"

	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentAttendance translates the portal's deliberately narrow attendance
// query into Student Presence's public facade. The caller's tenant transaction
// is preserved; no room, device or staff facts are returned to the workflow.
func BindParentAttendance(reader parentAttendanceSource) parentportal.AttendanceReader {
	if reader == nil {
		return nil
	}
	return parentAttendanceReader{reader: reader}
}

type parentAttendanceSource interface {
	ListAttendance(context.Context, studentpresence.AttendanceFilter) ([]studentpresence.Attendance, error)
	HasAttendance(context.Context, studentpresence.AttendanceFilter) (bool, error)
}

type parentAttendanceReader struct{ reader parentAttendanceSource }

func (r parentAttendanceReader) ListAttendance(ctx context.Context, filter parentportal.AttendanceFilter) ([]parentportal.Attendance, error) {
	rows, err := r.reader.ListAttendance(ctx, parentAttendanceFilter(filter))
	if err != nil || rows == nil {
		return nil, err
	}
	out := make([]parentportal.Attendance, len(rows))
	for i, row := range rows {
		out[i] = parentportal.Attendance{CheckInTime: row.CheckInTime, CheckOutTime: row.CheckOutTime}
	}
	return out, nil
}

func (r parentAttendanceReader) HasAttendance(ctx context.Context, filter parentportal.AttendanceFilter) (bool, error) {
	return r.reader.HasAttendance(ctx, parentAttendanceFilter(filter))
}

func parentAttendanceFilter(filter parentportal.AttendanceFilter) studentpresence.AttendanceFilter {
	return studentpresence.AttendanceFilter{StudentIDs: filter.StudentIDs, FromDate: filter.FromDate, UntilDate: filter.UntilDate}
}

type parentAbsenceNotifier struct {
	notifier deliveryCompose.AbsenceNotifier
}

func bindParentAbsenceNotifier(notifier deliveryCompose.AbsenceNotifier) parentportal.AbsenceNotifier {
	if notifier == nil {
		return nil
	}
	return parentAbsenceNotifier{notifier: notifier}
}

func (n parentAbsenceNotifier) NotifyAbsenceReported(ctx context.Context, report parentportal.AbsenceReport) error {
	return n.notifier.NotifyAbsenceReported(ctx, deliveryCompose.AbsenceReport{
		TenantID: report.TenantID, StudentIDs: report.StudentIDs, Status: report.Status,
		Dates: report.Dates, FromParent: report.FromParent, ActorAccountID: report.ActorAccountID,
	})
}

type parentMessageNotifier struct {
	notifier deliveryCompose.StaffParentMessageNotifier
}

func bindParentMessageNotifier(notifier deliveryCompose.StaffParentMessageNotifier) parentportal.StaffParentMessageNotifier {
	if notifier == nil {
		return nil
	}
	return parentMessageNotifier{notifier: notifier}
}

func (n parentMessageNotifier) NotifyStaffParentMessage(ctx context.Context, report parentportal.StaffParentMessageReport) error {
	return n.notifier.NotifyStaffParentMessage(ctx, deliveryCompose.StaffParentMessageReport{
		TenantID: report.TenantID, ThreadID: report.ThreadID, MessageID: report.MessageID,
		StudentID: report.StudentID, ActorAccountID: report.ActorAccountID, Leadership: report.Leadership,
	})
}
