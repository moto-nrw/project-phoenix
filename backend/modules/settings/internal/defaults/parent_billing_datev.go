package defaults

import (
	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// The DATEV booking batch of the Elternabrechnung (#4694): the values the
// header and the bookings of the file need. The customer has not delivered
// them yet (SJR, "wartet auf Ute"); a plausible preset would book silently
// on wrong accounts, so the numbers and accounts stay empty and the export
// names what is missing. Only the fiscal year has a preset (January, the
// calendar year of most associations) and the Festschreibung is off, so the
// tax office of the Träger can still correct a booking. The patterns follow
// the DATEV header (Beraternummer 1001 to 9999999, Mandantennummer 1 to
// 99999, Sachkontenlänge 4 to 8); whether the revenue account fits the
// length is checked when the file is made.
func init() {
	registerDefinitions(parentBillingDatevDefinitions)
}

var (
	datevConsultantPattern = `^(100[1-9]|10[1-9]\d|1[1-9]\d{2}|[2-9]\d{3}|[1-9]\d{4,6})$`
	datevClientPattern     = `^[1-9]\d{0,4}$`
	// The revenue account is digits and not zero; DATEV refuses an account
	// of zeros (Konto ^(?!0{1,9}$)(\d{1,9})$). Its length limit is
	// datevAccountMaxLength; that it fits the Sachkontenlänge is a
	// cross-field rule the Träger level cannot hold (ADR 0086), so the
	// export check names it.
	datevAccountPattern = `^0*[1-9][0-9]*$`
	// datevAccountMaxLength: a Sachkonto has at most 8 digits.
	datevAccountMaxLength = 8
)

var datevAccountLengthOptions = &config.SelectOptions{Static: []config.SelectOption{
	{Label: "4 Stellen", Value: "4"}, {Label: "5 Stellen", Value: "5"}, {Label: "6 Stellen", Value: "6"},
	{Label: "7 Stellen", Value: "7"}, {Label: "8 Stellen", Value: "8"},
}}

var datevChartOptions = &config.SelectOptions{Static: []config.SelectOption{
	{Label: "SKR 03", Value: "03"}, {Label: "SKR 04", Value: "04"},
}}

var datevMonthOptions = &config.SelectOptions{Static: []config.SelectOption{
	{Label: "Januar", Value: "1"}, {Label: "Februar", Value: "2"}, {Label: "März", Value: "3"},
	{Label: "April", Value: "4"}, {Label: "Mai", Value: "5"}, {Label: "Juni", Value: "6"},
	{Label: "Juli", Value: "7"}, {Label: "August", Value: "8"}, {Label: "September", Value: "9"},
	{Label: "Oktober", Value: "10"}, {Label: "November", Value: "11"}, {Label: "Dezember", Value: "12"},
}}

// The two groups of the DATEV tab: the client in DATEV and the bookings.
var (
	datevClientGroup  = config.Definition{Tab: "datev", Category: "datev_mandant"}
	datevBookingGroup = config.Definition{Tab: "datev", Category: "datev-buchungen"}
)

func datevDefinition(group config.Definition, key string, order int, label, description string) config.Definition {
	return config.Definition{
		Key:             key,
		Levels:          config.LevelTraeger,
		Label:           label,
		Description:     description,
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             group.Tab,
		Category:        group.Category,
		SortOrder:       order,
	}
}

var parentBillingDatevDefinitions = func() []config.Definition {
	consultant := datevDefinition(datevClientGroup, config.KeyParentBillingDatevConsultantNumber, 10, "Beraternummer",
		"Die Nummer Ihres Steuerbüros in DATEV, 4 bis 7 Ziffern. Sie steht in der DATEV-Datei oben. Ohne sie gibt es keine DATEV-Datei.")
	consultant.Validation = &config.ValidationRules{Pattern: &datevConsultantPattern, AllowEmpty: true}

	client := datevDefinition(datevClientGroup, config.KeyParentBillingDatevClientNumber, 20, "Mandantennummer",
		"Ihre Nummer beim Steuerbüro in DATEV, 1 bis 5 Ziffern. Ohne sie gibt es keine DATEV-Datei.")
	client.Validation = &config.ValidationRules{Pattern: &datevClientPattern, AllowEmpty: true}

	length := datevDefinition(datevClientGroup, config.KeyParentBillingDatevAccountLength, 30, "Länge der Sachkonten",
		"So viele Ziffern haben Ihre Sachkonten in DATEV. Fragen Sie Ihr Steuerbüro. Debitoren haben eine Ziffer mehr.")
	length.Type, length.Options = config.FieldSelect, datevAccountLengthOptions

	chart := datevDefinition(datevClientGroup, config.KeyParentBillingDatevChartOfAccounts, 40, "Kontenrahmen",
		"Der Kontenrahmen Ihrer Buchhaltung. Fragen Sie Ihr Steuerbüro.")
	chart.Type, chart.Options = config.FieldSelect, datevChartOptions

	fiscal := datevDefinition(datevClientGroup, config.KeyParentBillingDatevFiscalYearStart, 50, "Beginn des Wirtschaftsjahres",
		"In diesem Monat beginnt Ihr Wirtschaftsjahr. Bei den meisten Vereinen ist das der Januar.")
	fiscal.Type, fiscal.Options, fiscal.Default = config.FieldSelect, datevMonthOptions, "1"

	revenue := datevDefinition(datevBookingGroup, config.KeyParentBillingDatevRevenueAccount, 60, "Erlöskonto für Elternbeiträge",
		"Auf dieses Sachkonto bucht moto die Elternbeiträge. Es hat höchstens so viele Ziffern wie Ihre Sachkonten.")
	revenue.Validation = &config.ValidationRules{Pattern: &datevAccountPattern, AllowEmpty: true, MaxLength: &datevAccountMaxLength}

	locked := datevDefinition(datevBookingGroup, config.KeyParentBillingDatevLocked, 70, "Buchungen festschreiben",
		"Ist das an, sind die Buchungen nach dem Einlesen in DATEV festgeschrieben. Ihr Steuerbüro kann sie dann nicht mehr ändern.")
	locked.Type, locked.Default = config.FieldBoolean, false

	return []config.Definition{consultant, client, length, chart, fiscal, revenue, locked}
}()
