package api

import (
	"encoding/json"
	"fmt"
)

// seedExportHandover enters the recipient of the finance exports of the
// Träger (#6824, ADR 0217) and hands the Sollstellung of the released
// holiday run over once. The recipient is in the documentation range
// 192.0.2.0/24, which the address policy refuses before any connection: the
// seed never dials out, and the export page, the protocol and the journal
// show a handover that did not arrive, with its reason.
func seedExportHandover(client *Client, auth AuthRef, runID int64) error {
	if _, err := client.PutWithAuth(auth, "/traeger/export-recipient/", map[string]any{
		"enabled": true, "host": "192.0.2.10", "port": 22, "username": "kaemmerei",
		// A placeholder of the seed, never a credential of a real recipient.
		"password":             "seed-kein-echtes-passwort",
		"remote_directory":     "/eingang/moto",
		"host_key_fingerprint": "SHA256:" + "AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE",
	}); err != nil {
		return fmt.Errorf("enter the recipient of the finance exports: %w", err)
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
	if result := envelope.Data; result.State != "failed" || result.Reason != "address_denied" {
		return fmt.Errorf("the seed handover must be refused by the address policy, got %s %s", envelope.Data.State, envelope.Data.Reason)
	}
	return nil
}
