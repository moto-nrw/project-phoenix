package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// Seed keys of the income declaration in the demo form (#4783). They match
// the keys the form editor's Einkommenserklärung preset uses.
const (
	seedIncomeLevelKey = "einkommen_stufe"
	seedIncomeProofKey = "einkommen_nachweis"
)

// seedIncomeDeclarationFields are the two questions of the preset: the income
// level the school maintains and the proof, shown for a reduced level only.
func seedIncomeDeclarationFields() []map[string]any {
	return []map[string]any{
		{
			"key": seedIncomeLevelKey, "label": "Einkommensstufe", "type": "select", "sort_order": 30,
			"options": []map[string]any{
				{"label": "Voller Beitrag", "value": "voll"},
				{"label": "Ermäßigt Stufe 1", "value": "stufe_1"},
				{"label": "Ermäßigt Stufe 2", "value": "stufe_2"},
				{"label": "Beitragsfrei", "value": "frei"},
			},
		},
		{
			"key": seedIncomeProofKey, "label": "Einkommensnachweis", "type": "file", "sort_order": 31,
			"help_text":    "Bitte laden Sie den aktuellen Einkommensnachweis hoch, zum Beispiel den Steuerbescheid.",
			"file_options": map[string]any{"allowed_types": []string{"pdf", "jpg", "png"}, "max_size_mb": 10, "multiple": true},
			"visible_when": map[string]any{"source": "field", "field": seedIncomeLevelKey, "operator": "neq", "value": "voll"},
		},
	}
}

// seedEnrollmentIncomeProofStep submits one family with a reduced income
// level and its uploaded proof, so the request detail shows a Nachweis.
type seedEnrollmentIncomeProofStep struct{}

func (seedEnrollmentIncomeProofStep) Name() string { return "Seeding enrollment income proof" }

func (seedEnrollmentIncomeProofStep) Run(_ context.Context, rt *Runtime) error {
	if rt.Bootstrap == nil || rt.Enrollment.PhaseID == 0 {
		return nil
	}
	rt.Client.BindAuth(rt.TenantAuth)
	slug := rt.Bootstrap.TenantSlug
	raw, err := rt.Client.PostFileWithFields("/api/enrollment/"+slug+"/uploads", "file", "Einkommensbescheid-2025.pdf",
		demoPDF("Einkommensbescheid 2025", []string{"Familie Rahimi", "Jahreseinkommen laut Steuerbescheid", "Kopie für die Anmeldung"}),
		map[string]string{"phase_id": strconv.FormatInt(rt.Enrollment.PhaseID, 10), "field_key": seedIncomeProofKey})
	if err != nil {
		return fmt.Errorf("upload income proof: %w", err)
	}
	var upload struct {
		Data struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &upload); err != nil || upload.Data.Token == "" {
		return fmt.Errorf("upload income proof returned no token: %s", string(raw))
	}
	body := map[string]any{
		"phase_id":            rt.Enrollment.PhaseID,
		"guardian_first_name": "Zahra",
		"guardian_last_name":  "Rahimi",
		"guardian_email":      "zahra.rahimi@example.test",
		"guardian_phone":      "+49 221 555 991",
		"consent_flags":       map[string]any{"data_processing": true},
		"custom_data": map[string]any{
			seedIncomeLevelKey: "stufe_1",
			seedIncomeProofKey: []map[string]any{{"token": upload.Data.Token}},
		},
		"children": []map[string]any{{
			"first_name": "Ahmad", "last_name": "Rahimi", "date_of_birth": "2019-05-12", "target_grade_level": 1,
			"offering_ids":  []int64{rt.Enrollment.Offerings["ogs-kurz"], rt.Enrollment.Offerings["mittagessen"]},
			"offering_days": []map[string]any{{"offering_id": rt.Enrollment.Offerings["ogs-kurz"], "selected_days": []string{"tue", "thu"}}, {"offering_id": rt.Enrollment.Offerings["mittagessen"], "selected_days": []string{"tue", "thu"}}},
			"custom_data":   map[string]any{"allergies": "keine"},
		}},
	}
	if _, err := rt.Client.PostPublicWithHeaders("/api/enrollment/"+slug+"/submit", body, publicEnrollmentSeedHeaders(90)); err != nil {
		return fmt.Errorf("submit enrollment with income proof: %w", err)
	}
	fmt.Println("Seeded 1 enrollment request with an income proof")
	return nil
}
