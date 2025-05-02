// Package group provides models and business logic for group management
package group

import (
	"errors"
	"fmt"
	"time"

	models2 "github.com/moto-nrw/project-phoenix/models"
)

// ValidateGroup performs business logic validation for a group
func ValidateGroup(group *models2.Group) error {
	// Check for required fields
	if group == nil {
		return errors.New("group cannot be nil")
	}

	if group.Name == "" {
		return errors.New("name is required")
	}

	return nil
}

// ValidateCombinedGroup performs business logic validation for a combined group
func ValidateCombinedGroup(combinedGroup *models2.CombinedGroup) error {
	// Check for required fields
	if combinedGroup == nil {
		return errors.New("combined group cannot be nil")
	}

	if combinedGroup.Name == "" {
		return errors.New("name is required")
	}

	// Access policy must be one of the allowed values
	if combinedGroup.AccessPolicy != "all" &&
		combinedGroup.AccessPolicy != "first" &&
		combinedGroup.AccessPolicy != "specific" &&
		combinedGroup.AccessPolicy != "manual" {
		return errors.New("access policy must be one of: all, first, specific, manual")
	}

	// If access policy is "specific", then specific group ID is required
	if combinedGroup.AccessPolicy == "specific" && combinedGroup.SpecificGroupID == nil {
		return errors.New("specific group ID is required when access policy is 'specific'")
	}

	return nil
}

// GetGroupSummary generates a summary of a group's details
func GetGroupSummary(group *models2.Group) map[string]interface{} {
	if group == nil {
		return map[string]interface{}{
			"error": "Group not found",
		}
	}

	// Count supervisors
	supervisorCount := 0
	if group.Supervisors != nil {
		supervisorCount = len(group.Supervisors)
	}

	// Count students
	studentCount := 0
	if group.Students != nil {
		studentCount = len(group.Students)
	}

	// Check if group has a room
	hasRoom := group.RoomID != nil
	roomName := ""
	if hasRoom && group.Room != nil {
		roomName = group.Room.RoomName
	}

	// Check if group has a representative
	hasRepresentative := group.RepresentativeID != nil
	representativeName := ""
	if hasRepresentative && group.Representative != nil && group.Representative.CustomUser != nil {
		representativeName = group.Representative.CustomUser.FirstName + " " + group.Representative.CustomUser.SecondName
	}

	summary := map[string]interface{}{
		"group_id":            group.ID,
		"name":                group.Name,
		"student_count":       studentCount,
		"supervisor_count":    supervisorCount,
		"has_room":            hasRoom,
		"room_name":           roomName,
		"has_representative":  hasRepresentative,
		"representative_name": representativeName,
		"created_at":          group.CreatedAt,
		"updated_at":          group.ModifiedAt,
	}

	return summary
}

// GetCombinedGroupSummary generates a summary of a combined group's details
func GetCombinedGroupSummary(combinedGroup *models2.CombinedGroup) map[string]interface{} {
	if combinedGroup == nil {
		return map[string]interface{}{
			"error": "Combined group not found",
		}
	}

	// Count groups
	groupCount := 0
	if combinedGroup.Groups != nil {
		groupCount = len(combinedGroup.Groups)
	}

	// Count specialists
	specialistCount := 0
	if combinedGroup.AccessSpecialists != nil {
		specialistCount = len(combinedGroup.AccessSpecialists)
	}

	// Calculate expiration status
	isExpired := false
	timeUntilExpiration := ""
	if combinedGroup.ValidUntil != nil {
		if combinedGroup.ValidUntil.Before(time.Now()) {
			isExpired = true
		} else {
			duration := combinedGroup.ValidUntil.Sub(time.Now())
			if duration.Hours() > 24 {
				timeUntilExpiration = fmt.Sprintf("≈%d days", int(duration.Hours()/24))
			} else {
				timeUntilExpiration = fmt.Sprintf("≈%d hours", int(duration.Hours()))
			}
		}
	}

	summary := map[string]interface{}{
		"combined_group_id":     combinedGroup.ID,
		"name":                  combinedGroup.Name,
		"is_active":             combinedGroup.IsActive && !isExpired,
		"is_expired":            isExpired,
		"access_policy":         combinedGroup.AccessPolicy,
		"group_count":           groupCount,
		"specialist_count":      specialistCount,
		"time_until_expiration": timeUntilExpiration,
		"created_at":            combinedGroup.CreatedAt,
	}

	return summary
}

// IsGroupMember checks if a student is a member of a group
func IsGroupMember(group *models2.Group, studentID int64) bool {
	if group == nil || group.Students == nil {
		return false
	}

	for _, student := range group.Students {
		if student.ID == studentID {
			return true
		}
	}

	return false
}

// IsSpecialistSupervisorOfGroup checks if a specialist is a supervisor of a group
func IsSpecialistSupervisorOfGroup(group *models2.Group, specialistID int64) bool {
	if group == nil || group.Supervisors == nil {
		return false
	}

	for _, supervisor := range group.Supervisors {
		if supervisor.ID == specialistID {
			return true
		}
	}

	return false
}

// HasGroupAccessToCombinedGroup checks if a group has access to a combined group
func HasGroupAccessToCombinedGroup(combinedGroup *models2.CombinedGroup, groupID int64) bool {
	if combinedGroup == nil {
		return false
	}

	// Based on access policy, determine if the group has access
	switch combinedGroup.AccessPolicy {
	case "all":
		// All groups in the combined group have access
		for _, group := range combinedGroup.Groups {
			if group.ID == groupID {
				return true
			}
		}
	case "first":
		// Only the first group has access
		if len(combinedGroup.Groups) > 0 && combinedGroup.Groups[0].ID == groupID {
			return true
		}
	case "specific":
		// Only the specific group has access
		if combinedGroup.SpecificGroupID != nil && *combinedGroup.SpecificGroupID == groupID {
			return true
		}
	case "manual":
		// Access is determined manually (assume no automatic access)
		return false
	}

	return false
}

// HasSpecialistAccessToCombinedGroup checks if a specialist has access to a combined group
func HasSpecialistAccessToCombinedGroup(combinedGroup *models2.CombinedGroup, specialistID int64) bool {
	if combinedGroup == nil || combinedGroup.AccessSpecialists == nil {
		return false
	}

	// Check if the specialist is in the access list
	for _, specialist := range combinedGroup.AccessSpecialists {
		if specialist.ID == specialistID {
			return true
		}
	}

	return false
}
