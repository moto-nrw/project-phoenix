---
status: proposed
---

# Zahlungsweg, Fälligkeit und Empfänger entscheidet eine Regel, und die Rechnung hält sie fest

**Status: Vorgeschlagen.**

Vorschlag vom 08.11.2026 zu [#4689](https://github.com/moto-nrw/project-phoenix/issues/4689),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611)
(User Stories 74, 76, 77). Er folgt
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md)
(ausgestellte Rechnungen ändern sich nie),
[ADR 0158](0158-monatsbeitrag-entsteht-aus-der-wirksamen-vertragsfassung-und-den-tarifen-des-traegers.md)
und [ADR 0164](0164-ermaessigungen-des-monatsbeitrags-sind-beitragsregeln-des-traegers-mit-nachweis-je-vertrag.md)
(Familie = Mail-Adresse). Er führt keinen Owner ein; die vier neuen Tabellen
gehören der Elternabrechnung (`backend/architecture/policy/parent-billing.json`).

## Kontext

Der SJR Pforzheim (Start 01.01.2027) zieht die Schulkindbetreuung per
Lastschrift ein und lässt die Ferienprogramme überweisen. Ein Monatslauf hat
ein Fälligkeitsdatum je Zahlungsweg (Punkt 5: Überweisung 02.11., SEPA 04.11.).
Einzelne Familien zahlen anders oder lassen die Rechnung an einen anderen
Zahler gehen; getrennte Eltern oder verstorbene Bezugspersonen dürfen nicht
automatisch Empfänger werden (Transkript Z. 49–51). Bisher schrieb jedes
Schreiben ein einziges „Zahlen bis“ des Laufs, und Empfänger war immer die
Vertragspartei.

## Entscheidung

1. **Bedingungen je Leistung und Träger** (`parent_billing.service_terms`):
   Zahlungsweg (`transfer`, `direct_debit`), Zahlungsziel in Tagen und die
   Leistung, auf deren Rechnung die eigenen Positionen stehen (Bündelung).
   Ohne Eintrag gilt: Überweisung, 14 Tage, Mensa auf der Rechnung der
   Betreuung. Welche Leistung mit welcher reisen darf, steht in einer Liste
   (`parentbilling.BillingServices`), nicht in Verzweigungen. Die
   Ferienbetreuung hat einen eigenen Lauf und Nummernkreis und reist nie mit.
2. **Fälligkeit je Lauf und Zahlungsweg** (`parent_billing.run_due_dates`) für
   Monats- und Ferienläufe. Nach der Freigabe lehnt die Datenbank jede
   Änderung ab (Trigger `refuse_released_due_date`).
3. **Abweichung der Familie** (`parent_billing.family_billing`): Empfänger
   (Anmelder, beide Eltern, abweichender Zahler mit Anschrift), eigener
   Zahlungsweg, eigener Fälligkeitstag im Monat (1–28). Schlüssel ist die
   Mail-Adresse der Anmeldung (Vertragspartei bzw. Ferien-Anmelder), nicht die
   des Zahlers. Die Zeile hängt am Vertrag, aus dem sie gesetzt wurde, und geht
   mit ihm. Jeder Lese- und Schreibzugriff steht ohne Namen im Trail
   (`family_billing_events`), der ebenfalls am Vertrag hängt.
4. **Eine Regel:** `parentbilling.ResolvePayment` (Zahlungsweg der Familie,
   sonst der Leistung; Fälligkeit der Familie, sonst des Laufs für diesen
   Zahlungsweg, sonst Rechnungsdatum + Zahlungsziel) und
   `parentbilling.ResolveRecipient`. Vorschau, Freigabe, Schreiben und Export
   rufen nur diese Funktionen. Die Prüfphase (#5916) setzt die Daten des Laufs
   über `BillingTerms.SetCareRunDueDates`. Die Mensa (#4686) liest später
   `BundleWith` ihrer Bedingungen (`TermsSet.Of(ServiceMeal)`); bis dahin
   greift die Bündelung für die Mensa noch nicht.
5. **Snapshot:** Die Freigabe kopiert Familie, Empfängerart, zweiten Elternteil,
   Zahler-Anschrift, Zahlungsweg, Fälligkeit und Herkunft der Fälligkeit auf
   die Rechnung. Rechnungen vor dieser Änderung behalten NULL und ihr
   Schreiben nimmt weiter das Datum der Schreiben des Laufs.
6. **Familie bleibt Familie:** Die Geschwisterregel zählt nach der kopierten
   Familie, nicht nach dem Zahler; die Familienansicht (#4691) gruppiert danach.

## Folgen

- Eine Änderung wirkt ab der nächsten Freigabe; ausgestellte Rechnungen bleiben.
- Eine Familie ohne Mail-Adresse in der Anmeldung kann keine Abweichung haben
  (`parent_billing.family_unknown`). Ein echter Familienbegriff mit eigener ID
  ersetzt den Schlüssel später an einer Stelle (`parentbilling.FamilyKey`).
- Eine Familie nur mit Ferienanmeldung (ohne Vertrag) bekommt ihre
  Abweichung erst mit einem Vertrag.
- Die Bündelungsregel der Mensa ist gespeichert, wirkt aber erst, wenn #4686
  (bzw. der zweite der beiden PRs) die Mensa-Positionen nach `BundleWith`
  verteilt; eine eigene Mensa-Rechnung baut #4686.

## Register

- Pass-through-Budget `modules/parentbilling` 7 → 8, Summe 1406 → 1407:
  `BillingTerms.ServiceTerms` prüft den Träger und ergänzt die gespeicherten
  Bedingungen um die Standardwerte jeder Leistung (Evidence und Review mit
  Entscheidung 0167).
- Compose-LOC `modules/parentbilling/compose` 62 → 67, Summe 13088 → 13093:
  der Konstruktor `NewBillingTerms`.
- Query-Budgets `api.traeger.billing_terms` und `api.traeger.family_billing`
  (je 2, exakt). Der Lauf liest Bedingungen, Abweichungen und Fälligkeiten in
  einer Abfrage (`InvoicePaymentTerms`).

## Verworfene Optionen

- **Eine Rechnung je Familie statt je Vertrag:** bräche Nummern, Storno,
  Postfach (je Kind) und die Eindeutigkeit je Vertrag und Monat; eigener
  Schritt, wenn ein Kunde es verlangt.
- **Fälligkeit je Familie als festes Datum:** gilt nur einen Monat; ein Tag im
  Monat gilt für jeden Lauf.
- **Empfänger aus allen Bezugspersonen:** genau das schließt der SJR aus.
