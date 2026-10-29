package api

import (
	"context"
	"encoding/json"
	"fmt"
)

const seedHolidayLegalPeriodKey = "traeger.holiday_legal_period"
const seedHolidayLegalPublishedKey = "traeger.holiday_legal_published"

type seedHolidayLegalStep struct{}

func (seedHolidayLegalStep) Name() string { return "Seeding holiday PDF notices and acceptances" }
func (seedHolidayLegalStep) Run(_ context.Context, rt *Runtime) error {
	period, ok := rt.Values[seedHolidayLegalPeriodKey].(int64)
	if !ok {
		return fmt.Errorf("the seeded holiday period is missing")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	path := fmt.Sprintf("/traeger/holiday-periods/%d/form", period)
	if err := seedHolidayLegalForm(rt, auth, path); err != nil {
		return err
	}
	return seedHolidayLegalRegistration(rt)
}

func seedHolidayLegalForm(rt *Runtime, auth AuthRef, path string) error {
	raw, err := rt.Client.GetWithAuth(auth, path)
	if err != nil {
		return err
	}
	var form struct {
		Data struct {
			Fields json.RawMessage `json:"fields"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &form); err != nil {
		return err
	}
	blocks := []map[string]any{}
	for i, title := range []string{"Hinweise zur Ferienbetreuung", "Hinweise zu Ausflügen"} {
		id, err := seedHolidayLegalPDF(rt, auth, path, i+1, title)
		if err != nil {
			return err
		}
		blocks = append(blocks, map[string]any{"key": fmt.Sprintf("hinweis_%d", i+1), "kind": "terms", "source": "custom", "title": title, "label": "Ich habe die Hinweise gelesen und zur Kenntnis genommen.", "enabled": true, "required": true, "display_mode": "pdf", "attachments": []map[string]string{{"label": title, "document_id": id}}})
	}
	blocks = append(blocks, map[string]any{"key": "fotos_ferien", "kind": "consent", "source": "custom", "title": "Fotos in der Ferienbetreuung", "label": "Mein Kind darf auf Erinnerungsfotos zu sehen sein.", "text": "Die Fotos bleiben in der OGS.", "enabled": true})
	_, err = rt.Client.PutWithAuth(auth, path, map[string]any{"fields": form.Data.Fields, "legal_blocks": blocks})
	return err
}

func seedHolidayLegalPDF(rt *Runtime, auth AuthRef, path string, position int, title string) (string, error) {
	content := demoPDF(title, []string{"Bitte geben Sie Ihrem Kind eine Trinkflasche mit.", "Bei Fragen hilft Ihnen die OGS."})
	raw, err := rt.Client.uploadFile(auth, "POST", path+"/documents", "document", fmt.Sprintf("hinweis-%d.pdf", position), content, nil)
	if err != nil {
		return "", err
	}
	var doc struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err = json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	if doc.Data.ID == "" {
		return "", fmt.Errorf("the seeded holiday PDF has no ID")
	}
	return doc.Data.ID, nil
}

// Older registrations retain version 1 without legal blocks. This later
// registration demonstrates the evidence display of the new version.
func seedHolidayLegalRegistration(rt *Runtime) error {
	if rt.Values[seedHolidayRegistrationsSkippedKey] == true {
		return nil
	}
	published, ok := rt.Values[seedHolidayLegalPublishedKey].([]byte)
	if !ok {
		return fmt.Errorf("the published holiday offer is missing")
	}
	token, body, err := seedHolidayRegistration(published, rt.Bootstrap.SchoolID)
	if err != nil {
		return err
	}
	body["guardian"].(map[string]any)["email"] = "hinweise.ferien@example.org"
	body["legal_acceptances"] = map[string]bool{"hinweis_1": true, "hinweis_2": true, "fotos_ferien": false}
	_, err = rt.Client.PostPublic("/traeger/holiday-registrations/"+token, body)
	return err
}
