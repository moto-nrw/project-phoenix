package migrations

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The database refuses any change of an operator password that does not move
// the credential generation on (#6308), whatever writes it: a second writer
// in raw SQL, quoted or behind a table alias, or a BUN model update that
// writes every mapped column.

const operatorOldHash = "$argon2id$v=19$m=65536,t=3,p=2$old$old"

func operatorCredentialState(t *testing.T, db bun.IDB, id int64) (hash string, generation int64) {
	t.Helper()
	require.NoError(t, db.NewRaw(`SELECT password_hash, credential_generation FROM platform.operators WHERE id = ?`, id).
		Scan(context.Background(), &hash, &generation))
	return hash, generation
}

func credentialOperator(t *testing.T, db *bun.DB) int64 {
	t.Helper()
	operator := testpkg.CreateTestOperator(t, db)
	_, err := db.NewRaw(`UPDATE platform.operators SET password_hash = ?, credential_generation = credential_generation + 1 WHERE id = ?`,
		operatorOldHash, operator.ID).Exec(context.Background())
	require.NoError(t, err)
	return operator.ID
}

// credentialOperatorModel maps the credential columns the way a second ORM
// writer would; a model update writes all of them.
type credentialOperatorModel struct {
	bun.BaseModel `bun:"table:platform.operators,alias:operator"`

	ID                   int64  `bun:"id,pk"`
	Email                string `bun:"email"`
	PasswordHash         string `bun:"password_hash"`
	CredentialGeneration int64  `bun:"credential_generation"`
}

func TestOperatorPasswordChangeWithoutGenerationIsRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()

	for name, statement := range map[string]string{
		"plain column":       `UPDATE platform.operators SET password_hash = ? WHERE id = ?`,
		"aliased and quoted": `UPDATE "platform"."operators" AS "o" SET "password_hash" = ? WHERE "o"."id" = ?`,
	} {
		t.Run(name, func(t *testing.T) {
			id := credentialOperator(t, db)
			_, before := operatorCredentialState(t, db, id)
			_, err := db.NewRaw(statement, "$argon2id$v=19$m=65536,t=3,p=2$new$new", id).Exec(ctx)
			require.ErrorContains(t, err, "without moving its credential generation on")
			hash, generation := operatorCredentialState(t, db, id)
			assert.Equal(t, operatorOldHash, hash, "the password is unchanged")
			assert.Equal(t, before, generation)
		})
	}

	t.Run("model update writing every column", func(t *testing.T) {
		id := credentialOperator(t, db)
		row := credentialOperatorModel{ID: id}
		require.NoError(t, db.NewSelect().Model(&row).WherePK().Scan(ctx))
		row.PasswordHash = "$argon2id$v=19$m=65536,t=3,p=2$new$new"
		_, err := db.NewUpdate().Model(&row).WherePK().Exec(ctx)
		require.ErrorContains(t, err, "without moving its credential generation on")
	})
}

func TestOperatorPasswordChangeThatMovesTheGenerationIsAccepted(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	id := credentialOperator(t, db)
	_, before := operatorCredentialState(t, db, id)
	_, err := db.NewRaw(`UPDATE platform.operators SET password_hash = 'renewed', credential_generation = credential_generation + 1 WHERE id = ?`, id).Exec(context.Background())
	require.NoError(t, err)
	hash, generation := operatorCredentialState(t, db, id)
	assert.Equal(t, "renewed", hash)
	assert.Equal(t, before+1, generation)

	_, err = db.NewRaw(`UPDATE platform.operators SET display_name = 'Renamed' WHERE id = ?`, id).Exec(context.Background())
	require.NoError(t, err, "a write that leaves the password alone needs no step")
}

func TestOperatorCredentialGenerationNeverMovesBackOrToZero(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	id := testpkg.CreateTestOperator(t, db).ID
	_, generation := operatorCredentialState(t, db, id)
	assert.EqualValues(t, 1, generation, "a new operator starts past the generation reserved for artifacts from before the binding")
	_, err := db.NewRaw(`UPDATE platform.operators SET credential_generation = 3 WHERE id = ?`, id).Exec(ctx)
	require.NoError(t, err)
	_, err = db.NewRaw(`UPDATE platform.operators SET credential_generation = 2 WHERE id = ?`, id).Exec(ctx)
	require.ErrorContains(t, err, "never moves back")
	_, err = db.NewRaw(`INSERT INTO platform.operators (email, display_name, password_hash, credential_generation) VALUES (?, 'Zero', 'x', 0)`,
		fmt.Sprintf("operator-generation-zero-%d@test.local", testpkg.UniqueSuffix())).Exec(ctx)
	require.ErrorContains(t, err, "operators_credential_generation_positive")
}

// The binding refuses what was issued before it: a code or device row naming
// no generation stands at 0, which no operator ever holds.
func TestOperatorArtifactsFromBeforeTheBindingAreRefused(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	id := testpkg.CreateTestOperator(t, db).ID
	var challengeGeneration, deviceGeneration int64
	require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_mfa_email_challenges (operator_id, code_hash, expires_at)
		VALUES (?, 'hash', NOW() + interval '10 minutes') RETURNING credential_generation`, id).Scan(ctx, &challengeGeneration))
	require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_mfa_trusted_devices (operator_id, token_hash, expires_at)
		VALUES (?, ?, NOW() + interval '1 day') RETURNING credential_generation`, id, fmt.Sprintf("legacy-device-%d", testpkg.UniqueSuffix())).Scan(ctx, &deviceGeneration))
	_, current := operatorCredentialState(t, db, id)
	assert.Zero(t, challengeGeneration)
	assert.Zero(t, deviceGeneration)
	assert.NotEqual(t, current, challengeGeneration, "an unbound code matches no operator")
}

// A rollback must not revive what the binding had already ended: the code
// paths before it accept any active code and any unrevoked device. Down
// therefore spends every code and revokes every device of an older
// credential before the columns go, and a later Up refuses what remains
// until it is issued again.
func TestOperatorCredentialBindingRollbackKeepsStaleArtifactsEnded(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	id := credentialOperator(t, db)
	artifact := func(label string) (challenge, device int64) {
		require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_mfa_email_challenges (operator_id, code_hash, expires_at, credential_generation)
			VALUES (?, ?, NOW() + interval '10 minutes', (SELECT credential_generation FROM platform.operators WHERE id = ?)) RETURNING id`,
			id, label, id).Scan(ctx, &challenge))
		require.NoError(t, db.NewRaw(`INSERT INTO platform.operator_mfa_trusted_devices (operator_id, token_hash, expires_at, credential_generation)
			VALUES (?, encode(sha256(convert_to(?, 'UTF8')), 'hex'), NOW() + interval '30 days', (SELECT credential_generation FROM platform.operators WHERE id = ?)) RETURNING id`,
			id, label, id).Scan(ctx, &device))
		return challenge, device
	}
	staleChallenge, staleDevice := artifact("before-the-change")
	_, err := db.NewRaw(`UPDATE platform.operators SET password_hash = 'changed', credential_generation = credential_generation + 1 WHERE id = ?`, id).Exec(ctx)
	require.NoError(t, err)
	currentChallenge, currentDevice := artifact("after-the-change")

	require.NoError(t, operatorCredentialGenerationBindingDown(ctx, db))
	open := func(challenge, device int64) (bool, bool) {
		var challengeOpen, deviceOpen bool
		require.NoError(t, db.NewRaw(`SELECT consumed_at IS NULL FROM platform.operator_mfa_email_challenges WHERE id = ?`, challenge).Scan(ctx, &challengeOpen))
		require.NoError(t, db.NewRaw(`SELECT revoked_at IS NULL FROM platform.operator_mfa_trusted_devices WHERE id = ?`, device).Scan(ctx, &deviceOpen))
		return challengeOpen, deviceOpen
	}
	challengeOpen, deviceOpen := open(staleChallenge, staleDevice)
	assert.False(t, challengeOpen, "the rolled-back schema finds no open code of the old password")
	assert.False(t, deviceOpen, "the rolled-back schema finds no unrevoked device of the old password")
	challengeOpen, deviceOpen = open(currentChallenge, currentDevice)
	assert.True(t, challengeOpen, "a code of the current password survives the rollback")
	assert.True(t, deviceOpen, "a device of the current password survives the rollback")

	require.NoError(t, operatorCredentialGenerationBindingUp(ctx, db))
	var usable int
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM platform.operator_mfa_email_challenges AS challenge WHERE challenge.operator_id = ?
			AND challenge.consumed_at IS NULL AND challenge.credential_generation = (SELECT credential_generation FROM platform.operators WHERE id = ?))
		+ (SELECT count(*) FROM platform.operator_mfa_trusted_devices AS device WHERE device.operator_id = ?
			AND device.revoked_at IS NULL AND device.credential_generation = (SELECT credential_generation FROM platform.operators WHERE id = ?))`,
		id, id, id, id).Scan(ctx, &usable))
	assert.Zero(t, usable, "after Up again nothing issued without a binding is accepted")
}

// Down runs again on a schema it already rolled back, as every Down of the
// repository may: the columns are gone, so there is nothing left to spend or
// revoke, and Up applies again afterwards.
func TestOperatorCredentialBindingDownIsIdempotent(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	require.NoError(t, operatorCredentialGenerationBindingDown(ctx, db))
	require.NoError(t, operatorCredentialGenerationBindingDown(ctx, db), "a second Down finds nothing to do")
	require.NoError(t, operatorCredentialGenerationBindingUp(ctx, db))
	require.NoError(t, operatorCredentialGenerationBindingUp(ctx, db), "Up applies again too")
}
