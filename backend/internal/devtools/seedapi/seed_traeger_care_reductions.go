package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
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

type seedCareRule struct {
	ID         int64          `json:"id"`
	Kind       string         `json:"kind"`
	ValidUntil *calendar.Date `json:"valid_until"`
	SchoolIDs  []int64        `json:"school_ids"`
}

type seedCareRuleSet struct {
	Rules   []seedCareRule `json:"rules"`
	Schools []struct {
		ID int64 `json:"id"`
	} `json:"schools"`
}

type seedCareEvidenceSet struct {
	Contracts []struct {
		ContractID int64             `json:"contract_id"`
		SchoolID   int64             `json:"school_id"`
		Evidences  []json.RawMessage `json:"evidences"`
	} `json:"contracts"`
	Levels []seedCareRule `json:"levels"`
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

// seedCareRules are the placeholder rules of the Träger: two income levels
// and one rule of every other kind.
var seedCareRules = []map[string]any{
	{"kind": "income_level", "name": "Stufe 1 (Platzhalter)", "discount_basis_points": 5000},
	{"kind": "income_level", "name": "Stufe 2 (Platzhalter)", "discount_basis_points": 2500},
	{"kind": "sibling", "name": "Geschwister ab dem 2. Kind (Platzhalter)", "sibling_from_child": 2, "discount_basis_points": 5000,
		"counts_family_children": true},
	{"kind": "single_parent", "name": "Alleinerziehende (Platzhalter)", "discount_cents": 1000},
	{"kind": "cap", "name": "Höchstbetrag je Familie (Platzhalter)", "cap_cents": 25000},
}

// seedCareReductions keeps the placeholder rules of the Träger from month on,
// an income level and a single-parent evidence for the first two contracts
// of the seeded school, its cost bearer shares and its family numbers.
func seedCareReductions(client *Client, auth AuthRef, schoolID int64, month calendar.Date) (int, int, error) {
	rules, err := seedCareRuleChain(client, auth, schoolID, month)
	if err != nil {
		return 0, 0, err
	}
	evidences, own, err := seedCareEvidences(client, auth, schoolID, month)
	if err != nil {
		return rules, evidences, err
	}
	if err := seedCareCostShares(client, auth, schoolID, month); err != nil {
		return rules, evidences, err
	}
	families, err := seedFamilyChildren(client, auth, own, month)
	fmt.Printf("  Träger family numbers: %d confirmed\n", families)
	return rules, evidences, err
}

// seedCareRuleChain keeps every placeholder rule for the seeded school. A
// Träger without rules of a kind gets them for all its schools. The public
// demo seeds many schools into one Träger, one after another: a school that
// came after the first seed gets rules of its own, since a rule of a kind
// the school does not fall under cannot carry its evidences (#6127). Rules
// the Träger already keeps stay as they are.
func seedCareRuleChain(client *Client, auth AuthRef, schoolID int64, month calendar.Date) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-rules/")
	if err != nil {
		return 0, fmt.Errorf("read the care rules: %w", err)
	}
	set, err := seedReadData[seedCareRuleSet](raw, "care rules")
	if err != nil {
		return 0, err
	}
	if len(set.Schools) == 0 {
		return 0, nil
	}
	schools := make([]int64, len(set.Schools))
	for i, school := range set.Schools {
		schools[i] = school.ID
	}
	kept := len(set.Rules)
	for _, placeholder := range seedCareRules {
		kind, _ := placeholder["kind"].(string)
		targets := seedCareRuleTargets(set.Rules, kind, schools, schoolID, month)
		if len(targets) == 0 {
			continue
		}
		kept++
		// Demo schools of one Träger seed in parallel: another seed may have
		// kept the rule of this kind a moment earlier, possibly without the
		// seeded school, which it did not know yet.
		err := seedKeepCareRule(client, auth, placeholder, targets, month)
		if seedConflictCode(err, seedCareRuleOverlap) && len(targets) > 1 && slices.Contains(targets, schoolID) {
			err = seedKeepCareRule(client, auth, placeholder, []int64{schoolID}, month)
		}
		if err != nil && !seedConflictCode(err, seedCareRuleOverlap) {
			return kept, err
		}
	}
	return kept, nil
}

// seedCareRuleTargets are the schools a placeholder rule of kind is kept
// for: every school of a Träger without such a rule, otherwise the seeded
// school if no rule of kind reaches it yet.
func seedCareRuleTargets(rules []seedCareRule, kind string, schools []int64, schoolID int64, month calendar.Date) []int64 {
	if !seedCareKindKept(rules, kind) {
		return schools
	}
	if !slices.Contains(schools, schoolID) || seedCareRuleCovers(rules, kind, schoolID, month) {
		return nil
	}
	return []int64{schoolID}
}

func seedKeepCareRule(client *Client, auth AuthRef, placeholder map[string]any, schools []int64, month calendar.Date) error {
	rule := maps.Clone(placeholder)
	rule["valid_from"], rule["school_ids"] = month.String(), schools
	if _, err := client.PostWithAuth(auth, "/traeger/care-rules/", rule); err != nil {
		return fmt.Errorf("keep the care rule %q: %w", rule["name"], err)
	}
	return nil
}

// seedCareKindKept reports whether the Träger keeps any rule of kind.
func seedCareKindKept(rules []seedCareRule, kind string) bool {
	return slices.ContainsFunc(rules, func(rule seedCareRule) bool { return rule.Kind == kind })
}

// seedCareRuleCovers reports whether a rule of kind applies at the school
// in month or later.
func seedCareRuleCovers(rules []seedCareRule, kind string, schoolID int64, month calendar.Date) bool {
	return slices.ContainsFunc(rules, func(rule seedCareRule) bool {
		return rule.Kind == kind && slices.Contains(rule.SchoolIDs, schoolID) && (rule.ValidUntil == nil || !rule.ValidUntil.Before(month))
	})
}

// seedCareEvidences gives the first contract of the seeded school without
// an evidence an income level of that school and the next one the
// single-parent status. Contracts of the Träger's other schools belong to
// other demo visitors and stay untouched. It also answers the contracts of
// the seeded school in force.
func seedCareEvidences(client *Client, auth AuthRef, schoolID int64, month calendar.Date) (int, []int64, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/care-evidences/")
	if err != nil {
		return 0, nil, fmt.Errorf("read the care evidences: %w", err)
	}
	set, err := seedReadData[seedCareEvidenceSet](raw, "care evidences")
	if err != nil {
		return 0, nil, err
	}
	var own []int64
	for _, contract := range set.Contracts {
		if contract.SchoolID == schoolID {
			own = append(own, contract.ContractID)
		}
	}
	var kinds []map[string]any
	if level := slices.IndexFunc(set.Levels, func(rule seedCareRule) bool {
		return seedCareRuleCovers([]seedCareRule{rule}, rule.Kind, schoolID, month)
	}); level >= 0 {
		kinds = append(kinds, map[string]any{"kind": "income_level", "rule_id": set.Levels[level].ID, "evidence": "Einkommensteuerbescheid 2025"})
	}
	kinds = append(kinds, map[string]any{"kind": "single_parent", "evidence": "Erklärung des Elternteils"})
	added := 0
	for _, contract := range set.Contracts {
		if added == len(kinds) {
			break
		}
		if contract.SchoolID != schoolID || len(contract.Evidences) > 0 {
			continue
		}
		body := kinds[added]
		body["contract_id"], body["valid_from"], body["checked_on"] = contract.ContractID, month.String(), calendar.TodayDate().String()
		if _, err := client.PostWithAuth(auth, "/traeger/care-evidences/", body); err != nil && !seedConflictCode(err, seedCareEvidenceOverlap) {
			return added, own, fmt.Errorf("keep a care evidence: %w", err)
		}
		added++
	}
	return added, own, nil
}
