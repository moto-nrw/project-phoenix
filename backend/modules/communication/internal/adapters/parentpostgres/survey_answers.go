package parentpostgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	"github.com/uptrace/bun"
)

type surveyParticipationRow struct {
	bun.BaseModel `bun:"table:users.survey_participations"`
	TenantID      int64 `bun:"tenant_id,notnull"`
	SurveyID      int64 `bun:"survey_id,notnull"`
	AccountID     int64 `bun:"account_id,notnull"`
}

type surveyResponseRow struct {
	bun.BaseModel `bun:"table:users.survey_responses,alias:sr"`
	ID            string `bun:"id,pk"`
	TenantID      int64  `bun:"tenant_id,notnull"`
	SurveyID      int64  `bun:"survey_id,notnull"`
	AccountID     *int64 `bun:"account_id"`
}

type surveyAnswerRow struct {
	bun.BaseModel `bun:"table:users.survey_answers"`
	TenantID      int64   `bun:"tenant_id,notnull"`
	ResponseID    string  `bun:"response_id,notnull"`
	QuestionID    int64   `bun:"question_id,notnull"`
	OptionID      *int64  `bun:"option_id"`
	ScaleValue    *int    `bun:"scale_value"`
	TextValue     *string `bun:"text_value"`
}

// RecordSubmission stores one submission: the participation of the account
// and, separately, the response with its answers. accountID names the
// response only for a named survey (named); an anonymous response is written
// without it, and the database refuses one that carries it. It reports false
// when the account took part already; then nothing is written.
//
// The participation and the response are two statements in one transaction.
// Neither returns the other's key, and the response id comes from
// gen_random_uuid(), so the stored rows share nothing the app could join.
func (s *SurveyStore) RecordSubmission(ctx context.Context, tenantID, surveyID, accountID int64, named bool, answers []communication.SurveyAnswerInput) (bool, error) {
	db, err := s.anyDB(ctx)
	if err != nil {
		return false, err
	}
	if err := lockOpenSurveyForSubmission(ctx, db, tenantID, surveyID); err != nil {
		return false, err
	}
	result, err := db.NewInsert().Model(&surveyParticipationRow{TenantID: tenantID, SurveyID: surveyID, AccountID: accountID}).
		On("CONFLICT (survey_id, account_id) DO NOTHING").Exec(ctx)
	if err != nil {
		return false, fmt.Errorf("record survey participation: %w", err)
	}
	if affected, err := result.RowsAffected(); err != nil || affected != 1 {
		return false, err
	}
	response := &surveyResponseRow{TenantID: tenantID, SurveyID: surveyID}
	if named {
		response.AccountID = &accountID
	}
	if _, err := db.NewInsert().Model(response).ExcludeColumn("id").Returning("id").Exec(ctx); err != nil {
		return false, fmt.Errorf("record survey response: %w", err)
	}
	rows := answerRows(tenantID, response.ID, answers)
	if len(rows) == 0 {
		return true, nil
	}
	if _, err := db.NewInsert().Model(&rows).Exec(ctx); err != nil {
		return false, fmt.Errorf("record survey answers: %w", err)
	}
	return true, nil
}

// lockOpenSurveyForSubmission serializes the final open check with closing a
// survey. PostgreSQL rechecks the predicate after waiting for a concurrent
// FOR UPDATE lock, so a submission that waited for CloseSurvey observes its
// committed close and writes nothing.
func lockOpenSurveyForSubmission(ctx context.Context, db bun.IDB, tenantID, surveyID int64) error {
	var row struct {
		Open bool `bun:"open"`
	}
	err := db.NewSelect().TableExpr("users.surveys").ColumnExpr("TRUE AS open").
		Where("id = ? AND tenant_id = ? AND published_at IS NOT NULL AND closes_at > clock_timestamp()", surveyID, tenantID).
		For("UPDATE").Scan(ctx, &row)
	if errors.Is(err, sql.ErrNoRows) {
		return communication.ErrSurveyNotOpen
	}
	if err != nil {
		return fmt.Errorf("lock open survey for submission: %w", err)
	}
	return nil
}

func answerRows(tenantID int64, responseID string, answers []communication.SurveyAnswerInput) []surveyAnswerRow {
	rows := make([]surveyAnswerRow, 0, len(answers))
	for _, a := range answers {
		base := surveyAnswerRow{TenantID: tenantID, ResponseID: responseID, QuestionID: a.QuestionID}
		switch {
		case len(a.OptionIDs) > 0:
			for _, optionID := range a.OptionIDs {
				row := base
				id := optionID
				row.OptionID = &id
				rows = append(rows, row)
			}
		case a.ScaleValue != nil:
			row := base
			row.ScaleValue = a.ScaleValue
			rows = append(rows, row)
		case a.Text != nil:
			row := base
			row.TextValue = a.Text
			rows = append(rows, row)
		}
	}
	return rows
}

// HasParticipated reports whether the account took part in the survey.
func (s *SurveyStore) HasParticipated(ctx context.Context, tenantID, surveyID, accountID int64) (bool, error) {
	db, err := s.anyDB(ctx)
	if err != nil {
		return false, err
	}
	exists, err := db.NewSelect().TableExpr("users.survey_participations").
		Where("tenant_id = ? AND survey_id = ? AND account_id = ?", tenantID, surveyID, accountID).Exists(ctx)
	if err != nil {
		return false, fmt.Errorf("check survey participation: %w", err)
	}
	return exists, nil
}

// ParticipantAccounts returns the accounts that took part, for the reminder
// to everyone else. It never leaves the service: the school sees counts only.
func (s *SurveyStore) ParticipantAccounts(ctx context.Context, surveyID int64) ([]int64, error) {
	db, tenantID, err := s.tenantDB(ctx, "list survey participants")
	if err != nil {
		return nil, err
	}
	var ids []int64
	if err := db.NewSelect().TableExpr("users.survey_participations").Column("account_id").
		Where("tenant_id = ? AND survey_id = ?", tenantID, surveyID).Scan(ctx, &ids); err != nil {
		return nil, fmt.Errorf("list survey participants: %w", err)
	}
	return ids, nil
}

// Tally counts the answers of a survey. Every statement groups or shuffles:
// none returns a response id, and the free texts come in random order.
func (s *SurveyStore) Tally(ctx context.Context, surveyID int64) (*communication.SurveyTally, error) {
	db, tenantID, err := s.tenantDB(ctx, "count survey answers")
	if err != nil {
		return nil, err
	}
	tally := &communication.SurveyTally{
		AnswerCounts: map[int64]int{}, OptionCounts: map[int64]int{}, ScaleCounts: map[int64][]int{}, Texts: map[int64][]string{},
	}
	if err := db.NewSelect().TableExpr("users.survey_responses").ColumnExpr("count(*)").
		Where("tenant_id = ? AND survey_id = ?", tenantID, surveyID).Scan(ctx, &tally.ResponseCount); err != nil {
		return nil, fmt.Errorf("count survey responses: %w", err)
	}
	if tally.ResponseCount == 0 {
		return tally, nil
	}
	if err := tallyAnswers(ctx, db, tenantID, surveyID, tally); err != nil {
		return nil, err
	}
	if err := tallyTexts(ctx, db, tenantID, surveyID, tally); err != nil {
		return nil, err
	}
	return tally, nil
}

func tallyAnswers(ctx context.Context, db bun.IDB, tenantID, surveyID int64, tally *communication.SurveyTally) error {
	var rows []struct {
		QuestionID int64  `bun:"question_id"`
		OptionID   *int64 `bun:"option_id"`
		ScaleValue *int   `bun:"scale_value"`
		Answers    int    `bun:"answers"`
		Responses  int    `bun:"responses"`
	}
	if err := db.NewSelect().TableExpr("users.survey_answers AS a").
		Join("JOIN users.survey_questions AS q ON q.id = a.question_id").
		ColumnExpr("a.question_id, a.option_id, a.scale_value, count(*) AS answers, count(DISTINCT a.response_id) AS responses").
		Where("a.tenant_id = ? AND q.survey_id = ? AND a.text_value IS NULL", tenantID, surveyID).
		GroupExpr("GROUPING SETS ((a.question_id, a.option_id, a.scale_value), (a.question_id))").
		Scan(ctx, &rows); err != nil {
		return fmt.Errorf("count survey answers: %w", err)
	}
	for _, r := range rows {
		switch {
		case r.OptionID != nil:
			tally.OptionCounts[*r.OptionID] = r.Answers
		case r.ScaleValue != nil:
			counts := tally.ScaleCounts[r.QuestionID]
			if counts == nil {
				counts = make([]int, communication.SurveyScaleMax)
			}
			if *r.ScaleValue >= communication.SurveyScaleMin && *r.ScaleValue <= communication.SurveyScaleMax {
				counts[*r.ScaleValue-1] = r.Answers
			}
			tally.ScaleCounts[r.QuestionID] = counts
		default:
			tally.AnswerCounts[r.QuestionID] = r.Responses
		}
	}
	return nil
}

func tallyTexts(ctx context.Context, db bun.IDB, tenantID, surveyID int64, tally *communication.SurveyTally) error {
	var rows []struct {
		QuestionID int64  `bun:"question_id"`
		Text       string `bun:"text_value"`
	}
	if err := db.NewSelect().TableExpr("users.survey_answers AS a").
		Join("JOIN users.survey_questions AS q ON q.id = a.question_id").
		Column("a.question_id", "a.text_value").
		Where("a.tenant_id = ? AND q.survey_id = ? AND a.text_value IS NOT NULL", tenantID, surveyID).
		OrderExpr("random()").
		Scan(ctx, &rows); err != nil {
		return fmt.Errorf("read survey texts: %w", err)
	}
	for _, r := range rows {
		tally.Texts[r.QuestionID] = append(tally.Texts[r.QuestionID], r.Text)
		tally.AnswerCounts[r.QuestionID]++
	}
	return nil
}

// NamedResponses returns the responses of a named survey with their accounts
// and answers. It reads only responses that name an account, so an
// anonymous survey yields none even if it were asked.
func (s *SurveyStore) NamedResponses(ctx context.Context, surveyID int64) ([]communication.SurveyNamedResponse, error) {
	db, tenantID, err := s.tenantDB(ctx, "list named survey responses")
	if err != nil {
		return nil, err
	}
	var rows []struct {
		ResponseID string  `bun:"response_id"`
		AccountID  int64   `bun:"account_id"`
		QuestionID *int64  `bun:"question_id"`
		OptionID   *int64  `bun:"option_id"`
		ScaleValue *int    `bun:"scale_value"`
		TextValue  *string `bun:"text_value"`
	}
	if err := db.NewSelect().TableExpr("users.survey_responses AS sr").
		Join("JOIN users.surveys AS sv ON sv.id = sr.survey_id AND NOT sv.anonymous").
		Join("LEFT JOIN users.survey_answers AS a ON a.response_id = sr.id").
		ColumnExpr("sr.id AS response_id, sr.account_id, a.question_id, a.option_id, a.scale_value, a.text_value").
		Where("sr.tenant_id = ? AND sr.survey_id = ? AND sr.account_id IS NOT NULL", tenantID, surveyID).
		OrderExpr("sr.account_id, a.question_id, a.option_id").
		Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("list named survey responses: %w", err)
	}
	var result []communication.SurveyNamedResponse
	index := map[string]int{}
	for _, r := range rows {
		i, ok := index[r.ResponseID]
		if !ok {
			i = len(result)
			index[r.ResponseID] = i
			result = append(result, communication.SurveyNamedResponse{AccountID: r.AccountID})
		}
		if r.QuestionID == nil {
			continue
		}
		result[i].Answers = appendNamedAnswer(result[i].Answers, *r.QuestionID, r.OptionID, r.ScaleValue, r.TextValue)
	}
	return result, nil
}

func appendNamedAnswer(answers []communication.SurveyAnswerInput, questionID int64, optionID *int64, scale *int, text *string) []communication.SurveyAnswerInput {
	if optionID != nil {
		for i := range answers {
			if answers[i].QuestionID == questionID {
				answers[i].OptionIDs = append(answers[i].OptionIDs, *optionID)
				return answers
			}
		}
		return append(answers, communication.SurveyAnswerInput{QuestionID: questionID, OptionIDs: []int64{*optionID}})
	}
	return append(answers, communication.SurveyAnswerInput{QuestionID: questionID, ScaleValue: scale, Text: text})
}
