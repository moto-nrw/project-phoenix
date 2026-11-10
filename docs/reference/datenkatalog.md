# Gemeinsamer Datenkatalog für Löschung und Rückgabe

Stand: 23.10.2026. Entscheidung:
[ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md) und
[ADR 0097](../adr/0097-mandantenrueckgabe-hat-einen-eigenen-leseadapter.md).
Grundlage für #3837 und #3838, Slices 3837-a/b und 3838-a/b/c.

## Stand und Quelle

**Im Code klassifiziert:** 257 Tabellen und neun Upload-Kinds in
`backend/modules/organizationtenancy/data_lifecycle_catalog.json`.
`organizationtenancy.DataLifecycleCatalogSource` stellt denselben Katalog zur
Laufzeit als `embed.FS` bereit. `ReadFile("data_lifecycle_catalog.json")` liefert
eine unabhängige Byte-Kopie; JSON wird in `DataLifecycleCatalog` dekodiert. Kein zweiter
Lösch-/Exportkatalog. Der JSON-Drahtvertrag hat Version 1.

Ausgangsschema mit Migrationen bis 1.15.479: `users.external_caregivers`
ersetzt `users.guests` (1.15.477, ADR 0091). Der Eintrag bleibt `tenant` mit
`export: include` und dem Personen-FK `staff_id` auf
`users.staff_school_memberships.id`. Die Migrationen 1.15.478 (`ag-leitung`)
und 1.15.479 (Gast nur lesend) ändern Datensätze in den bereits erfassten
Tabellen `auth.roles`, `auth.role_permissions` und `auth.permissions`, keine
Tabellen oder Spalten. Die Tabellenzahl und die Spaltenausschlüsse bleiben
unverändert; 1.15.481 ergänzt anschließend die Nachweistabelle.
Globale Systemrollen mit `tenant_id IS NULL` sind kein
Schulbestand; erforderliche Rollenbedeutung muss die spätere Rückgabe
als Kontext liefern, nicht durch ungefilterten Export globaler Rollen.

Die Mittagessen-Migrationen 1.15.600 bis 1.15.604 ergänzen zehn Tabellen.
Bestellungen, Anmeldeänderungen, Einladungen und Belieferungen sind schulbezogen.
Caterer-Konten und Betriebe sind schulübergreifend; eine Schule darf sie nicht
allein löschen oder als eigenen Bestand ausgeben. Einladungs-Hashes bleiben
von der Rückgabe ausgeschlossen.

Migration 1.15.706 ergänzt den Elternsprechtag (#4801):
`calendar.parent_conferences` (Tag einer Lehrkraft für eine Klasse, Personen-FK
`organizer_staff_id` und das Konto für ihren Hinweis) und
`calendar.parent_conference_bookings` (gebuchtes Zeitfenster je Kind, mit
`student_id`, `guardian_profile_id` und dem Termin im Kalender). Beide sind
`tenant`, werden zurückgegeben und mit der Schule gelöscht. Der nächtliche Lauf
löscht Tage und Buchungen zum Beginn des neuen Schuljahres (1. August)
zusammen mit den Terminen der Buchungen.

Migration 1.15.721 ergänzt die Schulferien (#5345, ADR 0144):
`calendar.school_vacations` (Ferien je Bundesland aus der OpenHolidays API) ist
`platform`: dieselben Zeilen gelten für jede Schule eines Bundeslands, sie
gehören keiner Schule und werden nicht zurückgegeben.
`calendar.school_vacation_acknowledgements` (die Schule plant in einem
Ferienfenster bewusst Betreuung, mit dem bestätigenden Konto) ist `tenant`,
wird zurückgegeben und mit der Schule gelöscht.

Migration 1.15.758 ergänzt `platform.email_delivery_events` und
`platform.email_suppressions` (#5932): Zustellereignisse einer Mail und die
Adressen einer Schule mit endgültigem Fehler oder Beschwerde. Beide sind
`tenant`, werden nicht zurückgegeben (kein Schuldatensatz, nur Zustellhilfe) und
mit der Schule gelöscht; die nächtliche Delivery-Aufbewahrung löscht sie vorher:
die Ereignisse mit ihrer Mail, die Sperrliste ohne feste Frist, sobald niemand an
der Schule die Adresse mehr hat. Eine offene Anmeldung zählt mit (nicht
zurückgezogen, mindestens ein Kind wartet auf eine Entscheidung); eine
abgeschlossene nicht.

Migration 1.15.772 ergänzt `users.guardian_profiles.emails_stopped_at` (#5934):
der Zeitpunkt, zu dem die Schule die Mails an diesen Elternkontakt gestoppt hat.
Das Feld gehört zum Elternkontakt, gilt also auch nach einer Adressänderung, und
wird mit ihm zurückgegeben und gelöscht. Die Sicht
`platform.delivery_guardian_contacts` speichert nichts; sie nennt dem Versand die
Profil- und Kontoadresse je Elternkontakt samt Stopp.

Migration 1.15.727 ergänzt `platform.realtime_notifications` (#5504): eine
Kopie eines In-App-Hinweises mit Empfängern, die andere API-Prozesse zwei
Minuten lang lesen. Bei Eltern-Hinweisen enthält das JSON zusätzlich die
Schüler-IDs (`relay_student_ids`), damit die empfangende API die aktuellen
Beziehungsrechte vor der Zustellung erneut prüft. Die Sicht
`platform.delivery_realtime_recipients` speichert nichts; sie beantwortet je
Konto und Schule Zuordnung, Rolle und Kind-Freigabe für diese Prüfung. Die Kopie ist `tenant`, wird
nicht zurückgegeben (kein Schuldatensatz, nur ein Zustellhilfsmittel) und mit
der Schule gelöscht; die
nächtliche Delivery-Aufbewahrung löscht sie vorher. Beim Löschen eines
Demo-Besuchs entfernt `platform.erase_closed_demo_visitors` seit Migration
1.15.732 alle Kopien der ausgemusterten Demo-Schule mit (#5540).

**Im Code umgesetzt, Slice 3838-a:** `tenant export --school … --operator … --uploads … --out …`
liest den freigegebenen Katalogbestand in einer schreibgeschützten
`REPEATABLE READ`-Transaktion über eine eigene administrative Leserolle.
Direkte Tabellen werden per `tenant_id`, Kindtabellen per Katalog-FK-Pfad,
Schul-/Trägerwurzeln und gemeinsame Konten per ausdrücklicher Schulzuordnung
ausgewählt. Originaldateien, JSONL-Tabellen, Modellmetadaten und SHA-256-
Prüfsummen liegen in einem neuen privaten Verzeichnis. Ein Teilfehler
verhindert den Fertig-Marker. Zwei-Mandanten-Tests prüfen jede Katalogtabelle,
alle Datei-Kinds und alle ausgeschlossenen Spalten. Seit #6326 verlangt jeder
Lauf `--operator` und steht im Operator-Protokoll (ADR 0187, Addendum).
Ausführung, Übergabe und Nachweis: [Runbook](../operations/auskunft-und-rueckgabe.md).

**Im Code umgesetzt (Datenbankteil, 3837-b):** Migration 1.15.481 stellt
`platform.erase_school` und die getrennte Nachweistabelle
`platform.school_erasures` bereit. Ein aus diesem Katalog abgeleiteter,
installierter Plan enthält 241 Löschziele (seit Migration 1.15.605 mit den
Mittagessen-Tabellen) mit `tenant`, `tenantChild` oder
`school`. Die explizit gesperrten Rechnungsreferenzen
`platform.billing_key_date_counts` bleiben zehn Jahre ab Kalenderjahresende
erhalten und sind von der Datenrückgabe an Dritte ausgeschlossen (ADR 0092). Ein Wächter gleicht ihn mit dem Produktionskatalog ab.
FK-Reihenfolge, Trigger-Nachläufe, Restprüfung, Mandantentrennung und
Wiederholung nach DB-Fehlern werden lokal geprüft. Der Nachweis bleibt nach
dem DB-Schritt bei `db_done`, nicht `completed`.
[Vertrag und Grenzen](mandantenloeschung-datenbank.md).

**CLI und Dateiphase im Code umgesetzt (3837-c):**
[Runbook](../operations/tenant-erasure.md), Migration `1.15.490`.
**Personenauskunft im Code (3838-b/c):** `tenant export --person
kind:<id>|guardian:<id>|staff:<id>` leitet ein internes Prüfpaket aus demselben
Tabellenexport ab, mit benötigtem Kontext, referenzierten Originalen und
verbindlichen Zeilen-/Dateimarkierungen in `review.json`. Kontextpersonen
werden nicht zu neuen Auswahlwurzeln. Geheimnisausschlüsse bleiben wirksam.
Die manuelle Ergänzungs-/Drittpersonenprüfung und Schulfreigabe sind Bestandteil
des [Verfahrens](../operations/auskunft-und-rueckgabe.md#6-personenauskunft-auf-weisung-der-schule),
nicht durch `COMPLETE` erledigt.

**Offen:** produktiver Nachweis, manuelle Nebensystemschritte,
Wiederholung nach Restore (3837-d), organisatorische Fristenbestätigung
(3838-d) und Data-Act-Abgleich (3838-e).
Die Katalogeinträge selbst sind
keine Berechtigungen und kein produktiver Durchführungsnachweis.
Die breiten Ausschlüsse geheimer/undurchsichtiger Container bleiben wirksam;
nicht geheimer Settings-/Nachrichtenkontext und rechtlich erforderlicher
Kontext aus internen Protokollen brauchen weitere geprüfte Projektionen.
Der technische Export des freigegebenen Umfangs ist daher noch kein Nachweis
aller Rückgabe-/Data-Act-Pflichten von #3838.

**Produktiv nachzuweisen:** Einsatz und Wirksamkeit der Verfahren,
Nebensysteme, Sicherungszyklus und erneute Löschung nach Wiederherstellung.
Dieser Slice verwendet nur lokale Schema-Metadaten, keine produktiven Daten.

## Klassifizierung

Jeder Tabellenname hat dieselbe Form `schema.tabelle` wie im bestehenden
`tableRetentions`-Register. Die Register beantworten unterschiedliche Fragen:
reguläre Löschfrist dort, Mandantenzuordnung und Rückgabeumfang hier.

- `tenant`: besitzt `tenant_id`. Nur Zeilen des beauftragten Schul-Mandanten
  auswählen. Vorhandene NULL-Mandanten in System-Outbox/-Auth-Daten sind kein
  Auftrag, globale Daten mitzulöschen oder auszugeben.
- `tenantChild`: besitzt kein `tenant_id`. `parent` nennt Quelltabelle/-spalte
  und Zielspalte eines echten FKs; der Pfad muss eine `tenant`-Tabelle erreichen.
- `account`: globales Konto oder kontobezogener Bestand. Keine Löschung
  allein durch Schulzugehörigkeit. Bestehenden Kontenjob und Rechte anderer
  Schulen/Personen berücksichtigen; Rate-Limit-Daten haben nur einen
  E-Mail-Bezug und folgen ihrem bestehenden Aufräumjob.
- `school`, `organization`, `tenantReference`: Schul-/Trägerwurzeln bzw.
  bewusst nicht FK-gebundene Schulreferenzen. `scopeColumn` und `reason`
  machen die Auswahl ausdrücklich, statt sie aus CASCADE-FKs abzuleiten.
- `platform`, `demo`: begründeter moto-/Demobestand außerhalb des
  Kundenschul-Lebenszyklus, nicht pauschal „keine Personendaten“.

`export` ist `include`, `excludeSecret` oder begründetes `excludeInternal`.
Tabellenausschlüsse gelten für alle Spalten. Bei `include` gelten zusätzlich
`excludeSecretColumns`; `allowedSensitiveColumns` begründet bewusst
freigegebene verdächtige Namen, etwa Postleitzahl oder Inhaltsprüfsumme.
`openSealedColumns` ordnet einer feldverschlüsselten, in
`excludeSecretColumns` ausgeschlossenen Spalte einen neuen Klartext-Spaltennamen
zu: Die Rückgabe enthält den geöffneten Wert statt des Chiffrats, das ohne den
Schlüssel der Installation nutzlos ist. Der Besitzer der Tabelle öffnet ihn
über `TenantExportSealedFields` (#3838): IBAN und Kontoinhaber der
SEPA-Mandate und -Lastschriften (Bankring, ADR 0192) sowie die
Gesundheitsangaben der Betreuungsprofile, Ferienanmeldungen, -kinder und
-gäste (Gesundheitsring, ADR 0193, #6328). Die Ferienwerte stehen als der
JSON-Text, den sie halten.
`projectedColumns` ordnet einem ausgeschlossenen, untypisierten Container eine
neue Spalte mit seiner geprüften Projektion zu (ADR 0193). Der Besitzer
entscheidet über `TenantExportProjectedFields`, was Kundendaten ist: Settings
geben den Wert einer registrierten, nicht geheimen Definition zurück und `null`
für `password`-Definitionen und unbekannte Schlüssel; `platform.schools.settings`
gibt nur `logoUrl` und `loginImageUrl` zurück. Ein Rohcontainer verlässt den
Export nie.
`include` ist **keine Freigabe für Rohdaten, andere Mandanten oder andere
Personen**. Der Export wendet Öffnung und Projektion an; den Auftragszweck
prüft der Mensch. Zustellprotokolle (`platform.email_suppressions`,
`platform.email_delivery_events`, Push-Inhalte) gehen mit; der Mail-Payload
bleibt draußen, weil Sicherheitsmails dort Link oder Code tragen. Die übrigen
`excludeInternal`-Tabellen sind technische Zähler, Läufe oder Kopien ohne
eigenen Kundendatensatz. Ein Tabellenausschluss ersetzt die rechtliche Prüfung
von Auskunftskontext nicht.

Teilnahmen, Antwortbögen und Einzelantworten von Umfragen (#4781) bleiben als
`excludeSecret` außerhalb des Schulexports. Ihre Zeilen würden die Antworten
einer Person bündeln, und zwei Exporte im Vergleich würden auf die Antworten
derer zeigen, die dazwischen teilgenommen haben. Die Schule erhält die Summen
der Auswertung.

`personKeys` enthält direkte FK-Belege zu Kind, Erziehungsberechtigten,
Beschäftigten, Person, Konto oder Eltern-Kind-Beziehung. Der Wächter prüft die
Belege, nicht ihre Vollständigkeit für Art.-15-Auskunft. Indirekte Beziehungen,
Snapshots, JSON/Freitext und Drittpersonenrechte bleiben in #3838 zu bearbeiten.

Datei-Kinds verwenden `tenantDir` (`kind/tenant/name`), `tenantPrefix`
(`kind/tenant_name`) oder `account` (`avatars/global/name` und die Altablage
`avatars/<tenantId>/name`). Avatarverweise stehen in `auth.accounts.avatar` und
`users.profiles.avatar`; offene Löschaufträge in `auth.account_avatar_file_cleanup`
gehören ebenfalls zum Abgleich beider Ablagen. Neue Avatar-Dateien erhalten
`0600`, ihr Zielverzeichnis `0700`; die Altberechtigungen erfordern das Verfahren
in [Private Fotodateien](../operations/private-fotodateien-3840.md).
Auch 3837-c muss beide Ablagen berücksichtigen. Globale Avatare bleiben beim
geteilten Kontenlebenszyklus. Originaldateien, Metadaten und
offene Datei-Löschaufträge gehören zusammen. Ein verwaistes Objekt darf nicht stillschweigend fehlen.

## Pflege und Prüfung

1. Neue/umbenannte/entfernte Tabelle im **Produktionskatalog** ändern, auch
   technische Tabellen. Scope und Rückgabeumfang begründen. Die bisherige
   Fristen-/RLS-Klassifizierung unabhängig weiter pflegen.
2. Neue geheimnisverdächtige Spalten ausschließen oder ihre Freigabe mit
   konkreter Bedeutung begründen. Auch anders benannte Container prüfen:
   `settings`, Zustellpayloads und Credential-JSON können Geheimnisse enthalten.
   Enthält ein solcher Container Kundendaten, braucht er eine Projektion über
   `projectedColumns` beim Besitzer, keinen Rohexport. Feldverschlüsselte
   Spalten (`*_sealed`, `health_data`) brauchen `openSealedColumns`; der
   Stolperdraht erkennt beide Namensformen.
3. Neue Dateiablage mit Kind und Layout in denselben Katalog aufnehmen.
   Der Quellenwächter durchsucht Produktions-Go-Dateien nach Upload-Literalen,
   Upload-`Join`-Pfaden, Namespace-Konstanten, `TenantKey`-/Private-Object-Aufrufen
   und `ObjectStore.Kind`. Er ist kein vollständiger Datenflussbeweis:
   neue dynamische Konstruktionen oder andere Sprachen brauchen eine
   Erweiterung samt negativem Test, nicht einen stillen Ausschluss.
4. Prüfen, aus `backend/` über den Repo-Runner:
   `../scripts/run-go-toolchain.sh go test -tags=testhooks ./database/migrations ./modules/organizationtenancy`.
   Die lokale Test-DB initialisiert sich selbst. Wächter:
   `TestEveryTableHasDataLifecycleClassification` und
   `TestEveryUploadKindHasDataLifecycleClassification` sowie
   `TestSchoolErasurePlanCoversCatalogAndDeleteTriggers` und die
   `TestSchoolErasure*`-Verhaltenstests.
5. Ändert sich eine zurückgegebene Tabelle, Spalte, Beziehung oder ein
   Ausschluss, das Register des Rückgabepakets (Art. 26 Data Act,
   [ADR 0201](../adr/0201-mandantenrueckgabe-hat-ein-register-und-einen-abrufweg.md))
   im selben PR neu erzeugen:
   `../scripts/run-go-toolchain.sh go test -tags=testhooks ./modules/organizationtenancy/internal/adapters/tenantexport -run TestReturnRegisterMatchesMigratedSchema -update-return-register`.
   Der Test schlägt sonst fehl. Die API liefert die eingebettete Datei unter
   `/public/data-return/register`.
6. Bei Verfahrensänderungen den Katalogstand im späteren datensparsamen
   Durchführungsnachweis festhalten. Schemaabdeckung allein beweist weder
   Mandantentrennung des Verfahrens noch erfolgreiche Löschung/Rückgabe.

## Nicht im PostgreSQL-/Upload-Katalog

Loki, Sentry, PostHog, journald und Sicherungen sind separate Systeme.
Deren Bestände, Fristen, Zuständigkeiten, Fehlerbehandlung und menschliche
Nachweise müssen die Folge-Slices samt Betrieb konkret abdecken. Es wird
hier kein produktiver Lösch-/Exportnachweis vorgetäuscht. Der interne
Rückgabeablauf ist im oben verlinkten Runbook vorbereitet.
