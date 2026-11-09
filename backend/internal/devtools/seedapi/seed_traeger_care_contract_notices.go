package api

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type seedContractNoticeOptions struct {
	ReinterpretedEndsOn *calendar.Date `json:"reinterpreted_ends_on"`
	Notice              *struct {
		TerminationKind string `json:"termination_kind"`
		EndDecision     string `json:"end_decision"`
	} `json:"notice"`
}

// seedCareContractNotice leaves an ordinary termination and three extraordinary
// cases: still open, end set, and reason refused. If the Worker has not yet
// concluded enough contracts, use signed paper fixtures through the same API
// as the signature seed. Repeated seeds fill only missing cases, never decide an
// existing open notice or consume Lina Amini's amendment example.
func seedCareContractNotice(client *Client, auth AuthRef) (int, error) {
	present, err := seedNoticeCasesPresent(client, auth)
	if err != nil {
		return 0, err
	}
	active, err := seedNoticeContracts(client, auth, "aktiv")
	if err != nil {
		return 0, err
	}
	drafts, err := seedNoticeContracts(client, auth, "entwurf")
	if err != nil {
		return 0, err
	}
	for i := range drafts {
		drafts[i].NeedsSignature = true
	}
	created := 0
	for _, contract := range append(active, drafts...) {
		if (contract.ChildFirstName == "Lina" && contract.ChildLastName == "Amini") ||
			(contract.ChildFirstName == "Tilda" && contract.ChildLastName == "Berger") {
			continue
		}
		next := nextSeedNoticeCase(present)
		if next == "" {
			break
		}
		if err := seedNoticeCase(client, auth, contract, next); err != nil {
			return created, err
		}
		present[next] = true
		created++
	}
	return created, nil
}

func seedNoticeCasesPresent(client *Client, auth AuthRef) (map[string]bool, error) {
	present := map[string]bool{}
	terminated, err := seedNoticeContracts(client, auth, "gekuendigt")
	if err != nil {
		return nil, err
	}
	for _, contract := range terminated {
		raw, err := client.GetWithAuth(auth, seedNoticePath(contract.ID))
		if err != nil {
			return nil, err
		}
		options, err := seedNoticeOptions(raw)
		if err != nil {
			return nil, err
		}
		if options.Notice == nil {
			continue
		}
		switch options.Notice.TerminationKind {
		case "ordentlich":
			present["ordentlich"] = true
		case "ausserordentlich":
			decision := options.Notice.EndDecision
			if decision == "" {
				decision = "offen"
			}
			present[decision] = true
		}
	}

	return present, nil
}

func nextSeedNoticeCase(present map[string]bool) string {
	for _, candidate := range []string{"ordentlich", "offen", "festgelegt", "umgedeutet"} {
		if !present[candidate] {
			return candidate
		}
	}
	return ""
}

type seedNoticeContract struct {
	NeedsSignature bool   `json:"-"`
	PartyName      string `json:"party_name"`
	ID             int64  `json:"id"`
	ChildFirstName string `json:"child_first_name"`
	ChildLastName  string `json:"child_last_name"`
}

func seedNoticeContracts(client *Client, auth AuthRef, status string) ([]seedNoticeContract, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contracts/?status="+status)
	if err != nil {
		return nil, fmt.Errorf("read the %s contracts: %w", status, err)
	}
	var envelope struct {
		Data struct {
			Contracts []seedNoticeContract `json:"contracts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, err
	}
	return envelope.Data.Contracts, nil
}

func seedNoticePath(id int64) string { return "/traeger/contract-notices/" + strconv.FormatInt(id, 10) }

func seedNoticeOptions(raw []byte) (seedContractNoticeOptions, error) {
	var envelope struct {
		Data seedContractNoticeOptions `json:"data"`
	}
	err := json.Unmarshal(raw, &envelope)
	return envelope.Data, err
}

func seedNoticeCase(client *Client, auth AuthRef, contract seedNoticeContract, decision string) error {
	id := contract.ID
	if contract.NeedsSignature {
		if err := seedPaperContractSignature(client, auth, id, contract.PartyName); err != nil {
			return err
		}
	}
	today := calendar.TodayDate()
	body := map[string]any{"kind": "kuendigung", "termination_kind": "ordentlich", "received_on": today.String()}
	if decision != "ordentlich" {
		body["termination_kind"], body["reason"] = "ausserordentlich", "Wir ziehen um."
	}
	path := seedNoticePath(id)
	raw, err := client.PostWithAuth(auth, path, body)
	if err != nil {
		return fmt.Errorf("record the %s termination: %w", decision, err)
	}
	switch decision {
	case "festgelegt":
		_, err = client.PutWithAuth(auth, path+"/ends-on", map[string]any{"ends_on": today.String()})
	case "umgedeutet":
		options, readErr := seedNoticeOptions(raw)
		if readErr != nil {
			return readErr
		}
		if options.ReinterpretedEndsOn == nil {
			return fmt.Errorf("contract %d has no ordinary end for the refusal", id)
		}
		_, err = client.PostWithAuth(auth, path+"/reason-rejection", map[string]any{"expected_ends_on": options.ReinterpretedEndsOn.String()})
	}
	if err != nil {
		return fmt.Errorf("decide the %s termination: %w", decision, err)
	}
	return nil
}
