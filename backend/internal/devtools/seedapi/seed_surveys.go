package api

import (
	"context"
	"fmt"
	"time"
)

// seedSurveysStep writes the surveys (#4781): an anonymous staff survey six
// colleagues answered, so the evaluation passes the minimum of five, and an
// anonymous parent survey the demo parents answered in the parents portal.
// Every row comes through the real routes: the answers are anonymous only if
// the API stores them so.
type seedSurveysStep struct{}

func (seedSurveysStep) Name() string { return "Seeding surveys" }

// seedSurveyAnswerers is how many people answer each survey: one more than
// the minimum an anonymous evaluation needs.
const seedSurveyAnswerers = 6

func (seedSurveysStep) Run(ctx context.Context, rt *Runtime) error {
	rt.Client.BindAuth(rt.TenantAuth)
	defer rt.Client.BindAuth(rt.TenantAuth)
	staffSurvey, err := seedSurvey(rt, seedStaffSurveyBody())
	if err != nil {
		return err
	}
	staff, err := seedStaffSurveyAnswers(ctx, rt, staffSurvey)
	if err != nil {
		return err
	}
	parentSurvey, err := seedSurvey(rt, seedParentSurveyBody())
	if err != nil {
		return err
	}
	parents, err := seedParentSurveyAnswers(ctx, rt, parentSurvey)
	if err != nil {
		return err
	}
	fmt.Printf("  2 anonymous surveys published, answered by %d staff and %d parents\n", staff, parents)
	return nil
}

type seedSurveyQuestion struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Options []struct {
		ID string `json:"id"`
	} `json:"options"`
}

type seedSurveyResponse struct {
	Data struct {
		ID        string               `json:"id"`
		Questions []seedSurveyQuestion `json:"questions"`
	} `json:"data"`
}

func seedSurveyQuestions(staff bool) []map[string]any {
	if staff {
		return []map[string]any{
			{"kind": "scale", "prompt": "Ich fühle mich im Team gut aufgehoben.", "required": true},
			{"kind": "scale", "prompt": "Die Absprachen zu Beginn des Tages helfen mir.", "required": true},
			{"kind": "scale", "prompt": "Ich habe genug Zeit für Pausen.", "required": true},
			{"kind": "multiple", "prompt": "Was würde Ihnen den Alltag erleichtern?", "required": false, "options": []string{"Mehr Material", "Feste Rückzugsräume", "Kürzere Besprechungen", "Mehr Zeit für Absprachen"}},
			{"kind": "text", "prompt": "Was möchten Sie der Leitung sonst noch sagen?", "required": false},
		}
	}
	return []map[string]any{
		{"kind": "scale", "prompt": "Mein Kind geht gern in die OGS.", "required": true},
		{"kind": "scale", "prompt": "Ich fühle mich gut über den Tag meines Kindes informiert.", "required": true},
		{"kind": "single", "prompt": "Wie zufrieden sind Sie mit dem Mittagessen?", "required": true, "options": []string{"Sehr zufrieden", "Zufrieden", "Weniger zufrieden"}},
		{"kind": "text", "prompt": "Was wünschen Sie sich von der OGS?", "required": false},
	}
}

func seedStaffSurveyBody() map[string]any {
	return map[string]any{
		"title": "Wie geht es Ihnen im Team?", "intro": "Ihre Meinung hilft uns, den Alltag in der OGS besser zu planen. Die Umfrage dauert etwa fünf Minuten.",
		"audience": "staff", "anonymous": true, "send_email": false,
		"closes_at": time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339),
		"targets":   []map[string]any{}, "questions": seedSurveyQuestions(true),
	}
}

func seedParentSurveyBody() map[string]any {
	return map[string]any{
		"title": "Elternbefragung zum Schuljahr", "intro": "Wir möchten wissen, was gut läuft und was wir besser machen können.",
		"audience": "parents", "anonymous": true, "send_email": false,
		"closes_at": time.Now().Add(21 * 24 * time.Hour).UTC().Format(time.RFC3339),
		"targets":   []map[string]any{{"target_type": "school_all"}}, "questions": seedSurveyQuestions(false),
	}
}

// seedSurvey creates and publishes one survey and returns its questions.
func seedSurvey(rt *Runtime, body map[string]any) (seedSurveyResponse, error) {
	var survey seedSurveyResponse
	raw, err := rt.Client.Post("/api/surveys/", body)
	if err != nil {
		return survey, fmt.Errorf("create survey %q: %w", body["title"], err)
	}
	if err := parseJSON(raw, &survey); err != nil || survey.Data.ID == "" {
		return survey, fmt.Errorf("parse survey %q: %w", body["title"], err)
	}
	if _, err := rt.Client.Post("/api/surveys/"+survey.Data.ID+"/publish", nil); err != nil {
		return survey, fmt.Errorf("publish survey %q: %w", body["title"], err)
	}
	return survey, nil
}

// seedSurveyAnswers fills every question; i varies the answers per person.
func seedSurveyAnswers(questions []seedSurveyQuestion, i int) map[string]any {
	texts := []string{"Mehr Zeit für Gespräche mit den Eltern.", "Ein ruhiger Raum für die Hausaufgaben.", "Weiter so!"}
	answers := make([]map[string]any, 0, len(questions))
	for q, question := range questions {
		answer := map[string]any{"question_id": question.ID}
		switch question.Kind {
		case "scale":
			answer["scale_value"] = 2 + (i+q)%4
		case "single":
			answer["option_ids"] = []string{question.Options[i%len(question.Options)].ID}
		case "multiple":
			answer["option_ids"] = []string{question.Options[i%len(question.Options)].ID, question.Options[(i+2)%len(question.Options)].ID}
		case "text":
			if i%2 == 1 {
				continue
			}
			answer["text"] = texts[(i/2)%len(texts)]
		}
		answers = append(answers, answer)
	}
	return map[string]any{"answers": answers}
}

func seedStaffSurveyAnswers(ctx context.Context, rt *Runtime, survey seedSurveyResponse) (int, error) {
	if rt.FixedSeeder == nil || rt.Bootstrap == nil {
		return 0, nil
	}
	answered := 0
	for i, staff := range rt.FixedSeeder.staffCredentials {
		if answered == seedSurveyAnswerers {
			break
		}
		auth, err := rt.Adapter.LoginTenant(ctx, staff.Email, staff.Password, rt.Bootstrap.TenantSlug)
		if err != nil {
			return answered, fmt.Errorf("login %s to answer the staff survey: %w", staff.Email, err)
		}
		if _, err := rt.Client.PostWithAuth(auth, "/api/surveys/"+survey.Data.ID+"/answers", seedSurveyAnswers(survey.Data.Questions, i)); err != nil {
			return answered, fmt.Errorf("answer staff survey as %s: %w", staff.Email, err)
		}
		answered++
	}
	return answered, nil
}

func seedParentSurveyAnswers(ctx context.Context, rt *Runtime, survey seedSurveyResponse) (int, error) {
	answered := 0
	for i, parent := range rt.Parents {
		if answered == seedSurveyAnswerers {
			break
		}
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return answered, fmt.Errorf("login parent to answer the survey: %w", err)
		}
		announcementID, err := seedSurveyCarrier(rt, auth, survey.Data.ID)
		if err != nil {
			return answered, err
		}
		if announcementID == "" {
			continue
		}
		if _, err := rt.Client.PostWithAuth(auth, "/parent/me/news/"+announcementID+"/survey", seedSurveyAnswers(survey.Data.Questions, i)); err != nil {
			return answered, fmt.Errorf("answer parent survey: %w", err)
		}
		answered++
	}
	return answered, nil
}

// seedSurveyCarrier finds the feed entry of the survey in a parent's news.
func seedSurveyCarrier(rt *Runtime, auth AuthRef, surveyID string) (string, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/parent/me/news")
	if err != nil {
		return "", fmt.Errorf("load parent news for the survey: %w", err)
	}
	var feed struct {
		Data []struct {
			ID     string `json:"id"`
			Survey *struct {
				SurveyID string `json:"survey_id"`
			} `json:"survey"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &feed); err != nil {
		return "", fmt.Errorf("parse parent news: %w", err)
	}
	for _, item := range feed.Data {
		if item.Survey != nil && item.Survey.SurveyID == surveyID {
			return item.ID, nil
		}
	}
	return "", nil
}
