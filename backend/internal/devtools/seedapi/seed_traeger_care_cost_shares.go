package api

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The cost bearer shares of the monthly contribution (#4687): the Jugendamt
// takes the whole contribution of one child and Bildung und Teilhabe a fixed
// part of another's, whose decision ends in three weeks, so the page
// „Kostenträger“, the warning before the end and the shares on the invoices
// and in the lists are filled on every dev machine. The cost bearers and the
// references are invented.

const seedCareCostShareOverlap = "parent_billing.care_cost_share_overlap"

type seedCareCostShareContract struct {
	ContractID int64             `json:"contract_id"`
	SchoolID   int64             `json:"school_id"`
	Shares     []json.RawMessage `json:"shares"`
}

type seedCareCostShareSet struct {
	Contracts   []seedCareCostShareContract `json:"contracts"`
	CostBearers []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		Active bool   `json:"active"`
	} `json:"cost_bearers"`
}

// seedCareCostShares keeps a cost bearer for Bildung und Teilhabe and gives
// the third and fourth contract of the seeded school one share each. A
// school whose contracts already have shares keeps them; the contracts of
// the Träger's other schools stay untouched.
func seedCareCostShares(client *Client, auth AuthRef, school int64, month calendar.Date) error {
	raw, err := client.GetWithAuth(auth, "/traeger/care-cost-shares/")
	if err != nil {
		return fmt.Errorf("read the care cost shares: %w", err)
	}
	set, err := seedReadData[seedCareCostShareSet](raw, "care cost shares")
	if err != nil {
		return err
	}
	set.Contracts = slices.DeleteFunc(set.Contracts, func(contract seedCareCostShareContract) bool { return contract.SchoolID != school })
	if len(set.Contracts) < 4 {
		return nil
	}
	for _, contract := range set.Contracts {
		if len(contract.Shares) > 0 {
			return nil
		}
	}
	youthOffice, err := seedCostBearer(client, auth, set, "Fachdienst Jugend des Landkreises Musterstadt", "FJ 51-2027")
	if err != nil {
		return err
	}
	jobcenter, err := seedCostBearer(client, auth, set, "Jobcenter Musterstadt, Bildung und Teilhabe", "BuT 2026")
	if err != nil {
		return err
	}
	today := calendar.TodayDate()
	shares := []map[string]any{
		{"contract_id": set.Contracts[2].ContractID, "kind": "youth_office", "cost_bearer_id": youthOffice,
			"share_basis_points": 10000, "valid_until": calendar.NewDate(month.Year()+1, 7, 31).String(),
			"reference": "Bescheid FJ 51-2027-0042"},
		{"contract_id": set.Contracts[3].ContractID, "kind": "education_participation", "cost_bearer_id": jobcenter,
			"share_cents": 2500, "valid_until": today.AddDays(20).String(), "reference": "BuT-Bescheid Az. 2026-117"},
	}
	for _, share := range shares {
		share["valid_from"], share["checked_on"] = month.String(), today.String()
		if _, err := client.PostWithAuth(auth, "/traeger/care-cost-shares/", share); err != nil && !seedConflictCode(err, seedCareCostShareOverlap) {
			return fmt.Errorf("keep a care cost share: %w", err)
		}
	}
	fmt.Println("  Träger monthly contributions: the Jugendamt takes one contribution, Bildung und Teilhabe a part of another")
	return nil
}

// seedCostBearer answers the active cost bearer of that name, or keeps a new
// one.
func seedCostBearer(client *Client, auth AuthRef, set seedCareCostShareSet, name, reference string) (int64, error) {
	for _, bearer := range set.CostBearers {
		if bearer.Name == name && bearer.Active {
			return bearer.ID, nil
		}
	}
	raw, err := client.PostWithAuth(auth, "/traeger/cost-bearers/", map[string]any{
		"name": name, "address": "Am Kreishaus 1\n12345 Musterstadt", "file_reference": reference, "active": true,
	})
	if err != nil {
		return 0, fmt.Errorf("keep the cost bearer %q: %w", name, err)
	}
	id, err := decodeSeedEntityID(raw)
	if err != nil {
		return 0, fmt.Errorf("read the cost bearer %q: %w", name, err)
	}
	return id, nil
}
