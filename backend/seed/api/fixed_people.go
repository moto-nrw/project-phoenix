package api

import (
	"context"
	"encoding/json"
	"fmt"
)

func (s *FixedSeeder) seedPersons(_ context.Context, result *FixedResult) error {
	// Create persons for staff only
	// (Students will have persons created automatically via student API)
	for _, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		body := map[string]string{
			"first_name": staff.FirstName,
			"last_name":  staff.LastName,
		}

		respBody, err := s.client.Post("/api/users", body)
		if err != nil {
			return fmt.Errorf("failed to create person %s: %w", personKey, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse person response: %w", err)
		}

		s.personIDs[personKey] = resp.Data.ID
		result.PersonCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d persons created (staff)\n", result.PersonCount)
	}
	return nil
}

func (s *FixedSeeder) seedStaff(_ context.Context, result *FixedResult) error {
	for _, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff %s", personKey)
		}

		body := map[string]any{
			"person_id":   personID,
			"is_teacher":  staff.IsTeacher,
			"staff_notes": fmt.Sprintf("Position: %s", staff.Position),
			"role":        staff.Position, // Role field for teacher record
		}

		respBody, err := s.client.Post("/api/staff", body)
		if err != nil {
			return fmt.Errorf("failed to create staff %s: %w", personKey, err)
		}

		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID        int64 `json:"id"`
				TeacherID int64 `json:"teacher_id,omitempty"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse staff response: %w", err)
		}

		s.staffIDs[personKey] = resp.Data.ID
		// Store teacher ID if this is a teacher (for group assignment)
		if resp.Data.TeacherID > 0 {
			s.teacherIDs[personKey] = resp.Data.TeacherID
		}
		result.StaffCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d staff created (%d teachers)\n", result.StaffCount, len(s.teacherIDs))
	}
	return nil
}

// seedStaffAccounts creates auth accounts for staff and links them to persons
func (s *FixedSeeder) seedStaffAccounts(_ context.Context, result *FixedResult) error {
	// Get role IDs for different staff types
	adminRoleID, ok := s.roleIDs["admin"]
	if !ok {
		return fmt.Errorf("admin role not found - available roles: %v", s.roleIDs)
	}
	teacherRoleID, ok := s.roleIDs["teacher"]
	if !ok {
		return fmt.Errorf("teacher role not found - available roles: %v", s.roleIDs)
	}
	guestRoleID, ok := s.roleIDs["guest"]
	if !ok {
		return fmt.Errorf("guest role not found - available roles: %v", s.roleIDs)
	}

	for i, staff := range DemoStaff {
		personKey := fmt.Sprintf("%s %s", staff.FirstName, staff.LastName)
		personID, ok := s.personIDs[personKey]
		if !ok {
			return fmt.Errorf("person not found for staff account %s", personKey)
		}

		// Generate email and credentials
		email := fmt.Sprintf("%s.%s@example.com",
			normalizeForEmail(staff.FirstName),
			normalizeForEmail(staff.LastName))
		password := s.defaultPassword
		pin := fmt.Sprintf("%04d", 1000+i)

		// Assign role based on position:
		// - OGS-Büro → admin (OGS leadership with full access)
		// - Extern → guest (external helpers with limited access)
		// - Pädagogische Fachkraft → teacher (standard pedagogical staff)
		var roleID int64
		switch staff.Position {
		case "OGS-Büro":
			roleID = adminRoleID
		case "Extern":
			roleID = guestRoleID
		default:
			roleID = teacherRoleID
		}

		// Create account via /register with role_id
		registerBody := map[string]any{
			"email":            email,
			"username":         fmt.Sprintf("%s.%s", normalizeForEmail(staff.FirstName), normalizeForEmail(staff.LastName)),
			"password":         password,
			"confirm_password": password,
			"role_id":          roleID,
		}

		respBody, err := s.client.Post("/auth/register", registerBody)
		if err != nil {
			return fmt.Errorf("failed to create account for %s: %w", personKey, err)
		}

		// Parse response to get account ID
		var resp struct {
			Status string `json:"status"`
			Data   struct {
				ID int64 `json:"id"`
			} `json:"data"`
		}
		if err := json.Unmarshal(respBody, &resp); err != nil {
			return fmt.Errorf("failed to parse account response: %w", err)
		}

		// Link account to person
		linkPath := fmt.Sprintf("/api/users/%d/account", personID)
		linkBody := map[string]any{
			"account_id": resp.Data.ID,
		}
		_, err = s.client.Put(linkPath, linkBody)
		if err != nil {
			return fmt.Errorf("failed to link account to person %s: %w", personKey, err)
		}

		// Store credentials for summary
		s.staffCredentials = append(s.staffCredentials, StaffCredentials{
			Email:    email,
			Password: password,
			PIN:      pin,
			Name:     personKey,
			Position: staff.Position,
		})

		result.AccountCount++
	}

	if s.verbose {
		fmt.Printf("  ✓ %d staff accounts created and linked\n", result.AccountCount)
	}
	return nil
}

// normalizeForEmail converts a name to a valid email component
func normalizeForEmail(name string) string {
	// Convert to lowercase
	result := []rune{}
	for _, r := range name {
		switch r {
		case 'ä', 'Ä':
			result = append(result, 'a', 'e')
		case 'ö', 'Ö':
			result = append(result, 'o', 'e')
		case 'ü', 'Ü':
			result = append(result, 'u', 'e')
		case 'ß':
			result = append(result, 's', 's')
		default:
			if r >= 'A' && r <= 'Z' {
				result = append(result, r+32) // lowercase
			} else if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				result = append(result, r)
			}
		}
	}
	return string(result)
}
