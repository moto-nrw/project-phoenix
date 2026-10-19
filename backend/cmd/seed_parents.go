package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"

	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	api "github.com/moto-nrw/project-phoenix/roots/owners"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"

	"github.com/spf13/cobra"

	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// seedParentsCmd promotes demo guardians (created by `seed`) into loginable
// parent-portal accounts. DEV ONLY — Identity provisions the same account and
// access records the guardian-invitation accept flow creates,
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

		db, err := transactionruntime.OpenDatabase()
		if err != nil {
			log.Fatalf("seed-parents: database connection failed: %v", err)
		}
		defer func() { _ = db.Close() }()

		if err := seedParentAccounts(context.Background(), db, count, password); err != nil {
			log.Fatalf("seed-parents: %v", err)
		}
	},
}

// assertLocalDevEnv refuses to run anywhere but an environment on the dev-only
// allow-list (isDevOnlyEnv), so the command can never create loginable
// guardian accounts against a staging or production database. It writes
// through DB_DSN, so refusing by APP_ENV alone is the only signal available
// before connecting.
func assertLocalDevEnv(appEnv string) error {
	if isDevOnlyEnv(appEnv) {
		return nil
	}
	return fmt.Errorf(
		"seed-parents is dev-only; refusing to run with APP_ENV=%q (allowed: development, test, local, demo, or unset)",
		appEnv,
	)
}

// isDevOnlyEnv is the allow-list of the dev-only commands. Unset APP_ENV is
// the local-dev default; demo is the public demo environment (ADR 0027), which
// holds synthetic data only. The dev tools binary (cmd/devtools) keeps the
// same list for seed, simulate and demo.
func isDevOnlyEnv(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "", "local", "development", "dev", "test", "demo":
		return true
	default:
		return false
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

func seedParentAccounts(ctx context.Context, db transactionruntime.Database, count int, password string) error {
	if count <= 0 {
		count = 5
	}

	passwordHash, err := securityruntime.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	repos := guardianSeedRepositories{GuardianProfile: api.NewGuardianProfileRepository(db), StudentGuardian: api.NewStudentGuardianRepository(db)}

	seedAccount, err := identityaccessCompose.NewGuardianSeedAccess(db.Pool())
	if err != nil {
		return fmt.Errorf("compose guardian access: %w", err)
	}
	ctx = tenant.WithUnitOfWork(ctx, db.UnitOfWork())

	var candidates []parentCandidate
	err = tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
		var readErr error
		candidates, readErr = promotableGuardians(txCtx, repos, count)
		return readErr
	})
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
		var reused bool
		err := tenant.WithinAdmin(ctx, func(txCtx context.Context) error {
			var promoteErr error
			reused, promoteErr = promoteGuardian(tenant.WithTenantID(txCtx, c.TenantID), repos, seedAccount, c, passwordHash)
			return promoteErr
		})
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
func promotableGuardians(ctx context.Context, repos guardianSeedRepositories, count int) ([]parentCandidate, error) {
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
		links, err := repos.StudentGuardian.FindByGuardianProfileID(tenant.WithTenantID(ctx, profile.GetTenantID()), profile.ID)
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
	repos guardianSeedRepositories,
	seedAccount func(context.Context, string, string) (int64, bool, error),
	c parentCandidate,
	passwordHash string,
) (bool, error) {
	accountID, reused, err := seedAccount(ctx, c.Email, passwordHash)
	if err != nil {
		return false, err
	}

	// Stamp the profile so the cross-tenant child query resolves.
	if err := repos.GuardianProfile.LinkAccount(ctx, c.ProfileID, accountID); err != nil {
		return false, fmt.Errorf("link guardian profile: %w", err)
	}

	return reused, nil
}

// guardianSeedRepositories are the guardian rows the parent-portal seed
// command promotes to portal accounts.
type guardianSeedRepositories struct {
	GuardianProfile peoplePorts.GuardianProfileRepository
	StudentGuardian peoplePorts.StudentGuardianRepository
}
