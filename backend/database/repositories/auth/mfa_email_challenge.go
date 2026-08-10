package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	"github.com/moto-nrw/project-phoenix/models/auth"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/uptrace/bun"
)

const (
	mfaEmailChallengeTable      = "auth.mfa_email_challenges"
	mfaEmailChallengeTableAlias = `auth.mfa_email_challenges AS "mfa_email_challenge"`
)

// MFAEmailChallengeRepository implements auth.MFAEmailChallengeRepository.
type MFAEmailChallengeRepository struct {
	*base.Repository[*auth.MFAEmailChallenge]
	db *bun.DB
}

// NewMFAEmailChallengeRepository creates a new repository for MFA email challenge codes.
func NewMFAEmailChallengeRepository(db *bun.DB) auth.MFAEmailChallengeRepository {
	return &MFAEmailChallengeRepository{
		Repository: base.NewRepository[*auth.MFAEmailChallenge](db, mfaEmailChallengeTable, "MfaEmailChallenge"),
		db:         db,
	}
}

// Create persists a new challenge after light validation.
func (r *MFAEmailChallengeRepository) Create(ctx context.Context, challenge *auth.MFAEmailChallenge) error {
	if challenge == nil {
		return fmt.Errorf("mfa email challenge cannot be nil")
	}
	if challenge.AccountID == 0 {
		return fmt.Errorf("account_id is required")
	}
	if challenge.CodeHash == "" {
		return fmt.Errorf("code_hash is required")
	}
	if challenge.ExpiresAt.IsZero() {
		return fmt.Errorf("expires_at is required")
	}
	return r.Repository.Create(ctx, challenge)
}

// FindActiveByAccountID returns the most recent unconsumed, unexpired challenge for an account.
// Returns a DatabaseError wrapping sql.ErrNoRows when none exists; callers should check for that.
func (r *MFAEmailChallengeRepository) FindActiveByAccountID(ctx context.Context, accountID int64) (*auth.MFAEmailChallenge, error) {
	challenge := new(auth.MFAEmailChallenge)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(challenge).
		ModelTableExpr(mfaEmailChallengeTableAlias).
		Where("account_id = ?", accountID).
		Where("consumed_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Order("expires_at DESC").
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active mfa email challenge", Err: err}
	}
	return challenge, nil
}

// FindActiveByIDForAccount returns one specific unconsumed, unexpired
// challenge, scoped to its owning account. This is the lookup the verify path
// uses: the challenge JWT names the row it was minted for, so a code emailed
// for one portal can never be redeemed against another in-flight challenge of
// the same account. The account_id predicate makes a guessed or forged id
// useless even though the id also travels inside a signed token.
// Returns a DatabaseError wrapping sql.ErrNoRows when no such row exists.
func (r *MFAEmailChallengeRepository) FindActiveByIDForAccount(ctx context.Context, id, accountID int64) (*auth.MFAEmailChallenge, error) {
	challenge := new(auth.MFAEmailChallenge)
	err := base.GetDB(ctx, r.db).NewSelect().
		Model(challenge).
		ModelTableExpr(mfaEmailChallengeTableAlias).
		Where("id = ?", id).
		Where("account_id = ?", accountID).
		Where("consumed_at IS NULL").
		Where("expires_at > ?", time.Now()).
		Limit(1).
		Scan(ctx)
	if err != nil {
		return nil, &modelBase.DatabaseError{Op: "find active mfa email challenge by id", Err: err}
	}
	return challenge, nil
}

// MarkConsumed marks a challenge as redeemed. The unique partial index on
// (account_id, expires_at) WHERE consumed_at IS NULL means the challenge
// becomes immediately invisible to FindActiveByAccountID after this update.
func (r *MFAEmailChallengeRepository) MarkConsumed(ctx context.Context, id int64, consumedAt time.Time) error {
	res, err := base.GetDB(ctx, r.db).NewUpdate().
		Model((*auth.MFAEmailChallenge)(nil)).
		ModelTableExpr(mfaEmailChallengeTable).
		Set("consumed_at = ?", consumedAt).
		Where(whereID, id).
		Where("consumed_at IS NULL").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "mark mfa email challenge consumed", Err: err}
	}
	return base.AssertRowsAffected(res, 1, "mark mfa email challenge consumed")
}

// CountRecentByAccountID returns the number of challenges issued to the account at or after `since`.
func (r *MFAEmailChallengeRepository) CountRecentByAccountID(ctx context.Context, accountID int64, since time.Time) (int, error) {
	count, err := base.GetDB(ctx, r.db).NewSelect().
		Model((*auth.MFAEmailChallenge)(nil)).
		ModelTableExpr(mfaEmailChallengeTableAlias).
		Where("account_id = ?", accountID).
		Where("created_at >= ?", since).
		Count(ctx)
	if err != nil {
		return 0, &modelBase.DatabaseError{Op: "count recent mfa email challenges", Err: err}
	}
	return count, nil
}

// DeleteExpired removes challenges past their TTL, regardless of consumption status.
// Intended for the periodic cleanup scheduler.
func (r *MFAEmailChallengeRepository) DeleteExpired(ctx context.Context) (int, error) {
	deleted, err := r.DeleteBefore(ctx, "expires_at", time.Now(), "delete expired mfa email challenges")
	return int(deleted), err
}
