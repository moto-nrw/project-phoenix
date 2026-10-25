package application

import (
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/communication"
)

// Limits of a survey (#4781). A questionnaire is meant to take minutes, not
// to replace an evaluation system.
const (
	maxSurveyQuestions      = 30
	minSurveyOptions        = 2
	maxSurveyOptions        = 10
	maxSurveyPromptLen      = 300
	maxSurveyOptionLen      = 120
	maxSurveyTextAnswerLen  = 2000
	maxSurveyOpenDuration   = 366 * 24 * time.Hour
	surveyDefaultParentBody = "Die Schule bittet Sie, an einer kurzen Umfrage teilzunehmen."
)

func surveyInvalid(reason string) error {
	return fmt.Errorf("%w: %s", communication.ErrSurveyInvalid, reason)
}

// normalizeSurveyInput trims and checks a survey as the author wrote it and
// returns the stored form.
func normalizeSurveyInput(in communication.SurveyInput, now time.Time) (communication.SurveyInput, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.Intro = strings.TrimSpace(in.Intro)
	if in.Title == "" || len([]rune(in.Title)) > maxTitleLen {
		return in, surveyInvalid("title is required and short")
	}
	if len([]rune(in.Intro)) > maxBodyLen {
		return in, surveyInvalid("intro too long")
	}
	if !in.ClosesAt.After(now) {
		return in, surveyInvalid("closes_at must be in the future")
	}
	if in.ClosesAt.After(now.Add(maxSurveyOpenDuration)) {
		return in, surveyInvalid("closes_at must be within one year")
	}
	targets, err := normalizeSurveyAudience(in)
	if err != nil {
		return in, err
	}
	in.Targets = targets
	if in.Audience == communication.SurveyAudienceStaff {
		in.SendEmail = false
	}
	questions, err := normalizeSurveyQuestions(in.Questions)
	if err != nil {
		return in, err
	}
	in.Questions = questions
	return in, nil
}

func normalizeSurveyAudience(in communication.SurveyInput) ([]communication.SurveyTarget, error) {
	switch in.Audience {
	case communication.SurveyAudienceStaff:
		if len(in.Targets) > 0 {
			return nil, surveyInvalid("a staff survey has no targets")
		}
		return []communication.SurveyTarget{}, nil
	case communication.SurveyAudienceParents:
		return normalizeSurveyTargets(in.Targets)
	default:
		return nil, surveyInvalid("audience must be parents or staff")
	}
}

// normalizeSurveyTargets applies the parent announcement target rules: the
// carrier announcement is published with exactly these targets.
func normalizeSurveyTargets(targets []communication.SurveyTarget) ([]communication.SurveyTarget, error) {
	if len(targets) == 0 {
		return nil, surveyInvalid("a parent survey needs at least one target")
	}
	seen := make(map[string]bool, len(targets))
	out := make([]communication.SurveyTarget, 0, len(targets))
	for _, t := range targets {
		if t.TargetType == AnnouncementTargetPendingEnrollment {
			return nil, surveyInvalid("pending_enrollment cannot be targeted by a survey")
		}
		target, err := normalizeAnnouncementTarget(TargetInput{TargetType: t.TargetType, RefID: t.RefID, RefText: t.RefText}, StaffParentAnnouncementResponseNone)
		if err != nil {
			return nil, surveyInvalid(err.Error())
		}
		key := dedupeKey(target)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, communication.SurveyTarget{TargetType: target.TargetType, RefID: target.TargetRefID, RefText: target.TargetRefText})
	}
	return out, nil
}

func normalizeSurveyQuestions(questions []communication.SurveyQuestionInput) ([]communication.SurveyQuestionInput, error) {
	if len(questions) == 0 {
		return nil, surveyInvalid("a survey needs at least one question")
	}
	if len(questions) > maxSurveyQuestions {
		return nil, surveyInvalid(fmt.Sprintf("a survey allows at most %d questions", maxSurveyQuestions))
	}
	out := make([]communication.SurveyQuestionInput, 0, len(questions))
	for _, q := range questions {
		normalized, err := normalizeSurveyQuestion(q)
		if err != nil {
			return nil, err
		}
		out = append(out, normalized)
	}
	return out, nil
}

func normalizeSurveyQuestion(q communication.SurveyQuestionInput) (communication.SurveyQuestionInput, error) {
	q.Prompt = strings.TrimSpace(q.Prompt)
	if q.Prompt == "" || len([]rune(q.Prompt)) > maxSurveyPromptLen {
		return q, surveyInvalid("question prompt is required and short")
	}
	switch q.Kind {
	case communication.SurveyQuestionSingle, communication.SurveyQuestionMultiple:
		options, err := normalizeSurveyOptions(q.Options)
		if err != nil {
			return q, err
		}
		q.Options = options
	case communication.SurveyQuestionScale, communication.SurveyQuestionText:
		if len(q.Options) > 0 {
			return q, surveyInvalid("only choice questions have options")
		}
		q.Options = nil
	default:
		return q, surveyInvalid("unknown question kind")
	}
	return q, nil
}

func normalizeSurveyOptions(options []string) ([]string, error) {
	seen := make(map[string]bool, len(options))
	out := make([]string, 0, len(options))
	for _, raw := range options {
		label := strings.TrimSpace(raw)
		if label == "" {
			continue
		}
		if len([]rune(label)) > maxSurveyOptionLen {
			return nil, surveyInvalid("option label too long")
		}
		key := strings.ToLower(label)
		if seen[key] {
			return nil, surveyInvalid("duplicate option")
		}
		seen[key] = true
		out = append(out, label)
	}
	if len(out) < minSurveyOptions || len(out) > maxSurveyOptions {
		return nil, surveyInvalid(fmt.Sprintf("a choice question needs %d to %d options", minSurveyOptions, maxSurveyOptions))
	}
	return out, nil
}

// normalizeSurveyAnswers checks a submission against the stored questions
// and returns one clean answer per answered question. Every required
// question must be answered; unknown questions, foreign options and a second
// answer to the same question are refused.
func normalizeSurveyAnswers(survey *communication.Survey, answers []communication.SurveyAnswerInput) ([]communication.SurveyAnswerInput, error) {
	given, err := surveyAnswersByQuestion(survey, answers)
	if err != nil {
		return nil, err
	}
	out := make([]communication.SurveyAnswerInput, 0, len(given))
	for _, q := range survey.Questions {
		a, ok := given[q.ID]
		if !ok {
			if q.Required {
				return nil, surveyInvalid("a required question is unanswered")
			}
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

// surveyAnswersByQuestion checks every given answer against its question and
// keeps the answered ones.
func surveyAnswersByQuestion(survey *communication.Survey, answers []communication.SurveyAnswerInput) (map[int64]communication.SurveyAnswerInput, error) {
	byID := make(map[int64]communication.SurveyQuestion, len(survey.Questions))
	for _, q := range survey.Questions {
		byID[q.ID] = q
	}
	given := make(map[int64]communication.SurveyAnswerInput, len(answers))
	seen := make(map[int64]bool, len(answers))
	for _, a := range answers {
		q, ok := byID[a.QuestionID]
		if !ok {
			return nil, surveyInvalid("answer to an unknown question")
		}
		if seen[a.QuestionID] {
			return nil, surveyInvalid("two answers to one question")
		}
		seen[a.QuestionID] = true
		normalized, answered, err := normalizeSurveyAnswer(q, a)
		if err != nil {
			return nil, err
		}
		if answered {
			given[a.QuestionID] = normalized
		}
	}
	return given, nil
}

// normalizeSurveyAnswer checks one answer; answered is false for an empty
// answer, which only an optional question may have.
func normalizeSurveyAnswer(q communication.SurveyQuestion, a communication.SurveyAnswerInput) (communication.SurveyAnswerInput, bool, error) {
	clean := communication.SurveyAnswerInput{QuestionID: q.ID}
	switch q.Kind {
	case communication.SurveyQuestionSingle, communication.SurveyQuestionMultiple:
		if a.ScaleValue != nil || a.Text != nil {
			return clean, false, surveyInvalid("a choice question takes options")
		}
		ids, err := surveyOptionSelection(q, a.OptionIDs)
		clean.OptionIDs = ids
		return clean, len(ids) > 0, err
	case communication.SurveyQuestionScale:
		if len(a.OptionIDs) > 0 || a.Text != nil {
			return clean, false, surveyInvalid("a scale question takes a value")
		}
		if a.ScaleValue == nil {
			return clean, false, nil
		}
		if *a.ScaleValue < communication.SurveyScaleMin || *a.ScaleValue > communication.SurveyScaleMax {
			return clean, false, surveyInvalid("scale value out of range")
		}
		value := *a.ScaleValue
		clean.ScaleValue = &value
		return clean, true, nil
	default:
		return normalizeSurveyTextAnswer(clean, a)
	}
}

func normalizeSurveyTextAnswer(clean, a communication.SurveyAnswerInput) (communication.SurveyAnswerInput, bool, error) {
	if len(a.OptionIDs) > 0 || a.ScaleValue != nil {
		return clean, false, surveyInvalid("a text question takes a text")
	}
	if a.Text == nil {
		return clean, false, nil
	}
	text := strings.TrimSpace(*a.Text)
	if text == "" {
		return clean, false, nil
	}
	if len([]rune(text)) > maxSurveyTextAnswerLen {
		return clean, false, surveyInvalid("text answer too long")
	}
	clean.Text = &text
	return clean, true, nil
}

func surveyOptionSelection(q communication.SurveyQuestion, selected []int64) ([]int64, error) {
	valid := make(map[int64]bool, len(q.Options))
	for _, o := range q.Options {
		valid[o.ID] = true
	}
	seen := make(map[int64]bool, len(selected))
	out := make([]int64, 0, len(selected))
	for _, id := range selected {
		if !valid[id] {
			return nil, surveyInvalid("option of another question")
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if q.Kind == communication.SurveyQuestionSingle && len(out) > 1 {
		return nil, surveyInvalid("a single choice question takes one option")
	}
	return out, nil
}
