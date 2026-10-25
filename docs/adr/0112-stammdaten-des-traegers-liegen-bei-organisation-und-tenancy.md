---
status: accepted
---

# Stammdaten des Trägers liegen bei Organisation & Tenancy

Entscheidung vom 25.10.2026 zu [#4653](https://github.com/moto-nrw/project-phoenix/issues/4653),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Story 9. Sie setzt
[ADR 0086](0086-traeger-werte-liegen-zwischen-standard-und-schule.md)
Abschnitt 1 für das Branding um und hält fest, was das in den Registern
ändert (ADR 0057 Punkt 5, ADR 0070). Sie führt keinen neuen Owner ein.

## Kontext

Das Träger-Büro soll Logo, Farbe, Briefkopf, Fußzeile, Ansprechpartner und
eine oder mehrere Bankverbindungen hinterlegen. Spätere Dokumente des Trägers
(Bestätigung, Rechnung, Vertrag; #4669) nutzen diese Angaben. ADR 0086
Abschnitt 1 sagt: Logo und Briefkopf-Grafik sind Träger-Stammdaten bei
Organisation & Tenancy, Farben und Texte des Brandings sind Einstellungen,
Bankverbindungen sind fachliche Objekte mit Träger-ID bei ihrem Fach-Owner.
Der Owner Elternabrechnung aus
[ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
hat noch keinen Code. Die Schul-Logos liegen als URL in
`platform.schools.settings`; File Storage verwaltet nur Dateien einer Schule.

## Entscheidung

1. **Einordnung.**
   - Logo, Ansprechpartner (Name, Telefon, E-Mail) und Bankverbindungen sind
     Stammdaten des Trägers bei Organisation & Tenancy:
     `platform.traeger_profiles` (eine Zeile je Träger mit Logo und
     Ansprechpartner) und `platform.traeger_bank_accounts`.
   - Farbe, Absender im Briefkopf und Fußzeile sind Einstellungen der Ebene
     `traeger` (`branding.primary_color`, `branding.letterhead_text`,
     `branding.footer_text`). Sie sind die ersten Schlüssel dieser Ebene;
     ADR 0107 Punkt 7 gilt für sie nicht mehr, weil die Vorschau im
     Träger-Portal und #4669 sie lesen. Die Schulen sehen sie nur lesend.
   - Eine eigene Briefkopf-Grafik gibt es in v1 nicht. Der Briefkopf besteht
     aus Logo, Absender und Farbe.
2. **Bankverbindungen.** Die Konten, auf die Eltern an den Träger zahlen,
   sind Stammdaten des Trägers, nicht der Elternabrechnung. Jede hat eine
   Bezeichnung, die je Träger eindeutig ist, Kontoinhaber, IBAN (ISO 13616
   mit Prüfziffer), optional BIC und Bank. Genau eine ist Standard: die
   erste wird es von selbst, eine neue Standard-Verbindung nimmt der alten
   das Kennzeichen, und wird die Standard-Verbindung gelöscht, wird die
   älteste übrige Standard. Der Rechnungssteller der Elternabrechnung
   (ADR 0088 Abschnitt 4) verweist später über ihre ID auf eine
   Bankverbindung. Die Gläubiger-ID gehört zum Rechnungssteller und bleibt
   offen wie in ADR 0088.
3. **Träger-Transaktion.** Die Tabellen tragen `organization_id`, kein
   `tenant_id`, und die Policy `organization_id = app.current_org_id` mit
   `FORCE ROW LEVEL SECURITY` (ADR 0105 Punkt 2). Alle Routen laufen über
   `TraegerTxMiddleware`; kein Owner-Befehl öffnet die Transaktion selbst.
   Das Postgres-Paket prüft vor jeder Anweisung, dass es in der
   Träger-Transaktion genau dieses Trägers läuft, und nennt den Träger in
   jedem Filter noch einmal. Jede Änderung sperrt zuerst die Profilzeile des
   Trägers; so laufen Änderungen eines Trägers nacheinander, auch die erste.
   Jede Ablehnung fällt vor dem ersten Schreiben, weil die Route 4xx-Antworten
   committet.
4. **Logo.** Nur PNG und JPEG, höchstens 1 MiB und 2000 Pixel je Seite, als
   `bytea` in der Profilzeile. Die Route liest den Upload vor der
   Träger-Transaktion mit Größengrenze und prüft die Dateiendung; der Owner
   prüft Signatur, Kopf mit Maßen und dekodiert das ganze Bild, bevor er
   schreibt (`.claude/rules/security/project-security.md`). SVG ist
   ausgeschlossen, weil es Skript tragen kann. Ausgeliefert wird es mit dem
   gespeicherten Typ, `X-Content-Type-Options: nosniff` und ohne Cache.
5. **Rechte.** Lesen mit „Übersicht lesen“, Ändern mit „Einstellungen“
   (ADR 0084 Abschnitt 4). Die Root bindet die Rechte.
6. **Protokoll.** `platform.traeger_master_data_audit` hält jedes Lesen der
   Stammdaten (Ansprechpartner und Bankverbindungen sind personenbezogen)
   und jede Änderung mit altem und neuem Wert, Konto und Zeitpunkt, in
   derselben Transaktion. Scheitert der Eintrag, scheitert die Anfrage. Das
   Protokoll ist für den Träger append-only; jeder neue Eintrag löscht die
   Einträge des Trägers, die älter als drei Jahre sind (wie ADR 0107
   Punkt 6). `audit.data_access_log` gilt für Daten einer Schule
   (ADR 0084 Abschnitt 7) und bleibt unberührt. Das Logo ohne Person liest
   jeder mit „Übersicht lesen“ ohne Eintrag.
7. **Register.**
   - `policy.json`: Paket `modules/organizationtenancy/inbound/traeger`
     (Rolle `http`) und die drei `data_objects` mit `write_owner`
     `organization-tenancy`. Keine neue Regel.
   - Transaktionsregister unverändert (ADR 0105 Folgen).
   - `moduleComposeBudgets`: `modules/organizationtenancy/compose` 270 → 283
     (der Konstruktor `NewTraegerMasterData`), `moduleComposeBudgetTotal`
     12581 → 12594.
   - `modulePassthroughOwnerBudgets`: `modules/organizationtenancy` 11 → 12
     (`TraegerMasterData.LogoImage`, mit Evidenz in `passthrough.json`),
     Summe 1404 → 1405.
   - `command-effects.json`: eine geprüfte Bindung
     (`MasterDataResource.TraegerRouter`, query).
   - Query-Budget `api.traeger.master_data` (3, exact).

## Verworfene Optionen

- **Bankverbindungen bei der Elternabrechnung:** Der Owner hat noch keinen
  Code. Ein neuer Owner nur für eine Liste von Konten wäre ein leerer Owner
  vor seiner Aufgabe; der Rechnungssteller kann später auf die Konten
  verweisen.
- **Ansprechpartner als Einstellung:** Einstellungen haben kein
  Leseprotokoll, und der Ansprechpartner ist eine Person.
- **Logo in File Storage:** File Storage ist je Schule gebaut (Ordner, Quote,
  Tenant-Transaktion). Ein Träger-Logo bräuchte dort eine zweite
  Transaktionsart.
- **Logo als URL wie bei den Schulen:** Das Träger-Büro hat keinen Ort, an
  dem es Dateien veröffentlicht, und eine fremde URL ließe sich nicht prüfen.

## Folgen

- Dokumente des Trägers (#4669) lesen Logo, Ansprechpartner und
  Bankverbindungen über `organizationtenancy.TraegerMasterData` in der
  Träger-Transaktion und die Farbe und Texte über die Einstellungen.
- Eine Schule liest die Stammdaten ihres Trägers nicht. Braucht ein Dokument
  der Schule sie, kommt eine lesende Policy wie bei den Träger-Werten mit
  eigener Entscheidung.
- Das Protokoll ist gespeichert; eine Anzeige im Träger-Portal folgt mit der
  Änderungsliste der Einstellungen.
