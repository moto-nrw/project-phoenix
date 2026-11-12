package api

import (
	"fmt"
	"slices"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The children of the families (#5918): the Abrechnung confirms a number for
// the first two families without one, a family of three and one whose
// confirmation ends with the next month, so the page „Kinderzahl“ shows a
// confirmed number and one that asks for a check on every dev machine. The
// placeholder sibling rule counts the family (seedCareReductions).

type seedFamilyChild struct {
	ContractID int64 `json:"contract_id"`
}

type seedFamilyChildrenSet struct {
	Families []struct {
		Children []seedFamilyChild `json:"children"`
		Count    *struct {
			ID int64 `json:"id"`
		} `json:"count"`
	} `json:"families"`
}

// seedFamilyChildren confirms the numbers of families with a contract of
// the seeded school (own) and answers how many it kept. Families of the
// Träger's other schools stay untouched.
func seedFamilyChildren(client *Client, auth AuthRef, own []int64, month calendar.Date) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/family-children/")
	if err != nil {
		return 0, fmt.Errorf("read the family children: %w", err)
	}
	set, err := seedReadData[seedFamilyChildrenSet](raw, "family children")
	if err != nil {
		return 0, err
	}
	nextMonthEnd := calendar.NewDate(month.Year(), month.Month()+2, 1).AddDays(-1)
	numbers := []map[string]any{
		{"children": 3, "valid_from": month.String()},
		{"children": 2, "valid_from": month.String(), "valid_until": nextMonthEnd.String()},
	}
	kept := 0
	for _, family := range set.Families {
		if kept == len(numbers) {
			break
		}
		child := slices.IndexFunc(family.Children, func(child seedFamilyChild) bool { return slices.Contains(own, child.ContractID) })
		if family.Count != nil || child < 0 {
			continue
		}
		body := numbers[kept]
		body["contract_id"] = family.Children[child].ContractID
		if _, err := client.PutWithAuth(auth, "/traeger/family-children/", body); err != nil {
			return kept, fmt.Errorf("confirm the children of a family: %w", err)
		}
		kept++
	}
	return kept, nil
}
