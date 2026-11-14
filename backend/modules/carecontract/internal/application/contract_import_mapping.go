package application

import (
	"slices"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/textnorm"
)

// The mapping of an import of existing contracts (#4723): which column of
// the file gives which field of a contract, and the header names moto maps
// by itself.

// contractImportFields are the fields in the order the suggestion gives a
// header away: a header names one field only, the first that knows it.
var contractImportFields = []string{
	carecontract.ContractImportFieldChildFirstName, carecontract.ContractImportFieldChildLastName,
	carecontract.ContractImportFieldChildBirthDate, carecontract.ContractImportFieldSchool,
	carecontract.ContractImportFieldModule, carecontract.ContractImportFieldWeekdays,
	carecontract.ContractImportFieldEffectiveFrom, carecontract.ContractImportFieldEndsOn,
	carecontract.ContractImportFieldConcludedOn, carecontract.ContractImportFieldPartyFirstName,
	carecontract.ContractImportFieldPartyLastName, carecontract.ContractImportFieldPartyEmail,
	carecontract.ContractImportFieldDebtorNumber,
}

// contractImportRequired are the fields every import needs a column for;
// the first day, the school and the module need a column or a choice.
var contractImportRequired = []string{
	carecontract.ContractImportFieldChildFirstName, carecontract.ContractImportFieldChildLastName,
}

// contractImportHeaderNames are the header names moto maps without help,
// after textnorm.HeaderKey folded them. The signing date of a SEPA mandate
// ("Unterschriftsdatum") is not the conclusion of the contract and stays
// out.
var contractImportHeaderNames = map[string][]string{
	carecontract.ContractImportFieldChildFirstName: {"vornamekind", "vornamedeskindes", "kindvorname", "vorname", "rufname", "firstname"},
	carecontract.ContractImportFieldChildLastName: {"nachnamekind", "nachnamedeskindes", "kindnachname", "nachname", "familienname",
		"name", "lastname"},
	carecontract.ContractImportFieldChildBirthDate: {"geburtsdatum", "geburtsdatumkind", "geburtsdatumdeskindes", "geburtstag", "gebdatum",
		"geborenam", "birthdate", "dateofbirth"},
	carecontract.ContractImportFieldSchool: {"schule", "schulname", "einrichtung", "standort", "betreuungsort", "ogs", "school"},
	carecontract.ContractImportFieldModule: {"modul", "module", "angebot", "betreuungsangebot", "betreuungsmodul", "betreuungsform",
		"betreuungsart", "betreuungsumfang", "tarif"},
	carecontract.ContractImportFieldWeekdays: {"wochentage", "betreuungstage", "gebuchtetage", "tage", "weekdays"},
	carecontract.ContractImportFieldEffectiveFrom: {"vertragsbeginn", "betreuungsbeginn", "beginn", "gueltigab", "vertraggueltigab",
		"wirkdatum", "startdatum", "eintritt", "eintrittsdatum", "aufnahmedatum", "betreuungab", "effectivefrom"},
	carecontract.ContractImportFieldEndsOn: {"vertragsende", "betreuungsende", "ende", "gueltigbis", "vertraggueltigbis", "enddatum",
		"austritt", "austrittsdatum", "betreuungbis", "endson"},
	carecontract.ContractImportFieldConcludedOn: {"vertragsdatum", "abschlussdatum", "vertragsabschluss", "vertragabgeschlossenam",
		"abgeschlossenam", "concludedon"},
	carecontract.ContractImportFieldPartyFirstName: {"vornameeltern", "elternvorname", "vornameerziehungsberechtigte",
		"vornamesorgeberechtigte", "vornamevertragspartner", "vornamezahler", "zahlervorname"},
	carecontract.ContractImportFieldPartyLastName: {"nachnameeltern", "elternnachname", "nachnameerziehungsberechtigte",
		"nachnamesorgeberechtigte", "nachnamevertragspartner", "vertragspartner", "nachnamezahler", "zahlernachname", "zahlername",
		"zahler", "kontoinhaber"},
	carecontract.ContractImportFieldPartyEmail: {"email", "emailadresse", "mail", "emaileltern", "emaildereltern", "emailzahler",
		"emailerziehungsberechtigte", "emailvertragspartner"},
	carecontract.ContractImportFieldDebtorNumber: {"debitorennummer", "kinderdebitorennummer", "debitornummer", "debitor",
		"debitorennr", "debitornr", "kundennummer", "personenkonto", "debtornumber"},
}

// suggestContractImportMapping maps each field to the first header with one
// of its names. A header names one field only.
func suggestContractImportMapping(headers []string) carecontract.ContractImportMapping {
	found := map[string]string{}
	for _, header := range headers {
		key := textnorm.HeaderKey(header)
		if key == "" {
			continue
		}
		for _, field := range contractImportFields {
			if _, done := found[field]; !done && slices.Contains(contractImportHeaderNames[field], key) {
				found[field] = header
				break
			}
		}
	}
	return carecontract.ContractImportMapping{Columns: carecontract.ContractImportColumns{
		ChildFirstName: found[carecontract.ContractImportFieldChildFirstName],
		ChildLastName:  found[carecontract.ContractImportFieldChildLastName],
		ChildBirthDate: found[carecontract.ContractImportFieldChildBirthDate],
		School:         found[carecontract.ContractImportFieldSchool],
		Module:         found[carecontract.ContractImportFieldModule],
		Weekdays:       found[carecontract.ContractImportFieldWeekdays],
		EffectiveFrom:  found[carecontract.ContractImportFieldEffectiveFrom],
		EndsOn:         found[carecontract.ContractImportFieldEndsOn],
		ConcludedOn:    found[carecontract.ContractImportFieldConcludedOn],
		PartyFirstName: found[carecontract.ContractImportFieldPartyFirstName],
		PartyLastName:  found[carecontract.ContractImportFieldPartyLastName],
		PartyEmail:     found[carecontract.ContractImportFieldPartyEmail],
		DebtorNumber:   found[carecontract.ContractImportFieldDebtorNumber],
	}}
}

func contractImportHeaders(columns carecontract.ContractImportColumns) map[string]string {
	return map[string]string{
		carecontract.ContractImportFieldChildFirstName: columns.ChildFirstName,
		carecontract.ContractImportFieldChildLastName:  columns.ChildLastName,
		carecontract.ContractImportFieldChildBirthDate: columns.ChildBirthDate,
		carecontract.ContractImportFieldSchool:         columns.School,
		carecontract.ContractImportFieldModule:         columns.Module,
		carecontract.ContractImportFieldWeekdays:       columns.Weekdays,
		carecontract.ContractImportFieldEffectiveFrom:  columns.EffectiveFrom,
		carecontract.ContractImportFieldEndsOn:         columns.EndsOn,
		carecontract.ContractImportFieldConcludedOn:    columns.ConcludedOn,
		carecontract.ContractImportFieldPartyFirstName: columns.PartyFirstName,
		carecontract.ContractImportFieldPartyLastName:  columns.PartyLastName,
		carecontract.ContractImportFieldPartyEmail:     columns.PartyEmail,
		carecontract.ContractImportFieldDebtorNumber:   columns.DebtorNumber,
	}
}

// contractImportIndex is a mapping resolved against a file: the position of
// each mapped column and the choices for a file without a school or a
// module column.
type contractImportIndex struct {
	fields        map[string]int
	schoolID      int64
	moduleName    string
	effectiveFrom *calendar.Date
}

func (index contractImportIndex) has(field string) bool {
	_, ok := index.fields[field]
	return ok
}

// cell is the value of a field in a row; empty for an unmapped field or a
// short row.
func (index contractImportIndex) cell(cells []string, field string) string {
	position, ok := index.fields[field]
	if !ok || position >= len(cells) {
		return ""
	}
	return cells[position]
}

// settleContractImportMapping completes a mapping with the choices only one
// answer exists for: the school of a Träger with one school, the module of
// schools that offer one. A column always goes before a choice.
func settleContractImportMapping(mapping carecontract.ContractImportMapping, schools []importSchool) carecontract.ContractImportMapping {
	mapping.ModuleName = strings.TrimSpace(mapping.ModuleName)
	if mapping.Columns.EffectiveFrom != "" {
		mapping.EffectiveFrom = nil
	}
	if mapping.Columns.School != "" {
		mapping.SchoolID = 0
	} else if mapping.SchoolID == 0 && len(schools) == 1 {
		mapping.SchoolID = schools[0].school.ID
	}
	if mapping.Columns.Module != "" {
		mapping.ModuleName = ""
	} else if mapping.ModuleName == "" {
		if names := importModuleNames(schools, mapping.SchoolID); len(names) == 1 {
			mapping.ModuleName = names[0]
		}
	}
	return mapping
}

// importModuleNames are the distinct names of the modules a file can name
// at the school, or at every school for schoolID 0.
func importModuleNames(schools []importSchool, schoolID int64) []string {
	names := []string{}
	seen := map[string]bool{}
	for _, school := range schools {
		if schoolID != 0 && school.school.ID != schoolID {
			continue
		}
		for _, module := range school.modules {
			if key := textnorm.HeaderKey(module.Name); !seen[key] {
				seen[key] = true
				names = append(names, module.Name)
			}
		}
	}
	return names
}

// resolveContractImportMapping finds the column of each mapped field. A
// header the file does not have or a school that is not the Träger's refuses
// the mapping; unmapped names the required fields without a column or a
// choice.
func resolveContractImportMapping(headers []string, mapping carecontract.ContractImportMapping, schools []importSchool) (contractImportIndex, []string, error) {
	refused := map[string]string{}
	index := contractImportIndex{fields: map[string]int{}, schoolID: mapping.SchoolID, moduleName: mapping.ModuleName,
		effectiveFrom: mapping.EffectiveFrom}
	used := map[string]string{}
	for _, field := range contractImportFields {
		header := contractImportHeaders(mapping.Columns)[field]
		if header == "" {
			continue
		}
		position := slices.Index(headers, header)
		switch {
		case position < 0:
			refused[field] = "unknown_column"
		case used[header] != "":
			refused[field] = "column_used_twice"
		default:
			index.fields[field], used[header] = position, field
		}
	}
	if mapping.SchoolID != 0 && !slices.ContainsFunc(schools, func(s importSchool) bool { return s.school.ID == mapping.SchoolID }) {
		refused["school_id"] = "unknown"
	}
	if mapping.EffectiveFrom != nil && mapping.EffectiveFrom.Before(earliestImportDay) {
		refused[carecontract.ContractImportFieldEffectiveFrom] = "invalid"
	}
	if len(refused) > 0 {
		return contractImportIndex{}, nil, &carecontract.ValidationError{Fields: refused}
	}
	return index, index.unmapped(), nil
}

// unmapped names the required fields without a column or a choice.
func (index contractImportIndex) unmapped() []string {
	unmapped := []string{}
	for _, field := range contractImportRequired {
		if !index.has(field) {
			unmapped = append(unmapped, field)
		}
	}
	if !index.has(carecontract.ContractImportFieldEffectiveFrom) && index.effectiveFrom == nil {
		unmapped = append(unmapped, carecontract.ContractImportFieldEffectiveFrom)
	}
	if !index.has(carecontract.ContractImportFieldSchool) && index.schoolID == 0 {
		unmapped = append(unmapped, carecontract.ContractImportFieldSchool)
	}
	if !index.has(carecontract.ContractImportFieldModule) && index.moduleName == "" {
		unmapped = append(unmapped, carecontract.ContractImportFieldModule)
	}
	return unmapped
}

// importWeekdayNames are the names a file gives a weekday, folded.
var importWeekdayNames = map[string]string{
	"mo": "mon", "mon": "mon", "montag": "mon", "montags": "mon", "monday": "mon",
	"di": "tue", "die": "tue", "tue": "tue", "dienstag": "tue", "dienstags": "tue", "tuesday": "tue",
	"mi": "wed", "mit": "wed", "wed": "wed", "mittwoch": "wed", "mittwochs": "wed", "wednesday": "wed",
	"do": "thu", "don": "thu", "thu": "thu", "donnerstag": "thu", "donnerstags": "thu", "thursday": "thu",
	"fr": "fri", "fre": "fri", "fri": "fri", "freitag": "fri", "freitags": "fri", "friday": "fri",
	"sa": "sat", "sam": "sat", "sat": "sat", "samstag": "sat", "samstags": "sat", "sonnabend": "sat", "saturday": "sat",
	"so": "sun", "son": "sun", "sun": "sun", "sonntag": "sun", "sonntags": "sun", "sunday": "sun",
}

// parseImportWeekdays reads the weekdays a cell names: single days ("Mo, Di,
// Do", "Montag und Mittwoch") and ranges ("Mo-Fr", "Montag bis Freitag"), in
// the order of the week. ok is false for a word that is no weekday.
func parseImportWeekdays(value string) ([]string, bool) {
	text := strings.NewReplacer("–", "-", "—", "-", " bis ", "-", " und ", ",", "&", ",", "+", ",", ";", ",", "/", ",", "|", ",").
		Replace(" " + strings.ToLower(value) + " ")
	chosen := map[string]bool{}
	for _, word := range strings.FieldsFunc(strings.ReplaceAll(text, " - ", "-"), func(r rune) bool { return r == ',' || r == ' ' }) {
		start, end, ok := importWeekdaySpan(word)
		if !ok {
			return nil, false
		}
		for _, day := range weekdayOrder[start : end+1] {
			chosen[day] = true
		}
	}
	days := []string{}
	for _, day := range weekdayOrder {
		if chosen[day] {
			days = append(days, day)
		}
	}
	return days, len(days) > 0
}

// importWeekdaySpan reads one word as a weekday or a range of weekdays and
// answers its first and last position in the week.
func importWeekdaySpan(word string) (int, int, bool) {
	from, to, ranged := strings.Cut(word, "-")
	first, ok := importWeekdayNames[textnorm.HeaderKey(from)]
	if !ok {
		return 0, 0, false
	}
	last := first
	if ranged {
		if last, ok = importWeekdayNames[textnorm.HeaderKey(to)]; !ok {
			return 0, 0, false
		}
	}
	start, end := slices.Index(weekdayOrder, first), slices.Index(weekdayOrder, last)
	return start, end, end >= start
}
