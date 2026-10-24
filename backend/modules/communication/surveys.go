package communication

import (
	"context"
	"errors"
	"time"
)

// Surveys (Umfragen, #4781) ask the guardians of a school or its staff a
// short questionnaire, optionally anonymous.
//
// Protection goal of an anonymous survey: the school cannot attribute an
// answer to a person through the app, its exports or its logs. Participation
// ("this account took part") and the answers are stored apart without a
// shared key, and the evaluation shows sums and distributions only, from
// SurveyMinAnonymousResponses answers on.

// Survey audiences.
const (
	SurveyAudienceParents = "parents"
	SurveyAudienceStaff   = "staff"
)

// Question kinds.
const (
	SurveyQuestionSingle   = "single"
	SurveyQuestionMultiple = "multiple"
	SurveyQuestionScale    = "scale"
	SurveyQuestionText     = "text"
)

// Survey states, derived from publication and closing time.
const (
	SurveyStatusDraft  = "draft"
	SurveyStatusOpen   = "open"
	SurveyStatusClosed = "closed"
)

const (
	// SurveyMinAnonymousResponses is the number of answers an anonymous survey
	// needs before its evaluation shows anything. Fewer answers would let a
	// school read single persons out of the distribution.
	SurveyMinAnonymousResponses = 5
	// SurveyScaleMin and SurveyScaleMax bound a scale answer (1 to 5).
	SurveyScaleMin = 1
	SurveyScaleMax = 5
)

var (
	// ErrSurveyNotFound: no survey with this id in the school, or not one the
	// caller may answer.
	ErrSurveyNotFound = errors.New("survey: not found")
	// ErrSurveyInvalid: the survey input or the answers break a rule.
	ErrSurveyInvalid = errors.New("survey: invalid")
	// ErrSurveyNotDraft: the change needs a draft, the survey is published.
	ErrSurveyNotDraft = errors.New("survey: already published")
	// ErrSurveyNotOpen: the survey does not collect answers (draft or closed).
	ErrSurveyNotOpen = errors.New("survey: not open")
	// ErrSurveyAlreadyAnswered: this account has taken part already.
	ErrSurveyAlreadyAnswered = errors.New("survey: already answered")
	// ErrSurveyAnonymous: named answers were requested for an anonymous survey.
	ErrSurveyAnonymous = errors.New("survey: anonymous")
)

// SurveyTarget is one audience target of a parent survey; the shape and the
// rules are the ones of a parent announcement target.
type SurveyTarget struct {
	TargetType string  `json:"target_type"`
	RefID      *int64  `json:"ref_id,omitempty"`
	RefText    *string `json:"ref_text,omitempty"`
}

// SurveyQuestionInput is one question as the author writes it.
type SurveyQuestionInput struct {
	Kind     string
	Prompt   string
	Required bool
	Options  []string
}

// SurveyInput is a survey as the author writes it.
type SurveyInput struct {
	Title     string
	Intro     string
	Audience  string
	Anonymous bool
	SendEmail bool
	ClosesAt  time.Time
	Targets   []SurveyTarget
	Questions []SurveyQuestionInput
}

// SurveyOption is one answer choice of a choice question.
type SurveyOption struct {
	ID    int64
	Label string
}

// SurveyQuestion is one stored question.
type SurveyQuestion struct {
	ID       int64
	Position int
	Kind     string
	Prompt   string
	Required bool
	Options  []SurveyOption
}

// Survey is one stored survey with its questions.
type Survey struct {
	ID          int64
	TenantID    int64
	Title       string
	Intro       string
	Audience    string
	Anonymous   bool
	SendEmail   bool
	Targets     []SurveyTarget
	ClosesAt    time.Time
	PublishedAt *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Questions   []SurveyQuestion
	// ParticipantCount is how many accounts took part.
	ParticipantCount int
	// AnnouncementID is the system announcement that carries a published
	// parent survey to the families; nil for staff surveys and drafts.
	AnnouncementID *int64
}

// Status derives draft, open or closed at the given instant.
func (s Survey) Status(now time.Time) string {
	if s.PublishedAt == nil {
		return SurveyStatusDraft
	}
	if !s.ClosesAt.After(now) {
		return SurveyStatusClosed
	}
	return SurveyStatusOpen
}

// SurveyAnswerInput is the answer to one question: option ids for a choice
// question, a value from 1 to 5 for a scale, a text for a free-text question.
// An optional question may be left out.
type SurveyAnswerInput struct {
	QuestionID int64
	OptionIDs  []int64
	ScaleValue *int
	Text       *string
}

// SurveyOptionCount is how often one option was chosen.
type SurveyOptionCount struct {
	OptionID int64
	Label    string
	Count    int
}

// SurveyQuestionResult is the evaluation of one question. Highest and Lowest
// name the most and least chosen options (choice) or the most and least
// given values (scale); ties name all of them.
type SurveyQuestionResult struct {
	QuestionID  int64
	Position    int
	Kind        string
	Prompt      string
	AnswerCount int
	Options     []SurveyOptionCount
	// ScaleCounts[i] counts the value i+1.
	ScaleCounts []int
	Average     *float64
	// Texts are the free-text answers without time or author, in random order.
	Texts   []string
	Highest []string
	Lowest  []string
}

// SurveyHighlight is one scale question in the overall strengths or
// improvement list.
type SurveyHighlight struct {
	QuestionID int64
	Prompt     string
	Average    float64
}

// SurveyResults is the evaluation of a survey. Available is false while an
// anonymous survey has fewer than MinResponses answers; then nothing but the
// counts is filled.
type SurveyResults struct {
	SurveyID      int64
	Title         string
	Anonymous     bool
	Status        string
	ResponseCount int
	MinResponses  int
	Available     bool
	Questions     []SurveyQuestionResult
	// Strengths and Improvements are the best and worst rated scale questions.
	Strengths    []SurveyHighlight
	Improvements []SurveyHighlight
}

// SurveyRespondentAnswer is one answer of a named respondent.
type SurveyRespondentAnswer struct {
	QuestionID int64
	Options    []string
	ScaleValue *int
	Text       *string
}

// SurveyRespondent is one person's answers to a named (not anonymous) survey.
type SurveyRespondent struct {
	Name    string
	Answers []SurveyRespondentAnswer
}

// SurveyForm is what a person sees before answering.
type SurveyForm struct {
	Survey       Survey
	SchoolName   string
	Participated bool
}

// SurveyToAnswer is one open staff survey in a staff member's list.
type SurveyToAnswer struct {
	ID           int64
	Title        string
	Intro        string
	Anonymous    bool
	ClosesAt     time.Time
	Participated bool
}

// PortalSurveySummary is what the parent feed shows on a survey entry.
type PortalSurveySummary struct {
	SurveyID     int64
	Anonymous    bool
	ClosesAt     time.Time
	Participated bool
}

// SurveyTally is what the persistence counts for an evaluation. It holds no
// response id, account or time: an evaluation cannot leak what it never
// receives.
type SurveyTally struct {
	ResponseCount int
	// AnswerCounts counts the responses that answered a question.
	AnswerCounts map[int64]int
	// OptionCounts counts how often an option was chosen.
	OptionCounts map[int64]int
	// ScaleCounts[question][value-1] counts a scale value.
	ScaleCounts map[int64][]int
	// Texts are the free-text answers per question, in random order.
	Texts map[int64][]string
}

// SurveyNamedResponse is one response of a named survey: the account and its
// answers, as stored.
type SurveyNamedResponse struct {
	AccountID int64
	Answers   []SurveyAnswerInput
}

// SurveyManagement authors and evaluates the school's surveys. It runs in the
// request's school transaction behind the surveys:manage permission.
type SurveyManagement interface {
	ListSurveys(ctx context.Context) ([]Survey, error)
	GetSurvey(ctx context.Context, id int64) (*Survey, error)
	CreateSurvey(ctx context.Context, createdBy int64, in SurveyInput) (*Survey, error)
	UpdateSurvey(ctx context.Context, id int64, in SurveyInput) (*Survey, error)
	DeleteSurvey(ctx context.Context, id int64) error
	PublishSurvey(ctx context.Context, publishedBy, id int64) (*Survey, error)
	CloseSurvey(ctx context.Context, id int64) (*Survey, error)
	// RemindSurvey notifies the audience members who have not taken part and
	// returns how many were reached.
	RemindSurvey(ctx context.Context, id int64) (int, error)
	SurveyResults(ctx context.Context, id int64) (*SurveyResults, error)
	// SurveyRespondents lists the answers per person of a named survey. An
	// anonymous survey fails with ErrSurveyAnonymous.
	SurveyRespondents(ctx context.Context, id int64) ([]SurveyRespondent, error)
}

// SurveyParticipation lets a staff member answer the staff surveys of the
// request's school.
type SurveyParticipation interface {
	StaffSurveysToAnswer(ctx context.Context, accountID int64) ([]SurveyToAnswer, error)
	StaffSurveyForm(ctx context.Context, accountID, surveyID int64) (*SurveyForm, error)
	SubmitStaffSurvey(ctx context.Context, accountID, surveyID int64, answers []SurveyAnswerInput) error
}

// PortalSurveys lets a guardian answer a parent survey, addressed by its
// carrier announcement. It runs in the parent portal's administrative
// transaction; the workflow has checked the announcement audience.
type PortalSurveys interface {
	PortalSurveySummaries(ctx context.Context, accountID int64, announcementIDs []int64) (map[int64]PortalSurveySummary, error)
	PortalSurveyForm(ctx context.Context, tenantID, announcementID, accountID int64) (*SurveyForm, error)
	SubmitPortalSurvey(ctx context.Context, tenantID, announcementID, accountID int64, answers []SurveyAnswerInput) error
}

// SurveyRetention deletes surveys the school's window has passed: parent
// surveys after the parent message window, staff surveys after the staff
// message window, counted from closing (a draft from its last change).
type SurveyRetention interface {
	DeleteExpiredParentSurveys(ctx context.Context, retentionDays int) (int, error)
	DeleteExpiredStaffSurveys(ctx context.Context, retentionDays int) (int, error)
}

// SurveyCapability is the whole survey surface.
type SurveyCapability interface {
	SurveyManagement
	SurveyParticipation
	PortalSurveys
	SurveyRetention
}
