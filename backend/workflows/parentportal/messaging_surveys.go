package parentportal

import "github.com/moto-nrw/project-phoenix/workflows/parentportal/internal/messaging"

// Parent survey values and port (#4781). The root binds the port to
// Communication's parent surveys.
type (
	Surveys        = messaging.Surveys
	SurveySummary  = messaging.SurveySummary
	SurveyForm     = messaging.SurveyForm
	SurveyQuestion = messaging.SurveyQuestion
	SurveyOption   = messaging.SurveyOption
	SurveyAnswer   = messaging.SurveyAnswer
)

var (
	ErrSurveyNotOpen         = messaging.ErrSurveyNotOpen
	ErrSurveyAlreadyAnswered = messaging.ErrSurveyAlreadyAnswered
	ErrInvalidSurveyAnswers  = messaging.ErrInvalidSurveyAnswers
)
