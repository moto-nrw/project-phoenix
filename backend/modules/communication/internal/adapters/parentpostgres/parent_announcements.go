package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
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
	SenderName              *string    `bun:"sender_name"`
	SenderSchoolClass       *string    `bun:"sender_school_class"`
	ReminderAt              *time.Time `bun:"reminder_at"`
	ReminderText            *string    `bun:"reminder_text"`
	ReminderSentAt          *time.Time `bun:"reminder_sent_at"`
	declarationColumns
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
		SenderName: r.SenderName, SenderSchoolClass: r.SenderSchoolClass,
		ReminderAt: r.ReminderAt, ReminderText: r.ReminderText, ReminderSentAt: r.ReminderSentAt,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, Declaration: r.declarationColumns.value(),
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
		SenderName: value.SenderName, SenderSchoolClass: value.SenderSchoolClass,
		ReminderAt: value.ReminderAt, ReminderText: value.ReminderText, ReminderSentAt: value.ReminderSentAt,
		declarationColumns: declarationRow(value.Declaration),
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return err
	}
	if announcement.TenantID == 0 {
		announcement.TenantID = scope.TenantID
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(parentAnnouncementRow)
	query := db.NewSelect().
		Model(row).
		ModelTableExpr(parentAnnouncementTableExpr).
		Where(`"parent_announcement".id = ?`, id).
		Limit(1)
	query = withTenant(query, parentAnnouncementAlias, scope)
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewDelete().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Where(`"parent_announcement".id = ?`, id)
	query = withTenant(query, parentAnnouncementAlias, scope)
	if _, err := query.Exec(ctx); err != nil {
		return fmt.Errorf("delete parent announcement: %w", err)
	}
	return nil
}

// ListForTenant returns the school's announcements newest-first, each with
// its target rows attached through one batched query.
func (s *AnnouncementStore) ListForTenant(ctx context.Context, includeInactive bool) ([]*domain.ParentAnnouncement, error) {
	db, scope, err := s.database(ctx)
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
	query = withTenant(query, parentAnnouncementAlias, scope)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list parent announcements for tenant: %w", err)
	}
	announcements := make([]*domain.ParentAnnouncement, 0, len(rows))
	for i := range rows {
		value := rows[i].value()
		announcements = append(announcements, value)
	}
	if err := s.attachTargets(ctx, db, scope.TenantID, announcements); err != nil {
		return nil, err
	}
	return announcements, nil
}

// attachTargets attaches target rows for a set of announcements in one query.
// Both list views use it so scheduler scans do not issue one target query per
// due reminder.
func (s *AnnouncementStore) attachTargets(ctx context.Context, db bun.IDB, tenantID int64, announcements []*domain.ParentAnnouncement) error {
	if len(announcements) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(announcements))
	byID := make(map[int64]*domain.ParentAnnouncement, len(announcements))
	for _, announcement := range announcements {
		announcement.Targets = []*domain.ParentAnnouncementTarget{}
		ids = append(ids, announcement.ID)
		byID[announcement.ID] = announcement
	}
	var targets []parentTargetRow
	targetQuery := db.NewSelect().
		Model(&targets).
		ModelTableExpr(parentTargetTableExpr).
		Where(`"pat".announcement_id IN (?)`, bun.List(ids)).
		OrderExpr(`"pat".id ASC`)
	targetQuery = withTenant(targetQuery, "pat", TenantScope{TenantID: tenantID})
	if err := targetQuery.Scan(ctx); err != nil {
		return fmt.Errorf("list parent announcement targets for tenant: %w", err)
	}
	for i := range targets {
		if announcement, ok := byID[targets[i].AnnouncementID]; ok {
			announcement.Targets = append(announcement.Targets, targets[i].value())
		}
	}
	return nil
}

// UpdateDraft writes only the editable content columns of a DRAFT and is
// atomic against a concurrent publish: the WHERE requires published_at IS
// NULL, so a publish that lands between the service's read and this write
// matches nothing and returns ErrParentAnnouncementPublished instead of
// silently reverting the published wording. Editing a draft also invalidates
// the read/ack rows and poll answers of the retracted wording.
func (s *AnnouncementStore) UpdateDraft(ctx context.Context, announcement *domain.ParentAnnouncement) error {
	if announcement == nil || announcement.ID <= 0 {
		return errors.New("update parent announcement draft: announcement id is required")
	}
	db, scope, err := s.database(ctx)
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
		Set("reminder_at = ?", announcement.ReminderAt).
		Set("reminder_text = ?", announcement.ReminderText).
		Set("updated_at = ?", now).
		Where(`"parent_announcement".id = ?`, announcement.ID).
		Where(`"parent_announcement".published_at IS NULL`)
	query = withTenant(setDeclarationColumns(query, announcement.Declaration), parentAnnouncementAlias, scope)
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return err
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("published_at = ?", publishedAt).
		Where(`"parent_announcement".id = ?`, id)
	if publishedAt == nil {
		// Unpublishing starts a new publication cycle. A reminder belonged to
		// the withdrawn version, so its time, wording and sent marker must all
		// disappear before staff can schedule a reminder for the correction.
		query.Set("reminder_at = NULL").
			Set("reminder_text = NULL").
			Set("reminder_sent_at = NULL")
	}
	query = withTenant(query, parentAnnouncementAlias, scope)
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
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("published_at = ?", publishedAt).
		Where(`"parent_announcement".id = ?`, id).
		Where(`"parent_announcement".published_at IS NULL`)
	query = withTenant(query, parentAnnouncementAlias, scope)
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

// reminderLivePredicate is the row state in which a scheduled reminder may
// still fire: the announcement is published, active, and not expired at the
// moment the reminder is evaluated. Shared by the due-scan and the claim so the
// two cannot disagree. Bind arg: the evaluation instant (for expires_at).
const reminderLivePredicate = `"parent_announcement".reminder_at IS NOT NULL
	AND "parent_announcement".reminder_sent_at IS NULL
	AND "parent_announcement".published_at IS NOT NULL
	AND "parent_announcement".active
	AND ("parent_announcement".expires_at IS NULL OR "parent_announcement".expires_at > ?)`

// SetReminder writes the reminder moment and wording of one announcement and
// reports whether the write applied. The WHERE requires reminder_sent_at IS
// NULL: a reminder that already went out is history and stays as it was. A
// nil reminderAt removes the reminder together with its text (the CHECK
// constraint would refuse a text without a moment anyway).
func (s *AnnouncementStore) SetReminder(ctx context.Context, id int64, reminderAt *time.Time, reminderText *string) (bool, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	if reminderAt == nil {
		reminderText = nil
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("reminder_at = ?", reminderAt).
		Set("reminder_text = ?", reminderText).
		Set("updated_at = ?", time.Now()).
		Where(`"parent_announcement".id = ?`, id).
		Where(`"parent_announcement".reminder_sent_at IS NULL`)
	query = withTenant(query, parentAnnouncementAlias, scope)
	result, err := query.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("set parent announcement reminder: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("set parent announcement reminder rows affected: %w", err)
	}
	return affected > 0, nil
}

// ListDueReminders returns the live announcements whose unsent reminder fell
// due in (notBefore, dueBefore], oldest first, with targets attached. RLS and
// the tenant filter pin the scan to the current tenant.
func (s *AnnouncementStore) ListDueReminders(ctx context.Context, notBefore, dueBefore time.Time) ([]*domain.ParentAnnouncement, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []parentAnnouncementRow
	query := db.NewSelect().
		Model(&rows).
		ModelTableExpr(parentAnnouncementTableExpr).
		Where(reminderLivePredicate, dueBefore).
		Where(`"parent_announcement".reminder_at > ?`, notBefore).
		Where(`"parent_announcement".reminder_at <= ?`, dueBefore).
		OrderExpr(`"parent_announcement".reminder_at ASC, "parent_announcement".id ASC`)
	query = withTenant(query, parentAnnouncementAlias, scope)
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list due parent announcement reminders: %w", err)
	}
	announcements := make([]*domain.ParentAnnouncement, 0, len(rows))
	for i := range rows {
		announcements = append(announcements, rows[i].value())
	}
	if err := s.attachTargets(ctx, db, scope.TenantID, announcements); err != nil {
		return nil, err
	}
	return announcements, nil
}

// ClaimReminder stamps reminder_sent_at and reports whether THIS call made
// the change. The guard repeats the liveness predicate: a tick that lost the
// race to an overlapping tick, to an unpublish, or to the expiry matches
// nothing and must not send.
func (s *AnnouncementStore) ClaimReminder(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("reminder_sent_at = ?", sentAt).
		Where(`"parent_announcement".id = ?`, id).
		Where(reminderLivePredicate, sentAt).
		Where(`"parent_announcement".reminder_at <= ?`, sentAt)
	query = withTenant(query, parentAnnouncementAlias, scope)
	result, err := query.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("claim parent announcement reminder: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("claim parent announcement reminder rows affected: %w", err)
	}
	return affected > 0, nil
}

// ReleaseReminderClaim restores an unsent reminder only when this scheduler
// run made the matching claim. It is used for a temporary notification-window
// suppression, which queued no delivery and must be retried after the window
// opens without rolling back later reminders in the tenant batch.
func (s *AnnouncementStore) ReleaseReminderClaim(ctx context.Context, id int64, sentAt time.Time) (bool, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return false, err
	}
	query := db.NewUpdate().
		Model((*parentAnnouncementRow)(nil)).
		ModelTableExpr(parentAnnouncementTableExpr).
		Set("reminder_sent_at = NULL").
		Where(`"parent_announcement".id = ?`, id).
		Where(`"parent_announcement".reminder_sent_at = ?`, sentAt)
	query = withTenant(query, parentAnnouncementAlias, scope)
	result, err := query.Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("release parent announcement reminder claim: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("release parent announcement reminder claim rows affected: %w", err)
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
