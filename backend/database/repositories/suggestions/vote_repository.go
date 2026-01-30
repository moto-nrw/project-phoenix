package suggestions

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/suggestions"
	"github.com/uptrace/bun"
)

// VoteRepository implements suggestions.VoteRepository
type VoteRepository struct {
	db *bun.DB
}

// NewVoteRepository creates a new VoteRepository
func NewVoteRepository(db *bun.DB) suggestions.VoteRepository {
	return &VoteRepository{db: db}
}

// conn returns the transaction from context if present, otherwise the base DB.
func (r *VoteRepository) conn(ctx context.Context) bun.IDB {
	if tx, ok := modelBase.TxFromContext(ctx); ok && tx != nil {
		return tx
	}
	return r.db
}

// Upsert creates or updates a vote using ON CONFLICT
func (r *VoteRepository) Upsert(ctx context.Context, vote *suggestions.Vote) error {
	if vote == nil {
		return fmt.Errorf("vote cannot be nil")
	}
	if err := vote.Validate(); err != nil {
		return err
	}

	_, err := r.conn(ctx).NewInsert().
		Model(vote).
		ModelTableExpr(tableVotes).
		On("CONFLICT (post_id, voter_id) DO UPDATE").
		Set("direction = EXCLUDED.direction").
		Set("updated_at = NOW()").
		Returning("*").
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "upsert vote", Err: err}
	}
	return nil
}

// DeleteByPostAndVoter removes a vote
func (r *VoteRepository) DeleteByPostAndVoter(ctx context.Context, postID, voterID int64) error {
	_, err := r.conn(ctx).NewDelete().
		TableExpr(tableVotes).
		Where("post_id = ? AND voter_id = ?", postID, voterID).
		Exec(ctx)
	if err != nil {
		return &modelBase.DatabaseError{Op: "delete vote", Err: err}
	}
	return nil
}

// FindByPostAndVoter returns the vote for a given post and voter, or nil
func (r *VoteRepository) FindByPostAndVoter(ctx context.Context, postID, voterID int64) (*suggestions.Vote, error) {
	vote := new(suggestions.Vote)
	err := r.conn(ctx).NewSelect().
		Model(vote).
		ModelTableExpr(`suggestions.votes AS "vote"`).
		Where(`"vote".post_id = ? AND "vote".voter_id = ?`, postID, voterID).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, &modelBase.DatabaseError{Op: "find vote", Err: err}
	}
	return vote, nil
}
