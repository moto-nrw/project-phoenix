// Package student provides models and business logic for student management
package student

import (
	"errors"
	"time"

	models2 "github.com/moto-nrw/project-phoenix/models"
)

// ValidateStudent performs business logic validation for a student
func ValidateStudent(student *models2.Student) error {
	// Check for required fields
	if student == nil {
		return errors.New("student cannot be nil")
	}

	if student.SchoolClass == "" {
		return errors.New("school class is required")
	}

	if student.NameLG == "" {
		return errors.New("legal guardian name is required")
	}

	if student.ContactLG == "" {
		return errors.New("legal guardian contact is required")
	}

	if student.CustomUserID == 0 {
		return errors.New("custom user ID is required")
	}

	if student.GroupID == 0 {
		return errors.New("group ID is required")
	}

	// Bus, InHouse, WC, and SchoolYard are booleans and have default values

	return nil
}

// GetStudentCurrentStatus returns a descriptive status based on a student's location flags
func GetStudentCurrentStatus(student *models2.Student) string {
	if student == nil {
		return "Unknown"
	}

	if student.InHouse {
		if student.WC {
			return "In bathroom"
		}
		return "In school"
	} else if student.SchoolYard {
		return "In school yard"
	} else if student.Bus {
		return "On bus"
	}

	return "Not in school"
}

// CheckStudentPresent determines if a student is present in the school
func CheckStudentPresent(student *models2.Student) bool {
	if student == nil {
		return false
	}

	return student.InHouse || student.SchoolYard
}

// CalculateVisitDuration calculates the total duration of a visit
func CalculateVisitDuration(visit *models2.Visit) time.Duration {
	if visit == nil || visit.Timespan == nil {
		return 0
	}

	// If the visit has a timespan with start and end, calculate the duration
	if visit.Timespan.EndTime != nil {
		return visit.Timespan.EndTime.Sub(visit.Timespan.StartTime)
	}

	// If still active (no end time), calculate duration from start until now
	return time.Since(visit.Timespan.StartTime)
}

// ProcessStudentVisits summarizes a student's visits for reporting
func ProcessStudentVisits(visits []models2.Visit) map[string]interface{} {
	summary := map[string]interface{}{
		"total_visits": len(visits),
		"rooms_visited": make(map[int64]string),
		"total_time": time.Duration(0),
	}

	roomVisits := make(map[int64]string)

	for _, visit := range visits {
		if visit.Room != nil {
			roomVisits[visit.RoomID] = visit.Room.RoomName
		}

		// Add the visit duration to total time
		if visit.Timespan != nil {
			var duration time.Duration
			if visit.Timespan.EndTime != nil {
				duration = visit.Timespan.EndTime.Sub(visit.Timespan.StartTime)
			} else {
				duration = time.Since(visit.Timespan.StartTime)
			}
			summary["total_time"] = summary["total_time"].(time.Duration) + duration
		}
	}

	summary["rooms_visited"] = roomVisits

	return summary
}

// VerifyRoomAccess checks if a student has access to a specific room
func VerifyRoomAccess(student *models2.Student, roomID int64) bool {
	// If the student's group has this room assigned, allow access
	if student != nil && student.Group != nil && student.Group.RoomID != nil && *student.Group.RoomID == roomID {
		return true
	}

	// Additional access rules could be implemented here
	// For example, checking if the student is part of an activity group that uses this room

	return false
}

// GetStudentAttendanceRate calculates the attendance rate for a student over a period
func GetStudentAttendanceRate(visits []models2.Visit, daysInPeriod int) float64 {
	if daysInPeriod <= 0 {
		return 0
	}

	// Group visits by day to count days with at least one visit
	visitDays := make(map[string]bool)

	for _, visit := range visits {
		dayKey := visit.Day.Format("2006-01-02")
		visitDays[dayKey] = true
	}

	// Calculate attendance rate
	attendanceRate := float64(len(visitDays)) / float64(daysInPeriod)

	// Ensure the rate is between 0 and 1
	if attendanceRate > 1 {
		return 1
	}

	return attendanceRate
}

// GenerateStudentReport creates a comprehensive report for a student
func GenerateStudentReport(student *models2.Student, visits []models2.Visit) map[string]interface{} {
	if student == nil {
		return map[string]interface{}{
			"error": "Student not found",
		}
	}

	// Calculate days since the start of the school year or last 30 days
	// For this example, we'll use 30 days
	daysInPeriod := 30

	// Process visits
	visitSummary := ProcessStudentVisits(visits)

	// Calculate attendance rate
	attendanceRate := GetStudentAttendanceRate(visits, daysInPeriod)

	// Generate the report
	report := map[string]interface{}{
		"student_id": student.ID,
		"name": student.CustomUser.FirstName + " " + student.CustomUser.SecondName,
		"school_class": student.SchoolClass,
		"group": student.Group.Name,
		"current_status": GetStudentCurrentStatus(student),
		"attendance_rate": attendanceRate,
		"visit_summary": visitSummary,
		"report_generated_at": time.Now(),
	}

	return report
}