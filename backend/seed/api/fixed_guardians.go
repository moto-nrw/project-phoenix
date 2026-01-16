package api

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *FixedSeeder) seedGuardians(_ context.Context, result *FixedResult) error {
	for _, guardian := range DemoGuardians {
		guardianKey := fmt.Sprintf("%s %s", guardian.FirstName, guardian.LastName)

		// 1. Create guardian profile
		body := map[string]any{
			"first_name":               guardian.FirstName,
			"last_name":                guardian.LastName,
			"preferred_contact_method": "email",
			"language_preference":      "de",
		}

		// Add contact methods
		if guardian.Email != "" {
			body["email"] = guardian.Email
		}
		if guardian.Phone != "" {
			body["phone"] = guardian.Phone
		}
		if guardian.MobilePhone != "" {
			body["mobile_phone"] = guardian.MobilePhone
		}

		respBody, err := s.client.Post("/api/guardians", body)
		if err != nil {
			return fmt.Errorf("failed to create guardian %s: %w", guardianKey, err)
		}

		// Parse response to extract guardian ID
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse guardian response: %w", err)
		}

		guardianID := resp.Data.ID
		s.guardianIDs[guardianKey] = guardianID

		// 2. Link guardian to student
		studentID, ok := s.studentIDByIndex[guardian.StudentIndex]
		if !ok {
			if s.verbose {
				fmt.Printf("    Warning: student index %d not found for guardian %s\n", guardian.StudentIndex, guardianKey)
			}
			continue
		}

		linkPath := fmt.Sprintf("/api/guardians/students/%d/guardians", studentID)
		linkBody := map[string]any{
			"guardian_profile_id":  guardianID,
			"relationship_type":    guardian.Relationship,
			"is_primary":           guardian.IsPrimary,
			"is_emergency_contact": true,
			"can_pickup":           true,
			"emergency_priority":   1,
		}
		if !guardian.IsPrimary {
			linkBody["emergency_priority"] = 2
		}

		_, err = s.client.Post(linkPath, linkBody)
		if err != nil {
			if s.verbose {
				fmt.Printf("    Warning: failed to link guardian %s to student: %v\n", guardianKey, err)
			}
			continue
		}

		result.GuardianCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d guardians created and linked to students\n", result.GuardianCount)
	}
	return nil
}
