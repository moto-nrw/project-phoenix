package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The database refuses any change of an account credential that does not
// move the credential generation on (#6289), whatever writes it: a second
// writer in raw SQL, quoted or behind a table alias, or a BUN model update
// that writes every mapped column.

func credentialAccount(t *testing.T, db *bun.DB, password bool) int64 {
	t.Helper()
	var hash any
	if password {
		hash = "$argon2id$v=19$m=65536,t=3,p=2$old$old"
	}
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.accounts (email, password_hash, active) VALUES (?, ?, TRUE) RETURNING id`,
		fmt.Sprintf("credential-trigger-%d@test.local", time.Now().UnixNano()), hash).Scan(context.Background(), &id))
	t.Cleanup(func() {
		_, err := db.NewRaw(`DELETE FROM auth.accounts WHERE id = ?`, id).Exec(context.Background())
		require.NoError(t, err)
	})
	return id
}

func credentialState(t *testing.T, db *bun.DB, id int64) (hash *string, otp bool, generation int64) {
	t.Helper()
	require.NoError(t, db.NewRaw(`SELECT password_hash, is_password_otp, mfa_reset_generation FROM auth.accounts WHERE id = ?`, id).
		Scan(context.Background(), &hash, &otp, &generation))
	return hash, otp, generation
}

// credentialAccountModel maps the credential columns the way a second ORM
// writer would; a model update writes all of them.
type credentialAccountModel struct {
	bun.BaseModel `bun:"table:auth.accounts,alias:account"`

	ID                 int64   `bun:"id,pk"`
	Email              string  `bun:"email"`
	PasswordHash       *string `bun:"password_hash"`
	IsPasswordOTP      bool    `bun:"is_password_otp"`
	MFAResetGeneration int64   `bun:"mfa_reset_generation"`
}

func TestCredentialChangeWithoutGenerationIsRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	for name, statement := range map[string]string{
		"quoted column":      `UPDATE auth.accounts SET "password_hash" = ? WHERE id = ?`,
		"aliased and quoted": `UPDATE "auth"."accounts" AS "a" SET "password_hash" = ? WHERE "a"."id" = ?`,
		"one-time flag":      `UPDATE auth.accounts SET is_password_otp = (? IS NOT NULL) WHERE id = ?`,
	} {
		t.Run(name, func(t *testing.T) {
			id := credentialAccount(t, db, true)
			_, err := db.NewRaw(statement, "$argon2id$v=19$m=65536,t=3,p=2$new$new", id).Exec(ctx)
			require.ErrorContains(t, err, "without moving its credential generation on")
			hash, _, generation := credentialState(t, db, id)
			assert.Equal(t, "$argon2id$v=19$m=65536,t=3,p=2$old$old", *hash, "the credential is unchanged")
			assert.Zero(t, generation)
		})
	}

	t.Run("model update writing every column", func(t *testing.T) {
		id := credentialAccount(t, db, true)
		row := credentialAccountModel{ID: id}
		require.NoError(t, db.NewSelect().Model(&row).WherePK().Scan(ctx))
		changed := "$argon2id$v=19$m=65536,t=3,p=2$new$new"
		row.PasswordHash = &changed
		_, err := db.NewUpdate().Model(&row).WherePK().Exec(ctx)
		require.ErrorContains(t, err, "without moving its credential generation on")
	})
}

func TestCredentialChangeThatMovesTheGenerationIsAccepted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	id := credentialAccount(t, db, true)
	_, err := db.NewRaw(`UPDATE auth.accounts SET password_hash = 'renewed', mfa_reset_generation = mfa_reset_generation + 1 WHERE id = ?`, id).Exec(context.Background())
	require.NoError(t, err)
	hash, _, generation := credentialState(t, db, id)
	assert.Equal(t, "renewed", *hash)
	assert.EqualValues(t, 1, generation)
}

func TestFirstPasswordKeepsTheGeneration(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	id := credentialAccount(t, db, false)
	_, err := db.NewRaw(`UPDATE auth.accounts SET password_hash = 'first' WHERE id = ?`, id).Exec(context.Background())
	require.NoError(t, err, "an account without a password takes its first one")
	hash, _, generation := credentialState(t, db, id)
	assert.Equal(t, "first", *hash)
	assert.Zero(t, generation, "nothing can be bound to an earlier credential")
}

func TestClearedCredentialMovesTheGenerationItself(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	id := credentialAccount(t, db, true)
	_, err := db.NewRaw(`UPDATE auth.accounts SET password_hash = NULL, is_password_otp = FALSE WHERE id = ?`, id).Exec(context.Background())
	require.NoError(t, err, "the erasures clear the credential without naming the generation")
	hash, _, generation := credentialState(t, db, id)
	assert.Nil(t, hash)
	assert.EqualValues(t, 1, generation, "clearing ends the artifacts of the old credential too")
}

func TestCredentialGenerationNeverMovesBack(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	id := credentialAccount(t, db, true)
	ctx := context.Background()
	_, err := db.NewRaw(`UPDATE auth.accounts SET mfa_reset_generation = 2 WHERE id = ?`, id).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE auth.accounts SET mfa_reset_generation = 1 WHERE id = ?`, id).Exec(ctx)
	require.ErrorContains(t, err, "never moves back")
}
