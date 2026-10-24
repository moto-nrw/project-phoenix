package announcement

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	announcementService "github.com/moto-nrw/project-phoenix/modules/communication"
)

// Wire shapes of the surveys (#4781). Ids travel as strings.

type surveyTargetWire struct {
	TargetType string  `json:"target_type"`
	RefID      *string `json:"ref_id,omitempty"`
	RefText    *string `json:"ref_text,omitempty"`
}

type surveyQuestionRequest struct {
	Kind     string   `json:"kind"`
	Prompt   string   `json:"prompt"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}

type surveyRequest struct {
	Title     string                  `json:"title"`
	Intro     string                  `json:"intro"`
	Audience  string                  `json:"audience"`
	Anonymous bool                    `json:"anonymous"`
	SendEmail bool                    `json:"send_email"`
	ClosesAt  *time.Time              `json:"closes_at"`
	Targets   []surveyTargetWire      `json:"targets"`
	Questions []surveyQuestionRequest `json:"questions"`
}

func (req surveyRequest) input() (announcementService.SurveyInput, error) {
	if req.ClosesAt == nil {
		return announcementService.SurveyInput{}, errors.New("closes_at is required")
	}
	in := announcementService.SurveyInput{
		Title: req.Title, Intro: req.Intro, Audience: req.Audience, Anonymous: req.Anonymous, SendEmail: req.SendEmail,
		ClosesAt: *req.ClosesAt, Targets: make([]announcementService.SurveyTarget, 0, len(req.Targets)),
		Questions: make([]announcementService.SurveyQuestionInput, 0, len(req.Questions)),
	}
	for _, t := range req.Targets {
		target := announcementService.SurveyTarget{TargetType: t.TargetType, RefText: t.RefText}
		if t.RefID != nil && *t.RefID != "" {
			id, err := strconv.ParseInt(*t.RefID, 10, 64)
			if err != nil || id <= 0 {
				return announcementService.SurveyInput{}, errors.New("invalid target ref_id")
			}
			target.RefID = &id
		}
		in.Targets = append(in.Targets, target)
	}
	for _, q := range req.Questions {
		in.Questions = append(in.Questions, announcementService.SurveyQuestionInput{Kind: q.Kind, Prompt: q.Prompt, Required: q.Required, Options: q.Options})
	}
	return in, nil
}

type surveyOptionResponse struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type surveyQuestionResponse struct {
	ID       string                 `json:"id"`
	Position int                    `json:"position"`
	Kind     string                 `json:"kind"`
	Prompt   string                 `json:"prompt"`
	Required bool                   `json:"required"`
	Options  []surveyOptionResponse `json:"options"`
}

type surveyResponse struct {
	ID               string                   `json:"id"`
	Title            string                   `json:"title"`
	Intro            string                   `json:"intro"`
	Audience         string                   `json:"audience"`
	Anonymous        bool                     `json:"anonymous"`
	SendEmail        bool                     `json:"send_email"`
	Status           string                   `json:"status"`
	ClosesAt         time.Time                `json:"closes_at"`
	PublishedAt      *time.Time               `json:"published_at,omitempty"`
	CreatedAt        time.Time                `json:"created_at"`
	UpdatedAt        time.Time                `json:"updated_at"`
	ParticipantCount int                      `json:"participant_count"`
	Targets          []surveyTargetWire       `json:"targets"`
	Questions        []surveyQuestionResponse `json:"questions"`
}

func toSurveyQuestions(questions []announcementService.SurveyQuestion) []surveyQuestionResponse {
	out := make([]surveyQuestionResponse, 0, len(questions))
	for _, q := range questions {
		options := make([]surveyOptionResponse, 0, len(q.Options))
		for _, o := range q.Options {
			options = append(options, surveyOptionResponse{ID: strconv.FormatInt(o.ID, 10), Label: o.Label})
		}
		out = append(out, surveyQuestionResponse{
			ID: strconv.FormatInt(q.ID, 10), Position: q.Position, Kind: q.Kind, Prompt: q.Prompt, Required: q.Required, Options: options,
		})
	}
	return out
}

func toSurveyResponse(s *announcementService.Survey) surveyResponse {
	targets := make([]surveyTargetWire, 0, len(s.Targets))
	for _, t := range s.Targets {
		target := surveyTargetWire{TargetType: t.TargetType, RefText: t.RefText}
		if t.RefID != nil {
			id := strconv.FormatInt(*t.RefID, 10)
			target.RefID = &id
		}
		targets = append(targets, target)
	}
	return surveyResponse{
		ID: strconv.FormatInt(s.ID, 10), Title: s.Title, Intro: s.Intro, Audience: s.Audience, Anonymous: s.Anonymous,
		SendEmail: s.SendEmail, Status: s.Status(time.Now()), ClosesAt: s.ClosesAt, PublishedAt: s.PublishedAt,
		CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt, ParticipantCount: s.ParticipantCount,
		Targets: targets, Questions: toSurveyQuestions(s.Questions),
	}
}

type surveyFormSurvey struct {
	ID        string                   `json:"id"`
	Title     string                   `json:"title"`
	Intro     string                   `json:"intro"`
	Anonymous bool                     `json:"anonymous"`
	ClosesAt  time.Time                `json:"closes_at"`
	Questions []surveyQuestionResponse `json:"questions"`
}

type surveyFormResponse struct {
	Survey       surveyFormSurvey `json:"survey"`
	SchoolName   string           `json:"school_name"`
	Participated bool             `json:"participated"`
}

func toSurveyFormResponse(f *announcementService.SurveyForm) surveyFormResponse {
	return surveyFormResponse{
		Survey: surveyFormSurvey{
			ID: strconv.FormatInt(f.Survey.ID, 10), Title: f.Survey.Title, Intro: f.Survey.Intro,
			Anonymous: f.Survey.Anonymous, ClosesAt: f.Survey.ClosesAt, Questions: toSurveyQuestions(f.Survey.Questions),
		},
		SchoolName: f.SchoolName, Participated: f.Participated,
	}
}

type surveyToAnswerResponse struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Intro        string    `json:"intro"`
	Anonymous    bool      `json:"anonymous"`
	ClosesAt     time.Time `json:"closes_at"`
	Participated bool      `json:"participated"`
}

type surveyAnswerRequest struct {
	QuestionID string   `json:"question_id"`
	OptionIDs  []string `json:"option_ids,omitempty"`
	ScaleValue *int     `json:"scale_value,omitempty"`
	Text       *string  `json:"text,omitempty"`
}

type surveyAnswersRequest struct {
	Answers []surveyAnswerRequest `json:"answers"`
}

// decodeSurveyAnswers reads the answers of a submission; the service checks
// them against the questions.
func decodeSurveyAnswers(r *http.Request) ([]announcementService.SurveyAnswerInput, error) {
	var req surveyAnswersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return nil, errors.New("invalid request body")
	}
	out := make([]announcementService.SurveyAnswerInput, 0, len(req.Answers))
	for _, a := range req.Answers {
		questionID, err := strconv.ParseInt(a.QuestionID, 10, 64)
		if err != nil || questionID <= 0 {
			return nil, errors.New("invalid question_id")
		}
		answer := announcementService.SurveyAnswerInput{QuestionID: questionID, ScaleValue: a.ScaleValue, Text: a.Text}
		for _, raw := range a.OptionIDs {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return nil, errors.New("invalid option id")
			}
			answer.OptionIDs = append(answer.OptionIDs, id)
		}
		out = append(out, answer)
	}
	return out, nil
}

type surveyOptionCountResponse struct {
	OptionID string `json:"option_id"`
	Label    string `json:"label"`
	Count    int    `json:"count"`
}

type surveyQuestionResultResponse struct {
	QuestionID  string                      `json:"question_id"`
	Position    int                         `json:"position"`
	Kind        string                      `json:"kind"`
	Prompt      string                      `json:"prompt"`
	AnswerCount int                         `json:"answer_count"`
	Options     []surveyOptionCountResponse `json:"options"`
	ScaleCounts []int                       `json:"scale_counts"`
	Average     *float64                    `json:"average"`
	Texts       []string                    `json:"texts"`
	Highest     []string                    `json:"highest"`
	Lowest      []string                    `json:"lowest"`
}

type surveyHighlightResponse struct {
	QuestionID string  `json:"question_id"`
	Prompt     string  `json:"prompt"`
	Average    float64 `json:"average"`
}

type surveyResultsResponse struct {
	SurveyID      string                         `json:"survey_id"`
	Title         string                         `json:"title"`
	Anonymous     bool                           `json:"anonymous"`
	Status        string                         `json:"status"`
	ResponseCount int                            `json:"response_count"`
	MinResponses  int                            `json:"min_responses"`
	Available     bool                           `json:"available"`
	Questions     []surveyQuestionResultResponse `json:"questions"`
	Strengths     []surveyHighlightResponse      `json:"strengths"`
	Improvements  []surveyHighlightResponse      `json:"improvements"`
}

func toSurveyResultsResponse(r *announcementService.SurveyResults) surveyResultsResponse {
	out := surveyResultsResponse{
		SurveyID: strconv.FormatInt(r.SurveyID, 10), Title: r.Title, Anonymous: r.Anonymous, Status: r.Status,
		ResponseCount: r.ResponseCount, MinResponses: r.MinResponses, Available: r.Available,
		Questions:    make([]surveyQuestionResultResponse, 0, len(r.Questions)),
		Strengths:    toHighlights(r.Strengths),
		Improvements: toHighlights(r.Improvements),
	}
	for _, q := range r.Questions {
		options := make([]surveyOptionCountResponse, 0, len(q.Options))
		for _, o := range q.Options {
			options = append(options, surveyOptionCountResponse{OptionID: strconv.FormatInt(o.OptionID, 10), Label: o.Label, Count: o.Count})
		}
		out.Questions = append(out.Questions, surveyQuestionResultResponse{
			QuestionID: strconv.FormatInt(q.QuestionID, 10), Position: q.Position, Kind: q.Kind, Prompt: q.Prompt,
			AnswerCount: q.AnswerCount, Options: options, ScaleCounts: q.ScaleCounts, Average: q.Average,
			Texts: q.Texts, Highest: q.Highest, Lowest: q.Lowest,
		})
	}
	return out
}

func toHighlights(rows []announcementService.SurveyHighlight) []surveyHighlightResponse {
	out := make([]surveyHighlightResponse, 0, len(rows))
	for _, h := range rows {
		out = append(out, surveyHighlightResponse{QuestionID: strconv.FormatInt(h.QuestionID, 10), Prompt: h.Prompt, Average: h.Average})
	}
	return out
}

type surveyRespondentAnswerResponse struct {
	QuestionID string   `json:"question_id"`
	Options    []string `json:"options"`
	ScaleValue *int     `json:"scale_value,omitempty"`
	Text       *string  `json:"text,omitempty"`
}

type surveyRespondentResponse struct {
	Name    string                           `json:"name"`
	Answers []surveyRespondentAnswerResponse `json:"answers"`
}

func toRespondentResponses(rows []announcementService.SurveyRespondent) []surveyRespondentResponse {
	out := make([]surveyRespondentResponse, 0, len(rows))
	for _, row := range rows {
		answers := make([]surveyRespondentAnswerResponse, 0, len(row.Answers))
		for _, a := range row.Answers {
			answers = append(answers, surveyRespondentAnswerResponse{
				QuestionID: strconv.FormatInt(a.QuestionID, 10), Options: a.Options, ScaleValue: a.ScaleValue, Text: a.Text,
			})
		}
		out = append(out, surveyRespondentResponse{Name: row.Name, Answers: answers})
	}
	return out
}
