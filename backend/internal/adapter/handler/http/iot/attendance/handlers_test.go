package attendance

import "net/http"

// GetAttendanceStatusHandler returns the getAttendanceStatus handler.
func (rs *Resource) GetAttendanceStatusHandler() http.HandlerFunc { return rs.getAttendanceStatus }

// ToggleAttendanceHandler returns the toggleAttendance handler.
func (rs *Resource) ToggleAttendanceHandler() http.HandlerFunc { return rs.toggleAttendance }
