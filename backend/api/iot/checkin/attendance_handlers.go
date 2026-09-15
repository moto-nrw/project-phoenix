package checkin

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/moto-nrw/project-phoenix/modules/devicescan"
)

// getAttendanceStatus answers a student's attendance status by RFID.
func (rs *AttendanceResource) getAttendanceStatus(w http.ResponseWriter, r *http.Request) {
	status, err := rs.attendance.AttendanceStatus(r.Context(), chi.URLParam(r, "rfid"))
	if err != nil {
		rs.fail(w, r, err)
		return
	}
	response := AttendanceStatusResponse{
		Student:    attendanceStudentInfo(status.Student),
		Attendance: attendanceInfo(status.Attendance),
	}
	rs.runtime.Success(w, r, http.StatusOK, response, "Student attendance status retrieved successfully")
}

// toggleAttendance confirms, cancels or finalizes a student's attendance.
func (rs *AttendanceResource) toggleAttendance(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, err := rs.attendance.Device(ctx); err != nil {
		rs.fail(w, r, err)
		return
	}

	req := &AttendanceToggleRequest{}
	if err := render.Bind(r, req); err != nil {
		invalidRequest(w, r, rs.runtime, err)
		return
	}

	command := devicescan.AttendanceToggleCommand{RFIDTag: req.RFID, Action: req.Action}
	if req.Destination != nil {
		command.Destination = *req.Destination
	}
	result, err := rs.attendance.ToggleAttendance(ctx, command)
	if err != nil {
		rs.fail(w, r, err)
		return
	}

	response := AttendanceToggleResponse{
		Action:          result.Action,
		Student:         attendanceStudentInfo(result.Student),
		Message:         result.Message,
		FeedbackEnabled: result.FeedbackEnabled,
	}
	if result.Attendance != nil {
		response.Attendance = attendanceInfo(*result.Attendance)
	}
	switch result.Action {
	case devicescan.AttendanceActionCancelled:
		rs.runtime.Success(w, r, http.StatusOK, response, "Attendance tracking cancelled")
	case devicescan.ScanActionCheckedOutDaily:
		rs.runtime.Success(w, r, http.StatusOK, response, "Daily checkout confirmed")
	default:
		if req.Action == devicescan.AttendanceActionDailyCheckout {
			rs.runtime.Success(w, r, http.StatusOK, response, "Daily checkout confirmed")
			return
		}
		rs.runtime.Success(w, r, http.StatusOK, response, fmt.Sprintf("Student %s successfully", result.Action))
	}
}

func attendanceStudentInfo(student devicescan.AttendanceStudent) AttendanceStudentInfo {
	info := AttendanceStudentInfo{ID: student.ID, FirstName: student.FirstName, LastName: student.LastName}
	if student.Group != nil {
		info.Group = &AttendanceGroupInfo{ID: student.Group.ID, Name: student.Group.Name}
	}
	return info
}

func attendanceInfo(state devicescan.AttendanceState) AttendanceInfo {
	info := AttendanceInfo{
		Status: state.Status, CheckInTime: state.CheckInTime, CheckOutTime: state.CheckOutTime,
		CheckedInBy: state.CheckedInBy, CheckedOutBy: state.CheckedOutBy,
	}
	if state.Date != "" {
		date := state.Date
		info.Date = &date
	}
	return info
}
