package api

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// seedImportTries bounds the modules the seed offers the import: the first
// whose phase still runs next month takes the file.
const seedImportTries = 6

// seedImportedCareContracts takes over three children with their existing
// contracts from a CSV export of another program (#4723), so the demo shows
// imported contracts and the import log. The contracts start next month, so
// no monthly run of the seed bills them. A repeated seed finds the children
// again and takes over nothing.
func seedImportedCareContracts(client *Client, auth AuthRef) (int, error) {
	raw, err := client.GetWithAuth(auth, "/traeger/contract-templates/")
	if err != nil {
		return 0, fmt.Errorf("read the contract templates for the import: %w", err)
	}
	var templates struct {
		Data []struct {
			SchoolName   string          `json:"school_name"`
			OfferingName string          `json:"offering_name"`
			Active       bool            `json:"active"`
			Template     json.RawMessage `json:"template"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &templates); err != nil {
		return 0, fmt.Errorf("read the contract templates for the import: %w", err)
	}
	today := calendar.TodayDate()
	start := calendar.NewDate(today.Year(), today.Month()+1, 1).Format("02.01.2006")
	tries := 0
	for _, offering := range templates.Data {
		if !offering.Active || len(offering.Template) == 0 || string(offering.Template) == "null" {
			continue
		}
		if tries++; tries > seedImportTries {
			break
		}
		imported, ready, err := seedCareContractImport(client, auth, offering.SchoolName, offering.OfferingName, start)
		if err != nil || ready {
			return imported, err
		}
	}
	return 0, nil
}

// seedCareContractImport previews the file for one module like the office
// and takes over what the preview showed. ready is false when the module
// takes no row, so the caller offers the next one.
func seedCareContractImport(client *Client, auth AuthRef, school, module, start string) (int, bool, error) {
	rows := []string{"Vorname Kind;Nachname Kind;Geburtsdatum;Schule;Modul;Vertragsbeginn;Vorname Eltern;Nachname Eltern;E-Mail"}
	for _, child := range [][4]string{
		{"Jonte", "Petersen", "14.03.2018", "Maren"},
		{"Malia", "Okafor", "02.09.2019", "Chidi"},
		{"Emil", "Wagner", "27.11.2017", "Sabine"},
	} {
		email := strings.ToLower(child[3]+"."+child[1]) + "@familie.example.test"
		rows = append(rows, strings.Join([]string{child[0], child[1], child[2], school, module, start, child[3], child[1], email}, ";"))
	}
	csv := []byte(strings.Join(rows, "\r\n") + "\r\n")
	const file = "ogs-connect-stammdaten.csv"
	raw, err := client.PostFileWithAuthFields(auth, "/traeger/contract-imports/preview", "file", file, csv, nil)
	if err != nil {
		return 0, false, fmt.Errorf("preview the import of care contracts: %w", err)
	}
	var preview struct {
		Data struct {
			Fingerprint string `json:"fingerprint"`
			Counts      struct {
				Ready     int `json:"ready"`
				Duplicate int `json:"duplicate"`
			} `json:"counts"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &preview); err != nil {
		return 0, false, fmt.Errorf("read the preview of the contract import: %w", err)
	}
	if preview.Data.Counts.Ready == 0 {
		// A repeated seed finds the children again: nothing is left to do.
		return 0, preview.Data.Counts.Duplicate > 0, nil
	}
	if _, err := client.PostFileWithAuthFields(auth, "/traeger/contract-imports/", "file", file, csv,
		map[string]string{"fingerprint": preview.Data.Fingerprint}); err != nil {
		return 0, false, fmt.Errorf("import the care contracts: %w", err)
	}
	return preview.Data.Counts.Ready, true, nil
}
