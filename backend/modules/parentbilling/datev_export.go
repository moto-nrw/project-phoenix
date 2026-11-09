package parentbilling

import "context"

// The DATEV booking batch of the Elternabrechnung (#4694). The file comes
// with the export protocol and the file number (#4693); until then the
// Träger can check whether its settings make a valid batch. The check runs
// the same formatter as the export over a probe booking, so it cannot
// disagree with it.

// DatevSetting is one setting of the Träger with its key, so the check can
// name it.
type DatevSetting struct {
	Key   string
	Value string
}

// DatevSettings are the values of the Träger the batch needs: the header
// (Beraternummer, Mandantennummer, Sachkontenlänge, Kontenrahmen, first
// month of the fiscal year as "1" to "12", Festschreibung) and the revenue
// account of the contributions. An empty value is unset.
type DatevSettings struct {
	ConsultantNumber DatevSetting
	ClientNumber     DatevSetting
	AccountLength    DatevSetting
	ChartOfAccounts  DatevSetting
	FiscalYearStart  DatevSetting
	RevenueAccount   DatevSetting
	Locked           bool
}

// DatevBillingSettings reads the DATEV settings of a Träger: its own values
// or the registry defaults. The root binds it to the Träger level of the
// settings system; it joins the caller's Träger transaction.
type DatevBillingSettings interface {
	DatevSettings(ctx context.Context, organizationID int64) (DatevSettings, error)
}

// DatevReadiness says whether the settings of the Träger make a valid
// batch. It names the keys of the settings that are missing or invalid,
// never their values.
type DatevReadiness struct {
	Ready           bool
	MissingSettings []string
	InvalidSettings []string
}

// DatevExport is the DATEV booking batch of a Träger. The caller opens the
// Träger transaction of the session's Träger.
type DatevExport interface {
	DatevReadiness(ctx context.Context, reviewer HolidayReviewer) (DatevReadiness, error)
}
