package api

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The rule chain of the monthly contribution (#4685): placeholder rules for
// every kind at every school of the demo Träger and two evidences, so the
// pages „Ermäßigungen“ and „Nachweise“ and the reductions on the invoices
// are filled on every dev machine. The amounts are placeholders, no
// customer's prices.

// The registered codes of a rule or an evidence another seed of the same
// Träger kept a moment earlier.
const (
	seedCareRuleOverlap     = "parent_billing.contribution_rule_overlap"
	seedCareEvidenceOverlap = "parent_billing.care_evidence_overlap"
)

// seedConflictCode reports a 409 with the given registered code.
func seedConflictCode(err error, code string) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == 409 && apiErr.Code == code
}

type seedCareRuleSet struct {
	Rules []struct {
		ID   int64  `json:"id"`
		Kind string `json:"kind"`
	} `json:"rules"`
	Schools []struct {
		ID int64 `json:"id"`
	} `json:"schools"`
}

type seedCareEvidenceSet struct {
	Contracts []struct {
		ContractID int64             `json:"contract_id"`
		Evidences  []json.RawMessage `json:"evidences"`
	} `json:"contracts"`
	Levels []struct {
		ID int64 `json:"id"`
	} `json:"levels"`
}

func seedReadData[T any](raw []byte, what string) (T, error) {
	var envelope struct {
		Data T `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return envelope.Data, fmt.Errorf("read the %s: %w", what, err)
	}
	return envelope.Data, nil
}

// seedCareReductions keeps the placeholder rules of the Träger from month on
// and an income level and a single-parent evidence for the first two
// contracts. A Träger that already has care rules keeps them.
func seedCareReductions(client *Client, auth AuthRef, month calendar.Date) (int, int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-rules/")
	if err != nil {
		return 0, 0, fmt.Errorf("read the care rules: %w", err)
	}
	set, err := seedReadData[seedCareRuleSet](raw, "care rules")
	if err != nil {
		return 0, 0, err
	}
	if len(set.Schools) == 0 {
		return 0, 0, nil
	}
	schools := make([]int64, len(set.Schools))
	for i, school := range set.Schools {
		schools[i] = school.ID
	}
	if len(set.Rules) == 0 {
		for _, rule := range []map[string]any{
			{"kind": "income_level", "name": "Stufe 1 (Platzhalter)", "discount_basis_points": 5000},
			{"kind": "income_level", "name": "Stufe 2 (Platzhalter)", "discount_basis_points": 2500},
			{"kind": "sibling", "name": "Geschwister ab dem 2. Kind (Platzhalter)", "sibling_from_child": 2, "discount_basis_points": 5000},
			{"kind": "single_parent", "name": "Alleinerziehende (Platzhalter)", "discount_cents": 1000},
			{"kind": "cap", "name": "Höchstbetrag je Familie (Platzhalter)", "cap_cents": 25000},
		} {
			rule["valid_from"], rule["school_ids"] = month.String(), schools
			// Demo schools of one Träger seed in parallel: another seed may
			// have kept the rule of this kind for the same schools already.
			if _, err := client.PostWithAuth(auth, "/traeger/care-rules/", rule); err != nil && !seedConflictCode(err, seedCareRuleOverlap) {
				return 0, 0, fmt.Errorf("keep the care rule %q: %w", rule["name"], err)
			}
		}
	}
	evidences, err := seedCareEvidences(client, auth, month)
	return max(len(set.Rules), 5), evidences, err
}

// seedCareEvidences gives the first contract without an evidence the first
// income level and the next one the single-parent status.
func seedCareEvidences(client *Client, auth AuthRef, month calendar.Date) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-evidences/")
	if err != nil {
		return 0, fmt.Errorf("read the care evidences: %w", err)
	}
	set, err := seedReadData[seedCareEvidenceSet](raw, "care evidences")
	if err != nil {
		return 0, err
	}
	if len(set.Levels) == 0 {
		return 0, nil
	}
	kinds := []map[string]any{
		{"kind": "income_level", "rule_id": set.Levels[0].ID, "evidence": "Einkommensteuerbescheid 2025"},
		{"kind": "single_parent", "evidence": "Erklärung des Elternteils"},
	}
	added := 0
	for _, contract := range set.Contracts {
		if added == len(kinds) {
			break
		}
		if len(contract.Evidences) > 0 {
			continue
		}
		body := kinds[added]
		body["contract_id"], body["valid_from"], body["checked_on"] = contract.ContractID, month.String(), calendar.TodayDate().String()
		if _, err := client.PostWithAuth(auth, "/traeger/care-evidences/", body); err != nil && !seedConflictCode(err, seedCareEvidenceOverlap) {
			return added, fmt.Errorf("keep a care evidence: %w", err)
		}
		added++
	}
	return added, nil
}
