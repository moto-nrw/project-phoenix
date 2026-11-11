---
status: proposed
---

# Ferienplatzänderungen und Neueinfrieren haben einen Workflow

**Status: Vorgeschlagen.** Vorschlag vom 11.11.2026 zu
[#6555](https://github.com/moto-nrw/project-phoenix/issues/6555), nach Yannicks
Entscheidung im Issue vom selben Tag. Yannick gibt die Architektur frei.
Das Zusammenführen dieses Doku-PRs nimmt die ADR nicht an und erlaubt noch
keinen neuen Owner im Produktcode.

## Anlass und Umfang

Drei Wege ändern Ferienplätze in Enrollment und berechnen danach die
festgehaltenen Beiträge in Parent Billing neu: Umbuchen und Kapazität ändern
(#4699, PR #6367) sowie die Büroentscheidung nach Kriterien (#4700,
[PR #6448](https://github.com/moto-nrw/project-phoenix/pull/6448), Commit
`4bdf9779b8`). Alle drei brauchen eine gemeinsame Sperrreihenfolge und
Transaktion. Das ist Koordination nach
[ADR 0067, Addendum #4538](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md#addendum-2026-10-23-command-participation-and-coordination-4538),
keine bloße Teilnahme eines fremden Leaf-Commands.

Ausgangspunkt ist der
[Bot-Entwurf 48b2877dcf](https://github.com/moto-nrw/project-phoenix/commit/48b2877dcfd4caf3e2ae8d739bb4d93c8ec1f358)
auf `holiday-allocation-billing-workflow`. Übernommen werden der datenlose
Workflow, explizite Ports für Rechnungssperre und Neueinfrieren sowie der
Wegfall des Entscheidungs-Callbacks. Ergänzt werden Umbuchung und
Kapazitätsänderung. Dessen ADR 0199, Status `accepted`, Epoch 100 und konkrete
Budgeterhöhungen werden nicht übernommen: Nummer 0199 ist inzwischen anders
belegt; Policy und Budgets sind am späteren Implementierungsstand zu prüfen.
Der Entwurfsbranch bleibt erhalten.

Diese ADR verschiebt Verantwortlichkeiten für genau diese drei Wege. Sie
ändert weder HTTP-Verträge noch Berechtigungen, Vergabekriterien, Preise,
Geschwisterregeln oder Rechnungen. Bestätigung, Stornierung, Zusammenführen,
Trennen und Debitorennummer bleiben in ihrem bestehenden Schnitt; ihre
Rechnung-vor-Zeitraum-Reihenfolge bleibt verbindlich.

## Heutiger Ablauf als Sperrbeleg

Gelesener Stand ist
[`development` 292e3ef72d](https://github.com/moto-nrw/project-phoenix/tree/292e3ef72d25377668eb08b72c9b829950eed1b3).
Die folgenden Pfade sind relativ zu `backend/`; die Links halten diesen Stand fest.

| Weg | Heutiger Koordinator und Reihenfolge innerhalb derselben Träger-Transaktion |
|---|---|
| Umbuchen | Parent Billings `HolidayContributions.RebookHolidayChild` ruft `lockSiblings`: Abrechnungssperre → `HoldHolidayPeriod` → Gruppen und ausgestellte Rechnungen lesen. Enrollment sperrt die Anmeldung, prüft Änderbarkeit und Tage, ersetzt Buchungen und sperrt betroffene Wochen aufsteigend für die Platzvergabe. Nachrücken schreibt Verlauf und Mail-Outbox. Parent Billing ruft `refreezeMovedUp` mit `MovedUp` auf. |
| Kapazität ändern | Parent Billings `SetHolidayWeekCapacity` nimmt dasselbe Präfix. Enrollment hält den Zeitraum und die Woche, schreibt Kapazität und Verlauf, vergibt freie Plätze und schreibt Nachrück-Mails; Parent Billing berechnet die Gruppen der nachgerückten Anmeldungen neu. |
| Büroentscheidung | `enrollment/http.NewBilledHolidayAllocation` ruft Parent Billings `AllocateHolidayPlaces` auf. Dieser nimmt dasselbe Präfix und ruft den Enrollment-Callback auf. Enrollment hält Zeitraum → Kriterienkatalog → gewählte Wochen aufsteigend, prüft den aktuellen Vorschlag und schreibt Entscheidungen, Plätze, Verlauf und Outbox. `PlacedRegistrations` enthält direkte Zuteilungen **und** Nachrücker. Danach folgt `refreezeMovedUp`. |

Belege: [heutige Parent-Billing-Koordination](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/parentbilling/internal/application/holiday_places.go#L150),
[`lockSiblings`](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/parentbilling/internal/application/holiday_siblings.go#L223),
[Umbuchung und Anmeldungslock](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/enrollment/internal/application/holiday_registration_review.go#L386),
[Kapazitätsänderung](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/enrollment/internal/application/holiday_places.go#L370)
und [Büroentscheidung](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/enrollment/internal/application/holiday_allocation_decide.go#L18).

## Entscheidung

### Owner und Verträge

Ein neuer Owner **`holiday-allocation`**, Kind `workflow`, lebt unter
`backend/workflows/holidayallocation` (Arbeitsname). Er besitzt keine Tabellen,
Projektionen oder eigene SQL-Abfragen. Wie bei
[ADR 0013](0013-staff-offboarding-is-an-application-workflow.md) und
[ADR 0018](0018-open-room-move-is-an-application-workflow.md) besitzt er das
ownerübergreifende Protokoll: äußere Einheit, Reihenfolge und Fehlerabbruch.
Er bietet drei explizite Commands: `RebookHolidayChild`,
`SetHolidayWeekCapacity`, `DecideHolidayAllocation`.

| Liefernder Owner | Consumer-owned Port des Workflows und Grenze |
|---|---|
| Parent Billing (`parent-billing`) | `Billing`: `HoldHolidayAllocationInvoices` sperrt ausschließlich die Abrechnung dieses Zeitraums; `RefreezeHolidayAllocationContributions` berechnet für die übergebenen betroffenen Anmeldungs-IDs deren bestätigte Geschwistergruppen neu. Beide sind öffentliche, geprüfte Commands, keine Repository-Fassade und kein Callback in einen fremden Owner. |
| Enrollment (`enrollment`) | `Periods`: `HoldHolidayPeriod`; `Places`: Umbuchung und Kapazitätsänderung; `Allocation`: Büroentscheidung. Die vorhandenen öffentlichen Commands behalten ihre Guards und liefern Ergebnis plus Nachrücker beziehungsweise `PlacedRegistrations`. Vorhandene öffentliche Werte werden wiederverwendet, keine Kopie interner Modelle. |
| Tenant Runtime (`tenant-runtime`) | `WithinTraeger`: öffnet eine Träger-Transaktion oder tritt der laufenden Einheit desselben Trägers bei. Eine Schul-, Admin- oder fremde Träger-Transaktion wird abgelehnt. |

Nach [ADR 0068](0068-principle-owners-compose-themselves-and-the-roots-only-wire.md)
baut `holidayallocation/compose` nur den eigenen Workflow und bindet den
Träger-Transaktionsläufer. Die Root baut Enrollment und Parent Billing,
adaptiert ihre öffentlichen Capabilities an diese Ports und bindet die drei
Commands an die vorhandenen HTTP-Ressourcen. Die Ressourcen erhalten einen
schmalen Schreibport statt des bisherigen Koordinators. Lesefunktionen und
andere Commands bleiben direkt an ihren Owner gebunden. Weder Enrollment
noch Parent Billing importieren den Workflow; HTTP und Root führen keine
Sperr- oder Beitragsentscheidungen aus.

Der Workflow verlangt den bisherigen Büro-Akteur mit positivem Träger- und
Konto-Schlüssel. Die HTTP-Grenze behält Träger-Sitzung und Recht
`traeger_enrollment:manage`; die Owner prüfen weiterhin Träger, Identität und ihre
fachlichen Guards. Jeder Port erhält denselben Transaktionskontext und
dieselben Schlüssel. Es gibt keinen Admin- oder Schul-Scope-Wechsel.

### Transaktion und Sperrreihenfolge

Für jeden der drei Commands gilt:

1. Eingaben und Akteur prüfen, `WithinTraeger` öffnen oder beitreten.
2. Parent Billing nimmt **B**, `LockPeriodBilling` für Träger,
   `ServiceHoliday` und Zeitraum. „Rechnungen sperren“ bedeutet heute ein
   `pg_advisory_xact_lock` mit Schlüssel
   `parent_billing.period_billing:<Träger>:<Leistung>:<Zeitraum>`, kein
   `FOR UPDATE` jeder vorhandenen Rechnung. Es gilt auch ohne Rechnung.
3. Enrollment nimmt **P**, die Zeile des Ferienzeitraums `FOR UPDATE`.
   Erst danach werden veränderliche Grundlagen gelesen. Die bisherigen
   ungenutzten Gruppen-/Rechnungsabfragen von `lockSiblings` müssen in diesen
   drei Wegen nicht vorab weiterlaufen; Parent Billing liest seine Grundlagen
   für das Neueinfrieren unter denselben gehaltenen Sperren.
4. Enrollment führt den gewählten Command aus und liest unter seinen
   Sperren erneut: Umbuchung **P → Anmeldung → betroffene Wochen aufsteigend**;
   Kapazität **P → Woche**; Büroentscheidung **P → Kriterienkatalog → Wochen
   aufsteigend**. Vorschlagsvergleich, Pending-Status, freie Plätze,
   Änderbarkeit und Tagesgrenzen bleiben dort. Wiederholtes Sperren des
   bereits gehaltenen Zeitraums beziehungsweise derselben Wochen in derselben
   Transaktion fügt keine Gegenrichtung hinzu.
5. Parent Billing friert anhand des Ergebnisses die betroffenen Gruppen neu
   ein. **B und P bleiben bis zum Commit gehalten**, einschließlich aller
   Enrollment-Lesezugriffe, Beitragsfassungen, Verlaufseinträge und Outbox-Zeilen.
   Nur die äußerste Einheit committet; bei Teilnahme propagiert der Workflow
   den Fehler bis zu deren Rollback.

Der Workflow bearbeitet einen Zeitraum je Aufruf und sperrt keinen
Abrechnungslauf. Für die geprüften Interleavings ist die Reihenfolge
deadlock-frei: Zwei dieser Commands auf demselben Zeitraum warten bereits an
B, bevor einer P oder dessen nachgeordnete Zeilen hält. Bestätigung,
Stornierung, Geschwisteränderung und Debitorennummer nehmen ebenfalls B → P.
Die Freigabe eines Ferienlaufs nimmt zuerst ihre eigene Laufzeile, dann B,
und liest erst danach die Beitrags-/Anmeldungsgrundlagen. Der Workflow nimmt
keine Laufzeile, also entsteht kein Rückweg vom gehaltenen P zur Laufzeile.
Ein reiner Enrollment-Schreiber nimmt P und verlangt danach kein B. Der
Kriterienkatalog-Schreiber nimmt Katalog/Kriterien, aber weder P noch B;
zwei Büroentscheidungen verschiedener Zeiträume warten gegebenenfalls am
gemeinsamen Katalog, verlangen aber nicht den Zeitraum des anderen.

Deshalb darf das Neueinfrieren weder nachträglich eine zuvor ungenommene
Abrechnungssperre noch eine Laufzeile anfordern. Die Enrollment-internen
Unterschiede zwischen Anmeldung und Wochen bilden keinen Zyklus, weil die
Schreiber desselben Zeitraums schon durch P serialisiert sind. Das ist eine
aus den heutigen Sperrpfaden abgeleitete Aussage, kein ausgeführter
Nebenläufigkeitstest und kein Beweis globaler Deadlock-Freiheit. Neue
Teilnehmer oder mehrere Zeiträume pro Einheit brauchen einen neuen Abgleich.

Zusätzliche Belege: [Abrechnungssperre](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/parentbilling/internal/adapters/postgres/billing_runs.go#L332),
[Freigabe](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/parentbilling/internal/application/billing_runs.go#L267),
[sortierte Wochensperren](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/enrollment/internal/adapters/postgres/holiday_week_places.go#L40)
und [Kriterienkatalog-Schreiber](https://github.com/moto-nrw/project-phoenix/blob/292e3ef72d25377668eb08b72c9b829950eed1b3/backend/modules/enrollment/internal/adapters/postgres/holiday_criteria.go#L164).

### Was bei den Fachmodulen bleibt

**Enrollment** besitzt Zeitraum, Wochen, Kapazität, Anmeldungen und Buchungen,
Kriterien/Rangfolge, Vorschlagsprüfung, Entscheidungen, Nachrücken und Verlauf.
Es schreibt Entscheidungs- und Nachrück-Mails über den bestehenden
Träger-Outbox-Port in derselben Transaktion. Der Worker stellt nur committete
Einträge nach seiner bestehenden Wiederholungsregel zu; es gibt keinen
direkten Versand oder verbindlichen After-Commit-Callback im Workflow.

**Parent Billing** besitzt Abrechnungssperre, Geschwistergruppen, Regelkette,
Beitragsfassungen, Rechnungen und Korrekturbedarf nach
[ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md).
Umbuchung und Kapazität übergeben die Nachrücker, die Büroentscheidung alle
direkt platzierten und nachgerückten Anmeldungen. Parent Billing dedupliziert
Gruppen und berechnet auch bestätigte Geschwister einer noch unbestätigten
Anmeldung neu. Die noch unbestätigte Anmeldung selbst friert keinen Beitrag ein.

Direkte Umbuchungen behalten ihren eingefrorenen Grundpreis; zusätzliche
Nachrück-Wochen bekommen den heutigen Preis. Bereits bezahlte Ersatzwochen
werden über die bestehende Ersetzungskette nicht doppelt berechnet.
Geschwisterrang, festgehaltene Regelwahl, Befreiung und Kostenübernahme bleiben
in der bisherigen Berechnung. Nur eine geänderte bestätigte Berechnung erhält
eine neue Fassung. Auch nach ausgestellter Rechnung darf ein Nachrücker den
Beitrag ändern: Die Rechnung behält ihre Positionen, `correction_needed`
zeigt den Unterschied. Die Sonderregel für eine inzwischen entfallene
Befreiung in `refreezeMovedUp` bleibt erhalten.

### Fehler und Wiederholung

Jeder Fehler beim Sperren, Enrollment-Command, Lesen, Audit, Neueinfrieren
oder Enqueue verwirft die gesamte Einheit, einschließlich Platzänderung,
Kapazität, Entscheidung, neuer Beitragsfassung und beider Mailarten. Es gibt
keinen erfolgreichen Teilschritt, keine spätere Beitragsreparatur und keinen
best-effort-Zweig. Öffentliche Fehleridentitäten und die bestehende
HTTP-Zuordnung bleiben erhalten; notwendige Typ-/Fehleradapter liegen in der
Root, fachliche Refusals bei ihren Ownern.

`WithinTraeger` verwendet heute `execute(..., false, ...)`: Der Workflow
führt keinen automatischen DB-Retry ein und wiederholt keinen einzelnen
Portaufruf nach einem Fehler. Ein abgebrochener Versuch kann nur als ganze
neue Transaktion mit erneuter Berechtigungs- und Zustandsprüfung laufen.
Bei geändertem Vergabevorschlag muss das Büro neu lesen und entscheiden;
ein blindes Replay ist unzulässig. Nach unklarer Commit-Antwort liest der
Client den Stand neu. Es wird keine allgemeine HTTP-Idempotenz zugesagt:
Wiederholte Umbuchung/Kapazitätsänderung behalten ihre heutigen Audit-Regeln,
bereits entschiedene Buchungen sind nicht mehr pending. Unveränderte Beiträge
erzeugen keine weitere Fassung; Outbox-Schlüssel der Entscheidung und der
jeweiligen Buchungsgeneration verhindern doppelte Zustellaufträge für
dasselbe Ereignis. Eine erneute echte Beförderung bleibt ein neues Ereignis.

### Architekturprüfung bei der späteren Umsetzung

Nach Yannicks Annahme wird der neue Workflow nach
[ADR 0070](0070-principle-only-the-target-is-a-rule-and-debt-leaves-with-evidence.md)
registriert. Dieser Doku-PR ändert noch keinen dieser Einträge:

| Register/Policy | Erforderlicher Schnitt im Implementierungs-PR |
|---|---|
| `backend/architecture/policy.json` | Neuer datenloser Owner `holiday-allocation`, Kind `workflow`, mit dann aktueller Policy-Epoch nach dem Verfahren für neue Workflow-Pakete. |
| `backend/architecture/policy/holiday-allocation.json` (neu) | Neue Pakete `workflows/holidayallocation` (`public`) und `compose`; gegebenenfalls eigene `port`-Rolle. Nur benötigte öffentliche Enrollment-/Parent-Billing-Wertetypen und Tenant-Runtime-Verträge erlauben, getrennte Testrollen registrieren. Ports ohne Fremdtypen brauchen keine Importregel zum Lieferanten. Keine fremden Internals, SQL oder eigenen `data_objects`. |
| `policy/enrollment.json`, `policy/parent-billing.json` | Datenobjekte behalten ihre Owner; bestehende `http.public`-Regeln bleiben für die jeweiligen Fachfunktionen. HTTP konsumiert die Workflow-Commands über eigene Ports, ohne Domain→Workflow-Import und ohne neue Erlaubnis für den alten Koordinator. Parent Billings `HolidayRegistrations` verliert die beiden Platzänderungs-Methoden; seine weiterhin benötigten Lese-/Bestätigungs-/Storno-/Zeitraum-Commands bleiben. |
| Command-Effects: `reviews/enrollment.json` und zugehörige `evidence/enrollment.json` | Den `leaf`-Eintrag für `enrollment/http.HolidayAllocationBilling.AllocateHolidayPlaces` mit dem entfernten HTTP-Decorator/Callback entfernen; tatsächliche Workflow-Bindungen und deren Effekte erfassen. |
| Command-Effects: `reviews/parent-billing.json` und zugehörige Evidence, Workflow-Evidence | Die Bindungen `HolidayRegistrations.RebookHolidayChild` und `.SetHolidayWeekCapacity` aus Parent Billing entfernen und die drei vollständigen Protokolle beim Workflow nachweisen. Keine bloße Umklassifizierung des unveränderten Domain-Koordinators als Leaf. |
| Composition-/Pass-through-/Quell-Ratchets und Architektur-Inventare | Die neue Composition und verschobene Commands vollständig messen; entfernte Callback-/Decorator-Flächen abziehen. Keine pauschale Budgetfreigabe durch diese ADR, keine Übernahme der Zahlen des Bot-Entwurfs, kein Wachstum von `legacy.jsonl`. Benötigte Erhöhungen erfordern konkrete, gesondert freizugebende ADR-Belege. |

Abhängigkeiten verlaufen von HTTP-Ports über die Root-Bindung zum Workflow
und von dessen Ports zu öffentlichen Owner-Commands. Das hält den Graphen
azyklisch nach [ADR 0071](0071-principle-owner-dependencies-point-one-way.md).

## Verworfene Alternativen

- **Status quo / als Leaf dokumentieren:** Die gemeinsame Reihenfolge B → P
  und das anschließende Neueinfrieren sind ein eigenes Protokoll nach ADR 0067.
  Ein HTTP-Decorator oder ein Callback in Parent Billing verbirgt dessen Owner.
- **Choreografie über Events:** Eine später verarbeitete Platzvergabe ließe
  zwischen Platzänderung und Beitragsfassung ein Fenster für die Freigabe
  eines falschen Ferienlaufs. Kompensation, zusätzliche Freigabesperren und
  Zustände wären neue Semantik. Events bleiben für dauerhafte Zustellung;
  Plätze und Beiträge committen gemeinsam.
- **Nur Büroentscheidung verschieben:** Lässt zwei Kopien desselben Protokolls
  in Parent Billing und erfüllt #6555 nicht. Ebenso unzureichend ist ein
  Workflow, der nur den unveränderten Parent-Billing-Koordinator aufruft.

## Abnahmekriterien der Umsetzung

Die spätere Umsetzung muss folgende Nachweise liefern; diese ADR behauptet
keinen bereits ausgeführten Produktcode-Test:

1. Porttests für **alle drei** Commands: B → P → Enrollment → Neueinfrieren,
   derselbe Träger-Kontext, Abbruch an jeder Stufe, keine späteren Aufrufe nach
   einem Fehler, direkte Plätze und Nachrücker vollständig weitergegeben.
2. Real-DB-Rollbacktests mit abgelehntem Beitrag für jeden Weg: keine
   Platz-/Kapazitäts-/Entscheidungsänderung, keine Beitragsfassung und kein
   Outbox-Eintrag. Als bestehende Referenz bleibt
   `TestHolidayAllocationHoldsFreePlacesForThePendingBookings` grün.
3. Zweiverbindungs-Tests mit beobachteten PostgreSQL-Sperren: jeder Weg gegen
   Ferienlauf-Freigabe, Wege untereinander im selben Zeitraum und
   Büroentscheidung gegen Katalogänderung/anderen Zeitraum. Kein `40P01`,
   kein veralteter Beitrag im freigegebenen Lauf, geänderter Vorschlag abgelehnt.
4. Bestehende API-Regressionen: `holiday_allocation_regression_api_test.go`,
   `TestHolidayPlacesMoveUpOnRebookingAndCapacity` und
   `holiday_places_review_api_test.go` (Grundpreis, Ersatzwoche, Geschwister,
   ausgestellte Rechnung, erneute Beförderung). Autorisierung,
   Träger-Isolation und Fehlerzuordnung bleiben geschützt.
5. Architekturprüfung samt aktualisierter Effect-/Composition-Evidence,
   relevanten Tests und `scripts/check-changed.mjs` grün, ohne neue verborgene
   Koordination oder Ratchet-Ausnahme.
