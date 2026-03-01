package fixed

import (
	"context"
	"fmt"
	"log"
	"time"
)

// seedPlatform creates the organization, schools, and resets sequences.
// Must run before any tenant-scoped data so that tenant_id FK references are valid.
func (s *Seeder) seedPlatform(ctx context.Context) error {
	now := time.Now()

	// 1. Upsert organization "Test Träger" (ID=1)
	_, err := s.tx.NewRaw(`
		INSERT INTO platform.organizations (id, name, slug, active, created_at, updated_at)
		VALUES (1, 'Test Träger', 'test-traeger', true, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			updated_at = EXCLUDED.updated_at
	`, now, now).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert organization: %w", err)
	}

	// 2. Upsert school-a (ID=1, subdomain: school-a) — default tenant
	_, err = s.tx.NewRaw(`
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active, created_at, updated_at)
		VALUES (1, 1, 'Grundschule Am Park', 'school-a', 'school-a', true, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			subdomain = EXCLUDED.subdomain,
			updated_at = EXCLUDED.updated_at
	`, now, now).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert school-a: %w", err)
	}

	// 3. Create school-b (ID=2, subdomain: school-b) — second test tenant
	_, err = s.tx.NewRaw(`
		INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active, created_at, updated_at)
		VALUES (2, 1, 'Grundschule Sonnenweg', 'school-b', 'school-b', true, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			subdomain = EXCLUDED.subdomain,
			updated_at = EXCLUDED.updated_at
	`, now, now).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to create school-b: %w", err)
	}

	// 4. Sync the schools sequence so future inserts don't collide with explicit IDs
	_, err = s.tx.NewRaw(`SELECT setval('platform.schools_id_seq', GREATEST(2, (SELECT COALESCE(MAX(id), 0) FROM platform.schools)))`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync schools sequence: %w", err)
	}

	if s.verbose {
		log.Println("Created organization 'Test Träger' with schools 'school-a' (ID=1) and 'school-b' (ID=2)")
	}

	return nil
}

// seedAccountTenants creates account-tenant mappings for all seeded accounts.
// Must run after all accounts have been created.
func (s *Seeder) seedAccountTenants(ctx context.Context) error {
	now := time.Now()

	// Map ALL existing accounts to school-a (tenant_id=1) — the primary test school
	result, err := s.tx.NewRaw(`
		INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
		SELECT id, 1, 'active', ?, ?, ?
		FROM auth.accounts
		WHERE id NOT IN (
			SELECT account_id FROM auth.account_tenants WHERE tenant_id = 1
		)
	`, now, now, now).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to map accounts to school-a: %w", err)
	}
	schoolACount, _ := result.RowsAffected()

	// Map the admin account to school-b (tenant_id=2) as well — enables switch-tenant testing
	if s.result.AdminAccount != nil {
		_, err = s.tx.NewRaw(`
			INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
			VALUES (?, 2, 'active', ?, ?, ?)
			ON CONFLICT (account_id, tenant_id) DO NOTHING
		`, s.result.AdminAccount.ID, now, now, now).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to map admin to school-b: %w", err)
		}

		// Assign admin role for school-b so the admin can actually operate there
		adminRole := s.result.Roles[0] // Admin role
		_, err = s.tx.NewRaw(`
			INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
			VALUES (?, ?, 2, ?, ?)
			ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING
		`, s.result.AdminAccount.ID, adminRole.ID, now, now).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to assign admin role for school-b: %w", err)
		}
	}

	if s.verbose {
		log.Printf("Created %d account-tenant mappings for school-a, admin mapped to school-b", schoolACount)
	}

	return nil
}

// seedSchoolBData creates a minimal dataset for school-b (tenant_id=2) to test cross-tenant isolation.
// Creates: 4 rooms, 5 persons (2 staff + 3 students), accounts, education group, and student enrollment.
func (s *Seeder) seedSchoolBData(ctx context.Context) error {
	now := time.Now()
	const tenantID = 2

	// --- Rooms ---
	roomNames := []struct {
		name     string
		building string
		capacity int
		category string
	}{
		{"Klassenraum Sonne", "Hauptgebäude", 28, "Classroom"},
		{"Klassenraum Mond", "Hauptgebäude", 28, "Classroom"},
		{"Betreuungsraum", "OGS-Gebäude", 20, "Activity Room"},
		{"Mensa", "OGS-Gebäude", 60, "Cafeteria"},
	}

	var roomIDs []int64
	for _, r := range roomNames {
		var roomID int64
		err := s.tx.NewRaw(`
			INSERT INTO facilities.rooms (name, building, floor, capacity, category, tenant_id, created_at, updated_at)
			VALUES (?, ?, 0, ?, ?, ?, ?, ?)
			RETURNING id
		`, r.name, r.building, r.capacity, r.category, tenantID, now, now).Scan(ctx, &roomID)
		if err != nil {
			return fmt.Errorf("failed to create school-b room %s: %w", r.name, err)
		}
		roomIDs = append(roomIDs, roomID)
	}

	// --- Staff persons with accounts ---
	staffData := []struct {
		first string
		last  string
		email string
	}{
		{"Maria", "Bergmann", "m.bergmann@school-b.example.com"},
		{"Hans", "Keller", "h.keller@school-b.example.com"},
	}

	var staffPersonIDs []int64
	for _, sd := range staffData {
		var personID int64
		err := s.tx.NewRaw(`
			INSERT INTO users.persons (first_name, last_name, tenant_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id
		`, sd.first, sd.last, tenantID, now, now).Scan(ctx, &personID)
		if err != nil {
			return fmt.Errorf("failed to create school-b person %s: %w", sd.first, err)
		}
		staffPersonIDs = append(staffPersonIDs, personID)

		// Create account
		var accountID int64
		err = s.tx.NewRaw(`
			INSERT INTO auth.accounts (email, password_hash, active, created_at, updated_at)
			VALUES (?, ?, true, ?, ?)
			ON CONFLICT (email) DO UPDATE SET updated_at = EXCLUDED.updated_at
			RETURNING id
		`, sd.email, s.result.AdminAccount.PasswordHash, now, now).Scan(ctx, &accountID)
		if err != nil {
			return fmt.Errorf("failed to create school-b account %s: %w", sd.email, err)
		}
		// Link person to account
		_, err = s.tx.NewRaw(`
			UPDATE users.persons SET account_id = ? WHERE id = ?
		`, accountID, personID).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to link person to account: %w", err)
		}

		// Assign user role
		if len(s.result.Roles) > 1 {
			_, err = s.tx.NewRaw(`
				INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING
			`, accountID, s.result.Roles[1].ID, tenantID, now, now).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to assign role for school-b staff: %w", err)
			}
		}

		// Map to school-b tenant
		_, err = s.tx.NewRaw(`
			INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
			VALUES (?, ?, 'active', ?, ?, ?)
			ON CONFLICT (account_id, tenant_id) DO NOTHING
		`, accountID, tenantID, now, now, now).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to map school-b account to tenant: %w", err)
		}
	}

	// --- Staff records ---
	for i, personID := range staffPersonIDs {
		notes := "Betreuungskraft OGS"
		if i == 0 {
			notes = "Klassenleitung"
		}
		_, err := s.tx.NewRaw(`
			INSERT INTO users.staff (person_id, staff_notes, tenant_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
		`, personID, notes, tenantID, now, now).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create school-b staff: %w", err)
		}
	}

	// --- Education group (one class) ---
	var groupID int64
	err := s.tx.NewRaw(`
		INSERT INTO education.groups (name, room_id, tenant_id, created_at, updated_at)
		VALUES ('1A', ?, ?, ?, ?)
		RETURNING id
	`, roomIDs[0], tenantID, now, now).Scan(ctx, &groupID)
	if err != nil {
		return fmt.Errorf("failed to create school-b education group: %w", err)
	}

	// --- Student persons ---
	studentData := []struct {
		first string
		last  string
	}{
		{"Linus", "Vogel"},
		{"Amelie", "Roth"},
		{"Theo", "Braun"},
	}

	for _, sd := range studentData {
		var personID int64
		err := s.tx.NewRaw(`
			INSERT INTO users.persons (first_name, last_name, tenant_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id
		`, sd.first, sd.last, tenantID, now, now).Scan(ctx, &personID)
		if err != nil {
			return fmt.Errorf("failed to create school-b student person %s: %w", sd.first, err)
		}

		_, err = s.tx.NewRaw(`
			INSERT INTO users.students (person_id, school_class, group_id, tenant_id, created_at, updated_at)
			VALUES (?, '1A', ?, ?, ?, ?)
		`, personID, groupID, tenantID, now, now).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create school-b student %s: %w", sd.first, err)
		}
	}

	if s.verbose {
		log.Printf("Created school-b data: 4 rooms, 2 staff, 3 students, 1 education group")
	}

	return nil
}
