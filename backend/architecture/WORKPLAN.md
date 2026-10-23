# Backend-Zielarchitektur #2580: Abschluss und Verlauf

Die Migration ist abgeschlossen. #2580 und die Konsolidierung #4069 sind
geschlossen. Die Belege stehen im
[DoD-Kommentar zu #2580](https://github.com/moto-nrw/project-phoenix/issues/2580#issuecomment-6278786747)
und im
[Abschlusskommentar zu #4069](https://github.com/moto-nrw/project-phoenix/issues/4069#issuecomment-6278782071).
Diese Datei ist keine Arbeitsliste mehr, sondern der Verlauf der Migration.

## Endstand

Stand `development` @ `7c00e51003` (20.10.2026). Maßgeblich ist die jeweilige
Quelle im Code; die Tabelle hält nur den Endstand fest.

| Register | Endstand | Quelle |
|---|---:|---|
| Ratchet-Keys | 0 | `backend/architecture/legacy.jsonl` |
| Legacy-Composition | 0 | `legacy_composition` in `policy.json` |
| Policy-Epoche | 88 | `policy_epoch` in `policy.json` |
| Regeln mit `issue`-Feld oder Übergangsmarker | 0 von 919 | `rules` in `policy.json`, `validateTargetOnlyRule` in `internal/architecture/target_rule.go` |
| Composition-Surface | 0 | `scripts/backend-architecture.sh check` |
| Legacy-LOC in `legacy/`-Bäumen und `legacy_*.go` | 0 und 0 | `backend/test/module_legacy_budget_ratchet_test.go` |
| Compose-LOC | 12.489 in 54 Paketen | `moduleComposeBudgetTotal`, ebenda |
| Pass-through-Methoden | 2.925 | `modulePassthroughOwnerBudgetTotal` in `backend/test/module_passthrough_ratchet_test.go` |

Die Regeln, die weiter gelten, stehen in den Prinzip-ADRs 0066 bis 0071
([Übersicht](README.md#architecture-principles)) und in `backend/CLAUDE.md`.

## Folgearbeit außerhalb von #2580

Den Stand zeigt jeweils das Issue:

- #3423: Code-Shape-Register der Modul-Ratchets; die Pass-through-Entscheidung liegt in #4403.
- #4408: ST1005-`nolint`-Direktiven.
- #4407: Care-Resume bei unbekanntem oder ausgeschiedenem Kind.
- #2809 und #2821: Träger-Portal an der Zielarchitektur.

## Historischer Verlauf bis 19.10.2026

Alles ab hier ist der Arbeitsstand während der Migration und bleibt
unverändert erhalten. Zähler, Budgets, Epochen sowie „offen“ und „vergebbar“
gelten für den jeweiligen Zwischenstand, nicht für heute. Die Tickets dieses
Verlaufs sind geschlossen, soweit sie oben nicht als Folgearbeit stehen.
Der frühere Kopf lautete: „Offene Tickets, geordnet nach Vergebbarkeit statt
nach Chronologie.“

Stand 19.10.2026 · Ratchet 0 · Composition 0 · Policy-Epoche 86 ·
0 Regeln mit Übergangsmarker oder `issue` (ADR 0057) · 0 Legacy-Verzeichnisse und
0 `legacy_*.go` · Compose-Cap 20.119 LOC.

`backend/architecture/legacy.jsonl` ist leer. Der normale Architekturcheck und
der Check ohne Baseline müssen beide mit null Verstößen bestehen. Der letzte
Data-Import-Autorisierungsport wird in der Root an Security Runtime gebunden.
Der Nachweis deckt den letzten Key von #4004 ab:
[Zero-Ratchet-Cutover](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/zero-ratchet-owner-cutover-4004.json).

| Carrier | Keys |
|---|---:|
| #4004 | 0 |
| #4076 | 0 |
| #4077 | 0 |
| #2751 | 0 |
| Summe = `wc -l backend/architecture/legacy.jsonl` | 0 |

Der leere Ratchet beweist nicht die restlichen Layout- und Rollen-Exits.
#4084 bleibt wegen der inventarisierten Laufzeitlogik in Kompositionen offen.
Die verbleibenden 16 Nicht-Root-Implementierungspakete liegen jetzt unter
`internal`. Fünf private Projektionsadapter liegen bei ihren Projektions-Ownern.
`cmd` bleibt eine Root-Rolle (ADR 0053/0059), `templates/email` enthält Templates und externe Tests, keine Produktions-Go-Datei.
Der Layout-Exit von #4086 ist damit belegt. Der vollständige Compose-Audit und
der verworfene Announcement-Port-Versuch stehen im
[aktuellen Exit-Audit](../../docs/architecture/final-legacy-cluster.md).
Die ADR-0020-Erweiterung akzeptiert nur geprüfte Pfadwechsel gleicher Grants
und Registereinträge; Tabellen, Rechte, Operationen, Gründe und Bodies bleiben gleich.
Die aktuelle Composition ist 0; der gemeinsame Vergleich beider Policy-Stände
zählt elf bereits vorhandene Delivery-Root/Test-Felder auf beiden Seiten.
Es kommt kein Feld hinzu.
Die folgenden Abschnitte enthalten den historischen Verlauf; ihre früheren
Zähler sind kein aktueller Stand.

#4078-Abschluss: Alle HTTP-Pakete gehören ihren fachlichen, Plattform- oder
Workflow-Ownern bzw. Root. `api/common` gehört zu `tenant-runtime/http`.
`architecture-check` und `dev-tools` sind echte `tool`-Owner (Schema 5),
keine Inbound-Aliasse. Ihre sechs Paket- und Testrollen bleiben unverändert.
Die fertigen Inbound- und Rollenübergänge samt Tests sowie der obsolete
Common-Akzeptor sind gelöscht. Parent- und HTTP-Host-Sicherungen verlieren
nur den veralteten Common-Zweig; heutige negative Fälle bleiben geprüft.
Inbound-Owner 2 → 0, #4078-Keys 0 → 0, Regeln 829 → 829. Nur
`own-test.inbound.contract` wird gleich eng zu `own-test.tool.contract`.
Epoche 72, Composition 1.657, Routen, Antworten, IoT, Goldens und Budgets
bleiben unverändert. Historische Basisformate sind lesbar, Live-Policies
lehnen `inbound` ab. Die normalen Ownership- und Berechtigungsprüfungen
haben keine neue Ausnahme. Nachweis: ADR 0060, Addendum vom 18.10.2026.

#4074: Communication verwendet vollständige native Workflow-Werte und eigene
Ports für Staff-Announcements und Elternnachrichten. Die Root bindet die
öffentlichen Owner-Fassaden und dieselben tenant-sicheren Personen- und
Guardian-Projektionen. Der Parent-Store projiziert seine eigenen SQL-Werte
unmittelbar in native Records; die fremden Modelle, Repository-Aliasse und
Retained-Konverter entfallen. Die ursprünglichen Fehlerinitialisierer liegen
einmal beim tatsächlichen Owner. Assertionen und DB-Zeitpunkt bleiben bei der
mechanischen Fixture-Umstellung erhalten. Der native Schulnamen-Aufruf nutzt
weiter den äußeren Decorator. Keine Änderung an SQL, Filtern, Berechtigungen,
Sessions, Routen, Antworten, Goldens oder IoT.
Fünf #4074-Keys und der #4076-Testbegleiter fallen: 119 → 113, nach unabhängigen
Development-Reduktionen aktuell 104. #4074 5 → 0, #4076 14 → 13. Eigener
Policy-Diff null; aktuell 842 Regeln, Epoche 72 und Composition 1.657.
Communication-Composition 1.481 → 1.480, People-Vokabular 110 → 106,
Compose-Gesamtstand 34.546 LOC. Keine neue Freigabe, kein Alias, keine
Ausnahme und kein höheres Budget.
Nachweis: [native Communication-Records](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-staff-announcements-4074.json).

#4074: Elternnachrichten verwenden vollständige native Workflow-Werte und
Ports. Die gebundenen Stores bieten native Facetten; Schul- und Gruppen-
Dekoratoren führen ihre Listen weiterhin über dieselben angereicherten Reads.
Die gemeinsamen Cursor- und Receipt-Regeln wandern einmalig und unverändert
in die eigene Application. Root-Callbacks binden denselben Conversation-Core
und dieselben Personen-, Schüler- und Kontakt-Reads. Unveränderte Payloads
behalten ihre ursprünglichen Typen und Maps; Teil-Mutationen samt Fehlern,
JSON-Ganzzahlen, nil/leere Werte und undurchsichtige Receipt-Payloads bleiben
sichtbar. Die unveränderte Deklarationsprobe liegt beim Announcement-Baustein;
dessen bestehender Carrier bleibt sichtbar. Drei Keys fallen: 123 → 120,
#4074 8 → 5. Regeln 843, Epoche 72 und Composition 1.657 bleiben gleich.
Communication-Compose sinkt 1.486 → 1.481 LOC, die Summe 34.577 → 34.572.
Keine neue Freigabe, Go-Typalias, Klassifizierung, Ausnahme, Root-Graph-Felder
oder höheren Budgets. Routen, Antworten, Goldens, IoT, Beziehungsrechte,
Sessions und Portal-Isolation bleiben unverändert.
Nachweis: [native Elternnachrichten](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-parent-messages-4074.json).

#4074: Der Staff-Store bindet native Records direkt an dieselben SQL-Stores
und dieselbe Inbox-Projektion. Die Root bindet Picker, Berechtigungsprüfung
und Rollen an eine Instanz derselben People-Directory-Abfrage. Der kanonische
Teilnehmerschlüssel wandert einmalig und unverändert in die Communication-Domain.
DB-Zeitstempel, Sperren, Cursor und die Metadaten-Rückgabe nur bei erfolgreichem
Insert bleiben erhalten. Ein Produktionskey fällt: 126 → 125, #4074 9 → 8.
Regeln 844, Epoche 72, Composition 1.657 und Compose 34.584 bleiben gleich.
Keine neue Freigabe, Go-Typalias, Klassifizierung, Ausnahme, Graph-Felder oder
höheren Budgets. Routen, Antworten, Goldens, IoT, Beziehungsrechte, Sessions
und Portal-Isolation bleiben unverändert.
Nachweis: [nativer Staff-Store](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-staff-store-4074.json).

#4074: Staff-Messaging verwendet native Workflow-Records und eigene Ports für
Threads, Nachrichten, Inbox und Empfänger. Dieselben gebundenen Stores bieten
öffentliche native Facetten neben den verbliebenen Aufrufern. Alle Metadaten,
Teil-Mutationen und Fehler, nil/leere Listen, Teilnehmer- und Read-Selektoren,
Sperren, DB-Zeitstempel und Cursor bleiben erhalten. Die Root bindet denselben
Personen-Read für die unveränderten Namensbestandteile; Trim und Fallback bleiben
im Workflow. Ein Produktionskey fällt: 127 → 126, #4074 10 → 9.
Regeln 844, Epoche 72, Composition 1.657, Communication-Compose 1.486 LOC und
Compose-Summe 34.584 bleiben gleich. Keine neue Freigabe, Go-Typalias,
Klassifizierung, Ausnahme, Root-Graph-Felder oder höheren Budgets. Die übrigen
Store-Keys bleiben sichtbar. Routen, Antworten, Goldens, IoT, Beziehungsrechte,
Sessions und Portal-Isolation bleiben unverändert.
Nachweis: [native Staff-Records](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-staff-records-4074.json).

#4074: Communication besitzt native Mail-Werte und schmale Lese- und Schreibports.
Die Root bindet dieselbe Outbox, denselben Mail-Transport und dieselbe Audit-Facette.
Der native Outbox-Einstieg teilt Validierung, Schul-Auflösung, Encoding und Insert
mit dem bisherigen Einstieg. Vollständige Metadaten, Fehlerreihenfolge, große
JSON-Ganzzahlen und nil/leere Schreibobjekte bleiben erhalten. Renderer lesen
die ursprünglichen String-/Bool-Werte ohne JSON-Roundtrip; Header, Template,
Inhalt und ausgelassene Schlüssel bleiben gleich. Die Audit-Bindung wandert an
die Root und erhält SetChanges-, Kontext- und Create-Fehlersemantik.
Zwölf Keys fallen: 139 → 127, #4074 22 → 10. Regeln 844, Epoche 72 und
Composition 1.657 bleiben gleich. Communication-Compose sinkt 1.514 → 1.486 LOC,
die Summe 34.612 → 34.584, das Dateigrößenbudget 1.183 → 1.178.
Keine neue Freigabe, Go-Typalias, Klassifizierung, Ausnahme, Graph-Felder oder
höheren Budgets. Routen, Antworten, Goldens, IoT, Beziehungsrechte, Sessions und
Portal-Isolation bleiben unverändert.
Nachweis: [native Communication-Mail](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-mail-4074.json).

#4074: Communication besitzt native Application-Events und schmale Delivery-Ports.
Die Root bindet denselben Router über öffentliche Commands. Alle Event- und
Audience-Felder, Consent, Schul- und Portal-Selektoren, Kontext, Transaktionen,
Fehleridentitäten sowie Teil-Receipts bleiben erhalten. Locale, Branding und
kanonische Push-Texte bleiben beim Delivery-Owner; die Texte wandern einmalig
in dessen bestehende Locale-Application. Zwölf Keys fallen: 151 → 139,
#4074 34 → 22. Regeln 844, Epoche 72, Composition 1.657 und Compose 34.612 LOC
bleiben gleich. Keine neue Freigabe, Go-Typalias, Klassifizierung, Ausnahme,
Graph-Felder oder höheren Budgets. Eine überholte Funktionslängen-Ausnahme
entfällt; das Dateigrößenbudget sinkt 1.188 → 1.183. Routen, Antworten, Goldens, IoT, Beziehungsrechte, Sessions und
Portal-Isolation bleiben unverändert.
Nachweis: [native Communication-Notifications](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-notifications-4074.json).

#4074: Communication besitzt eigene semantische Settings-Ports. Die Root
bindet fünf schmale öffentliche Settings-Facetten an denselben Resolver.
Schlüssel, Registry-Defaults, Overrides, Cache und Fehler bleiben beim Owner;
`false`, `0`, Schul-ID und Kontext bleiben erhalten.
Log-Schlüssel benötigen keinen weiteren Read. Fehlende Abhängigkeiten bleiben
`nil`, breite Service-Typen und ein Retention-Typalias entfallen. Sieben Keys
fallen: 158 → 151, #4074 41 → 34. Regeln 844, Epoche 72, Composition 1.657 und
Compose 34.612 LOC bleiben gleich. Das Dateigrößenbudget sinkt 1.189 → 1.188.
Keine neue Freigabe, Go-Typalias, Ausnahme oder höheren Budgets. Routen,
Antworten, Goldens, IoT, Beziehungsrechte, Sessions und Portal-Isolation bleiben unverändert.
Nachweis: [native Communication-Settings](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-settings-4074.json).

#4074: Communication besitzt schmale Ports für Schul-ID, After-Commit und
Detached-Transaktionen. Die Root bindet dieselbe öffentliche Tenant Runtime.
Eine native Transaction-Runtime-Callback-Factory erhält die konkrete Typprüfung
ohne ORM-Import oder Alias in der Root.
Hintergrund-Kontext, UnitOfWork, konkrete `bun.Tx`-Prüfung, Missing-DB-Guard,
Callback-Reihenfolge und Rollback bleiben erhalten. Ungenutzte DB-Slots entfallen;
Konstruktoren verlangen die Runtime ausdrücklich. Drei Produktionskeys fallen:
161 → 158, #4074 44 → 41. Regeln 844 und Epoche 72 bleiben gleich. Communication-
Compose sinkt 1.518 → 1.514 LOC, Transaction Runtime 173 → 172,
die Summe 34.617 → 34.612. Keine neue Freigabe,
Alias, Ausnahme oder höheren Budgets. Routen, Antworten, Goldens, IoT,
Beziehungsrechte, Sessions und Portal-Isolation bleiben unverändert.
Nachweis: [native Communication-Runtime](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/communication-native-runtime-4074.json).

#4073: Messaging verwendet vollständige native Workflow-Werte für Feed,
Deklarationen, Nachrichten, Inbox, Familienschutz und Request-Sharing. Die Root
bindet schmale öffentliche Owner-Facetten und projiziert eigene Antwortwerte.
Dieselben gebundenen Store-, Schul-, Gruppen- und Staff-Reads bleiben erhalten.
Die gemeinsame Conversation-Implementierung besitzt weiterhin Append, Cursor
und Receipts; Mutationen samt Fehlern werden zurückgegeben. Typisierte
JSON-Objekte erhalten `nil`, leere Objekte, `omitempty` und Ganzzahlen.
Die kanonischen Deklarations-Encoder berechnen Hash-Eingaben aus den aktuellen
Werten; es gibt keine Kopie der Regeln und keinen Cache kanonischer Bytes.
Familienschutz behält den Ereigniszeitpunkt, damit alte Shares nicht wieder gelten.
Der letzte Produktionskey fällt: 162 → 161, #4073 1 → 0. #4074 bleibt bei 44;
seine alten Adapterverträge bleiben sichtbar. Regeln 844, Epoche 72,
Composition 1.657 und alle Budgets bleiben gleich. Keine neue Freigabe, Alias,
Klassifizierung, Ausnahme oder höheren Limits. Routen, Antworten, Goldens,
IoT, Beziehungsrechte, Sessions und falsches Portal bleiben unverändert.
Nachweis: [native Messaging-Werte](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/parent-portal-native-messaging-4073.json).

#4073: Care verwendet eigene Plan-, Ausnahme-, Abwesenheits- und Angebots-DTOs
sowie schmale Ports. Die Root bindet die öffentlichen Care-Plan-Facetten;
die HTTP-Projektionen erhalten öffentliche Workflow-Snapshots. Cutoff-Prüfung,
lazy Uhr, Buchungsdatum und Fehlerursachen bleiben beim kanonischen Owner.
Die eigene Filterform enthält nur die vier verwendeten Selektoren. Metadaten,
Roh-JSON, `nil` und leere Listen bleiben erhalten. Die Compose-Konstruktoren
wandern an die Root; Auto-Excusal und optionale Test-Overrides bleiben erhalten.
Zwei Produktionskeys fallen: 164 → 162, #4073 2 → 1 und #4004 82 → 81.
#4074 bleibt bei 44. Regeln 844, Epoche 72 und Composition 1.657 bleiben gleich.
Portal-Compose sinkt 478 → 465 LOC, die Summe 34.630 → 34.617. Keine neue
Freigabe, Regel, Ausnahme, Alias oder höheren Budgets. Routen, Antworten,
Goldens, IoT, Beziehungsrechte und Portal-Isolation bleiben unverändert.
Nachweis: [native Care-Plan-Ports](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/parent-portal-native-care-plan-4073.json).

#4073: Care verwendet native Workflow-Werte für Beziehungen, Stammdaten-Anträge
und Ledger-Ereignisse. Die Root bindet die öffentlichen Owner-Facetten an eigene
Ports. Dieselben Abfragen und Sperren bleiben erhalten; gemeinsame Kontaktregeln
bewahren die kanonische Validierung samt Teil-Normalisierung vor Fehlern.
Fehler behalten Ursache und Text. JSON bewahrt Permission-Werte, leere Objekte
und große Ganzzahlen. Der Ledger nutzt denselben Insert mit `clock_timestamp`.
Ein Produktionskey fällt: 165 → 164, #4073 3 → 2; #4074 bleibt bei 44.
Regeln 844, Epoche 72 und Composition 1.657 bleiben gleich. Portal-Compose sinkt
von 479 auf 478 LOC, die Summe von 34.631 auf 34.630. Keine neue Freigabe,
Klassifizierung oder Ausnahme; keine neuen Typaliase und keine höheren Budgets. Routen, Antworten,
Goldens, IoT und Portal-Isolation bleiben unverändert.
Nachweis: [native Care-Werte](https://github.com/moto-nrw/project-phoenix/blob/46506b00c695a6f102865cae2dc9f24010371b7c/backend/architecture/parent-portal-native-care-records-4073.json).

#4073: Die äußere Workflow-Suite konstruiert Beziehungen, Telefone und
Schutzereignisse mit nativen Testwerten. Die Root bindet Test-Facetten derselben
Owner-Stores; Validierung, Eigentümer-Hälften, Zeitstempel, Fehler und Transaktionen
bleiben dort. Ledger-Assertions lesen native Werte über dieselben Abfragen.
Die Konstanten-Vertragsprüfung liest die tatsächlichen Modellwerte über den
komponierten Owner. Typunabhängige Sperr- und Fehler-Probes reichen dieselben
Zeilen durch; der Locale-Stub erhält einen nativen Profilwert.
Ein Test-Key fällt: 166 → 165, #4073 4 → 3; #4074 bleibt bei 44.
Die drei Produktions-DTO-Keys bleiben sichtbar. Keine neue Regel, Freigabe, Alias,
Klassifizierung, Ausnahme oder höhere Budgets; keine Produktlogik wird geändert.

#4073: Die reinen Stammdaten-Helfer vergleichen eigene Feld-Snapshots statt
Persistenzzeilen. Der Kontaktvergleich nutzt den vorhandenen Schreibport-Wert;
die Telefonnummernwahl erhält das unveränderte Primär-Prädikat als Funktion.
Der Offering-/Kurs-Stub leitet die unveränderte leere Kindzeile aus dem eigenen
Workflow-Leseport ab; dessen Produktions-DTO-Schuld bleibt sichtbar.
Die kanonische Abholarten-Policy bleibt unverändert und als Produktionsschuld sichtbar.
Die interne Suite nutzt native Werte und behält ihre Testfälle und Assertions.
Ein Test-Key fällt: 167 → 166, #4073 5 → 4; #4074 bleibt bei 44.
Keine neue Regel, Freigabe, Alias, Klassifizierung, Ausnahme oder höhere Budgets.
SQL, Transaktionen, Berechtigungen, Sessions, Routen und Antworten bleiben gleich.

#4073: Care-Verhaltenstests konstruieren Mitteilungen, Zielgruppen und Poll-Optionen
mit expliziten nativen Testwerten. Die Root bindet sie an denselben Communication-Store;
Defaults, Datenbankwerte, Zeitstempel und Fehler bleiben erhalten. Deklarationen,
Nachrichten und Stammdatenanfragen nutzen das bereits bestehende Workflow-Vokabular.
Das ist kein Produktions-DTO-Cutover: Die fünf übrigen Keys bleiben sichtbar.
Ein Test-Key fällt: 168 → 167, #4073 6 → 5; #4074 bleibt bei 44.
Keine Produktionsänderung, neue Regel, Freigabe, Alias, Klassifizierung oder höhere Budgets.
Alle bestehenden Tests und Assertions bleiben nach den Fixture-Ersetzungen erhalten.

#4073: Die Workflow-Komposition nimmt schmale, verbraucherseitige Ports statt
breiter `models/users`-Repository-Verträge. Messaging bekommt nur Feed-/Deklarationsaktionen,
Guardian-Inbox-Reads, Thread-Auflösung und Sharing; Care schreibt das Request-Ledger nur.
Die bisherigen Owner-Werte, Bindungen, Aufrufe und alle Testkörper bleiben unverändert.
Die privaten DTO-Abhängigkeiten von Care und Messaging bleiben offen und sichtbar.
Ein Produktionskey fällt: 169 → 168, #4073 7 → 6. Keine Regel, Freigabe, Alias,
Klassifizierung, Ausnahme oder höhere Budgets; Composition bleibt 1.657.
Parent-Compose sinkt 486 → 485 LOC, die Compose-Summe 35.380 → 35.379.

#4073: Die letzten privaten Schedule-Fixtures verwenden native Care-Plan-Werte.
Root-Testhelfer nutzen dieselbe Owner-Capability, vorhandene Record-Codecs und Fehlerabbildung;
Validierung, Filter und zurückgeschriebene IDs/Zeitstempel bleiben gleich. Der direkte
Arrival-Insert verwendet den bestehenden typisierten BUN-Fixture-Record; Kalender- und
Protokollwerte ändern sich nicht. Der doppelte Auto-Excusal-Testadapter entfällt zugunsten
des bestehenden Root-Bindings mit derselben Implementierung. Alle Assertions bleiben bestehen.
Zwei Test-Keys fallen: 171 → 169, #4073 9 → 7. Produktionscode, Regeln 838, Composition
und alle Budgets bleiben unverändert; keine neue Freigabe, Alias oder Ausnahme.

#4073: Der Deklarationsworkflow nutzt einen eigenen Policy-Port; die Root bindet
Communications öffentliche, reine Wertepolicy. Die bisherigen Algorithmen und ihre
Testkörper ziehen unverändert zur kanonischen Implementierung. Der verbleibende
Mitarbeiteradapter delegiert dieselben Regeln über seinen bestehenden Eigen-Owner-Zugriff.
Beziehungsrechte, Fristen, Widerruf, Fehler, kanonische Hashbytes, SQL, Routen und Antworten
bleiben gleich. Ein Produktionskey fällt: 172 → 171, #4073 10 → 9. Regeln 836 und
Composition 1.657 bleiben gleich; Parent-Compose 487 → 486 LOC, Compose-Summe 52.846 → 52.845.
Keine neue Regel, Freigabe, Klassifizierung, Go-Typalias, Ausnahme oder höhere Budgets.

#4073: Verhaltens-Fixtures binden native Testgrenzen statt fremder privater Pakete.
Claims verwenden denselben bestehenden Kontext-Helper; Presence und Messaging werden über
bestehende Root-Konstruktoren gebaut. Rollen-Fixtures erhalten kanonische Presets aus der
öffentlichen Security Runtime, ohne fremde Zeilentypen oder neue Aliase. Protokollwerte bleiben
gleich; ein nur auf Schlüssel geprüftes Audit-Register wird ein Set. Der unbenutzte breite
QueryOptions-Stub entfällt. Alle bestehenden Testkörper und Assertions bleiben nach den
deklarierten Fixture- und Konstanten-Ersetzungen gleich; zwei zusätzliche Prüfungen sichern
Fehler beim nativen Store-Aufbau. Acht Test-Keys fallen: 180 → 172, #4073 18 → 10.
Regeln 835, Composition 1.657, Compose 54.032 LOC und alle Budgets bleiben unverändert.
Keine neue Freigabe, Regel, Alias, Klassifizierung oder Ausnahme; Produktionsflows bleiben gleich.
Auch die zuvor veraltete Summenzeile 185 wird mit den tatsächlichen Baseline-Zeilen abgeglichen.

#4073: Rollenregeln, gespeicherte Permission-Werte und Deklarationshashes laufen über
eigene Workflow-Ports zur öffentlichen Security Runtime. Die Root bindet den bestehenden
Live-Event-Trigger und Deliverys öffentlichen Sprachkatalog; eine eigene Delivery-Anwendung
nutzt dafür denselben Locale-Adapter. Beziehungsgates, Rollen-Defaults, kanonische Bytes,
Sprachprüfung, Fehler, Routen und Antworten bleiben gleich. Der aufruferlose alte
ContentFingerprint-Helper entfällt; die öffentliche Security Runtime enthält bereits denselben Hash. Drei Keys fallen: 183 → 180,
#4073 21 → 18. Eine Compose-Regel entfällt ersatzlos: 836 → 835; keine neue Freigabe.
Composition bleibt 1.657; Parent-Compose 505 → 487 LOC, Compose-Summe 54.050 → 54.032.
Der Delegations-Ratchet sinkt gemessen von 22 auf 21, seine Summe von 3.091 auf 3.090.
Ein Anwendungspaket wird klassifiziert;
keine neuen Aliase, Ausnahmen oder höheren Budgets.
Der zuvor veraltete globale Stand 185 wird mit der gemessenen Baseline abgeglichen.

#4073: Parent-HTTP deklariert eigene Push-, Benachrichtigungs- und PWA-Ports.
Die Root übersetzt Deliverys öffentlichen Geräte- und Einwilligungsvertrag; dieselben
bestehenden Services führen weiterhin die atomaren schulübergreifenden Operationen aus.
Fehlerdiagnosen und Ursachen, leere Listen und JSON-Felder bleiben erhalten. Die Sprachprüfung
nutzt weiterhin den bestehenden Sprachkatalog. Drei Keys fallen: 197 → 194, #4073 29 → 26.
Regeln 865, Composition 1.657, Compose 57.188 LOC und alle Budgets bleiben unverändert.
Ein öffentliches Paket wird klassifiziert; keine neue Regel, Freigabe, Alias oder Ausnahme.

#4073: Care-Integrationstests bauen Kind- und Messaging-Flows über die öffentliche Workflow-API.
Dieselben Konstruktoren dienen der Produktionskomposition und delegieren unverändert an die
bestehenden Flows. Test-Historien verwenden eigene Port-DTOs statt Enrollment-Typen;
Deklarationshashes kommen über den bestehenden Root-Adapter zum Security Runtime und dieselben
kanonischen Bytes. Testfälle, Assertions, Fixture-Werte und Zeitgrenzen bleiben gleich.
Drei Keys fallen: 204 → 201, #4073 32 → 29. Regeln 877, Composition 1.657,
Compose 57.283 LOC und alle Budgets bleiben unverändert. Keine neue Regel, Freigabe,
Klassifizierung oder Checker-Ausnahme; auch keine Freigabe für eigene private Testziele.

#4073: Angebotskatalog und Identität einer Angebotsänderung laufen über eigene Workflow-DTOs.
Die Root übersetzt Enrollments öffentliche Reads. Die vorhandenen Owner-Codecs bleiben
im Leseweg: Dekodierfehler, Not-found-Marker, Katalogreihenfolge und leere ID-Filter ändern
sich nicht. Der Kalender hält nun die bereits bestehende Wochentagsübersetzung gemeinsam;
der Workflow delegiert an diese Implementierung; der unbenutzte Modell-Einstieg entfällt.
Drei `models/enrollment`-Keys fallen: 207 → 204, #4073 35 → 32. Regeln 878,
Composition 1.657, Compose 57.283 LOC und alle Budgets bleiben unverändert.
Keine neue Freigabe, Rolle, Klassifizierung, Paketverschiebung oder Regel.

#4073: Kind-, Phasen- und Einschreibungsleseports geben Workflow-eigene DTOs zurück.
Die Root übersetzt öffentliche Care-Plan-Projektionen; gespeicherte Berechtigungswerte
bleiben bei Security Runtimes gemeinsamem Prädikat. Personenfilter und Audience-Regeln
liegen nun in Care Plans Application statt in Compose; Record-Codecs liegen im eigenen
Postgres-Adapter. Admin-Transaktionen, Beziehungsschutz,
Fehler, Nil-/Leerslices, Kalenderdaten, JSON und Statusgrund-Redaktion bleiben gleich.
Fünf `models/parent`-Keys fallen: 214 → 209, #4073 40 → 35, Regeln 893 unverändert,
Composition 1.657 unverändert. Compose: Care Plan 4.509 → 4.334, Parent Portal 507 → 506,
gesamt 67.428 → 67.252 LOC; beide Register und die Summe sinken entsprechend.
Keine neue Freigabe, Klassifizierung, Rolle oder Paketverschiebung.

#4073: Kind-Ereignisse und Messaging-Aktivierungsprüfung laufen über eigene Workflow-Ports.
Die Root übersetzt Ereignisse in Communications öffentlichen, typisierten Elternvertrag und
bindet die gemeinsame Fail-open-Policy. Assertions, Empfänger-/Akteurtrennung, Referenzzeilen,
After-Commit-Fan-out und Beziehungsschutz bleiben gleich. Der deklarationsbezogene Zugriff
auf die Kompatibilitätsschicht bleibt vorerst bestehen. Zwei Keys fallen: Care und Compose
nennen keinen konkreten Emitter mehr. Keys 230 → 228, #4073 42 → 40, Regeln 902 → 902,
Composition 1.657 unverändert, Portal-Compose 507 LOC unverändert. Keine neue Freigabe.

#4078: Common liest keine Presence-Daten mehr. Der Location-Batchloader
liegt neben Snapshot und Resolver bei Student Presence; die unveränderte
ID-Deduplizierung ist für das Funktionslimit als eigener Schritt benannt; die HTTP-Aufrufer nennen
den bestehenden öffentlichen Vertrag direkt. Die 59 Tests ziehen mit unveränderten
Assertions mit. Die Produktionsregel `inbound-common.http.student-presence-public`
entfällt ohne Nachfolger: Regeln 837 → 836, Keys 183 → 183, Inbound-Owner 5 → 5.
Routen, Responses, Goldens, Composition und Budgets bleiben unverändert.

#4078: Der Meal-Plan-HTTP-Adapter liegt bereits unter `modules/mealplan/http` und gehört jetzt
zu `meal-plan`; `inbound-mealplan` und seine zwei Regeln sind gelöscht. Eine gleich enge
Eigen-HTTP-Regel ersetzt den Capability-Zugriff; die stehende Root-Reichweite ersetzt die andere.
Inbound-Owner 34 → 33, Regeln 952 → 951, Keys und Budgets unverändert. Der eng begrenzte
Checker-Übergang für bestehende Paketpfade entfällt mit seinen Tests im letzten
#4078-PR (`tool`-Kind), sobald kein Inbound-Owner mehr bleibt.

#4078: Auch `modules/feedback/http` gehört seinem echten Owner `feedback`.
`inbound-feedback` und seine drei Regeln entfallen; zwei gleich enge Eigen-HTTP-Regeln
ersetzen die bisherigen Capability- und Berechtigungsverträge. Die stehende Root-Regel
trägt den Mount. Inbound-Owner 33 → 32, Regeln 951 → 950; Keys und Budgets unverändert.

#4078: Die Staff- und Klassenlisten-HTTP-Adapter gehören zu `school-membership`.
`inbound-staff` und `inbound-classlistentries` entfallen. Zwei gleich enge Eigen-HTTP-Regeln
ersetzen vier Capability-/Berechtigungsregeln; die stehende Root-Regel ersetzt beide
Mount-Regeln. Inbound-Owner 32 → 30, Regeln 950 → 946; Keys und Budgets unverändert.

#4078: `modules/facilities/http/rooms` gehört zu `facilities`; `inbound-rooms` entfällt.
Die gleich enge Eigen-HTTP-Regel ersetzt die eine bisherige Capability-Regel. Der Import
von `facilities/compose/httpadapter` bleibt als exakter #4004-Key bestehen: Die stehende
Policy erlaubt der Komposition keine HTTP-Rolle. Inbound-Owner 30 → 29, Regeln 946 → 946;
Keys und Budgets unverändert.

#4078: `modules/emergencysnapshot/http` gehört zu `emergency-snapshot`.
`inbound-emergency` und vier Regeln entfallen; drei gleich enge HTTP-Regeln erhalten
Capability, Shared-HTTP-Runtime und Berechtigungsvertrag. Die Root-Reichweite steht bereits.
Inbound-Owner 29 → 28, Regeln 946 → 945; Keys und Budgets unverändert.

#4078: `modules/classday/http` gehört zu `class-day-view`; `inbound-classday` entfällt.
Sieben gleich enge Nachfolger ersetzen acht Regeln. Die nun ungenutzte generische
Regel `inbound.adapter-test.projection.public` entfällt zusätzlich. Die beiden eingehenden Schulportal-Regeln
bleiben auf denselben Paketpfad und ihre bisherigen Produktions-/External-Test-Scopes begrenzt.
Die Root-Reichweite steht bereits. Inbound-Owner 28 → 27, Regeln 945 → 943; Keys und Budgets unverändert.

#4078: File-Storage-HTTP gehört zu `file-storage`; `inbound-filestore` und die leeren
Owner `inbound-activities` und `inbound-database` entfallen. Fünf gleich enge
Nachfolger ersetzen sechs Regeln; die Root-Reichweite steht bereits. Die Upload-Suite
setzt denselben registrierten Schlüssel als Testwert statt über das fremde Modell.
Inbound-Owner 27 → 24, Regeln 938 → 937, Keys 316 → 315, #4078 11 → 10; Budgets unverändert.

#4075: Die letzten zehn Delivery-Cutover-Keys sind entfernt. Enrollment bindet Mail-Branding,
Organisation & Tenancy das PWA-Zeitfenster, School Structure die Ereignisse und die
Studentenrouten den Abwesenheitserzeuger über Consumer-Ports aus der Root. Der Erzeuger
von Anfrageentscheidungen liegt bei Delivery; Communication prüft weiter zuerst den
Kindzugriff. Tests prüfen Settings- und HTTP-Verträge ohne fremde Implementierungsimporte.
Keine neue oder gelöschte Regel, keine neuen Type-Aliase; `delivery-cutover.*` bleibt leer.
Compose-LOC 89.901 → 89.899, Application-Passthroughs 718 → 717, Keys 326 → 316.
`inbound-notifications` bleibt bei #4078; Scheduler und `cmd/devtools` sind unverändert.

#4078: `modules/peopledirectory/http` gehört zu `people-directory`; `inbound-users`
entfällt. Seine drei Regeln werden ohne neue Regel gelöscht: Die bestehenden
People-Directory-Regeln tragen den öffentlichen Vertrag und den Berechtigungsvertrag,
die stehende Root-Regel den Mount. Inbound-Owner 24 → 23, Regeln 940 → 937;
Paketpfade, Go-Code, Keys (307), #4078-Keys (10) und Budgets bleiben unverändert.

#4078: Der Elternportal-HTTP-Adapter gehört zu `parent-portal`; `inbound-parent` entfällt.
Die Owner-Zuordnung kommt vor der physischen Verschiebung: ADR 0020 verlangt bei einer
Relocation denselben Owner und dieselben Rollen. Die Verschiebung nach
`workflows/parentportal/http` ist ein eigener, schuldenneutraler Schnitt.
Die stehende Root-Reichweite löst den einen #2750-Key `api → parent` auf.
Laufzeit-Code, Routen, Antworten, Assertions und offene #4073-/#4004-Keys bleiben hier unverändert.
Der bestehende endliche Parent-HTTP-Epochenschutz folgt nur dem Owner-Namen;
Selektoren, Scopes und Test-Assertions bleiben gleich. Zehn Regeln entfallen, acht
gleich enge Nachfolger bleiben. Der Retirement-Nachweis liegt
in `parent-portal-http-owner-4078.json`; Composition und Budgets bleiben unverändert.
Nach dem Merge von #4147: Inbound-Owner 23 → 22, Regeln 936 → 934, Keys 303 → 302;
#2750 2 → 1, #4078 10 → 10 und #4079 15 → 15, Policy-Epoche 51 unverändert.

#4078: Parent-HTTP liegt nach ADR 0020 unter `workflows/parentportal/http`.
Owner, Produktions- und Testrollen sowie alle 52 Go-Dateinamen bleiben gleich.
Die Relocation benennt drei bestehende #4073-Keys und vier Funktionslängen-Einträge
nur um. Aktive Evidenz-Dateiverweise folgen den Dateien; historische Keys bleiben
unverändert. Keine neuen Regeln, höheren Budgets oder geänderten Assertions/HTTP-Verträge.
Policy-Epoche 54 → 55 nach dem Merge von #4156; 281 Keys und Composition 1.664 bleiben unverändert.

#4078: Data Import übernimmt seinen HTTP-Adapter; `inbound-import` entfällt.
Die Root hält die Middleware-/Identitäts-/Tenant-Verdrahtung und die fünf bisherigen
HTTP-Suiten; deren Assertions bleiben unverändert. Der Decoder kommt über die
bestehende Owner-Komposition. Workbook-Fixtures bleiben im Fileformat-Adapter,
seine Rendering-Testregel entfällt. Dreizehn Regeln werden durch eine gleich enge
Eigen-HTTP-Regel ersetzt; Inbound-Owner 22 → 21, Regeln 938 → 926,
Keys 281 → 279, #4078 10 → 9, #2750 1 → 0. Composition bleibt gleich; das
Compose-LOC-Budget sinkt um 131 und die verwaiste HTTP-ORM-Ausnahme entfällt.
Der Retirement-Nachweis liegt in `data-import-http-owner-4078.json`.
Die physische Verschiebung nach `modules/dataimport/http` folgt separat gemäß ADR 0020.

#4078: Statistik-HTTP gehört zu `student-presence`; `inbound-statistics` entfällt.
Die Root bindet dieselbe Berechtigungspaar-Prüfung, den Actor und den Export über
verbraucherseitige HTTP-Ports. Das Dokument wird unverändert in den bestehenden
Rendering-Vertrag übersetzt; keine zusätzliche Reichweite für andere HTTP-Pakete.
Zwei Suiten laufen unverändert über die Root und bestehende Fixture-Zeilen statt
über private Modelle; alle 144 Assertions bleiben erhalten. Sieben Regeln entfallen,
kein Nachfolger: Die stehenden Regeln tragen Public/Common und den Root-Mount.
Inbound-Owner 21 → 20, Regeln 925 → 918, Keys 273 → 270,
#4078 9 → 6; Composition 1.664, Budgets und #4079 bleiben unverändert.
Retirement-Nachweis: `statistics-http-owner-4078.json`.

#4078: Die IoT-Datenrouten (`api/iot/data`) gehören zu `process-device-scan`;
`inbound-iot-data` entfällt. Drei bestehende Regeln werden gleich eng ersetzt:
Eigen-Public, das öffentliche Feedback und die bestehende IoT-Komposition zum
HTTP-Adapter. Pfad, Produktions- und Testrollen bleiben gleich; kein Go-Code,
keine Route, Response, IoT-Zeichenkette oder Assertion ändert sich.
Inbound-Owner 20 → 19, Regeln 918 → 918; 270 Keys, #4078 6, Composition 1.664,
Epoche 55 und alle Budgets bleiben unverändert. Die physische Relocation folgt
separat gemäß ADR 0020.

#4078: IoT-Staffclock und die gemeinsamen IoT-HTTP-Fehler gehören zu
`process-device-scan`; `inbound-iot-staffclock` und `inbound-iot-internal-shared`
entfallen. Drei überflüssige Regeln werden gelöscht, ohne Nachfolger: Die
stehenden Scan-Public- und IoT-Kompositionsregeln decken die reklassifizierten
HTTP-Pakete. Pfade, Produktions-/Testrollen, Go-Code, Verträge und Assertions
bleiben unverändert. Inbound-Owner 19 → 17, Regeln 918 → 915; 270 Keys,
#4078 6, Composition 1.664, Epoche 55 und alle Budgets bleiben unverändert.

#4078: IoT-Check-in gehört zu `process-device-scan`; `inbound-iot-checkin`
entfällt. Die Root-Testunterstützung baut für die bestehende Kiosk-Suite exakt
wie zuvor das unbeobachtete Student-Presence-Modul. Nur Import und Konstruktor-
Aufruf wechseln; alle Assertions und Produktionshandler bleiben unverändert.
Drei Regeln entfallen, ohne Nachfolger. Inbound-Owner 17 → 16, Regeln 915 → 912;
270 Keys, #4078 6, Composition 1.664, Epoche 55 und alle Budgets bleiben gleich.
Keine IoT-Zeichenkette, Route oder Response ändert sich.

#4078: Data-Import-HTTP und die vier übernommenen IoT-Pakete liegen nach
ADR 0020 unter `modules/dataimport/http` bzw. `modules/devicescan/http`.
Alle 61 Go-Dateinamen, Owner, Produktions- und Testrollen bleiben gleich.
Import- und Registry-Pfade folgen den Dateien; der Funktionslängen-Eintrag
behält seinen Wert 66, historische Retirement-Keys bleiben unverändert.
Der IoT-Deklarations-Lookup folgt zwei verschobenen Dateien; kanonische
Golden-Identitäten, Code-Tabelle, Header und Assertions bleiben unverändert.
Der PyrePortal-Quellscan liest dieselben zwei verschobenen Dateien zusätzlich
über seine bestehende Quellliste; Code-Liste und Assertions bleiben gleich.
Der bisher nur unter `api/` liegende 139-Zeilen-Feedback-Handler wird für die
Modulgrenze in private Schritte unter 60 Zeilen geteilt, ohne neue Allowlist.
Row-Lock, Graduierten-Prüfung, Zeitstempel und Antworten bleiben in gleicher
Reihenfolge; ein Inlining-Audit reproduziert alle ursprünglichen Statements.
Epoche 55 → 56; 16 Inbound-Owner, 902 Regeln, 218 Keys, #4078 6,
Composition 1.657 und alle Budgets bleiben unverändert. Keine Route, Response,
IoT-Zeichenkette oder Assertion ändert sich.

#4078: `api/iot/sessions` gehört jetzt als HTTP-Adapter zu
`process-device-scan`; `inbound-iot-sessions` entfällt. Der Adapter erhält
Session End über seinen eigenen Callback, den Root an denselben Workflow
bindet. Close-Zeitpunkt, unveränderter Fehler, Status und Rollback bleiben
gleich. Drei bisher interne Handlerfälle laufen über diese Root-Bindung und
den öffentlichen Router; ihre elf Assertions und die 25 Assertions der
HTTP-Suite bleiben vollständig unverändert. Die vorhandene Root-Testkomposition
liefert die identischen Workflow-Werte, ohne neue Testrolle oder Regel.
Vier Regeln entfallen ohne Nachfolger. Inbound-Owner 16 → 15, Regeln
902 → 898; #4078 6, #4079 15, Composition 1.657 und alle Budgets bleiben
gleich. Keine Route, Response oder IoT-Zeichenkette ändert sich.

#4078: Die bereits übernommenen HTTP-Pakete für Geräteverwaltung und
Kiosk-Sessions liegen nach ADR 0020 unter `modules/devicefleet/http/devices`
und `modules/devicescan/http/sessions`. Alle 20 Go-Dateinamen, Owner und
Produktions- sowie Testrollen bleiben identisch. Ein AST-Audit über alle
24 verschobenen oder importierenden Dateien zeigt ausschließlich neue
Importpfade. Epoche 58 → 59; Regeln, Legacy-Keys, Composition und Budgets
bleiben gleich. Keine Route, Response, IoT-Zeichenkette oder Assertion ändert
sich; die vorangehende Sessions-Owner-Übernahme muss zuerst gemergt sein.

#4078: `api/absence-types` und `api/work-time-models` gehören zu Workforce/HTTP;
`inbound-absence-types` und `inbound-work-time-models` entfallen. Ihre unveränderten
Bindungen von Authentifizierung, Actor, Berechtigungen, Fehlern und
Nachbenachrichtigung liegen in Root. Die bisherigen Kompositionsfälle laufen
über diese Bindungen, ohne den veränderlichen `ComposeAbsenceTypes`-Testhook.
Alle 243 Assertions bleiben unverändert: 39 für Abwesenheitsarten einschließlich
der bisherigen Setup-Prüfung, 197 für Time Tracking und sieben für Arbeitszeitmodelle.
Time Tracking erhält seine elf Tenant-Transaktionen über einen Root-gebundenen
Port statt über Bun. Root validiert dieselbe Tenant-ID und bindet die öffentliche
Unit-of-Work-API; HTTP erhält weder Pool noch Transaktionsobjekt.
Fünf Regeln entfallen ohne Nachfolger, einschließlich der breiten HTTP-ORM-Regel
und der eingehenden Workflow-Test-Regel für Arbeitszeitmodell-HTTP.
Inbound-Owner 15 → 13, Regeln 893 → 888, Legacy-Keys 214 → 212; Composition bleibt gleich.
Zwei erledigte #4004-Kanten des bisherigen Compose-Adapters entfallen mit
Nachweis in `workforce-configuration-http-owner-4078.json`.
Das Compose-Budget sinkt von 67.428 auf 67.360 LOC, der HTTP-Adapter-Eintrag von
81 auf 13. Der Dateigrößen-Eintrag für Time Tracking sinkt von 863 auf 862;
der ORM-Ausnahmeeintrag entfällt. Keine Route, Response oder Assertion ändert sich.

#4078: Die verbleibenden Workforce-HTTP-Adapter für Vertretungen, Schichtarten
und Dienstpläne gehören Workforce/HTTP; ihre drei Inbound-Owner entfallen.
Root bindet und mountet dieselben Ressourcen direkt. Die vorhandenen Suiten
laufen dort ohne den veränderlichen `ComposeShiftTypes`-Hook. Der Kategorie-Lookup
bleibt eine lokale Funktion des Test-Routers statt eines Mehr-Owner-Containers.
Time Tracking erhält den unveränderten Dienstplan-Serializer als Root-gebundenen
Konstruktor-Port; es importiert kein anderes HTTP-Paket mehr. Seine Resource-
Deklarationen liegen in `resource.go`, ohne Handler- oder Callback-Änderung.
Elf Regeln entfallen ohne Nachfolger. Inbound-Owner 13 → 10, Regeln 888 → 877,
Legacy-Keys 207 und Composition 1.657 bleiben gleich. Das Compose-Budget sinkt
um die letzten 13 LOC des leeren HTTP-Adapters auf 59.260; dessen Paketeintrag
und Budget entfallen. Der Dateigrößen-Eintrag für Time Tracking sinkt von 862
auf 804. Alle 356 bisherigen Assertions bleiben unverändert; die vollständigen
Bindungs- und Handler-Deklarationen sind bis auf Paketnamen und Port exakt.

#4078: Die sechs übernommenen Workforce-HTTP-Pakete liegen nach ADR 0020
unter `modules/workforce/http/{absencetypes,worktimemodels,shifttypes,staffshifts,substitutions,timetracking}`.
Alle 69 Go-Dateinamen, Owner und Produktions- sowie Testrollen bleiben identisch.
Nur Importpfade, eingefrorene Pfadschlüssel der Größen-/Längen-/Komplexitätsregister
und der aktuelle Evidence-Quellpfad folgen der Relocation. Epoche 60 → 61;
Regeln, Legacy-Keys, Composition und Budgetwerte bleiben unverändert.
Die bisher unter `api/` liegende 67-Zeilen-Übersichtsfunktion fällt erstmals
unter die Modul-Längengrenze. Der unveränderte Personal-Mapper ist als benannter
Schritt extrahiert; Wiedereinsetzen ergibt denselben vollständigen AST. Keine neue
Allowlist-Zeile. Keine Route, Response, IoT-Zeichenkette oder Assertion ändert sich.

#4078: SSE gehört Delivery Platform/HTTP; `inbound-sse` entfällt. Root bindet
die bisherigen JWT-/Portal-Router, skalare Identität und Admin-Prüfung,
Token-Deadline und Tenant-Transaktion. Die HTTP-eigenen typisierten Event-,
Client- und Hub-Ports verwenden weiterhin die nativen Eventwerte, Client-Zeiger
und denselben 32er-Kanal; keine Relay-Goroutine und kein zusätzlicher Puffer.
Die vorhandene Testkomposition liefert den echten Root-Runtime-Wert. Vier
Auth-/Router-Fälle liegen in Root; sämtliche 81 Assertions bleiben identisch.
Zwei ownerweite Provider-/Rendering-Regeln werden durch sechs engere Rollen-
und Scope-Regeln ersetzt. Mit den sechs Inbound-Regeln entfallen acht Regeln;
Regelzahl 877 → 875, Inbound-Owner 10 → 9, Keys 204 → 203, #4078 6 → 5.
Keine neue Reichweite, kein Selektor, kein Root-Service-Graph-Feld. Composition
bleibt 1.657. Der bereits gemergte typisierte Transaktionsport aus #4184 bleibt über die
Root-Bindung erhalten; die vorhandene
Längen-Ausnahme des School-Stream-Handlers entfällt: Der unveränderte
Registrierungsschritt ist benannt und der öffentliche Handler liegt unter 60 Zeilen. Der Einzel-Key
ist in `delivery-sse-http-owner-4078.json` belegt. Routen, Responses und Goldens
bleiben unverändert; Communication bleibt der letzte `inbound-*`-Schnitt.


#4078: Display-HTTP gehört Device Fleet; `inbound-display` entfällt. Root bindet
unveränderte JWT-, Tenant-, Berechtigungs- und Fehler-Middleware und hostet die
bisherigen Workflow-Tests als route-sized Komposition. Alle 156 Assertions
bleiben identisch, der mutable Test-Kompositionshook entfällt. SQL-Not-found-
und Transient-Klassifikation liegen im eigenen Postgres-Adapter; Root bindet
sie über den HTTP-eigenen Port. Display-Token-Minting und -Hashing liegen als
unveränderter Credential-Adapter hinter dem bestehenden Token-Port statt in
Compose. Die ownerweite Krypto-Regel wird auf Produktion/Adapter eingeengt;
die HTTP-ORM-Regel entfällt. Elf Regeln entfallen, eine engere folgt:
875 → 865. Inbound 9 → 8, Keys 200 → 197 (#4004 86 → 83), Composition
unverändert 1.657. Compose sinkt um 95 LOC: 57.283 → 57.188, Device Fleet
712 → 701 und der leere HTTP-Compose-Eintrag (84) entfällt. Keine Selektoren,
keine neue Reichweite, keine höheren Budgets oder Root-Service-Graph-Felder.
Die drei #4004-Retirements sind in `display-http-owner-4078.json` belegt.
Routen, Responses, IoT-Strings, Statuscodes und Goldens bleiben unverändert.

#4078: IoT-Info und der IoT-HTTP-Host gehören Device Scan; `inbound-iot`
entfällt. Root bindet die nativen HTTP-Blätter über typisierte Konstruktor-Ports,
JWT-/Device-Guards, Tenant-Transaktion und Metrics. Der Host importiert keine
fremde HTTP-Ressource. Device Fleets öffentlicher HTTP-Runtime-Facade liefert
dieselben Principal-Fakten, den Fail-closed-Guard und den nativen Fehler-Renderer.
Sentry-I/O bleibt im IoT-Adapter; Root bindet nur DSN-Parsing, Projekt-Tagging und
die bisherigen Fehler-Wrapper. Kein zusätzlicher HTTP-Aufruf, Puffer oder Worker.
Die echten Konstruktor-/Nil-Verträge bleiben im Owner; route-sized Tests bauen
über Root und vorhandenen Test-Support. Alle 69 Assertions bleiben identisch.
`TestFullProductionRouterGolden` bleibt vollständig identisch, Golden-Dateien
ändern sich nicht. Nur der exakte umgezogene Metrics-Symbolname wird im bestehenden
Provenienz-Normalisierer abgebildet. Achtzehn Regeln entfallen ohne Nachfolger;
865 → 847, Inbound 8 → 7, Keys 194 → 193 (#4004 83 → 82), #4078 fünf unverändert.
Composition 1.657 und alle Budgets bleiben gleich. Der einzelne #4004-Key ist
in `iot-http-owner-4078.json` belegt. Testrollen, Routen, Middleware-Reihenfolge,
Responses und PyrePortal-Strings bleiben gleich; keine Selektoren, Schemaänderung,
neue Freigabe, mutable Hooks oder Root-Service-Graph-Felder.

#4078-Vorbereitung für School-HTTP: Die bisherigen JWT-Claim-Prüfungen, der
stabile Quota-Identity-Read und die unveränderten Opaque-Token-Primitiven liegen
in `modules/identityaccess/tokenprimitives` unter dem öffentlichen Identity-&-Access-
Vertrag. Session Tokens behalten alle APIs und nativen SDK-Typen und delegieren
unverändert. Der Quota-Read liefert typisierte Fakten, keinen öffentlichen Filter-Map.
Token-Authentifizierung, Claims, Token-Entropie, Fingerprint und Quota-Format bleiben
gleich. Keine Owner-/Key-/Regel-/Budgetänderung, keine Selektoren. Die noch ownerweite
Krypto-Regel wird erst mit dem School-Owner-Schnitt rollen-/scope-eng eingegrenzt;
dessen gelöschte Inbound-Regeln finanzieren die nötigen engeren Nachfolger.

## Konsolidierung · [#4069](https://github.com/moto-nrw/project-phoenix/issues/4069)

Entscheidungen vom 15.10.2026 nach dem Architektur-Audit: Nur das Ziel ist eine Regel (ADR 0057), der
Zielgraph ist azyklisch (ADR 0058), die Roots halten keinen Service-Graphen (ADR 0059), Laufzeitregeln
kommen als letzter Schritt. Die Definition of Done von #2580 ist überarbeitet. [#4070](https://github.com/moto-nrw/project-phoenix/issues/4070)
hat die Übergangsregeln in Keys umgewandelt (Policy-Epoche 46 → 47): 405 Regeln gelöscht, 514 Keys
aufgenommen, Ratchet 6 → 520, Regeln 1.351 → 976. Der Anstieg ist gewollt; er zeigt die Schuld, die die
Regeln versteckt haben. Die Validierung lehnt seitdem ab: Regeln in fremde private Rollen und fremde
`models/*` (ADR 0057 a, b), Plattform → Fach, Projektion, Workflow (c), Übergangsmarker und `issue` (d)
und Owner-Kompositionen in fremde `compose`- oder `http`-Rollen (ADR 0059 Punkt 6). Seit #3417
(Policy-Epoche 48) lehnt sie auch einen Zielgraphen mit zyklischer Komponente ab (ADR 0058 Punkt 1):
14 zyklusschließende Regeln sind gelöscht, ihre 18 Importe Keys unter #4079 (15) und #4076 (3),
Ratchet 480 → 498, einmalig und nur für diese Umwandlung freigegeben. Die Schichtung steht im Nachtrag
zu ADR 0058. `inbound`-Owner liegen außerhalb des Zielgraphen; `settings-platform.http.inbound-common`
schließt keinen Zyklus und bleibt Regel. #3990: Repository-Vokabular aus `models/users` verschieben,
ohne auf den Ersatz von `database/repositories/users` zu warten. #4071 misst, wohin Code gewandert ist:
Composition zählt jedes Root-Struct mehrerer Owner (139 → 1.732, Messgröße für #4080), Passthrough die
`compose`-Pakete (2.394), dazu `legacy_*.go`, LOC je `compose`-Paket (Messgröße für #4084) und die
Handler-Ratchets unter `modules/*/http`, `*/inbound`; Budget-Erhöhungen lehnt der Check gegen die Base ab.
Test-Hooks aus dem Produktions-Build: [#4094](https://github.com/moto-nrw/project-phoenix/issues/4094).

- [ ] Phase 0: [#4070](https://github.com/moto-nrw/project-phoenix/issues/4070) Übergangsregeln → Keys (erledigt), [#4071](https://github.com/moto-nrw/project-phoenix/issues/4071) Messlücken der Ratchets, danach #3417
- [ ] Phase 1: #4063 (ADR 0056), #4064, #4067, #2750, #3990 → #2729
- [ ] Phase 2, Struktur: [#4079](https://github.com/moto-nrw/project-phoenix/issues/4079) Mehr-Owner-HTTP, [#4082](https://github.com/moto-nrw/project-phoenix/issues/4082) Fachlogik aus der Root, [#4080](https://github.com/moto-nrw/project-phoenix/issues/4080) `api.serviceGraph` auflösen, [#4083](https://github.com/moto-nrw/project-phoenix/issues/4083) Worker-Root, [#4084](https://github.com/moto-nrw/project-phoenix/issues/4084) `compose` konstruiert nur, [#4085](https://github.com/moto-nrw/project-phoenix/issues/4085) Vokabular-Aliase, [#4086](https://github.com/moto-nrw/project-phoenix/issues/4086) Altlayout, [#4091](https://github.com/moto-nrw/project-phoenix/issues/4091) Prinzipien-ADRs
- [ ] Phase 3, Laufzeit (nach #2751): [#4087](https://github.com/moto-nrw/project-phoenix/issues/4087) Outbox, [#4088](https://github.com/moto-nrw/project-phoenix/issues/4088) Tenant- und Transaktionskontext, [#4089](https://github.com/moto-nrw/project-phoenix/issues/4089) Fehler per Sentinel, [#4090](https://github.com/moto-nrw/project-phoenix/issues/4090) Testlücken, dann #3021

## Entscheidungen

- [x] [#3387](https://github.com/moto-nrw/project-phoenix/issues/3387) Legacy-Guardian-Felder auf den Schüler-Endpunkten — blockt #2760
- [x] [#3417](https://github.com/moto-nrw/project-phoenix/issues/3417) Zyklen — ADR 0058 mit Nachtrag zur Schichtung: People Directory unten, Care Plan darüber, Enrollment über Care Plan, Workforce über Student Presence; 14 zyklusschließende Regeln als 18 Keys (#4079, #4076), Policy-Epoche 47 → 48 lehnt jeden Zyklus im Zielgraphen ab (`internal/architecture/acyclic_target.go`), Ratchet 480 → 498
- [x] [#3414](https://github.com/moto-nrw/project-phoenix/issues/3414) Projection-Owner `operator-dashboard-view` bleibt (ADR 0033)
- [x] [#2747](https://github.com/moto-nrw/project-phoenix/issues/2747) Wo Code liegt, der mehrere Owner über eine Datenbank zusammensetzt: ADR 0053. Owner-Kompositionen bauen nur ihren eigenen Owner, die typisierten Roots (`.`, `api`, `cmd`) setzen Owner zusammen (stehende Reichweite, [#4001](https://github.com/moto-nrw/project-phoenix/issues/4001)); Entscheidungen und Schreibvorgänge über Owner hinweg gehören in Owner oder Workflows; die Datenbank kommt als ein Wert der Transaction Runtime ([#4002](https://github.com/moto-nrw/project-phoenix/issues/4002)); Tests bauen die Roots
- [x] [#3487](https://github.com/moto-nrw/project-phoenix/issues/3487) `legacy/jwt` zieht jetzt nach `modules/identityaccess/sessiontoken` um (ADR 0054); die Substanz von ADR 0031 bleibt, nur der Zeitpunkt ändert sich
- [x] [#3446](https://github.com/moto-nrw/project-phoenix/issues/3446) Rolle der Identity-Behavior-Suiten: eigene Suiten von Identity, Punkt bleibt (ADR 0053, Punkt 7)
- [x] [#4004](https://github.com/moto-nrw/project-phoenix/issues/4004) Fremd-Reichweite der Owner-Kompositionen — entschieden in ADR 0059 Punkt 6: in fremde Implementierungsrollen exakte Schuld (Keys mit [#4070](https://github.com/moto-nrw/project-phoenix/issues/4070), Abbau unter #4004), in fremde `compose`/`http` in die Root, `public`/`contract` nur in Richtung von ADR 0058
- [x] [#4063](https://github.com/moto-nrw/project-phoenix/issues/4063) Reichweite der typisierten Roots zu HTTP-Runtime, CLI-Werkzeug, Dev-Tools und den Session-, Tenant-, Worker- und Analytics-Adaptern: ADR 0056. Die Server-Root ist der HTTP-Host (Router, Server, CORS, globale Middleware-Kette, Routentabelle), hält aber keine HTTP-Ressourcen; Policy-Epoche 45 → 46 mit sechs stehenden Regeln unter `reviewedHTTPHostRule` (`http-router`, `api/common`, Session-Adapter in Produktion und internem Test, `cli-config` im internen Test, `dev-tools`), `middleware` und `analytics` als `http` ihrer Owner unter `root-composition.reach.platform.http` (#4064); 15 Keys weg (21 → 6), der Worker-Vertrag zieht unter [#4067](https://github.com/moto-nrw/project-phoenix/issues/4067) um, die beiden Ressourcen der Root unter [#4068](https://github.com/moto-nrw/project-phoenix/issues/4068)
- [x] Träger-Strang [#2809](https://github.com/moto-nrw/project-phoenix/issues/2809) / [#2821](https://github.com/moto-nrw/project-phoenix/issues/2821) gehört nicht zu #2580 (Entscheidung vom 15.10.2026: Produktfeature, #2580 schließt sie aus)

## Jetzt vergebbar · 9 Keys

- [x] [#4001](https://github.com/moto-nrw/project-phoenix/issues/4001) Policy-Epoche 40 → 41: stehende Reichweiten von ADR 0053 (Root, Owner-Komposition, Tests, Fixture-Ziel Session-Adapter) — 0 (zwölf Keys weg: neun #2750, zwei #3446, einer #2748; die drei Root-Regeln in fremde Implementierungsrollen ergeben vier exakte Keys unter [#4003](https://github.com/moto-nrw/project-phoenix/issues/4003), 55 → 47; vier Akzeptanzfunktionen in `internal/architecture/composition_reach_rule.go`, 79 neue Regeln ohne `issue` (19 Root, 22 Owner-Komposition, 36 Eigen-Tests und Eigen-Test-Support, 1 Root-Test, 1 Fixture-Ziel `identity-access/adapter`), 301 abgedeckte Regeln gelöscht, darunter vier temporäre (#2750, dreimal #2747), `e2e-test.compose` auf `legacy-composition` verengt und unter #2747; Regeln 1.663 → 1.438, Composition 252 → 252, Zielgraph-Zyklen unverändert)
- [x] [#4002](https://github.com/moto-nrw/project-phoenix/issues/4002) Ein Datenbank-Wert der Transaction Runtime, Roots ohne `bun` — 0 (`transactionruntime.Database` hält Pool und Unit of Work, schließt den Pool und registriert Query-Hooks; `api`, `cmd` und `.` halten nur diesen Wert; die vier #2750-Keys und `cmd -> database` weg, 60 → 55)
- [x] [#4014](https://github.com/moto-nrw/project-phoenix/issues/4014) Fixtures bilden Care-Offering-Zeilen selbst ab — 0 (`test -> models/enrollment` entfernt, #2748, 60 → 59; ohne Regel und ohne Fixture-Ziel)
- [x] [#2733](https://github.com/moto-nrw/project-phoenix/issues/2733) services/enrollment, repositories/enrollment — 0 (`database/repositories/enrollment` gelöscht, 7 Keys weg; die `services/enrollment`-Keys fallen über #3558 bis #3565; #3559 hat den Care-Offering-Katalog nach `modules/careplan` verschoben, ohne Key; #3560 die Buchungs-Materialisierung, zwei erledigte Keys entfernt; #3561 die Angebotswechsel-, Kurs- und Abholzeit-Reviews, vier erledigte Keys entfernt, darunter `crypto/subtle`, `legacy/jwt` und `services/parentmessaging`; #3562 Phasen, Formulare, Captcha und Eltern-Mails nach `modules/enrollment/internal/application`, sechs erledigte Keys entfernt, darunter `email`, `pgdriver` und `net/http`, Policy-Epoche 30 → 31, Kompositionsregel per ADR 0041; #3563 Klassentag, Aufsichtsblatt und Ankunftsausnahmen nach `modules/classday`, Care-Usage- und Klassenlisten-Reports nach `modules/enrollment`, sechs erledigte Keys entfernt, darunter `internal/collation`, `internal/sliceutil` und `models/education`, acht verwaiste Regeln gelöscht, ohne neue Regel; #3564 Entscheidung, Wiederherstellung, Rollover, Löschung und Aufbewahrungs-Cleanup nach `modules/enrollment`, acht erledigte Keys entfernt, darunter `models/audit`, `models/base`, `models/schedule`, `realtime`, `services/import` und `bun.DB.RunInTx`, ohne neue Regel; #3565 Intake und Änderungsanträge nach `modules/enrollment`, `services/enrollment` gelöscht, die letzten 45 Keys entfernt, 14 verwaiste Regeln gelöscht, ohne neue Regel)
- [x] [#2742](https://github.com/moto-nrw/project-phoenix/issues/2742) services/education, repositories/education, api/groups, api/admin — 0 (alle 63 Keys weg, dazu `api -> api/admin` und `api -> api/groups` (#2750), `services/users -> models/education` (#2728), `models -> models/education` im internen Test (#2751) und `test -> database/repositories/education` (#2748), 350 → 282; `api/groups` Datei für Datei nach `modules/schoolstructure/http` (`school-structure`/`http`), `api/admin` nach `workflows/gradetransition/http` (`grade-transition`/`http`), Owner `inbound-groups` und `inbound-admin` gelöscht, ihre Regeln an die neuen Punkte verschoben; `services/education` über eigene Ports und das `models/education`-Vokabular, Tenant-Runtime aus `modules/schoolstructure/compose`; `database/repositories/education` ohne `base`, `tenant` und Fremdtabelle `education.class_arrival_times`; zwei verwaiste Regeln gelöscht, Composition 597 → 596)
- [x] [#2728](https://github.com/moto-nrw/project-phoenix/issues/2728) services/users — 0 (#3354: fünf erledigte Keys entfernt, darunter `cmp`, `realtime` und `services/parentmessaging`; #2742: `models/education` in Produktion entfernt; #3556: `models/education` im externen Test entfernt; #3753: Person- und Schülerdienste nach `modules/peopledirectory/compose`, sieben Keys entfernt, darunter `internal/timezone`, `models/config` und `services/config`; #3754: Betreuungsfähigkeit nach `modules/peopledirectory/compose`, Sperre und Blocker über Owner-Ports, `database/sql` und `models/activities` in Produktion entfernt, eine verwaiste Regel gelöscht, ohne neue Regel; #3755: Erziehungsberechtigte nach `modules/peopledirectory/compose`, Audit und Rollen-Presets über Ports, `services/users` gelöscht, die letzten 22 Keys entfernt, dazu `services -> services/users` (#2747) und die beiden #3421-Keys des Elternportals, 164 → 139; die Suiten in den neuen Vertragstest `modules/peopledirectory/contracttest` (`people-directory`/`e2e-test`), sieben Regeln dorthin verschoben, zwölf neue am neuen Punkt, Composition 490 → 490)
- [x] [#2734](https://github.com/moto-nrw/project-phoenix/issues/2734) api/enrollment — 0 (alle 26 Keys weg, dazu `api -> api/enrollment` (#2750) und drei #3421-Keys des Elternportals, 239 → 209; `api/enrollment` Datei für Datei nach `modules/enrollment/http` (`enrollment`/`http`, die externen Suiten als `workflow-integration-test`), Owner `inbound-enrollment` gelöscht, seine Regeln an den neuen Punkt verschoben; kein `bun` und kein `database/sql` mehr, die Transaktionen laufen über `tenant.WithinTenant`/`WithinAdmin`; Anmeldungen und Angebote als eigene dekodierte Werte über dem öffentlichen Enrollment-Vokabular statt `models/enrollment`, der Angebotskatalog über `enrollmentCompose.CareOfferingValues`, das Autofill-Profil über den Enrollment-Port `GuardianAutofillReader` (gebunden in `services`), die Exporte über den Vertrag `modules/documentrendering/lists`, Kalender über `sharedkernel/calendar`; das Elternportal erreicht den Formularfluss über seinen Port `EnrollmentForms` statt über den Import; Composition 591 → 591)
- [x] [#2738](https://github.com/moto-nrw/project-phoenix/issues/2738) api/common — 0 (20 Keys weg, 262 → 242; gemeinsame HTTP-Schicht bleibt, gezielte Owner-Verträge per ADR 0046 / #3898, Policy-Epoche 33 → 33; Settings über öffentlichen Vertrag, Guardian-Renderer mit HTTP-Eingabewerten, SQL-Fehler beim Transaction Runtime, Byte-Speicherung nativ bei Delivery; DB-tragende Route-Wrapper, ungenutzte DB-Verdrahtung und `internal/storage` gelöscht; 8 ORM-Regeln und 12 HTTP-ORM-Ratchet-Einträge entfernt, Composition 596 → 596)
- [x] [#2727](https://github.com/moto-nrw/project-phoenix/issues/2727) database/repositories/users — 0 (alle 14 Keys weg, 402 → 388; Tenant und Transaktion über die Runtime aus `modules/peopledirectory/compose`, Fehler- und Query-Formen über das `models/users`-Vokabular, Kalender über `sharedkernel/calendar`, Berechtigungen über `modules/securityruntime`; die Suiten komponieren über `api/testutil` → `services` und benennen Zeilen über `test`; eine stale Communication-Regel gelöscht, Composition 603 → 603)
- [x] [#2729](https://github.com/moto-nrw/project-phoenix/issues/2729) models/users — 0 (#3757: `internal/timezone` in Produktion, internen und externen Tests durch `sharedkernel/calendar` ersetzt; die drei Kalender-Keys entfernt, 265 → 262; keine neue Regel, Composition 596 → 596; #3991: Zeilen-Basis lokal (`users.Model`, `users.TenantModel`), die drei externen Tests in `package users`, `models/base` im internen und `models/users` im externen Test entfernt, 86 → 84, mit dem #3421-Key 86 → 83; offen: der Produktions-Key `models/users -> models/base` für das Repository-Vokabular in `repository.go` und `repository_vocabulary.go`, fällt mit #3990; eine Ersatzregel lehnt der Checker als Lockerung ab, eine Issue-Neuzuordnung des Keys ebenso; #4070: die sieben Regeln mit `issue` #2729 sind gelöscht, ihre acht Importe von `models/users` sind Keys, 1 → 9; #3990: Repository-Verträge, Abfrage- und Fehler-Vokabular nach `database/repositories/users` (eigene Typen, kein Alias auf `models/base`), die drei Root-Nähte vorher typisiert (#4105 Präsenz, #4107 Workforce, #4109 Timetable), Record-Fehler der Root über `transactionruntime.RecordErrors`, Unique-Prüfungen über `transactionruntime`; `models/users -> models/base` und `workflows/shiftplansync/compose -> models/users` entfernt, 9 → 7, Composition 1.743 → 1.741; Workforce-Dienstplan und Shift Plan Sync lesen Mitarbeitende als `workforce.PlanStaff`, die Root bildet die People-Directory-Zeilen über `api.NewPlanStaffDirectory` ab, 7 → 1; offen: `modules/timetable/compose` im internen Test; Tagesinformationen als `models/schedule.StaffNotice`, die Timetable-Ports lesen `timetable.StaffMember`, `timetable.Person` und `timetable.LifecycleStudent`, die Root bildet die People-Directory-Zeilen ab, 1 → 0)

## Legacy-Nester auflösen · 31.074 LOC, 0 Keys

- [x] [#3424](https://github.com/moto-nrw/project-phoenix/issues/3424) `modules/timetable/legacy/timetableplanning` aufgelöst — 5.710 LOC (Schnitt S6 Schulkalender erledigt: Zeiträume, Schließtage, Feiertage, Wochenzyklus und Dateframes beim Owner `modules/schoolcalendar`, Budget 24.299 → 23.093, `planexport/legacy` 410 → 359, Legacy-Summe 44.517 → 43.250, 3 Kompatibilitätsregeln weg statt konvertiert, Policy-Epoche 22 → 23, Ausnahme für die 7 Ersatzregeln per ADR 0038; Schnitt S4 Konflikterkennung und Personalplanung erledigt (#3550): Konflikte, Personalpool, Dienstplan-Abdeckung und Personalbedarf beim Owner `modules/timetable`, Budget 23.093 → 20.326, `planexport/legacy` 359 → 337, `supervisiondashboard/legacy` 729 → 722, Legacy-Summe 43.250 → 40.454, 4 Kompatibilitätsregeln weg statt konvertiert, Policy-Epoche 25 → 26, Ausnahme für die 5 Ersatzregeln per ADR 0038; Schnitt S5 Stundenplan-Lesepfade, Betrieb und Bereinigung erledigt (#3551): Planer-Lesepfade, operativer Tag, Aufbewahrungs-Bereinigung, Abschluss beendeter Sitzungen und Planungsspuren beim Owner `modules/timetable`, Budget 20.326 → 16.159, `supervisiondashboard/legacy` 722 → 721, Legacy-Summe 40.454 → 36.286, 17 Kompatibilitätsregeln weg statt konvertiert, 3 erledigte Keys gelöscht, Policy-Epoche 26 → 27, Ausnahme für die 3 Ersatzregeln per ADR 0038; Schnitt S2 Vorlagen, Materialisierung und Rosterpflege erledigt (#3552): Vorlagen-Schreibpfade, Materialisierung mit Erkennung bearbeiteter Termine, Rosterpflege und Serien-Sperre beim Owner `modules/timetable`, Budget 16.159 → 8.925, Legacy-Summe 36.286 → 29.052, 13 Kompatibilitätsregeln weg statt konvertiert, 0 Keys, Policy-Epoche 27 → 28, Ausnahme für die 1 Ersatzregel per ADR 0038; Schnitt S3 Abweichungen, Vertretungen und Anwesenheitsspiegel erledigt (#3553): Abweichungs-, Vertretungs- und Krankmeldungs-Schreibpfade, Anwesenheitskorrektur und Anwesenheitsspiegel beim Owner `modules/timetable`, Spiegel als Timetable-Kommando in der Presence-Transaktion, Budget 8.925 → 5.710, Legacy-Summe 29.052 → 25.837, 5 Kompatibilitätsregeln weg statt konvertiert, 0 Keys, Policy-Epoche 28 → 29, Ausnahme für die 1 Ersatzregel per ADR 0038; Schnitt S1 Instanz-Lebenszyklus erledigt (#3554): Lebenszyklus, Personalwechsel, Serienumwandlung, Auto-Start und Auto-Ende beim Owner `modules/timetable`, Nest und `timetablesqltest` gelöscht samt Paketeinträgen, Budget-Eintrag gelöscht (5.818 nach #3608), Legacy-Summe 25.832 → 20.013, 78 Regeln weg (55 mit #3424, 16 der Testprovider, 7 verwaiste), 13 `api/timetable`-Keys gelöscht (584 → 571), Policy-Epoche 29 → 30, ADR-0038-Ausnahme endet), #2732 entblockt
- [x] [#3422](https://github.com/moto-nrw/project-phoenix/issues/3422) `modules/studentpresence/legacy` aufgelöst — 13.969 LOC, Budget-Eintrag gelöscht (Legacy-Summe 51.310 → 49.904), 79 `student-presence`-`adapter`/`domain`-Regeln weg statt konvertiert, 0 Keys (Ratchet unverändert), Komplexitäts-Ratchet −35 Einträge, Policy-Epoche 20 → 21; Ausnahme für die 57 Ersatzregeln per ADR 0036; offen: Passthrough-Budget `modules/studentpresence` 64 → 76
- [x] [#3413](https://github.com/moto-nrw/project-phoenix/issues/3413) `modules/workforce/legacy/timetracking` aufgelöst — 13.327 LOC, dazu der Arbeitssitzungs-Shim `legacy/worksession_repositories.go` (1.158 LOC) samt fünf Konstruktoren und Root-Bindungen gelöscht; Budget `modules/workforce/legacy` 15.666 → 1.181, Legacy-Summe 19.312 → 4.827; Zeiterfassungs-Dienste in `modules/workforce/internal/application/timetracking`, gebaut von `compose.NewTimeTracking`, die Adapter der öffentlichen Verträge aus `services` in `modules/workforce/compose`, Zeilen-Abbildung privat in der Komposition; `services.Factory` 8 → 1 Zeiterfassungsfeld (Composition 259 → 252), 0 Keys (Ratchet unverändert), Passthrough-Budget Workforce 47 → 62 (15 Dienst-Lesepfade, 70 Adapter in die Komposition, 2 tote Lesepfade gelöscht), 9 Regeln des Pakets weg oder umgeschrieben, Policy-Epoche 39 → 40, Ausnahme per ADR 0052; Zeilen-Vokabular `adapters/timerecords` mit #3996 aufgelöst
- [x] [#3996](https://github.com/moto-nrw/project-phoenix/issues/3996) `modules/workforce/adapters/timerecords` aufgelöst — Zeilen und Repository-Verträge der Zeiterfassung in `modules/workforce/internal/application/timetracking` (`*_record.go`, `time_record_repositories.go`), `DurationDays` als Funktion (Passthrough-Budget Workforce bleibt 62); Abwesenheits- und Audit-Adapter aus `modules/workforce/legacy` nach `modules/workforce/compose` (`absence_rows.go`), die beiden Dekoratoren in `services` benennen den Vertrag über Aliase der Komposition, die Presence-Suiten die Zeilen über vorübergehende `services`-Aliase (bis #4008/#4007); der Presence-Port `EnsureCheckedIn` liefert die öffentliche `workforce.WorkSession`; Budget `modules/workforce/legacy` 1.181 → 763, Legacy-Summe 4.826 → 4.408; Paketeintrag und 10 Regeln weg (die 9 unter #3996 und `workforce.adapter-test.time-records-calendar-dates`), 0 Keys (Ratchet unverändert), Composition 252 → 252, Policy-Epoche 40 unverändert
- [x] [#3418](https://github.com/moto-nrw/project-phoenix/issues/3418) `modules/workforce/legacy/shiftplanning` aufgelöst — 5.336 LOC (Legacy-Summe 49.904 → 44.517), 44 Kompatibilitätsregeln weg statt konvertiert, 5 verwaiste Regeln gelöscht (195 → 150), 0 Keys (Ratchet unverändert), Policy-Epoche 21 → 22; Dienstplan-Kommandos in `modules/workforce/internal/planning`, Tagesinformationen bei Timetable, Krankheits-Kaskade und Terminvertretung als Workflow `workflows/shiftplansync`; Ausnahme für die Ersatzregeln per ADR 0037; die `models/schedule`-Zeilen der Dienstplan-Reads fielen mit #3554 (Dienstplan-Zeilen jetzt Workforce-eigen, 7 Regeln weg)
- [x] [#3427](https://github.com/moto-nrw/project-phoenix/issues/3427) `modules/careplan/legacy` aufgelöst (`carelifecycle` und `careexitview`) — 5.278 LOC, 46 Kompatibilitätsregeln weg, 1 Key weniger (625 → 624); Ausnahme für Operator-Setting und verschobene Suiten per ADR 0035
- [x] [#3420](https://github.com/moto-nrw/project-phoenix/issues/3420) `workflows/parentportal/legacy` aufgelöst — 5.881 LOC, 46 Kompatibilitätsregeln weg, Schreibpfade als Owner-Commands (Care Plan, People Directory, Audit Platform); offen: Announcement-Quittungen in `messaging` (Communication)
- [x] [#3410](https://github.com/moto-nrw/project-phoenix/issues/3410) `modules/careplan/legacy` Wurzelpaket aufgelöst — 874 LOC, 15 Keys weniger (644 → 629)

## Schuld sichtbar machen · 4 Regeln, 0 Keys

- [x] [#3421](https://github.com/moto-nrw/project-phoenix/issues/3421) `inbound-parent.*` zu exaktem Debt konvertiert — 27 Regeln weg, 27 Keys unter #3421 (568 → 597; #3565 entfernt zwei; #3755 entfernt zwei: `services/users` in Produktion und internem Test, das Profil kommt über einen eigenen Port; #2734 entfernt drei: `api/enrollment` in Produktion und internem Test, `models/enrollment` im internen Test; #2743 entfernt einen: `modules/careplan/inbound/parent -> database/repositories` im externen Test; #3991 entfernt einen: `models/base` im internen Test, die Zeilen-Literale nennen `users.Model`); Stufe 2: die letzten 20 Keys weg, 81 → 61; Kalender über `sharedkernel/calendar`, die zurückgereichten People-Directory-, Care-Plan- und Enrollment-Werte über das Vokabular des Workflow-Ports, das Profil als HTTP-Werte über den eigenen Port (Übersetzung in der Root), die Locale-Prüfung über die Notification-Texte von Delivery, Transaktionen über `tenant.WithinTenant`/`WithinAdmin` ohne `*bun.DB`, die Handler-Suiten über `modules/settings` und `api/testutil` ohne viper; `api/common`, Session-Adapter und Tenant-Runtime als stehende Regeln per ADR 0051, Policy-Epoche 38 → 39, vier Regeln; HTTP-ORM-Ratchet −2 Einträge, Composition 259 → 259)
- [x] [#3416](https://github.com/moto-nrw/project-phoenix/issues/3416) `issue`-Feld an Policy-Regeln und `rules.stale` (das `issue`-Feld hat #4070 wieder abgeschafft, ADR 0057)
- [x] [#4070](https://github.com/moto-nrw/project-phoenix/issues/4070) Übergangsregeln in exakte Keys — 405 Regeln weg, 514 Keys unter #4072 bis #4078, #4081, #4004, #2729, #2750 und #4096 (6 → 520), Policy-Epoche 46 → 47; `models/*` als `legacy-model`, Session-Token-Runtime als `identity-access`/`http`, neun abgelaufene Cutover-Akzeptoren gelöscht, Temp-Rules-Ratchet strukturell
- [x] [#4096](https://github.com/moto-nrw/project-phoenix/issues/4096) Dev-Tools als eigenes Binary `cmd/devtools` (ADR 0060) — `cmd -> seed/api` und `cmd -> simulate` weg (492 → 490), Policy-Epoche 48 → 49; `cmd/devtools` ist das einzige Paket in `dev-tools`/`compose` mit acht `dev-tools.compose.*`-Regeln; HTTP-Client, Demo-Runtime samt Lease und Zufallsquelle aus `api` ins Binary; nur das Demo-Image enthält es; Composition 1.740 → 1.724
- [x] [#3423](https://github.com/moto-nrw/project-phoenix/issues/3423) Die acht Quality-Ratchets auf null — am 15.10.2026 aus #2580 gelöst, läuft als eigenes Ticket weiter

## api/students · 0 Keys

- [x] [#3351](https://github.com/moto-nrw/project-phoenix/issues/3351) `modules/careplan/legacy/careschedule` auflösen — 9.561 LOC (nativ in Care Plan, ADR 0030)
- [x] [#3352](https://github.com/moto-nrw/project-phoenix/issues/3352) Status-Tage und Präsenz-Reads → `modules/studentpresence` — die 16 Produktionsdateien hängen seit #3422 am öffentlichen Vertrag (`StatusDays`, `StatusDayOverviews`, `StudentHistory`); `api/students` bindet die Präsenz jetzt über den eigenen Port `StudentPresence` statt der ganzen `Presence`-Komposition, 0 Keys, keine Regel geändert
- [x] [#3353](https://github.com/moto-nrw/project-phoenix/issues/3353) Offering-Change- und Pickup-Entscheidungen → Owner-Commands
- [x] [#3354](https://github.com/moto-nrw/project-phoenix/issues/3354) Stammdaten- und Elternantrags-Reviews → Owner-Module — Entscheidung, Korrektur, Mitsorgeberechtigten-Hinweis und Ledger der Stammdaten-Anfragen nativ in `modules/careplan` (`masterdatarequests.Decisions`), Sammelfreigabe und Konfliktauflösung als Care-Plan-Koordinator über alle vier Queues (`parentrequests.Coordinator`, neues Contract-Paket ohne neue Regel), acht Dateien aus `services/users` gelöscht, 5 Keys weniger (468 → 463), 2 verwaiste Regeln gelöscht, Composition 607 → 604
- [x] [#3356](https://github.com/moto-nrw/project-phoenix/issues/3356) Settings-, Listenexport-, Messaging-, IoT-, Aktivitäts- und Schulstruktur-Kanten — `api/students` liest Settings, weckt Sorgeberechtigte, liest Gruppen und Anmeldungen und stößt den Regeltermin-Resync über eigene Ports an (`TenantSettings`, `GuardianWake`, `SchoolGroups`, `ActiveEnrollments`, `OfferingSourceResyncer`), die Root bindet die Owner; die vier Exporte füllen den neuen Document-Rendering-Vertrag `modules/documentrendering/lists` (drei Regeln am neuen Contract-Punkt); Audit-Vokabular und Sweep-Batchgröße (`careplan.StudentDocumentSweepBatchSize`) ohne `models/*`; 19 Keys weniger (463 → 444), Composition 604 → 604
- [x] [#2731](https://github.com/moto-nrw/project-phoenix/issues/2731) Carrier geschlossen — `api/students` Datei für Datei nach `modules/peopledirectory/inbound/students` (`people-directory`/`http`), Owner `inbound-students` gelöscht; Schülerzeilen über die People-Directory-Capability, Personen-Schreibpfade über den Port `PersonRecords` (`services.NewStudentRoutePersons`); alle 37 Keys plus `api -> api/students` (#2750) und `api/enrollment -> api/students` (#2734) weg, dazu drei `api/common`-Keys (#2738) mit dem umgezogenen Listen-Snapshot (444 → 402), Composition 604 → 603; die File-Storage-Ausnahme bleibt, jetzt unter #2706

## Identity · 0 Keys

- [x] [#3446](https://github.com/moto-nrw/project-phoenix/issues/3446) Fremde Implementierungs-Imports aus den Identity-Behavior-Suiten entfernen — 0 (`tenant` über die Tenant-Runtime-Helfer von `test` entfernt, 86 → 85; #4001: beide Eigen-Imports weg (ADR 0053, Punkt 7.1), 3 → 1; `auth/rotation` weg: der Recovery-Proof ist Eingabe von `AccountAuthentication.RefreshTokenWithAudit`, `modules/identityaccess/compose` bindet ihn für die Rotation-Policy, die Account-Refresh-Route reicht den Header durch, die Suiten prüfen Grace und Family-Fingerprint per Wert, 46 → 45; keine Regel, Composition 252 → 252)

- [x] [#3487](https://github.com/moto-nrw/project-phoenix/issues/3487) `modules/identityaccess/legacy/jwt` → `modules/identityaccess/sessiontoken` — Relocation (Policy-Epoche 41 → 42, ADR 0054), Paketname `sessiontoken`, Owner und Rollen unverändert; 0 Keys weniger (46 → 46): die drei verbleibenden Keys tragen den neuen Pfad und behalten ihre Carrier (`api` in Produktion und internem Test #2750, `services` #2747); die inaktive Deklaration `auth/jwt -> legacy/jwt` von #3226 entfernt; `modules/identityaccess/legacy` gelöscht, Legacy-Budget 4.408 → 3.050 LOC
- [x] [#3230](https://github.com/moto-nrw/project-phoenix/issues/3230) `api/auth` → `modules/identityaccess/inbound/auth` — Relocation (Epoche 19), 3 Keys weniger (684 → 681), 26 bleiben unter dem neuen Pfad bei #2736
- [x] [#3231](https://github.com/moto-nrw/project-phoenix/issues/3231) Identity-Handler aus `api/operator` → `modules/identityaccess/inbound/operator`, 1 Key weniger (681 → 680). Router, Fremd-Komposition und die Handler-Tests mit `jwt` oder `models/platform` bleiben in `api/operator`: nur `inbound-operator` darf die Owner-Routen mounten, PR-Modus lässt keine neue Erlaubnis zu. Paketlöschung bei #2736 nach #2725
- [x] [#2736](https://github.com/moto-nrw/project-phoenix/issues/2736) Carrier geschlossen — alle 37 Keys und alle Regeln mit #2736 weg (624 → 586): Account-Routen nach `modules/identityaccess/inbound/account` (`identity-access`/`http`), Settings Platform mit Public-Vertrag `modules/settings` und `modules/settings/compose`, `api/operator` nur noch Router; offen: Weitergabe des Recovery-Proof-Headers der Operator-Refresh-Route ohne Test
- [x] [#3501](https://github.com/moto-nrw/project-phoenix/issues/3501) `modules/identityaccess/legacy/usercontext` in den Caller-Context von Identity & Access aufgelöst — alle 55 Keys von #2725 weg (680 → 625), `/api/me` in `modules/identityaccess/inbound/me`; offen: Timetable-Read der geplanten Aufsichten, Personen-Read und -Schreiben über People Directory

## Storage

- [x] [#2759](https://github.com/moto-nrw/project-phoenix/issues/2759) Cutover `users.students` — DB-Hälfte durch PR #3384, offen ist der Caller-Switch
- [x] [#3432](https://github.com/moto-nrw/project-phoenix/issues/3432) Drei Projection-Grants von `users.students` umhängen — blockt #2760
- [x] [#2760](https://github.com/moto-nrw/project-phoenix/issues/2760) Contract `users.students` — blockiert durch #2759, #3387, #3432
- [x] [#2755](https://github.com/moto-nrw/project-phoenix/issues/2755) Backfill `users.students_guardians` — Migration 1.15.413 und `backfill guardian-owner`; Checkpoint und Verifikation (Counts, Checksummen, Account-Bindung, RLS) liegen für #2756 bereit, `users.students_guardians` bleibt autoritativ
- [x] [#2756](https://github.com/moto-nrw/project-phoenix/issues/2756) Cutover `users.students_guardians` — Migration 1.15.417 und Caller-Switch in einem Release: People Directory schreibt die Beziehung und führt die Arbeitseinheit, Care Plan die Abholberechtigung, Identity & Access den Portalzugang; gelesen wird über die Projektion `guardian-link-view`. Die alte Tabelle bleibt als trigger-gepflegter Rollback-Spiegel mit Zähler für #2757
- [x] [#2757](https://github.com/moto-nrw/project-phoenix/issues/2757) Contract `users.students_guardians` — Migration 1.15.429 entfernt Spiegeltabelle, Routing-, Spiegel- und Single-Primary-Trigger, alte Grant-Invalidierungsfunktionen und Zähler nach Integritätsprüfung unter Sperre (inkl. `incomplete_links` und `mirror_drift`); Fixtures und Tests schreiben die Owner-Tabellen
- [x] [#2753](https://github.com/moto-nrw/project-phoenix/issues/2753) Cutover `users.staff` — Migration 1.15.409 und Caller-Switch in einem Release: School Membership schreibt die Mitgliedschaft, Workforce das Beschäftigungsprofil (`StaffEmployments`); Kompatibilitäts-View, Archiv und Zähler bleiben für #2754
- [x] [#2754](https://github.com/moto-nrw/project-phoenix/issues/2754) Contract `users.staff` — Migration 1.15.427 entfernt View, Routing, Archiv `users.staff_legacy` und Zähler nach Integritätsprüfung unter Sperre; `models/users.Staff` benennt keine Tabelle mehr, Fixtures und Tests schreiben die Owner-Tabellen
- [x] [#2762](https://github.com/moto-nrw/project-phoenix/issues/2762) Cutover Activity Instances und Participants — Migration 1.15.415 und Caller-Switch in einem Release: Timetable plant Blöcke und Teilnehmer, Student Presence führt sie aus und erfasst die Anwesenheit (`active.activity_sessions`, `active.activity_session_attendance`); die alten Spalten bleiben als trigger-gepflegter Rollback-Spiegel mit Zähler für #2763
- [x] [#2763](https://github.com/moto-nrw/project-phoenix/issues/2763) Contract Activity Instances und Participants — Migration 1.15.432 entfernt die gespiegelten Ausführungs- und Anwesenheitsspalten, Spiegel- und Routing-Trigger und Zähler nach Integritätsprüfung unter Sperre (inkl. `execution mirror drift` und `attendance mirror drift`) und verengt den Planungsstatus auf planned/cancelled; Fixtures und Tests schreiben die Owner-Tabellen
- [x] [#2719](https://github.com/moto-nrw/project-phoenix/issues/2719) Contract Enrollment Request-Child

## Worker und Scheduler · 0 Keys

- [x] [#4067](https://github.com/moto-nrw/project-phoenix/issues/4067) Worker-Vertrag der Root als definierte Typen in einen `scheduler-runtime`/`contract`-Punkt (ADR 0056 Punkt 4) — 0 (entfernt `api -> services/scheduler`, 37 Symbole, ohne Alias und ohne Regel; der Key trägt weiter #2750, weil der Baseline-Vergleich keinen Issue-Wechsel zulässt, #4067 blockt #2750). Entscheidung vom 16.10.2026: Der Vertrag darf keine Typen anderer Owner nennen (ADR 0057), also erst die Ports (ADR 0058 Punkt 2), dann der Umzug; #4067 trägt dafür alle Keys mit Quelle `services/scheduler` (#4072, #4075). Schritt 1: Der Erzeuger der persönlichen Erinnerungs-Benachrichtigungen liegt in `workflows/reminderdelivery` (`PersonalReminderNotifications`), der Worker hält nur den Tick-Port `ReminderNotificationTick`; die Root bindet Zustellung, Zustimmung, Konten und Dienstanwesenheit (`workforce.OnDutyStaffIDs`). Fünf Keys weg (519 → 514): `services/scheduler -> delivery/application/notifications` und `-> workforce` in Produktion und internem Test, `-> workflows/reminderdelivery` im internen Test. Schritt 2: eigene Ports des Workers für Sitzungsende und verlassene Sitzungen, die nächtliche Anwesenheitsbereinigung, das Archiv der Status-Flags, die PWA-Nutzungsbereinigung und die Überfällig-Ereignisse; die Root bindet sie (`api/scheduler_presence_ports.go`), Care Plans Status und Quelle bindet die Root, die Ereignisse baut `realtime`. Sieben Keys weg (512 → 505): `-> studentpresence`, `-> delivery/application/pwa` und `-> delivery/application/realtimeevents` in Produktion und internem Test, `-> careplan/absencerecords` in Produktion; damit trägt `services/scheduler` keinen #4075-Key mehr. Schritt 3: eigene Ports für Materialisierung, Zeitplan-Bereinigung, automatischen Start und automatisches Ende (Timetable) und die geplanten Erinnerungen (Reminder Delivery); die Root bindet sie (`api/scheduler_timetable_ports.go`) und meldet die Läufe des Workers als Scheduler-Materialisierung, den Status „geplant“ bindet die Root (`DayInstance.Planned`). Drei Keys weg (498 → 495): `-> timetable` in Produktion und internem Test, `-> workflows/reminderdelivery` in Produktion; `services/scheduler` hat keinen Key mehr, #4072 ist leer. Offen: der Vertrag Schritt 4: der Vertrag liegt als definierte Typen in `modules/schedulerruntime/ports` (`scheduler-runtime`/`port`, Nachtrag zu ADR 0056 vom 16.10.2026), dazu `Worker`; die Root baut den Worker über `compose.NewWorker`, der Lauf der Löschfristen entsteht in `NewWorker`, den Rollover-Läufer bindet die Root; Policy-Epoche 50 → 51 mit `root-composition.reach.platform.port`, `owner-compose.platform.port` und `own-test.platform.port` (stehende Reichweiten aus ADR 0053); `api -> services/scheduler` entfernt, 316 → 315; Compose 85 → 83 LOC
- [x] [#2726](https://github.com/moto-nrw/project-phoenix/issues/2726) Worker hinter erneuerbarem DB-Lease — Migration 1.15.437 (`platform.worker_leases`, `platform.assert_worker_lease`), Store in `modules/schedulerruntime/workerlease`, jeder Job-Lauf und jeder Commit hängt am Lease-Term; eigenständiger Prozess `worker`, `serve --embedded-worker=false`; 0 Keys (Ratchet 378 → 378), Composition 603 → 603; Runbook `docs/operations/worker-lease-2726.md`
- [x] [#2746](https://github.com/moto-nrw/project-phoenix/issues/2746) services/scheduler — 0 (alle 27 Keys weg, 377 → 350): Tenant-Runtime über den Scheduler-Port `TenantRuntime`, gebunden durch das neue Paket `modules/schedulerruntime/jobruntime` (scheduler-runtime/postgres, ohne neue Regel); Settings-Keys als lokale Konstanten, die Root prüft sie beim Start gegen die Registry (`settingsCompose.VerifyPreloadKeys`) und bindet den Minuten-Snapshot über `settingsCompose.NewTenantSnapshots`; Overdue-Tick über `realtimeevents` und eigene Ports, Booking-Audit, Schülerstatus und Change-Log über Ports, die `api/scheduler_ports.go` an die Legacy-Typen bindet; die Suiten laufen über Test-Support (`testpkg.PassthroughTenantRuntime`, `ScriptedTenantRuntime`) statt über Repositories, die SQL-Seiten der Overdue- und Session-End-Pfade pinnen zwei neue Tests in `database/repositories`; das Budget-Szenario `services.scheduler.minute_snapshot.setting_values` prüft jetzt `services.TestSchedulerMinuteSnapshotReadsEverySchoolOnce` über die echte Bindung; Composition 603 → 597

## Laufzeit

- [x] [#3415](https://github.com/moto-nrw/project-phoenix/issues/3415) `validate-ticket` in CI, Evidenzvertrag durchsetzbar machen — blockt #3021
- [x] [#3411](https://github.com/moto-nrw/project-phoenix/issues/3411) Checkpoint-Workload ausweiten — blockt #3021
- [x] [#3412](https://github.com/moto-nrw/project-phoenix/issues/3412) Rückzugs-Warteschlange: Paginierung und Query-Budget
- [x] [#3419](https://github.com/moto-nrw/project-phoenix/issues/3419) IoT-Fehlertexte im Golden, `apiErrors.ts`-Pfad korrigieren
- [ ] [#3021](https://github.com/moto-nrw/project-phoenix/issues/3021) Runtime-Checkpoint 3

## Spur A · 0 Keys

- [x] [#2706](https://github.com/moto-nrw/project-phoenix/issues/2706) Dokument-Rendering — Plan-Export über `modules/planexport/compose` an Lesepfade im öffentlichen Vokabular von Workforce, Timetable und Facilities gebunden (Block-, Personal-, Angebots- und Kinderzahl-Lesepfade laufen in der Root weiter über die Retained-Repositories, Abbau mit #2743), `modules/planexport/legacy` gelöscht; File Storage bedient `documents.file_cleanup` mit eigenem Adapter und übernimmt die Tabelle per ADR 0045 (Policy-Epoche 31 → 32), generisches Dokument-Repository, Modell und Binder gelöscht; Upload-Koordinator mit eigenem Objektspeicher-Port; alle 17 Keys weg (282 → 265), Composition 596 → 596. Offen bleibt die benannte Ausnahme `people-directory.http.file-storage-adapter` (Kinderdokumente brauchen eine öffentliche File-Storage-Capability)

## Schnitt von #2747 (ADR 0053) · 0 Keys

Die Keys bleiben bei ihren Carriern. Jede Produktionsdatei von `services` gehört genau einem Ticket von #4006 bis #4013; `factory.go` und `tenant_runtime.go` bleiben für #2747.

- [x] [#4005](https://github.com/moto-nrw/project-phoenix/issues/4005) Test-Hooks verlassen `services` (104 Dateien, 8.053 LOC) — 0 (Policy-Epoche 43 → 44: die Hooks liegen in `api/*_test_helpers.go` und nehmen `transactionruntime.Database`; Tests des Owners, zu dem ein Hook oder Alias nur durchreichte, rufen bzw. benennen dessen Komposition selbst, ungenutzte Hooks und Aliase sind gelöscht; den Session-Codec baut `modules/identityaccess/compose` aus Secret und Laufzeiten (`NewConfiguredSessionTokens`); `school_membership.go` ist eine gewöhnliche Produktionsdatei, `MembershipTestRepositories` liegt bei seinem Builder in `membership_repositories.go`; interne Tests der Pakete, die `api` importiert, erreichen die Root über eine Paketvariable, die ein externes `root_composition_test.go` bindet, oder über `modules/workforce/workforcetest`; zehn Regeln unter den stehenden Reichweiten von ADR 0053, 13 verwaiste gelöscht; `api/testutil` ohne `services`, `api/testutil -> services` entfernt (#2748), 41 → 40, Composition 252 → 252)
- [x] [#4006](https://github.com/moto-nrw/project-phoenix/issues/4006) Identity & Access — 26 Dateien, 4.656 LOC — 0 (`services -> sessiontoken` entfernt (#2747), 40 → 39; die Root komponiert Identity & Access (`api/identity_root.go`): `NewFactoryWithModules` nimmt statt des Token-Adapters `services.IdentityRoot`, ruft dessen `Compose` mit den Werten, die die Factory selbst baut (Settings, Audit, Mail, Identitätskette, Outbox und Zustellstatus zur Aufrufzeit), und verbraucht Modul, Caller-Kontext, die Bindungen der Owner ohne Vertragszugriff und die drei Identity-Mail-Renderer; alle Fremd-Owner-Seams liegen in `api/identity_*.go` mit schmalen, eigenen Zeilen-Ports (`api/identity_rows.go`, Regel 1); die Portal-Runtimes binden direkt an die Portal-Typen, `modules/schoolportal/portaltest` samt Paketeintrag und zwei Regeln gelöscht, die stehende Regel `own-test.inbound.test-support` war damit verwaist; die Entscheidung über die Reihenfolge der Eltern-Willkommensmail liegt in Identity (`domain.GuardianWelcomeOrder`, Port `compose.PrecedingMails`), die Root meldet nur den Delivery-Zustand; Bereinigung und Seeder über `compose.NewSessionMaintenance` (ohne Signer, der Sweep signiert nie), `NewInvitationMaintenance`, `NewGuardianSeedAccess`, `NewDemoAccessExpiry`, `AccountExists`; Factory-Felder `Auth`, `MFA`, `Passkey`, `Invitation`, `GuardianInvitation`, `UserContext` und die vier Operator-Felder sowie sieben Identity-Methoden gelöscht, Composition 250 → 240; `services` 260 → 232 Produktionsdateien, 35.473 → 30.454 LOC; mit ihrem einzigen Aufrufer verlassen `operator_provisioning_links_composition.go` und `sse_presence.go` `services`, Verzeichnis-, Seam- und Port-Deklarationen, deren Aufrufer bleiben, liegen in `person_projection_auth.go`, `owner_roots.go` und `operator_provisioning_composition.go`; die Identity-Verhaltenssuiten importieren `services` nicht mehr; `api -> sessiontoken` (#2750) bleibt, weil Middleware und Claims-Lesepfade der Root den Adapter weiter brauchen)
- [x] [#4007](https://github.com/moto-nrw/project-phoenix/issues/4007) People Directory, School Membership, School Structure — 0 (keine der 44 Dateien liegt mehr in `services`, `student_http_test_withdrawals.go` gab es auf dem Messstand nicht mehr; Einzelbesitz in die Owner-Kompositionen: Guardian-Provider, Staff-Verzeichnis, Raum-Snapshot- und Hinweisnamen-Lesepfade und die Rollen-Presets der Beziehung (`ApplyDefaultGuardianRole`, Punkt 5) in `modules/peopledirectory/compose`, `StaffByID` in `modules/schoolmembership/compose`, `LegacyGroupRepository`, `GroupNames` und `EnrichGroupNames` in `modules/schoolstructure/compose`, `FindSchool` und `SchoolsByID` in `modules/organizationtenancy/compose`, der späte Absage-Publisher in `modules/communication/composition`; `CanSeeAll` entscheidet `workforce.CanSeeAllSubstitutions` beim Owner von `education.group_substitution` hinter dem Port `SubstitutionOverviewScope` (Punkt 4); `StaffGroupPair` und `SchoolContact` re-exportiert `modules/delivery/compose` (Punkt 6); den Mehr-Owner-Kleber baut die Root: `api.NewPeopleWiring` liefert Retained-Repositories und Adapter als `services.PeopleWiring` an `NewFactoryWithModules`, Adapter über Werte, die der Graph selbst baut (Audit-Trails, Identity-Modul, Session-Records, Personen-Service), als Konstruktoren; `NewStaffMembershipRuntime` und `NewGuardianDirectoryRuntime` sind Root-Funktionen (Punkt 3), `GuardianProfileLoader` und `Birthdays` baut die Root, wo sie sie mountet; die Root-Adapter nehmen eigene Ports statt Retained-Repository-Typen (`TestHandlerLayerRatchet` R1), `MembershipTestRepositories` liegt bei den Test-Hooks; was andere Slices noch aufrufen, liegt top-down bei seinen Aufrufern: Vertretungsverträge in `staff_projection_education.go` und Operator-Kontoverzeichnis in `operator_account_directory.go` (nach #4006 und #4008 noch in `services`, Aufrufer ist die Factory), Schedule-Helfer und `StudentRecord` in den Care-Plan-Dateien (#4011), `NewPersonRepository` in `owner_roots.go` (#4013), `NewSchoolMembership` bleibt für `timetable.go` und `command_roots.go`; zwei verwaiste Regeln `legacy-composition.compose.offboarding.staff-offboarding.*` gelöscht; gemessen nach dem Merge von #4006 und #4008: Keys 39 → 39, Composition 239 → 237, `services` 197 → 156 Produktionsdateien und 27.922 → 21.330 LOC)
- [x] [#4008](https://github.com/moto-nrw/project-phoenix/issues/4008) Workforce — 34 Dateien, 2.721 LOC — 0 (die eigene Verdrahtung in `modules/workforce/compose`, der Klebstoff zwischen Ownern in `api`; die Factory nimmt `services.WorkforceWiring` als einen neuen Konstruktorparameter mit gebauten Werten und zwei Bindungen über die Zeilen, die nur sie baut; die Root baut die Stempeluhr am IoT-Mount, das Factory-Feld `StaffClock` entfällt, Composition 250 → 249; die Zeiterfassungs-Ports nehmen `transactionruntime.Database`; Helfer, die nur noch ein `services`-Builder ruft, liegen bei diesem und gehen mit #4006, #4007 oder #4010; `rfid_card_lookup.go` (#4010) ging mit der Stempeluhr; `legacy-composition.to.staff-clock-compose` gelöscht, Regeln 1.437 → 1.436; `services` 260 → 225 Produktionsdateien, 35.482 → 32.949 LOC; Keys 40 → 40)
- [x] [#4009](https://github.com/moto-nrw/project-phoenix/issues/4009) Timetable, School Calendar, Erinnerungen — 29 Dateien, 3.577 LOC — 0 (keine der 29 Dateien liegt mehr in `services`; Einzelbesitz in die Owner-Kompositionen: Nicht-Arbeitstage, Feiertage, Schließtage und Perioden-IDs in `modules/schoolcalendar/compose`, der iCalendar-Renderer des Portals in `modules/schoolcalendar/portal/compose`, `Planning`, Auto-Start/-Ende und Vertretungskonflikte über die Owner-Zeilen in `modules/timetable/compose`; den Klebstoff zwischen Ownern baut die Root (`api/timetable_*.go`, `api/calendar_*.go`, `api/reminder_*.go`, Geburtstage, Periodenverwaltung, Planungskalender, Planexport-Kalender); die Factory nimmt `services.TimetableWiring` als einen neuen Konstruktorparameter mit gebauten Werten (Timetable-Repositories über den Transaction-Runtime-Wert, Gruppennamen, Planungsspuren, Klassentag- und Verlaufs-Reads, Kalender-Erziehungsberechtigte) und Konstruktoren über das, was nur sie baut; deren Eingabetypen liegen bei der Factory in `factory.go`; die Root baut die Planungsspuren dort, wo sie sie mountet, das Factory-Feld `PlanningTracks` entfällt, Composition 237 → 236; die Root baut auch das Recurrence-Gate der Factory; die Factory-Methode `SchoolCalendarAdministration` ist `api.GuardedCalendarAdministration` über Settings, Gate und Angebotsprüfung statt über die Factory; Delivery nennt die Mail- und Benachrichtigungsnamen des Kalenderportals in `modules/delivery/compose` (Punkt 2); die Root-Adapter nehmen schmale Ports statt Repository-Typen (`TestHandlerLayerRatchet` R1); was andere Slices noch aufrufen, liegt top-down bei seinen Aufrufern: der unbeobachtete Timetable der Presence-Regeln in `student_presence_roots.go` (#4010, bis dahin doppelt neben `api.NewUnobservedTimetable`), die Cleanup-CLI reicht `api.NewTimetableCleanup` an `services.NewTimetableCleanupService` (#4013); die verwaisten Regeln `legacy-composition.reminder-compose` und `legacy-composition.reminder-ports` gelöscht, Regeln 1.431 → 1.429; gemessen auf `030716737b`: `services` 156 → 127 Produktionsdateien, 21.330 → 18.141 LOC; Keys 39 → 39)
- [x] [#4010](https://github.com/moto-nrw/project-phoenix/issues/4010) Student Presence, Facilities, Device Fleet, Klassentag — 39 Dateien, 2.227 LOC — 0 (keine der 39 Dateien liegt mehr in `services`, `rfid_card_lookup.go` und `sse_presence.go` gingen schon mit #4008 und #4006; Einzelbesitz in die Owner-Kompositionen: `NewRoomRepository`, `YardRoomColor` und `OpenGroupCatalog` in `modules/facilities/compose`, `PresenceSettings` in `modules/settings/compose`, `FailureLogger` und `DebugLogger` in `modules/studentpresence/compose`, `QueueInstanceStarted` und `QueueArrivalScheduleChanged` in `modules/delivery/compose`; sechs reine Weiterleitungen gelöscht, ihre Aufrufer rufen Timetable, Process Device Scan, Facilities, Student Presence, die Kalenderansicht und Security Runtime selbst; die Löschentscheidung für Räume (Punkt 4) liegt in Facilities hinter dem Port `RoomUsage` (`HasActiveSessions`, `RequiredByCareOffering`), die Root bindet ihn an Student Presence und Care Plan (`api.RoomUsage`), `ValidateDeletion` und das Fehlercode-Mapping der Root entfallen; den Mehr-Owner-Kleber baut die Root: `api.NewPresenceWiring` liefert Student Presence, Session-Records, Anwesenheits-Lesepfade, Raumdienst, Statustage und Klassenlisten-Leser als `services.PresenceWiring` an `NewFactoryWithModules`, dazu fünf Konstruktoren über Werte, die nur die Factory baut (Audit-Command, Settings, Personen-Service, Klassentag-Quellen, Zugriffslog); die Root-Wurzeln von Student Presence nehmen `transactionruntime.Database`; Factory-Felder `Facilities` und `StudentStatusDays` gelöscht, `api.TestFactory` behält beide für die Suiten (`unregistered_tag_scans.go` ging schon mit #4049), Composition 236 → 234; die Suiten von Student Presence, Care Plan und Workforce erreichen fremde Kompositionen über `api/presence_suite_test_helpers.go`; private Kopien bleiben bei ihren Aufrufern in `timetable.go` (#4009), `command_roots.go` und `cleanup_composition.go` (#4013), `pickup_excusal.go` (#4011) und `enrollment_report_composition.go` (#4012), die Root dupliziert das Care-Plan-Ausnahmeverzeichnis bis #4011; gemessen nach dem Merge von #4049: Keys 39 → 39, keine Regel, `services` 155 → 119 Produktionsdateien und 21.238 → 19.506 LOC)
- [x] [#4011](https://github.com/moto-nrw/project-phoenix/issues/4011) Care Plan — 45 Dateien, 5.988 LOC — 0 (keine der 45 Dateien liegt mehr in `services`; den Kleber zwischen Ownern baut die Root (`api/care_plan_*.go`, `api/parent_request_review_policy.go`): `api.newCarePlanWiring` liefert `services.CarePlanWiring` als einen neuen Konstruktorparameter an `NewFactoryWithModules`, gebaute Adapter über die Owner der Root und Bindungen über die Werte, die nur der Graph baut (Settings, Audit-Trails, Realtime-Hub, Eltern-Emitter, Caller-Kontext, Care-Plan-Dienste), der Parameter `observeCarePlan` entfällt; die Root-Adapter nehmen eigene Ports statt Retained-Repository-Typen (`TestHandlerLayerRatchet` R1); `modules/careplan/compose` bekommt `NewCareOfferings`; Entscheidungen (Punkt 4): der Auto-Entschuldigungs-Resync läuft in Care Plans eigener Arbeitseinheit (`ResyncFutureExceptions` über eine Schülerliste), er schreibt Care Plans Abholausnahmen und Fremdtabellen nur über Care Plans bestehende Ports `PartialAbsenceBlocks` und `PickupExtensions`, bleibt also beim Owner; die Sperrreihenfolge der wöchentlichen Abholfreigabe ist `LockWeeklyPickupStudents` hinter dem Port `PickupStudentLocks`, die Root bindet ihn an People Directory, `ErrStudentNotFound` wird übersprungen; Factory-Felder `Students` und `RequestReviewPolicy` gelöscht, Begleitgraph und Review-Policy baut die Root, wo sie sie mountet, Composition 235 → 233, damit unerreichbar und gelöscht: `peopleCompose.NewStudentService`; was verbleibender `services`-Code ruft, liegt bei seinen Aufrufern: `EnrollmentBookingProjection` in `enrollment_bookings.go`, Buchungs- und Settings-Adapter, Ledger und Messenger in `offering_change_composition.go` (#4012), Elternportal-Repositories in `owner_roots.go` (#4013), `pickupExceptionDirectory` in `command_roots.go` (#4013), `CareRequestStaff` in `factory.go`; `AbsenceNotifier` re-exportiert `modules/delivery/compose`; die verwaiste Regel `enrollment.http-integration-test.legacy-composition` gelöscht, Regeln 1.429 → 1.428; Keys 39 → 39; `services` 126 → 82 Produktionsdateien, 18.049 → 12.603 LOC; gemessen nach dem Merge von #4049 und #4009; nach dem Merge von #4010: Composition 231, verwaiste Regel `parent-portal.decision-test.legacy-composition.compose` gelöscht, Regeln 1.427 → 1.426, das manuelle Teilabwesenheits-Repository entfällt)
- [x] [#4012](https://github.com/moto-nrw/project-phoenix/issues/4012) Enrollment — 19 Dateien, 3.131 LOC — 0 (keine der 19 Dateien liegt mehr in `services`; Enrollment-eigene Regeln in `modules/enrollment`: `BookingProjection` (Auswahlen, Kapazitätsspitzen, Jahrgangszahlen, genehmigte Buchungen, Angebotsverweise des Buchungsaudits) hinter dem Verbraucher-Port `OfferingBookings` (Punkt 4), die Komposition baut nur (`NewDeletionModule`); der Mehr-Owner-Kleber in `api` mit schmalen Zeilen-Ports (`api/enrollment_rows.go`), der Buchungskonsistenz-Audit neben seinem Worker-Job; die Factory nimmt `services.EnrollmentWiring` als einen neuen Konstruktorparameter mit gebauten Werten und Konstruktoren über die Werte, die nur sie baut; die drei Settings-Guards aus `newFactory` sind Kleber und binden in der Root (`bindEnrollmentSettingsGuards`); Formularschemata, Captcha und Ablaufwarnungen baut die Root am Mount, Factory-Felder `EnrollmentFormSchema`, `EnrollmentCaptcha` und `EnrollmentPhaseExpiry` gelöscht, Composition 237 → 234; nach dem Merge von #4011 liegen auch die Care-Exit-Schreibpfade um die Root-Projektion und der Settings-Adapter des Angebotswechsels bei der Abholanpassung in `api`; `services` 156 → 137 Produktionsdateien, 21.330 → 18.532 LOC; Keys 39 → 39)
- [x] [#4013](https://github.com/moto-nrw/project-phoenix/issues/4013) Plattform, Kommunikation, Tenancy, Kommandos — 21 Dateien, 2.463 LOC — 0 (keine der Dateien liegt mehr in `services`, `operator_provisioning_links_composition.go` ging schon mit #4006; Einzelbesitz in die Owner-Kompositionen: Mail- und Push-Provider, Push-Berechtigung, Bereinigung abgelaufener Abos und dauerhafter E-Mail-Speicher in `modules/delivery/compose` (`MailPushProvider`, `NewDurableEmails`), die PWA-Nutzungszeilen re-exportiert Delivery (Punkt 6, der `PushSubscriptionRepository`-Typ wird nicht mehr benannt); Audit liest die umgebende Transaktion selbst (`NewTenantPersistence`, `NewHandoffPersistence`, `NewObservedCommand`, Punkt 2), den Typ-Switch über `bun` hält der Postgres-Adapter (`NewHandoffRuntime`); den Kleber baut die Root: `api.NewPlatformWiring` liefert Schul-Settings-Speicher, Operator-Ledger, Elternportal-Speicher, unbeobachtete Device Fleet und Listen-Renderer als Werte und die Audit-, Delivery- und Provisioning-Adapter als Konstruktoren als `services.PlatformWiring` an `NewFactoryWithModules`; Data Import und Datenbankstatistik baut die Root am Mount, Schulen und Ankündigungen liest sie aus ihren Ownern, Factory-Felder `Schools`, `Announcement`, `ListExport`, `Database`, `DatabaseStatsCapabilities`, `Import`, `StaffImport`, `ClassListImport`, `OpeningBalanceImport`, der Parameter `observeDataImport` und drei Factory-Methoden gelöscht, Composition 237 → 228; die Bereinigung komponiert ihre Läufe in `cmd/cleanup_roots.go`, was ihre Suiten teilen, liegt in `api/cleanup_roots.go`; `OutboxEnqueuer` und die Retained-Fehlerhelfer bleiben bei ihren Aufrufern; drei verwaiste Regeln gelöscht, Regeln 1.431 → 1.428; `services` 156 → 136 Produktionsdateien, 21.330 → 19.108 LOC; Keys 39 → 39; Settings, Realtime-Hub, Audit-Command, Delivery, File Storage, Operator-Provisioning und Messaging bleiben Factory-Felder, weil die Factory sie über eigene Werte baut oder die Root-Builder der anderen Slices sie über `*services.Factory` lesen)
- [x] [#4003](https://github.com/moto-nrw/project-phoenix/issues/4003) Root ohne fremde `adapter`/`postgres`-Rollen (Workforce, Data Import, Scheduler Runtime) — 0 (alle vier Keys weg, 46 → 42: `modules/workforce/compose/httpadapter` baut `/absence-types` und `/substitutions` (`AbsenceTypesRoutes`, `SubstitutionsRoutes`, nicht in `compose`, weil die internen Tests von `workforce/inbound` über `services` sonst einen Importzyklus hätten); `modules/dataimport/compose` liefert den Datei-Decoder (`NewFileDecoder`), Policy-Epoche 42 → 43 für `owner-compose.workflow.adapter` (stehende Reichweite, ADR 0053 Punkt 3, erste Nutzung); das neue Paket `modules/schedulerruntime/compose` (`scheduler-runtime`/`compose`) bindet Job-Runtime und Lease-Store an die Scheduler-Ports (`NewTenantRuntime`, `NewWorkerLeaseStore`) und braucht dafür `scheduler-runtime.compose.tenant-runtime` wie die 34 übrigen Owner-Kompositionen, die einzige neue Fremd-Reichweite (#4004: 206 → 207); Regeln 1.438 → 1.440, Composition 252 → 252, Zielgraph-Zyklen unverändert)

## Endkette · 7 Keys

- [ ] [#2750](https://github.com/moto-nrw/project-phoenix/issues/2750) root api, cmd, main composition — 2 (#2731: `api -> api/students` entfernt; #2742: `api -> api/admin` und `api -> api/groups` entfernt; #2734: `api -> api/enrollment` entfernt; #2745: `api -> api/absence-types`, `api/shift-types`, `api/staff-shifts`, `api/substitutions` und `api/work-time-models` entfernt; #2743: `api -> database/repositories` und `cmd -> database/repositories` in Produktion und internem Test sowie die vier Composition-Referenzen von `api` und `cmd` auf `repositories.Factory` und `NewFactory` entfernt; #3970: Tenant-Runtime, Demo-Runtime und Pools über `modules/transactionruntime/compose`; #4002: die Roots halten nur `transactionruntime.Database`, `api -> bun`, `cmd -> bun`, beide `database.direct-access` und `cmd -> database` entfernt) ADR 0053: [#4001](https://github.com/moto-nrw/project-phoenix/issues/4001) hat neun Keys entfernt (stehende Root-Reichweite: `. -> cmd`, `cmd -> api`, `cmd -> tenant`, `cmd -> database/migrations` in Produktion und internem Test, `api -> observability` und im internen Test `api -> auth/authorize/permissions`, `api -> realtime`, `api -> tenant`), 42 → 33; #2747 entfernt die drei `services`-Keys. #3487: `api -> legacy/jwt` in Produktion und internem Test heißt jetzt `api -> sessiontoken`, beide Keys bleiben. #4005: die Test-Hooks mehrerer Owner liegen als `api/*_test_helpers.go` unter dem bestehenden Key `api -> services`; die Composition-Referenz `api -> services.Factory` hat fünf Fundstellen mehr (Factory-Hooks), die Surface bleibt bei 252. #2747 erledigt: `api -> services`, `cmd -> services` und die Composition-Referenz `api -> services.Factory` sind weg, 33 → 30; die Root komponiert die Dienste selbst (`api/service_graph*.go`). Teilschnitt ohne Policy-Änderung: neun Keys weg, 34 → 25. `cmd` nimmt Datumswerte aus `sharedkernel/calendar` (`internal/timezone` in Produktion und internem Test), die Setting-Keys aus `modules/settings` (`api` im internen Test, `cmd`; der öffentliche Vertrag nennt sieben Keys mehr) und die gespeicherten Overrides über `settingsCompose.NewStoredOverrideReader` statt `NewSettingsCommandRepository` (`cmd -> models/config`); den Prozess-Logger baut `observability/compose.InstallProcessLogger` (`cmd -> applog`); Query-Log und Pool-Statistik bietet der Datenbank-Wert (`transactionruntime.Database.LogQueries`, `Capacity`, `api -> database`), die Schema-Prüfung beim Start `peopleCompose.VerifyRetainedStudentSchema`, jetzt außerhalb der Test-Hooks (`api -> database/repositories/users`); den Rate-Limit-Schlüssel aus den verifizierten Claims leitet der Session-Adapter ab (`sessiontoken.TokenAuth.RateLimitKey`, `api -> jwx/jwt` und `jwx/transform`), seine zwölf Tests ziehen mit; Composition 139 → 139. Die übrigen 21 Keys tragen weiter #2750: 16 brauchen eine Entscheidung ([#4063](https://github.com/moto-nrw/project-phoenix/issues/4063): `http-router` und `cli-config`, `api/common`, `dev-tools`, `middleware`, `sessiontoken`, `services/scheduler`, `analytics`), fünf einen Umzug in ihre Owner ([#4064](https://github.com/moto-nrw/project-phoenix/issues/4064): `api/config`, `api/operator`, `api/platform`, `api/pwa`, `api/reminders`). #4064, Policy-Epoche 44 → 45: `api -> api/config`, `api/platform`, `api/pwa` und `api/reminders` entfernt, 21 → 17; die Pakete liegen in ihren Ownern, `root-composition.reach.platform.http` ergänzt die Root-Reichweite aus ADR 0053 Punkt 2. #4063 (ADR 0056, Policy-Epoche 45 → 46): 15 Keys weg, 21 → 6. Offen: `api -> services/scheduler`, das mit [#4067](https://github.com/moto-nrw/project-phoenix/issues/4067) fällt, und `api -> api/operator`, das mit #4064 umzieht. #4070 (ADR 0057, Policy-Epoche 46 → 47): die zwei Regeln mit `issue` #2750 (`root-composition.to.import-http`, `root-composition.to.inbound-parent-http`) sind gelöscht, ihre Importe `api -> modules/dataimport/inbound` und `api -> modules/careplan/inbound/parent` sind Keys, 2 → 4; `cmd -> seed/api` und `cmd -> simulate` (vorher `root-composition.cli.dev-tools`) tragen [#4096](https://github.com/moto-nrw/project-phoenix/issues/4096). #4064: `api -> api/operator` entfernt, 4 → 3; der Operator-Router liegt im HTTP-Host (`api/operator_routes.go`). #4067: `api -> services/scheduler` entfernt, der Worker-Vertrag liegt in `modules/schedulerruntime/ports`.
- [x] [#4064](https://github.com/moto-nrw/project-phoenix/issues/4064) `api/config`, `api/operator`, `api/platform`, `api/pwa`, `api/reminders` in ihre Owner — 0 (die fünf Keys tragen #2750; vier umgezogen, Policy-Epoche 44 → 45: `api/config` → `modules/settings/http` (`settings-platform`), `api/platform` → `modules/communication/http/platformannouncements` (`communication`), `api/pwa` → `modules/delivery/http/pwa` (`delivery-platform`, Owner des Nutzungsdienstes), `api/reminders` → `workflows/reminderdelivery/http` (`reminder-delivery`, Owner der Erinnerungsabfrage); die vier `inbound-*`-Owner sind gelöscht, Root-Reichweite `root-composition.reach.platform.http` aus ADR 0053 Punkt 2; `api/operator` ist Kleber über Owner und liegt als `api/operator_routes.go` im HTTP-Host (ADR 0056), ohne Struct mehrerer Owner (ADR 0059): je Owner ein eigener Wert, Owner `inbound-operator` und seine acht Regeln gelöscht, die Root-Reichweite deckt alle Importe; `api -> api/operator` entfernt, 520 → 519, Composition 1.746 → 1.743)
- [ ] [#4068](https://github.com/moto-nrw/project-phoenix/issues/4068) `reportPWAUsage` und `serveStaffAvatar` aus der Root in ihre Owner (ADR 0056 Punkt 1) — 0 (kein Key)
- [x] [#2748](https://github.com/moto-nrw/project-phoenix/issues/2748) shared test und E2E composition — 0 (#3753: `services/users/userstest` gelöscht, drei Keys entfernt; #2742: `test -> database/repositories/education` im internen Test entfernt; #3556: `test -> models/education` entfernt, die Fixtures bilden ihre Zeilen selbst ab; Schnitt 1: `internal/testdb` ohne `crypto`, E2E ohne `jwt`, `tenant`, `auth/device`, `models/users`, `models/audit`; Epoche 24 gewährt dem Fixture-Owner per ADR 0039 eigenes Werkzeug, `tenant-runtime/public`, `legacy-shared/domain` und `security-runtime/contract`; für die übrigen Keys keine Regel; die meisten fallen nicht mit einem Carrier, weil ihre Zielpakete Implementierungsrollen bleiben, sondern über Owner-Test-Support oder Owner-Verträge (#3944); Schnitt 3: die Fixtures bilden Geräte-, Raum-, Aktivitäts- und Stundenplanzeilen selbst ab, `api/testutil` ohne `models/iot` und `models/users`, E2E ohne `jwt`, `models/activities` und `models/schedule`, zwölf Keys weg, 191 → 179; offen bleiben Vokabular, das andere Suiten über `test` beziehen (`models/users`, `models/enrollment`, `models/platform`, `models/audit`, `auth/authorize`), Plattform-Implementierungsrollen ohne Grant (`database`, `email`, `realtime`, `auth/userpass`, `legacy/jwt`, `auth/device`, `services/config` in `configtest`), die Legacy-Composition `services` (#2747) und interne Tests im `test`-Paket, die Audit- und Settings-Repositories direkt prüfen; #3944 Scheibe A: Mail-, Broadcaster-, Argon2- und Geräte-Testhilfen liegen in den Owner-`test-support`-Punkten `email/emailtest`, `realtime/realtimetest`, `auth/userpass/userpasstest` und `auth/device/devicetest`, `test` behält Aliase, `test -> email`, `realtime`, `auth/userpass` und `api/testutil -> auth/device` weg, 177 → 173; die Appender- und Data-Deletion-Isolationstests laufen intern in `database/repositories/audit`; #3944 Scheibe B, Epoche 35 per ADR 0047: Guardian-Presets und `CanUpdateStudent` über `modules/securityruntime`, `TestOperationalOverviewScopeIsTenantScoped` in die Settings-Suite (`services/config`), der Live-Group-Test schreibt den Override als Tenant-Zeile mit den Konstanten aus `modules/settings`, die Booking-Consistency-Suite nach `database/repositories/audit/integration`, `test -> auth/authorize` in beiden Scopes und die internen `test -> services/config`, `services/config/defaults`, `models/config`, `database/repositories/config`, `database/repositories/audit` weg, drei verwaiste Regeln gelöscht, 173 → 166; die zehn verbleibenden Keys: `api/testutil -> services` #2747/#2751, `legacy/jwt` #3487, `database` und `models/audit` #2750, `models/enrollment` #2747, `models/platform` Owner des Outbox-Ports, `models/users` in beiden Scopes offen (People-Directory-Vokabular), `configtest -> services/config` in beiden Scopes, weil das Settings-Schema `models/config`-Typen trägt, die kein Vertrag zeigen darf; #3965: kein Verbraucher liest Schema, `staffannouncements`, `staffmessages`, `parentmessages`, Notification-Router und -Preferences, PWA-Usage, der Change-Log-Cleanup in `services` und `supervisiondashboard/legacy` (Port `supervisiondashboard.Resolver` mit `ResolveBool`, `ResolveString` und `Prefetch` in den öffentlichen Ports, ebenso `grouplive.Resolver` für `grouplive/legacy`; das Settings-Service bietet `Prefetch` als Batch-Read mit Snapshot im Kontext, der Konstruktor liefert `BatchSettingsService`, beide Adapter importieren `services/config` nicht mehr, Legacy-LOC 710 → 701 und 548 → 539) deklarieren schmale Ports, jedes Testpaket hält einen lokalen Func-Fake `settingsDouble`, `configtest` samt Paketeintrag gelöscht, `configtest -> services/config` in beiden Scopes weg, die verwaisten Regeln `delivery.application.settings`, `communication.parent-compatibility.application-settings-application` `calendar-view.adapter-test.settings-platform-application`, `calendar-view.adapter.settings-platform-application` und `group-live-view.adapter.settings-platform-application` gelöscht, 166 → 164; Zuschnitt vom 12.10.2026 gegen den Zyklus #2748 → #2750 → #2749 → #2748: `database` und die Kontextadapter von `models/audit` fallen nicht mit #2750, sondern über [#3970](https://github.com/moto-nrw/project-phoenix/issues/3970) (Owner-Naht für den Postgres-Tenant-Runtime-Bootstrap, die `api` und `cmd` in #2750 danach mitbenutzen), das übrige Audit-Vokabular und `models/platform` über [#3967](https://github.com/moto-nrw/project-phoenix/issues/3967), `models/users` in beiden Scopes über [#3966](https://github.com/moto-nrw/project-phoenix/issues/3966) mit eigenen Zeilentypen; alle drei blockieren #2748 nativ, #3970 blockiert zusätzlich #2750 und #3967; #3970, Policy-Epoche 36 → 37 per Nachtrag zu ADR 0047: `modules/transactionruntime/compose` (`transaction-runtime`/`compose`) baut aus einem `*bun.DB` die fertige `tenant.UnitOfWork` (`NewTenantRuntime`, `NewDemoTenantRuntime`, `ObserveLockWaits`) und öffnet die Pools (`OpenPool`, `OpenServePool`), die Audit-Kontextadapter kommen über `modules/auditlog`; `test` baut Runtime und Pools dort, `test -> database` weg, 112 → 111; `test -> models/audit` hängt nur noch an Typen (`AuthEvent`, `AuthEventRepository`, `Command`, `GuardianFinancialChange`, `GuardianFinancialChangeCreator`, `DataDeletion`, `NewDataDeletion`, `DeletionTypeManual`, `EnrollmentOfferingAdjustment`, `StammdatenSectionSchoolClasses`, `BookingConsistencyRepository`, `Date`), die #3967 abbaut; weil das Deadcode-Gate keine nur von Tests erreichte Produktionsfunktion zulässt, bauen `api` und `cmd` Runtime, Demo-Runtime und Pools im selben Diff dort (zwei an das neue Paket verankerte Regeln), `services.BindTenantRuntime` samt den beiden Observer-Brücken gelöscht; `api -> database` und `cmd -> database` bleiben für die übrigen Datenbankfunktionen bei #2750; #3966: die People-Directory-Fixtures geben eigene Zeilen zurück (`test/people_rows.go`), 34 Suitendateien der Repository-Suiten laufen in `modules/peopledirectory/contracttest` und benennen `models/users` selbst, die beiden Stranding-Batch-Suiten bleiben in `modules/peopledirectory`; die Withdrawal-Fixtures schreiben die offene Aufgabe als Zeile; die drei Werte, die Produktionscode als Modell verlangt (Kind der Review-Policy, Query-Optionen der Vertretungs-Suite, Personen-Double der Timetable-Operations), liegen in den `services`-Suitenhilfen hinter `api/testutil`; `test -> models/users` in beiden Scopes weg ohne neuen Key, 111 -> 109, die verwaiste Regel `people-directory.behavior-test.enrollment-support` gelöscht; #3967 ohne ADR und Epoche: `CapturingOutbox` liegt im neuen Punkt `models/platform/platformtest` (`organization-tenancy`/`test-support`, Muster Scheibe A, zwei an ihn verankerte Regeln), `test` behält den Alias; die Fixtures geben eigene Operator- und Organisationszeilen zurück (`test/platform_rows.go`) und schreiben Data-Deletion- und Offering-Adjustment-Zeilen selbst (`test/audit_rows.go`), der Stammdaten-Abschnitt ist der gespeicherte Spaltenwert, `NewAuditCommand` ersetzt die beiden typisierten Command-Helfer generisch über die `Create`-Methode der Trail, die Booking-Consistency-Szenarien nehmen eine Funktion `BookingConsistencyAudit`, die die Audit-Integrationssuite aus ihrem Repository baut; `test -> models/platform` und `test -> models/audit` weg, 86 → 84; #4014 ohne ADR und Epoche: die Fixtures bilden ihre `enrollment.care_offerings`-Zeile selbst ab (`test/care_offering_rows.go`), die Phasen-, Antrags-, Kind- und Angebotswerte kommen aus `modules/enrollment/enrollmenttest` (die Angebotswerte `fixed` und `parent_choice` dort als gespeicherte Spaltenwerte, weil kein öffentliches Owner-Paket sie nennt; die Auswahlregel nimmt den Spaltendefault), die Timetable-Suiten legen ihre Quellangebote über `InsertTestCareOffering` statt über `EnrollmentCareOfferingRows()` der Legacy-Factory an, die Factory-Methode ist gelöscht; `test -> models/enrollment` weg, 60 → 59; offen bleiben `api/testutil -> services` (#2747) und `test -> legacy/jwt`) #4001: `test -> legacy/jwt` weg, der Session-Adapter (`identity-access/adapter`) ist per ADR 0053 Punkt 7.3 stehendes Fixture-Ziel, 2 → 1. #4005: `api/testutil -> services` weg, `api/testutil` hält nur noch owner-agnostische HTTP-Helfer, 1 → 0. Abschluss: die letzte Kompatibilitätsregel unter #2748, `test-support.e2e-test.inbound-parent-http`, ist gelöscht; die Kalender-E2E-Suite mountet das Elternportal über `api.CalendarTestModule.ParentCalendarRouter` und importiert `modules/careplan/inbound/parent` nicht mehr; Kompatibilitätsregeln 50 → 49, Familie `test-support` leer, Baseline unverändert bei 39.
- [x] [#2743](https://github.com/moto-nrw/project-phoenix/issues/2743) repository Factory — 0 (alle 15 Keys weg, dazu acht #2750-, drei #2747- und ein #3421-Key, 139 → 112; #3556: `models/education` entfernt, die Werte kommen über `modules/schoolstructure/compose`; ADR 0049, Policy-Epoche 35 → 36: Audit- und Settings-Persistenz bauen `modules/auditlog/compose` und `modules/settings/compose`; jeder übrige Adapter des Wurzelpakets liegt in seiner Owner-Komposition oder in `services`, `repositories.Factory` und `NewFactory` gelöscht (Caller-Inventar `callers/repository-factory-2743.json`), `database/repositories` hat keine Go-Datei mehr und verlässt Paketliste und `legacy_composition`; die Spiegel-Nähte der Timetable-Komposition auf ihre Aufrufer verengt, Care Plan 50 → 40, Workforce 15 → 9 Methoden; Composition 490 → 260)
- [x] [#2747](https://github.com/moto-nrw/project-phoenix/issues/2747) service Factory — 0 (#3755: `services -> services/users` entfernt; #2743: `services -> database/repositories` in Produktion und externem Test sowie die Composition-Referenz `services -> repositories.Factory` entfernt; #3970: `BindTenantRuntime` und die Observer-Brücken gelöscht; Teilschnitt per ADR 0050, Policy-Epoche 37 → 38: 23 Keys weg, 109 → 86. Die Subpakete `services/*` und das Vokabular von `models/{config,platform,audit,users}`, `email`, `realtime`, `analytics` kommen über die Owner-Kompositionen und -Verträge, `internal/timezone` über `sharedkernel/calendar`, `sql.ErrNoRows` über `modules/transactionruntime`, die externen Tests laufen intern; die Produktionsadapter halten keine DB-, Tenant- oder JWT-Logik mehr, die liegt in den Owner-Kompositionen oder kommt als gebauter Wert von der Root. Offen: `bun`, `tenant`, `legacy/jwt` — die Mehr-Owner-Builder `services/*_roots.go`, `command_roots.go`, `cleanup_composition.go`, `factory.go` und rund 90 Test-Hooks brauchen sie; ihr legales Zuhause (Root mit rund 30–40 neuen Rechten oder anderes) ist eine offene Entscheidung, `legacy/jwt` zusätzlich #3487; Factory noch nicht gelöscht) Entschieden in ADR 0053: letzter Schritt nach [#4005](https://github.com/moto-nrw/project-phoenix/issues/4005) bis [#4013](https://github.com/moto-nrw/project-phoenix/issues/4013) (Factory und `services` löschen). #4001: die stehenden Reichweiten gelten ab Policy-Epoche 41; `e2e-test.compose` erreicht nur noch `legacy-composition` und trägt #2747, bis `services` gelöscht ist; die drei temporären Kompositionsregeln `audit-platform.compose.retained-application`, `audit-platform.compose.vocabulary-domain` und `data-import.compose.retained-application` fallen unter die Owner-Kompositions-Reichweite und sind gelöscht. #3487 erledigt: `services -> legacy/jwt` heißt jetzt `services -> sessiontoken`, der Key hängt nur noch an #2747. #4005: `services` hält keine `*_test_helpers.go` mehr, kein Test außerhalb von `services` nutzt ein Hook-Symbol. #4006: `services -> sessiontoken` weg, die Root komponiert Identity & Access; offen bleiben `bun` und `tenant`. #4008: die Workforce-Verdrahtung liegt in `modules/workforce/compose` und `api`, die Factory bekommt sie als `WorkforceWiring` von der Root, Composition 250 → 249, `services` 260 → 225 Produktionsdateien. #4009: die Timetable-, Schulkalender- und Erinnerungs-Verdrahtung liegt in `modules/timetable/compose`, `modules/schoolcalendar/compose` und `api`, die Factory bekommt sie als `TimetableWiring` von der Root, Composition 237 → 236, `services` 156 → 127 Produktionsdateien. Abschluss: `services` und die Factory sind gelöscht. Die Root komponiert die Dienste in `newServiceGraph` (`api/service_graph_compose*.go`) in Stufen über die Owner und die Verdrahtung, die sie gebaut hat; jede Stufe nimmt den Bau und gibt ihn erweitert zurück, spät gebundene Werte liest der Dienst zur Aufrufzeit. `newServeServiceGraph` bedient den Server, `NewServiceGraphForTests` (vorher `NewFactoryForTests`) die Testgraphen; `Factory.SetTenantRuntime` ist `bindTenantRuntime` des Graphen, `EnableStudentPhotos` entfällt, den Foto-Lebenszyklus baut der Graph aus Slot und Unlinker der Root. Was die Root nicht importieren darf, kommt über Owner-Kompositionen: Delivery-Dienste über `modules/delivery/compose`, Projektionen über `modules/grouplive/compose`, `modules/supervisiondashboard/compose` und das neue `modules/emergencysnapshot/compose`, der PWA-Nutzungsspeicher über das neue `observability/compose`, die Vertretungszeile über `modules/workforce/compose`; Timetable nimmt verbraucherseitige Ports (`LifecycleStaff`, `DeviationStaff`, `CorrectionPeople`), People Directory benennt seine Retained-Speicher (`Repositories`); Regel 1 und gocognit 15 halten unter `api/` ohne Allowlist-Eintrag. Policy: Paketeintrag `services`, `legacy_composition`-Eintrag `services.Factory` und 69 verwaiste Regeln gelöscht, zwei neue Compose-Pakete klassifiziert, Epoche unverändert 44. Keys 39 → 34 (`services -> bun`, `services -> tenant` und die drei #2750-Keys); der Composition-Ratchet zählt `api.serviceGraph` als zentrales Aggregat und bildet die Felder von `services.Factory` darauf ab, Composition 140 → 139 nach dem Merge von #2749 (Setter `EnableStudentPhotos` weg). Caller-Inventar `callers/service-factory-2747.json`.
- [x] [#2751](https://github.com/moto-nrw/project-phoenix/issues/2751) Legacy-Composition entfernt, Ratchet 0, Check ohne Baseline grün; aktueller Nachweis im Zero-Ratchet-Cutover von #4004.
- [x] [#2745](https://github.com/moto-nrw/project-phoenix/issues/2745) API-Aggregat — 0 (`api.API` und `api.New` gelöscht, `api` aus `legacy_composition`; jede Ressource wird gemountet, wo sie gebaut wird, `serveGraph` hält nur Router, Pool, Runtime und die Worker-Komposition (#2749); fünf #2750-Keys der reinen Feldtypen weg, 206 → 201, Composition 591 → 490)
- [x] [#2749](https://github.com/moto-nrw/project-phoenix/issues/2749) Scheduler-Setter, breite Test-Composition — 0 (die `Scheduler.Set*`-Methoden und `SetupAPITest` waren mit den Blockern schon weg; `newScheduler` baut den Scheduler und `api.workerDependencies` die `WorkerDependencies` jetzt in je einem Literal, die fünf Verdrahtungsfunktionen danach sind gelöscht; die Felder für Intervall und Schwelle der Sitzungsbereinigung las nur noch Testcode, sie sind gelöscht, das Pausen-Intervall entsteht beim Registrieren des Jobs; Caller-Inventar `callers/scheduler-setters-2749.json`, Composition 219 → 140, Keys 39 → 39; übrig: die Tageswechsel-Caches der Overdue- und Erinnerungs-Sperren (Job-Zustand) und der interne Testbauer `unitScheduler`; die Factory-Lesezugriffe des Workers über `serveGraph` fallen mit #2747)

#4078: School Portal wird Identity & Access HTTP. Root bindet Class Day,
Supervisions und Notifications über reine Router-Ports und normalisiert optionale
native Nil-Pointer vor der Interface-Konversion. Das Verhalten der Auth-/MFA-/Reset-
Handler bleibt unverändert; Root bindet die native Tenant-Kontext-Funktion. Die externen Router- und Tenant-Vertragstests bauen
über Root; der native Timetable-Wert bleibt derselbe. Konfigurationswerte und
Schedule-Fixtures kommen aus vorhandenen öffentlichen Settings-/Test-Fassaden.
Zwei #4078-Keys entfallen; dreizehn Regeln werden durch sechs engere Crypto-
Rollen-/Scope-Regeln ersetzt. HTTP und Adapter-Tests erhalten keine Crypto-Freigabe.
Keine Selektoren, neuen Graph-Felder, Routen-, Response- oder Golden-Änderungen.

#4078: Notification-HTTP gehört Delivery Platform; `inbound-notifications` entfällt.
Root bindet native Middleware, Caller-Fakten, Rendering und Fehler, während der
Adapter nur eigene typisierte Ports und DTOs nennt. Öffentliche Staff-Capabilities
begrenzen Push und Präferenzen auf Staff/School; Parent-Capabilities bleiben getrennt.
Native Methoden und Fehlerwerte bleiben erhalten, auch Nil-/Leer-Slices und
unabhängige Fehlerfamilien. Die echten E-Mail-Permission- und JWT-/Tenant-Routertests
bauen über Root; alle 44 bisherigen Assertions bleiben semantisch und nach
Namespace-Normalisierung AST-identisch. Fünf Regeln und zwei #4078-Keys entfallen,
keine Nachfolger, Selektoren, neuen Freigaben oder Root-Graph-Felder. Composition
1.657 und Budget-Caps bleiben unverändert. Routen, Responses und Goldens bleiben gleich.

#4078 / #4076: People Directory nennt für Dokument-Bytes nur einen eigenen
Koordinator-Port. Root bindet dieselbe lazy Backend-Auflösung und File Storages
nativen Koordinator über dessen reine Komposition. Native IO-/Header-/Cleanup-
Methoden bleiben unverändert; der identische doppelte Dateinamen-Generator
entfällt aus Compose. Ein #4076-Key fällt; keine Regel, Owner- oder Policy-Änderung.
File Storage Compose 642 → 636, Gesamtkappe 55.887 → 55.881; Composition 1.657
bleibt gleich. Routen, Responses und gesamte Golden-Dateien bleiben unverändert.

#4078: Der Request-Identity-Memo liegt bei Security Runtime/Public statt im
Session-Token-Adapter. Identity behält die Einträge; Root bindet weiter denselben
Memo-Port. Middleware, Note-Eviction und Verhaltenstests nutzen dieselbe Implementierung.
Paketklassifikation, Regeln, Keys und Composition ändern sich nicht. Die bestehende
Memo-Port-Bindung liegt nun in einer nativen Root-Datei: Der Legacy-File-Budgeteintrag
sinkt von 73 auf 64 LOC, die Summe von 3.795 auf 3786 LOC.


#4079 bounded delivery / #4314 decision: Request Review's native staff response
mappers move from `compose` into its public capability. Students HTTP stays
in People Directory and calls that single implementation, retiring its exact
`requestreview/compose` key. Policy epoch 72 → 73, rules 827 → 830, keys
101 → 100 (#4079 15 → 14), Root composition 1,652 unchanged. Three narrow
projection-public → Care Plan public/contract and Security Runtime public
rules serve actual imports. No rule is deleted. Compose LOC cap falls by 186.
The full candidate's Root-to-Root/test edges and new composition fields are not
approved. ADR 0058 records the settled owners and round-16 resolutions;
`docs/architecture/4079-remaining-http-edges.md` lists all 14 retained keys,
their blockers and two alternatives each. #4079 is not closed by this slice.


#4079 / #4314: Option 1 für alle 14 verbleibenden HTTP-Keys umgesetzt.
Owner-HTTP wird aus vorhandenen Root-Werten montiert; keine neuen Root-Felder.
Keys 76 → 62, Composition 1.644 unverändert, Compose-Cap 33.216 unverändert.
[Owner-HTTP-Nachweis](../../docs/architecture/4079-owner-http-evidence.md).
