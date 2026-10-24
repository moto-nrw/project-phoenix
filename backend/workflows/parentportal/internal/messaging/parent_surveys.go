package messaging

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Parent surveys (#4781). A survey reaches the families as a system
// announcement (system kind "survey"): the feed entry, the audience and the
// school come from the announcement, the questions and answers from
// Communication's survey. The guardian answers once per account, not per
// child.

const announcementSystemKindSurvey = "survey"

var (
	// ErrSurveyNotOpen means the survey takes no (more) answers.
	ErrSurveyNotOpen = errors.New("parent: survey not open")
	// ErrSurveyAlreadyAnswered means this account has taken part already.
	ErrSurveyAlreadyAnswered = errors.New("parent: survey already answered")
	// ErrInvalidSurveyAnswers means the answers do not fit the questions.
	ErrInvalidSurveyAnswers = errors.New("parent: invalid survey answers")
)

// SurveySummary is what the feed shows on a survey entry.
type SurveySummary struct {
	SurveyID     int64
	Anonymous    bool
	ClosesAt     time.Time
	Participated bool
}

// SurveyOption is one answer choice.
type SurveyOption struct {
	ID    int64
	Label string
}

// SurveyQuestion is one question of the form.
type SurveyQuestion struct {
	ID       int64
	Position int
	Kind     string
	Prompt   string
	Required bool
	Options  []SurveyOption
}

// SurveyForm is the survey a guardian answers.
type SurveyForm struct {
	SurveyID     int64
	Title        string
	Intro        string
	Anonymous    bool
	ClosesAt     time.Time
	Questions    []SurveyQuestion
	SchoolName   string
	Participated bool
}

// SurveyAnswer is the answer to one question.
type SurveyAnswer struct {
	QuestionID int64
	OptionIDs  []int64
	ScaleValue *int
	Text       *string
}

// Surveys is the consumer-owned port to Communication's parent surveys. The
// root maps Communication's survey errors onto the workflow's.
type Surveys interface {
	SurveySummaries(ctx context.Context, accountID int64, announcementIDs []int64) (map[int64]SurveySummary, error)
	SurveyForm(ctx context.Context, tenantID, announcementID, accountID int64) (*SurveyForm, error)
	SubmitSurvey(ctx context.Context, tenantID, announcementID, accountID int64, answers []SurveyAnswer) error
}

func isSurveyCarrier(systemKind *string) bool {
	return systemKind != nil && *systemKind == announcementSystemKindSurvey
}

// attachSurveyData adds the survey state to the survey entries of a feed
// page in one batched read.
func (s *Service) attachSurveyData(ctx context.Context, accountID int64, items []*AnnouncementFeedItem) error {
	if s.Surveys == nil {
		return nil
	}
	ids := make([]int64, 0, len(items))
	for _, item := range items {
		if isSurveyCarrier(item.SystemKind) {
			ids = append(ids, item.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	summaries, err := s.Surveys.SurveySummaries(ctx, accountID, ids)
	if err != nil {
		return fmt.Errorf("parent: load survey state: %w", err)
	}
	for _, item := range items {
		if summary, ok := summaries[item.ID]; ok {
			summary := summary
			item.Survey = &summary
		}
	}
	return nil
}

// GetAnnouncementSurvey returns the survey an announcement of the guardian's
// feed carries. Outside the audience it is 404, like the announcement.
func (s *Service) GetAnnouncementSurvey(ctx context.Context, accountID, announcementID int64) (*SurveyForm, error) {
	tenantID, err := s.surveyTenant(ctx, accountID, announcementID)
	if err != nil {
		return nil, err
	}
	var form *SurveyForm
	err = s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		form, err = s.Surveys.SurveyForm(txCtx, tenantID, announcementID, accountID)
		return err
	})
	return form, err
}

// SubmitAnnouncementSurvey stores the guardian's answers once per account,
// in a transaction of the survey's school.
func (s *Service) SubmitAnnouncementSurvey(ctx context.Context, accountID, announcementID int64, answers []SurveyAnswer) error {
	tenantID, err := s.surveyTenant(ctx, accountID, announcementID)
	if err != nil {
		return err
	}
	return s.Runtime.WithinTenant(ctx, tenantID, func(txCtx context.Context) error {
		return s.Surveys.SubmitSurvey(txCtx, tenantID, announcementID, accountID, answers)
	})
}

// surveyTenant is the school of a live announcement the account belongs to
// the audience of: the same gate as the announcement's attachments. Every
// miss is the same 404; an announcement that carries no survey fails later
// with the same 404.
func (s *Service) surveyTenant(ctx context.Context, accountID, announcementID int64) (int64, error) {
	if s.Surveys == nil {
		return 0, ErrAnnouncementNotFound
	}
	tenantID, err := s.GuardianAnnouncementTenant(ctx, accountID, announcementID)
	if err != nil {
		return 0, fmt.Errorf("parent: resolve survey school: %w", err)
	}
	if tenantID == 0 {
		return 0, ErrAnnouncementNotFound
	}
	return tenantID, nil
}
