package data

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// getAvailableTeachers handles getting the list of teachers available for device login selection
// This endpoint only requires device authentication (no PIN required)
func (rs *Resource) getAvailableTeachers(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context (no staff context required)
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	responses, err := rs.Directory.Teachers(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, responses, "Available teachers retrieved successfully")
}

// getTeacherStudents handles getting students supervised by authenticated teacher(s).
// When teacher_ids is omitted, returns ALL students (used for bracelet assignment).
func (rs *Resource) getTeacherStudents(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	// If teacher_ids query key is absent entirely, return all students.
	// An explicitly empty value (?teacher_ids=) still goes through parseTeacherIDs
	// which returns an empty result — preserving previous behavior.
	if _, hasTeacherIDs := r.URL.Query()["teacher_ids"]; !hasTeacherIDs {
		rs.getAllStudents(w, r)
		return
	}

	// Parse teacher IDs from query parameters
	teacherIDs, ok := rs.parseTeacherIDs(w, r)
	if !ok {
		return
	}

	response, err := rs.Directory.Students(r.Context(), teacherIDs)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, fmt.Sprintf("Found %d unique students", len(response)))
}

// getTeacherActivities handles getting activities supervised by the authenticated teacher (for RFID devices)
func (rs *Resource) getTeacherActivities(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device from context
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	response, err := rs.Directory.Activities(r.Context())
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, "Activities fetched successfully")
}

// getAvailableRoomsForDevice handles getting available rooms for RFID devices
func (rs *Resource) getAvailableRoomsForDevice(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	// Parse capacity parameter if provided
	capacity := 0
	if capacityStr := r.URL.Query().Get("capacity"); capacityStr != "" {
		if cap, err := strconv.Atoi(capacityStr); err == nil && cap > 0 {
			capacity = cap
		}
	}

	responses, err := rs.Rooms.AvailableRooms(r.Context(), capacity)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, responses, "Available rooms retrieved successfully")
}

// checkRFIDTagAssignment handles checking if an RFID tag is assigned and to whom
func (rs *Resource) checkRFIDTagAssignment(w http.ResponseWriter, r *http.Request) {
	// Get authenticated device and staff from context
	if _, ok := rs.runtime.requireDevice(w, r); !ok {
		return
	}

	// Get tagId from URL parameter
	tagID := chi.URLParam(r, "tagId")
	if tagID == "" {
		rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("tagId parameter is required"), "")
		return
	}

	response, err := rs.TagAssignments.LookupTagAssignment(r.Context(), tagID)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "Internal server error")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, "RFID tag assignment status retrieved")
}

// getAllStudents returns all students with group info (no teacher filter)
func (rs *Resource) getAllStudents(w http.ResponseWriter, r *http.Request) {
	response, err := rs.Directory.Students(r.Context(), nil)
	if err != nil {
		rs.runtime.Failure(w, r, http.StatusInternalServerError, err, "")
		return
	}

	rs.runtime.Success(w, r, http.StatusOK, response, fmt.Sprintf("Found %d students", len(response)))
}

// Helper functions

// parseTeacherIDs parses comma-separated teacher IDs from query parameter
func (rs *Resource) parseTeacherIDs(w http.ResponseWriter, r *http.Request) ([]int64, bool) {
	teacherIDsParam := r.URL.Query().Get("teacher_ids")
	if teacherIDsParam == "" {
		rs.runtime.Success(w, r, http.StatusOK, []TeacherStudentResponse{}, "No teacher IDs provided")
		return nil, false
	}

	teacherIDStrings := strings.Split(teacherIDsParam, ",")
	teacherIDs := make([]int64, 0, len(teacherIDStrings))
	for _, idStr := range teacherIDStrings {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			rs.runtime.Failure(w, r, http.StatusBadRequest, errors.New("invalid teacher ID: "+idStr), "")
			return nil, false
		}
		teacherIDs = append(teacherIDs, id)
	}

	if len(teacherIDs) == 0 {
		rs.runtime.Success(w, r, http.StatusOK, []TeacherStudentResponse{}, "No valid teacher IDs provided")
		return nil, false
	}

	return teacherIDs, true
}
