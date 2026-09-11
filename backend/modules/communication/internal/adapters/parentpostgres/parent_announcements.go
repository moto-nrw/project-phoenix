package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
)

const (
	parentAnnouncementTable     = "users.parent_announcements"
	parentAnnouncementTableExpr = `users.parent_announcements AS "parent_announcement"`
	parentAnnouncementAlias     = "parent_announcement"
	parentTargetTable           = "users.parent_announcement_targets"
	parentTargetTableExpr       = `users.parent_announcement_targets AS "pat"`
	parentOptionTable           = "users.parent_announcement_options"
	parentOptionTableExpr       = `users.parent_announcement_options AS "pao"`
	parentAnnouncementReadTable = "users.parent_announcement_reads"
	parentResponseTable         = "users.parent_announcement_responses"
)

// AnnouncementStore owns the parent-announcement rows: the announcement, its
// audience selectors, poll options, read/acknowledgement state, and poll
// answers. Every statement here touches Communication's own tables only; the
// audience reads that join People Directory live in the audience projection.
type AnnouncementStore struct{ store }

// NewAnnouncementStore builds the store over the ambient transaction runtime.
func NewAnnouncementStore(database Database) *AnnouncementStore {
	return &AnnouncementStore{store: newStore(database)}
}

type parentAnnouncementRow struct {
	bun.BaseModel           `bun:"table:users.parent_announcements,alias:parent_announcement"`
	ID                      int64      `bun:"id,pk,autoincrement"`
	TenantID                int64      `bun:"tenant_id,notnull"`
	CreatedAt               time.Time  `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt               time.Time  `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	Title                   string     `bun:"title,notnull"`
	Body                    string     `bun:"body,notnull"`
	Priority                string     `bun:"priority,notnull"`
	LinkURL                 *string    `bun:"link_url"`
	RequiresAcknowledgement bool       `bun:"requires_acknowledgement,notnull"`
	SendEmail               bool       `bun:"send_email,notnull"`
	PublishedAt             *time.Time `bun:"published_at"`
	ExpiresAt               *time.Time `bun:"expires_at"`
	Active                  bool       `bun:"active,notnull"`
	CreatedBy               int64      `bun:"created_by,notnull"`
	ResponseType            string     `bun:"response_type,notnull,nullzero,default:'none'"`
	ResponseDeadline        *time.Time `bun:"response_deadline"`
	DeliveryMode            string     `bun:"delivery_mode,notnull,nullzero,default:'standard'"`
	EmailAudience           string     `bun:"email_audience,notnull,nullzero,default:'portal_only'"`
	SystemKind              *string    `bun:"system_kind"`
}

func (r *parentAnnouncementRow) value() *domain.ParentAnnouncement {
	if r == nil {
		return nil
	}
	return &domain.ParentAnnouncement{
		ID: r.ID, TenantID: r.TenantID, Title: r.Title, Body: r.Body, Priority: r.Priority,
		LinkURL: r.LinkURL, RequiresAcknowledgement: r.RequiresAcknowledgement, SendEmail: r.SendEmail,
		PublishedAt: r.PublishedAt, ExpiresAt: r.ExpiresAt, Active: r.Active, CreatedBy: r.CreatedBy,
		ResponseType: r.ResponseType, ResponseDeadline: r.ResponseDeadline,
		DeliveryMode: r.DeliveryMode, EmailAudience: r.EmailAudience, SystemKind: r.SystemKind,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func announcementRow(value *domain.ParentAnnouncement) *parentAnnouncementRow {
	return &parentAnnouncementRow{
		ID: value.ID, TenantID: value.TenantID, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt,
		Title: value.Title, Body: value.Body, Priority: value.Priority, LinkURL: value.LinkURL,
		RequiresAcknowledgement: value.RequiresAcknowledgement, SendEmail: value.SendEmail,
		PublishedAt: value.PublishedAt, ExpiresAt: value.ExpiresAt, Active: value.Active,
		CreatedBy: value.CreatedBy, ResponseType: value.ResponseType, ResponseDeadline: value.ResponseDeadline,
		DeliveryMode: value.DeliveryMode, EmailAudience: value.EmailAudience, SystemKind: value.SystemKind,
	}
}

type parentTargetRow struct {
	bun.BaseModel  `bun:"table:users.parent_announcement_targets,alias:pat"`
	ID             int64     `bun:"id,pk,autoincrement"`
	TenantID       int64     `bun:"tenant_id,notnull"`
	AnnouncementID int64     `bun:"announcement_id,notnull"`
	TargetType     string    `bun:"target_type,notnull"`
	TargetRefID    *int64    `bun:"target_ref_id"`
	TargetRefText  *string   `bun:"target_ref_text"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

func (r *parentTargetRow) value() *domain.ParentAnnouncementTarget {
	return &domain.ParentAnnouncementTarget{
		ID: r.ID, TenantID: r.TenantID, AnnouncementID: r.AnnouncementID, TargetType: r.TargetType,
		TargetRefID: r.TargetRefID, TargetRefText: r.TargetRefText, CreatedAt: r.CreatedAt,
	}
}

type parentOptionRow struct {
	bun.BaseModel  `bun:"table:users.parent_announcement_options,alias:pao"`
	ID             int64     `bun:"id,pk,autoincrement"`
	TenantID       int64     `bun:"tenant_id,notnull"`
	AnnouncementID int64     `bun:"announcement_id,notnull"`
	Label          string    `bun:"label,notnull"`
	Position       int       `bun:"position,notnull"`
	CreatedAt      time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

func (r *parentOptionRow) value() *domain.ParentAnnouncementOption {
	return &domain.ParentAnnouncementOption{
		ID: r.ID, TenantID: r.TenantID, AnnouncementID: r.AnnouncementID,
		Label: r.Label, Position: r.Position, CreatedAt: r.CreatedAt,
	}
}

type parentAnnouncementReadRow struct {
	bun.BaseModel  `bun:"table:users.parent_announcement_reads,alias:par"`
	TenantID       int64      `bun:"tenant_id,notnull"`
	AnnouncementID int64      `bun:"announcement_id,pk"`
	AccountID      int64      `bun:"account_id,pk"`
	ReadAt         time.Time  `bun:"read_at,notnull"`
	AcknowledgedAt *time.Time `bun:"acknowledged_at"`
}

type parentAnnouncementResponseRow struct {
	bun.BaseModel  `bun:"table:users.parent_announcement_responses,alias:par_resp"`
	ID             int64     `bun:"id,pk,autoincrement"`
	TenantID       int64     `bun:"tenant_id,notnull"`
	AnnouncementID int64     `bun:"announcement_id,notnull"`
	OptionID       int64     `bun:"option_id,notnull"`
	StudentID      int64     `bun:"student_id,notnull"`
	AccountID      int64     `bun:"account_id,notnull"`
	RespondedAt    time.Time `bun:"responded_at,nullzero,notnull,default:current_timestamp"`
}

// Create inserts one announcement row. A zero tenant takes the ambient one.
func (s *AnnouncementStore) Create(ctx context.Context, announcement *domain.ParentAnnouncement) error {
	if announcement == nil {
		return errors.New("create parent announcement: announcement is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	if announcement.TenantID == 0 {
		announcement.TenantID = tenantID
	}
	row := announcementRow(announcement)
	if _, err := db.NewInsert().
		Model(row).
		ModelTableExpr(parentAnnouncementTable).
		Returning("id, created_at, updated_at, response_type, delivery_mode, email_audience").
		Exec(ctx); err != nil {
		return fmt.Errorf("create parent announcement: %w", err)
	}
	announcement.ID = row.ID
	announcement.CreatedAt = row.CreatedAt
	announcement.UpdatedAt = row.UpdatedAt
	announcement.ResponseType = row.ResponseType
	announcement.DeliveryMode = row.DeliveryMode
	announcement.EmailAudience = row.EmailAudience
	return nil
}

// FindByID returns the announcement within the current tenant, or nil.
func (s *AnnouncementStore) FindByID(ctx context.Context, id int64) (*domain.ParentAnnouncement, error) {
	return s.find(ctx, id, false)
}

// FindByIDForUpdate reads the announcement while holding its row lock until
// the transaction ends, or nil when absent. The attachment writes use it so
// that "still a draft, still under the limit" and the write that follows
// cannot be overtaken by a concurrent upload or publish.
func (s *AnnouncementStore) FindByIDForUpdate(ctx context.Context, id int64) (*domain.ParentAnnouncement, error) {
	return s.find(ctx, id, true)
}

func (s *AnnouncementStore) find(ctx context.Context, id int64, lock bool) (*domain.ParentAnnouncement, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(parentAnnouncementRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(parentAnnouncementTableExpr).
		Where(`"parent_announcement".id = ?`, id).
		Limit(1)
	query = withTenant(query, parentAnnouncementAlias, tenantID)
	if lock {
		query = query.For("UPDATE")
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find parent announcement %d: %w", id, err)
	}
	return row.value(), nil
}

// Delete removes the announcement by id; targets, options, reads, and
// responses cascade in the database.
func (s *AnnouncementStore) Delete(ctx context.Context, id int64) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Where(`"parent_announcement".id = ?`, id)
	query = withTenant(query, parentAnnouncementAlias, tenantID)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete parent announcement: %w", err)
	}
	return nil
}

// ListForTenant returns the school's announcements newest-first, each with
// its target rows attached through one batched query.
func (s *AnnouncementStore) ListForTenant(ctx context.Context, includeInactive bool) ([]*domain.ParentAnnouncement, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentAnnouncementRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentAnnouncementTableExpr).
		OrderExpr(`"parent_announcement".created_at DESC`)
	if !includeInactive {
		query = query.Where(`"parent_announcement".active`)
	}
	query = withTenant(query, parentAnnouncementAlias, tenantID)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent announcements for tenant: %w", err)
	}
	announcements := make([]*domain.ParentAnnouncement, 0, len(rows))
	if len(rows) == 0 {
		return announcements, nil
	}
	ids := make([]int64, 0, len(rows))
	byID := make(map[int64]*domain.ParentAnnouncement, len(rows))
	for i := range rows {
		value := rows[i].value()
		value.Targets = []*domain.ParentAnnouncementTarget{}
		announcements = append(announcements, value)
		ids = append(ids, value.ID)
		byID[value.ID] = value
	}
	var targets []parentTargetRow
	targetQuery := db.NewSelect().
		Model(&targets).
		ModelTableExpr(parentTargetTableExpr).
		Where(`"pat".announcement_id IN (?)`, bun.List(ids)).
		OrderExpr(`"pat".id ASC`)
	targetQuery = withTenant(targetQuery, "pat", tenantID)
	if err := targetQuery.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent announcement targets for tenant: %w", err)
	}
	for i := range targets {
		if announcement, ok := byID[targets[i].AnnouncementID]; ok {
			announcement.Targets = append(announcement.Targets, targets[i].value())
		}
	}
	return announcements, nil
}

// UpdateDraft writes only the editable content columns of a DRAFT and is
// atomic against a concurrent publish: the WHERE requires published_at IS
// NULL, so a publish that lands between the service's read and this write
// matches nothing and returns ErrParentAnnouncementPublished instead of
// silently reverting the published wording. Editing a draft also invalidates
// any read/ack rows and poll answers left over from a previous publication;
// they belong to the retracted wording.
func (s *AnnouncementStore) UpdateDraft(ctx context.Context, announcement *domain.ParentAnnouncement) error {
	if announcement == nil || announcement.ID <= 0 {
		return errors.New("update parent announcement draft: announcement id is required")
	}
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("title = ?", announcement.Title).
		Set("body = ?", announcement.Body).
		Set("priority = ?", announcement.Priority).
		Set("link_url = ?", announcement.LinkURL).
		Set("requires_acknowledgement = ?", announcement.RequiresAcknowledgement).
		Set("send_email = ?", announcement.SendEmail).
		Set("expires_at = ?", announcement.ExpiresAt).
		Set("response_type = ?", announcement.ResponseType).
		Set("response_deadline = ?", announcement.ResponseDeadline).
		Set("delivery_mode = ?", announcement.DeliveryMode).
		Set("email_audience = ?", announcement.EmailAudience).
		Set("updated_at = ?", now).
		Where(`"parent_announcement".id = ?`, announcement.ID).
		Where(`"parent_announcement".published_at IS NULL`)
	query = withTenant(query, parentAnnouncementAlias, tenantID)
	result, err := query.Exec(ctx)
	if err != nil {
		return fmt.Errorf("update parent announcement draft: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update parent announcement draft rows affected: %w", err)
	}
	if affected == 0 {
		return domain.ErrParentAnnouncementPublished
	}
	announcement.UpdatedAt = now
	return s.ClearEngagement(ctx, announcement.ID)
}

// ClearEngagement drops every read/acknowledgement and poll answer of an
// announcement. RLS pins both deletes to the current tenant.
func (s *AnnouncementStore) ClearEngagement(ctx context.Context, announcementID int64) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	if _, err := db.NewDelete().
		Model((*parentAnnouncementReadRow)(nil)).
		ModelTableExpr(parentAnnouncementReadTable).
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return fmt.Errorf("clear parent announcement reads: %w", err)
	}
	if _, err := db.NewDelete().
		Model((*parentAnnouncementResponseRow)(nil)).
		ModelTableExpr(parentResponseTable).
		Where("announcement_id = ?", announcementID).
		Exec(ctx); err != nil {
		return fmt.Errorf("clear parent announcement responses: %w", err)
	}
	return nil
}

// SetPublished sets (or clears, when publishedAt is nil) the publication
// timestamp. RLS pins the update to the current tenant.
func (s *AnnouncementStore) SetPublished(ctx context.Context, id int64, publishedAt *time.Time) error {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("published_at = ?", publishedAt).
		Where(`"parent_announcement".id = ?`, id)
	query = withTenant(query, parentAnnouncementAlias, tenantID)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("set parent announcement published_at: %w", err)
	}
	return nil
}

// PublishIfDraft atomically flips a draft to published and reports whether
// THIS call made the change. A concurrent second publish blocks on the row
// lock, re-reads a now non-null published_at, and returns false, so the
// caller enqueues the opt-in e-mails only once.
func (s *AnnouncementStore) PublishIfDraft(ctx context.Context, id int64, publishedAt time.Time) (bool, error) {
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("published_at = ?", publishedAt).
		Where(`"parent_announcement".id = ?`, id).
		Where(`"parent_announcement".published_at IS NULL`)
	query = withTenant(query, parentAnnouncementAlias, tenantID)
	result, err := query.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("publish parent announcement: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("publish parent announcement rows affected: %w", err)
	}
	return affected > 0, nil
}

// lockDraft locks the announcement row and reports whether it is still a
// draft. Target and option swaps must only ever land on a draft: a publish
// that slipped in between the content update and the swap would otherwise
// e-mail the OLD audience while the feed shows the NEW one.
func (s *AnnouncementStore) lockDraft(ctx context.Context, db bun.IDB, tenantID, announcementID int64, operation string) error {
	var publishedAt *time.Time
	if err := db.NewSelect().
		ColumnExpr("published_at").
		TableExpr(parentAnnouncementTable).
		Where("id = ?", announcementID).
		Where("tenant_id = ?", tenantID).
		For("UPDATE").
		Scan(ctx, &publishedAt); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	if publishedAt != nil {
		return domain.ErrParentAnnouncementPublished
	}
	return nil
}

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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentTargetRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentTargetTableExpr).
		Where(`"pat".announcement_id = ?`, announcementID).
		OrderExpr(`"pat".id ASC`)
	query = withTenant(query, "pat", tenantID)
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
	db, tenantID, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentOptionRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentOptionTableExpr).
		Where(`"pao".announcement_id = ?`, announcementID).
		OrderExpr(`"pao".position ASC, "pao".id ASC`)
	query = withTenant(query, "pao", tenantID)
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
// within the caller's transaction. Option-level uniqueness alone is not
// enough: two guardians could otherwise both delete and then insert a
// different option for the same child.
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
