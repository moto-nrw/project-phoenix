---
status: accepted
---

# Ausrichtende OGS liest ihre Teilnehmer in der Schul-Transaktion

Entscheidung vom 25.10.2026 zu [#4665](https://github.com/moto-nrw/project-phoenix/issues/4665),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 41 und 42. Sie setzt
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 6 für die Teilnehmerliste um und löst die Folge aus
[ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
ein: „Die Lese-Policy für die Schule nach ADR 0086 Abschnitt 5 (die
ausrichtende OGS sieht ihre Wochen) kommt mit der Teilnehmerliste.“ Sie führt
keinen neuen Owner und keine neue Stelle der Träger- oder Admin-Transaktion
ein.

## Kontext

Die Ferienanmeldungen gehören dem Träger (ADR 0110, ADR 0114). Ihre Tabellen
tragen `organization_id`, und nur die Träger-Transaktion sieht sie. Die
ausrichtende OGS arbeitet im Schul-Portal mit einer Schul-Sitzung. Sie braucht
vor ihrer Woche die Kinder, die zu ihr kommen, auch Kinder anderer Schulen des
Trägers und Feriengäste ohne Schule. Ihr Token trägt nur ihre eigene
`tenant_id` (D4).

## Entscheidung

1. **Owner und Route.** Die Teilnehmerliste gehört Enrollment wie die
   Anmeldung. `GET /api/holiday-weeks` listet die Wochen, die die Schule
   ausrichtet; `GET /api/holiday-weeks/{weekId}/participants` liefert die
   Liste einer Woche, `.../participants/export?format=csv|xlsx` die Datei über
   `lists.Document` wie der Export aus ADR 0114. Die Routen hängen im
   Schul-Router hinter der Schul-Sitzung, dem Recht `config:read` (wie die
   Anmeldelisten im Enrollment-Modul) und der Tenant-Transaktion der Schule.
   Sie gehören zu moto Basis: Die Schule braucht die Liste unabhängig von
   ihren gebuchten Modulen.
2. **Zugriff nur aus bestätigter Buchung.** Die Schul-Transaktion liest die
   Tabellen des Trägers über eine zweite, nur lesende Policy (ADR 0086
   Abschnitt 5), enger als dort vorgesehen:
   - eine Woche eines veröffentlichten Zeitraums, die die Schule ausrichtet,
     und Zeitraum und Zeitmodelle dieser Woche;
   - die Buchungen dieser Woche, deren Anmeldung bestätigt ist, und die
     Kinder dieser Buchungen.

   Anmeldung, Eltern, Kontakte, Einwilligungen und das Protokoll des Trägers
   sieht die Schule nicht. Keine Policy erlaubt ihr zu schreiben. Eine andere
   Schule des Trägers und jede Schule eines anderen Trägers sehen nichts.
   Storniert oder bucht das Träger-Büro um, verschwindet das Kind sofort.
   Zwei Funktionen mit `SECURITY DEFINER` (`enrollment.holiday_host_week`,
   `enrollment.holiday_host_booking`) entscheiden Woche und Buchung. Sie lesen
   Zeitraum, Woche und Anmeldung ohne die Träger-Policy, damit die Policies
   keinen Kreis bilden, und antworten außerhalb einer Schul-Transaktion mit
   „nein“. Die Träger-Transaktion behält genau ihre bisherige Sicht.
3. **Mindestfelder.** Je Kind Vorname, Nachname, Geburtsdatum, Heimatschule
   oder Feriengast mit der Schule, die die Eltern genannt haben, und das
   gebuchte Zeitmodell. Gesundheitsangaben, Notfallkontakte und Abholung
   bleiben beim Träger; sie kommen mit dem Feriengast im Betreuungsalltag
   (#4666). Die Liste zählt die Kinder je Zeitmodell und je Heimatschule.
4. **Ende nach der Woche.** Der Zugriff besteht von der Bestätigung bis zum
   letzten Tag der Woche und endet danach ohne Zutun. Enrollment entscheidet
   das mit dem Kalendertag seiner Uhr (Test-Uhr im Test). Die Datenbank prüft
   es unabhängig davon noch einmal mit dem Berliner Kalendertag: Ist der
   letzte Tag vorbei, sieht die Schule Woche und Kinder nicht mehr, auch wenn
   eine Abfrage den Filter vergäße. Die sieben Tage nur lesend aus ADR 0084
   Abschnitt 6 gelten für den Feriengast im Betreuungsalltag (#4666), nicht
   für die Liste.
5. **Kein Token mit weiterer Schule.** Das Token der Sitzung trägt nur die
   `tenant_id` der ausrichtenden Schule. Der Zugriff ergibt sich allein aus
   der Datenbank-Policy für diese Schule.
6. **Zugriffsprotokoll.** Jedes Lesen der Liste und jeder Export schreiben in
   derselben Tenant-Transaktion einen Eintrag in `audit.data_access_log` der
   ausrichtenden Schule (`holiday_participants`,
   `holiday_participants_export`), mit Woche, Zahl der Kinder und Format.
   Scheitert der Eintrag, scheitert die Anfrage. Die Wochenliste enthält nur
   Zahlen und wird nicht protokolliert (ADR 0084 Abschnitt 7). Die Einträge
   erscheinen im Zugriffsprotokoll der Schule und im Träger-Portal (ADR 0109).
7. **Schulnamen.** Die Namen der Heimatschulen liest die Route vor der
   Tenant-Transaktion über `organizationtenancy.FindSchool` und
   `ListSchoolsByOrganization`, gebunden an den Port
   `enrollment.HolidayHostSchoolNames` (wie ADR 0114 Punkt 9). Enrollment
   liest `platform.schools` nicht selbst.
8. **Register.**
   - `command-effects.json`: die Bindung der Schulnamen (query) und das
     Anhängen ans Zugriffsprotokoll (platform, ADR 0098).
   - `passthrough.json`: `HolidayHostParticipants.HostedHolidayWeeks` als
     Übersetzung an der Owner-Grenze und der Adapter des Zugriffsprotokolls
     in der Root. Das Pass-through-Budget von `modules/enrollment` steigt
     deshalb um eins (27 auf 28, Summe 1406 auf 1407).
   - Query-Budgets `api.holiday_weeks.list` (2, exact) und
     `api.holiday_weeks.participants` (6, exact).
   - Keine neue Tabelle, kein neuer `data_objects`-Eintrag, keine neue Regel
     in `policy.json`.

## Verworfene Optionen

- **Kopie in die Schule bei Bestätigung:** ADR 0084 sieht den Feriengast als
  befristete Kopie in der ausrichtenden OGS vor. Er gehört zum
  Betreuungsalltag mit Gruppe und Check-in (#4666) und braucht ein eigenes
  Modell. Die Liste soll schon vor der Woche den aktuellen Stand zeigen, auch
  nach einer Umbuchung oder Stornierung, ohne dass eine Kopie nachgezogen
  werden muss.
- **Lesen in der Träger-Transaktion des Trägers der Schule:** Die Schule
  hätte für die Dauer der Anfrage alle Anmeldungen des Trägers vor sich, und
  nur die Abfrage würde filtern. Eine neue Stelle der Träger-Transaktion im
  Schul-Portal widerspricht ADR 0105 Punkt 3.
- **Ende nur im Code:** Die Test-Uhr allein reicht für den Test, aber dann
  hinge das Ende an jeder künftigen Abfrage. Die Datenbank-Prüfung ist das
  Netz, der Code bleibt testbar.

## Folgen

- Der Feriengast im Betreuungsalltag (#4666) kann dieselbe Policy und
  dieselben Funktionen nutzen oder eine Kopie anlegen; das entscheidet er.
- Die Löschung drei Monate nach Ende (#4667) betrifft die Tabellen des
  Trägers; die Liste hält keine eigene Kopie.
- Wer in der Schule `config:read` hat, sieht die Liste. Ein eigenes Recht
  dafür gibt es nicht.
