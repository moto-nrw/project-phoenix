---
status: proposed
---

# Ermäßigungen des Monatsbeitrags sind Beitragsregeln des Trägers mit Nachweis je Vertrag

**Status: Vorgeschlagen.**

Entscheidung zu [#4685](https://github.com/moto-nrw/project-phoenix/issues/4685),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 60 bis 62. Sie ist ein Bau-Schnitt des Owners Elternabrechnung
aus [ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
§4 und baut auf dem Monatsbeitrag aus
[ADR 0158](0158-monatsbeitrag-entsteht-aus-der-wirksamen-vertragsfassung-und-den-tarifen-des-traegers.md)
und den Beitragsregeln der Ferien aus
[ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md)
auf. Sie führt keinen Owner ein (ADR 0057 Punkt 5, ADR 0070).

## Entscheidung

1. **Eine Regeltabelle.** Die Regeln des Monatsbeitrags sind Zeilen von
   `parent_billing.contribution_rules` mit der Leistung `care`, neben den
   Ferienregeln. Es gibt keine zweite Regelkette: Anteil und fester Abzug
   rechnet dieselbe Funktion wie bei den Ferien (`siblingDiscount`), die
   Rangfolge der Kinder folgt ADR 0115 Punkt 3.
2. **Reihenfolge.** Nach dem Grundpreis aus der Preisliste rechnet der
   Monatslauf: Einkommensstufe, Geschwister, Alleinerziehende, Höchstbetrag
   (ADR 0088 §4; Befreiung, Kostenträger und Korrektur folgen mit eigenen
   Issues). Jede Regel rundet kaufmännisch auf ganze Cent und schreibt eine
   eigene Ermäßigungszeile; geteilte Abzüge über mehrere Module ergeben den
   Betrag auf den Cent (größter Rest). Keine Ermäßigung macht eine Position,
   eine Rechnung oder eine Familie negativ.
3. **Geltungsbereich.** Eine Regel nennt ihren Namen, die Monate (vom 1. bis
   zum Monatsende), die Schulen, an denen sie gilt, und optional Module; ohne
   Module gilt sie für alle Module dieser Schulen. Zwei Regeln einer Art
   gelten nie im selben Monat an derselben Schule; Einkommensstufen sind die
   Ausnahme, ein Vertrag hat höchstens eine. Die Prüfung läuft unter der
   Sperre der Monatsabrechnung, so dass eine Freigabe nie eine halbe
   Regeländerung sieht.
4. **Geschwister.** Wirkung ab dem 2. bis 9. Kind: Anteil, fester Abzug oder
   **fester Endpreis** (SJR-Abgleich Zeile 5). Die Familie ist bis #4689 die
   Mail-Adresse des Rechnungsempfängers ohne Groß- und Kleinschreibung, wie
   die Familien aus #4691; ohne Adresse ist ein Vertrag seine eigene
   Familie. Gezählt werden die Kinder (Kind-Monat, ADR 0158 E1) der Familie,
   die an einer **Zählschule** der Regel ein Modul der Regel gebucht haben.
   Ohne Wahl sind die Zählschulen die Schulen der Regel, nie automatisch
   alle Schulen des Trägers (E5 A); schulübergreifend zählt nur, wer es
   wählt. Reihenfolge: höchster Grundbeitrag der Regelmodule zuerst, bei
   Gleichstand das ältere Kind, dann eine feste Reihenfolge. Die Rechnung
   nennt den Rang („2. Kind von 3“) und die Regel; Namen anderer Kinder
   stehen nicht darauf. Ein Kind, das ein früherer Lauf des Monats
   abgerechnet hat, zählt im Rang mit. Die Kinderzahl der Familie (#5918,
   ADR 0088 §4 Nachtrag 2026-11-08) ergänzt die Zählung je Regel: Eine
   Regel, die sie nutzt, zählt `max(bestätigte Zahl, Kinder mit Vertrag)`;
   die Geschwister ohne Vertrag stehen in der Reihenfolge vorn. Kinder mit
   Vertrag nur an Schulen außerhalb der Zählschulen der Regel gehen von der
   Angabe ab (je Standort getrennt, SJR 08.11.2026).

   **Grenzen der Familie über die Mail-Adresse, bis #4689 und #5918
   hingenommen:** Nutzen zwei Familien dieselbe Rechnungsadresse, legt moto
   sie zu einer Familie zusammen; Geschwisterregel und Höchstbetrag zählen
   dann Kinder beider Familien. Hat eine Familie zwei Adressen (etwa je
   Elternteil ein Vertrag), sind es zwei Familien; die Geschwisterermäßigung
   und der gemeinsame Höchstbetrag entfallen. Das Büro erkennt beides an
   der Rechnungsadresse der Verträge und gleicht sie im Vertrag an. Erst die
   Zahler aus #4689 und die bestätigte Familie mit Kinderzahl aus #5918
   ersetzen die Adresse als Schlüssel.
5. **Höchstbetrag.** Was eine Familie an den Schulen der Regel nach den
   übrigen Regeln zahlt, sinkt auf den Höchstbetrag; der Überschuss geht vom
   letzten Kind der Rangfolge ab. Eine schon ausgestellte Rechnung des
   Monats zählt mit, wird aber nicht geändert.
6. **Nachweise.** Einkommensstufe und Alleinerziehende brauchen einen
   Nachweis je Vertrag (`parent_billing.care_evidences`): Art des
   Nachweises (Text), Prüfdatum, Gültigkeit, für die Stufe die Regel. Kein
   Dokument, kein Einkommen (Datenminimierung). Ein Nachweis gilt für den
   Monat, wenn er am 1. gilt, sonst der erste, der im Monat beginnt, wie die
   Vertragsfassung (ADR 0158 E2). Nachweise einer Art überschneiden sich je
   Vertrag nie (Ausschlussbedingung). Eine Stufe, die ein Nachweis nennt,
   lässt sich weder löschen noch in eine andere Art ändern.
7. **Zugriff und Protokoll.** Routen `/traeger/care-rules` und
   `/traeger/care-evidences` verlangen das Träger-Recht „Abrechnung“ und
   laufen in der Träger-Transaktion. Jedes Lesen der Nachweisseite und jede
   Änderung schreibt `parent_billing.care_evidence_events` je Vertrag mit
   Konto. Der Monatslauf liest die Nachweise im Rahmen seines Zwecks; sein
   Lesen steht wie bisher in `care_run_events`.
8. **Unveränderlich.** Eine ausgestellte Rechnung speichert ihre
   Ermäßigungen in `parent_billing.care_invoice_reductions` mit einer Kopie
   von Name, Parametern und Modulen der Regel, dazu je Position den
   zahlbaren Betrag (`payable_cents`). Ein späterer Lauf des Monats rechnet
   damit Höchstbetrag und festen Geschwisterpreis gegen das Ausgestellte:
   Ein früherer Einkommensabzug bleibt auf den Modulen, die seine Regel
   genannt hat; der Trigger aus ADR 0119 Punkt 7 lehnt
   jede Änderung ab. Eine spätere Änderung oder Löschung der Regel oder des
   Nachweises erreicht sie nicht.
9. **Löschung.** Nachweise und ihre Spur hängen mit `ON DELETE CASCADE` am
   Vertrag, Ermäßigungen an der Rechnung. Die Demo-Löschung (ADR 0062,
   1.15.805) erfasst sie damit ohne Änderung an
   `platform.erase_closed_demo_visitors`.

## Verworfene Optionen

- **Eigene Tabelle für Monatsregeln:** zwei Regelwerke mit getrennter
  Pflege. Die Ferienregeln behalten ihre Ausschlussbedingung, die
  Monatsregeln prüft der Owner, weil sie je Schule nebeneinander gelten.
- **Nachweis als hochgeladenes Dokument:** mehr Daten als nötig; das Büro
  prüft den Bescheid und hält fest, dass es ihn geprüft hat.
- **Geschwister über alle Schulen des Trägers:** SJR rechnet je Ort; der
  Zählbereich ist je Regel einstellbar.
- **Namen der Geschwister auf der Rechnung:** Rang und Gesamtzahl erklären
  die Reihenfolge ohne Daten anderer Kinder.

## Register

- `policy/parent-billing.json`: drei `data_objects` mit `write_owner`
  `parent-billing`.
- Datenlebenszyklus und Aufbewahrung: je drei Einträge.
- Query-Budgets `api.traeger.care_rules` und `api.traeger.care_evidences`.
- Migration `1.15.807`.
