package api

import (
	"encoding/json"
	"fmt"
)

// seedExportHandover enters two recipients of the finance exports of the
// Träger (#6824, ADR 0218; #6911, ADR 0221): the Kämmerei receives the
// Sollstellung and the debit file, the tax office the DATEV booking batch.
// It then hands the Sollstellung of the released holiday run over once.
// Both recipients are in the documentation range 192.0.2.0/24, which the
// address policy refuses before any connection: the seed never dials out,
// and the export page, the protocol and the journal show a handover that
// did not arrive, with its reason.
func seedExportHandover(client *Client, auth AuthRef, runID int64) error {
	existing, err := seededExportRecipients(client, auth)
	if err != nil {
		return err
	}
	kaemmerei, err := seedExportRecipient(client, auth, existing, "Kämmerei", "192.0.2.10", "kaemmerei", "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE")
	if err != nil {
		return err
	}
	steuerbuero, err := seedExportRecipient(client, auth, existing, "Steuerbüro", "192.0.2.20", "steuerbuero", "ZyXwVuTsRqPoNmLkJiHgFeDcBa9876543210+/ZyXwV")
	if err != nil {
		return err
	}
	if _, err := client.PutWithAuth(auth, "/traeger/export-recipient/formats", map[string]any{"assignments": []map[string]any{
		{"format": "sollstellung", "recipient_id": kaemmerei},
		{"format": "datev", "recipient_id": steuerbuero},
		{"format": "pain008", "recipient_id": kaemmerei},
	}}); err != nil {
		return fmt.Errorf("assign the formats to the recipients: %w", err)
	}
	raw, err := client.PostWithAuth(auth, fmt.Sprintf("/traeger/finance-handovers/runs/holiday-runs/%d/sollstellung", runID), map[string]any{})
	if err != nil {
		return fmt.Errorf("hand the Sollstellung over: %w", err)
	}
	var envelope struct {
		Data struct {
			State  string `json:"state"`
			Reason string `json:"reason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("read the handover: %w", err)
	}
	// The API answers a refused handover as a result; any other outcome
	// would mean the seed reached a network.
	if result := envelope.Data; result.State != "" && (result.State != "failed" || result.Reason != "address_denied") {
		return fmt.Errorf("the seed handover must be refused by the address policy, got %s %s", envelope.Data.State, envelope.Data.Reason)
	}
	return nil
}

// seededExportRecipients reads the recipients the Träger already has, by
// name: a second seed run on the same database saves them again instead of
// adding a recipient of a taken name.
func seededExportRecipients(client *Client, auth AuthRef) (map[string]int64, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/export-recipient/")
	if err != nil {
		return nil, fmt.Errorf("read the recipients of the finance exports: %w", err)
	}
	var envelope struct {
		Data struct {
			Recipients []struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
			} `json:"recipients"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("read the recipients of the finance exports: %w", err)
	}
	existing := make(map[string]int64, len(envelope.Data.Recipients))
	for _, recipient := range envelope.Data.Recipients {
		existing[recipient.Name] = recipient.ID
	}
	return existing, nil
}

// seedExportRecipient adds one recipient, or saves the one of that name
// again, and answers its id.
func seedExportRecipient(client *Client, auth AuthRef, existing map[string]int64, name, host, username, fingerprint string) (int64, error) {
	body := map[string]any{
		"name": name, "enabled": true, "host": host, "port": 22, "username": username,
		// A placeholder of the seed, never a credential of a real recipient.
		"password":             "seed-kein-echtes-passwort",
		"remote_directory":     "/eingang/moto",
		"host_key_fingerprint": "SHA256:" + fingerprint,
	}
	var raw []byte
	var err error
	if id, ok := existing[name]; ok {
		raw, err = client.PutWithAuth(auth, fmt.Sprintf("/traeger/export-recipient/%d", id), body)
	} else {
		raw, err = client.PostWithAuth(auth, "/traeger/export-recipient/", body)
	}
	if err != nil {
		return 0, fmt.Errorf("enter the recipient %s of the finance exports: %w", name, err)
	}
	var envelope struct {
		Data struct {
			ID int64 `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil || envelope.Data.ID <= 0 {
		return 0, fmt.Errorf("read the recipient %s: %w", name, err)
	}
	return envelope.Data.ID, nil
}
