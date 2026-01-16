package students

import "net/http"

// =============================================================================
// Exported Handler Methods for Testing
// =============================================================================
// These methods expose the underlying handlers for test access without going
// through the router's middleware chain.

// ListStudentsHandler returns the handler for listing students.
func (rs *Resource) ListStudentsHandler() http.HandlerFunc { return rs.listStudents }

// GetStudentHandler returns the handler for getting a single student.
func (rs *Resource) GetStudentHandler() http.HandlerFunc { return rs.getStudent }

// CreateStudentHandler returns the handler for creating a student.
func (rs *Resource) CreateStudentHandler() http.HandlerFunc { return rs.createStudent }

// UpdateStudentHandler returns the handler for updating a student.
func (rs *Resource) UpdateStudentHandler() http.HandlerFunc { return rs.updateStudent }

// DeleteStudentHandler returns the handler for deleting a student.
func (rs *Resource) DeleteStudentHandler() http.HandlerFunc { return rs.deleteStudent }

// GetStudentCurrentLocationHandler returns the handler for getting a student's current location.
func (rs *Resource) GetStudentCurrentLocationHandler() http.HandlerFunc {
	return rs.getStudentCurrentLocation
}

// GetStudentInGroupRoomHandler returns the handler for checking if a student is in their group room.
func (rs *Resource) GetStudentInGroupRoomHandler() http.HandlerFunc { return rs.getStudentInGroupRoom }

// GetStudentCurrentVisitHandler returns the handler for getting a student's current visit.
func (rs *Resource) GetStudentCurrentVisitHandler() http.HandlerFunc {
	return rs.getStudentCurrentVisit
}

// GetStudentVisitHistoryHandler returns the handler for getting a student's visit history.
func (rs *Resource) GetStudentVisitHistoryHandler() http.HandlerFunc {
	return rs.getStudentVisitHistory
}

// GetStudentPrivacyConsentHandler returns the handler for getting a student's privacy consent.
func (rs *Resource) GetStudentPrivacyConsentHandler() http.HandlerFunc {
	return rs.getStudentPrivacyConsent
}

// UpdateStudentPrivacyConsentHandler returns the handler for updating a student's privacy consent.
func (rs *Resource) UpdateStudentPrivacyConsentHandler() http.HandlerFunc {
	return rs.updateStudentPrivacyConsent
}

// AssignRFIDTagHandler returns the handler for assigning an RFID tag to a student.
func (rs *Resource) AssignRFIDTagHandler() http.HandlerFunc { return rs.assignRFIDTag }

// UnassignRFIDTagHandler returns the handler for unassigning an RFID tag from a student.
func (rs *Resource) UnassignRFIDTagHandler() http.HandlerFunc { return rs.unassignRFIDTag }
