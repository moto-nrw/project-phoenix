package api

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The children of the families (#5918): the Abrechnung confirms a number for
// the first two families without one, a family of three and one whose
// confirmation ends with the next month, so the page „Kinderzahl“ shows a
// confirmed number and one that asks for a check on every dev machine. The
// placeholder sibling rule counts the family (seedCareReductions).

type seedFamilyChildrenSet struct {
	Families []struct {
		Children []struct {
			ContractID int64 `json:"contract_id"`
		} `json:"children"`
		Count *struct {
			ID int64 `json:"id"`
		} `json:"count"`
	} `json:"families"`
}

// seedFamilyChildren confirms the numbers and answers how many it kept.
func seedFamilyChildren(client *Client, auth AuthRef, month calendar.Date) (int, error) {
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
		if family.Count != nil || len(family.Children) == 0 {
			continue
		}
		body := numbers[kept]
		body["contract_id"] = family.Children[0].ContractID
		if _, err := client.PutWithAuth(auth, "/traeger/family-children/", body); err != nil {
			return kept, fmt.Errorf("confirm the children of a family: %w", err)
		}
		kept++
	}
	return kept, nil
}
