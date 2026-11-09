package api

import (
	"context"
	"encoding/json"
	"fmt"
)

// seedParentSEPAMandateStep lets a demo parent give the SEPA mandate of the
// family in the parents portal (#4696): the demo Träger allows the online
// mandate, and the first parent whose contract has no mandate in force
// previews the text and confirms exactly that text. It runs after the
// monthly run, which gave the Träger its creditor identifier.
type seedParentSEPAMandateStep struct{}

func (seedParentSEPAMandateStep) Name() string { return "Giving a SEPA mandate in the parents portal" }

// seedOnlineMandateSettingKey is the Träger switch of the online mandate.
const seedOnlineMandateSettingKey = "parent_billing.online_mandate"

func (seedParentSEPAMandateStep) Run(ctx context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the parents' SEPA mandate seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+seedOnlineMandateSettingKey, map[string]any{"value": true}); err != nil {
		return fmt.Errorf("allow the online SEPA mandate: %w", err)
	}
	for _, parent := range rt.Parents {
		parentAuth, err := rt.Adapter.LoginParent(ctx, parent.Email, parent.Password)
		if err != nil {
			return fmt.Errorf("log in parent %s: %w", parent.Email, err)
		}
		given, err := seedParentMandateOf(rt.Client, parentAuth, parent.Email)
		if err != nil {
			return err
		}
		if given {
			fmt.Printf("  Parents portal: %s gave the SEPA mandate of the family\n", parent.Email)
			return nil
		}
	}
	fmt.Println("  Parents portal: no contract without a SEPA mandate to give one for")
	return nil
}

// seedParentMandateView is the part of the mandate view the seed reads.
type seedParentMandateView struct {
	Data struct {
		CanManage       bool            `json:"can_manage"`
		CreditorMissing bool            `json:"creditor_missing"`
		Mandate         json.RawMessage `json:"mandate"`
		Prefill         struct {
			AccountHolder string `json:"account_holder"`
		} `json:"prefill"`
	} `json:"data"`
}

// seedParentMandateOf gives the mandate on the first contract of the
// parent's inbox that may take one; false when none may.
func seedParentMandateOf(client *Client, auth AuthRef, email string) (bool, error) {
	raw, err := client.GetWithAuth(auth, "/parent/me/documents/contracts")
	if err != nil {
		// A parent without the inbox right has no contracts.
		fmt.Printf("    parent %s has no contract inbox: %v\n", email, err)
		return false, nil
	}
	var inbox struct {
		Data []struct {
			ID             string `json:"id"`
			OrganizationID string `json:"organization_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &inbox); err != nil {
		return false, fmt.Errorf("read the contracts of parent %s: %w", email, err)
	}
	for _, contract := range inbox.Data {
		path := fmt.Sprintf("/parent/me/documents/%s/contracts/%s/sepa-mandate", contract.OrganizationID, contract.ID)
		holder, open := seedOpenMandate(client, auth, path)
		if !open {
			continue
		}
		if err := seedGrantMandate(client, auth, path, holder); err != nil {
			return false, fmt.Errorf("give the SEPA mandate of contract %s: %w", contract.ID, err)
		}
		return true, nil
	}
	return false, nil
}

// seedOpenMandate tells whether the contract takes a new mandate from the
// parent, with the proposed account holder.
func seedOpenMandate(client *Client, auth AuthRef, path string) (string, bool) {
	raw, err := client.GetWithAuth(auth, path)
	if err != nil {
		return "", false
	}
	var view seedParentMandateView
	if json.Unmarshal(raw, &view) != nil {
		return "", false
	}
	inForce := len(view.Data.Mandate) > 0 && string(view.Data.Mandate) != "null"
	if !view.Data.CanManage || view.Data.CreditorMissing || inForce {
		return "", false
	}
	if view.Data.Prefill.AccountHolder == "" {
		return "Erika Muster", true
	}
	return view.Data.Prefill.AccountHolder, true
}

// seedGrantMandate previews the text and confirms exactly that text, like
// the parents do.
func seedGrantMandate(client *Client, auth AuthRef, path, holder string) error {
	input := map[string]any{"account_holder": holder, "iban": demoIBAN(9300), "street": "Lindenstraße 12",
		"postal_code": "75175", "city": "Pforzheim"}
	raw, err := client.PostWithAuth(auth, path+"/preview", input)
	if err != nil {
		return fmt.Errorf("read the mandate text: %w", err)
	}
	var text struct {
		Data struct {
			Reference    string `json:"reference"`
			TextSHA256   string `json:"text_sha256"`
			AccountToken string `json:"account_token"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &text); err != nil {
		return fmt.Errorf("read the mandate text: %w", err)
	}
	input["reference"], input["text_sha256"], input["holder_confirmed"] = text.Data.Reference, text.Data.TextSHA256, true
	input["account_token"] = text.Data.AccountToken
	_, err = client.PostWithAuth(auth, path, input)
	return err
}
