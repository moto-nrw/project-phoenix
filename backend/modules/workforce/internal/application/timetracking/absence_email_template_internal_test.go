package timetracking

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAbsenceEmailTemplateDataFollowsTheTemplate(t *testing.T) {
	t.Parallel()
	content := AbsenceEmailContent{
		FirstName: "Mia", LastName: "Meier", RequesterName: "Ben Bauer", AbsenceTypeLabel: "Urlaub", DateRange: "19.–23.10.",
		Note: "Familie", PreviousQuestion: "Frage", DecisionNote: "Genehmigt", LinkURL: "https://app.test/x", LogoURL: "https://logo",
	}
	shared := map[string]any{
		"FirstName": "Mia", "LastName": "Meier", "AbsenceTypeLabel": "Urlaub", "DateRange": "19.–23.10.",
		"LinkURL": "https://app.test/x", "LogoURL": "https://logo",
	}

	request := AbsenceEmailMessage{Template: absenceRequestReceivedTemplate, Content: content}.TemplateData()
	want := map[string]any{"RequesterName": "Ben Bauer", "Note": "Familie", "PreviousQuestion": "Frage"}
	for key, value := range shared {
		want[key] = value
	}
	assert.Equal(t, want, request)

	decision := AbsenceEmailMessage{Template: "absence-request-approved.html", Content: content}.TemplateData()
	want = map[string]any{"DecisionNote": "Genehmigt"}
	for key, value := range shared {
		want[key] = value
	}
	assert.Equal(t, want, decision)
}
