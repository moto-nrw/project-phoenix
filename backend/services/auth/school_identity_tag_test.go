package auth_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

// A transponder typed or scanned as "ab:cd-12 34" is the card stored as
// "ABCD1234". The school identity used to resolve it through the normalizing
// card repository; the module lookup must keep accepting the reader spelling
// and store the card's own id on the person (#3225).
func TestEnsureSchoolIdentityAcceptsTheReaderSpellingOfATag(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	card := testpkg.CreateTestRFIDCard(t, db, "ABCD")
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("tag-spelling-%d@example.com", time.Now().UnixNano()))
	testpkg.MapAccountToTenant(t, db, account.ID, testpkg.Tenant(t))

	provisioning, err := services.NewSchoolIdentityForTests(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)

	lower := strings.ToLower(card.ID)
	submitted := " " + lower[:2] + ":" + lower[2:4] + "-" + lower[4:6] + " " + lower[6:] + " "
	var identity *auth.SchoolIdentity
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		var ensureErr error
		identity, ensureErr = provisioning.EnsureSchoolIdentity(txCtx, auth.SchoolIdentityInput{
			AccountID:    account.ID,
			TenantID:     testpkg.Tenant(t),
			Role:         &auth.RoleFacts{Name: "admin", IsSystem: true},
			FirstName:    "Tag",
			LastName:     "Spelling",
			TagID:        &submitted,
			CreatePerson: true,
		})
		return ensureErr
	})
	require.NoError(t, err)
	require.NotNil(t, identity)

	var stored string
	require.NoError(t, db.NewRaw(`SELECT tag_id FROM users.persons WHERE id = ?`, identity.PersonID).Scan(context.Background(), &stored))
	require.Equal(t, card.ID, stored)
}

// An unknown transponder reaches the retained handlers as the services/auth
// sentinel they render as 400, with the German text unchanged.
func TestEnsureSchoolIdentityReportsAnUnknownTagAsTheRetainedSentinel(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	account := testpkg.CreateTestAccount(t, db, fmt.Sprintf("tag-unknown-%d@example.com", time.Now().UnixNano()))
	testpkg.MapAccountToTenant(t, db, account.ID, testpkg.Tenant(t))

	provisioning, err := services.NewSchoolIdentityForTests(db, testpkg.TenantRuntime(t, db))
	require.NoError(t, err)

	unknown := "NOTACARD"
	err = tenant.WithinCurrentTenant(ctx, func(txCtx context.Context) error {
		_, ensureErr := provisioning.EnsureSchoolIdentity(txCtx, auth.SchoolIdentityInput{
			AccountID:    account.ID,
			TenantID:     testpkg.Tenant(t),
			Role:         &auth.RoleFacts{Name: "admin", IsSystem: true},
			FirstName:    "Tag",
			LastName:     "Unknown",
			TagID:        &unknown,
			CreatePerson: true,
		})
		return ensureErr
	})
	require.ErrorIs(t, err, auth.ErrSchoolIdentityTagUnknown)
	require.True(t, auth.IsSchoolIdentityRequestError(err))
	require.Contains(t, err.Error(), "Der angegebene Transponder ist an dieser Schule nicht bekannt")
}
