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
   - die Buchungen dieser Woche, deren Anmeldung bestätigt ist (nur IDs).

   Die Kinder liest die Schule nur über die Funktion
   `enrollment.holiday_host_participants`: die Mindestfelder (Punkt 3) der
   bestätigten Kinder einer Woche, die sie ausrichtet. Eine Zeilen-Policy auf
   `enrollment.holiday_registration_children` gibt es für die Schule nicht,
   weil sie einer Rolle, die die ganze Tabelle lesen darf, die
   Gesundheitsspalten nicht verbergen könnte. Anmeldung, Eltern, Kontakte,
   Einwilligungen und das Protokoll des Trägers sieht die Schule nicht. Keine
   Policy erlaubt ihr zu schreiben. Eine andere Schule des Trägers und jede
   Schule eines anderen Trägers sehen nichts. Storniert oder bucht das
   Träger-Büro um, verschwindet das Kind sofort. Die Funktionen mit
   `SECURITY DEFINER` (`enrollment.holiday_host_week`,
   `enrollment.holiday_host_booking`, `enrollment.holiday_host_participants`)
   lesen Zeitraum, Woche, Anmeldung und Kind ohne die Träger-Policy, damit
   die Policies keinen Kreis bilden, und liefern außerhalb einer
   Schul-Transaktion nichts. Sie sind kein allgemeiner Weg an RLS vorbei:
   jede prüft die ausrichtende Schule der Sitzung selbst. Die
   Träger-Transaktion behält genau ihre bisherige Sicht.
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

## Nachtrag 2026-10-28: Freigegebene Antworten und zweite sorgeberechtigte Person (#5202)

Der Nutzer hat am 28.10.2026 Option B „Ohne Gesundheit“ gewählt
([#5202](https://github.com/moto-nrw/project-phoenix/issues/5202), Spec #4611
B7). Punkt 3 gilt mit dieser Erweiterung:

- **Mehr als die Mindestfelder.** Zu den Mindestfeldern kommen je Kind Name
  und Telefon der zweiten sorgeberechtigten Person (#5197) und die Antworten
  auf die Fragen, die der Träger im Formular-Baukasten (#5198) für die
  ausrichtende OGS freigibt (`share_with_host`). Die Standardvorlage gibt
  „Darf allein nach Hause gehen“ und „Darf an Ausflügen teilnehmen“ frei;
  Formularversionen von vorher geben nichts frei. Eine Freigabe wirkt ab der
  Version, die sie setzt: Es zählt die Version, mit der die Anmeldung kam.
- **Keine Gesundheitsangaben.** Gesundheitsangaben bleiben beim Träger, bis
  [#4722](https://github.com/moto-nrw/project-phoenix/issues/4722) umgesetzt
  ist; das gilt für Allergien, Medikamente und Gesundheitshinweise und für
  jede Frage mit Gesundheitskennzeichen. Der Baukasten lehnt eine Freigabe
  an einer Gesundheitsfrage ab (`enrollment.holiday_form_field_not_allowed`,
  Feld `fields.N.share_with_host`).
- **Filter in der Datenbank.** `enrollment.holiday_host_participants` gibt
  eine Antwort nur heraus, wenn die Version der Anmeldung die Frage freigibt
  und als `health: false` (JSON-Boolean) markiert, keine Version des
  Zeitraums dieselbe Frage anders markiert und die Frage eine Antwort hat
  (kein Infotext). Ein fehlendes oder unklares Kennzeichen gilt als
  Gesundheitsangabe. Keine E-Mail-Adresse verlässt die Funktion.
- Liste und Export zeigen eine Spalte je freigegebener Frage und die zweite
  Person, wenn ein Kind eine hat. Ohne beides bleiben sie wie bisher. Das
  Zugriffsprotokoll (Punkt 6) und das Ende nach der Woche (Punkt 4) bleiben
  unverändert.
- **Register.** Migration `1.15.712` ersetzt die Funktion; der Down-Pfad
  stellt die alte Signatur wieder her. Keine neue Tabelle, keine neue Regel
  in `policy.json`.

Option C (Gesundheitsangaben für die OGS) braucht #4722 und einen eigenen
Nachtrag.

## Nachtrag 2026-11-01: Informationsfluss statt Datensilos

Der Nutzer ersetzt Option B „Ohne Gesundheit“ aus dem Nachtrag vom 28.10.2026
als dauerhaftes Produktziel durch Option C: Die ausrichtende OGS erhält die
für die Betreuung benötigten Gesundheitsangaben im normalen Ablauf. Dazu
gehören Allergien, Medikamente, Gesundheitshinweise und Antworten auf
Gesundheitsfragen. Dass die Anmeldung beim Träger liegt, begründet keine
Informationssperre für die tatsächlich betreuende OGS. Es gilt die
[Regel zum Informationsfluss](../../.claude/rules/information-flow.md).

Die Umsetzung und ihre Nachweise gehören zu
[#4722](https://github.com/moto-nrw/project-phoenix/issues/4722), das die
Weitergabe bereits umfasst. Der Zugriff muss den berechtigten Beteiligten
für die bestätigte Betreuung und deren Zeitraum dienen, mit passender
Rechtsgrundlage, erforderlichen Einwilligungen, Zugriffsschutz und
Zugriffsprotokoll. Für bestehende Anmeldungen muss die Rechtsgrundlage die
konkrete Weitergabe ebenfalls tragen; dieser Nachtrag erteilt keine
rückwirkende Freigabe.

**Produktentscheidung, noch kein Implementierungsnachweis:** Dieser Nachtrag
ersetzt das dauerhafte Ziel „keine Gesundheitsangaben für die OGS“, entfernt
aber keine bestehenden Filter oder Berechtigungen. Bis zur nachgewiesenen
Umsetzung beschreibt der Nachtrag vom 28.10.2026 weiter den technischen
Stand. Die neue Entscheidung erlaubt weder Zugriff durch andere Schulen
noch pauschale Gesundheitsexporte oder eine Ausweitung bestehender Fristen.

## Nachtrag 2026-11-01: Standard ist freigegeben (#5470)

Neue Fragen im Ferien-Baukasten starten mit `share_with_host: true`
([#5470](https://github.com/moto-nrw/project-phoenix/issues/5470)). Die
ausrichtende OGS braucht die Antworten für die Betreuung; das Träger-Büro
kann die Freigabe je Frage ausschalten. In der Standardvorlage sind alle
Fragen ohne Gesundheitskennzeichen freigegeben. Die Online-Anmeldung der
Schule behält ihren bisherigen Standard.

Gespeicherte Formularversionen und abgegebene Anmeldungen bleiben unverändert.
Eine Freigabe wirkt weiterhin erst ab der Version, die sie setzt. Bis zur
Umsetzung von #4722 bleiben Gesundheitsfragen von der Freigabe ausgeschlossen.
Die Empfänger, Berechtigungen, Zugriffsprotokolle und Fristen ändern sich nicht.

## Nachtrag 2026-11-01: Gesundheitsangaben für die ausrichtende OGS (#4722)

Der Nutzer hat am 01.11.2026 entschieden: keine Datensilos, Option C aus
[#5202](https://github.com/moto-nrw/project-phoenix/issues/5202) sofort mit
[#4722](https://github.com/moto-nrw/project-phoenix/issues/4722). Eine
Gesundheitsangabe für die Ferienbetreuung kommt dort an, wo das Kind betreut
wird, geschützt und befristet. Punkt 3 und der Nachtrag vom 28.10.2026
(„Keine Gesundheitsangaben“) gelten mit dieser Änderung:

- **Was die OGS sieht.** Je bestätigtem Kind ihrer Woche Allergien,
  Medikamente, weitere Hinweise zur Gesundheit und die Antworten auf die
  Gesundheitsfragen, die die Version der Anmeldung mit `share_with_host`
  freigibt (ein fehlendes oder unklares Gesundheitskennzeichen gilt als
  Gesundheitsfrage). Liste und Feriengast zeigen sie unter „Gesundheit“.
  Die Liste öffnet sie nur für ein Konto mit `users:read`, dem Recht, das
  auch das Gesundheitsfeld eines Kindes öffnet; ohne es bleibt die Spalte
  leer.
- **Nur mit neuer Einwilligung.** Die Angaben gehen nur weiter, wenn die
  Eltern der Gesundheitsangabe zugestimmt haben und die Anmeldung unter den
  Einwilligungstexten der Version 2 kam (`consent_version`), die die OGS als
  Leserin nennen. Ältere Anmeldungen geben nichts weiter.
- **Verschlüsselt bis in die Schul-Transaktion.**
  `enrollment.holiday_host_participants` liefert die verschlüsselten Werte
  und die Definition der freigegebenen Gesundheitsfragen, nie Klartext.
  Enrollment entschlüsselt sie erst in der Tenant-Transaktion der
  ausrichtenden Schule.
- **Dauer.** Wie die übrigen Angaben: von der Bestätigung bis zum letzten Tag
  der Woche (Punkt 4).
- **Kein Gesundheitsexport.** CSV und Excel bleiben ohne Gesundheitsspalten;
  die Liste im Browser reicht.
- **Eigene Kinder der Schule.** Für ein Kind der ausrichtenden Schule zeigt
  die Kindakte die Angaben als „Angaben aus der Ferienanmeldung“
  (`GET /api/holiday-weeks/students/{studentId}/health`, Recht `users:read`
  wie das Gesundheitsfeld). Zugeordnet wird über Heimatschule, Vor- und
  Nachname und Geburtsdatum. Das Gesundheitsfeld der Schule bleibt
  unverändert; weichen die Angaben ab, steht beides untereinander.
- **Zugriffsprotokoll.** Der Eintrag einer Liste nennt mit
  `health_participants`, wie viele Kinder mit Gesundheitsangaben sie zeigte;
  das Lesen am eigenen Kind schreibt `holiday_student_health` mit dem Kind.
- **Baukasten.** Das Verbot `enrollment.holiday_form_field_not_allowed` für
  `share_with_host` an einer Gesundheitsfrage entfällt; die Standardvorlage
  gibt ihre Gesundheitsfragen frei. Damit endet auch der Ausschluss aus dem
  Nachtrag zu #5470: Neue Fragen starten freigegeben, Gesundheitsfragen
  eingeschlossen.
- **Register.** Migration `1.15.722` verschlüsselt die bestehenden Zeilen,
  entfernt die Klartextspalten und ersetzt die Funktion; der Down-Pfad stellt
  Klartext und Funktion von #5202 wieder her. Neue Route im Schul-Router
  (lesend, keine Nutzungsanalyse). Keine neue Tabelle, keine neue Regel in
  `policy.json` außer der Regel für die Migration auf Security Runtime.
