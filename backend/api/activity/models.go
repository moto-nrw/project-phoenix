// Package activity provides models and business logic for activity group management
package activity

import (
	"errors"
	"fmt"

	models2 "github.com/moto-nrw/project-phoenix/models"
)

// ValidateAg performs business logic validation for an activity group
func ValidateAg(ag *models2.Ag) error {
	// Check for required fields
	if ag == nil {
		return errors.New("activity group cannot be nil")
	}

	if ag.Name == "" {
		return errors.New("name is required")
	}

	if ag.MaxParticipant < 1 {
		return errors.New("maximum participants must be at least 1")
	}

	if ag.SupervisorID == 0 {
		return errors.New("supervisor ID is required")
	}

	if ag.AgCategoryID == 0 {
		return errors.New("category ID is required")
	}

	return nil
}

// ValidateAgCategory performs business logic validation for an activity group category
func ValidateAgCategory(category *models2.AgCategory) error {
	// Check for required fields
	if category == nil {
		return errors.New("category cannot be nil")
	}

	if category.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

// ValidateAgTime performs business logic validation for an activity group time slot
func ValidateAgTime(agTime *models2.AgTime) error {
	// Check for required fields
	if agTime == nil {
		return errors.New("time slot cannot be nil")
	}

	// Validate weekday
	validWeekdays := map[string]bool{
		"Monday":    true,
		"Tuesday":   true,
		"Wednesday": true,
		"Thursday":  true,
		"Friday":    true,
		"Saturday":  true,
		"Sunday":    true,
	}

	if !validWeekdays[agTime.Weekday] {
		return errors.New("invalid weekday; must be one of: Monday, Tuesday, Wednesday, Thursday, Friday, Saturday, Sunday")
	}

	if agTime.TimespanID == 0 {
		return errors.New("timespan ID is required")
	}

	// AgID is not required during initial creation of an activity
	// It will be set automatically in the CreateAg method

	return nil
}

// GetAgSummary generates a summary of an activity group's details
func GetAgSummary(ag *models2.Ag) map[string]interface{} {
	if ag == nil {
		return map[string]interface{}{
			"error": "Activity group not found",
		}
	}

	// Count participants
	participantCount := 0
	if ag.Students != nil {
		participantCount = len(ag.Students)
	}

	// Count time slots
	timeSlotCount := 0
	if ag.Times != nil {
		timeSlotCount = len(ag.Times)
	}

	// Get category name
	categoryName := ""
	if ag.AgCategory != nil {
		categoryName = ag.AgCategory.Name
	}

	// Get supervisor name
	supervisorName := ""
	if ag.Supervisor != nil && ag.Supervisor.CustomUser != nil {
		supervisorName = ag.Supervisor.CustomUser.FirstName + " " + ag.Supervisor.CustomUser.SecondName
	}

	// Check if the activity group is active based on datespan
	isActive := true
	if ag.Datespan != nil {
		isActive = ag.Datespan.IsActive()
	}

	// Format time slots
	timeSlots := make([]map[string]interface{}, 0)
	if ag.Times != nil {
		for _, t := range ag.Times {
			timeSlot := map[string]interface{}{
				"weekday": t.Weekday,
			}

			if t.Timespan != nil {
				startTime := t.Timespan.StartTime.Format("15:04")
				timeSlot["start_time"] = startTime

				if t.Timespan.EndTime != nil {
					endTime := t.Timespan.EndTime.Format("15:04")
					timeSlot["end_time"] = endTime
				}
			}

			timeSlots = append(timeSlots, timeSlot)
		}
	}

	// Calculate spaces left
	spacesLeft := ag.MaxParticipant - participantCount
	if spacesLeft < 0 {
		spacesLeft = 0
	}

	summary := map[string]interface{}{
		"ag_id":             ag.ID,
		"name":              ag.Name,
		"category":          categoryName,
		"supervisor":        supervisorName,
		"is_open":           ag.IsOpenAg,
		"is_active":         isActive,
		"max_participants":  ag.MaxParticipant,
		"participant_count": participantCount,
		"spaces_left":       spacesLeft,
		"time_slots":        timeSlots,
		"time_slot_count":   timeSlotCount,
		"created_at":        ag.CreatedAt,
		"updated_at":        ag.ModifiedAt,
	}

	return summary
}

// IsStudentEnrolledInAg checks if a student is enrolled in an activity group
func IsStudentEnrolledInAg(ag *models2.Ag, studentID int64) bool {
	if ag == nil || ag.Students == nil {
		return false
	}

	for _, student := range ag.Students {
		if student.ID == studentID {
			return true
		}
	}

	return false
}

// IsSpecialistSupervisorOfAg checks if a specialist is a supervisor of an activity group
func IsSpecialistSupervisorOfAg(ag *models2.Ag, specialistID int64) bool {
	if ag == nil {
		return false
	}

	return ag.SupervisorID == specialistID
}

// HasAvailableSpace checks if the activity group has space for more participants
func HasAvailableSpace(ag *models2.Ag) bool {
	if ag == nil {
		return false
	}

	participantCount := 0
	if ag.Students != nil {
		participantCount = len(ag.Students)
	}

	return participantCount < ag.MaxParticipant
}

// FormatTimeslots formats the time slots of an activity group into a readable string
func FormatTimeslots(ag *models2.Ag) string {
	if ag == nil || ag.Times == nil || len(ag.Times) == 0 {
		return "No scheduled times"
	}

	result := ""
	for i, t := range ag.Times {
		if i > 0 {
			result += ", "
		}

		timeStr := t.Weekday
		if t.Timespan != nil {
			timeStr += " " + t.Timespan.StartTime.Format("15:04")
			if t.Timespan.EndTime != nil {
				timeStr += "-" + t.Timespan.EndTime.Format("15:04")
			}
		}

		result += timeStr
	}

	return result
}

// IsAgActive checks if an activity group is currently active
func IsAgActive(ag *models2.Ag) bool {
	if ag == nil {
		return false
	}

	// If there's no datespan, assume it's always active
	if ag.Datespan == nil {
		return true
	}

	return ag.Datespan.IsActive()
}

// GetConflictingTimeslots finds timeslots of a student's enrolled AGs that conflict with a new AG
func GetConflictingTimeslots(newAg *models2.Ag, enrolledAgs []models2.Ag) []string {
	if newAg == nil || newAg.Times == nil || len(newAg.Times) == 0 {
		return nil
	}

	conflicts := make([]string, 0)

	// Create a map of the new AG's timeslots
	newTimeslots := make(map[string]map[string]bool)
	for _, t := range newAg.Times {
		if t.Timespan == nil {
			continue
		}

		weekday := t.Weekday
		if newTimeslots[weekday] == nil {
			newTimeslots[weekday] = make(map[string]bool)
		}

		startTime := t.Timespan.StartTime.Format("15:04")
		newTimeslots[weekday][startTime] = true

		if t.Timespan.EndTime != nil {
			// We could check for overlapping time ranges here if needed
			// For now, just mark the timespan by its start time
		}
	}

	// Check against enrolled AGs' timeslots
	for _, enrolled := range enrolledAgs {
		if enrolled.Times == nil {
			continue
		}

		for _, t := range enrolled.Times {
			if t.Timespan == nil {
				continue
			}

			weekday := t.Weekday
			startTime := t.Timespan.StartTime.Format("15:04")

			// If this is a conflicting timeslot
			if newTimeslots[weekday] != nil && newTimeslots[weekday][startTime] {
				conflict := fmt.Sprintf("%s at %s (%s)", weekday, startTime, enrolled.Name)
				conflicts = append(conflicts, conflict)
			}
		}
	}

	return conflicts
}
