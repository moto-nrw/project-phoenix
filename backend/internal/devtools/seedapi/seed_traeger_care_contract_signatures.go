package api

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// seedTraegerCareContractSignaturesStep signs the demo care contracts
// (#4679, ADR 0155) the way families and the Träger-Büro do: a parent with
// an account signs a contract sent to them in the parents portal, and the
// Träger-Büro records one draft the parents signed on paper. The contract
// list then shows signed, sent and draft contracts side by side. Both steps
// are idempotent: a signed contract is no longer sent, a paper contract no
// longer a draft.
type seedTraegerCareContractSignaturesStep struct{}

func (seedTraegerCareContractSignaturesStep) Name() string {
	return "Signing the Träger care contracts"
}

// seedPaperContractPDF stands in for the scan of a signed paper contract.
var seedPaperContractPDF = []byte("%PDF-1.4\n% Betreuungsvertrag, auf Papier unterschrieben (Demo)\n%%EOF\n")

func (seedTraegerCareContractSignaturesStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger care contract signature seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	paper, err := seedPaperContract(rt.Client, auth)
	if err != nil {
		return err
	}
	if err := seedSendContractsToParents(rt, auth); err != nil {
		return err
	}
	signed, err := seedParentContractSignatures(ctx, rt)
	if err != nil {
		return err
	}
	fmt.Printf("  Träger care contract signatures: %d on paper, %d in the parents portal\n", paper, signed)
	return nil
}

// seedPaperContract records the first draft as signed on paper, unless a
// paper contract exists already.
func seedPaperContract(client *Client, auth AuthRef) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contracts/?status=aktiv")
	if err != nil {
		return 0, fmt.Errorf("read the active contracts: %w", err)
	}
	var active struct {
		Data struct {
			Counts map[string]int `json:"counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &active); err != nil {
		return 0, fmt.Errorf("read the active contracts: %w", err)
	}
	if active.Data.Counts["aktiv"] > 0 {
		return 0, nil
	}
	raw, err = client.GetWithAuth(auth, "/traeger/contracts/?status=entwurf")
	if err != nil {
		return 0, fmt.Errorf("read the drafts: %w", err)
	}
	var drafts struct {
		Data struct {
			Contracts []struct {
				ID        int64  `json:"id"`
				PartyName string `json:"party_name"`
			} `json:"contracts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &drafts); err != nil {
		return 0, fmt.Errorf("read the drafts: %w", err)
	}
	if len(drafts.Data.Contracts) == 0 {
		return 0, nil
	}
	draft := drafts.Data.Contracts[len(drafts.Data.Contracts)-1]
	if err := seedPaperContractSignature(client, auth, draft.ID, draft.PartyName); err != nil {
		return 0, err
	}
	return 1, nil
}

func seedPaperContractSignature(client *Client, auth AuthRef, id int64, signer string) error {
	if signer == "" {
		signer = "Erika Muster"
	}
	path := fmt.Sprintf("/traeger/contracts/%d/paper-signature", id)
	if _, err := client.PostFileWithAuthAndFields(auth, path, "file", "vertrag-unterschrieben.pdf", seedPaperContractPDF,
		map[string]string{"signer_name": signer, "signed_on": calendar.TodayDate().String()}); err != nil {
		return fmt.Errorf("record the paper contract %d: %w", id, err)
	}
	return nil
}

// seedParentContractSignatures lets the first parent whose inbox holds a
// contract waiting for its signature sign it with a drawn signature.
func seedParentContractSignatures(ctx context.Context, rt *Runtime) (int, error) {
	for _, parent := range rt.Parents {
		auth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return 0, fmt.Errorf("log in parent %s: %w", parent.Email, err)
		}
		raw, err := rt.Client.GetWithAuth(auth, "/parent/me/documents/contracts")
		if err != nil {
			// A parent without the inbox right has no contracts to sign.
			fmt.Printf("    parent %s has no contract inbox: %v\n", parent.Email, err)
			continue
		}
		var inbox struct {
			Data []struct {
				ID             string `json:"id"`
				OrganizationID string `json:"organization_id"`
				ContractStatus string `json:"contract_status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &inbox); err != nil {
			return 0, fmt.Errorf("read the contracts of parent %s: %w", parent.Email, err)
		}
		for _, contract := range inbox.Data {
			if contract.ContractStatus != "versendet" {
				continue
			}
			path := fmt.Sprintf("/parent/me/documents/%s/contracts/%s/sign", contract.OrganizationID, contract.ID)
			if _, err := rt.Client.PostWithAuth(auth, path, map[string]any{
				"method": "drawn", "image_png": demoSignaturePNG(parent.Email),
				"withdrawal_notice_acknowledged": true, "early_start_requested": true,
			}); err != nil {
				// The relationship may grant reading but not signing.
				fmt.Printf("    parent %s cannot sign contract %s: %v\n", parent.Email, contract.ID, err)
				continue
			}
			return 1, nil
		}
	}
	return 0, nil
}

// seedSendContractsToParents sends the drafts addressed to the demo parents,
// so one of them finds a contract to sign in the parents portal.
func seedSendContractsToParents(rt *Runtime, auth AuthRef) error {
	emails := map[string]bool{}
	for _, parent := range rt.Parents {
		emails[strings.ToLower(parent.Email)] = true
	}
	raw, err := rt.Client.GetWithAuth(auth, "/traeger/contracts/?status=entwurf&limit=100")
	if err != nil {
		return fmt.Errorf("read the drafts: %w", err)
	}
	var drafts struct {
		Data struct {
			Contracts []struct {
				ID         int64  `json:"id"`
				PartyEmail string `json:"party_email"`
			} `json:"contracts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &drafts); err != nil {
		return fmt.Errorf("read the drafts: %w", err)
	}
	ids := []int64{}
	for _, draft := range drafts.Data.Contracts {
		if emails[strings.ToLower(draft.PartyEmail)] {
			ids = append(ids, draft.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	if _, err := rt.Client.PostWithAuth(auth, "/traeger/contracts/send", map[string]any{"contract_ids": ids}); err != nil {
		return fmt.Errorf("send the contracts of the demo parents: %w", err)
	}
	return nil
}
