---
status: proposed
---

# Monatsrechnungen nehmen Schreiben, Versand und Storno der Ferienrechnungen

**Status: Vorgeschlagen.**

Vorschlag vom 08.11.2026 zu [#4684](https://github.com/moto-nrw/project-phoenix/issues/4684),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611).
Er löst den offenen Punkt 11 aus
[ADR 0158](0158-monatsbeitrag-entsteht-aus-der-wirksamen-vertragsfassung-und-den-tarifen-des-traegers.md)
(„Senden und Druck der Schreiben“) und die Zusammenführung für Versand und
Storno, die ADR 0158 unter „Verworfene Optionen“ offenließ. Er folgt
[ADR 0119](0119-abrechnungslauf-stellt-unveraenderliche-rechnungen-aus-einem-nummernkreis-aus.md),
[ADR 0121](0121-bestaetigungsschreiben-ist-die-rechnung-im-layout-des-traegers.md),
[ADR 0123](0123-bestaetigungsschreiben-gehen-als-mail-des-traegers-ueber-die-outbox.md)
und [ADR 0126](0126-dokumentenpostfach-der-eltern-app-liest-die-schreiben-des-traegers.md).
Er führt keinen Owner ein und hält fest, was er in den Registern ändert
(ADR 0057 Punkt 5, ADR 0070).

## Kontext

Ein freigegebener Monatslauf stellt Rechnungen aus, aber die Familien bekommen
sie noch nicht. Der SJR Pforzheim prüft zwischen „erstellt“ und „ins
Elternportal freigegeben“ (SJR-Abgleich E1 A: Ausstellen und Senden bleiben
getrennt; nach der Ausstellung nur Storno und Neuausstellung). Für die
Ferienrechnungen gibt es diese Schritte schon: Schreiben im Layout des Trägers,
Mail über die Outbox, Postfach der Eltern erst nach dem Senden, Sammeldruck,
Storno mit demselben Nummernkreis.

## Entscheidung

1. **Dieselbe Strecke, eigene Tabellen.** Die Monatsrechnungen nutzen den
   Renderer und das Layout der Schreiben (ADR 0121), die Mail-Art
   `parent_billing_letter` mit eigener Vorlage `billing-care-letter.html`, die
   Outbox und den Push „Neues Dokument vom Träger“ (ADR 0123, 0126), den
   Nummernkreis und Zähler der Leistung `care` (ADR 0158 Punkt 5) und den
   Trigger gegen jede Änderung (ADR 0119 Punkt 7). Gespeichert werden sie in
   `parent_billing.care_run_letters`, `care_letters`, `care_letter_mailings`
   und `care_invoice_cancellations`, weil `parent_billing.documents` an
   Ferienanmeldung und Ferienzeitraum hängt (ADR 0158, verworfene Option).
2. **Erst erstellen, dann senden.** Die Abrechnung erstellt die Schreiben eines
   freigegebenen Laufs einmal, mit Zahlungsdatum und freiem Hinweis. Eine Familie
   sieht ein Schreiben erst, wenn es gesendet ist: Das Postfach liest nur
   Schreiben mit mindestens einem Versand (ADR 0126 Punkt 1).
3. **Genau einmal.** Jeder Versand ist ein Eintrag der Outbox mit Schlüssel je
   Schreiben und Versandnummer, gespeichert in derselben Träger-Transaktion. Der
   Versand an alle sendet nur Schreiben ohne Versand; ein erneuter Versand wartet,
   bis der letzte Versuch fertig ist, und nur der erste Versand pusht. Lauf und
   Rechnung zeigen den Stand (nicht gesendet, wird gesendet, gesendet,
   fehlgeschlagen, unbekannt) und erlauben den erneuten Versuch.
4. **Empfänger beim Versand neu geprüft.** Die Mail geht an die Adresse, die die
   Rechnung kopiert hat, und nur solange sie noch die Adresse der Vertragspartei
   ist, an die der Vertrag abrechnet (Port `CareContracts.Recipients`). Sonst
   lehnt der Versand ab (409 `parent_billing.care_recipient_changed`); die
   Abrechnung storniert, und der nächste Lauf kopiert die Partei von heute. Das
   Postfach zeigt ein Schreiben nur dem Konto mit dieser Adresse und mit
   `parent_portal.enrollments.view` an einer Schule des Trägers (ADR 0126 Punkt 2).
5. **Eine ID für das Postfach.** `care_letters.id` kommt aus der Sequenz von
   `parent_billing.documents`. So nennt das Postfach Ferien- und Monatsschreiben
   mit einer ID ohne Überschneidung, und Workflow, Route und Push bleiben
   unverändert. Die Lesung steht in der Spur des jeweiligen Laufs.
6. **Storno und Neuausstellung.** Ein Storno nennt seine Rechnung, hat eine
   Pflichtbegründung, verneint den ganzen Betrag und nimmt die nächste Nummer aus
   dem Zähler der Rechnungen. Eine Rechnung wird höchstens einmal storniert. Die
   stornierte Rechnung rechnet ihren Vertrag nicht mehr ab: Der nächste Lauf des
   Monats stellt sie neu aus und nennt sie (`replaces_invoice_id`, höchstens
   einmal: `uq_care_invoices_reissue`; die erste Rechnung je Vertrag und Monat
   bleibt eindeutig). Storno und Freigabe halten dieselbe Sperre der
   Monatsabrechnung.
7. **Storno vor dem Versand erreicht die Familie nicht.** Wurde das Schreiben der
   Rechnung nie gesendet, bekommt der Storno kein Schreiben, die Rechnung wird
   nicht mehr gesendet, und weder Mail noch Postfach noch Druck nennen sie
   (Nachtrag 2026-11-08 zu ADR 0119). Wurde sie gesendet, bekommt der Storno
   sofort sein Schreiben, das die Abrechnung danach sendet.
8. **Sammeldruck.** Alle Schreiben, die der Lauf noch senden darf, gehen als ein
   PDF; mit `only=without_email` nur die der Familien ohne Adresse.
9. **Demo-Löschung.** Schreiben und Versände gehen mit ihrer Rechnung (Cascade).
   Die Kopien auf der Outbox (Mail mit PDF und Adresse, ihr Zustellergebnis, der
   Push) nennen die Rechnung nur über den Versand; `platform.erase_closed_demo_visitors`
   löscht sie vor den Rechnungen und hält vorher die Läufe wie ein Versand.

## Verworfene Optionen

- **Monatsrechnungen in `parent_billing.documents` überführen:** Alle Lesepfade
  der Ferien (Familien, Übersicht, Gutschrift, Korrekturvorschläge) müssten Art
  und Bezug unterscheiden; der Umbau gehört zu #4689 (Bündelung).
- **Freigabe ins Postfach als eigener Schalter:** Ein Schreiben im Postfach ohne
  Mail widerspräche ADR 0126 Punkt 1; der Versand stellt beides zu.
- **Storno-Schreiben auch vor dem Versand:** Die Familie bekäme ein Storno zu
  einer Rechnung, die sie nie sah (SJR-Abgleich E1).
- **Eigene Mail-Art für Monatsschreiben:** eine zweite Darstellung und eine zweite
  Zeilenart der Outbox ohne neuen Inhalt; die Art des Dokuments steht im Payload.

## Register

- `policy.json`: vier `data_objects` mit `write_owner` `parent-billing`, keine
  neue Regel.
- Datenlebenszyklus und Aufbewahrung je vier Einträge.
- Acht Routen im Route-Golden, fünf schreibende in der Nutzungsanalyse als
  `notCaptured` (Träger, keine Schule).
- Query-Budget `api.traeger.care_billing_run_letters`.
- Budgets: `modules/parentbilling/compose` und Durchreichungen, soweit
  `scripts/backend-architecture.sh check` sie misst, mit dieser Entscheidung.
- Fehlercodes `parent_billing.care_invoice_not_found`, `care_invoice_cancelled`,
  `care_recipient_changed`.
- Migration `1.15.809` (Tabellen, `care_invoices.replaces_invoice_id`,
  Demo-Löschung auf der Fassung von 1.15.805).
