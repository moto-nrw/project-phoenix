package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type clarificationReviewRow struct {
	bun.BaseModel `bun:"table:parent_billing.beleg_clarification_reviews"`

	ID                   int64         `bun:"id,pk,autoincrement"`
	OrganizationID       int64         `bun:"organization_id"`
	BelegType            string        `bun:"beleg_type"`
	Reason               string        `bun:"reason"`
	Note                 string        `bun:"note"`
	Escalated            bool          `bun:"escalated"`
	EscalationReason     *string       `bun:"escalation_reason"`
	ReviewedOn           calendar.Date `bun:"reviewed_on,type:date"`
	NextReviewOn         calendar.Date `bun:"next_review_on,type:date"`
	ReviewedByOperatorID *int64        `bun:"reviewed_by_operator_id"`
	ReviewedAt           time.Time     `bun:"reviewed_at,nullzero,default:current_timestamp"`
}

func (r clarificationReviewRow) review() parentbilling.BelegClarificationReview {
	review := parentbilling.BelegClarificationReview{
		ID: r.ID, BelegType: r.BelegType, Reason: r.Reason, Note: r.Note, Escalated: r.Escalated,
		ReviewedOn: r.ReviewedOn, NextReviewOn: r.NextReviewOn, ReviewedAt: r.ReviewedAt,
	}
	if r.EscalationReason != nil {
		review.EscalationReason = *r.EscalationReason
	}
	return review
}

// InsertClarificationReview stores the review of the clarification entry of
// a type and reason and moves the review day of its Belege to the next
// review; the evaluation keeps that day while they stay listed for the same
// reason. An entry without a listed Beleg is not found.
func (s *Store) InsertClarificationReview(ctx context.Context, organizationID, operatorID int64, today calendar.Date,
	input parentbilling.NewBelegClarificationReview,
) (parentbilling.BelegClarificationReview, error) {
	db, err := adminDB(ctx)
	if err != nil {
		return parentbilling.BelegClarificationReview{}, err
	}
	if err := lockTraeger(ctx, db, organizationID); err != nil {
		return parentbilling.BelegClarificationReview{}, err
	}
	result, err := db.NewRaw(`UPDATE parent_billing.beleg_retentions SET review_on = ?
		WHERE organization_id = ? AND beleg_type = ? AND reason = ? AND state = 'clarification'`,
		input.NextReviewOn, organizationID, input.BelegType, input.Reason).Exec(ctx)
	if err != nil {
		return parentbilling.BelegClarificationReview{}, fmt.Errorf("parent billing postgres: move clarification review: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil {
		return parentbilling.BelegClarificationReview{}, err
	} else if affected == 0 {
		return parentbilling.BelegClarificationReview{}, parentbilling.ErrBelegClarificationNotFound
	}
	row := &clarificationReviewRow{
		OrganizationID: organizationID, BelegType: input.BelegType, Reason: input.Reason, Note: input.Note,
		Escalated: input.Escalated, ReviewedOn: today, NextReviewOn: input.NextReviewOn, ReviewedByOperatorID: optionalOperator(operatorID),
	}
	if input.Escalated {
		row.EscalationReason = &input.EscalationReason
	}
	if _, err := db.NewInsert().Model(row).Returning("id, reviewed_at").Exec(ctx); err != nil {
		return parentbilling.BelegClarificationReview{}, fmt.Errorf("parent billing postgres: insert clarification review: %w", writeError(err))
	}
	return row.review(), nil
}
