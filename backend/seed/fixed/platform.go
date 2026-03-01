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

	// 4. Upsert organization "Träger Nord" (ID=2) — second carrier for multi-org testing
	_, err = s.tx.NewRaw(`
		INSERT INTO platform.organizations (id, name, slug, active, created_at, updated_at)
		VALUES (2, 'Träger Nord', 'traeger-nord', true, ?, ?)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			slug = EXCLUDED.slug,
			updated_at = EXCLUDED.updated_at
	`, now, now).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert organization Träger Nord: %w", err)
	}

	// 5. Create schools for Träger Nord (IDs 3-5)
	tragerNordSchools := []struct {
		id        int64
		name      string
		slug      string
		subdomain string
	}{
		{3, "OGS Nordpark", "ogs-nordpark", "ogs-nordpark"},
		{4, "OGS Waldblick", "ogs-waldblick", "ogs-waldblick"},
		{5, "OGS Flussaue", "ogs-flussaue", "ogs-flussaue"},
	}
	for _, school := range tragerNordSchools {
		_, err = s.tx.NewRaw(`
			INSERT INTO platform.schools (id, organization_id, name, slug, subdomain, active, created_at, updated_at)
			VALUES (?, 2, ?, ?, ?, true, ?, ?)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				slug = EXCLUDED.slug,
				subdomain = EXCLUDED.subdomain,
				updated_at = EXCLUDED.updated_at
		`, school.id, school.name, school.slug, school.subdomain, now, now).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to create %s: %w", school.slug, err)
		}
	}

	// 6. Sync sequences so future inserts don't collide with explicit IDs
	_, err = s.tx.NewRaw(`SELECT setval('platform.organizations_id_seq', GREATEST(2, (SELECT COALESCE(MAX(id), 0) FROM platform.organizations)))`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync organizations sequence: %w", err)
	}
	_, err = s.tx.NewRaw(`SELECT setval('platform.schools_id_seq', GREATEST(5, (SELECT COALESCE(MAX(id), 0) FROM platform.schools)))`).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to sync schools sequence: %w", err)
	}

	if s.verbose {
		log.Println("Created 2 organizations: 'Test Träger' (school-a, school-b) and 'Träger Nord' (ogs-nordpark, ogs-waldblick, ogs-flussaue)")
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

	// Map the admin account to all other schools — enables switch-tenant testing across orgs
	if s.result.AdminAccount != nil {
		adminRole := s.result.Roles[0] // Admin role
		otherTenantIDs := []int64{2, 3, 4, 5}
		for _, tenantID := range otherTenantIDs {
			_, err = s.tx.NewRaw(`
				INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
				VALUES (?, ?, 'active', ?, ?, ?)
				ON CONFLICT (account_id, tenant_id) DO NOTHING
			`, s.result.AdminAccount.ID, tenantID, now, now, now).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to map admin to tenant %d: %w", tenantID, err)
			}

			_, err = s.tx.NewRaw(`
				INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING
			`, s.result.AdminAccount.ID, adminRole.ID, tenantID, now, now).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to assign admin role for tenant %d: %w", tenantID, err)
			}
		}
	}

	if s.verbose {
		log.Printf("Created %d account-tenant mappings for school-a, admin mapped to all schools", schoolACount)
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

// seedTraegerNordData creates minimal datasets for all schools of "Träger Nord" (tenant_ids 3-5).
// Each school gets: 3 rooms, 2 staff with accounts, 1 education group, 3 students.
func (s *Seeder) seedTraegerNordData(ctx context.Context) error {
	now := time.Now()

	schools := []struct {
		tenantID int64
		slug     string
		rooms    []struct {
			name     string
			building string
			capacity int
			category string
		}
		staff []struct {
			first string
			last  string
			email string
		}
		students []struct {
			first string
			last  string
		}
		groupName  string
		groupClass string
	}{
		{
			tenantID: 3, slug: "ogs-nordpark",
			rooms: []struct {
				name     string
				building string
				capacity int
				category string
			}{
				{"Klassenraum Eiche", "Hauptgebäude", 26, "Classroom"},
				{"OGS-Raum", "Nebengebäude", 22, "Activity Room"},
				{"Mensa", "Nebengebäude", 50, "Cafeteria"},
			},
			staff: []struct {
				first string
				last  string
				email string
			}{
				{"Petra", "Scholz", "p.scholz@nordpark.example.com"},
				{"Markus", "Wendt", "m.wendt@nordpark.example.com"},
			},
			students: []struct {
				first string
				last  string
			}{
				{"Emma", "Fischer"},
				{"Noah", "Weber"},
				{"Mia", "Hartmann"},
			},
			groupName: "1A", groupClass: "1A",
		},
		{
			tenantID: 4, slug: "ogs-waldblick",
			rooms: []struct {
				name     string
				building string
				capacity int
				category string
			}{
				{"Klassenraum Birke", "Hauptgebäude", 28, "Classroom"},
				{"Betreuungsraum", "OGS-Trakt", 20, "Activity Room"},
				{"Mensa", "OGS-Trakt", 45, "Cafeteria"},
			},
			staff: []struct {
				first string
				last  string
				email string
			}{
				{"Sandra", "Richter", "s.richter@waldblick.example.com"},
				{"Tobias", "Lang", "t.lang@waldblick.example.com"},
			},
			students: []struct {
				first string
				last  string
			}{
				{"Leon", "Koch"},
				{"Hannah", "Bauer"},
				{"Felix", "Schreiber"},
			},
			groupName: "1A", groupClass: "1A",
		},
		{
			tenantID: 5, slug: "ogs-flussaue",
			rooms: []struct {
				name     string
				building string
				capacity int
				category string
			}{
				{"Klassenraum Weide", "Schulgebäude", 24, "Classroom"},
				{"OGS-Raum", "Schulgebäude", 18, "Activity Room"},
				{"Mensa", "Schulgebäude", 40, "Cafeteria"},
			},
			staff: []struct {
				first string
				last  string
				email string
			}{
				{"Julia", "Neumann", "j.neumann@flussaue.example.com"},
				{"Stefan", "Kraus", "s.kraus@flussaue.example.com"},
			},
			students: []struct {
				first string
				last  string
			}{
				{"Sophia", "Maier"},
				{"Ben", "Hoffmann"},
				{"Lara", "Klein"},
			},
			groupName: "1A", groupClass: "1A",
		},
	}

	for _, school := range schools {
		// --- Rooms ---
		var roomIDs []int64
		for _, r := range school.rooms {
			var roomID int64
			err := s.tx.NewRaw(`
				INSERT INTO facilities.rooms (name, building, floor, capacity, category, tenant_id, created_at, updated_at)
				VALUES (?, ?, 0, ?, ?, ?, ?, ?)
				RETURNING id
			`, r.name, r.building, r.capacity, r.category, school.tenantID, now, now).Scan(ctx, &roomID)
			if err != nil {
				return fmt.Errorf("failed to create %s room %s: %w", school.slug, r.name, err)
			}
			roomIDs = append(roomIDs, roomID)
		}

		// --- Staff with accounts ---
		for i, sd := range school.staff {
			var personID int64
			err := s.tx.NewRaw(`
				INSERT INTO users.persons (first_name, last_name, tenant_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?)
				RETURNING id
			`, sd.first, sd.last, school.tenantID, now, now).Scan(ctx, &personID)
			if err != nil {
				return fmt.Errorf("failed to create %s person %s: %w", school.slug, sd.first, err)
			}

			var accountID int64
			err = s.tx.NewRaw(`
				INSERT INTO auth.accounts (email, password_hash, active, created_at, updated_at)
				VALUES (?, ?, true, ?, ?)
				ON CONFLICT (email) DO UPDATE SET updated_at = EXCLUDED.updated_at
				RETURNING id
			`, sd.email, s.result.AdminAccount.PasswordHash, now, now).Scan(ctx, &accountID)
			if err != nil {
				return fmt.Errorf("failed to create %s account %s: %w", school.slug, sd.email, err)
			}

			_, err = s.tx.NewRaw(`UPDATE users.persons SET account_id = ? WHERE id = ?`, accountID, personID).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to link %s person to account: %w", school.slug, err)
			}

			if len(s.result.Roles) > 1 {
				_, err = s.tx.NewRaw(`
					INSERT INTO auth.account_roles (account_id, role_id, tenant_id, created_at, updated_at)
					VALUES (?, ?, ?, ?, ?)
					ON CONFLICT (account_id, role_id, tenant_id) DO NOTHING
				`, accountID, s.result.Roles[1].ID, school.tenantID, now, now).Exec(ctx)
				if err != nil {
					return fmt.Errorf("failed to assign role for %s staff: %w", school.slug, err)
				}
			}

			_, err = s.tx.NewRaw(`
				INSERT INTO auth.account_tenants (account_id, tenant_id, status, activated_at, created_at, updated_at)
				VALUES (?, ?, 'active', ?, ?, ?)
				ON CONFLICT (account_id, tenant_id) DO NOTHING
			`, accountID, school.tenantID, now, now, now).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to map %s account to tenant: %w", school.slug, err)
			}

			notes := "Betreuungskraft OGS"
			if i == 0 {
				notes = "Leitung OGS"
			}
			_, err = s.tx.NewRaw(`
				INSERT INTO users.staff (person_id, staff_notes, tenant_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?)
			`, personID, notes, school.tenantID, now, now).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create %s staff: %w", school.slug, err)
			}
		}

		// --- Education group ---
		var groupID int64
		err := s.tx.NewRaw(`
			INSERT INTO education.groups (name, room_id, tenant_id, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?)
			RETURNING id
		`, school.groupName, roomIDs[0], school.tenantID, now, now).Scan(ctx, &groupID)
		if err != nil {
			return fmt.Errorf("failed to create %s education group: %w", school.slug, err)
		}

		// --- Students ---
		for _, sd := range school.students {
			var personID int64
			err := s.tx.NewRaw(`
				INSERT INTO users.persons (first_name, last_name, tenant_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?)
				RETURNING id
			`, sd.first, sd.last, school.tenantID, now, now).Scan(ctx, &personID)
			if err != nil {
				return fmt.Errorf("failed to create %s student %s: %w", school.slug, sd.first, err)
			}

			_, err = s.tx.NewRaw(`
				INSERT INTO users.students (person_id, school_class, group_id, tenant_id, created_at, updated_at)
				VALUES (?, ?, ?, ?, ?, ?)
			`, personID, school.groupClass, groupID, school.tenantID, now, now).Exec(ctx)
			if err != nil {
				return fmt.Errorf("failed to create %s student record %s: %w", school.slug, sd.first, err)
			}
		}

		if s.verbose {
			log.Printf("Created %s data: %d rooms, %d staff, %d students, 1 education group",
				school.slug, len(school.rooms), len(school.staff), len(school.students))
		}
	}

	return nil
}
