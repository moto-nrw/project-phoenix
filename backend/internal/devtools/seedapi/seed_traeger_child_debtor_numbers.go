package api

import (
	"encoding/json"
	"fmt"
	"slices"
)

// The debtor number of a child (#7007). The demo Träger keeps its numbers
// per payer, as Bissendorf does; the seed prepares a number for each child
// of the first families, siblings under contract first, so switching
// „Debitorennummer gilt je“ to „Kind“ shows Abrechnung → Familien with the
// children and their own numbers at once, and the next monthly run copies
// them. The numbers have five digits like the other seeded numbers
// (seedDebtorNumberFormat).

// seedChildDebtorNumberFamilies is how many families with siblings get a
// number per child.
const seedChildDebtorNumberFamilies = 2

// seedChildFamily is a family of the list of children as the seed reads it.
type seedChildFamily struct {
	FamilyEmail string `json:"family_email"`
	Children    []struct {
		FirstName string  `json:"first_name"`
		LastName  string  `json:"last_name"`
		BirthDate *string `json:"birth_date"`
	} `json:"children"`
}

// seedChildDebtorNumbers sets the numbers of the children of the first
// families with several children through the list of families. A second
// seed run sets the same numbers again.
func seedChildDebtorNumbers(client *Client, auth AuthRef) error {
	raw, err := client.GetWithAuth(auth, "/traeger/billing-families/children")
	if err != nil {
		return fmt.Errorf("read the children of the families: %w", err)
	}
	var envelope struct {
		Data []seedChildFamily `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("read the children of the families: %w", err)
	}
	// Siblings first; a demo without siblings under contract takes the
	// first families.
	slices.SortStableFunc(envelope.Data, func(a, b seedChildFamily) int { return len(b.Children) - len(a.Children) })
	families, number := 0, 60100
	for _, family := range envelope.Data {
		if len(family.Children) == 0 || families == seedChildDebtorNumberFamilies {
			continue
		}
		families++
		for _, child := range family.Children {
			number++
			if _, err := client.PutWithAuth(auth, "/traeger/billing-families/child-debtor-number", map[string]any{
				"family_email": family.FamilyEmail, "child_first_name": child.FirstName, "child_last_name": child.LastName,
				"child_birth_date": child.BirthDate, "debtor_number": fmt.Sprintf("%05d", number),
			}); err != nil {
				return fmt.Errorf("keep the debtor number of a child: %w", err)
			}
		}
	}
	fmt.Printf("  Debitorennummer je Kind: %d families with a number per child prepared\n", families)
	return nil
}
