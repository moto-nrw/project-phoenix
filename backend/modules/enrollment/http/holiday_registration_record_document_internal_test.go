package enrollmenthttp

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/lists"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// recordText is every title, label and value of the file, one per line.
func recordText(doc lists.RecordDocument) string {
	lines := []string{doc.Title, doc.Subtitle, doc.Footer}
	for _, record := range doc.Records {
		lines = append(lines, record.Title)
		for _, field := range record.Fields {
			lines = append(lines, field.Label+": "+field.Value)
		}
		for _, sub := range record.Subs {
			lines = append(lines, sub.Title)
			for _, field := range sub.Fields {
				lines = append(lines, field.Label+": "+field.Value)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func holidayTestRecord(t *testing.T) (capability.HolidayPeriod, capability.HolidayRegistrationRecord) {
	t.Helper()
	texts, ok := capability.HolidayConsentTextsOf(capability.HolidayConsentVersionCurrent, 365)
	require.True(t, ok)
	given := time.Date(2026, 11, 16, 8, 30, 0, 0, time.UTC)
	school, deletesOn := int64(7), calendar.NewDate(2028, 1, 16)
	form := capability.HolidayPeriodForm{
		Version: 2,
		Fields: []capability.HolidayFormField{
			{FormField: capability.FormField{Key: "schwimmen", Label: "Darf Ihr Kind schwimmen?", Type: capability.FormFieldBoolean, AppliesToCh: true}},
			{FormField: capability.FormField{Key: "impfung", Label: "Tetanus-Impfschutz", Type: capability.FormFieldText, AppliesToCh: true}, Health: true},
			{FormField: capability.FormField{Key: "anreise", Label: "Wie kommt Ihr Kind?", Type: capability.FormFieldText}},
			{FormField: capability.FormField{Key: "diaet", Label: "Besondere Kost in der Familie", Type: capability.FormFieldText}, Health: true},
		},
		LegalBlocks: []capability.FormLegalBlock{
			{Key: "fotos", Title: "Fotos", Label: "Mein Kind darf fotografiert werden.", Text: " Fotos bleiben in der OGS. ", Enabled: true},
			{Key: "ausflug", Title: "Ausflug", Label: "Mein Kind darf mit.", Enabled: true},
		},
	}
	detail := capability.HolidayRegistrationDetail{
		HolidayRegistrationSummary: capability.HolidayRegistrationSummary{
			ID: 42, Status: capability.HolidayRegistrationConfirmed, Frozen: true, SubmittedAt: given,
			GuardianFirstName: "Eva", GuardianLastName: "Muster", GuardianEmail: "eva@example.org", GuardianPhone: "0151 2345678",
			BenefitsDeclared: true, DebtorNumber: "D-4711", RetentionDays: 365, DeletesOn: &deletesOn,
			Children: []capability.HolidayRegistrationChild{{
				ID: 5, FirstName: "Lia", LastName: "Muster", BirthDate: calendar.NewDate(2018, 3, 1), HomeSchoolID: &school,
				Bookings: []capability.HolidayRegistrationBooking{{
					WeekStartsOn: calendar.NewDate(2027, 1, 4), WeekEndsOn: calendar.NewDate(2027, 1, 8), HostSchoolID: 7,
					TimeModelName: "bis 16 Uhr", Placement: capability.HolidayPlacementWaitlisted,
				}},
			}},
		},
		GuardianAddress: &capability.HolidayAddress{Street: "Musterweg 12", PostalCode: "49143", City: "Bissendorf"},
		SecondGuardian:  &capability.HolidayGuardianInput{FirstName: "Tom", LastName: "Muster", Phone: "0160 1"},
		Form:            &form, BenefitsProofAttached: true,
		CustomData:      capability.HolidayAnswers{"anreise": json.RawMessage(`"Mit dem Bus"`)},
		ChildCustomData: []capability.HolidayChildAnswers{{ChildID: 5, CustomData: capability.HolidayAnswers{"schwimmen": json.RawMessage(`true`)}}},
		Contacts: []capability.HolidayRegistrationContact{
			{Kind: capability.HolidayContactEmergency, Name: "Oma Muster", Phone: "05402 1234", Relationship: "Großmutter"},
			{Kind: capability.HolidayContactPickup, Name: "Opa Muster"},
		},
		PrivacyConsentAt: given, HealthConsentAt: &given, CustodyConfirmedAt: &given,
		ConsentVersion: texts.Version, ConsentTexts: &texts, DeclarationName: "Eva Muster",
		LegalAcceptances: map[string]time.Time{"fotos": given},
	}
	period := capability.HolidayPeriod{Name: "Weihnachtsferien 2026/2027"}
	return period, capability.HolidayRegistrationRecord{Detail: detail}
}

func TestHolidayRegistrationFileNamesEverythingTheFamilyEnteredAndDeclared(t *testing.T) {
	t.Parallel()
	period, record := holidayTestRecord(t)
	names := holidayRecordNames{schools: map[int64]string{7: "Grundschule Wissingen"}, generatedAt: time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC)}

	doc := holidayRegistrationRecordDocument(period, record, names)
	text := recordText(doc)
	for _, want := range []string{
		"Ferienanmeldung Nr. 42", "Weihnachtsferien 2026/2027", "Vertraulich. Enthält personenbezogene Daten.",
		"Status: Bestätigt", "Eingegangen am: 16.11.2026 09:30 Uhr", "Eingefroren: Ja", "Debitorennummer: D-4711",
		"Leistungen angegeben: Ja", "Nachweis liegt vor: Ja", "Löschung in moto am: 16.01.2028",
		"Name: Eva Muster", "E-Mail: eva@example.org", "Telefon: 0151 2345678", "Anschrift: Musterweg 12, 49143 Bissendorf",
		"Wie kommt Ihr Kind?: Mit dem Bus", "Zweite sorgeberechtigte Person", "Name: Tom Muster",
		"Kind: Lia Muster", "Geburtsdatum: 01.03.2018", "Schule: Grundschule Wissingen", "Darf Ihr Kind schwimmen?: Ja",
		"04.01. bis 08.01.2027, Grundschule Wissingen: bis 16 Uhr (Warteliste)",
		"Notfallkontakt: Oma Muster, Großmutter, 05402 1234", "Abholberechtigt: Opa Muster",
		"Erklärung der Eltern", "Fassung der Texte: Fassung 5, Löschung 365 Tage nach den Ferien",
		"Erklärt am: 16.11.2026 09:30 Uhr",
		"Wortlaut: " + record.Detail.ConsentTexts.PrivacyConsent,
		"Wortlaut: " + record.Detail.ConsentTexts.HealthConsent,
		"Wortlaut: " + record.Detail.ConsentTexts.CustodyConfirmed,
		"Fotos", "Wortlaut: Mein Kind darf fotografiert werden.\nFotos bleiben in der OGS.",
	} {
		assert.Contains(t, text, want)
	}
	assert.NotContains(t, text, "Verbindliche Anmeldung", "a declaration the parents did not give is not named")
	assert.NotContains(t, text, "Ausflug", "nor a notice of the Träger they did not confirm")

	file, err := holidayRecordFile(period, record, names)
	require.NoError(t, err)
	assert.Equal(t, "ferienanmeldung-42-muster.pdf", file.Filename)
	assert.Equal(t, "application/pdf", file.ContentType)
}

func TestHolidayRegistrationFileNamesHealthInformationOnlyWhenTheRecordCarriesIt(t *testing.T) {
	t.Parallel()
	period, record := holidayTestRecord(t)
	names := holidayRecordNames{schools: map[int64]string{7: "Grundschule Wissingen"}}
	secrets := []string{"Erdnüsse", "Asthmaspray", "Trägt eine Brille", "Tetanus-Impfschutz", "vegan"}

	text := recordText(holidayRegistrationRecordDocument(period, record, names))
	for _, secret := range secrets {
		assert.NotContains(t, text, secret)
	}

	record.Health = &capability.HolidayRegistrationHealthDetail{
		CustomData: capability.HolidayAnswers{"diaet": json.RawMessage(`"vegan"`)},
		Children: []capability.HolidayRegistrationHealth{{
			ChildID: 5, Allergies: "Erdnüsse", Medication: "Asthmaspray", HealthNotes: "Trägt eine Brille",
			CustomData: capability.HolidayAnswers{"impfung": json.RawMessage(`"ja"`)},
		}},
	}
	text = recordText(holidayRegistrationRecordDocument(period, record, names))
	for _, want := range []string{
		"Gesundheitsangaben der Familie", "Besondere Kost in der Familie: vegan", "Gesundheitsangaben\nAllergien: Erdnüsse",
		"Medikamente: Asthmaspray", "Hinweise zur Gesundheit: Trägt eine Brille", "Tetanus-Impfschutz: ja",
	} {
		assert.Contains(t, text, want)
	}
}

func TestHolidayRegistrationFileOfAnOldVersionSaysItsWordingIsNotKept(t *testing.T) {
	t.Parallel()
	period, record := holidayTestRecord(t)
	record.Detail.ConsentVersion, record.Detail.ConsentTexts = 0, nil
	text := recordText(holidayRegistrationRecordDocument(period, record, holidayRecordNames{}))
	assert.Contains(t, text, "Wortlaut: Der Wortlaut dieser Fassung ist nicht gespeichert.")
	assert.Contains(t, text, "Schule: Schule nicht mehr vorhanden")
}

func TestHolidayRegistrationFileOfTheThreeMonthsTextQuotesItsWording(t *testing.T) {
	t.Parallel()
	period, record := holidayTestRecord(t)
	texts, ok := capability.HolidayConsentTextsOf(3, capability.HolidayLegacyRetentionDays)
	require.True(t, ok)
	record.Detail.ConsentVersion, record.Detail.ConsentTexts = 3, &texts
	text := recordText(holidayRegistrationRecordDocument(period, record, holidayRecordNames{}))
	assert.Contains(t, text, "Fassung der Texte: Fassung 3\n", "an earlier version names no days it never named")
	assert.Contains(t, text, "Er löscht sie drei Monate nach den Ferien.")
}

func TestHolidayArchiveNameIsPlainASCII(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ferienanmeldungen-weihnachtsferien-2026-2027.zip", holidayArchiveName(capability.HolidayPeriod{Name: "Weihnachtsferien 2026/2027"}))
	assert.Equal(t, "ferienanmeldungen-fruehjahr-osterspass.zip", holidayArchiveName(capability.HolidayPeriod{Name: " Frühjahr: Osterspaß! "}))
	assert.Equal(t, "ferienanmeldungen.zip", holidayArchiveName(capability.HolidayPeriod{Name: "???"}))
}

// flushRecorder notes how many bytes of the response had been written at
// every flush.
type flushRecorder struct {
	*httptest.ResponseRecorder
	flushedAt []int
}

func (f *flushRecorder) Flush() { f.flushedAt = append(f.flushedAt, f.Body.Len()) }

// TestHolidayArchiveSendsEveryFileAsSoonAsItIsRendered pins the stream of
// the archive (#6691): each file is flushed to the client before the next
// one is rendered, so a large period sends from the first file on.
func TestHolidayArchiveSendsEveryFileAsSoonAsItIsRendered(t *testing.T) {
	t.Parallel()
	period, record := holidayTestRecord(t)
	second := record
	second.Detail.ID, second.Detail.GuardianLastName = 43, "Özdemir"
	set := capability.HolidayRegistrationRecordSet{Period: period, Records: []capability.HolidayRegistrationRecord{record, second}}
	names := holidayRecordNames{generatedAt: time.Date(2027, 2, 1, 9, 0, 0, 0, time.UTC)}
	first, err := holidayRecordEntry(set, 0, names)
	require.NoError(t, err)

	w := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	require.NoError(t, writeHolidayArchive(w, set, first, names))
	require.Len(t, w.flushedAt, 2, "one flush per file")
	assert.Positive(t, w.flushedAt[0], "the first file left before the second was rendered")
	assert.Greater(t, w.flushedAt[1], w.flushedAt[0])

	archive, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	require.NoError(t, err)
	require.Len(t, archive.File, 2)
	assert.Equal(t, "ferienanmeldung-42-muster.pdf", archive.File[0].Name)
	assert.Equal(t, "ferienanmeldung-43-oezdemir.pdf", archive.File[1].Name)
}
