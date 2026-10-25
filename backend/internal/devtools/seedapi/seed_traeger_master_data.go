package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
)

// seedTraegerMasterDataStep gives the demo Träger its branding (#4653, ADR
// 0112) through the Träger routes, the way the Träger-Büro does it: colour,
// letterhead and footer as Träger settings, a logo, a contact person and two
// bank accounts. The page "Briefkopf" then shows a full letterhead preview.
type seedTraegerMasterDataStep struct{}

func (seedTraegerMasterDataStep) Name() string { return "Seeding the Träger master data" }

// The branding of the demo Träger. Every value is invented.
var (
	seedTraegerBranding = []struct {
		key   string
		value string
	}{
		{"branding.primary_color", "#217A78"},
		{"branding.letterhead_text", "Ganztag im Quartier e.V.\nAm Markt 4\n49143 Bissendorf"},
		{"branding.footer_text", "Vorstand: Anna Brandt, Jonas Weber · Amtsgericht Osnabrück VR 200123 · Steuernummer 66/270/01234"},
	}
	seedTraegerContact      = map[string]any{"name": "Petra Lindner", "phone": "+49 5402 1234-0", "email": "buero@ganztag-quartier.example.test"}
	seedTraegerBankAccounts = []map[string]any{
		{"label": "Elternbeiträge", "account_holder": "Ganztag im Quartier e.V.", "iban": "DE89 3704 0044 0532 0130 00", "bic": "COBADEFFXXX", "bank_name": "Commerzbank", "is_default": true},
		{"label": "Ferienbetreuung", "account_holder": "Ganztag im Quartier e.V.", "iban": "DE02 1203 0000 0000 2020 51", "bic": "BYLADEM1001", "bank_name": "Deutsche Kreditbank"},
	}
)

func (seedTraegerMasterDataStep) Run(_ context.Context, rt *Runtime) error {
	if rt == nil || rt.Client == nil || rt.Bootstrap == nil || rt.Bootstrap.AdminEmail == "" || rt.Bootstrap.AdminPassword == "" {
		return fmt.Errorf("the Träger master data seed prerequisites are not available")
	}
	auth, err := seedTraegerSession(rt)
	if err != nil {
		return err
	}
	for _, setting := range seedTraegerBranding {
		if _, err := rt.Client.PutWithAuth(auth, "/traeger/settings/values/"+setting.key, map[string]any{"value": setting.value}); err != nil {
			return fmt.Errorf("set the Träger value of %s: %w", setting.key, err)
		}
	}
	if _, err := rt.Client.PutWithAuth(auth, "/traeger/master-data/contact", seedTraegerContact); err != nil {
		return fmt.Errorf("set the Träger contact person: %w", err)
	}
	logo, err := seedTraegerLogo()
	if err != nil {
		return err
	}
	if _, err := rt.Client.PutFileWithAuth(auth, "/traeger/master-data/logo", "logo", "logo.png", logo); err != nil {
		return fmt.Errorf("upload the Träger logo: %w", err)
	}
	if err := seedTraegerBankAccountsOnce(rt.Client, auth); err != nil {
		return err
	}
	fmt.Println("  Träger master data: letterhead, logo, contact person and two bank accounts")
	return nil
}

// seedTraegerBankAccountsOnce adds the bank accounts the Träger does not
// have yet; their labels are unique per Träger.
func seedTraegerBankAccountsOnce(client *Client, auth AuthRef) error {
	raw, err := client.GetWithAuth(auth, "/traeger/master-data/")
	if err != nil {
		return fmt.Errorf("read the Träger master data: %w", err)
	}
	var envelope struct {
		Data struct {
			BankAccounts []struct {
				Label string `json:"label"`
			} `json:"bank_accounts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("read the Träger bank accounts: %w", err)
	}
	existing := map[string]bool{}
	for _, account := range envelope.Data.BankAccounts {
		existing[account.Label] = true
	}
	for _, account := range seedTraegerBankAccounts {
		if existing[account["label"].(string)] {
			continue
		}
		if _, err := client.PostWithAuth(auth, "/traeger/master-data/bank-accounts", account); err != nil {
			return fmt.Errorf("add the Träger bank account %s: %w", account["label"], err)
		}
	}
	return nil
}

// seedTraegerLogo draws a simple wordmark-sized PNG: a petrol field with a
// green bar, so the preview shows where the logo sits.
func seedTraegerLogo() ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 240, 80))
	petrol := color.RGBA{R: 0x21, G: 0x7A, B: 0x78, A: 0xFF}
	green := color.RGBA{R: 0x83, G: 0xCD, B: 0x2D, A: 0xFF}
	for y := range 80 {
		for x := range 240 {
			switch {
			case y >= 60 && x >= 20 && x < 220:
				img.Set(x, y, green)
			case x < 80:
				img.Set(x, y, petrol)
			default:
				img.Set(x, y, color.White)
			}
		}
	}
	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		return nil, fmt.Errorf("draw the Träger logo: %w", err)
	}
	return out.Bytes(), nil
}
