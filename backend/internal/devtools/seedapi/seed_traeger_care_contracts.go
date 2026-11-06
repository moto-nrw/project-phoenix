package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedTraegerCareContractsStep gives the demo Träger its care contracts
// (#4678, ADR 0153) through the Träger routes, the way the Träger-Büro works:
// a contract template for every active offering of its schools, a draft for
// every accepted application, and every second draft sent. The page
// "Verträge" then shows drafts and sent contracts side by side. Every step
// is idempotent, so several demo schools may seed the same Träger at once.
type seedTraegerCareContractsStep struct{}

func (seedTraegerCareContractsStep) Name() string { return "Seeding the Träger care contracts" }

// seedCareContractTemplate is the contract template of every demo offering.
var seedCareContractTemplate = map[string]any{
	"term_kind": "school_year", "notice_months": 1, "notice_to": "month_end",
	"special_termination": true, "special_termination_note": "Bei Umzug, Schulwechsel oder Verlust des Arbeitsplatzes können Sie zum Ende des Folgemonats kündigen.",
	"statute_required": true, "statute_title": "Satzung über die Elternbeiträge für die Offene Ganztagsschule",
	"statute_url": "https://ganztag-quartier.example.test/satzung",
}

type seedCareContractCandidate struct {
	SchoolID       int64 `json:"school_id"`
	RequestChildID int64 `json:"request_child_id"`
	CareOfferingID int64 `json:"care_offering_id"`
	HasTemplate    bool  `json:"has_template"`
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
	created, err := seedCareContractDrafts(rt.Client, auth)
	if err != nil {
		return err
	}
	sent, err := seedCareContractSends(rt.Client, auth)
	if err != nil {
		return err
	}
	fmt.Printf("  Träger care contracts: %d templates, %d drafts created, %d sent\n", templates, created, sent)
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
func seedCareContractDrafts(client *Client, auth AuthRef) (int, error) {
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
	keys := []map[string]any{}
	for _, candidate := range envelope.Data {
		if candidate.HasTemplate && len(keys) < 200 {
			keys = append(keys, map[string]any{"school_id": candidate.SchoolID, "request_child_id": candidate.RequestChildID,
				"care_offering_id": candidate.CareOfferingID})
		}
	}
	if len(keys) == 0 {
		return 0, nil
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
	return len(created.Data.Created), nil
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
