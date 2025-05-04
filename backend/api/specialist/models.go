// Package specialist provides models and business logic for pedagogical specialist management
package specialist

import (
	"errors"
	"fmt"

	models2 "github.com/moto-nrw/project-phoenix/models"
)

// ValidateSpecialist performs business logic validation for a pedagogical specialist
func ValidateSpecialist(specialist *models2.PedagogicalSpecialist) error {
	// Check for required fields
	if specialist == nil {
		return errors.New("specialist cannot be nil")
	}

	if specialist.Role == "" {
		return errors.New("role is required")
	}

	// If user ID is 0 and CustomUser is provided, it means we're creating a new user
	if specialist.UserID == 0 && specialist.CustomUser != nil {
		if specialist.CustomUser.FirstName == "" {
			return errors.New("first name is required when creating a new user")
		}
		if specialist.CustomUser.SecondName == "" {
			return errors.New("second name is required when creating a new user")
		}
	} else if specialist.UserID == 0 {
		return errors.New("user ID is required")
	}

	return nil
}

// GetSpecialistSummary generates a summary of a specialist's details
func GetSpecialistSummary(specialist *models2.PedagogicalSpecialist) map[string]interface{} {
	if specialist == nil {
		return map[string]interface{}{
			"error": "Specialist not found",
		}
	}

	// Get user name
	userName := "Unknown"
	if specialist.CustomUser != nil {
		userName = fmt.Sprintf("%s %s", specialist.CustomUser.FirstName, specialist.CustomUser.SecondName)
	}

	// Get tag ID if available
	tagID := "None"
	if specialist.CustomUser != nil && specialist.CustomUser.TagID != nil {
		tagID = *specialist.CustomUser.TagID
	}

	// Check if associated with an account
	hasAccount := false
	if specialist.CustomUser != nil && specialist.CustomUser.AccountID != nil {
		hasAccount = true
	}

	summary := map[string]interface{}{
		"specialist_id": specialist.ID,
		"name":          userName,
		"role":          specialist.Role,
		"tag_id":        tagID,
		"has_account":   hasAccount,
		"created_at":    specialist.CreatedAt,
		"updated_at":    specialist.ModifiedAt,
	}

	return summary
}

// IsSpecialistAssignedToGroup checks if a specialist is assigned to a specific group
func IsSpecialistAssignedToGroup(specialist *models2.PedagogicalSpecialist, groupID int64) bool {
	if specialist == nil || len(specialist.Groups) == 0 {
		return false
	}

	for _, group := range specialist.Groups {
		if group.ID == groupID {
			return true
		}
	}

	return false
}

// GetSpecialistAssignments returns a list of group assignments for a specialist
func GetSpecialistAssignments(specialist *models2.PedagogicalSpecialist) []map[string]interface{} {
	if specialist == nil || len(specialist.Groups) == 0 {
		return []map[string]interface{}{}
	}

	assignments := make([]map[string]interface{}, 0, len(specialist.Groups))
	for _, group := range specialist.Groups {
		assignment := map[string]interface{}{
			"group_id":   group.ID,
			"group_name": group.Name,
		}

		if group.Room != nil {
			assignment["room_name"] = group.Room.RoomName
		}

		assignments = append(assignments, assignment)
	}

	return assignments
}
