package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication/internal/domain"
	"github.com/uptrace/bun"
	"github.com/uptrace/bun/driver/pgdriver"
)

// Erklärungen (#3430): the declaration columns of an announcement, the frozen
// versions and the append-only submissions. Every statement touches
// Communication's own tables only.

const (
	declarationVersionTable          = "users.parent_announcement_declaration_versions"
	declarationVersionTableExpr      = `users.parent_announcement_declaration_versions AS "pdv"`
	declarationVersionAlias          = "pdv"
	declarationSubmissionTable       = "users.parent_announcement_declaration_submissions"
	declarationSubmissionTableExpr   = `users.parent_announcement_declaration_submissions AS "pds"`
	declarationSubmissionAlias       = "pds"
	declarationVersionUniqueViolated = "uq_parent_declaration_versions_number"
)

// ErrDeclarationVersionConflict reports a version number written concurrently.
var ErrDeclarationVersionConflict = errors.New("parent declaration version already exists")

// declarationColumns is embedded in parentAnnouncementRow. NULL kind/signers
// is every non-declaration announcement.
type declarationColumns struct {
	DeclarationKind             *string `bun:"declaration_kind"`
	DeclarationSigners          *string `bun:"declaration_signers"`
	DeclarationRevocable        bool    `bun:"declaration_revocable,notnull"`
	DeclarationRequiresPassword bool    `bun:"declaration_requires_password,notnull"`
}

func (c declarationColumns) value() domain.DeclarationSettings {
	settings := domain.DeclarationSettings{
		Revocable:        c.DeclarationRevocable,
		RequiresPassword: c.DeclarationRequiresPassword,
	}
	if c.DeclarationKind != nil {
		settings.Kind = *c.DeclarationKind
	}
	if c.DeclarationSigners != nil {
		settings.Signers = *c.DeclarationSigners
	}
	return settings
}

func declarationRow(settings domain.DeclarationSettings) declarationColumns {
	columns := declarationColumns{
		DeclarationRevocable:        settings.Revocable,
		DeclarationRequiresPassword: settings.RequiresPassword,
	}
	if settings.Kind != "" {
		kind := settings.Kind
		columns.DeclarationKind = &kind
	}
	if settings.Signers != "" {
		signers := settings.Signers
		columns.DeclarationSigners = &signers
	}
	return columns
}

// setDeclarationColumns adds the declaration columns to a draft update.
func setDeclarationColumns(query *bun.UpdateQuery, settings domain.DeclarationSettings) *bun.UpdateQuery {
	columns := declarationRow(settings)
	return query.
		Set("declaration_kind = ?", columns.DeclarationKind).
		Set("declaration_signers = ?", columns.DeclarationSigners).
		Set("declaration_revocable = ?", columns.DeclarationRevocable).
		Set("declaration_requires_password = ?", columns.DeclarationRequiresPassword)
}

type declarationVersionRow struct {
	bun.BaseModel  `bun:"table:users.parent_announcement_declaration_versions,alias:pdv"`
	ID             int64                             `bun:"id,pk,autoincrement"`
	TenantID       int64                             `bun:"tenant_id,notnull"`
	AnnouncementID int64                             `bun:"announcement_id,notnull"`
	VersionNo      int                               `bun:"version_no,notnull"`
	Title          string                            `bun:"title,notnull"`
	Body           string                            `bun:"body,notnull"`
	Kind           string                            `bun:"declaration_kind,notnull"`
	Attachments    []declarationAttachmentDigestJSON `bun:"attachments,type:jsonb,notnull"`
	ContentHash    string                            `bun:"content_hash,notnull"`
	PublishedAt    time.Time                         `bun:"published_at,notnull"`
}

type declarationAttachmentDigestJSON struct {
	AttachmentID int64  `json:"attachment_id"`
	Filename     string `json:"filename"`
	ContentType  string `json:"content_type"`
	SizeBytes    int64  `json:"size_bytes"`
	SHA256       string `json:"sha256"`
}

func (r *declarationVersionRow) value() *domain.DeclarationVersion {
	attachments := make([]domain.DeclarationAttachmentDigest, 0, len(r.Attachments))
	for _, a := range r.Attachments {
		attachments = append(attachments, domain.DeclarationAttachmentDigest(a))
	}
	return &domain.DeclarationVersion{
		ID: r.ID, TenantID: r.TenantID, AnnouncementID: r.AnnouncementID, VersionNo: r.VersionNo,
		Title: r.Title, Body: r.Body, Kind: r.Kind, Attachments: attachments,
		ContentHash: r.ContentHash, PublishedAt: r.PublishedAt,
	}
}

type declarationSubmissionRow struct {
	bun.BaseModel     `bun:"table:users.parent_announcement_declaration_submissions,alias:pds"`
	ID                int64     `bun:"id,pk,autoincrement"`
	TenantID          int64     `bun:"tenant_id,notnull"`
	AnnouncementID    int64     `bun:"announcement_id,notnull"`
	VersionID         int64     `bun:"version_id,notnull"`
	VersionNo         int       `bun:"version_no,scanonly"`
	StudentID         int64     `bun:"student_id,notnull"`
	AccountID         *int64    `bun:"account_id"`
	GuardianProfileID *int64    `bun:"guardian_profile_id"`
	SignerName        string    `bun:"signer_name,notnull"`
	GuardianRole      string    `bun:"guardian_role,notnull"`
	Action            string    `bun:"action,notnull"`
	Method            string    `bun:"method,notnull"`
	PasswordConfirmed bool      `bun:"password_confirmed,notnull"`
	ContentHash       string    `bun:"content_hash,notnull"`
	RecordHash        string    `bun:"record_hash,notnull"`
	SubmittedAt       time.Time `bun:"submitted_at,notnull"`
	CreatedAt         time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp"`
}

func (r *declarationSubmissionRow) value() *domain.DeclarationSubmission {
	return &domain.DeclarationSubmission{
		ID: r.ID, TenantID: r.TenantID, AnnouncementID: r.AnnouncementID, VersionID: r.VersionID,
		VersionNo: r.VersionNo, StudentID: r.StudentID, AccountID: r.AccountID,
		GuardianProfileID: r.GuardianProfileID, SignerName: r.SignerName, GuardianRole: r.GuardianRole,
		Action: r.Action, Method: r.Method, PasswordConfirmed: r.PasswordConfirmed,
		ContentHash: r.ContentHash, RecordHash: r.RecordHash, SubmittedAt: r.SubmittedAt,
	}
}

// LatestDeclarationVersion returns the newest version of an announcement, or
// nil when it was never published as a declaration.
func (s *AnnouncementStore) LatestDeclarationVersion(ctx context.Context, tenantID, announcementID int64) (*domain.DeclarationVersion, error) {
	db, ambient, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	row := new(declarationVersionRow)
	query := db.NewSelect().Model(row).ModelTableExpr(declarationVersionTableExpr).
		Where(`"pdv".announcement_id = ?`, announcementID).
		OrderExpr(`"pdv".version_no DESC`).
		Limit(1)
	query = withTenant(query, declarationVersionAlias, firstTenant(tenantID, ambient))
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find latest declaration version: %w", err)
	}
	return row.value(), nil
}

// ListDeclarationVersions returns every version, newest first.
func (s *AnnouncementStore) ListDeclarationVersions(ctx context.Context, tenantID, announcementID int64) ([]*domain.DeclarationVersion, error) {
	db, ambient, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationVersionRow
	query := db.NewSelect().Model(&rows).ModelTableExpr(declarationVersionTableExpr).
		Where(`"pdv".announcement_id = ?`, announcementID).
		OrderExpr(`"pdv".version_no DESC`)
	query = withTenant(query, declarationVersionAlias, firstTenant(tenantID, ambient))
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list declaration versions: %w", err)
	}
	versions := make([]*domain.DeclarationVersion, 0, len(rows))
	for i := range rows {
		versions = append(versions, rows[i].value())
	}
	return versions, nil
}

// LatestDeclarationVersions returns the newest version per announcement for a
// feed page in one statement. Explicit ids scope it; the caller has already
// resolved which announcements the account may see.
func (s *AnnouncementStore) LatestDeclarationVersions(ctx context.Context, announcementIDs []int64) (map[int64]*domain.DeclarationVersion, error) {
	out := make(map[int64]*domain.DeclarationVersion, len(announcementIDs))
	if len(announcementIDs) == 0 {
		return out, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationVersionRow
	if err := db.NewSelect().Model(&rows).ModelTableExpr(declarationVersionTableExpr).
		DistinctOn(`"pdv".announcement_id`).
		Where(`"pdv".announcement_id IN (?)`, bun.List(announcementIDs)).
		OrderExpr(`"pdv".announcement_id, "pdv".version_no DESC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list latest declaration versions: %w", err)
	}
	for i := range rows {
		out[rows[i].AnnouncementID] = rows[i].value()
	}
	return out, nil
}

// InsertDeclarationVersion writes a version. A concurrent publish that wrote
// the same number surfaces as ErrDeclarationVersionConflict.
func (s *AnnouncementStore) InsertDeclarationVersion(ctx context.Context, version *domain.DeclarationVersion) error {
	if version == nil {
		return errors.New("insert declaration version: version is required")
	}
	db, ambient, err := s.database(ctx)
	if err != nil {
		return err
	}
	attachments := make([]declarationAttachmentDigestJSON, 0, len(version.Attachments))
	for _, a := range version.Attachments {
		attachments = append(attachments, declarationAttachmentDigestJSON(a))
	}
	row := &declarationVersionRow{
		TenantID: firstTenant(version.TenantID, ambient), AnnouncementID: version.AnnouncementID,
		VersionNo: version.VersionNo, Title: version.Title, Body: version.Body, Kind: version.Kind,
		Attachments: attachments, ContentHash: version.ContentHash, PublishedAt: version.PublishedAt,
	}
	if _, err := db.NewInsert().Model(row).ModelTableExpr(declarationVersionTable).
		Returning("id").Exec(ctx); err != nil {
		if isUniqueViolation(err, declarationVersionUniqueViolated) {
			return ErrDeclarationVersionConflict
		}
		return fmt.Errorf("insert declaration version: %w", err)
	}
	version.ID = row.ID
	version.TenantID = row.TenantID
	return nil
}

type declarationSettingsRow struct {
	ID int64 `bun:"id"`
	declarationColumns
}

// DeclarationSettings returns the declaration columns per announcement id.
func (s *AnnouncementStore) DeclarationSettings(ctx context.Context, announcementIDs []int64) (map[int64]domain.DeclarationSettings, error) {
	out := make(map[int64]domain.DeclarationSettings, len(announcementIDs))
	if len(announcementIDs) == 0 {
		return out, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationSettingsRow
	if err := db.NewSelect().
		TableExpr(parentAnnouncementTableExpr).
		ColumnExpr(`"parent_announcement".id, "parent_announcement".declaration_kind,
			"parent_announcement".declaration_signers, "parent_announcement".declaration_revocable,
			"parent_announcement".declaration_requires_password`).
		Where(`"parent_announcement".id IN (?)`, bun.List(announcementIDs)).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("load declaration settings: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = row.value()
	}
	return out, nil
}

// CountDeclarationSubmissions counts every submission of an announcement, in
// any version. The delete guard asks it.
func (s *AnnouncementStore) CountDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) (int, error) {
	db, ambient, err := s.database(ctx)
	if err != nil {
		return 0, err
	}
	query := db.NewSelect().TableExpr(declarationSubmissionTableExpr).
		Where(`"pds".announcement_id = ?`, announcementID)
	query = withTenant(query, declarationSubmissionAlias, firstTenant(tenantID, ambient))
	count, err := query.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("count declaration submissions: %w", err)
	}
	return int(count), nil
}

// declarationSubmissionColumns selects a submission with its version number.
const declarationSubmissionColumns = `"pds".*, "pdv".version_no AS version_no`

const declarationSubmissionVersionJoin = `JOIN users.parent_announcement_declaration_versions AS "pdv"
	ON "pdv".id = "pds".version_id AND "pdv".tenant_id = "pds".tenant_id`

// ListDeclarationSubmissions returns the whole history, newest first.
func (s *AnnouncementStore) ListDeclarationSubmissions(ctx context.Context, tenantID, announcementID int64) ([]*domain.DeclarationSubmission, error) {
	db, ambient, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationSubmissionRow
	query := db.NewSelect().Model(&rows).ModelTableExpr(declarationSubmissionTableExpr).
		ColumnExpr(declarationSubmissionColumns).
		Join(declarationSubmissionVersionJoin).
		Where(`"pds".announcement_id = ?`, announcementID).
		OrderExpr(`"pds".submitted_at DESC, "pds".id DESC`)
	query = withTenant(query, declarationSubmissionAlias, firstTenant(tenantID, ambient))
	if err := query.Scan(ctx); err != nil {
		return nil, fmt.Errorf("list declaration submissions: %w", err)
	}
	return submissionValues(rows), nil
}

// ListDeclarationSubmissionsForStudents returns the history of the given
// announcements for the given children, newest first. The parent feed passes
// ids it already authorized.
func (s *AnnouncementStore) ListDeclarationSubmissionsForStudents(ctx context.Context, announcementIDs, studentIDs []int64) ([]*domain.DeclarationSubmission, error) {
	if len(announcementIDs) == 0 || len(studentIDs) == 0 {
		return []*domain.DeclarationSubmission{}, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationSubmissionRow
	if err := db.NewSelect().Model(&rows).ModelTableExpr(declarationSubmissionTableExpr).
		ColumnExpr(declarationSubmissionColumns).
		Join(declarationSubmissionVersionJoin).
		Where(`"pds".announcement_id IN (?)`, bun.List(announcementIDs)).
		Where(`"pds".student_id IN (?)`, bun.List(studentIDs)).
		OrderExpr(`"pds".submitted_at DESC, "pds".id DESC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list declaration submissions for students: %w", err)
	}
	return submissionValues(rows), nil
}

// ListDeclarationSubmissionsForAccountAndAnnouncements returns an account's
// submission history for already authorized announcements.
func (s *AnnouncementStore) ListDeclarationSubmissionsForAccountAndAnnouncements(ctx context.Context, accountID int64, announcementIDs []int64) ([]*domain.DeclarationSubmission, error) {
	if len(announcementIDs) == 0 {
		return []*domain.DeclarationSubmission{}, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationSubmissionRow
	if err := db.NewSelect().Model(&rows).ModelTableExpr(declarationSubmissionTableExpr).
		ColumnExpr(declarationSubmissionColumns).
		Join(declarationSubmissionVersionJoin).
		Where(`"pds".account_id = ?`, accountID).
		Where(`"pds".announcement_id IN (?)`, bun.List(announcementIDs)).
		OrderExpr(`"pds".submitted_at DESC, "pds".id DESC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list declaration submissions for account announcements: %w", err)
	}
	return submissionValues(rows), nil
}

// ListDeclarationSubmissionsForAccountInTenants returns an account's
// submission history within the already authorized feed schools.
func (s *AnnouncementStore) ListDeclarationSubmissionsForAccountInTenants(ctx context.Context, accountID int64, tenantIDs []int64) ([]*domain.DeclarationSubmission, error) {
	if len(tenantIDs) == 0 {
		return []*domain.DeclarationSubmission{}, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []declarationSubmissionRow
	if err := db.NewSelect().Model(&rows).ModelTableExpr(declarationSubmissionTableExpr).
		ColumnExpr(declarationSubmissionColumns).
		Join(declarationSubmissionVersionJoin).
		Where(`"pds".account_id = ?`, accountID).
		Where(`"pds".tenant_id IN (?)`, bun.List(tenantIDs)).
		OrderExpr(`"pds".submitted_at DESC, "pds".id DESC`).
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list declaration submissions for account tenants: %w", err)
	}
	return submissionValues(rows), nil
}

func submissionValues(rows []declarationSubmissionRow) []*domain.DeclarationSubmission {
	out := make([]*domain.DeclarationSubmission, 0, len(rows))
	for i := range rows {
		out = append(out, rows[i].value())
	}
	return out
}

// LockDeclarationChild serializes submissions for one (announcement, child)
// pair until the caller's transaction ends, so a double click or two guardians
// declaring at once are decided one after the other against the same history.
func (s *AnnouncementStore) LockDeclarationChild(ctx context.Context, announcementID, studentID int64) error {
	db, _, err := s.database(ctx)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("parent-declaration:%d:%d", announcementID, studentID)
	if _, err := db.NewRaw("SELECT pg_advisory_xact_lock(hashtextextended(?, 0))", key).Exec(ctx); err != nil {
		return fmt.Errorf("lock parent declaration: %w", err)
	}
	return nil
}

// InsertDeclarationSubmission appends one submission.
func (s *AnnouncementStore) InsertDeclarationSubmission(ctx context.Context, submission *domain.DeclarationSubmission) error {
	if submission == nil {
		return errors.New("insert declaration submission: submission is required")
	}
	db, ambient, err := s.database(ctx)
	if err != nil {
		return err
	}
	row := &declarationSubmissionRow{
		TenantID: firstTenant(submission.TenantID, ambient), AnnouncementID: submission.AnnouncementID,
		VersionID: submission.VersionID, StudentID: submission.StudentID, AccountID: submission.AccountID,
		GuardianProfileID: submission.GuardianProfileID, SignerName: submission.SignerName,
		GuardianRole: submission.GuardianRole, Action: submission.Action, Method: submission.Method,
		PasswordConfirmed: submission.PasswordConfirmed, ContentHash: submission.ContentHash,
		RecordHash: submission.RecordHash, SubmittedAt: submission.SubmittedAt,
	}
	if _, err := db.NewInsert().Model(row).ModelTableExpr(declarationSubmissionTable).
		Returning("id").Exec(ctx); err != nil {
		return fmt.Errorf("insert declaration submission: %w", err)
	}
	submission.ID = row.ID
	submission.TenantID = row.TenantID
	return nil
}

func firstTenant(explicit, ambient int64) int64 {
	if explicit > 0 {
		return explicit
	}
	return ambient
}

func isUniqueViolation(err error, constraint string) bool {
	var postgresError pgdriver.Error
	return errors.As(err, &postgresError) && postgresError.IntegrityViolation() && postgresError.Field('n') == constraint
}

type readDeclarationRow struct {
	ID       int64      `bun:"id"`
	Deadline *time.Time `bun:"response_deadline"`
}

// ReadOpenDeclarations returns the live, still open Erklärungen of the given
// schools that the account has opened. An opened Erklärung no longer counts
// as unread, but it still needs an answer; the parent flow decides from the
// audience whether this account owes one.
func (s *AnnouncementStore) ReadOpenDeclarations(ctx context.Context, accountID int64, tenantIDs []int64) (map[int64]*time.Time, error) {
	out := map[int64]*time.Time{}
	if len(tenantIDs) == 0 {
		return out, nil
	}
	db, _, err := s.database(ctx)
	if err != nil {
		return nil, err
	}
	var rows []readDeclarationRow
	if err := db.NewSelect().
		TableExpr(parentAnnouncementTableExpr).
		ColumnExpr(`"parent_announcement".id, "parent_announcement".response_deadline`).
		Join(`JOIN users.parent_announcement_reads AS par
			ON par.announcement_id = "parent_announcement".id AND par.tenant_id = "parent_announcement".tenant_id`).
		Where(`par.account_id = ?`, accountID).
		Where(`"parent_announcement".tenant_id IN (?)`, bun.List(tenantIDs)).
		Where(`"parent_announcement".delivery_mode = 'declaration'`).
		Where(`"parent_announcement".active`).
		Where(`"parent_announcement".published_at IS NOT NULL AND "parent_announcement".published_at <= NOW()`).
		Where(`("parent_announcement".expires_at IS NULL OR "parent_announcement".expires_at > NOW())`).
		Where(`("parent_announcement".response_deadline IS NULL OR "parent_announcement".response_deadline > NOW())`).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list read open declarations: %w", err)
	}
	for _, row := range rows {
		out[row.ID] = row.Deadline
	}
	return out, nil
}
