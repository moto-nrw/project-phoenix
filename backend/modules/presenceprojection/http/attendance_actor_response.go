package presenceprojectionhttp

import (
	time "time"

	lists "github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	studentpresence "github.com/moto-nrw/project-phoenix/modules/studentpresence"
)

// AttendanceActorResponse says who recorded an arrival or a pickup (#4761):
// kind "staff" with the staff member's name, "device" for a kiosk without a
// person, "former_staff" for an erased staff member and "automatic" for a
// checkout moto recorded itself. Only the staff portal's student routes
// carry it; the parents and school portals never do.
type AttendanceActorResponse struct {
	Kind string `json:"kind"`
	Name string `json:"name,omitempty"`
}

func newAttendanceActorResponse(actor *studentpresence.AttendanceActor) *AttendanceActorResponse {
	if actor == nil {
		return nil
	}
	return &AttendanceActorResponse{Kind: string(actor.Kind), Name: actor.Name}
}

// applyAttendanceActorsToDetail names who recorded today's arrival and
// pickup on the staff portal's child detail.
func applyAttendanceActorsToDetail(response *StudentDetailResponse, status *studentpresence.DailyAttendanceStatus) {
	if response == nil || status == nil {
		return
	}
	response.ActualArrivalBy = newAttendanceActorResponse(status.CheckedInActor)
	response.ActualPickupBy = newAttendanceActorResponse(status.CheckedOutActor)
}

// attachAttendanceActors names who recorded each day's arrival and pickup.
func attachAttendanceActors(days []attendanceHistoryDay, recorders studentpresence.AttendanceRecorders) {
	for i := range days {
		record := days[i].Attendance
		if record == nil || record.inRow == nil {
			continue
		}
		checkIn := recorders.CheckIn(*record.inRow)
		record.CheckedInActor = newAttendanceActorResponse(&checkIn)
		if record.CheckOutTime != nil && record.outRow != nil {
			record.CheckedOutActor = newAttendanceActorResponse(recorders.CheckOut(*record.outRow))
		}
	}
}

// attendanceActorExportLabel is the export cell for an actor; a pickup not
// recorded yet stays empty like the other missing values.
func attendanceActorExportLabel(actor *studentpresence.AttendanceActor) string {
	if actor == nil {
		return "–"
	}
	switch actor.Kind {
	case studentpresence.AttendanceActorStaff:
		return actor.Name
	case studentpresence.AttendanceActorDevice:
		return "am Gerät"
	case studentpresence.AttendanceActorAutomatic:
		return "automatisch"
	default:
		return "Ehemalige Mitarbeitende"
	}
}

// withAttendanceActors fills the export row's "Angemeldet von" and
// "Abgemeldet von" from the school stay the row belongs to; a row outside
// every stay keeps both empty.
func withAttendanceActors(row lists.Row, stay *studentpresence.Attendance, recorders *studentpresence.AttendanceRecorders) lists.Row {
	if recorders == nil {
		return row
	}
	checkedIn, checkedOut := "–", "–"
	if stay != nil {
		actor := recorders.CheckIn(*stay)
		checkedIn = attendanceActorExportLabel(&actor)
		checkedOut = attendanceActorExportLabel(recorders.CheckOut(*stay))
	}
	row.Values[attendanceColumnCheckedIn] = checkedIn
	row.Values[attendanceColumnCheckedOut] = checkedOut
	return row
}

// stayAt is the school stay that covers the instant: checked in by then and
// not checked out before it.
func stayAt(rows []*studentpresence.Attendance, at *time.Time) *studentpresence.Attendance {
	if at == nil {
		return nil
	}
	for _, row := range rows {
		if row == nil || row.CheckInTime.After(*at) {
			continue
		}
		if row.CheckOutTime == nil || !row.CheckOutTime.Before(*at) {
			return row
		}
	}
	return nil
}
