package tenantexport

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/uptrace/bun"
)

// The register of the return package (Art. 26 Data Act, ADR 0201) describes
// every table, field, type, nullability and relation a package can carry.
// It reads the schema the way an export does, under the export's read role,
// and describes each table with exportTableModel, the function model.json
// uses. It never reads a row. It runs only in the guard test against the
// migrated schema; production serves the generated, embedded file.

type returnRegisterBuilder struct {
	db           *bun.DB
	fingerprints organizationtenancy.ExportFingerprints
}

// newReturnRegisterBuilder builds the register from a migrated database.
func newReturnRegisterBuilder(db *bun.DB, fingerprints organizationtenancy.ExportFingerprints) *returnRegisterBuilder {
	return &returnRegisterBuilder{db: db, fingerprints: fingerprints}
}

const (
	packageSchool       = "school"
	packageOrganization = "organization"
)

func (b *returnRegisterBuilder) BuildReturnRegister(ctx context.Context) (organizationtenancy.ReturnRegister, error) {
	var register organizationtenancy.ReturnRegister
	source, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	if err != nil {
		return register, errors.New("return register: catalog unavailable")
	}
	var catalog organizationtenancy.DataLifecycleCatalog
	if err := json.Unmarshal(source, &catalog); err != nil {
		return register, errors.New("return register: invalid catalog")
	}
	tx, err := b.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return register, errors.New("return register: cannot open read-only transaction")
	}
	defer func() { _ = tx.Rollback() }()
	// The export's read role sees the columns the export sees.
	if _, err := tx.ExecContext(ctx, "SET LOCAL ROLE phoenix_tenant_export"); err != nil {
		return register, errors.New("return register: administrative read role unavailable")
	}
	schema, err := loadExportSchema(ctx, tx, catalog)
	if err != nil {
		return register, errors.New("return register: schema unavailable")
	}
	register = returnRegisterHead(catalog.Version, b.fingerprints.Bytes(source))
	// A Träger package leaves what a school holds to the school packages;
	// the scope of a Träger run says which tables those are.
	traeger := &exportRun{catalog: catalog, options: organizationtenancy.TenantExportOptions{OrganizationID: 1}}
	for _, table := range catalog.Tables {
		if table.Export != "include" {
			register.ExcludedTables = append(register.ExcludedTables, organizationtenancy.ReturnRegisterExcluded{
				Name: table.Name, Classification: table.Export, Reason: table.ExportReason})
			continue
		}
		model, err := exportTableModel(table, schema)
		if err != nil {
			return register, err
		}
		packages := []string{packageSchool}
		if _, err := traeger.organizationScope(table, "t", 0); err == nil {
			packages = append(packages, packageOrganization)
		} else if !errors.Is(err, errInSchoolPackage) {
			return register, err
		}
		register.Tables = append(register.Tables, organizationtenancy.ReturnRegisterTable{
			ExportModelTable: model, Packages: packages, ExcludedColumns: table.ExcludeSecretColumns})
	}
	for _, kind := range catalog.Files {
		register.Files = append(register.Files, organizationtenancy.ReturnRegisterFileKind{Kind: kind.Kind, Export: kind.Export, Reason: kind.Reason})
	}
	return register, nil
}

func returnRegisterHead(catalogVersion int, catalogSHA256 string) organizationtenancy.ReturnRegister {
	return organizationtenancy.ReturnRegister{
		RegisterVersion:      organizationtenancy.ReturnRegisterVersion,
		FormatVersion:        organizationtenancy.ExportFormatVersion,
		CatalogVersion:       catalogVersion,
		CatalogSHA256:        catalogSHA256,
		About:                registerAbout,
		Standards:            registerStandards,
		CommonSpecifications: registerCommonSpecifications,
		TimeEstimate:         registerTimeEstimate,
		Retrieval:            registerRetrieval,
		Packages: []organizationtenancy.ReturnRegisterPackage{
			{Kind: packageSchool, Description: "Alle Daten einer Schule und die Daten ihres Trägers. Eine Datei je Tabelle."},
			{Kind: packageOrganization, Description: "Die Daten eines Trägers: seine Tabellen, die Stammdaten seiner Schulen und die Konten der Geschäftsstelle. Was eine Schule hält, steht im Paket der Schule."},
		},
		Layout: registerLayout,
		Values: registerValues,
	}
}

const registerAbout = "Dieses Register beschreibt das Rückgabepaket von moto: welche Tabellen und Felder es enthält, " +
	"welche Typen sie haben, ob sie leer sein dürfen und wie die Tabellen verbunden sind. " +
	"Es enthält keine Kundendaten. Jedes Paket enthält dieselbe Beschreibung für seine Tabellen als model.json. " +
	"formatVersion und catalogSHA256 stehen auch in der manifest.json des Pakets."

const registerCommonSpecifications = "Für diese Art von Dienst gibt es noch keine gemeinsamen Spezifikationen oder " +
	"harmonisierten Normen im Unionsrepositorium nach Art. 35 Data Act. Deshalb geben wir die Daten in den " +
	"oben genannten offenen Standards zurück (Art. 30 Abs. 5 Data Act)."

const registerTimeEstimate = "Unverbindliche Schätzung: Ein Paket erstellen und prüfen wir voraussichtlich innerhalb " +
	"von 10 Arbeitstagen nach Ihrem Auftrag. Bei vielen gespeicherten Dateien kann es länger dauern. " +
	"Maßgeblich sind die Fristen in Ihrem Vertrag."

const registerRetrieval = "Wir erstellen und prüfen das Paket und übergeben es Ihnen verschlüsselt. " +
	"Die Rückgabe ist kostenlos."

var registerStandards = []organizationtenancy.ReturnRegisterStandard{
	{Name: "JSON", Reference: "RFC 8259", Use: "manifest.json, model.json, Zeilen der Tabellen"},
	{Name: "JSON Lines", Reference: "https://jsonlines.org", Use: "Eine Datei <Schema>.<Tabelle>.jsonl je Tabelle, ein JSON-Objekt je Zeile"},
	{Name: "UTF-8", Reference: "RFC 3629", Use: "Zeichenkodierung aller Text- und JSON-Dateien"},
	{Name: "SHA-256", Reference: "FIPS 180-4", Use: "Prüfsumme jeder Datei in manifest.json und der Inhalt von COMPLETE"},
	{Name: "Datum und Zeit", Reference: "ISO 8601 / RFC 3339", Use: "Zeitpunkte und Kalendertage in den Zeilen"},
	{Name: "SQL-Datentypen", Reference: "ISO/IEC 9075, information_schema", Use: "type und udt der Felder in model.json und in diesem Register"},
	{Name: "ZIP", Reference: "ISO/IEC 21320-1", Use: "Das Paket als eine Datei beim Abruf über die Schnittstelle"},
	{Name: "PAIN.008", Reference: "ISO 20022", Use: "SEPA-Lastschriftdateien im Paket eines Trägers"},
}

var registerLayout = []organizationtenancy.ReturnRegisterPath{
	{Path: "COMPLETE", Packages: []string{packageSchool, packageOrganization}, Description: "Entsteht zuletzt. Enthält die SHA-256-Prüfsumme von manifest.json. Ohne diese Datei ist das Paket unvollständig."},
	{Path: "manifest.json", Packages: []string{packageSchool, packageOrganization}, Description: "Formatversion, Katalogversion, Zeitpunkt, jede Tabelle mit Anzahl der Zeilen oder dem Grund ihres Fehlens, jede Datei mit Prüfsumme und Größe."},
	{Path: "model.json", Packages: []string{packageSchool, packageOrganization}, Description: "Felder, Typen, Nullbarkeit und Beziehungen der Tabellen dieses Pakets, wie in diesem Register."},
	{Path: "<Schema>.<Tabelle>.jsonl", Packages: []string{packageSchool, packageOrganization}, Description: "Die Zeilen einer Tabelle. Ein JSON-Objekt je Zeile, die Schlüssel sind die Felder aus model.json."},
	{Path: "objects/<Nummer>", Packages: []string{packageSchool, packageOrganization}, Description: "Gespeicherte Dateien im Originalformat, zum Beispiel Fotos und Dokumente."},
	{Path: "objects.jsonl", Packages: []string{packageSchool, packageOrganization}, Description: "Ordnet jede Datei unter objects/ ihrem ursprünglichen Speicherort zu."},
	{Path: "DOKUMENTE/", Packages: []string{packageOrganization}, Description: "Gespeicherte Dokumente des Trägers als einzelne Dateien. index.jsonl nennt Tabelle, Zeile, Spalte, Medientyp, Prüfsumme und Größe."},
	{Path: "SEPA-LASTSCHRIFTDATEIEN/", Packages: []string{packageOrganization}, Description: "Die eingereichten SEPA-Lastschriftdateien (PAIN.008), je Datei Original oder rekonstruiert."},
}

var registerValues = []organizationtenancy.ReturnRegisterValue{
	{Types: []string{"timestamp with time zone"}, Description: "Zeitpunkt nach ISO 8601 mit Zeitzone, zum Beispiel 2026-11-10T08:15:00+01:00."},
	{Types: []string{"date"}, Description: "Kalendertag als YYYY-MM-DD."},
	{Types: []string{"time without time zone"}, Description: "Uhrzeit als HH:MM:SS."},
	{Types: []string{"bytea"}, Description: "Binärwert als Text mit \\x und Hexadezimalziffern. Dokumente eines Trägers liegen zusätzlich als Datei unter DOKUMENTE/."},
	{Types: []string{"jsonb", "json"}, Description: "Verschachteltes JSON. Geheime Teile sind entfernt oder null."},
	{Types: []string{"ARRAY"}, Description: "JSON-Array der Werte."},
	{Types: []string{"USER-DEFINED"}, Description: "Wert als Text; udt nennt den Typ."},
}
