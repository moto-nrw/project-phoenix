package api

import (
	"fmt"
	"time"
)

type parentPortalSeedAction struct {
	actionType string
	path       string
	body       map[string]any
}

func (s parentEnrollmentSeedStep) seedParentPortalActions(rt *Runtime, parentAuth AuthRef, parent ParentCredentials) ([]SeedParentPortalAction, error) {
	if len(parent.StudentIDs) == 0 {
		return nil, nil
	}
	studentID := parent.StudentIDs[0]
	if err := loadParentPortalOverview(rt, parentAuth, studentID); err != nil {
		return nil, err
	}
	actions := parentPortalSeedActions(studentID)
	out := make([]SeedParentPortalAction, 0, len(actions))
	for _, action := range actions {
		body, err := rt.Client.PostWithAuth(parentAuth, action.path, action.body)
		if err != nil {
			return nil, fmt.Errorf("create parent portal action %s: %w", action.actionType, err)
		}
		if action.actionType == "care-exception" {
			if err := s.shareSeedPickupRequest(rt, parentAuth, parent, body); err != nil {
				return nil, err
			}
		}
		out = append(out, SeedParentPortalAction{ParentEmail: parent.Email, StudentID: studentID, Type: action.actionType})
	}
	return out, nil
}

func loadParentPortalOverview(rt *Runtime, auth AuthRef, studentID int64) error {
	for _, check := range []struct{ description, path string }{
		{"load parent profile", "/parent/me/profile"},
		{"load parent children", "/parent/me/children"},
		{"load parent child features", fmt.Sprintf("/parent/me/children/%d/features", studentID)},
	} {
		if _, err := rt.Client.GetWithAuth(auth, check.path); err != nil {
			return fmt.Errorf("%s: %w", check.description, err)
		}
	}
	return nil
}

func parentPortalSeedActions(studentID int64) []parentPortalSeedAction {
	sickNotePath := fmt.Sprintf("/parent/me/children/%d/sick-note", studentID)
	return []parentPortalSeedAction{
		{actionType: "sick-note", path: sickNotePath, body: map[string]any{
			"dates": []string{time.Now().AddDate(0, 0, 2).Format("2006-01-02")}, "reason": "Demo-Krankmeldung aus dem Elternportal",
		}},
		{actionType: "excused-note", path: sickNotePath, body: map[string]any{
			"dates": []string{todaySeedDate().AddDays(4).String()}, "status": "excused", "reason": "Demo-Entschuldigung aus dem Elternportal",
		}},
		{actionType: "care-exception", path: fmt.Sprintf("/parent/me/children/%d/care-exception", studentID), body: map[string]any{
			"date": time.Now().AddDate(0, 0, 3).Format("2006-01-02"), "pickup_time": "14:30", "reason": "Demo-Abholänderung aus dem Elternportal",
		}},
		{actionType: "related-account", path: fmt.Sprintf("/parent/me/children/%d/related-accounts", studentID), body: map[string]any{
			"email": "oma.schneider@example.test", "first_name": "Elke", "last_name": "Schneider",
		}},
	}
}
