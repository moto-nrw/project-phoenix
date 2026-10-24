package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/uptrace/bun"
)

// SurveyStore owns the surveys (#4781): the survey with its questions and
// options, who took part, and the answers. The participation and the answers
// are written and read apart: no statement here returns a participation
// together with an answer, and no answer statement returns a response id,
// an account of an anonymous survey or an order that follows the insertion.
type SurveyStore struct{ store }

// NewSurveyStore builds the survey store over the ambient transaction runtime.
func NewSurveyStore(database Database) *SurveyStore {
	return &SurveyStore{store: newStore(database)}
}

type surveyRow struct {
	bun.BaseModel    `bun:"table:users.surveys,alias:sv"`
	ID               int64                        `bun:"id,pk,autoincrement"`
	TenantID         int64                        `bun:"tenant_id,notnull"`
	Title            string                       `bun:"title,notnull"`
	Intro            string                       `bun:"intro,notnull"`
	Audience         string                       `bun:"audience,notnull"`
	Anonymous        bool                         `bun:"anonymous,notnull"`
	SendEmail        bool                         `bun:"send_email,notnull"`
	Targets          []communication.SurveyTarget `bun:"targets,type:jsonb,notnull"`
	ClosesAt         time.Time                    `bun:"closes_at,notnull"`
	PublishedAt      *time.Time                   `bun:"published_at"`
	AnnouncementID   *int64                       `bun:"announcement_id"`
	CreatedBy        *int64                       `bun:"created_by"`
	CreatedAt        time.Time                    `bun:"created_at,nullzero,notnull,default:current_timestamp"`
	UpdatedAt        time.Time                    `bun:"updated_at,nullzero,notnull,default:current_timestamp"`
	ParticipantCount int                          `bun:"participant_count,scanonly"`
}

func (r *surveyRow) value() *communication.Survey {
	targets := r.Targets
	if targets == nil {
		targets = []communication.SurveyTarget{}
	}
	return &communication.Survey{
		ID: r.ID, TenantID: r.TenantID, Title: r.Title, Intro: r.Intro, Audience: r.Audience,
		Anonymous: r.Anonymous, SendEmail: r.SendEmail, Targets: targets, ClosesAt: r.ClosesAt,
		PublishedAt: r.PublishedAt, AnnouncementID: r.AnnouncementID, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		ParticipantCount: r.ParticipantCount, Questions: []communication.SurveyQuestion{},
	}
}

type surveyQuestionRow struct {
	bun.BaseModel `bun:"table:users.survey_questions,alias:sq"`
	ID            int64  `bun:"id,pk,autoincrement"`
	TenantID      int64  `bun:"tenant_id,notnull"`
	SurveyID      int64  `bun:"survey_id,notnull"`
	Position      int    `bun:"position,notnull"`
	Kind          string `bun:"kind,notnull"`
	Prompt        string `bun:"prompt,notnull"`
	Required      bool   `bun:"required,notnull"`
}

type surveyOptionRow struct {
	bun.BaseModel `bun:"table:users.survey_question_options,alias:so"`
	ID            int64  `bun:"id,pk,autoincrement"`
	TenantID      int64  `bun:"tenant_id,notnull"`
	QuestionID    int64  `bun:"question_id,notnull"`
	Position      int    `bun:"position,notnull"`
	Label         string `bun:"label,notnull"`
}

const participantCountColumn = `(SELECT count(*) FROM users.survey_participations sp WHERE sp.survey_id = sv.id) AS participant_count`

func (s *SurveyStore) tenantDB(ctx context.Context, operation string) (bun.IDB, int64, error) {
	db, scope, err := s.database(ctx)
	if err != nil {
		return nil, 0, err
	}
	if scope.AcrossSchools || scope.TenantID <= 0 {
		return nil, 0, fmt.Errorf("%s: school transaction is required", operation)
	}
	return db, scope.TenantID, nil
}

// anyDB is the connection of the parent portal's administrative transaction;
// every statement on it names its school explicitly.
func (s *SurveyStore) anyDB(ctx context.Context) (bun.IDB, error) {
	db, _, err := s.database(ctx)
	return db, err
}

// ListSurveys returns the school's surveys, newest first, without questions.
func (s *SurveyStore) ListSurveys(ctx context.Context) ([]communication.Survey, error) {
	db, tenantID, err := s.tenantDB(ctx, "list surveys")
	if err != nil {
		return nil, err
	}
	var rows []surveyRow
	if err := db.NewSelect().Model(&rows).Column("sv.*").ColumnExpr(participantCountColumn).
		Where("sv.tenant_id = ?", tenantID).
		OrderExpr("sv.published_at IS NULL DESC, sv.closes_at DESC, sv.id DESC").
		Scan(ctx); err != nil {
		return nil, fmt.Errorf("list surveys: %w", err)
	}
	result := make([]communication.Survey, 0, len(rows))
	for i := range rows {
		result = append(result, *rows[i].value())
	}
	return result, nil
}

// FindSurvey returns one survey of the school with its questions, nil when
// absent. lock takes the row lock that serializes publishing, closing and
// deleting.
func (s *SurveyStore) FindSurvey(ctx context.Context, id int64, lock bool) (*communication.Survey, error) {
	db, tenantID, err := s.tenantDB(ctx, "find survey")
	if err != nil {
		return nil, err
	}
	return s.findSurvey(ctx, db, "sv.id = ? AND sv.tenant_id = ?", []any{id, tenantID}, lock)
}

// FindPortalSurvey returns the published parent survey an announcement of the
// given school carries, nil when there is none.
func (s *SurveyStore) FindPortalSurvey(ctx context.Context, tenantID, announcementID int64) (*communication.Survey, error) {
	db, err := s.anyDB(ctx)
	if err != nil {
		return nil, err
	}
	return s.findSurvey(ctx, db,
		"sv.announcement_id = ? AND sv.tenant_id = ? AND sv.audience = ? AND sv.published_at IS NOT NULL",
		[]any{announcementID, tenantID, communication.SurveyAudienceParents}, false)
}

func (s *SurveyStore) findSurvey(ctx context.Context, db bun.IDB, where string, args []any, lock bool) (*communication.Survey, error) {
	row := new(surveyRow)
	query := db.NewSelect().Model(row).Where(where, args...)
	if lock {
		query = query.For("UPDATE")
	} else {
		query = query.Column("sv.*").ColumnExpr(participantCountColumn)
	}
	if err := query.Scan(ctx); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find survey: %w", err)
	}
	survey := row.value()
	questions, err := loadSurveyQuestions(ctx, db, survey.TenantID, survey.ID)
	if err != nil {
		return nil, err
	}
	survey.Questions = questions
	return survey, nil
}

func loadSurveyQuestions(ctx context.Context, db bun.IDB, tenantID, surveyID int64) ([]communication.SurveyQuestion, error) {
	var questions []surveyQuestionRow
	if err := db.NewSelect().Model(&questions).
		Where("sq.survey_id = ? AND sq.tenant_id = ?", surveyID, tenantID).
		OrderExpr("sq.position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("load survey questions: %w", err)
	}
	var options []surveyOptionRow
	if err := db.NewSelect().Model(&options).
		Join("JOIN users.survey_questions AS sq ON sq.id = so.question_id").
		Where("sq.survey_id = ? AND so.tenant_id = ?", surveyID, tenantID).
		OrderExpr("so.question_id, so.position").Scan(ctx); err != nil {
		return nil, fmt.Errorf("load survey options: %w", err)
	}
	byQuestion := make(map[int64][]communication.SurveyOption, len(questions))
	for _, o := range options {
		byQuestion[o.QuestionID] = append(byQuestion[o.QuestionID], communication.SurveyOption{ID: o.ID, Label: o.Label})
	}
	result := make([]communication.SurveyQuestion, 0, len(questions))
	for _, q := range questions {
		opts := byQuestion[q.ID]
		if opts == nil {
			opts = []communication.SurveyOption{}
		}
		result = append(result, communication.SurveyQuestion{
			ID: q.ID, Position: q.Position, Kind: q.Kind, Prompt: q.Prompt, Required: q.Required, Options: opts,
		})
	}
	return result, nil
}

// CreateSurvey stores a new draft with its questions.
func (s *SurveyStore) CreateSurvey(ctx context.Context, in communication.SurveyInput, createdBy *int64) (*communication.Survey, error) {
	db, tenantID, err := s.tenantDB(ctx, "create survey")
	if err != nil {
		return nil, err
	}
	row := &surveyRow{
		TenantID: tenantID, Title: in.Title, Intro: in.Intro, Audience: in.Audience, Anonymous: in.Anonymous,
		SendEmail: in.SendEmail, Targets: in.Targets, ClosesAt: in.ClosesAt, CreatedBy: createdBy,
	}
	if row.Targets == nil {
		row.Targets = []communication.SurveyTarget{}
	}
	if _, err := db.NewInsert().Model(row).ExcludeColumn("participant_count").
		Returning("id, created_at, updated_at").Exec(ctx); err != nil {
		return nil, fmt.Errorf("create survey: %w", err)
	}
	if err := insertSurveyQuestions(ctx, db, tenantID, row.ID, in.Questions); err != nil {
		return nil, err
	}
	return s.FindSurvey(ctx, row.ID, false)
}

// ReplaceDraft overwrites a draft and its questions. It reports false when
// the survey is no draft (any more).
func (s *SurveyStore) ReplaceDraft(ctx context.Context, id int64, in communication.SurveyInput) (bool, error) {
	db, tenantID, err := s.tenantDB(ctx, "update survey")
	if err != nil {
		return false, err
	}
	targets := in.Targets
	if targets == nil {
		targets = []communication.SurveyTarget{}
	}
	row := &surveyRow{
		ID: id, TenantID: tenantID, Title: in.Title, Intro: in.Intro, Audience: in.Audience, Anonymous: in.Anonymous,
		SendEmail: in.SendEmail, Targets: targets, ClosesAt: in.ClosesAt,
	}
	result, err := db.NewUpdate().Model(row).
		Column("title", "intro", "audience", "anonymous", "send_email", "targets", "closes_at").
		Where("sv.id = ? AND sv.tenant_id = ? AND sv.published_at IS NULL", id, tenantID).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("update survey: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return false, err
	}
	if _, err := db.NewDelete().TableExpr("users.survey_questions").
		Where("survey_id = ? AND tenant_id = ?", id, tenantID).Exec(ctx); err != nil {
		return false, fmt.Errorf("replace survey questions: %w", err)
	}
	return true, insertSurveyQuestions(ctx, db, tenantID, id, in.Questions)
}

func insertSurveyQuestions(ctx context.Context, db bun.IDB, tenantID, surveyID int64, questions []communication.SurveyQuestionInput) error {
	for position, q := range questions {
		row := &surveyQuestionRow{TenantID: tenantID, SurveyID: surveyID, Position: position, Kind: q.Kind, Prompt: q.Prompt, Required: q.Required}
		if _, err := db.NewInsert().Model(row).Returning("id").Exec(ctx); err != nil {
			return fmt.Errorf("create survey question: %w", err)
		}
		if len(q.Options) == 0 {
			continue
		}
		options := make([]surveyOptionRow, 0, len(q.Options))
		for i, label := range q.Options {
			options = append(options, surveyOptionRow{TenantID: tenantID, QuestionID: row.ID, Position: i, Label: label})
		}
		if _, err := db.NewInsert().Model(&options).Exec(ctx); err != nil {
			return fmt.Errorf("create survey options: %w", err)
		}
	}
	return nil
}

// MarkPublished publishes a draft, with the carrier announcement of a parent
// survey. It reports false when the survey was no draft.
func (s *SurveyStore) MarkPublished(ctx context.Context, id int64, at time.Time, announcementID *int64) (bool, error) {
	db, tenantID, err := s.tenantDB(ctx, "publish survey")
	if err != nil {
		return false, err
	}
	result, err := db.NewUpdate().TableExpr("users.surveys").
		Set("published_at = ?", at).Set("announcement_id = ?", announcementID).
		Where("id = ? AND tenant_id = ? AND published_at IS NULL", id, tenantID).
		Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("publish survey: %w", err)
	}
	affected, err := result.RowsAffected()
	return affected == 1, err
}

// SetClosesAt moves the end of a survey.
func (s *SurveyStore) SetClosesAt(ctx context.Context, id int64, at time.Time) error {
	db, tenantID, err := s.tenantDB(ctx, "close survey")
	if err != nil {
		return err
	}
	result, err := db.NewUpdate().TableExpr("users.surveys").
		Set("closes_at = ?", at).
		Where("id = ? AND tenant_id = ?", id, tenantID).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("close survey: %w", err)
	}
	return assertOneRow(result, "close survey")
}

// DeleteSurvey removes a survey with its questions, participations and
// answers.
func (s *SurveyStore) DeleteSurvey(ctx context.Context, id int64) error {
	db, tenantID, err := s.tenantDB(ctx, "delete survey")
	if err != nil {
		return err
	}
	result, err := db.NewDelete().TableExpr("users.surveys").
		Where("id = ? AND tenant_id = ?", id, tenantID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete survey: %w", err)
	}
	return assertOneRow(result, "delete survey")
}

// ExpiredSurveys lists the school's surveys of one audience whose window has
// passed: published ones closed before the cutoff, drafts last changed
// before it. It returns their ids and carrier announcements.
func (s *SurveyStore) ExpiredSurveys(ctx context.Context, audience string, cutoff time.Time) ([]communication.Survey, error) {
	db, tenantID, err := s.tenantDB(ctx, "list expired surveys")
	if err != nil {
		return nil, err
	}
	var rows []surveyRow
	if err := db.NewSelect().Model(&rows).Column("sv.id", "sv.tenant_id", "sv.announcement_id").
		Where("sv.tenant_id = ? AND sv.audience = ?", tenantID, audience).
		Where("(sv.published_at IS NOT NULL AND sv.closes_at < ?) OR (sv.published_at IS NULL AND sv.updated_at < ?)", cutoff, cutoff).
		OrderExpr("sv.id").Scan(ctx); err != nil {
		return nil, fmt.Errorf("list expired surveys: %w", err)
	}
	result := make([]communication.Survey, 0, len(rows))
	for i := range rows {
		result = append(result, communication.Survey{ID: rows[i].ID, TenantID: rows[i].TenantID, AnnouncementID: rows[i].AnnouncementID})
	}
	return result, nil
}

// OpenStaffSurveys lists the school's published staff surveys still open at
// now, with whether the account took part.
func (s *SurveyStore) OpenStaffSurveys(ctx context.Context, accountID int64, now time.Time) ([]communication.SurveyToAnswer, error) {
	db, tenantID, err := s.tenantDB(ctx, "list open staff surveys")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ID           int64     `bun:"id"`
		Title        string    `bun:"title"`
		Intro        string    `bun:"intro"`
		Anonymous    bool      `bun:"anonymous"`
		ClosesAt     time.Time `bun:"closes_at"`
		Participated bool      `bun:"participated"`
	}
	if err := db.NewSelect().TableExpr("users.surveys AS sv").
		Column("sv.id", "sv.title", "sv.intro", "sv.anonymous", "sv.closes_at").
		ColumnExpr(`EXISTS (SELECT 1 FROM users.survey_participations sp WHERE sp.survey_id = sv.id AND sp.account_id = ?) AS participated`, accountID).
		Where("sv.tenant_id = ? AND sv.audience = ? AND sv.published_at IS NOT NULL AND sv.closes_at > ?", tenantID, communication.SurveyAudienceStaff, now).
		OrderExpr("sv.closes_at, sv.id").Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list open staff surveys: %w", err)
	}
	result := make([]communication.SurveyToAnswer, 0, len(rows))
	for _, r := range rows {
		result = append(result, communication.SurveyToAnswer{
			ID: r.ID, Title: r.Title, Intro: r.Intro, Anonymous: r.Anonymous, ClosesAt: r.ClosesAt, Participated: r.Participated,
		})
	}
	return result, nil
}

// PortalSummaries returns the survey behind each of the given carrier
// announcements, across the guardian's schools, with whether the account took
// part. The caller passes only announcements the guardian's feed holds.
func (s *SurveyStore) PortalSummaries(ctx context.Context, accountID int64, announcementIDs []int64) (map[int64]communication.PortalSurveySummary, error) {
	result := make(map[int64]communication.PortalSurveySummary, len(announcementIDs))
	if len(announcementIDs) == 0 {
		return result, nil
	}
	db, err := s.anyDB(ctx)
	if err != nil {
		return nil, err
	}
	var rows []struct {
		AnnouncementID int64     `bun:"announcement_id"`
		ID             int64     `bun:"id"`
		Anonymous      bool      `bun:"anonymous"`
		ClosesAt       time.Time `bun:"closes_at"`
		Participated   bool      `bun:"participated"`
	}
	if err := db.NewSelect().TableExpr("users.surveys AS sv").
		Column("sv.announcement_id", "sv.id", "sv.anonymous", "sv.closes_at").
		ColumnExpr(`EXISTS (SELECT 1 FROM users.survey_participations sp WHERE sp.survey_id = sv.id AND sp.account_id = ?) AS participated`, accountID).
		Where("sv.announcement_id IN (?) AND sv.audience = ? AND sv.published_at IS NOT NULL", bun.List(announcementIDs), communication.SurveyAudienceParents).
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list portal survey summaries: %w", err)
	}
	for _, r := range rows {
		result[r.AnnouncementID] = communication.PortalSurveySummary{SurveyID: r.ID, Anonymous: r.Anonymous, ClosesAt: r.ClosesAt, Participated: r.Participated}
	}
	return result, nil
}
