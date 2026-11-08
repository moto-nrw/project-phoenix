---
status: proposed
---

# Dokumentart je Leistung und Gebührenbescheid mit der Rechtsbehelfsbelehrung des Trägers

**Status: Vorgeschlagen.**

Vorschlag vom 08.11.2026 zu [#4688](https://github.com/moto-nrw/project-phoenix/issues/4688),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Story 73, Vorbild Gemeinde Hardt (MK-007). Er baut auf
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md),
[ADR 0152](0152-dokumentvorlagen-des-traegers-liegen-bei-organisation-und-tenancy.md) und
[ADR 0165](0165-monatsrechnungen-nehmen-schreiben-versand-und-storno-der-ferienrechnungen.md)
auf. Er führt keinen Owner ein und hält fest, was er in den Registern ändert
(ADR 0057 Punkt 5, ADR 0070).

## Kontext

Hardt veranlagt die Betreuung per Gebührenbescheid mit Rechtsbehelfsbelehrung,
Ferien und Essen per Rechnung (MK-001, MK-002, MK-007). Bissendorf schreibt ein
Bestätigungsschreiben, das zugleich Rechnung ist (ADR 0121). Entscheidung von
Yannick nach Rücksprache Conrad mit Hardt (06.11.2026): Die
Rechtsbehelfsbelehrung pflegt der Träger selbst als Pflicht-Baustein seiner
Vorlage; moto liefert keinen Rechtstext. Seite 2 von MK-007 wird nicht
nachgebaut. Ohne Belehrung verlängert sich die Frist für den Rechtsbehelf auf
ein Jahr (§ 58 Abs. 2 VwGO), deshalb wird ohne sie kein Bescheid ausgestellt.

## Entscheidung

1. **Dokumentart je Leistung.** `parent_billing.document_kinds` hält je Träger
   und Leistung die Art: Ferien `confirmation` (Bestätigungsschreiben als
   Rechnung), `invoice` oder `fee_notice`; Monatsbeitrag `invoice` oder
   `fee_notice`. Ohne Zeile gilt, was die Leistung vorher ausstellte
   (Ferien Bestätigung, Monat Rechnung). Ein Bestätigungsschreiben gehört zu
   einer Ferienanmeldung; der Monatsbeitrag hat keins. Die Zeile nennt keine
   Person. Lesen und Wählen unter `GET/PUT /traeger/billing-document-kinds`
   hinter dem Recht „Abrechnung“ in der Träger-Transaktion. Mensa (#4686)
   kommt als weitere Leistung dazu, sobald sie Belege ausstellt.
2. **Festgeschrieben bei der Ausstellung.** Die Freigabe schreibt die Art auf
   jedes Dokument (`documents.document_kind`, `care_invoices.document_kind`);
   Storno und Gutschrift übernehmen die Art ihrer Rechnung. Der Trigger gegen
   jede Änderung ausgestellter Dokumente hält sie fest. Eine spätere Wahl
   ändert nur die nächste Freigabe. Die Vorschau zeigt die Art, die die
   Freigabe jetzt festschreiben würde; im Monatslauf gehört sie zum
   Fingerabdruck der Prüfung, im Ferienlauf nennt der Freigabe-Body die
   gesehene Art. Weicht sie ab, lehnt die Freigabe ab (409
   `parent_billing.billing_run_changed`).
3. **Belehrung als Pflicht-Baustein der Vorlage.** Die Vorlage „Bescheid“
   (ADR 0152) bekommt den Teil `legal_remedy` (bis 5000 Zeichen,
   Platzhalter der Art erlaubt, nur bei `bescheid`). Er gehört zur Version
   und steht unter dem Trigger der Versionen. Das Speichern ohne Belehrung ist
   erlaubt, damit das Büro schrittweise schreiben kann. Die Standardvorlage
   hat keine.
4. **Ohne Belehrung kein Bescheid.** Freigabe und Schreiben eines Laufs mit
   Art `fee_notice` lehnen ab, solange die aktuelle Vorlage „Bescheid“ keine
   Belehrung hat (409 `parent_billing.legal_remedy_missing`), vor jeder
   Nummer. Lauf und Einstellungsseite zeigen das vorher
   (`legal_remedy_missing`, `legal_remedy_set`).
5. **Schreiben je Art.** Rechnung und Bescheid füllen die Vorlage ihrer Art
   (ADR 0152), das Bestätigungsschreiben bleibt wie bisher, die Monatsrechnung
   behält ihren festen Text. Der Bescheid nennt „Bescheidnummer“, die
   Debitorennummer als „Buchungszeichen“ und druckt die Belehrung nach dem
   Text der Vorlage unter „Rechtsbehelfsbelehrung“, sodass kein Vorlagentext
   sie weglassen kann (Muster der festen Blöcke des Vertrags, ADR 0153 Punkt
   5). Nummernkreis und Unveränderlichkeit bleiben (ADR 0119 Punkt 5 und 7).
6. **Storno eines Bescheids ist ein Aufhebungsbescheid.** Ein Bescheid ist ein
   Verwaltungsakt; er wird nicht wie eine Rechnung storniert, sondern
   aufgehoben, eine Senkung ist eine teilweise Aufhebung (Änderungsbescheid).
   Datenmodell und Regeln bleiben die von Storno und Gutschrift (ADR 0119
   Nachtrag 2026-11-03, ADR 0165 Punkt 6 und 7): dieselbe Nummernfolge,
   Pflichtgrund, höchstens ein Storno, Neuausstellung im nächsten Lauf. Nur
   Titel, Text und Mail heißen „Aufhebungsbescheid“ bzw. „Änderungsbescheid“
   und nennen „Zum Bescheid“. Die Neuausstellung ist ein neuer Bescheid mit
   Belehrung. Aufhebung und Senkung belasten die Familie nicht; sie drucken
   die Belehrung, wenn die Vorlage eine hat, und werden ohne sie nicht
   zurückgehalten.
7. **Mail.** Die Art steht im Payload (`letter_kind`); Bescheid, Aufhebung und
   Änderung und die Ferienrechnung haben eigene Betreffe und Vorlagen
   (`billing-notice-letter.html`, `billing-notice-correction-letter.html`,
   `billing-invoice-letter.html`). Payloads der Arten von vorher bleiben gleich.

## Verworfene Optionen

- **Fester Rechtstext von moto:** Der Text hängt von Behörde, Satzung und Weg
  des Widerspruchs ab; Entscheidung vom 06.11.2026.
- **Belehrung beim Speichern der Vorlage verlangen:** Die Standardvorlage hätte
  dann keine gültige Form, und das Büro könnte den Bescheid nicht schrittweise
  vorbereiten. Die Sperre gehört an die Ausstellung.
- **Dokumentart als Träger-Einstellung (ADR 0107):** Die Art wird bei der
  Freigabe in derselben Träger-Transaktion gelesen und festgeschrieben und
  gehört zum Recht „Abrechnung“, wie der Nummernkreis (ADR 0119, verworfene
  Optionen).
- **Eigener Nummernkreis für Aufhebungsbescheide:** teilte die lückenlose
  Folge je Leistung (ADR 0119 Nachtrag 2026-11-03).
- **Jahresbescheid mit Monatsraten:** Der Monatslauf stellt je Monat aus; ein
  Bescheid über einen Zeitraum mit Raten zum Monatsersten (MK-007) bräuchte
  einen eigenen Lauf über mehrere Monate. Vorgeschlagen in
  [ADR 0175](0175-jahresbescheid-veranlagt-die-monate-eines-jahres-und-keine-monatsrechnung-rechnet-sie-noch-einmal-ab.md) (#6219).

## Register

- Migration `1.15.818` (vom Orchestrator vergeben; 815 und 816 belegen #6163 und #6190): Tabelle, zwei Spalten
  der Dokumente, `traeger_document_templates.legal_remedy` mit Trigger.
- `policy.json`: ein `data_object` (`parent_billing.document_kinds`) für
  `parent-billing`; keine neue Regel. Der Port
  `parentbilling.BillingDocumentTemplates` bindet die Root an
  `TraegerDocumentTemplates.Current` (command effects: Abfrage).
- Datenlebenszyklus und Aufbewahrung je ein Eintrag (`noPersonalData`); die
  Demo-Löschung braucht keinen, weil die Tabelle keine Person nennt.
- Zwei Routen im Route-Golden, die schreibende in der Nutzungsanalyse als
  `notCaptured` (Träger, keine Schule). Query-Budget
  `api.traeger.billing_document_kinds` (3, exact).
- Fehlercodes `parent_billing.document_kind_invalid`,
  `parent_billing.legal_remedy_missing`.
