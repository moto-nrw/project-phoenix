package api

import (
	"encoding/json"
	"fmt"
)

// Seed distinct, readable PDFs through the same upload API the editor uses.
func seedEnrollmentAttachments(rt *Runtime) ([]map[string]string, error) {
	result := make([]map[string]string, 0, 2)
	for _, document := range []struct {
		label, filename string
		lines           []string
	}{
		{"Informationen zum Schulstart", "schulstart.pdf", []string{"Willkommen in der OGS.", "Die Betreuung beginnt nach dem Unterricht.", "Bitte geben Sie Ihrem Kind eine Trinkflasche mit."}},
		{"Abholen und Kontakt", "abholen.pdf", []string{"Bitte melden Sie Ihr Kind beim Abholen ab.", "Teilen Sie uns Änderungen der Abholzeiten rechtzeitig mit."}},
	} {
		raw, err := rt.Client.PostFile("/api/enrollment/legal-documents", "document", document.filename, demoPDF(document.label, document.lines))
		if err != nil {
			return nil, fmt.Errorf("upload enrollment information PDF: %w", err)
		}
		var response struct {
			Data struct {
				DocumentURL string `json:"document_url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(raw, &response); err != nil {
			return nil, fmt.Errorf("decode enrollment PDF upload: %w", err)
		}
		if response.Data.DocumentURL == "" {
			return nil, fmt.Errorf("enrollment PDF upload returned no document URL")
		}
		result = append(result, map[string]string{"label": document.label, "document_url": response.Data.DocumentURL})
	}
	return result, nil
}
