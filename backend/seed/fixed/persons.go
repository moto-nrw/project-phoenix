package fixed

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/userpass"
	"github.com/moto-nrw/project-phoenix/logging"
	"github.com/moto-nrw/project-phoenix/models/auth"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// Common German names for seeding
var (
	firstNames = []string{
		// Adults
		"Andreas", "Sabine", "Michael", "Petra", "Thomas", "Andrea", "Stefan",
		"Claudia", "Frank", "Monika", "Markus", "Birgit", "Christian", "Ute",
		"Martin", "Karin", "Ralf", "Gabriele", "Jörg", "Susanne", "Klaus",
		"Martina", "Bernd", "Heike", "Wolfgang", "Stefanie", "Uwe", "Sandra",
		"Peter", "Silke", "Dirk", "Katrin", "Oliver", "Melanie", "Matthias",
		// Children
		"Leon", "Emma", "Ben", "Mia", "Paul", "Hannah", "Felix", "Sophia",
		"Jonas", "Emilia", "Noah", "Anna", "Elias", "Lena", "Finn", "Lea",
		"Luis", "Marie", "Luca", "Lara", "Max", "Clara", "Tom", "Luisa",
		"David", "Amelie", "Julian", "Johanna", "Niklas", "Emily", "Tim",
		"Laura", "Erik", "Nele", "Jan", "Charlotte", "Moritz", "Ida",
		"Philipp", "Greta", "Alexander", "Ella", "Jakob", "Maja", "Anton",
		"Sarah", "Samuel", "Alina", "Leo", "Lisa", "Simon", "Sophie",
		"Oskar", "Julia", "Emil", "Mila", "Maximilian", "Zoe", "Henry",
		"Frieda", "Theo", "Mathilda", "Vincent", "Paula", "Liam", "Helena",
		"Adrian", "Pia", "Lennard", "Viktoria", "Fabian", "Jasmin", "Milan",
		"Luna", "Rafael", "Finja", "Nico", "Eva", "Tobias", "Nina", "Florian",
		"Carla", "Daniel", "Romy", "Sebastian", "Annika", "Dominik", "Isabel",
		"Marcel", "Stella", "Robin", "Marlene", "Kevin", "Lucia", "Pascal",
		"Ronja", "Jannik", "Miriam", "Benedikt", "Antonia", "Aaron", "Celine",
		"Constantin", "Vanessa", "Frederick", "Rebecca", "Valentin", "Katharina",
		"Malte", "Franziska", "Johann", "Magdalena", "Richard", "Elisabeth",
		"Robert", "Victoria", "Gabriel", "Alexandra", "Joshua", "Christina",
		"Elijah", "Theresa", "Lucas", "Diana", "Nils", "Natalie",
	}

	lastNames = []string{
		"Müller", "Schmidt", "Schneider", "Fischer", "Weber", "Meyer", "Wagner",
		"Becker", "Schulz", "Hoffmann", "Schäfer", "Koch", "Bauer", "Richter",
		"Klein", "Wolf", "Schröder", "Neumann", "Schwarz", "Zimmermann", "Braun",
		"Krüger", "Hofmann", "Hartmann", "Lange", "Schmitt", "Werner", "Schmitz",
		"Krause", "Meier", "Lehmann", "Schmid", "Schulze", "Maier", "Köhler",
		"Herrmann", "König", "Walter", "Mayer", "Huber", "Kaiser", "Fuchs",
		"Peters", "Lang", "Scholz", "Möller", "Weiß", "Jung", "Hahn", "Vogel",
		"Friedrich", "Keller", "Günther", "Frank", "Berger", "Winkler", "Roth",
		"Beck", "Lorenz", "Baumann", "Schuster", "Ludwig", "Böhm", "Winter",
		"Kraus", "Martin", "Schubert", "Jäger", "Arndt", "Seidel", "Schreiber",
		"Graf", "Brandt", "Kuhn", "Dietrich", "Engel", "Pohl", "Horn", "Sauer",
		"Arnold", "Thomas", "Bergmann", "Busch", "Pfeiffer", "Voigt", "Götz",
	}
)

// seedAdminAccount creates the admin account
func (s *Seeder) seedAdminAccount(ctx context.Context) error {
	// Create admin account
	passwordHash, err := userpass.HashPassword("Test1234%", nil)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	admin := &auth.Account{
		Email:        "admin@example.com",
		PasswordHash: &passwordHash,
		Active:       true,
	}
	admin.CreatedAt = time.Now()
	admin.UpdatedAt = time.Now()

	// Use raw SQL to avoid schema issues
	err = s.tx.NewRaw(`
		INSERT INTO auth.accounts (email, password_hash, active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (email) DO UPDATE SET 
			password_hash = EXCLUDED.password_hash,
			updated_at = EXCLUDED.updated_at
		RETURNING id
	`, admin.Email, admin.PasswordHash, admin.Active, admin.CreatedAt, admin.UpdatedAt).
		Scan(ctx, &admin.ID)
	if err != nil {
		return fmt.Errorf("failed to create admin account: %w", err)
	}

	// Assign admin role
	adminRole := s.result.Roles[0] // Admin role should be first
	accountRole := &auth.AccountRole{
		AccountID: admin.ID,
		RoleID:    adminRole.ID,
	}
	accountRole.CreatedAt = time.Now()
	accountRole.UpdatedAt = time.Now()

	// Use raw SQL to avoid schema issues
	_, err = s.tx.NewRaw(`
		INSERT INTO auth.account_roles (account_id, role_id, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT (account_id, role_id) DO NOTHING
	`, accountRole.AccountID, accountRole.RoleID, accountRole.CreatedAt, accountRole.UpdatedAt).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to assign admin role: %w", err)
	}

	// Ensure admin account has an associated person record for user-context flows
	person := &users.Person{
		FirstName: "System",
		LastName:  "Administrator",
	}
	now := time.Now()
	person.CreatedAt = now
	person.UpdatedAt = now
	accountID := admin.ID
	person.AccountID = &accountID

	_, err = s.tx.NewInsert().Model(person).
		ModelTableExpr("users.persons").
		On("CONFLICT (account_id) DO UPDATE").
		Set("first_name = EXCLUDED.first_name").
		Set("last_name = EXCLUDED.last_name").
		Set(SQLExcludedUpdatedAt).
		Returning(SQLBaseColumns).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert admin person: %w", err)
	}

	// Ensure admin has a staff profile so SSE and supervision flows work
	adminStaff := &users.Staff{
		PersonID:   person.ID,
		StaffNotes: "Systemadministrator",
	}
	adminStaff.CreatedAt = now
	adminStaff.UpdatedAt = now

	_, err = s.tx.NewInsert().Model(adminStaff).
		ModelTableExpr("users.staff").
		On("CONFLICT (person_id) DO UPDATE").
		Set("staff_notes = EXCLUDED.staff_notes").
		Set(SQLExcludedUpdatedAt).
		Returning(SQLBaseColumns).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to upsert admin staff: %w", err)
	}

	s.result.AdminAccount = admin
	s.result.Accounts = append(s.result.Accounts, admin)

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithField("email", admin.Email).Info("Created admin account and staff profile")
	}

	return nil
}

// personSeedData holds data for seeding a single person
type personSeedData struct {
	index     int
	firstName string
	lastName  string
	isStaff   bool
	isTeacher bool
}

// createRFIDCard creates and upserts an RFID card for the given person index
func (s *Seeder) createRFIDCard(ctx context.Context, index int, rng *rand.Rand) (*users.RFIDCard, error) {
	rfidCard := &users.RFIDCard{Active: true}

	// Use hardcoded RFID tags for first 3 students (for Bruno tests)
	switch index {
	case 30:
		rfidCard.ID = "E83BE72F" // Leon Huber
	case 31:
		rfidCard.ID = "CA5DE789" // Emma Schreiber
	case 32:
		rfidCard.ID = "43385429" // Ben Sauer
	default:
		rfidCard.ID = generateRFIDTag(rng)
	}
	rfidCard.CreatedAt = time.Now()
	rfidCard.UpdatedAt = time.Now()

	_, err := s.tx.NewInsert().Model(rfidCard).
		ModelTableExpr("users.rfid_cards").
		On("CONFLICT (id) DO UPDATE").
		Set("active = EXCLUDED.active").
		Set(SQLExcludedUpdatedAt).
		Returning(SQLBaseColumns).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert RFID card: %w", err)
	}

	return rfidCard, nil
}

// createStaffAccount creates an account with PIN for a staff member
func (s *Seeder) createStaffAccount(ctx context.Context, data personSeedData) (*auth.Account, string, error) {
	email := fmt.Sprintf("%s.%s@example.com",
		normalizeForEmail(data.firstName),
		normalizeForEmail(data.lastName))

	pin := fmt.Sprintf("%04d", 1000+data.index)
	pinHash, err := userpass.HashPassword(pin, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash PIN: %w", err)
	}
	passwordHash, err := userpass.HashPassword("Test1234%", nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to hash password: %w", err)
	}

	account := &auth.Account{
		Email:        email,
		PasswordHash: &passwordHash,
		PINHash:      &pinHash,
		Active:       true,
	}
	account.CreatedAt = time.Now()
	account.UpdatedAt = time.Now()

	var id int64
	var createdAt, updatedAt time.Time
	err = s.tx.QueryRowContext(ctx, `
		INSERT INTO auth.accounts (created_at, updated_at, email, password_hash, pin_hash, active)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (email) DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			pin_hash = EXCLUDED.pin_hash,
			active = EXCLUDED.active,
			updated_at = EXCLUDED.updated_at
		RETURNING id, created_at, updated_at`,
		account.CreatedAt, account.UpdatedAt, account.Email,
		account.PasswordHash, account.PINHash, account.Active).Scan(&id, &createdAt, &updatedAt)
	if err != nil {
		return nil, "", fmt.Errorf("failed to upsert account for %s: %w", email, err)
	}
	account.ID = id
	account.CreatedAt = createdAt
	account.UpdatedAt = updatedAt

	return account, pin, nil
}

// findRoleByName finds a role by name from the seeded roles
func (s *Seeder) findRoleByName(name string) int64 {
	for _, role := range s.result.Roles {
		if role.Name == name {
			return role.ID
		}
	}
	return 0
}

// assignRoleToAccount assigns a role to an account
func (s *Seeder) assignRoleToAccount(ctx context.Context, accountID, roleID int64) error {
	if roleID == 0 {
		return nil
	}

	accountRole := &auth.AccountRole{
		AccountID: accountID,
		RoleID:    roleID,
	}
	accountRole.CreatedAt = time.Now()
	accountRole.UpdatedAt = time.Now()

	_, err := s.tx.NewInsert().Model(accountRole).
		ModelTableExpr("auth.account_roles").
		On("CONFLICT (account_id, role_id) DO UPDATE").
		Set(SQLExcludedUpdatedAt).
		Returning("created_at, updated_at").
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to assign role: %w", err)
	}
	return nil
}

// createPerson creates and upserts a person record
func (s *Seeder) createPerson(ctx context.Context, firstName, lastName string, tagID *string, accountID *int64) (*users.Person, error) {
	person := &users.Person{
		FirstName: firstName,
		LastName:  lastName,
		TagID:     tagID,
		AccountID: accountID,
	}
	person.CreatedAt = time.Now()
	person.UpdatedAt = time.Now()

	_, err := s.tx.NewInsert().Model(person).
		ModelTableExpr("users.persons").
		On("CONFLICT (tag_id) DO UPDATE").
		Set("first_name = EXCLUDED.first_name").
		Set("last_name = EXCLUDED.last_name").
		Set("account_id = EXCLUDED.account_id").
		Set(SQLExcludedUpdatedAt).
		Returning(SQLBaseColumns).
		Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to upsert person %s %s: %w", firstName, lastName, err)
	}

	return person, nil
}

// seedPersonsWithAccounts creates persons with RFID cards and accounts
func (s *Seeder) seedPersonsWithAccounts(ctx context.Context) error {
	rng := rand.New(rand.NewSource(42))
	const totalPersons = 150 // 30 staff (20 teachers) + 120 students

	for i := range totalPersons {
		data := s.buildPersonSeedData(i, rng)

		rfidCard, err := s.createRFIDCard(ctx, i, rng)
		if err != nil {
			return err
		}
		s.result.RFIDCards = append(s.result.RFIDCards, rfidCard)

		var accountID *int64
		if data.isStaff {
			account, pin, err := s.createStaffAccount(ctx, data)
			if err != nil {
				return err
			}
			accountID = &account.ID
			s.result.Accounts = append(s.result.Accounts, account)
			s.result.StaffWithPINs[account.Email] = pin

			roleName := "staff"
			if data.isTeacher {
				roleName = "teacher"
			}
			if err := s.assignRoleToAccount(ctx, account.ID, s.findRoleByName(roleName)); err != nil {
				return err
			}
		}

		person, err := s.createPerson(ctx, data.firstName, data.lastName, &rfidCard.ID, accountID)
		if err != nil {
			return err
		}
		s.result.Persons = append(s.result.Persons, person)
		s.result.PersonByID[person.ID] = person
	}

	if s.verbose && logging.Logger != nil {
		logging.Logger.WithFields(map[string]any{
			"persons":  len(s.result.Persons),
			"accounts": len(s.result.Accounts) - 1, // -1 for admin
		}).Info("Created persons with RFID cards and staff accounts")
	}

	return nil
}

// buildPersonSeedData generates seed data for a person at the given index
func (s *Seeder) buildPersonSeedData(index int, rng *rand.Rand) personSeedData {
	lastName := lastNames[rng.Intn(len(lastNames))]
	var firstName string

	isStaff := index < 30
	if isStaff {
		firstName = firstNames[index%35] // First 35 names are adults
	} else {
		firstName = firstNames[35+((index-30)%(len(firstNames)-35))]
	}

	return personSeedData{
		index:     index,
		firstName: firstName,
		lastName:  lastName,
		isStaff:   isStaff,
		isTeacher: index < 20,
	}
}

// Helper functions
func generateRFIDTag(rng *rand.Rand) string {
	// Generate realistic RFID tag UIDs (always 4 bytes for deterministic seeding)
	length := 4

	tag := make([]byte, length)
	for i := range tag {
		tag[i] = byte(rng.Intn(256))
	}

	// Convert to hex string
	hex := fmt.Sprintf("%X", tag)
	return hex
}

// Unused - kept for potential future use
// func generatePINCode(rng *rand.Rand) string {
// 	return fmt.Sprintf("%04d", rng.Intn(10000))
// }

func normalizeForEmail(name string) string {
	// Convert to lowercase first
	name = strings.ToLower(name)

	// Replace German umlauts and special characters
	replacements := map[string]string{
		"ä": "ae", "ö": "oe", "ü": "ue", "ß": "ss",
		"é": "e", "è": "e", "ê": "e", "à": "a",
		"á": "a", "ô": "o", "û": "u", "ç": "c",
	}

	for german, ascii := range replacements {
		name = strings.ReplaceAll(name, german, ascii)
	}

	return name
}
