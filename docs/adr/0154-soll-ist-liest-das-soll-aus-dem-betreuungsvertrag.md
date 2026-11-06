---
status: proposed
---

# Soll und Ist liest das Soll aus dem Betreuungsvertrag

Vorschlag vom 06.11.2026 zu [#4683](https://github.com/moto-nrw/project-phoenix/issues/4683),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 56, 57 und 118. Er baut auf
[ADR 0139](0139-soll-und-ist-je-kind-is-a-statistics-read-over-a-swappable-soll-port.md)
(Soll und Ist je Kind) und
[ADR 0153](0153-betreuungsvertrag-entsteht-im-traeger-buero-aus-der-angenommenen-anmeldung.md)
(Betreuungsvertrag) auf und führt keinen neuen Owner ein.

## Kontext

- Soll und Ist (ADR 0139) vergleicht je Kind und Tag den Betreuungsplan der
  Schule mit dem ersten Einchecken und letzten Auschecken. Das Soll ist ein
  Port; ADR 0139 sieht vor, dass ein Vertrag des Trägers ihn ersetzt.
- Der Vertrag (ADR 0153) bucht je Wochentag Module mit ihrer Endzeit und
  nennt keine Ankunft. Er liegt beim Träger; die Tenant-Policy, mit der eine
  Schule ihre Verträge liest, sollte mit der ersten Schulfläche kommen
  (ADR 0153 Punkt 9).
- Das Träger-Büro will Überschreitungen über alle Schulen auswerten (Story
  57), standardmäßig zusammengefasst und personenscharf nur mit Recht (Story
  118). Keine Transaktion trägt Schule und Träger zugleich (ADR 0105).

## Entscheidung

1. **Schule liest ihre Verträge.** Migration 1.15.760 gibt der Schule in
   ihrer Tenant-Transaktion `SELECT` auf `contracts`, `contract_versions` und
   `contract_version_services` ihrer Verträge, die kein Entwurf mehr sind.
   Ein Entwurf ist Arbeitsstand des Träger-Büros und verspricht nichts.
   Dokumente, Parteien, Versand und Spur bleiben beim Träger. Alle Schreibwege
   bleiben bei der Träger-Policy.
2. **Lesende Schnittstelle.** `carecontract.BookedCare` liefert je Kind die
   nicht abgelehnten Versionen mit ihren Leistungen; `BookedContract.Day`
   sagt, was an einem Tag gebucht ist: Die neueste Version, die den Tag
   abdeckt, entscheidet; ein Modul nach seinem letzten Tag zählt nicht; das
   Ende ist die späteste Endzeit der Module des Wochentags, unbekannt, wenn
   ein Modul keine Endzeit hat. Die Statuslogik des Vertrags bleibt
   unverändert.
3. **Soll aus dem Vertrag.** Student Presence bekommt den Port
   `ContractCareDays`. An jedem Tag, den ein Vertrag abdeckt, ersetzt er das
   Soll des Betreuungsplans: gebucht ist der Tag, wenn der Vertrag an diesem
   Wochentag ein Modul bucht, die Abholung ist das Ende des Vertrags. Ankunft
   und „Kommt heute nicht“ bleiben aus dem Betreuungsplan, denn der Vertrag
   nennt keine Ankunft. Ohne Vertrag gilt der Betreuungsplan wie bisher. Jeder
   Tag trägt seine Quelle (`planned_source`); die Schule sieht „laut Vertrag“.
   Die Belegung nach Uhrzeit (#4764) liest dasselbe Soll.
4. **Auswertung des Trägers.** `carecontract.OverrunEvaluations` liest je
   Schule des Trägers in deren Tenant-Transaktion die Überschreitungsliste von
   Soll und Ist (Port `SchoolCareTimes`, gebunden an
   `StatisticsReports.CareTimeOverruns`) und fasst sie je Schule zusammen. Ein
   Fenster ist höchstens 92 Tage lang und endet spätestens heute.
   - `GET /traeger/care-time-overruns`: Zahlen je Schule, Recht „Berichte
     lesen“ (`traeger_reports:read`).
   - `GET /traeger/care-time-overruns/children`: zusätzlich jedes Kind mit
     Namen, Recht „Verträge verwalten“ (`traeger_contracts:manage`). Die
     Rolle, die Verträge führt und mit Eltern über Buchungen spricht, hat es
     schon; ein neues Recht ist nicht nötig.
   Eine Schule eines anderen Trägers antwortet mit 403
   (`care_contract.school_outside_traeger`).
5. **Protokoll.** Jede Lesung einer Schule steht im Zugriffsprotokoll der
   Schule mit dem Konto des Träger-Büros (Statistik, Bereich `care_times`).
   Zeigt die Auswertung Namen, schreibt sie in derselben Tenant-Transaktion
   je Schule zusätzlich `care_time_overruns` mit den Kindern und dem Fenster.
   Eine Schule ohne genanntes Kind schreibt keinen solchen Eintrag.
6. **Register.** Die Bindung `SchoolCareTimes.Overruns` ist ein Blatt nach
   ADR 0067 (Lesen mit Protokoll), `BookedCare` und `BookedContract.Day` sind
   Lesungen. Die Compose-Budgets steigen um 23 Zeilen
   (`modules/carecontract/compose` +19, `modules/studentpresence/compose`
   +4). Die drei Abfrage-Budgets von Soll und Ist steigen um je ein Statement:
   die eine Lesung der gebuchten Leistungen je Anfrage.

## Verworfene Optionen

- **Neuer Owner für die Auswertung:** Die Spec ordnet den Soll-Ist-Vergleich
  dem Betreuungsvertrag zu; der Owner hat schon Schulen, Laufzeit und
  Protokoll des Trägers.
- **Schule liest Verträge in einer Träger-Transaktion:** öffnet einer
  Schulsitzung die Rolle des Trägers und filtert nur in der Anwendung.
- **Überlagerung in der Wurzel:** Eine Wurzel-Struktur mit zwei Owners
  verbietet ADR 0059; die Regel gehört zum Soll von Student Presence.
- **Neues Recht für Namen:** Rollentabelle und Vorgaben aller Träger würden
  sich ändern, ohne dass eine Rolle anders arbeiten soll.

## Folgen

- Sobald #4680 Verträge bei der Annahme abschließt und #4679 sie
  unterschreibt, zählen sie ohne weitere Änderung, solange sie kein Entwurf
  sind.
- Ferien bucht die Ferienbetreuung (ADR 0115). Wie beim Betreuungsplan gilt
  das Soll des Wochentags auch an einem Ferientag, an dem das Kind da war.
  Ein Kind in der Ferienbetreuung kann dort länger als gebucht erscheinen.
  Ferientage aus dem Vergleich zu nehmen oder die Ferienbuchung als Soll zu
  lesen, ist ein eigener Schritt.
- Eine Toleranz je Träger wäre ein Setting; heute gelten die 15 Minuten von
  ADR 0139.
