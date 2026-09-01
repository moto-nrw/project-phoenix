package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/database/repositories"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
)

// seedParentsCmd promotes demo guardians (created by `seed`) into loginable
// parent-portal accounts. DEV ONLY — it writes auth rows directly via the
// repositories, the same records the guardian-invitation accept flow creates,
// so the parents portal can be exercised without the email/token dance.
var seedParentsCmd = &cobra.Command{
	Use:   "seed-parents",
	Short: "Promote demo guardians into loginable parent accounts (DEV ONLY)",
	Long: `Turn existing demo guardian profiles into parent-portal accounts.

The API seeder (` + "`seed`" + `) creates guardian profiles and links them to
students, but never creates login accounts. This command picks guardian
profiles that already have a student link and an email but no account, then
creates — for each — an auth account (with a shared password), the "guardian"
role, an active account_tenants mapping, and stamps guardian_profile.account_id.

Because the guardian↔student links already exist, the promoted parents see
their children in the portal immediately.

The shared password is taken from --password, or the SEED_PARENT_PASSWORD
env var when the flag is omitted. There is no built-in default — a missing
password fails fast (no hardcoded credentials in source).

REQUIRES: a database reachable via DB_DSN (run after migrate + seed). DEV ONLY.

Usage:
  SEED_PARENT_PASSWORD='<password>' docker compose run server ./main seed-parents
  docker compose run server ./main seed-parents --count 8 --password '<password>'`,
	Run: func(cmd *cobra.Command, _ []string) {
		if err := assertLocalDevEnv(os.Getenv("APP_ENV")); err != nil {
			log.Fatal(err)
		}

		count, _ := cmd.Flags().GetInt("count")
		password, _ := cmd.Flags().GetString("password")
		if strings.TrimSpace(password) == "" {
			password = os.Getenv("SEED_PARENT_PASSWORD")
		}
		if strings.TrimSpace(password) == "" {
			log.Fatal("seed-parents: no password provided — pass --password or set SEED_PARENT_PASSWORD")
		}

		db, err := database.DBConn()
		if err != nil {
			log.Fatalf("seed-parents: database connection failed: %v", err)
		}
		defer func() { _ = db.Close() }()

		if err := seedParentAccounts(context.Background(), db, count, password); err != nil {
			log.Fatalf("seed-parents: %v", err)
		}
	},
}

// assertLocalDevEnv refuses to run anywhere but an explicitly local/dev/test
// environment. Unset APP_ENV (the local-dev default) is allowed; staging,
// production, or any unrecognised value is rejected via an allow-list so the
// command can never create loginable guardian accounts against a deployed
// database. It writes through DB_DSN, so refusing by APP_ENV alone is the
// only signal available before connecting.
func assertLocalDevEnv(appEnv string) error {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "", "local", "development", "dev", "test":
		return nil
	default:
		return fmt.Errorf(
			"seed-parents is dev-only; refusing to run with APP_ENV=%q (allowed: development, test, local, or unset)",
			appEnv,
		)
	}
}

func init() {
	RootCmd.AddCommand(seedParentsCmd)
	seedParentsCmd.Flags().Int("count", 5, "How many demo guardians to promote into parent accounts")
	seedParentsCmd.Flags().String("password", "", "Shared password for the seeded parent accounts (or set SEED_PARENT_PASSWORD)")
}

// parentCandidate is one promotable guardian profile.
type parentCandidate struct {
	ProfileID int64  `bun:"id"`
	TenantID  int64  `bun:"tenant_id"`
	Email     string `bun:"email"`
	FirstName string `bun:"first_name"`
	LastName  string `bun:"last_name"`
}

func seedParentAccounts(ctx context.Context, db *bun.DB, count int, password string) error {
	if count <= 0 {
		count = 5
	}

	passwordHash, err := authsvc.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	repos := repositories.NewFactory(db)

	role, err := repos.Role.FindByName(ctx, "guardian")
	if err != nil {
		return fmt.Errorf("look up guardian role: %w", err)
	}
	if role == nil {
		return fmt.Errorf("guardian role not found — run migrations first")
	}

	// Guardian profiles that already have a student link and an email but no
	// account yet. Newest student-bearing guardians first is irrelevant; order
	// by id for determinism.
	var candidates []parentCandidate
	const query = `
		SELECT DISTINCT gp.id, gp.tenant_id,
		       lower(btrim(gp.email)) AS email,
		       gp.first_name, gp.last_name
		FROM users.guardian_profiles AS gp
		JOIN users.students_guardians AS sg ON sg.guardian_profile_id = gp.id
		WHERE gp.account_id IS NULL
		  AND gp.email IS NOT NULL
		  AND btrim(gp.email) <> ''
		ORDER BY gp.id
		LIMIT ?
	`
	if err := db.NewRaw(query, count).Scan(ctx, &candidates); err != nil {
		return fmt.Errorf("find promotable guardians: %w", err)
	}
	if len(candidates) == 0 {
		fmt.Println("No promotable guardians found (need an email + a student link + no existing account).")
		fmt.Println("Run `seed` first, or all eligible guardians already have accounts.")
		return nil
	}

	fmt.Printf("Promoting %d guardian(s) into parent accounts:\n\n", len(candidates))
	created := 0
	for _, c := range candidates {
		reused, err := promoteGuardian(ctx, repos, role.ID, c, passwordHash)
		if err != nil {
			return fmt.Errorf("promote %s: %w", c.Email, err)
		}
		created++
		suffix := ""
		if reused {
			suffix = "  (existing account — password unchanged)"
		}
		fmt.Printf("  ✓ %s %s\n      email:    %s\n      login credential: %s%s\n",
			c.FirstName, c.LastName, c.Email, password, suffix)
	}

	fmt.Printf("\nDone. %d parent account(s) ready. Log in at the parents subdomain.\n", created)
	return nil
}

// promoteGuardian creates (or reuses) the account, assigns the guardian role,
// ensures an active tenant mapping, and links the profile. Mirrors the
// guardian-invitation accept flow. Returns whether an existing account was
// reused (in which case its password is left untouched).
func promoteGuardian(
	ctx context.Context,
	repos *repositories.Factory,
	roleID int64,
	c parentCandidate,
	passwordHash string,
) (bool, error) {
	reused := false
	account, err := repos.Account.FindByEmail(ctx, c.Email)
	if err == nil && account != nil {
		reused = true
	} else {
		account = &authModels.Account{
			Email:        c.Email,
			Active:       true,
			PasswordHash: &passwordHash,
		}
		if err := repos.Account.Create(ctx, account); err != nil {
			return false, fmt.Errorf("create account: %w", err)
		}
	}

	// Guardian role (idempotent). A not-found lookup (nil row, possibly with a
	// not-found error) means we must create it; only an existing row is skipped.
	existingRole, _ := repos.AccountRole.FindByAccountAndRole(ctx, account.ID, roleID)
	if existingRole == nil {
		assignment := &authModels.AccountRole{AccountID: account.ID, RoleID: roleID}
		assignment.SetTenantID(c.TenantID)
		if err := repos.AccountRole.Create(ctx, assignment); err != nil {
			return false, fmt.Errorf("assign guardian role: %w", err)
		}
	}

	// Active tenant mapping (idempotent via EnsureActive).
	now := time.Now()
	mapping := &authModels.AccountTenant{
		AccountID:   account.ID,
		TenantID:    c.TenantID,
		Status:      authModels.AccountTenantStatusActive,
		ActivatedAt: &now,
	}
	if err := repos.AccountTenant.EnsureActive(ctx, mapping); err != nil {
		return false, fmt.Errorf("link account to tenant: %w", err)
	}

	// Stamp the profile so the cross-tenant child query resolves.
	if err := repos.GuardianProfile.LinkAccount(ctx, c.ProfileID, account.ID); err != nil {
		return false, fmt.Errorf("link guardian profile: %w", err)
	}

	return reused, nil
}
