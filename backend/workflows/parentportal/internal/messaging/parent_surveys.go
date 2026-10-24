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
	var form *SurveyForm
	err := s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		a, err := s.surveyCarrier(adminCtx, announcementID, accountID)
		if err != nil {
			return err
		}
		form, err = s.Surveys.SurveyForm(adminCtx, a.GetTenantID(), announcementID, accountID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return form, nil
}

// SubmitAnnouncementSurvey stores the guardian's answers once per account.
func (s *Service) SubmitAnnouncementSurvey(ctx context.Context, accountID, announcementID int64, answers []SurveyAnswer) error {
	return s.Runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		a, err := s.surveyCarrier(adminCtx, announcementID, accountID)
		if err != nil {
			return err
		}
		return s.Surveys.SubmitSurvey(adminCtx, a.GetTenantID(), announcementID, accountID, answers)
	})
}

// surveyCarrier loads a live survey announcement the account belongs to the
// audience of. Every miss is the same 404.
func (s *Service) surveyCarrier(adminCtx context.Context, announcementID, accountID int64) (*ParentAnnouncement, error) {
	if accountID <= 0 || announcementID <= 0 || s.Surveys == nil {
		return nil, ErrAnnouncementNotFound
	}
	a, err := s.AnnouncementRepo.FindByID(adminCtx, announcementID)
	if err != nil {
		return nil, fmt.Errorf("parent: load announcement: %w", err)
	}
	if a == nil || !announcementIsLive(a) || !isSurveyCarrier(a.SystemKind) {
		return nil, ErrAnnouncementNotFound
	}
	matched, err := s.AnnouncementRepo.AccountMatchesAnnouncement(adminCtx, a.GetTenantID(), announcementID, accountID)
	if err != nil {
		return nil, fmt.Errorf("parent: match announcement audience: %w", err)
	}
	if !matched {
		return nil, ErrAnnouncementNotFound
	}
	return a, nil
}
