---
status: accepted
---

# Betreuungsvertrag und Elternabrechnung sind eigene Owner beim Träger

Entscheidung vom 23.10.2026 zu [#4647](https://github.com/moto-nrw/project-phoenix/issues/4647),
Etappe 0 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 46 bis 90. Sie baut auf
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
und [ADR 0086](0086-traeger-werte-liegen-zwischen-standard-und-schule.md) auf.
Der Product Owner hat sie am 23.10.2026 angenommen
([ADR 0081](0081-owner-granularity-follows-invariants-not-source-size.md)).

Es gibt zwei neue Domain-Owner: **Betreuungsvertrag** (`care-contract`) und
**Elternabrechnung** (`parent-billing`). Beide speichern ihre Daten beim Träger,
in Tabellen mit RLS über die Träger-Transaktion. Der Betreuungsvertrag ist die
einzige Grundlage für Elternbeiträge, auch für Ferien. Beide Owner stehen im
Owner-Graph über Enrollment, Care Plan, Student Presence und Meal Plan. Kein
unterer Owner hängt von ihnen ab. Schreiben über mehrere Owner läuft über
Workflows. Ausgestellte Beitragsdokumente ändern sich nie.

## Kontext

- Ein Vertrag mit den Eltern fehlt. Care Plan führt die Buchungen
  (`enrollment.care_offering_bookings`) und hat an jedem Angebot `price_cents`,
  das nur angezeigt und nie summiert wird. Die Kategorie `betreuungsvertrag` in
  den Kinderdokumenten ist ein hochgeladenes PDF ohne Status.
- Enrollment entscheidet über Anmeldungen in der Tenant-Transaktion der Schule
  (`Decide`). Die Ausnahme E1 in
  [ADR 0067](0067-principle-owners-meet-through-contracts-and-write-together-in-workflows.md)
  verbietet, `Decide` auf weitere Owner auszudehnen.
- Von Enrollment hängt heute nur der Workflow `care-exit` ab. Care Plan hängt
  weder von Enrollment noch von Student Presence ab.
- „billing“ meint im Code die Abrechnung von moto mit den Schulen
  (Organisation & Tenancy). „Abrechnung“ meint im OGS-Portal die
  Lohnabrechnung. Deshalb heißt der neue Owner Elternabrechnung.
- IBAN und `is_payer` liegen bei People Directory je Schule. SEPA, Mandate und
  Gläubiger-ID gibt es noch nicht.
- Delivery versendet keine Anhänge. Export Transfer nimmt fertige Bytes an.
  Document Rendering hat keine Tabellen und keine Vorlagen.
- Es gibt keinen dauerhaften Kanal für Ereignisse zwischen Owners.
- Wird ein Kind gelöscht, löschen sich heute seine Anmeldung und seine
  Buchungen mit. Rechnungen sind 8 Jahre aufzubewahren (§ 147 AO,
  [Recherche](../research/aufbewahrungsfristen-2026-10-12.md)).
- Geschwister an verschiedenen Schulen, eine Rechnung je Familie über Schulen
  und ein Ferienlauf über alle Standorte brauchen Daten über Schulgrenzen.

## Entscheidung

### 1. Zwei neue Owner beim Träger

- `care-contract` und `parent-billing` sind Domain-Owner. Ihre Tabellen tragen
  `organization_id` und haben RLS nach ADR 0086 §5 (Träger-Transaktion).
- Vertragspartner ist der Träger oder die Stelle, die die Vertragsvorlage
  nennt. Die Schule ist nie Vertragspartner.
- Verträge und Elternabrechnung gibt es nur im Träger-Portal. Ein Träger mit
  einer einzigen OGS, der sie nutzt, bekommt das Träger-Portal freigeschaltet.
- Der Vertrag gehört weder zu Enrollment noch zu Care Plan (siehe Verworfene
  Optionen).

### 2. Betreuungsvertrag

**Objekte**

- **Vertragsvorlage** je Träger und Angebot: Laufzeit, Kündigungsfrist,
  Sonderkündigung, Satzungsbestätigung, Vertragspartner. Standards für Fristen
  sind Träger-Werte (ADR 0086 §4).
- **Betreuungsvertrag** mit genau einem **Betreuungsort** (bei einem Feriengast
  die ausrichtende OGS) und einem optionalen Verweis auf das Kind in einer
  Schule des Trägers.
- **Vertragsversion** mit Wirkdatum. Sie hält eine eingefrorene Kopie von Name
  und Geburtsdatum des Kindes, die gebuchten Leistungen und Zeiten und den
  Preis, der beim Anlegen der Version galt. Spätere Tarifänderungen ändern
  keine bestehende Version (Story 63).
- **Vertragspartei:** Name, Mail und optional ein Elternkonto.
- **Unterschrift** und **Beendigung** (Kündigung, Widerruf).

**Status**

| Objekt | Status |
|---|---|
| Vertrag | `entwurf` → `versendet` → `unterschrieben` → `aktiv` (ab Wirkdatum) → `beendet` |
| Endgrund | `gekündigt`, `widerrufen`, `ausgelaufen`, `aufgehoben` |
| Version | `beantragt`, `wirksam`, `abgelöst`, `abgelehnt` |

„Geändert“ ist kein Status. Die Oberfläche zeigt es, wenn es mehr als eine
Version gibt.

**Tabellen** (Schema `care_contract`): `contract_templates`, `contracts`,
`contract_versions`, `contract_parties`, `signatures` (nur anfügen),
`terminations`.

### 3. Unterschrift als austauschbare Schnittstelle

- `care-contract` hat einen Port `Unterschrift` mit drei Operationen:
  **anfordern** (für eine eingefrorene Version), **bestätigen** und **prüfen**.
- Jede Unterschrift speichert Verfahren, Identitätsnachweis (Elternkonto oder
  Hash des einmaligen Mail-Links), Zeitstempel, die Prüfsumme der
  strukturierten Version und SHA-256 des erzeugten PDFs.
- Verfahren in v1:
  - `einfach_elektronisch`: der Normalfall, nach dem Muster der
    Eltern-Erklärungen in Communication.
  - `mit_anmeldung`: Weg b. Die Anmeldung zeigt vor dem Absenden die
    Vertragsbedingungen. Das verbindliche Absenden ist der Nachweis, mit
    Prüfsumme des gezeigten Texts.
  - `papier`: Import von Bestandsverträgen (Story 58), mit Vermerk, wer
    importiert hat.
- Eine fortgeschrittene oder qualifizierte Signatur kommt als weiteres
  Verfahren dazu. Der Vertragsablauf ändert sich dadurch nicht.

### 4. Elternabrechnung

**Objekte**

- **Rechnungssteller:** Träger, Gemeinde oder Schulträger, mit Anschrift,
  Steuernummer, Bankverbindung und Gläubiger-ID. Ein Träger kann mehrere haben.
- **Tarif:** Einheit (Monat, Woche, Tag, Stunde, Modul), Preis, Gültigkeit,
  Rechnungssteller, Steuersatz (Standard 0 %) und Hinweis zur Befreiung.
- **Beitragsregel:** feste Regelarten mit Parametern, Gültigkeit und
  Geltungsbereich (Angebote, Schulen). Die Reihenfolge ist fest: Grundpreis,
  Einkommensstufe, Geschwister, Ermäßigung, Befreiung, Höchstbetrag,
  Kostenträger, Korrektur. Es gibt keine Formelsprache. Eine neue Regelart ist
  Code.
- **Zahler:** Person oder Stelle, mit Anschrift und Bankverbindung. Ein
  **Kostenträger** ist ein Zahler vom Typ Stelle. Seine Zuordnung zum Kind hat
  Anteil, Nachweis und Ablaufdatum.
- **Familie:** bestätigte Gruppe aus Kindern mit Vertrag und ihren Zahlern.
  moto schlägt sie aus der Anmeldung vor (Geschwister im selben Formular,
  gleiche Mail-Adresse), die Abrechnung bestätigt sie. Geschwister sind Kinder
  derselben Familie mit wirksamem Vertrag.
- **Mandat:** gehört zu einem Zahler und gilt für genau einen
  Rechnungssteller. Es hat Mandatsreferenz, Datum und Widerruf.
- **Nummernkreis** (Muster und Zähler) je Rechnungssteller und Leistung.
- **Bündelungsregel:** welche Leistungen auf ein Beitragsdokument kommen.
- **Abrechnungslauf** für einen Monat oder einen Ferienzeitraum, im Voraus oder
  nachträglich. Status `berechnet` → `freigegeben` → `abgeschlossen`.
- **Beitragsposition:** Grundpreis oder Wirkung einer Regel, mit Verweis auf
  die Vertragsversion. Eine **Nachberechnung** korrigiert einen schon
  abgerechneten Zeitraum um die Differenz und verweist auf das frühere
  Dokument.
- **Beitragsdokument** mit der Dokumentart Rechnung, Gebührenbescheid oder
  Bestätigungsschreiben. **Storno** und **Gutschrift** sind eigene
  Beitragsdokumente mit Verweis auf das Original. Beim Bescheid heißen sie in
  der Oberfläche Aufhebungs- und Änderungsbescheid.

**Regeln**

- Die einzige Quelle für Beiträge ist eine Vertragsversion. Jede bestätigte
  Ferienbuchung schließt deshalb einen Vertrag (Weg b).
- Jede Regel rundet ihr Ergebnis kaufmännisch auf ganze Cent und schreibt eine
  eigene Beitragsposition. Ein Beitrag ist die Summe seiner Positionen.
- Jede Position speichert Brutto, Netto und Steuer.
- Der Anteil eines Kostenträgers wird im Dokument der Familie abgezogen und
  steht als eigene Position in einem Dokument an die Stelle, aus demselben Lauf.
- Bis zur Freigabe darf der Lauf neu berechnet werden. Korrekturen sind
  Positionen mit Pflichtbegründung.
- Mit der Freigabe wird jedes Dokument **ausgestellt**: In derselben
  Transaktion bekommt es seine Nummer, lückenlos je Nummernkreis. Entwürfe
  haben keine Nummer.
- Ein ausgestelltes Dokument und sein PDF ändern sich nie. Ein Storno macht nur
  die Abrechnung von Hand.
- Die Finanzformate (PAIN.008, DATEV-Buchungsstapel, Sollstellung mit Zuordnung
  je Kunde, CIP und SIMBA als Adapter) erzeugt `parent-billing`. Jeder Export
  wird mit Lauf und Zeitpunkt festgehalten und lässt sich wiederholen
  (Story 90).

**Tabellen** (Schema `parent_billing`): `issuers`, `tariffs`,
`contribution_rules`, `payers`, `families`, `family_members`, `family_payers`,
`cost_bearer_assignments`, `mandates`, `number_ranges`, `bundling_rules`,
`runs`, `run_quantities`, `documents`, `positions`, `exports`.

### 5. Werte

- Geld ist `int64` in Cent, nur EUR. Gleitkommazahlen gibt es für Geld nicht.
- Steuersätze und Anteile sind Ganzzahlen in Basispunkten.
- Kalenderdaten (Wirkdatum, Leistungszeitraum, Fälligkeit, Ablauf eines
  Nachweises) sind `calendar.Date`. Zeitpunkte (Unterschrift, Freigabe,
  Ausstellen, Export) sind Zeitstempel.

### 6. Schreiben über mehrere Owner

- **`contractconclusion`** (Workflow): ruft `Decide` von Enrollment unverändert
  auf. Nach dem Commit schließt er den Vertrag in der Träger-Transaktion, bei
  Weg a als `entwurf`, bei Weg b als `aktiv` mit Wirkdatum der Annahme. Welcher
  Weg gilt, ist ein Träger-Wert (Story 47). Ein Abgleich im Scheduler legt
  fehlende Verträge idempotent nach. Schlüssel ist die Anmeldung des Kindes. E1
  bleibt unverändert.
- **Änderung:** Care Plan bleibt führend für das, was gebucht ist. Eine
  freigegebene Elternanfrage erzeugt über denselben Workflow und denselben
  Abgleich eine neue Vertragsversion, Wirkdatum ist `valid_from` der Buchung.
  `care-contract` schreibt nie in Care Plan.
- **Ende:** `care-exit` bekommt einen zweiten Einstieg „Vertragsende“. Eine
  bestätigte Kündigung oder ein Widerruf startet den Austritt zum Ende des
  Vertrags. Endet die Betreuung in Care Plan ohne Kündigung, bleibt der Vertrag
  bestehen. Der Abgleich zeigt dem Träger-Büro „Austritt ohne Kündigung“ an.
- **`billingrun`** (Workflow): liest je Betreuungsort in der
  Tenant-Transaktion dieser Schule über die Verträge von Student Presence und
  Meal Plan. Danach schreibt er die Mengen in der Träger-Transaktion in den
  Lauf. Es gibt keine Admin-Transaktion.
- **Mensa-Monatsfreigabe** (Story 65): ein Objekt von Meal Plan im Tenant der
  Schule. Der Lauf rechnet die Mensa einer Schule nur für freigegebene Monate
  spitz ab und zeigt sonst „Freigabe fehlt“.

### 7. Lesen und Zugriff

- **Schule:** Für Tabellen von `care-contract` liest die Tenant-Policy nur
  Zeilen, deren Betreuungsort die Schule ist. Damit ist ADR 0086 §5 für diesen
  Owner enger. Die Schule sieht Vertrag, Status, gebuchte Zeiten und den
  Soll-Ist-Vergleich (Story 56). Tabellen von `parent-billing` haben keine
  Tenant-Policy. Rechnungen, Zahler, Familie, Bankdaten und Mandate sieht die
  Schule nie.
- **Soll-Ist:** Story 56 liest `care-contract` in der Tenant-Transaktion über
  den Vertrag von Student Presence. Story 57 (alle Schulen) ist eine eigene
  Fläche in `traeger-view` nach ADR 0084 §1.
- **Eltern:** Ein Elternkonto sieht einen Vertrag nur als eingetragene
  Vertragspartei und ein Beitragsdokument nur als Konto des Zahlers.
  Mitgliedschaft in einer Schule und Verbindung zum Kind reichen nicht. Ohne
  Elternkonto gibt es nur den Mail-Link. Der Workflow `parent-portal` nutzt die
  Verträge beider Owner.
- **Audit:** Jeder Zugriff des Träger-Portals auf personenbezogene Daten schreibt
  nach ADR 0084 §7 einen Eintrag in `audit.data_access_log` im Tenant des
  Betreuungsorts. Familien- und Zahlerdaten protokolliert der Owner beim
  Betreuungsort jedes betroffenen Vertrags.

### 8. Dokumente und Versand

- Document Rendering erzeugt die PDFs. Der Owner des Dokuments speichert sie
  über File Storage: `care-contract` den Vertrag, `parent-billing` das
  Beitragsdokument.
- Delivery bekommt keine Anhänge. Eine Mail enthält einen einmaligen,
  befristeten Link oder den Hinweis auf das Postfach in der Eltern-App (#4671).
  Familien ohne Internet bekommen Papier (Story 100).
- Mail und Push gehen in die Outbox, in derselben Transaktion wie das
  auslösende Ereignis.

### 9. Aufbewahrung

- Verträge und Beitragsdokumente sind eigene Datenarten
  ([ADR 0073](0073-inhalte-der-schule-folgen-der-frist-ihrer-datenart.md)). Die
  Fristen legt #4720 fest.
- Kein Vertrag und kein Beitragsdokument wird mit dem Kind gelöscht. Beide
  halten ihre eigene Kopie von Kind, Empfänger und Positionen.
- Die Pflicht zur Aufbewahrung liegt beim Träger. Vor dem Löschen eines Trägers
  (Story 11) bekommt er einen vollständigen Export aller Dokumente. Danach
  löscht moto alles.

### 10. Owner-Graph

```
care-contract  → care-plan, student-presence, people-directory,
                 organization-tenancy, settings-platform, document-rendering,
                 file-storage, delivery, identity-access, Runtimes
parent-billing → care-contract, organization-tenancy, settings-platform,
                 document-rendering, file-storage, delivery, identity-access,
                 Runtimes
Workflows:       contractconclusion → enrollment, care-plan, care-contract
                 billingrun         → parent-billing, care-contract,
                                      student-presence, meal-plan
                 care-exit          + care-contract
                 parent-portal      + care-contract, parent-billing
Projektion:      traeger-view       + care-contract, student-presence
```

- Domain-Kanten gehen nur auf `public`- und `contract`-Rollen.
- Export Transfer bekommt die Bytes über einen Port, den die Root bindet, wie
  bei Workforce
  ([ADR 0074](0074-delivery-and-export-transfer-speak-their-own-contract.md)).
- Kein unterer Owner hängt von `care-contract` oder `parent-billing` ab. Der
  Graph bleibt azyklisch
  ([ADR 0058](0058-owner-dependencies-form-a-dag.md),
  [ADR 0071](0071-principle-owner-dependencies-point-one-way.md)).

## Verworfene Optionen

- **Vertrag als Teil von Enrollment:** Der Vertrag lebt Jahre über die
  Anmeldung hinaus und liegt beim Träger, Enrollment im Tenant der Schule.
  `Decide` dürfte nach E1 nicht wachsen.
- **Vertrag als Teil von Care Plan:** Care Plan hat 14 abhängige Owner. Version
  mit Wirkdatum, Unterschrift und Beendigung sind eigene Invarianten (ADR 0081).
- **Verträge und Rechnungen im Tenant der Schule:** Geschwister über Schulen,
  eine Rechnung je Familie und ein Ferienlauf über alle Standorte bräuchten
  Lesen über Schulgrenzen, das ADR 0084 nur in `traeger-view` erlaubt.
- **Zwei Quellen für Beiträge** (Verträge und Buchungen ohne Vertrag): Jede
  Regel bräuchte zwei Eingänge, und der festgehaltene Preis läge an zwei
  Stellen.
- **Allgemeiner Kanal für Ereignisse mit Outbox:** ein neuer Mechanismus für
  alle Owner. Workflow und Abgleich reichen für diesen Fall.
- **Annahme und Vertrag in einer Transaktion** mit Tenant und Träger zugleich:
  zwei RLS-Kontexte in einer Transaktion wären ein neuer Weg an der Trennung
  vorbei.
- **Freie Formelsprache für Beitragsregeln:** schwer zu prüfen und zu erklären.
  Die Satzungen der Kunden passen in feste Regelarten mit Parametern.
- **PDFs als Mail-Anhang:** Kinder- und Geldangaben lägen unverschlüsselt in
  Postfächern.

## Folgen

- Kein Code der beiden Owner, bevor sie in `policy.json`, in der Liste der
  Domain-Owner (`backend/internal/architecture/policy.go`), in ADR 0066 §1, in
  `backend/architecture/README.md` und im Glossar unter „Backend-Modul“ stehen.
- Etappe 1 (Ferien) baut schon den Kern von `care-contract`: Vertrag, Version,
  Status `aktiv` über Weg b, ohne eigene Unterschrift und ohne Änderungen.
- `care-exit` und `parent-portal` bekommen neue Kanten. `meal-plan` bekommt die
  Mensa-Monatsfreigabe.
- Die bestehenden IBAN-Daten bei People Directory dienen nur als Vorschlag beim
  Anlegen eines Zahlers. Die Elternabrechnung liest sie nicht laufend.
- Offen bleiben: Vorlagen mit Layout und Platzhaltern (T6), das Modell des
  Feriengasts (ADR 0084), die Fristen (#4720) und die Rechtsfragen der Spec
  (Form der Unterschrift, wer die Gläubiger-ID hält).
