---
status: accepted
---

# Soll und Ist liest das Soll aus dem Betreuungsvertrag

Freigegeben von Yannick Wenger am 07.11.2026

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

1. **Schule liest ihre Verträge.** Migration 1.15.785 gibt der Schule in
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
   +4). Die Abfrage-Budgets von Soll und Ist bleiben: Die Vertragslesung
   kostet ein Statement, dafür liest Soll und Ist die beiden Löschfristen
   (`gdpr.status_day_retention_days`, `gdpr.attendance_retention_days`) jetzt
   in einer Settings-Lesung (`StatisticsRetention.OccupancyRetentionDays`).

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

## Nachtrag 07.11.2026: Ferienbuchung (#5994)

Die Vorrangregel war in #4683 offen. Diese Umsetzung nimmt Option 1 aus
[#5994](https://github.com/moto-nrw/project-phoenix/issues/5994) an; eine
andere fachliche Entscheidung ändert diesen Abschnitt und die Tests.

1. **Ferienbuchung vor Vertrag.** An einem Tag mit bestätigter Ferienbuchung
   ist ihr Zeitmodell das Soll: Ankunft ist sein Beginn, Abholung sein Ende.
   Vertrag und Betreuungsplan treten an diesem Tag zurück. Es gibt genau ein
   Soll und einen Vergleich je Kind und Tag; nichts wird addiert. Ein
   „Kommt heute nicht“ bleibt. Die Quelle heißt `holiday_booking`, die
   Schule sieht „laut Ferienbuchung“.
2. **Zuordnung der Feriengäste.** Buchung und Anwesenheit eines Gasts
   verbindet seine Kopie in der ausrichtenden Schule
   (`enrollment.holiday_guests`, ADR 0120), gelesen in deren
   Tenant-Transaktion über `enrollment.HolidayGuestBookedCare`. Gebucht sind
   die Tage der Woche bis zum Tag vor einer Stornierung.
3. **Eigene Kinder.** Ein Kind der ausrichtenden Schule selbst wird nicht
   kopiert. Migration 1.15.796 gibt der Schule die schmale Funktion
   `enrollment.holiday_host_own_bookings`: die bestätigten Buchungen ihrer
   eigenen Kinder in den Wochen, die sie ausrichtet, mit Namen,
   Geburtsdatum, Woche und Zeitmodell; keine Gesundheit, keine Kontakte.
   Sie antwortet auch nach der Woche, denn der Vergleich schaut zurück;
   die Anmeldungen gehen 92 Tage nach dem Zeitraum (#4720) und mit ihnen
   diese Zeilen. Das erweitert ADR 0116 nur um diesen Zweck (Nachtrag
   vom 07.11.2026, akzeptiert). Das Kind findet
   Enrollment nach der Regel der Teilnehmerliste (#4722): gleiche Namen
   (ohne Groß- und Kleinschreibung und Leerzeichen am Rand) und gleiches
   Geburtsdatum, gelesen gebündelt aus dem People Directory und nur, wenn es
   eigene Buchungen gibt. Eine Buchung, die zu keinem oder zu mehreren
   Kindern passt, bucht niemanden; die Eindeutigkeit gilt für die ganze
   Schule, damit Klassen-, Gruppen- und Einzelansicht dasselbe Soll sehen.
   Storniert das Träger-Büro eine bestätigte Anmeldung, bleiben die Tage vor
   der Stornierung gebucht (wie `ended_on` bei Gästen); eine Umbuchung
   ändert die Buchung erst ab ihrem Tag. Bestätigung, Stornierung und
   Umbuchung stehen in der Spur der Anmeldung (ADR 0114). Die Zuordnung über
   Name und Geburtsdatum hat Yannick Wenger am 07.11.2026 freigegeben
   (#5994): Die Ferienanmeldung hat keine andere Verbindung zum Kind.
4. **Ohne Buchung.** Ist ein Kind an einem Ferientag da, für den keine
   Ferienbuchung gilt, entscheidet das Soll darunter; ohne Vertrag und Plan
   ist der Tag „ohne Buchung“. Ein verpasster Tag einer Ferienbuchung in den
   Schulferien zählt als „Nicht da“, nicht als geschlossen; Feiertage und
   Schließtage bleiben geschlossen.
5. **Abfrage-Budgets.** Gäste und eigene Buchungen kommen in einem
   Statement je Schule. Dafür liest der Schulkalender Schließtage und
   Ferienzeiträume jetzt in einer Abfrage (`schoolcalendar.CareCalendarQuery`);
   die Budgets bleiben gleich. Die Namen und Geburtsdaten liest Enrollment
   nur, wenn die Schule eigene Buchungen im Zeitraum hat.
