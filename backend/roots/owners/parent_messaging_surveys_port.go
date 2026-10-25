package owners

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/communication"
	parentportal "github.com/moto-nrw/project-phoenix/workflows/parentportal"
)

// BindParentSurveys binds the parent portal's survey port to Communication's
// parent surveys (#4781) and maps Communication's survey errors onto the
// workflow's.
func BindParentSurveys(source communication.PortalSurveys) parentportal.Surveys {
	if source == nil {
		return nil
	}
	return parentSurveys{source: source}
}

type parentSurveys struct {
	source communication.PortalSurveys
}

func (p parentSurveys) SurveySummaries(ctx context.Context, accountID int64, announcementIDs []int64) (map[int64]parentportal.SurveySummary, error) {
	rows, err := p.source.PortalSurveySummaries(ctx, accountID, announcementIDs)
	if err != nil {
		return nil, parentSurveyError(err)
	}
	out := make(map[int64]parentportal.SurveySummary, len(rows))
	for id, row := range rows {
		out[id] = parentportal.SurveySummary{SurveyID: row.SurveyID, Anonymous: row.Anonymous, ClosesAt: row.ClosesAt, Participated: row.Participated}
	}
	return out, nil
}

func (p parentSurveys) SurveyForm(ctx context.Context, tenantID, announcementID, accountID int64) (*parentportal.SurveyForm, error) {
	form, err := p.source.PortalSurveyForm(ctx, tenantID, announcementID, accountID)
	if err != nil {
		return nil, parentSurveyError(err)
	}
	questions := make([]parentportal.SurveyQuestion, 0, len(form.Survey.Questions))
	for _, q := range form.Survey.Questions {
		options := make([]parentportal.SurveyOption, 0, len(q.Options))
		for _, o := range q.Options {
			options = append(options, parentportal.SurveyOption{ID: o.ID, Label: o.Label})
		}
		questions = append(questions, parentportal.SurveyQuestion{
			ID: q.ID, Position: q.Position, Kind: q.Kind, Prompt: q.Prompt, Required: q.Required, Options: options,
		})
	}
	return &parentportal.SurveyForm{
		SurveyID: form.Survey.ID, Title: form.Survey.Title, Intro: form.Survey.Intro, Anonymous: form.Survey.Anonymous,
		ClosesAt: form.Survey.ClosesAt, Questions: questions, SchoolName: form.SchoolName, Participated: form.Participated,
	}, nil
}

func (p parentSurveys) SubmitSurvey(ctx context.Context, tenantID, announcementID, accountID int64, answers []parentportal.SurveyAnswer) error {
	in := make([]communication.SurveyAnswerInput, 0, len(answers))
	for _, a := range answers {
		in = append(in, communication.SurveyAnswerInput{QuestionID: a.QuestionID, OptionIDs: a.OptionIDs, ScaleValue: a.ScaleValue, Text: a.Text})
	}
	return parentSurveyError(p.source.SubmitPortalSurvey(ctx, tenantID, announcementID, accountID, in))
}

func parentSurveyError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, communication.ErrSurveyNotFound):
		return parentportal.ErrAnnouncementNotFound
	case errors.Is(err, communication.ErrSurveyNotOpen):
		return parentportal.ErrSurveyNotOpen
	case errors.Is(err, communication.ErrSurveyAlreadyAnswered):
		return parentportal.ErrSurveyAlreadyAnswered
	case errors.Is(err, communication.ErrSurveyInvalid):
		return errors.Join(parentportal.ErrInvalidSurveyAnswers, err)
	default:
		return err
	}
}
