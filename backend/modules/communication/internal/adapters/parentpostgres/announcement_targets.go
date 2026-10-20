package parentpostgres

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

// ReplaceTargets swaps an announcement's target rows wholesale inside the
// caller's tenant transaction. A published row returns
// ErrParentAnnouncementPublished.
func (s *AnnouncementStore) ReplaceTargets(ctx context.Context, tenantID, announcementID int64, targets []*domain.ParentAnnouncementTarget) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	if err := s.lockDraft(ctx, db, tenantID, announcementID, "lock parent announcement for target replace"); err != nil {
		return err
	}
	if _, err := db.NewDelete().
		Model((*parentTargetRow)(nil)).
		ModelTableExpr(parentTargetTable).
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return fmt.Errorf("clear parent announcement targets: %w", err)
	}
	if len(targets) == 0 {
		return nil
	}
	rows := make([]parentTargetRow, 0, len(targets))
	for _, target := range targets {
		target.AnnouncementID = announcementID
		target.TenantID = tenantID
		rows = append(rows, parentTargetRow{
			TenantID: tenantID, AnnouncementID: announcementID, TargetType: target.TargetType,
			TargetRefID: target.TargetRefID, TargetRefText: target.TargetRefText,
		})
	}
	if _, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr(parentTargetTable).
		Returning("id, created_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("insert parent announcement targets: %w", err)
	}
	for i := range rows {
		targets[i].ID = rows[i].ID
		targets[i].CreatedAt = rows[i].CreatedAt
	}
	return nil
}

// ListTargets returns an announcement's target rows.
func (s *AnnouncementStore) ListTargets(ctx context.Context, announcementID int64) ([]*domain.ParentAnnouncementTarget, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentTargetRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentTargetTableExpr).
		Where(`"pat".announcement_id = ?`, announcementID).
		OrderExpr(`"pat".id ASC`)
	query = withTenant(query, "pat", scope)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent announcement targets: %w", err)
	}
	targets := make([]*domain.ParentAnnouncementTarget, 0, len(rows))
	for i := range rows {
		targets = append(targets, rows[i].value())
	}
	return targets, nil
}

// ReplaceOptions swaps a DRAFT poll's answer options wholesale. Options are
// referenced by answer rows, so changing them under a published poll would
// silently re-label answers guardians already gave; a published row returns
// ErrParentAnnouncementPublished.
func (s *AnnouncementStore) ReplaceOptions(ctx context.Context, tenantID, announcementID int64, options []*domain.ParentAnnouncementOption) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	if err := s.lockDraft(ctx, db, tenantID, announcementID, "lock parent announcement for option replace"); err != nil {
		return err
	}
	if _, err := db.NewDelete().
		Model((*parentOptionRow)(nil)).
		ModelTableExpr(parentOptionTable).
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return fmt.Errorf("clear parent announcement options: %w", err)
	}
	if len(options) == 0 {
		return nil
	}
	rows := make([]parentOptionRow, 0, len(options))
	for i, option := range options {
		option.AnnouncementID = announcementID
		option.Position = i
		option.TenantID = tenantID
		rows = append(rows, parentOptionRow{
			TenantID: tenantID, AnnouncementID: announcementID, Label: option.Label, Position: i,
		})
	}
	if _, err := db.NewInsert().
		Model(&rows).
		ModelTableExpr(parentOptionTable).
		Returning("id, created_at").
		Exec(ctx); err != nil {
		return fmt.Errorf("insert parent announcement options: %w", err)
	}
	for i := range rows {
		options[i].ID = rows[i].ID
		options[i].CreatedAt = rows[i].CreatedAt
	}
	return nil
}

// ListOptions returns a poll's options in display order.
func (s *AnnouncementStore) ListOptions(ctx context.Context, announcementID int64) ([]*domain.ParentAnnouncementOption, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentOptionRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentOptionTableExpr).
		Where(`"pao".announcement_id = ?`, announcementID).
		OrderExpr(`"pao".position ASC, "pao".id ASC`)
	query = withTenant(query, "pao", scope)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent announcement options: %w", err)
	}
	return optionValues(rows), nil
}

// ListOptionsForAnnouncements batches the option lookup for a whole feed
// page. No tenant filter: the announcement id set is already the caller's
// authorized scope, and this runs in the cross-tenant admin transaction where
// a per-row tenant filter would exclude everything.
func (s *AnnouncementStore) ListOptionsForAnnouncements(ctx context.Context, announcementIDs []int64) ([]*domain.ParentAnnouncementOption, error) {
	if len(announcementIDs) == 0 {
		return []*domain.ParentAnnouncementOption{}, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentOptionRow
	if err := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentOptionTableExpr).
		Where(`"pao".announcement_id IN (?)`, bun.List(announcementIDs)).
		OrderExpr(`"pao".announcement_id ASC, "pao".position ASC, "pao".id ASC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent announcement options for feed: %w", err)
	}
	return optionValues(rows), nil
}

func optionValues(rows []parentOptionRow) []*domain.ParentAnnouncementOption {
	options := make([]*domain.ParentAnnouncementOption, 0, len(rows))
	for i := range rows {
		options = append(options, rows[i].value())
	}
	return options
}

// liveVersionGuardCTE yields a single row IFF the announcement is still live
// (active, published, publish time reached, not expired) AND its published_at
// still equals the version the client loaded, so a concurrent correction
// cannot slip a stale read/ack onto the corrected wording. Bind args:
// announcementID, tenantID, expectedPublishedAt.
const liveVersionGuardCTE = `WITH guard AS (
		SELECT 1 FROM users.parent_announcements a
		WHERE a.id = ? AND a.tenant_id = ?
			AND a.active
			AND a.published_at = ?
			AND a.published_at <= clock_timestamp()
			AND (a.expires_at IS NULL OR a.expires_at > clock_timestamp())
	)`

const markReadSQL = liveVersionGuardCTE + `,
		ins AS (
			INSERT INTO users.parent_announcement_reads (tenant_id, announcement_id, account_id, read_at)
			SELECT ?, ?, ?, ?
			WHERE EXISTS (SELECT 1 FROM guard)
			ON CONFLICT (announcement_id, account_id) DO NOTHING
		)
		SELECT EXISTS (SELECT 1 FROM guard)`

// MarkRead upserts the account's read row while the announcement is still
// live at expectedPublishedAt. It returns true when the version guard matched
// (the write applied or a prior read already existed) and false when the guard
// missed, so the caller can answer with a stale conflict.
func (s *AnnouncementStore) MarkRead(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	var live bool
	if err := db.NewRaw(markReadSQL,
		announcementID, tenantID, expectedPublishedAt,
		tenantID, announcementID, accountID, time.Now(),
	).Scan(ctx, &live); err != nil {
		return false, fmt.Errorf("mark parent announcement read: %w", err)
	}
	return live, nil
}

// acknowledgeExistingSQL stamps acknowledged_at on an existing read row
// (keeping an earlier acknowledgement) while the version guard matches.
// Bind args: guard (3), acknowledgedAt, announcementID, accountID, tenantID.
const acknowledgeExistingSQL = liveVersionGuardCTE + `,
		upd AS (
			UPDATE users.parent_announcement_reads
			SET acknowledged_at = COALESCE(acknowledged_at, ?)
			WHERE announcement_id = ? AND account_id = ? AND tenant_id = ?
				AND EXISTS (SELECT 1 FROM guard)
			RETURNING 1
		)
		SELECT EXISTS (SELECT 1 FROM guard) AS live, EXISTS (SELECT 1 FROM upd) AS applied`

// acknowledgeInsertSQL creates the read row already acknowledged while the
// version guard matches; a concurrent first read wins the conflict and leaves
// the row to acknowledgeExistingSQL. Bind args: guard (3), tenantID,
// announcementID, accountID, readAt, acknowledgedAt.
const acknowledgeInsertSQL = liveVersionGuardCTE + `,
		ins AS (
			INSERT INTO users.parent_announcement_reads (tenant_id, announcement_id, account_id, read_at, acknowledged_at)
			SELECT ?, ?, ?, ?, ?
			WHERE EXISTS (SELECT 1 FROM guard)
			ON CONFLICT (announcement_id, account_id) DO NOTHING
			RETURNING 1
		)
		SELECT EXISTS (SELECT 1 FROM guard) AS live, EXISTS (SELECT 1 FROM ins) AS applied`

type guardedWriteRow struct {
	Live    bool `bun:"live"`
	Applied bool `bun:"applied"`
}

// MarkAcknowledged upserts the read row and stamps acknowledged_at, with the
// same version guard and return contract as MarkRead. It updates an existing
// row first, inserts when none exists, and re-runs the update when a
// concurrent first read won the insert conflict; every write carries the
// guard, so a stale request records nothing.
func (s *AnnouncementStore) MarkAcknowledged(ctx context.Context, tenantID, announcementID, accountID int64, expectedPublishedAt time.Time) (bool, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	now := time.Now()
	update := func() (guardedWriteRow, error) {
		var row guardedWriteRow
		if err := db.NewRaw(acknowledgeExistingSQL,
			announcementID, tenantID, expectedPublishedAt,
			now, announcementID, accountID, tenantID,
		).Scan(ctx, &row); err != nil {
			return row, fmt.Errorf("mark parent announcement acknowledged: %w", err)
		}
		return row, nil
	}
	updated, err := update()
	if err != nil || !updated.Live || updated.Applied {
		return updated.Live, err
	}
	var inserted guardedWriteRow
	if err := db.NewRaw(acknowledgeInsertSQL,
		announcementID, tenantID, expectedPublishedAt,
		tenantID, announcementID, accountID, now, now,
	).Scan(ctx, &inserted); err != nil {
		return false, fmt.Errorf("mark parent announcement acknowledged: %w", err)
	}
	if !inserted.Live || inserted.Applied {
		return inserted.Live, nil
	}
	retried, err := update()
	return retried.Live, err
}

// LockResponse serializes every answer replacement for one (poll, child) pair
// in the caller's transaction: option uniqueness alone would let two guardians
// both delete and then insert a different option for the same child.
func (s *AnnouncementStore) LockResponse(ctx context.Context, announcementID, studentID int64) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", responseLockKey(announcementID, studentID)).Exec(ctx); err != nil {
		return fmt.Errorf("lock parent announcement response: %w", err)
	}
	return nil
}

func responseLockKey(announcementID, studentID int64) string {
	return fmt.Sprintf("parent-announcement-response:%d:%d", announcementID, studentID)
}

// responseGuardCTE yields one row IFF the poll is still live at the expected
// version and still accepts answers. The audience and relationship half of
// the guard belongs to People Directory and is checked through the audience
// projection before each write. Bind args: announcementID, tenantID,
// expectedPublishedAt.
const responseGuardCTE = `WITH guard AS (
		SELECT a.response_type
		FROM users.parent_announcements a
		WHERE a.id = ? AND a.tenant_id = ?
			AND a.active AND a.published_at = ? AND a.published_at <= clock_timestamp()
			AND (a.expires_at IS NULL OR a.expires_at > clock_timestamp())
			AND a.response_type <> 'none'
			AND (a.response_deadline IS NULL OR a.response_deadline > clock_timestamp())
	)`

// responseSelectionCTE validates the requested option set against the poll:
// no duplicates, every id belongs to this poll, and a single-choice poll takes
// at most one. Bind args: optionIDs, announcementID, tenantID.
const responseSelectionCTE = `, requested AS (
		SELECT option_id FROM unnest(?::bigint[]) AS selection(option_id)
	), selection_valid AS (
		SELECT EXISTS (SELECT 1 FROM guard)
			AND (SELECT count(*) FROM requested) = (SELECT count(DISTINCT option_id) FROM requested)
			AND (SELECT count(*) FROM requested) = (
				SELECT count(*) FROM requested
				JOIN users.parent_announcement_options o ON o.id = requested.option_id
				WHERE o.announcement_id = ? AND o.tenant_id = ?
			)
			AND ((SELECT response_type FROM guard LIMIT 1) <> 'single_choice'
				OR (SELECT count(*) FROM requested) <= 1) AS valid
	)`

const deleteResponseSQL = responseGuardCTE + responseSelectionCTE + `, del AS (
		DELETE FROM users.parent_announcement_responses
		WHERE announcement_id = ? AND student_id = ? AND tenant_id = ?
			AND (SELECT valid FROM selection_valid)
	)
	SELECT valid FROM selection_valid`

const insertResponseSQL = responseGuardCTE + responseSelectionCTE + `, ins AS (
		INSERT INTO users.parent_announcement_responses
			(tenant_id, announcement_id, option_id, student_id, account_id, responded_at)
		SELECT ?, ?, o.id, ?, ?, ?
		FROM users.parent_announcement_options o
		WHERE o.announcement_id = ? AND o.tenant_id = ? AND o.id IN (?)
			AND (SELECT valid FROM selection_valid)
		ON CONFLICT (announcement_id, student_id, option_id) DO NOTHING
	)
	SELECT valid FROM selection_valid`

// DeleteResponse removes a child's current answer rows, gated on the poll
// still being live at expectedPublishedAt and the requested selection being
// valid for it. It returns whether the guard matched.
func (s *AnnouncementStore) DeleteResponse(ctx context.Context, tenantID, announcementID, studentID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	var live bool
	if err := db.NewRaw(deleteResponseSQL,
		announcementID, tenantID, expectedPublishedAt,
		pgdialect.Array(optionIDs), announcementID, tenantID,
		announcementID, studentID, tenantID,
	).Scan(ctx, &live); err != nil {
		return false, fmt.Errorf("replace parent announcement response: %w", err)
	}
	return live, nil
}

// InsertResponse stores one row per selected option for a child, under the
// same guard as DeleteResponse. It returns whether the guard matched.
func (s *AnnouncementStore) InsertResponse(ctx context.Context, tenantID, announcementID, studentID, accountID int64, optionIDs []int64, expectedPublishedAt time.Time) (bool, error) {
	db, _, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	var live bool
	if err := db.NewRaw(insertResponseSQL,
		announcementID, tenantID, expectedPublishedAt,
		pgdialect.Array(optionIDs), announcementID, tenantID,
		tenantID, announcementID, studentID, accountID, time.Now(), announcementID, tenantID, bun.List(optionIDs),
	).Scan(ctx, &live); err != nil {
		return false, fmt.Errorf("insert parent announcement response: %w", err)
	}
	return live, nil
}
