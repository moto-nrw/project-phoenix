package studentpresence

import (
	"context"
	"time"
)

// DemoVisit combines current room occupancy with the latest school booking.
type DemoVisit struct {
	StudentID   int64
	Active, Web bool
	ChangedAt   time.Time
}

// LatestDemoVisits requires a tenant-scoped transaction context.
// Room visits determine occupancy; school attendance determines which device
// booked the child's latest arrival or departure.
func (m *Module) LatestDemoVisits(ctx context.Context, webDeviceID int64) ([]DemoVisit, error) {
	rows, err := m.ListVisitLocations(ctx, VisitLocationFilter{LatestPerStudent: true})
	if err != nil {
		return nil, err
	}
	visits := make(map[int64]DemoVisit, len(rows))
	for _, row := range rows {
		visits[row.Visit.StudentID] = DemoVisit{
			StudentID: row.Visit.StudentID,
			Active:    row.Visit.ExitTime == nil && row.Group != nil && row.Group.EndTime == nil,
		}
	}
	attendance, err := m.ListAttendance(ctx, AttendanceFilter{NewestFirst: true, StudentOrder: true})
	if err != nil {
		return nil, err
	}
	latest := make(map[int64]Attendance)
	for _, row := range attendance {
		previous, exists := latest[row.StudentID]
		if !exists || row.CheckInTime.After(previous.CheckInTime) || (row.CheckInTime.Equal(previous.CheckInTime) && row.ID > previous.ID) {
			latest[row.StudentID] = row
		}
	}
	for id, row := range latest {
		visits[id] = demoAttendanceAttribution(visits[id], row, webDeviceID)
	}
	result := make([]DemoVisit, 0, len(visits))
	for _, visit := range visits {
		result = append(result, visit)
	}
	return result, nil
}

func demoAttendanceAttribution(visit DemoVisit, row Attendance, webDeviceID int64) DemoVisit {
	visit.StudentID = row.StudentID
	visit.ChangedAt = row.CheckInTime
	visit.Web = row.DeviceID == webDeviceID
	if row.CheckOutTime != nil {
		visit.Active = false
		if row.CheckedOutDeviceID != nil {
			visit.Web = *row.CheckedOutDeviceID == webDeviceID
			visit.ChangedAt = *row.CheckOutTime
		} else if row.CheckedOutBy != nil {
			// Web checkout records the acting staff without a kiosk device.
			// Automatic closing has neither and must not renew the grace period.
			visit.Web = true
			visit.ChangedAt = *row.CheckOutTime
		}
	}
	return visit
}
