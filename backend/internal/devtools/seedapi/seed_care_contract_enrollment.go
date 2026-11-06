package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
)

// seedConclusionEnrollment submits a new parent's agreement through Enrollment,
// which snapshots and checksums the displayed text, then approves the child.
// The regular Worker can conclude it; the older candidates stay without proof.
func seedConclusionEnrollment(rt *Runtime) error {
	const key = "care-contract-acceptance"
	for _, request := range rt.Enrollment.Requests {
		if request.Key == key {
			return nil
		}
	}
	offeringID := rt.Enrollment.Offerings["ogs-ganztag"]
	text, err := seedContractAcceptanceText(rt, offeringID)
	if err != nil {
		return err
	}
	raw, err := rt.Client.PostWithAuth(rt.TenantAuth, "/api/enrollment/schema/", map[string]any{
		"name": "Anmeldung mit Betreuungsvertrag", "fields": []any{},
		"legal_blocks": []map[string]any{{
			"key": "care_contract", "kind": "terms", "title": "Betreuungsvertrag",
			"label": "Ich stimme den Vertragsbedingungen zu.", "text": text,
			"required": true, "enabled": true, "source": "custom",
		}},
	})
	if err != nil {
		return fmt.Errorf("create the enrollment form with contract conditions: %w", err)
	}
	schemaID, err := parseEnvelopeStringID(raw)
	if err != nil {
		return err
	}
	return withContractEnrollmentForm(rt, schemaID, func() error {
		submission := enrollmentSubmission{
			source: "public", key: key, path: "/api/enrollment/" + rt.Bootstrap.TenantSlug + "/submit",
			body: map[string]any{
				"phase_id":            rt.Enrollment.PhaseID,
				"guardian_first_name": "Miriam", "guardian_last_name": "Berger",
				"guardian_email": scopedEmail("miriam.berger@example.test", rt.FixedSeeder.accountScope),
				"consent_flags":  map[string]bool{"data_processing": true, "care_contract": true},
				"children": []map[string]any{{
					"first_name": "Tilda", "last_name": "Berger", "date_of_birth": "2019-05-14", "target_grade_level": 1,
					"offering_ids":  []int64{offeringID},
					"offering_days": []map[string]any{{"offering_id": offeringID, "selected_days": []string{"mon", "wed", "fri"}}},
				}},
			},
		}
		step := parentEnrollmentSeedStep{}
		request, err := step.submitSeedEnrollment(rt, rt.TenantAuth, submission, 92)
		if err != nil {
			return err
		}
		for _, childID := range request.ChildIDs {
			if err := step.decideEnrollmentChild(rt, rt.TenantAuth, request.RequestID, childID, "approved", "Zusage: Betreuung mit Betreuungsvertrag"); err != nil {
				return err
			}
		}
		request.Status = "approved"
		rt.Enrollment.Requests = append(rt.Enrollment.Requests, request)
		return nil
	})
}

func seedContractAcceptanceText(rt *Runtime, offeringID int64) (string, error) {
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return "", err
	}
	raw, err := rt.Client.GetWithAuth(auth, "/traeger/contract-templates/")
	if err != nil {
		return "", err
	}
	var envelope struct {
		Data []struct {
			SchoolID       int64  `json:"school_id"`
			OfferingID     int64  `json:"offering_id"`
			AcceptanceText string `json:"acceptance_text"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", err
	}
	for _, offering := range envelope.Data {
		if offering.SchoolID == rt.Bootstrap.SchoolID && offering.OfferingID == offeringID && offering.AcceptanceText != "" {
			return offering.AcceptanceText, nil
		}
	}
	return "", fmt.Errorf("offering %d has no saved contract conditions with an explicit partner", offeringID)
}

// Only the new submission uses this form; restore the phase even on failure.
func withContractEnrollmentForm(rt *Runtime, schemaID int64, submit func() error) (err error) {
	original, ok := rt.Values[enrollmentPhaseBodyKey].(map[string]any)
	if !ok || rt.Enrollment.PhaseID <= 0 {
		return fmt.Errorf("the enrollment phase is not available for the contract submission")
	}
	path := fmt.Sprintf("/api/enrollment/phases/%d", rt.Enrollment.PhaseID)
	body := maps.Clone(original)
	body["form_schema_id"] = strconv.FormatInt(schemaID, 10)
	if _, err := rt.Client.PutWithAuth(rt.TenantAuth, path, body); err != nil {
		return err
	}
	defer func() {
		if _, restoreErr := rt.Client.PutWithAuth(rt.TenantAuth, path, original); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore the enrollment form: %w", restoreErr))
		}
	}()
	return submit()
}
