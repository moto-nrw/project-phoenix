package api

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
)

// seedTraegerCareContractsStep gives the demo Träger its care contracts
// (#4678, ADR 0153) through the Träger routes, the way the Träger-Büro works:
// a contract template for every active offering of its schools, a draft for
// every accepted application over all modules the child books, and every
// second draft sent. The last few candidates of one school get no draft:
// that school then concludes its contracts at the acceptance (#4680), so the
// Worker's conclusion checks their acceptance evidence. Legacy demo
// submissions without a ticked, checksummed conditions block stay candidates.
// A fresh submission accepts the saved conditions before its approval; the
// seed never fabricates retrospective parental consent. Lina Amini, whose
// parents' change of care the school approved (parent enrollment seed), is
// reserved there too: once her contract is active, the Worker's next run
// gives it version 2 from the day of the change (#4681).
type seedTraegerCareContractsStep struct{}

func (seedTraegerCareContractsStep) Name() string { return "Seeding the Träger care contracts" }

// seedCareContractTemplate is the contract template of every demo offering.
var seedCareContractTemplate = map[string]any{
	"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end",
	"special_termination": true, "special_termination_note": "Bei Umzug, Schulwechsel oder Verlust des Arbeitsplatzes können Sie zum Ende des Folgemonats kündigen.",
	"statute_required": true, "statute_title": "Satzung über die Elternbeiträge für die Offene Ganztagsschule",
	"statute_url":      "https://ganztag-quartier.example.test/satzung",
	"contract_partner": "Ganztag im Quartier e. V.",
}

type seedCareContractCandidate struct {
	SchoolID       int64  `json:"school_id"`
	RequestChildID int64  `json:"request_child_id"`
	TemplateIssue  string `json:"template_issue"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
}

// seedChangedCare names the child whose parents changed the care with an
// approved request; her contract is concluded so it can follow the change.
func (c seedCareContractCandidate) seedChangedCare() bool {
	return c.ChildFirstName == "Lina" && c.ChildLastName == "Amini" && c.TemplateIssue == ""
}

func (seedTraegerCareContractsStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger care contract seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	templates, err := seedCareContractTemplates(rt.Client, auth)
	if err != nil {
		return err
	}
	created, err := seedCareContractDrafts(rt, auth)
	if err != nil {
		return err
	}
	sent, err := seedCareContractSends(rt.Client, auth)
	if err != nil {
		return err
	}
	terminated, err := seedCareContractNotice(rt.Client, auth)
	if err != nil {
		return err
	}
	changed, err := seedCareContractChange(rt, auth)
	if err != nil {
		return err
	}
	fmt.Printf("  Träger care contracts: %d templates, %d drafts created, %d sent, %d terminated, change asked: %t\n", templates, created, sent, terminated, changed)
	return nil
}

// seedCareContractTemplates saves the template of every active offering
// that has none yet.
func seedCareContractTemplates(client *Client, auth AuthRef) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contract-templates/")
	if err != nil {
		return 0, fmt.Errorf("read the contract templates: %w", err)
	}
	var envelope struct {
		Data []struct {
			SchoolID   int64           `json:"school_id"`
			OfferingID int64           `json:"offering_id"`
			Active     bool            `json:"active"`
			Template   json.RawMessage `json:"template"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the contract templates: %w", err)
	}
	saved := 0
	for _, offering := range envelope.Data {
		if !offering.Active || (len(offering.Template) > 0 && string(offering.Template) != "null") {
			continue
		}
		path := fmt.Sprintf("/traeger/contract-templates/%d/%d", offering.SchoolID, offering.OfferingID)
		if _, err := client.PutWithAuth(auth, path, seedCareContractTemplate); err != nil {
			return saved, fmt.Errorf("save the contract template of offering %d: %w", offering.OfferingID, err)
		}
		saved++
	}
	return saved, nil
}

// seedCareContractDrafts creates a draft for every candidate with a
// template; a candidate another seeding school took meanwhile is skipped.
func seedCareContractDrafts(rt *Runtime, auth AuthRef) (int, error) {
	client := rt.Client
	raw, err := client.GetWithAuth(auth, "/traeger/contracts/candidates")
	if err != nil {
		return 0, fmt.Errorf("read the contract candidates: %w", err)
	}
	var envelope struct {
		Data []seedCareContractCandidate `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the contract candidates: %w", err)
	}
	onAcceptance, reserved := seedConclusionCandidates(envelope.Data)
	keys := []map[string]any{}
	for _, candidate := range envelope.Data {
		held := candidate.SchoolID == onAcceptance && reserved[candidate.RequestChildID]
		if candidate.TemplateIssue == "" && !held && len(keys) < 200 {
			keys = append(keys, map[string]any{"school_id": candidate.SchoolID, "request_child_id": candidate.RequestChildID})
		}
	}
	if len(keys) == 0 {
		return 0, seedConclusionAtAcceptance(rt, onAcceptance)
	}
	raw, err = client.PostWithAuth(auth, "/traeger/contracts/", map[string]any{"contracts": keys})
	if err != nil {
		return 0, fmt.Errorf("create the contract drafts: %w", err)
	}
	var created struct {
		Data struct {
			Created []json.RawMessage `json:"created"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		return 0, fmt.Errorf("read the created contracts: %w", err)
	}
	return len(created.Data.Created), seedConclusionAtAcceptance(rt, onAcceptance)
}

// seedConclusionCandidates picks the school of the child with the changed
// care, else the school with the highest ID among the candidates, and
// reserves that child and its last three candidates for the conclusion at
// the acceptance; they get no draft.
func seedConclusionCandidates(candidates []seedCareContractCandidate) (int64, map[int64]bool) {
	var school int64
	changed := int64(0)
	for _, candidate := range candidates {
		school = max(school, candidate.SchoolID)
		if candidate.seedChangedCare() {
			changed = candidate.RequestChildID
		}
	}
	for _, candidate := range candidates {
		if candidate.RequestChildID == changed {
			school = candidate.SchoolID
		}
	}
	ids := []int64{}
	for _, candidate := range candidates {
		if candidate.SchoolID == school && candidate.TemplateIssue == "" {
			ids = append(ids, candidate.RequestChildID)
		}
	}
	slices.Sort(ids)
	reserved := map[int64]bool{}
	for _, id := range ids[max(0, len(ids)-3):] {
		reserved[id] = true
	}
	if changed > 0 {
		reserved[changed] = true
	}
	return school, reserved
}

// seedConclusionAtAcceptance lets the school conclude its contracts at the
// acceptance, after its other candidates got their drafts.
func seedConclusionAtAcceptance(rt *Runtime, schoolID int64) error {
	if schoolID <= 0 {
		return nil
	}
	path := fmt.Sprintf("/operator/schools/%d/settings/values/%s", schoolID, "care_contract.conclusion_mode")
	if _, err := rt.Client.PutWithAuth(rt.OperatorAuth, path, map[string]any{"value": "bei_annahme"}); err != nil {
		return fmt.Errorf("let school %d conclude at the acceptance: %w", schoolID, err)
	}
	if schoolID != rt.Bootstrap.SchoolID {
		return nil // This runtime only holds the enrollment and school session of its own school.
	}
	return seedConclusionEnrollment(rt)
}

// seedCareContractSends sends every second draft, so the list shows both
// states; once half of the contracts are sent it sends nothing more.
func seedCareContractSends(client *Client, auth AuthRef) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contracts/")
	if err != nil {
		return 0, fmt.Errorf("read the contracts: %w", err)
	}
	var envelope struct {
		Data struct {
			Contracts []struct {
				ID     int64  `json:"id"`
				Status string `json:"status"`
			} `json:"contracts"`
			Counts map[string]int `json:"counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the contracts: %w", err)
	}
	total := len(envelope.Data.Contracts)
	missing := (total+1)/2 - envelope.Data.Counts["versendet"]
	ids := []int64{}
	for i, contract := range envelope.Data.Contracts {
		if missing <= len(ids) {
			break
		}
		if contract.Status == "entwurf" && i%2 == 0 {
			ids = append(ids, contract.ID)
		}
	}
	if len(ids) == 0 {
		return 0, nil
	}
	if _, err := client.PostWithAuth(auth, "/traeger/contracts/send", map[string]any{"contract_ids": ids}); err != nil {
		return 0, fmt.Errorf("send the contracts: %w", err)
	}
	return len(ids), nil
}
