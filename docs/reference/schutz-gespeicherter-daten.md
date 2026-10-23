# Schutz gespeicherter Daten

Stand: 23.10.2026, Slice 3840-a, Teil von [#3840](https://github.com/moto-nrw/project-phoenix/issues/3840)
und [#3846](https://github.com/moto-nrw/project-phoenix/issues/3846).
Geprüfte Codebasis: `origin/development` bei
`d780efb392ef4d085a5731e615533eead3ee652b`.

## Ergebnis und Nachweisgrenze

Gewählt ist **Option 1 ohne Feld-/Dateiverschlüsselung in der Anwendung**:
Datenträgerverschlüsselung beim Hoster nachweisen, Sicherungen getrennt und
verschlüsselt betreiben, Diagnosezugriffe begrenzen und sensible Protokollkopien
minimieren. Entscheidung und Abwägung: [ADR 0093](../adr/0093-gespeicherte-daten-risikobasiert-ohne-feldverschluesselung-schuetzen.md).

**Das Schutzniveau ist nicht produktiv nachgewiesen.** Insbesondere fehlen
Datenträger-/Schlüsselnachweis (3840-c), eingeschränkte Diagnose-Leserolle
(3840-f) und produktive Backup-Abnahme (#3836). Gesundheitsfreitext bleibt
außerdem in einem Elternportal-Protokoll erhalten, siehe Befund unten.
Diese Punkte sind keine akzeptierten Ausnahmen und verhindern den Abschluss
der betreffenden Akzeptanzkriterien. Der Verzicht auf Feldverschlüsselung
ist weder eine Freigabe dieser Lücken noch eine Bestätigung laufender Sicherheit.

„Im Code umgesetzt“ bezeichnet hier den gelesenen Implementierungsstand,
nicht einen in diesem Slice ausgeführten Verhaltenstest. „Produktiv
nachgewiesen“ verlangt einen datierten Betriebsnachweis mit geprüftem Umfang.
Dieser Slice hat weder Produktions-/Stagingzugriff noch produktive Daten verwendet.
Wo „offen“ steht, liegt hier kein solcher Nachweis vor, nicht etwa der Beweis,
dass die Maßnahme im Betrieb fehlt.

## Bestand und Schutz im Code

Die Tabelle fasst die besonders schutzbedürftigen Bestände zusammen. Der
[gemeinsame Datenkatalog](datenkatalog.md) mit [ADR 0092](../adr/0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md)
bleibt die vollständige Tabellen-/Datei-Klassifizierung, nicht diese Auswahl.
Fristen und Löschpfade stehen in [Löschfristen](loeschfristen.md); es entstehen
hier keine neuen Aufbewahrungsfristen.

| Bestand | Im Code umgesetzt / gespeicherte Form | Produktiv nachgewiesen | Offen / Nachweisbedarf |
|---|---|---|---|
| `users.staff_financial_data`: `iban`, `tax_id`, `social_security_number` | `TEXT`, ohne Feldverschlüsselung; Finanzlesepfade maskieren standardmäßig und protokollieren Aufdecken [D1, D4] | Offen | Rollout, echte Rollen/DB-Zugänge, Datenträgerschutz; keine pauschalen Diagnose-SELECTs (3840-f) |
| `users.guardian_financial_data`: `iban`, `account_holder` | `TEXT`, ohne Feldverschlüsselung; IBAN standardmäßig maskiert, Aufdecken protokolliert. Kontoinhaber bleibt im Standardlesepfad sichtbar [D2, D4] | Offen | Wie oben; Maskierung der Anzeige ist keine Verschlüsselung des Bestands |
| `users.student_care_profiles.health_info` | `TEXT`, ohne Feldverschlüsselung; laufende Betreuungsangabe [D3] | Offen | Rollen, Datenträger und Erforderlichkeit je Einsatz; Gesundheitsangaben brauchen besonderen Schutz |
| Personal-, Eltern- und Kinderdaten, insbesondere `users.persons`, `users.staff_master_data`, `users.staff_school_memberships`, `users.guardian_profiles`, `users.student_profiles`, `users.student_school_memberships`, `users.student_notes` | Fachliche Stammdaten, Adressen, Beschäftigung, Beziehungen und Notizen ohne Feldverschlüsselung; Zuordnung und Löschpfade im Katalog [D5] | Offen | Tatsächliche Rechte, Löschläufe, Datenminimierung und Fehlablagen in Freitext prüfen |
| `audit.staff_master_data_changes`, `audit.guardian_financial_changes`: `old_value`, `new_value` | Finanzwerte beim Schreiben maskiert: IBAN-Ende bleibt sichtbar, Steuer-/Sozialversicherungsnummer und geänderter Kontoinhaber vollständig maskiert. Andere Personalwerte und freie Notizen sind nicht pauschal maskiert [D4] | Offen | Version/Bestand und Freitext prüfen; aus aktuellen Schreibern folgt kein vollständiger Nachweis historischer Bereinigung |
| `audit.student_field_edits`: `health_info` | Nur Änderungsmarkierungen; Migration `1.15.475` bereinigt vorhandene Gesundheitswerte. `extra_info` und weitere Freitexte bleiben inhaltlich erhalten [D6] | Offen | Rollout und Bereinigung nach Restore nach [Prüfablauf](../operations/gesundheitsangaben-aenderungsprotokoll-3840.md); Maskierung ist keine Verschlüsselung |
| `users.student_data_change_requests`: `old_value`, `new_value` bei `target=student`, `field_key=health_info` | **Klartext-Gesundheitskopien als JSONB**, auch bei `status=auto_applied`; nicht von `1.15.475` erfasst [D7] | Offen | Bestätigter Code-Mangel, Abhilfe einschließlich Altbestand und Restore fehlt; Bewertung unten |
| Zugriffs-/weitere fachliche Protokolle, insbesondere `audit.data_access_log` | Finanz-Aufdecken als eigener Zugriffstyp vorgesehen; weitere Protokolle enthalten Akteur-/Objektbezug und teils Personal-/Kinderdaten [D4, D5] | Offen | Nicht mit inhaltsfreier Protokollierung gleichsetzen; Rechte, Fristen und privilegierte Betriebszugriffe prüfen |
| Private Upload-Kinds `files`, `announcement-attachments`, `student-documents`, `staff-documents`, `student-photos`, `avatars` | Datei-Inhalte ohne Anwendungsverschlüsselung, etwa Atteste, AU, Lohnunterlagen und Fotos; bei privaten Object-Store-Schreibvorgängen 0600 für Datei und 0700 für Zielverzeichnis [D5, D8] | Offen | Altdateien, alle Schreibpfade, Eigentümer, übergeordnete Pfade, Mounts, Metadaten und verwaiste Dateien prüfen; Unix-Rechte schützen nicht vor Root |
| Öffentliche Upload-Kinds `login-images`, `enrollment-legal-documents`, `enrollment-form-legal-documents` | Explizit ohne Anmeldung bereitgestellt [D8] | Offen | Nur für öffentliche Inhalte verwenden; keine vertraulichen Dokumente oder personenbezogenen Fehlablagen |
| Datenträger/Volumes einschließlich PostgreSQL-Daten/WAL, Uploads, temporärer Dateien, Swap und Host-Snapshots | Keine Aussage zur Verschlüsselung beim Hoster aus dem Anwendungscode möglich | Offen | Mensch-Nachweis 3840-c nach [Prüfablauf](../operations/datentraegerschutz-3840.md), einschließlich Schlüsseltrennung und nicht abgedeckter Medien |
| Dumps und Release-Snapshots | Logische Kopien sind nicht durch die Verschlüsselung des Quelldatenträgers geschützt; Tagessicherung ersetzt keine Prüfung weiterer Kopien [D9] | Offen | Erzeuger, temporäre Klartextorte, Transport, Ziel, Zugriffe und Ablauf sämtlicher Kopien inventarisieren |
| Tagessicherung und bestehendes cron-Altverfahren | Neue DB-/Upload-Sicherung mit age, getrenntem Ziel und maximal 90 Tagen im Code vorbereitet. Alte Monitoring-Regeln belegen keine Eigenschaften des Altverfahrens [D9] | Offen | Einrichtung, Zieltrennung, Fristen, Alarmzustellung, Restore und Ablösung des Altverfahrens nach #3836 nachweisen |
| Schlüssel und Betriebsgeheimnisse | Kein Feld-/Dateischlüssel für diese Anwendungsdaten. Backup: öffentlicher age-Empfänger auf Quelle, private Identitäten nach ADR 0085 offline. SOPS für Betriebsgeheimnisse ist kein Schutz der DB-Inhalte [D9, D10] | Offen | Tatsächliche Schlüsselablage, Berechtigte, Wiederbeschaffung/Rotation, Hoster-Schlüssel und Root-/Deployment-Zugänge getrennt nachweisen |

**Mandantentrennung:** Tenant-Transaktionen mit `phoenix_tenant` und erzwungene
RLS sind im Code vorgesehen. `TestEveryTenantTableHasRLS` prüft Tabellen,
Aktivierung, FORCE und eine passende Policy. Das begrenzt Mandantenwechsel,
nicht den Zugriff eines berechtigten Kontos innerhalb seiner Schule, einen
DB-Superuser, Root oder den Inhalt eines Dumps. Belege: [Vertrag](../agents/contracts.md#tenant-boundary),
[RLS-Helfer](../../backend/database/migrations/rls_provisioning.go),
[Wächter](../../backend/database/migrations/rls_coverage_test.go).

**Datenbanktransport:** Die gelesene [Produktions-Compose](../../environments/production.compose.yml) deklariert den
Host-Port auf Loopback und `sslmode=require`. Das beweist weder die laufende
Netzkonfiguration noch Datenträgerverschlüsselung oder geprüfte Serveridentität;
die weitere TLS-Prüfung bleibt Slice 3842-d. Es wurden keine Verbindungen aufgebaut.

## Bedrohungsbewertung

Die Schadensschwere ist wegen Kinderdaten, Gesundheit, Identitätsmerkmalen und
Finanzdaten hoch. Es liegen keine Betriebsdaten für belastbare numerische
Eintrittswahrscheinlichkeiten vor. Die folgende Bewertung benennt deshalb
Angriffsbedingungen und Schutzgrenzen statt unbelegter Risikozahlen.

| Bedrohung und Folge | Gewählte Maßnahmen | Verbleibendes Risiko / Bewertung |
|---|---|---|
| Verlorener/ausgemusterter Datenträger: DB, WAL, Dateien oder Reste werden offline gelesen | Nachgewiesene Datenträger-/Volume-Verschlüsselung beim Hoster, geschützte getrennte Schlüssel, sichere Aussonderung; Swap/Temporärorte einschließen | **Offen, hoher möglicher Schaden.** Schutz erst belegbar, wenn konkrete Volumes, Medien und Schlüsselverfahren erfasst sind. Ein entsperrter laufender Host bleibt lesbar |
| Entwendeter Dump: vollständige logische DB-Kopie wird außerhalb des Volumes gelesen | Dumps nur zweckgebunden und zugriffsbeschränkt; im Backup-Code Klartext-Arbeitsbereich auf tmpfs, age vor Transport; andere Dump-/Snapshotpfade separat prüfen | **Offen.** Volume-Verschlüsselung schützt exportierten Klartext nicht. Feldverschlüsselung mit nicht im Dump enthaltenem Schlüssel könnte hier zusätzlich schützen; Option 1 setzt stattdessen lückenlosen Schutz der Kopien voraus |
| Gestohlene Sicherung: DB und Dateien samt Historie werden offengelegt oder alle Generationen vernichtet | #3836: verschlüsselte getrennte Sicherung, private Entschlüsselungsschlüssel offline, beschränkte Zugänge, Schutz vor Löschen aller Generationen, Frist und Restore-Test | **Offen bis Betriebsabnahme.** Zielzugänge auf dem Quellhost können ohne separate Zielkontrollen Löschung erlauben. Alte cron-Kopien und Release-Snapshots bleiben eigenständig zu prüfen |
| Unberechtigter interner Zugriff: Diagnose-/Betriebszugang liest Finanz- oder Gesundheitsdaten ohne Aufgabe | Aufgabenbezogene App-/DB-Rechte, Diagnose-Leserolle ohne sensible Spalten (3840-f), begrenzte persönliche Adminzugänge, Finanz-Zugriffsprotokoll, Datenminimierung | **Offen.** RLS ersetzt keine Spaltenrechte und sperrt keinen Superuser. Unix-Rechte und ein auf demselben Host verfügbarer Feldschlüssel schützen nicht vor uneingeschränktem Root-Zugriff |
| Kompromittierter Anwendungszugang: Angreifer liest alles, was Konto oder Prozess lesen darf | Rollen, Beziehungserlaubnisse im Elternportal, Tenant-Transaktionen/RLS, Zugangsschutz nach #3841, maskierte Standardanzeigen und minimale Protokolle | **Restrisiko bleibt, Wirksamkeit produktiv offen.** Ein berechtigtes kompromittiertes Konto kann Inhalte sehen; ein übernommener App-Prozess kann auch ihm zugängliche Schlüssel nutzen. Feldverschlüsselung mit demselben Prozess/Host als Entschlüssler löst diesen Angriff nicht allein |

Keine dieser Bewertungen akzeptiert pauschal die offenen Lücken. Vor Abschluss
von #3840 müssen die Nachweise vorliegen und der Gesundheitskopien-Befund
behoben oder mit einer konkret geprüften gleichwertigen Maßnahme behandelt sein.
Fehlt wirksamer Datenträger-/Kopienschutz, muss der Betreiber unmittelbar
Abhilfe und nötige Begrenzung der betroffenen Verarbeitung festlegen, nicht
bis zu einer späteren Neubewertung ungeprüft weiterfreigeben.

## Bestätigter Befund: Gesundheitskopien im Elternportal

**Auslöser und Speicherung:** Bei aktivierter Direktbearbeitung ändert ein
berechtigter Elternzugang `student/health_info`. `applyStudentEdit` erzeugt
Alt-/Neuwerte als JSON; `applyTrackAInTx` reicht sie an `recordAutoApplied`
weiter. Dieser schreibt sie unverändert in `users.student_data_change_requests`.
Gleichzeitig erzeugt People Directory einen bereits maskierten Eintrag in
`audit.student_field_edits`. Die Bereinigung `1.15.475` betrifft nur letztere
Tabelle. Belege: [D6, D7].

**Leser:** Technische DB-Zugänge mit SELECT auf die Tabelle sowie berechtigte
Mitarbeitende im Stammdaten-Anfrageverlauf können die alten Inhalte lesen.
`MasterDataReviews.ListHistory` lädt ausdrücklich `auto_applied`;
`ToMasterDataHistoryResponse` übernimmt `OldValue`/`NewValue` ohne Maskierung.
Die Anfrageberechtigungen und der Kind-/Schul-Scope gelten weiterhin, das ist
kein Beleg eines öffentlichen oder mandantenübergreifenden Lecks. Der normale
Eltern-Verlauf nutzt `ParentVisible` und filtert auf `person`/`departure`, nicht
`student`; daraus folgt kein allgemeiner Eltern-Lesezugriff auf diese Kopien.
Belege: [History-Leser](../../backend/modules/careplan/internal/application/master_data_reviews.go),
[HTTP-Projektion](../../backend/modules/requestreview/request_responses.go),
[Tenant-Abfrage](../../backend/modules/careplan/internal/adapters/postgres/requests.go),
[Eltern-Verlauf](../../backend/workflows/parentportal/internal/care/parent_master_data_request_service.go).

**Frist und Folge:** Die Tabelle hat keinen eigenen altersbasierten Löschlauf,
sondern folgt dem Kind (`parent("users.student_profiles")` im
[Fristenwächter](../../backend/database/migrations/retention_coverage_test.go)).
Der FK löscht beim endgültigen Löschen des Kindes mit; aktuell verweist er
laut [Cutover](../../backend/database/migrations/student_owner_compatibility.go)
auf `users.student_profiles`. Damit bleiben überholte Gesundheitsangaben
während der Betreuung und grundsätzlich bis zur Kinderlöschung nach Austritt
erhalten (Voreinstellung 365 Tage nach letztem Betreuungstag, konfigurierbar
30 Tage bis 3 Jahre). Die 90-Tage-Voreinstellung des Änderungsprotokolls pro
Kind gilt **nicht** für diese Tabelle. Die bestehende Kontenlöschung kann
Anfragen ebenfalls per FK entfernen, garantiert aber keine kürzere Frist.
Entfernen der aktuellen Gesundheitsangabe entfernt die alte Kopie nicht.
Backups enthalten sie bis zu ihrem eigenen Ablauf beziehungsweise der
Bereinigung nach Wiederherstellung.

**Bewertung:** Bestätigter Mangel an Datenminimierung, kein akzeptiertes
Restrisiko von Option 1. Die konkrete Nutzung/Anzahl produktiver Zeilen ist
unbekannt. Der Folgefix muss neue Klartextkopien verhindern, vorhandene Werte
gezielt und mandantenvollständig minimieren und Restore aus älteren Sicherungen
berücksichtigen. Regressionen müssen Hinzufügen, Ersetzen, Entfernen,
unveränderte Werte, unveränderte Live-Daten und Mandantentrennung abdecken.
Die Abhilfe ist vor Abnahme dieses Teilrisikos von #3840 erforderlich. Sie
ist hier nicht als kleiner Nebenfix enthalten, weil Altbestand/Migration und
Wiederherstellung dazugehören. Es wurde kein neues Issue angelegt.

`extra_info` und andere freie Notizen können sensible Fehlablagen enthalten.
Der Code speichert und protokolliert sie weiterhin inhaltlich [D3, D6]. Eine
regelmäßige Nutzung für Gesundheitsangaben ist daraus nicht belegbar.
Erforderlichkeit, Eingabehinweise und konkrete Fehlablagen muss der Betrieb
prüfen; keine pauschale Löschung oder weitere Maskierung in diesem Slice.

## Abschluss und erneute Bewertung

Der [Mensch-Prüfablauf 3840-c](../operations/datentraegerschutz-3840.md) führt
durch Datenträger-, Schlüssel- und Zugangsbelege. Unabhängig davon sind
[Backup-Abnahme](../operations/daily-backup.md), Diagnose-Leserolle 3840-f
und die oben genannte Abhilfe offen. Änderungen an Hosting, Schlüsseln,
Datenarten, Export-/Dumpwegen, Rechten, Kundenanforderungen oder ein Vorfall
lösen eine erneute Risikoprüfung aus. Notwendige Anpassungen dürfen nicht
allein mit Verweis auf Option 1 entfallen.

## Quellen der Codeprüfung

- **D1:** [Migration Finanz-/Personaldaten](../../backend/database/migrations/001015251_staff_stammdaten.go).
- **D2:** [Migration Eltern-Zahlungsdaten](../../backend/database/migrations/001015336_guardian_payment_data.go).
- **D3:** [Getrennte Kinder-/Betreuungsprofile](../../backend/database/migrations/001015393_student_owner_storage_expand.go).
- **D4:** [Personal-Finanzmaskierung](../../backend/modules/workforce/internal/domain/staffadmin.go), [Finanzlesepfad Personal](../../backend/modules/workforce/internal/application/staffadmin.go), [Eltern-Zahlungsservice](../../backend/modules/peopledirectory/internal/application/guardian_payment_service.go), [Zugriffstypen](../../backend/modules/auditlog/records/data_access_log.go), [Personal-Protokoll](../../backend/modules/auditlog/records/staff_master_data_change.go), [Eltern-Finanzprotokoll](../../backend/modules/auditlog/records/guardian_financial_change.go).
- **D5:** [Produktionskatalog](../../backend/modules/organizationtenancy/data_lifecycle_catalog.json), [Fristenliste](loeschfristen.md). Katalogabdeckung ist kein Nachweis ausgeführter Löschung/Rückgabe.
- **D6:** [Kind-Änderungsdiff](../../backend/modules/peopledirectory/internal/domain/student_audit.go), [Maskierung des Altbestands](../../backend/database/migrations/001015475_mask_student_health_audit.go).
- **D7:** [Eltern-Direktbearbeitung](../../backend/workflows/parentportal/internal/care/parent_master_data_service.go), [JSONB-Anfragetabelle](../../backend/database/migrations/001015151_create_student_data_change_requests.go), [Schreib-/Lesespeicher](../../backend/modules/careplan/internal/adapters/postgres/requests.go).
- **D8:** [Lokaler Dateispeicher](../../backend/modules/delivery/objects/local.go), [private Schreiboption](../../backend/modules/delivery/objects/private_uploads.go), [öffentliche Mounts](../../backend/api/serve_mounts_public.go). Die Rechte beim Schreiben belegen keine nachträgliche Bereinigung aller Altdateien.
- **D9:** [Backup-Implementierung](../../scripts/daily_backup.py), [Backup-Verfahren und Grenzen](../operations/daily-backup.md), [ADR 0085: Schlüsseltrennung](../adr/0085-backup-decryption-keys-stay-off-the-source-host.md).
- **D10:** [Betriebsgeheimnisse/SOPS](../agents/operations.md#environment-management-sops). Keine echten Schlüssel oder entschlüsselten Dateien wurden gelesen.

Rechtlicher Bewertungsrahmen, nur gelesen: Legal-Repo bei
`7a21f6742782bcf6141470a47333ae5be6c3182f`,
[Pflichtumfang](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/research/14-dsgvo-mindestumfang.md),
[Checkliste](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/intern/checkliste.md),
[AVV-Entwurf, Anhang 1 Ziff. 3/7](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/03-avv.md).
Maßstab ist die dort erläuterte risikobasierte Bewertung nach Art. 32 DSGVO,
keine allein aus einer älteren TOM abgeleitete Technologiepflicht.
