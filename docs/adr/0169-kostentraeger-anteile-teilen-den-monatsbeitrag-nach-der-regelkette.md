---
status: proposed
---

# Kostenträger-Anteile teilen den Monatsbeitrag nach der Regelkette

**Status: Vorgeschlagen.**

Entscheidung zu [#4687](https://github.com/moto-nrw/project-phoenix/issues/4687),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 66 bis 69. Sie ist ein Bau-Schnitt des Owners Elternabrechnung
aus [ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
§4 und baut auf den Kostenträgern der Ferien
([ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md),
Nachtrag #5627), der Regelkette des Monatsbeitrags
([ADR 0164](0164-ermaessigungen-des-monatsbeitrags-sind-beitragsregeln-des-traegers-mit-nachweis-je-vertrag.md))
und der Prüfphase (#5916) auf. Sie führt keinen Owner ein (ADR 0057 Punkt 5,
ADR 0070).

## Entscheidung

1. **Ein Kostenträger, eine Liste.** Ein Anteil nennt einen Kostenträger aus
   `parent_billing.cost_bearers` (#5627), den die Ferienabrechnung schon
   kennt. Es gibt keine zweite Liste von Stellen. Wie Zeile und Position aus
   #5627 nennt der Anteil seinen Kostenträger ohne Fremdschlüssel: Die
   Löschung eines Trägers entfernt beide in einer Kaskade, und das Down von
   1.15.745 bleibt möglich. Das Speichern hält den Kostenträger des eigenen
   Trägers unter seiner Zeilensperre (`LockCostBearer`, geteilt), die das
   Löschen exklusiv nimmt; ein Kostenträger, den ein Anteil oder eine
   ausgestellte Rechnung nennt, lässt sich nicht löschen, nur ausschalten. Ein ausgeschalteter
   Kostenträger bleibt am Anteil, der ihn nennt, steht aber für keinen neuen
   Anteil zur Wahl.
2. **Anteil je Vertrag.** `parent_billing.care_cost_shares` hält je Vertrag
   die Art (`education_participation`, `youth_office`, `other_subsidy`), den
   Kostenträger, entweder einen Anteil in Basispunkten oder einen festen
   Betrag im Monat, die Gültigkeit mit Ende, das Prüfdatum und einen Verweis
   auf den Bescheid (Aktenzeichen, höchstens 120 Zeichen). Ein Bescheid über
   Leistungen nach SGB II/XII, BuT oder § 90 SGB VIII hat immer ein Ende,
   deshalb ist `valid_until` Pflicht. Kein Dokument, kein Einkommen, nichts
   aus dem Bescheid (Zweckbindung, Datenminimierung). Anteile einer Art
   überschneiden sich je Vertrag nie (Ausschlussbedingung). Ein Anteil gilt
   für den Monat wie ein Nachweis (ADR 0164 Punkt 6): der am 1. gültige,
   sonst der erste, der im Monat beginnt.
3. **Reihenfolge: nach dem Höchstbetrag.** Die Kette ist jetzt
   Einkommensstufe → Geschwister → Alleinerziehende → Höchstbetrag →
   Kostenträger-Anteile. Der Anteil ist keine Ermäßigung, sondern teilt den
   Beitrag zwischen Zahlern (ADR 0088 §4 führt den Kostenträger als Zahler).
   Die Stelle übernimmt, was die Familie nach der Satzung schuldet; der
   Bescheid des Jugendamts nach § 90 Abs. 4 SGB VIII bemisst sich am
   festgesetzten Beitrag, nicht am Grundpreis. Lägen die Anteile vor den
   Ermäßigungen, bekäme eine Familie mit voller Übernahme noch eine
   Geschwisterermäßigung auf 0 € oder der Höchstbetrag zöge vom Anteil der
   Stelle ab. Die Ferienkette hat für die Kostenübernahme dieselbe
   Voreinstellung (`after_discount`, ADR 0115 Nachtrag #5664).
4. **Rechnen.** Ein Anteil in Basispunkten ist ein Teil des ganzen Beitrags
   nach den Regeln, kaufmännisch auf ganze Cent gerundet; ein fester Betrag
   ist der Betrag. Die Arten rechnen in der Reihenfolge Bildung und Teilhabe,
   Jugendamt, sonstiger Zuschuss, und jede nimmt höchstens den Rest: Die
   Anteile ergeben nie mehr als den Beitrag, der Rest der Familie ist nie
   negativ, und Beitrag gleich Anteile plus Rest auf den Cent. Der
   handgerechnete Tabellentest `TestCareCostSharesFollowTheRuleChain` deckt
   jede Regel der Kette mit Anteil, feste Beträge über dem Beitrag, drei
   Arten zugleich und die Cent-Aufteilung ab.
5. **Getrennt von den Positionen.** Die Anteile ändern die `payable_cents`
   der Positionen nicht; nur `total_cents` der Rechnung ist der Rest der
   Familie. So rechnet ein späterer Lauf des Monats Höchstbetrag und festen
   Geschwisterpreis weiter gegen den ganzen Beitrag der ausgestellten
   Rechnung, nicht gegen den Rest nach einem Anteil (Test „an issued share
   still counts toward the cap“).
6. **Beitragsfrei mit Vermerk.** Übernehmen Kostenträger den ganzen
   Beitrag, stellt der Lauf der Familie eine Rechnung über 0 € aus. Der Brief
   zeigt Monatsbeitrag, jeden Anteil mit Kostenträger und „Ihr Anteil“; statt
   der Zahlungsaufforderung steht „Beitragsfrei: X übernimmt Ihren Beitrag.“
   Eine Rechnung über 0 € statt keiner Rechnung hält die Familie informiert,
   lässt Nummernkreis, Storno (ADR 0165) und Neuausstellung unverändert und
   entspricht der Ferienabrechnung (ADR 0115 Nachtrag #5627 Punkt 4).
7. **Getrennte Abrechnung als Liste.** Je Lauf und Kostenträger gibt es eine
   Liste (PDF, Excel, CSV) mit Rechnungsnummer, Kind, Schule, Art,
   Monatsbeitrag, Anteil und übernommenem Betrag. Vor der Freigabe ist sie
   eine Vorschau ohne Nummern; nach der Freigabe stehen stornierte Rechnungen
   darin, zählen aber nicht zur Summe. Die Liste nennt kein Aktenzeichen des
   Bescheids: Der Kostenträger kennt seinen eigenen Bescheid. Jeder Download
   steht mit den genannten Verträgen in `care_run_events`. Ein eigenes
   Beitragsdokument mit Nummer an den Kostenträger folgt erst, wenn ein
   Träger es braucht (siehe Verworfene Optionen).
8. **Prüfphase.** Der Fingerabdruck der Prüfphase (`previewFingerprint`)
   umfasst jede Rechnung mit ihren Anteilen. Ein Anteil, der nach der
   Prüfung dazukommt, sich ändert oder wegfällt, lässt die Freigabe mit 409
   abbrechen, auch wenn er nur eine ausgeschlossene Rechnung betrifft
   (Test `TestCareReviewReleaseRefusesACostShareAddedSinceTheReview`).
9. **Festhalten.** Eine ausgestellte Rechnung speichert ihre Anteile in
   `parent_billing.care_invoice_cost_shares` mit einer Kopie von Art,
   Anteil und Namen des Kostenträgers; der Trigger aus ADR 0119 Punkt 7
   lehnt jede Änderung ab. Eine spätere Änderung des Anteils oder des
   Kostenträgers erreicht sie nicht.
10. **Zugriff und Protokoll.** Die Routen `/traeger/care-cost-shares`
    verlangen das Träger-Recht „Abrechnung“ und laufen in der
    Träger-Transaktion. Jedes Lesen der Seite und jede Änderung schreibt
    `parent_billing.care_cost_share_events` je Vertrag mit Konto. Leistungs-
    bezug ist ein sensibles Datum: Nur die Abrechnung sieht es, im Zweck der
    Abrechnung. Die Seite warnt 30 Tage vor dem Ende eines Anteils ohne
    Folgeanteil derselben Art.
11. **Löschung.** Anteile und Spur hängen mit `ON DELETE CASCADE` am
    Vertrag, ausgestellte Anteile an der Rechnung. Die Demo-Löschung (ADR
    0062, 1.15.805) erfasst sie damit ohne Änderung an
    `platform.erase_closed_demo_visitors`; der Kostenträger gehört dem
    Träger und bleibt.

## Verworfene Optionen

- **Anteil vor den Ermäßigungen:** Die Stelle trüge Ermäßigungen mit, die
  die Satzung der Familie gibt; bei voller Übernahme liefen
  Geschwisterermäßigung und Höchstbetrag ins Leere.
- **Anteil als Ermäßigungszeile in `care_invoice_reductions`:** Er würde
  `payable_cents` senken, und ein späterer Lauf rechnete den Höchstbetrag
  gegen einen zu kleinen Beitrag.
- **Keine Rechnung bei voller Übernahme:** Die Familie erführe nicht, dass
  ein Beitrag festgesetzt und übernommen ist; Storno und Neuausstellung
  bräuchten einen Sonderweg.
- **Eigenes Beitragsdokument mit Nummer an den Kostenträger:** Mehr als die
  Erstattung braucht; die Spec verlangt „abgerechnet oder exportiert“.
- **Nachweis als Dokument hochladen:** Mehr Daten als nötig; das Büro prüft
  den Bescheid und hält fest, dass es ihn geprüft hat (wie ADR 0164).
- **Anteil in `care_evidences`:** Ein Nachweis senkt den Beitrag, ein Anteil
  teilt ihn. Getrennte Tabellen halten Ausschlussbedingung, Spur und Liste
  einfach.

## Register

- `policy/parent-billing.json`: drei `data_objects` mit `write_owner`
  `parent-billing`.
- Datenlebenszyklus und Aufbewahrung: je drei Einträge.
- Query-Budget `api.traeger.care_cost_shares` (7, exact).
- Migration `1.15.816`.
