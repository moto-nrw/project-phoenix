package application

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/carecontract"
	"github.com/moto-ogs/project-phoenix/modules/carecontract/internal/ports"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

var importToday = calendar.NewDate(2026, 11, 14)

func importDay(year int, month, day int) *calendar.Date {
	date := calendar.NewDate(year, time.Month(month), day)
	return &date
}

// importFixture is a Träger with two schools: the first offers a short and
// a long module of the running school year with conditions, the second a
// module without conditions.
func importFixture() ([]importSchool, templateIndex) {
	price := 9500
	year := func(id int64, name string, days ...string) carecontract.Offering {
		return carecontract.Offering{ID: id, Name: name, PhaseName: "Schuljahr 2026/27", PriceCents: &price, Active: true,
			EndTimes:      map[string]string{"mon": "14:00", "tue": "14:00", "wed": "14:00", "thu": "14:00", "fri": "13:00"},
			AvailableDays: days, ServiceStart: importDay(2026, 8, 1), ServiceEnd: importDay(2027, 7, 31)}
	}
	last := year(9, "OGS bis 14 Uhr")
	last.PhaseName, last.Active = "Schuljahr 2025/26", false
	last.ServiceStart, last.ServiceEnd = importDay(2025, 8, 1), importDay(2026, 7, 31)
	north := importSchool{school: carecontract.School{ID: 1, Name: "Grundschule Nord"},
		offerings: []carecontract.Offering{last, year(10, "OGS bis 14 Uhr"), year(11, "OGS bis 16 Uhr", "mon", "tue", "wed", "thu")}}
	north.modules = north.offerings[1:]
	later := year(21, "Später")
	later.ServiceStart, later.ServiceEnd = importDay(2027, 8, 1), importDay(2028, 7, 31)
	south := importSchool{school: carecontract.School{ID: 2, Name: "Grundschule Süd"}, offerings: []carecontract.Offering{year(20, "Ganztag"), later}}
	south.modules = south.offerings
	terms := carecontract.TemplateInput{TermKind: carecontract.TermOpenEnded, NoticeMonths: 3, NoticeTo: carecontract.NoticeMonthEnd}
	return []importSchool{north, south}, templateIndex{{1, 10}: terms, {1, 11}: terms}
}

var importHeaders = []string{"Vorname Kind", "Nachname Kind", "Geburtsdatum", "Schule", "Modul", "Wochentage", "Vertragsbeginn",
	"Vertragsende", "Zahler", "E-Mail", "Debitor-Nr."}

func importRow(overrides map[int]string) []string {
	cells := []string{"Lia", "Beispiel", "03.05.2018", "Grundschule Nord", "OGS bis 14 Uhr", "", "01.08.2026", "", "Beispiel",
		"eva@example.org", "70001"}
	for position, value := range overrides {
		cells[position] = value
	}
	return cells
}

func importCheckOf(t *testing.T, running ...ports.ContractChild) *contractImportCheck {
	t.Helper()
	schools, templates := importFixture()
	mapping := settleContractImportMapping(suggestContractImportMapping(importHeaders), schools)
	index, unmapped, err := resolveContractImportMapping(importHeaders, mapping, schools)
	require.NoError(t, err)
	require.Empty(t, unmapped)
	return newContractImportCheck(index, schools, templates, running, importToday)
}

func problemsOf(row carecontract.ContractImportRow) []string {
	problems := []string{}
	for _, problem := range row.Problems {
		problems = append(problems, problem.Field+":"+problem.Code)
	}
	return problems
}

// A file in the layout of the master data of OGS Connect maps without help;
// the signing date of the SEPA mandate is not the conclusion of a contract.
func TestSuggestContractImportMappingReadsCommonHeaders(t *testing.T) {
	t.Parallel()
	mapping := suggestContractImportMapping([]string{"Nachname Kind", "Vorname Kind", "", "Geburtsdatum", "Debitor-Nr.", "Kontoinhaber",
		"Unterschriftsdatum", "Vertragsbeginn", "E-Mail"})
	assert.Equal(t, carecontract.ContractImportColumns{ChildFirstName: "Vorname Kind", ChildLastName: "Nachname Kind",
		ChildBirthDate: "Geburtsdatum", EffectiveFrom: "Vertragsbeginn", PartyLastName: "Kontoinhaber", PartyEmail: "E-Mail",
		DebtorNumber: "Debitor-Nr."}, mapping.Columns)
}

// A Träger with one school and one module needs no column for them; with
// several, the office has to choose.
func TestContractImportMappingSettlesTheOnlySchoolAndModule(t *testing.T) {
	t.Parallel()
	schools, _ := importFixture()
	headers := []string{"Vorname", "Nachname", "Vertragsbeginn", "Zahler"}
	suggested := suggestContractImportMapping(headers)

	only := schools[1]
	only.offerings = only.offerings[:1]
	only.modules = only.offerings
	one := settleContractImportMapping(suggested, []importSchool{only})
	assert.Equal(t, only.school.ID, one.SchoolID)
	assert.Equal(t, "Ganztag", one.ModuleName)
	_, unmapped, err := resolveContractImportMapping(headers, one, []importSchool{only})
	require.NoError(t, err)
	assert.Empty(t, unmapped)

	several := settleContractImportMapping(suggested, schools)
	_, unmapped, err = resolveContractImportMapping(headers, several, schools)
	require.NoError(t, err)
	assert.Equal(t, []string{carecontract.ContractImportFieldSchool, carecontract.ContractImportFieldModule}, unmapped)

	several.SchoolID = 99
	_, _, err = resolveContractImportMapping(headers, several, schools)
	var invalid *carecontract.ValidationError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, map[string]string{"school_id": "unknown"}, invalid.Fields)

	twice := suggested
	twice.Columns.PartyEmail = "Zahler"
	_, _, err = resolveContractImportMapping(headers, twice, schools)
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, map[string]string{carecontract.ContractImportFieldPartyEmail: "column_used_twice"}, invalid.Fields)
}

func TestParseImportWeekdaysReadsDaysAndRanges(t *testing.T) {
	t.Parallel()
	for value, want := range map[string][]string{
		"Mo, Di, Do":           {"mon", "tue", "thu"},
		"Mo-Fr":                {"mon", "tue", "wed", "thu", "fri"},
		"Montag bis Mittwoch":  {"mon", "tue", "wed"},
		"Mo. und Fr.":          {"mon", "fri"},
		"freitags/montags":     {"mon", "fri"},
		"Mo – Mi + Fr":         {"mon", "tue", "wed", "fri"},
		"Dienstag; Donnerstag": {"tue", "thu"},
		"mon tue":              {"mon", "tue"},
	} {
		days, ok := parseImportWeekdays(value)
		assert.True(t, ok, value)
		assert.Equal(t, want, days, value)
	}
	for _, value := range []string{"täglich", "Fr-Mo", "Mo, Xy", "5", "-"} {
		_, ok := parseImportWeekdays(value)
		assert.False(t, ok, value)
	}
}

// A ready row becomes a concluded contract: the module of the running
// phase on its own days, the conditions of the Träger, the price of the one
// module and the end of the phase.
func TestContractImportRowBecomesAContractOfTheRunningPhase(t *testing.T) {
	t.Parallel()
	candidate := importCheckOf(t).candidate(importRow(nil), 2)
	row := candidate.row
	require.Equal(t, carecontract.ContractImportReady, row.Status, problemsOf(row))
	assert.Equal(t, candidate.contract.SchoolID, row.SchoolID)
	assert.Equal(t, []string{"OGS bis 14 Uhr"}, row.ModuleNames)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu", "fri"}, row.Weekdays)
	assert.Equal(t, importDay(2027, 7, 31), row.EndsOn, "the contract ends with the phase of its module")
	assert.Equal(t, "Beispiel", row.PartyName)

	contract := candidate.contract
	assert.Equal(t, "lia\x1fbeispiel\x1f2018-05-03\x1f2026-08-01", contract.ImportKey)
	assert.Equal(t, calendar.NewDate(2026, 8, 1).BerlinMidnight(), contract.ConcludedAt, "concluded on its first day without a date of its own")
	version := contract.Version
	assert.Equal(t, carecontract.VersionEffective, version.Status)
	assert.Equal(t, carecontract.SourceImport, version.Source)
	assert.Equal(t, int64(9500), *version.PriceCents)
	assert.Equal(t, 3, version.Terms.NoticeMonths)
	require.Len(t, version.Services, 5)
	assert.Equal(t, carecontract.Service{Weekday: "fri", CareOfferingID: 10, ModuleName: "OGS bis 14 Uhr", EndsAt: "13:00"}, version.Services[4],
		"a module that runs as long as the version carries no end of its own")
}

func TestContractImportRowProblems(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		overrides map[int]string
		problems  []string
	}{
		"names and start are required": {map[int]string{0: "", 6: "", 8: ""},
			[]string{"child_first_name:missing", "party_last_name:missing", "effective_from:missing"}},
		"a birthday in the future":          {map[int]string{2: "01.01.2030"}, []string{"child_birth_date:future"}},
		"a date that is none":               {map[int]string{6: "bald"}, []string{"effective_from:invalid"}},
		"an unknown school":                 {map[int]string{3: "Grundschule West"}, []string{"school:unknown"}},
		"an unknown module":                 {map[int]string{4: "Hort"}, []string{"module:unknown"}},
		"a module whose phase begins later": {map[int]string{3: "grundschule süd", 4: "Später", 6: "01.12.2026"}, []string{"module:before_phase"}},
		"a module of an ended phase":        {map[int]string{3: "Grundschule Nord", 4: "OGS bis 14 Uhr", 6: "01.08.2025", 7: "31.07.2026"}, []string{"ends_on:ended"}},
		"a module without conditions": {map[int]string{3: "grundschule süd", 4: "Ganztag"},
			[]string{"module:conditions_missing"}},
		"a weekday the module does not run on": {map[int]string{4: "OGS bis 16 Uhr", 5: "Mo, Fr"}, []string{"weekdays:not_offered"}},
		"weekdays that are none":               {map[int]string{5: "immer"}, []string{"weekdays:invalid"}},
		"an end before the start":              {map[int]string{7: "01.07.2026"}, []string{"ends_on:before_start"}},
		"a mail address that is none":          {map[int]string{9: "eva at example"}, []string{"party_email:invalid"}},
	} {
		row := importCheckOf(t).candidate(importRow(test.overrides), 2).row
		assert.Equal(t, carecontract.ContractImportInvalid, row.Status, name)
		assert.Equal(t, test.problems, problemsOf(row), name)
	}
}

// A contract that runs for years books the module of the running phase: in
// moto it begins with that phase, as a contract from an application does,
// and keeps the day it was concluded.
func TestContractImportRowOfAnOlderContractStartsWithTheRunningPhase(t *testing.T) {
	t.Parallel()
	candidate := importCheckOf(t).candidate(importRow(map[int]string{6: "01.08.2024"}), 2)
	require.Equal(t, carecontract.ContractImportReady, candidate.row.Status, problemsOf(candidate.row))
	assert.Equal(t, importDay(2026, 8, 1), candidate.row.EffectiveFrom)
	assert.Equal(t, []string{carecontract.ContractImportHintStartMoved}, candidate.row.Hints)
	assert.Equal(t, "OGS bis 14 Uhr", candidate.contract.Version.Services[0].ModuleName)
	assert.Equal(t, importDay(2027, 7, 31), candidate.row.EndsOn, "the module of the running school year")
	assert.Equal(t, calendar.NewDate(2024, 8, 1).BerlinMidnight(), candidate.contract.ConcludedAt)
	assert.Equal(t, "lia\x1fbeispiel\x1f2018-05-03\x1f2026-08-01", candidate.contract.ImportKey)

	ended := importCheckOf(t).candidate(importRow(map[int]string{6: "01.08.2024", 7: "31.07.2025"}), 2).row
	assert.Equal(t, []string{"ends_on:ended"}, problemsOf(ended), "a contract that ended before the phase is ended, not misdated")
}

// A contract an earlier import stored is not stored again once it ended or
// was withdrawn: the row is a duplicate that names it.
func TestContractImportRecognisesAnEarlierImportThatEnded(t *testing.T) {
	t.Parallel()
	ended := calendar.NewDate(2026, 10, 31)
	stored := ports.ContractChild{ContractID: 12, SchoolID: 1, Status: carecontract.StatusTerminated, FirstName: "Lia", LastName: "Beispiel",
		BirthDate: importDay(2018, 5, 3), EndsOn: &ended, ImportOrigin: "lia\x1fbeispiel\x1f2018-05-03\x1f2026-08-01"}
	withdrawn := ports.ContractChild{ContractID: 13, SchoolID: 1, Status: carecontract.StatusWithdrawn, FirstName: "Ben", LastName: "Muster"}
	check := importCheckOf(t, stored, withdrawn)

	row := check.candidate(importRow(nil), 2).row
	assert.Equal(t, carecontract.ContractImportDuplicate, row.Status)
	assert.Equal(t, stored.ContractID, row.ContractID)
	assert.Equal(t, []string{carecontract.ContractImportHintImportedBefore}, row.Hints)

	later := check.candidate(importRow(map[int]string{6: "01.12.2026"}), 3).row
	assert.Equal(t, carecontract.ContractImportConflict, later.Status, "the same child twice in one file stays a conflict")
	other := check.candidate(importRow(map[int]string{0: "Ben", 1: "Muster"}), 4).row
	assert.Equal(t, carecontract.ContractImportReady, other.Status, "a withdrawn contract from an application blocks nothing")
}

// Two modules in one cell make one contract over both; it has no price of
// its own and names the days of each module.
func TestContractImportRowBooksSeveralModules(t *testing.T) {
	t.Parallel()
	candidate := importCheckOf(t).candidate(importRow(map[int]string{4: "OGS bis 14 Uhr + OGS bis 16 Uhr", 5: "Mo-Do", 7: "31.01.2027"}), 2)
	row := candidate.row
	require.Equal(t, carecontract.ContractImportReady, row.Status, problemsOf(row))
	assert.Equal(t, []string{"OGS bis 14 Uhr", "OGS bis 16 Uhr"}, row.ModuleNames)
	assert.Equal(t, []string{"mon", "tue", "wed", "thu"}, row.Weekdays)
	assert.Equal(t, importDay(2027, 1, 31), row.EndsOn)
	assert.Nil(t, candidate.contract.Version.PriceCents)
	assert.Len(t, candidate.contract.Version.Services, 8)
}

// A repeated import stores nothing twice: a child with a running contract at
// the school is a duplicate, also without a date of birth on one side; a
// child the file names twice is a conflict; an ended contract counts no
// more.
func TestContractImportFindsDuplicatesAndRepeatedChildren(t *testing.T) {
	t.Parallel()
	ended := calendar.NewDate(2026, 7, 31)
	running := ports.ContractChild{ContractID: 7, SchoolID: 1, FirstName: "LIA", LastName: "beispiel"}
	check := importCheckOf(t, running,
		ports.ContractChild{ContractID: 8, SchoolID: 1, FirstName: "Ben", LastName: "Muster", EndsOn: &ended},
		ports.ContractChild{ContractID: 9, SchoolID: 2, FirstName: "Mia", LastName: "Anders", BirthDate: importDay(2018, 1, 1)})

	duplicate := check.candidate(importRow(nil), 2).row
	assert.Equal(t, carecontract.ContractImportDuplicate, duplicate.Status)
	assert.Equal(t, running.ContractID, duplicate.ContractID)

	again := check.candidate(importRow(map[int]string{1: "Muster", 0: "Ben"}), 3).row
	assert.Equal(t, carecontract.ContractImportReady, again.Status, "the old contract has ended")

	repeated := check.candidate(importRow(map[int]string{1: "MUSTER", 0: " ben ", 2: ""}), 4).row
	assert.Equal(t, carecontract.ContractImportConflict, repeated.Status)
	assert.Equal(t, 3, repeated.RepeatsLine)
	assert.Equal(t, []string{"child_last_name:repeated"}, problemsOf(repeated))

	twin := check.candidate(importRow(map[int]string{0: "Mia", 1: "Anders", 2: "02.02.2019"}), 5).row
	assert.Equal(t, carecontract.ContractImportReady, twin.Status, "another school and another birthday is another child")
}

// The confirmed state names the mapping, the scope of the numbers and the
// rows; a change of any makes another one.
func TestContractImportStateFollowsMappingScopeAndRows(t *testing.T) {
	t.Parallel()
	check := importCheckOf(t)
	rows := []importCandidate{check.candidate(importRow(nil), 2)}
	mapping := carecontract.ContractImportMapping{SchoolID: 1}
	first := string(contractImportState(mapping, "payer", rows))
	assert.Equal(t, first, string(contractImportState(mapping, "payer", rows)))
	assert.NotEqual(t, first, string(contractImportState(mapping, "child", rows)))
	mapping.SchoolID = 2
	assert.NotEqual(t, first, string(contractImportState(mapping, "payer", rows)))
	mapping.SchoolID = 1
	rows[0].row.Status = carecontract.ContractImportDuplicate
	assert.NotEqual(t, first, string(contractImportState(mapping, "payer", rows)))
}

func TestImportFileNameKeepsNoPath(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "kinder.csv", importFileName(`C:\Users\eva\kinder.csv`))
	assert.Equal(t, "kinder.csv", importFileName("/tmp/kinder.csv "))
	assert.Equal(t, "import.csv", importFileName("  "))
}

func TestReadContractImportTableRefusesWhatIsNoCSV(t *testing.T) {
	t.Parallel()
	for name, content := range map[string]string{
		"binary":          "PK\x03\x04\x00\x00binary",
		"header only":     "Vorname;Nachname\n",
		"one column":      "Vorname\nLia\n",
		"repeated header": "Name;Schule;Name\nLia;Nord;Beispiel\n",
	} {
		_, err := readContractImportTable([]byte(content), "")
		require.ErrorIs(t, err, carecontract.ErrContractImportFileInvalid, name)
	}
	_, err := readContractImportTable(make([]byte, carecontract.MaxContractImportFileBytes+1), "")
	require.ErrorIs(t, err, carecontract.ErrContractImportFileTooLarge)
}

// A file without the first day of the contracts, such as the master data
// of OGS Connect, takes one day the office chose for every row; a column
// goes before the choice.
func TestContractImportTakesAChosenFirstDayForEveryRow(t *testing.T) {
	t.Parallel()
	schools, templates := importFixture()
	headers := []string{"Nachname Kind", "Vorname Kind", "", "Debitor-Nr.", "Kontoinhaber", "IBAN", "Mandat ist gültig ab", ""}
	mapping := settleContractImportMapping(suggestContractImportMapping(headers), schools)
	mapping.SchoolID, mapping.ModuleName = schools[0].school.ID, "OGS bis 14 Uhr"
	_, unmapped, err := resolveContractImportMapping(headers, mapping, schools)
	require.NoError(t, err)
	assert.Equal(t, []string{carecontract.ContractImportFieldEffectiveFrom}, unmapped)

	mapping.EffectiveFrom = importDay(2026, 9, 1)
	index, unmapped, err := resolveContractImportMapping(headers, mapping, schools)
	require.NoError(t, err)
	require.Empty(t, unmapped)
	check := newContractImportCheck(index, schools, templates, nil, importToday)
	row := check.candidate([]string{"Beispiel", "Lia", "", "70001", "Eva Beispiel", "", "01.03.2024 00:00", ""}, 2).row
	require.Equal(t, carecontract.ContractImportReady, row.Status, problemsOf(row))
	assert.Equal(t, importDay(2026, 9, 1), row.EffectiveFrom)
	assert.Empty(t, row.Hints)

	mapping.Columns.EffectiveFrom = "Mandat ist gültig ab"
	assert.Nil(t, settleContractImportMapping(mapping, schools).EffectiveFrom, "a column goes before the choice")

	mapping.Columns.EffectiveFrom, mapping.EffectiveFrom = "", importDay(1989, 12, 31)
	_, _, err = resolveContractImportMapping(headers, mapping, schools)
	var invalid *carecontract.ValidationError
	require.ErrorAs(t, err, &invalid)
	assert.Equal(t, map[string]string{carecontract.ContractImportFieldEffectiveFrom: "invalid"}, invalid.Fields)
}

// A row that names no parent at all still is a contract: it has no party
// and says so. A row that names a parent's first name or mail address
// without the last name is incomplete.
func TestContractImportRowWithoutParentsHasNoParty(t *testing.T) {
	t.Parallel()
	candidate := importCheckOf(t).candidate(importRow(map[int]string{8: "", 9: ""}), 2)
	require.Equal(t, carecontract.ContractImportReady, candidate.row.Status, problemsOf(candidate.row))
	assert.Equal(t, []string{carecontract.ContractImportHintNoParty}, candidate.row.Hints)
	assert.Empty(t, candidate.row.PartyName)
	assert.Equal(t, ports.NewParty{}, candidate.contract.Party)

	incomplete := importCheckOf(t).candidate(importRow(map[int]string{8: ""}), 2).row
	assert.Equal(t, []string{"party_last_name:missing"}, problemsOf(incomplete))
}
