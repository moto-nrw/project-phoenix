package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
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
	ProfileID int64
	TenantID  int64
	Email     string
	FirstName string
	LastName  string
}

func seedParentAccounts(ctx context.Context, db *bun.DB, count int, password string) error {
	if count <= 0 {
		count = 5
	}

	passwordHash, err := authsvc.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	repos := repositories.NewFactory(db, repositories.NewUnobservedTimetableDependencies(db))

	role, err := repos.Role.FindByName(ctx, "guardian")
	if err != nil {
		return fmt.Errorf("look up guardian role: %w", err)
	}
	if role == nil {
		return fmt.Errorf("guardian role not found — run migrations first")
	}

	candidates, err := promotableGuardians(ctx, repos, count)
	if err != nil {
		return err
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

// promotableGuardians picks guardian profiles that already have an email and
// a student link but no account yet, ordered by id for determinism. The
// superuser connection sees every tenant, so the invitable list spans the
// whole demo database; the guardian rows belong to the People Directory
// owner and are read through its repositories (#2663).
func promotableGuardians(ctx context.Context, repos *repositories.Factory, count int) ([]parentCandidate, error) {
	profiles, err := repos.GuardianProfile.FindInvitable(ctx)
	if err != nil {
		return nil, fmt.Errorf("find promotable guardians: %w", err)
	}
	sort.Slice(profiles, func(i, j int) bool { return profiles[i].ID < profiles[j].ID })

	candidates := make([]parentCandidate, 0, count)
	for _, profile := range profiles {
		if len(candidates) == count {
			break
		}
		if profile.Email == nil {
			continue
		}
		email := strings.ToLower(strings.TrimSpace(*profile.Email))
		if email == "" {
			continue
		}
		links, err := repos.StudentGuardian.FindByGuardianProfileID(ctx, profile.ID)
		if err != nil {
			return nil, fmt.Errorf("find student links of guardian %d: %w", profile.ID, err)
		}
		if len(links) == 0 {
			continue
		}
		candidates = append(candidates, parentCandidate{
			ProfileID: profile.ID,
			TenantID:  profile.GetTenantID(),
			Email:     email,
			FirstName: profile.FirstName,
			LastName:  profile.LastName,
		})
	}
	return candidates, nil
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
