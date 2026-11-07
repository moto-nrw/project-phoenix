package api

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedCareContractNotice terminates one active contract as the Träger-Büro
// records a termination it received by mail (#4682), so the notice page,
// the status gekündigt and the confirmation have demo data. A contract is
// active once the Worker concluded it at the acceptance (Weg b); a seed that
// runs before that run finds none and terminates nothing.
func seedCareContractNotice(client *Client, auth AuthRef) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contracts/?status=gekuendigt")
	if err != nil {
		return 0, fmt.Errorf("read the terminated contracts: %w", err)
	}
	var envelope struct {
		Data struct {
			Contracts []struct {
				ID int64 `json:"id"`
			} `json:"contracts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the terminated contracts: %w", err)
	}
	if len(envelope.Data.Contracts) > 0 {
		return 0, nil
	}
	if raw, err = client.GetWithAuth(auth, "/traeger/contracts/?status=aktiv"); err != nil {
		return 0, fmt.Errorf("read the active contracts: %w", err)
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return 0, fmt.Errorf("read the active contracts: %w", err)
	}
	if len(envelope.Data.Contracts) == 0 {
		return 0, nil
	}
	path := "/traeger/contract-notices/" + strconv.FormatInt(envelope.Data.Contracts[0].ID, 10)
	if _, err := client.PostWithAuth(auth, path, map[string]any{
		"kind": "kuendigung", "termination_kind": "ordentlich", "received_on": calendar.TodayDate().String(),
	}); err != nil {
		return 0, fmt.Errorf("record the termination: %w", err)
	}
	return 1, nil
}
