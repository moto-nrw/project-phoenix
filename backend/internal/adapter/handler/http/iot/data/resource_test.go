package data

import "net/http"

// GetAvailableTeachersHandler returns the getAvailableTeachers handler.
func (rs *Resource) GetAvailableTeachersHandler() http.HandlerFunc { return rs.getAvailableTeachers }

// GetTeacherStudentsHandler returns the getTeacherStudents handler.
func (rs *Resource) GetTeacherStudentsHandler() http.HandlerFunc { return rs.getTeacherStudents }

// GetTeacherActivitiesHandler returns the getTeacherActivities handler.
func (rs *Resource) GetTeacherActivitiesHandler() http.HandlerFunc { return rs.getTeacherActivities }

// GetAvailableRoomsHandler returns the getAvailableRoomsForDevice handler.
func (rs *Resource) GetAvailableRoomsHandler() http.HandlerFunc { return rs.getAvailableRoomsForDevice }

// CheckRFIDTagAssignmentHandler returns the checkRFIDTagAssignment handler.
func (rs *Resource) CheckRFIDTagAssignmentHandler() http.HandlerFunc {
	return rs.checkRFIDTagAssignment
}
