---
status: accepted
---

# Mandantenlöschung und Datenrückgabe nutzen einen gemeinsamen Katalog

Entscheidung vom 23.10.2026 für Slice 3837-a, Grundlage für
[#3837](https://github.com/moto-nrw/project-phoenix/issues/3837) und
[#3838](https://github.com/moto-nrw/project-phoenix/issues/3838) unter
[#3846](https://github.com/moto-nrw/project-phoenix/issues/3846).

## Ausgangslage

Die geprüfte Basis `origin/development` enthält nur `SoftDeleteSchool` und
`SoftDeleteOrganization` in Organisation & Tenancy, keine vollständige
Mandantenlöschung und keinen Gesamt-/Personenexport. Die laufenden
Löschfristen und die vorhandenen Bereichsexporte ersetzen diese Verfahren
nicht. Das migrierte Testschema enthält 247 Tabellen, davon 216 mit
`tenant_id`, sowie vier Tabellen mit indirektem Mandantenbezug über FKs.
Weitere Daten liegen in neun Upload-Namespaces und in Nebensystemen.

Ein gemeinsamer Lebenszyklus muss auch geteilte Konten, Schul-/Trägerwurzeln,
Auditdaten, Zustellwarteschlangen, nicht per FK gebundene Abrechnungszählungen
und Dateien berücksichtigen. Soft Delete ist dafür weder die endgültige
Löschung noch eine zwingende Vorbedingung. Ein generisches Löschen entlang
von CASCADE-FKs ist unvollständig und kann gemeinsam genutzte Daten treffen.

## Entscheidung

1. **Organisation & Tenancy verantwortet den Mandanten-Datenlebenszyklus.**
   Löschung und Datenrückgabe sind zwei Verfahren über denselben Bestand.
   Es entsteht kein neuer Owner. Alle bestehenden `data_objects`-Owner bleiben
   unverändert; die Katalogklassifizierung überträgt keine Schreibrechte.
2. **Benannte Ausnahme zu ADR 0067, Punkte 4 und 7:**
   `organization-tenancy.tenant-data-lifecycle` darf owner-übergreifend über
   eine eng berechtigte Datenbankfunktion löschen und über einen getrennten,
   schreibgeschützten administrativen Lesepfad exportieren. Einstieg ist
   jeweils ein CLI von Organisation & Tenancy, kein HTTP-/Schulzugriff und
   kein frei ausführbares Superuser-Skript. Für die Löschung gilt das Muster
   aus [ADR 0062](0062-demo-visitor-erasure-is-a-database-workflow.md):
   `SECURITY DEFINER`, fester sicherer Suchpfad, isolierte Ausführungsrolle,
   kein allgemeiner Tabellenzugriff für den Aufrufer. Vor Freigabe brauchen
   beide Pfade Rechte-, Mandantentrennungs-, Wiederholungs- und Fehlertests.
   Diese Ausnahme erlaubt weder fremde Go-Implementierungsimporte noch
   allgemeine Cross-Owner-Schreibzugriffe.
3. **Ein Katalog für beide Verfahren.**
   `modules/organizationtenancy/data_lifecycle_catalog.json` ist als
   Produktionsartefakt in die öffentliche `embed.FS`-Quelle
   `DataLifecycleCatalogSource` mit typisiertem JSON-Vertrag eingebettet. Er
   klassifiziert Tabellen, direkte Personen-FKs, Exportausschlüsse und Dateiablagen. Der öffentliche
   Owner-Standort erlaubt den späteren CLI-/Anwendungspfaden denselben
   Bestand zu lesen, ohne fremde Implementierungen oder einen neuen
   Plattform-Owner einzuführen. Tabellen- und Dateiauswahl bleiben explizit;
   `include` ist keine Erlaubnis für `SELECT *` oder ungescoped Kontenzugriffe.
4. **Keine zusätzliche Policy-Freigabe in diesem Slice.** Nach Prüfung von
   `backend/architecture/README.md`, `policy.json` und ADRs 0066–0071 entsteht
   hier weder ein Paket noch ein SQL-Lesepfad, Datenobjekt oder Import über
   eine Owner-Grenze. Der Migrationswächter liest das serialisierte
   Produktionsartefakt direkt, nicht einen zweiten Katalog. Deshalb bleiben
   Policy, Epoch, Projektionen und Eigentümerschaft unverändert. Die
   ausführenden Folge-Slices müssen ihre konkreten administrativen
   Datenbankwege unter dieser Ausnahme prüfen und nötige exakte
   Policy-Einträge nach ADR 0070 eintragen; keine pauschale Projektionsfreigabe
   für alle Tabellen auf Vorrat.

## Grenzen und Folgen

### Umsetzung Rückgabe, Slice 3838-a

Der interne CLI-Einstieg `tenant export` liegt bei Organisation & Tenancy.
Der neue Adapter `modules/organizationtenancy/internal/adapters/tenantexport`
besitzt keine Schreiboperation. Die Policy bildet die oben beschlossene
Ausnahme unter `organization-tenancy.tenant-data-lifecycle` mit genau den
bekannten exportierten Datenobjekten ab, ohne einen neuen Owner zu schaffen.
Die fünf Katalogtabellen ohne Runtime-Datenobjekt bleiben kataloggeführt;
es wird ihnen nicht nur für eine Lesefreigabe ein Schreib-Owner zugewiesen.
Der Architekturwächter akzeptiert ausschließlich diesen benannten
Owner-/Paket-/Rollenpunkt und dessen kataloggebauten `exportTable`-Lesecall.
Andere dynamische SQL-Aufrufe und owner-fremde Schreibzugriffe bleiben verboten.

Migration 1.15.489 führt `phoenix_tenant_export` ein: `NOLOGIN`, `BYPASSRLS`,
Leserechte über PostgreSQLs `pg_read_all_data`, keine Schreibrechte und keine
Zuweisung an die HTTP-Rolle. Das Wartungs-CLI wechselt lokal in diese Rolle
und prüft `REPEATABLE READ READ ONLY`. RLS-Bypass und Leserechte sind nicht
selbst die Mandantengrenze: ausdrückliche Tabellen-/Spaltenprojektionen aus
dem Katalog und Zwei-Schulen-Tests sichern den Exportumfang. Die Leserolle
darf technisch auch geheime Spalten lesen; deren Ausgabe verhindern die
getesteten Katalogprojektionen. Sie wird deshalb nicht als interaktiver
Kundenzugang bereitgestellt. Das Zurückrollen einer einzelnen Datenbank
entfernt die clusterweite Rolle nicht, da andere Datenbanken sie noch nutzen
können.

Der Fertig-Marker gilt nur für den technischen Katalogexport. Fachlich
erforderliche Ergänzungen, Verschlüsselung und produktiver Nachweis folgen
dem [Runbook](../operations/auskunft-und-rueckgabe.md). Die folgenden Aussagen
beschreiben den Ausgangsstand von Slice 3837-a und die weiter geltenden Grenzen.

- Der Katalog sagt **im Code klassifiziert**, nicht **Löschung/Export
  umgesetzt**. Slice 3837-a führt keine Migration, Löschung, Auskunft, CLI,
  Nachweistabelle oder Wiederholungsliste ein. 3837-b/c und 3838-a/b bauen
  darauf auf. Der produktive Nachweis ist offen.
- `TestEveryTableHasARetentionPath` prüft die regulären Löschwege und Fristen,
  `TestEveryTenantTableHasRLS` die RLS-Abdeckung. Die neuen Wächter prüfen
  stattdessen Klassifizierung, tatsächliche Spalten/FK-Pfade und
  Exportausschlüsse sowie bekannte Datei-Namespaces. Keiner ersetzt einen
  realistischen Zwei-Mandanten-Test des späteren ausführenden Verfahrens.
- Globale Konten und Avatare dürfen nicht allein deshalb gelöscht werden,
  weil eine Schule entfällt. Der bestehende Job `auth.erase_unused_accounts`
  bleibt maßgeblich. Nicht per FK gebundene `school_id`-Abrechnungszählungen
  brauchen eine ausdrückliche Entscheidung über Zweckbindung, Frist und
  Rückgabe. Globale Operator-Protokolle sind kein Roh-Gesamtexport einer
  Schule; rechtlich erforderlicher Kontext wird gesondert geprüft/redigiert.
- Geheime Spalten, Passkey-/Sitzungs-JSON sowie undurchsichtige Settings- und
  Zustellpayloads werden nicht roh ausgegeben. Nicht geheime Einstellungen
  und rechtlich erforderlicher Nachrichtenkontext müssen in 3838-a/b über
  geprüfte Feldprojektionen zurückgewonnen werden. Direkte Personen-FKs
  reichen nicht für eine vollständige Auskunft: indirekte Beziehungen,
  historische Schnappschüsse und JSON-Inhalte bleiben zuzuordnen.
- Datenbanken und Dateien sind nicht der ganze Bestand. Loki, Sentry,
  PostHog, journald und Sicherungskopien brauchen im späteren Verfahren
  gesonderte Prüfschritte, Zuständigkeiten und Nachweise. Sicherungen laufen
  nach dokumentiertem Zyklus aus; nach Wiederherstellung sind Löschungen vor
  regulärer Nutzung erneut umzusetzen. Dieser Slice bestätigt das nicht.
- Die Data-Act-Abruffrist und Wechselpflichten werden getrennt vom
  Löschauftrag geprüft. Hier wird weder ihr Beginn noch ihre Dauer entschieden;
  es entsteht keine pauschale 30-Tage-Wartefrist und kein jederzeitiger
  Selbstdownload. Auskunft und Rückgabe sind getrennte Zwecke.
- Der spätere Durchführungsnachweis enthält Auftragsreferenz, geprüfte
  Berechtigung, interne Schul-/Trägerreferenz, Verfahrens-/Katalogversion,
  Zeitpunkt, Schritte mit Anzahlen, Fehler-/Wiederholungsstatus und begrenzte
  Aufbewahrungsausnahmen mit Ablaufdatum. Keine Namen, Inhalte,
  Zugangsdaten oder unnötigen Dateinamen. Ein offener oder fehlgeschlagener
  Schritt verhindert den Gesamterfolg; ein DB-Erfolg bestätigt noch keine
  Dateilöschung oder Bereinigung von Sicherungen.

## Ergänzung: unabhängige Wiederholungsliste (3837-d)

Ein DB-Nachweis kann nach Restore eines älteren Stands die später vollzogene
Löschung nicht kennen. Organisation & Tenancy führt deshalb zusätzlich eine
append-only Hostdatei außerhalb der DB-/Upload-Volumes und Tagesbundles.
Sie enthält ausschließlich Schul-ID, Vorgangsreferenz, Abschlusszeitpunkt und
Quittungs-Prüfsumme. Erst ein synchronisierter Eintrag erlaubt CLI-Erfolg;
ein fehlgeschlagener Schritt wird mit demselben Auftrag nachgeholt.

Die Dateifähigkeit `organizationtenancy/erasureledger` ist als `public`
klassifiziert, analog zur bereits öffentlich konstruierten Dateiablage
`delivery/objects`. Der Root konstruiert sie und bindet sie an den
`ErasureLedger`-Port des Löschbefehls; die Owner-Composition baut weiterhin
DB-Adapter und Anwendung. Das hält die bestehende Compose-Fläche konstant.
Der Root bindet die bestehende `securityruntime.Fingerprint`-Fähigkeit für
SHA-256; es braucht keine neue Kryptografie-Freigabe und keine neue
Cross-Owner-Schreibberechtigung. Der bestehende Owner, DB-Vertrag,
Transaktionen und Rollen bleiben unverändert. Die Kette ist kein authentischer
Aktualitätsnachweis: eine getrennte aktuelle Kopie und ein unabhängig
verwahrter Dateiprüfwert gehören zwingend zum Betriebsverfahren.

Der Restore prüft diese Liste nach dem Import und sperrt die Freigabe bei
wieder vorhandenen Schulen, rückständiger Schul-ID-Sequenz oder
unlesbarer/beschädigter Liste. Später angelegte und wieder gelöschte Schulen
dürfen nach einem älteren Restore keine wiederverwendbaren IDs hinterlassen.
Wiederholung
läuft bewusst durch den bestehenden CLI, nicht durch einen neuen
Superuser-Löschpfad. Die schreibfreie Nachprüfung hebt den Review-Marker
nicht selbst auf. Die Liste läuft nach 97 Tagen aus (maximal 90 Tage für alle
Sicherungen plus sieben Tage Puffer), bei täglicher Bereinigung mit bis zu
24 Stunden Nachlauf und getrennten Vorgängerkopien für höchstens weitere
sieben Tage also insgesamt höchstens 105 Tage. Fristbereinigung setzt nachgewiesenen
Sicherungsablauf voraus. Produktive Rechte, Aktualität und Wirksamkeit bleiben
nachzuweisen, siehe [Runbook](../operations/tenant-erasure.md).

## Verworfene Alternativen

**Neuer Workflow-Owner mit Kommandos je Owner:** entspricht dem üblichen
Anwendungsworkflow, würde hier aber allein für den administrativen
Gesamtbestand einen umfangreichen privilegierten Kommandographen aufbauen.
Die eng begrenzte DB-Ausnahme bündelt die Vollständigkeitsprüfung, ohne die
Schreib-Owner neu zuzuordnen. Fachliche Einzelpersonen-Workflows bleiben
unverändert bei ihren bisherigen Ownern.

**Generisches Superuser-Skript:** ist kurz, bietet aber keine enge
Laufzeitberechtigung und keine klare Grenze für gemeinsam genutzte Konten,
Dateien, Aufbewahrung und Geheimnisse. Es wird nicht als Verfahren freigegeben.

## Quellen

- [Katalogpflege und offene Schritte](../reference/datenkatalog.md)
- [Löschfristen](../reference/loeschfristen.md)
- Legal-Repo, nur gelesen: `research/14-dsgvo-mindestumfang.md`,
  `vertragspaket/intern/checkliste.md`, `vertragspaket/03-avv.md`,
  Fassungen vom 23.10.2026. Sie sind kein Nachweis produktiver Wirksamkeit.

## Ergänzung: Aufbewahrung der Abrechnungszählungen (3837-b, Runde 2)

Die ausdrückliche Orchestrator-Entscheidung vom 23.10.2026 lautet:
`platform.billing_key_date_counts` wird bei der Mandantenlöschung **behalten
und gesperrt**, nicht gelöscht. Es enthält Organisationsnamen und Summen,
keine Daten betroffener Personen. Die unveränderten historischen Namen bleiben
als Rechnungsreferenz erforderlich (Migration 1.15.416); eine neutrale Kennung
würde die damalige Zuordnung nicht mehr allein belegen.

Nach der Schulentfernung darf nur die separate Abrechnungsrolle die Zeilen
lesen, nicht der reguläre Operator-Leser. Kein Export an Dritte. Die
Basistabellenrechte werden entzogen, weil `phoenix_admin` RLS umgeht; der
reguläre Leser nutzt eine View für noch vorhandene Schulen.

Die Frist beträgt vorsorglich **zehn Jahre ab Ende des Kalenderjahres des
Stichtags** mit Bezug auf § 147 AO / § 257 HGB. Die rechtliche Bestätigung
bleibt im Recht-Slice offen. Das Behalten ist später korrigierbar, das Löschen
der einmal erfassten Rechnungszahlen wäre unumkehrbar. Der getestete eigene
Löschweg und die Sperre sind im Code umgesetzt; regelmäßiger Aufruf,
Produktiv-Rollenprüfung und Wirksamkeitsnachweis bleiben offen.
Details: [DB-Vertrag](../reference/mandantenloeschung-datenbank.md) und
[Fristenliste](../reference/loeschfristen.md).

## Ergänzung: Trägerwurzel und Löschauftrag (#4657)

Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611) User
Story 11 verlangt das Löschen eines Trägers nach Vertragsende, nur nach
erfasster schriftlicher Bestätigung und mit Schonfrist. Die Ausnahme aus
Punkt 2 deckt dafür auch die Trägerwurzel; es entsteht kein neuer Owner und
keine neue Policy-Regel.

- **Löschauftrag:** `platform.organization_erasure_orders` (Schreib-Owner
  Organisation & Tenancy) hält Vorgangsnummer, Datum der schriftlichen
  Bestätigung und das Ende der Schonfrist, ohne FKs und ohne Namen. Das
  moto-Team erfasst und nimmt den Auftrag im Operator-Portal zurück, jeweils
  mit Operator-Audit. Erfassen setzt einen deaktivierten Träger voraus;
  Reaktivieren setzt die Rücknahme voraus.
- **Schonfrist:** Die Löschung darf frühestens am Tag nach der Erfassung
  laufen, der genaue Tag kommt aus der schriftlichen Bestätigung. Es gibt
  damit keine pauschale Wartefrist und keine vorgeschaltete Soft-Delete-
  Bedienhandlung; die Schonfrist ist das Zeitfenster für Rücknahme und
  Rückgabe.
- **Ausführung:** Einstieg bleibt `tenant erase --organization`. Der CLI fragt
  `platform.organization_erasure_check` vor der ersten Schule, löscht dann
  jede Schule über den bestehenden Schulvertrag und zuletzt die Wurzel über
  `platform.erase_organization` (`SECURITY DEFINER`, fester Suchpfad, nur
  `phoenix_school_erasure`). Die kaskadierenden FKs von Mitgliedschaft,
  Träger-Rollen und Träger-Sitzungen gehen mit; der Operator-Audit-Eintrag und
  der Abschluss des Auftrags entstehen im selben Schritt.
- **Grenzen:** Die unabhängige Löschungsliste bleibt bei Schul-IDs. Nach einem
  Restore wird der Trägerauftrag mit derselben Referenz erneut ausgeführt.
  Globale Konten bleiben wie bei Schulen dem Job `auth.erase_unused_accounts`
  überlassen; `platform.billing_key_date_counts` bleibt gesperrt erhalten.
