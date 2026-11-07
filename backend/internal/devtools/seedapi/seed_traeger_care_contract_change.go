package api

import (
	"context"
	"fmt"
	"slices"
	"strings"
)

// seedCareContractChange lets the parents of the first active contract
// whose party has a parents-app account ask to give up the last weekday of
// the modules they choose the days of (#5979). The Träger-Büro then finds an open change on the contract
// page that it can approve or reject itself. It answers whether it asked;
// a seed without such a contract asks nothing.
func seedCareContractChange(rt *Runtime, auth AuthRef) (bool, error) {
	raw, err := rt.Client.GetWithAuth(auth, "/traeger/contracts/?status=aktiv")
	if err != nil {
		return false, fmt.Errorf("list the active care contracts: %w", err)
	}
	var list struct {
		Data struct {
			Contracts []struct {
				ID         int64  `json:"id"`
				PartyEmail string `json:"party_email"`
			} `json:"contracts"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &list); err != nil {
		return false, err
	}
	for _, contract := range list.Data.Contracts {
		parent, ok := seedParentByEmail(rt, contract.PartyEmail)
		if !ok || len(parent.StudentIDs) == 0 {
			continue
		}
		parentAuth, err := rt.Client.adapter.LoginParent(context.Background(), parent.Email, parent.Password)
		if err != nil {
			return false, fmt.Errorf("sign in the parents of contract %d: %w", contract.ID, err)
		}
		return seedOneDayLess(rt, parentAuth, parent.StudentIDs[0])
	}
	return false, nil
}

func seedParentByEmail(rt *Runtime, email string) (ParentCredentials, bool) {
	for _, parent := range slices.Concat(rt.FamilyAccounts, rt.Parents) {
		if email != "" && strings.EqualFold(parent.Email, email) {
			return parent, true
		}
	}
	return ParentCredentials{}, false
}

// seedOneDayLess asks for the child's booked modules without the last
// weekday of each module whose days the parents choose, from the earliest
// day the school allows. Modules with fixed days stay as they are.
func seedOneDayLess(rt *Runtime, parentAuth AuthRef, studentID int64) (bool, error) {
	raw, err := rt.Client.GetWithAuth(parentAuth, fmt.Sprintf("/parent/me/children/%d/care-offerings/catalog", studentID))
	if err != nil {
		return false, fmt.Errorf("read the care offerings of child %d: %w", studentID, err)
	}
	var catalog struct {
		Data struct {
			EarliestEffectiveFrom string `json:"earliest_effective_from"`
			Items                 []struct {
				ID             string   `json:"id"`
				DaysOfWeekMode string   `json:"days_of_week_mode"`
				Selected       bool     `json:"selected"`
				SelectedDays   []string `json:"selected_days"`
				Automatic      bool     `json:"automatic"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := parseJSON(raw, &catalog); err != nil {
		return false, err
	}
	selections := []map[string]any{}
	changed := false
	for _, item := range catalog.Data.Items {
		if !item.Selected || item.Automatic {
			continue
		}
		days := slices.Clone(item.SelectedDays)
		if item.DaysOfWeekMode == "parent_choice" && len(days) > 1 {
			days, changed = days[:len(days)-1], true
		}
		selections = append(selections, map[string]any{"offering_id": item.ID, "selected_days": days})
	}
	if !changed || catalog.Data.EarliestEffectiveFrom == "" {
		return false, nil
	}
	if _, err := rt.Client.PostWithAuth(parentAuth, fmt.Sprintf("/parent/me/children/%d/care-offerings/requests", studentID), map[string]any{
		"offerings": selections, "effective_from": catalog.Data.EarliestEffectiveFrom,
		"note": "An diesem Tag holen wir unser Kind künftig mittags ab.",
	}); err != nil {
		return false, fmt.Errorf("ask for one day less for child %d: %w", studentID, err)
	}
	return true, nil
}
