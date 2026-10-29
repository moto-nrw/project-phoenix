---
status: accepted
---

# Feriengast ist ein befristetes Kind der ausrichtenden OGS

Entscheidung vom 26.10.2026 zu [#4666](https://github.com/moto-nrw/project-phoenix/issues/4666),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 43 und 44. Sie setzt
[ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 6 für den Betreuungsalltag um und beantwortet die dort offene
Frage, wer das Feriengast-Modell besitzt. Sie baut auf
[ADR 0116](0116-ausrichtende-ogs-liest-ihre-teilnehmer-in-der-schul-transaktion.md)
auf und führt keinen neuen Owner, keine neue Stelle der Träger- oder
Admin-Transaktion und keine neue Regel in `policy.json` ein.

## Kontext

Die ausrichtende OGS sieht seit ADR 0116 die bestätigten Kinder ihrer
Ferienwoche als Liste. Im Alltag braucht ihr Team die Feriengäste dort, wo es
alle Kinder sieht: in einer Gruppe und im Check-in, am Tablet wie im Browser.
Gruppen, Check-in, Anwesenheit und Kiosk lesen und schreiben Kinder der
Schule (`users.student_profiles` mit `tenant_id`), RLS hält sie in der
Schule. Ein Feriengast ist kein Kind der Schule: Er geht auf eine andere
Schule des Trägers oder auf gar keine Schule des Trägers. Das Token trägt nur
die `tenant_id` der ausrichtenden Schule (D4).

## Entscheidung

1. **Owner.** Der Feriengast gehört Enrollment, wie Anmeldung, Buchung und
   Teilnehmerliste (Spec „Enrollment und Care Plan: Feriengast als eigenes
   Modell“). Öffentliche Capability `enrollment.HolidayGuests`; die Tabellen
   `enrollment.holiday_host_groups`, `enrollment.holiday_guests` und
   `enrollment.holiday_guest_contacts` liegen im Tenant der ausrichtenden
   Schule, mit RLS auf `tenant_id`, Schreib-Owner Enrollment.
2. **Befristete Kopie als Kind der Schule.** Wie ADR 0084 Abschnitt 6 es
   vorsieht, wird der Feriengast in der ausrichtenden OGS angelegt: als Kind
   der Klasse `Feriengast` (People Directory verlangt eine Klasse; Listen und
   Karten zeigen so auf einen Blick, wer Gast ist), mit `enrolled_from` und
   `enrolled_until` gleich dem ersten und letzten Tag der Woche. Vor der Woche
   wartet das Kind wie ein neu aufgenommenes Kind vor seinem ersten
   Betreuungstag (`pending`, in keiner Gruppe); ab dem ersten Tag setzt der
   Abgleich es aktiv in die Gruppe, die die Schule für die Woche gewählt hat.
   So steht ein Kind mit zwei Wochen derselben OGS nie doppelt in der Gruppe. Enrollment
   schreibt das Kind über die vorhandenen Befehle des People Directory
   (`CreateChildPerson`, `CreateEnrollmentStudent`, `SetPersonDetails`,
   `RenewEnrollmentStudent`, `ApplyEnrollmentProfile`) in der
   Tenant-Transaktion der Schule, mit Validierung und Zeilensperre des
   Owners und ohne Sperrreihenfolge über Owner (ADR 0067, Nachtrag
   „command participation“, Bedingungen 1 bis 3). Gruppen, Check-in,
   Anwesenheit, Kiosk und Kiosk-Vertrag bleiben unverändert; der Gast hat
   kein Armband und wird im Browser an- und abgemeldet. Damit die
   Tagesplanung der Gruppe den Gast an seinen Tagen erwartet und an keinem
   anderen, trägt Enrollment für jeden Tag der Woche über die Care-Plan-
   Befehle `CreateArrivalException` und `CreatePickupException` eine
   Tagesausnahme mit Beginn und Ende des Zeitmodells ein (Grund
   „Ferienbetreuung“, Urheber die Person des Kontos, das die Gruppe gewählt
   hat); ein beendeter Gast verliert sie. Hat dieses Konto keinen
   Personaleintrag (mehr), kommt der Gast trotzdem in die Gruppe, und nur
   seine Tage bleiben offen. Jede Wahl der Gruppe plant alle Tage der Woche
   neu, auch Änderungen aus der Zeit ohne Gruppe.
3. **Zugriff nur aus bestätigter Buchung.** Die Schule wählt je Woche eine
   ihrer Gruppen (`PUT /api/holiday-weeks/{weekId}/guest-group`, Recht
   `users:create`, weil die Wahl Kinder anlegt). In derselben Transaktion
   und danach stündlich im Worker (`holiday-guest-reconciliation`, Basis)
   gleicht Enrollment die Gäste mit den bestätigten Buchungen ab. Es liest
   sie über die Policies und die Funktion
   `enrollment.holiday_host_participants` aus ADR 0116 und die neue Funktion
   `enrollment.holiday_host_guest_contacts`; beide antworten nur für eine
   Woche, die die Schule ausrichtet, bis zu ihrem letzten Tag. Kinder der
   ausrichtenden Schule selbst sind schon ihre Kinder und werden nicht
   kopiert. Ein neu bestätigtes Kind kommt dazu; ein storniertes oder
   umgebuchtes Kind, eine nicht mehr veröffentlichte oder an eine andere
   Schule vergebene Woche beenden den Gast: Seine Betreuung endet am Vortag,
   seine Kontakte werden gelöscht. Ein Abgleich hält die Sperre
   `pg_advisory_xact_lock` der Schule, damit Wahl und Worker kein Kind
   doppelt anlegen.
4. **Mindestfelder.** Die Kopie trägt Vorname, Nachname, Geburtsdatum,
   Heimatschule oder die von den Eltern genannte Schule, die Woche und das
   gebuchte Zeitmodell (Betreuung), die anmeldende Person mit Name und
   Telefon und die Notfallkontakte (Notfall) und die Personen, die das Kind
   abholen dürfen (Abholung). Keine Gesundheitsangaben, keine
   Einwilligungen, keine E-Mail-Adresse, keine Anschrift. Die Kindakte zeigt
   für einen Feriengast nur diese Angaben (`GET /api/holiday-guests/{studentId}`,
   Recht `users:read` wie Notfall und Abholung eines eigenen Kindes) und
   keine Reiter. Die Liste aus ADR 0116 bleibt unverändert.
5. **Ende nach der Woche.** Ab dem Tag nach der Woche betreut die Schule den
   Gast nicht mehr: Die Live-Listen von Gruppe, Check-in und Kiosk filtern
   schon heute auf `enrolled_until` (People Directory `currentCare`), mit dem
   Kalendertag des Lesers; keine zusätzliche Abfrage hängt daran. Danach ist
   das Blatt sieben Tage nur lesbar (ADR 0084 Abschnitt 6), dann löscht der
   Abgleich die Kontakte, und das Blatt antwortet 404. Das Kind selbst folgt
   danach der Löschfrist für Kinder (`gdpr.student_retention_days`, Lösch-
   Workflow der Kinder); die Löschung der Ferien-Daten drei Monate nach Ende
   (#4667) betrifft die Tabellen des Trägers.
6. **Kein Token mit weiterer Schule.** Das Token der Sitzung und die
   Tenant-Transaktion des Workers tragen nur die ausrichtende Schule.
7. **Zugriffsprotokoll.** Jede Übernahme (`holiday_guest_transfer`, mit
   Woche und Zahl der Gäste) und jeder Blick auf ein Blatt
   (`holiday_guest_sheet`, mit Kind und Woche) schreibt in derselben
   Tenant-Transaktion einen Eintrag in `audit.data_access_log` der Schule.
   Übernahmen des Workers stehen unter dem Konto, das die Gruppe gewählt hat,
   mit `automatic: true`. Scheitert der Eintrag, scheitert der Vorgang. Die
   Einträge erscheinen im Zugriffsprotokoll des Träger-Portals (ADR 0109).
8. **Register.**
   - `policy.json`: drei `data_objects` mit `write_owner` Enrollment.
   - `command-effects.json`: die Bindung `HolidayGuestPeople` an die
     People-Directory-Befehle (leaf, ADR 0067) und an `FindPerson` (query),
     `HolidayGuestCareDays` an die Tagesausnahmen von Care Plan (leaf,
     ADR 0067) und an `ResolveStaffIDByAccountID` (query), das
     Zugriffsprotokoll (platform, ADR 0098).
   - Datenkatalog (`data_lifecycle_catalog.json`, Scope `tenant`),
     Löschfristen und Schul-Löschung: Migration `1.15.681` installiert Plan,
     Sperrliste mit neuem FK-Fingerabdruck, Restprüfung, Löschung und
     Probelauf mit den drei Tabellen (ADR 0092).
   - Compose-LOC `modules/enrollment/compose` 585 → 591, Summe 12654 → 12660.
   - Route-, Middleware- und Modul-Golden +2 Routen (Basis), eine davon
     schreibend (Nutzungsanalyse `holiday_guests_taken_over`); Worker-Job
     `holiday-guest-reconciliation` in Basis.

## Verworfene Optionen

- **Live-Lesen in Gruppe und Check-in:** Jede Liste von Gruppe, Check-in,
  Anwesenheit und Kiosk müsste Kinder aus den Tabellen des Trägers mischen,
  und Anwesenheit und Besuche müssten auf Buchungen statt auf Kinder zeigen.
  ADR 0084 hat das verworfen.
- **Kopie bei der Bestätigung durch den Träger:** Die Bestätigung läuft in
  der Träger-Transaktion. Die Kopie bräuchte einen Workflow mit einem
  Tenant-Schritt je Schule, und ohne gewählte Gruppe wüsste niemand, wohin
  das Kind gehört. Der Abgleich in der Schule kommt mit den vorhandenen
  Policies aus und repariert sich selbst.
- **Eigene Feriengruppe automatisch anlegen:** Die Schule hätte eine Gruppe
  ohne Betreuungskräfte. Sie wählt besser eine eigene Gruppe mit ihrem Team.
- **Notfall und Abholung als Erziehungsberechtigte des People Directory:**
  Die Kontakte würden zu Personen der Schule mit Portal-Rechten und blieben
  nach dem Gast. Die Kontakte gehören zum Gast und gehen mit ihm.

## Folgen

- Ein Feriengast zählt in der Schule als aktives Kind seiner Woche, auch in
  Auswertungen, die aktive Kinder an einem Tag zählen. Liegt ein Stichtag
  der Abrechnung (ADR 0092) in einer Ferienwoche, zählt er dort mit; das
  Abrechnungsmodell entscheidet ein eigenes Ticket.
- Neu bestätigte Kinder kommen spätestens nach einer Stunde in die Gruppe;
  die Leitung kann mit derselben Wahl sofort abgleichen.
- `GET /api/active/cross-tenant-students` (ADR 0084 Abschnitt 6) bleibt
  bestehen, bis Gruppen und Kiosk keine Besuche fremder Kinder mehr
  brauchen; ein eigenes Ticket entfernt ihn.

## Nachtrag 2026-10-28: Zweite sorgeberechtigte Person und freigegebene Antworten (#5202)

Mit der Entscheidung des Nutzers vom 28.10.2026 für Option B
([#5202](https://github.com/moto-nrw/project-phoenix/issues/5202), Nachtrag
zu ADR 0116) gilt Punkt 4 mit dieser Erweiterung:

- **Zweite Person als Notfallkontakt.** `enrollment.holiday_host_guest_contacts`
  liefert die zweite sorgeberechtigte Person (#5197) als Notfallkontakt
  „Eltern“ mit Name und Telefon, direkt nach der anmeldenden Person.
- **Freigegebene Antworten.** Die Kopie übernimmt die Antworten der
  freigegebenen Fragen ohne Gesundheitsangabe, so wie die Teilnehmerliste sie
  liefert, in `enrollment.holiday_guests.answers` (JSONB). Das Blatt
  `GET /api/holiday-guests/{studentId}` zeigt sie unter „Angaben aus der
  Ferienanmeldung“.
- Weiter gilt: keine Gesundheitsangaben, keine Einwilligungen, keine
  E-Mail-Adresse, keine Anschrift.
- **Löschung wie bei den Kontakten.** Die Antworten folgen Punkt 5: sieben
  Tage nach der Woche nur lesbar, dann löscht der Abgleich sie mit den
  Kontakten. Eine Stornierung oder Umbuchung löscht sie beim nächsten
  Abgleich.
- **Prüfung beim Lesen.** Die Kopie liest ihre Antworten nur über
  `enrollment.holiday_guest_shared_answers`. Die Funktion prüft jede Antwort
  noch einmal gegen alle Formularversionen des Zeitraums, mit denselben Regeln
  wie die Teilnehmerliste. Stuft der Träger eine Frage später als
  Gesundheitsangabe ein, ist die kopierte Antwort sofort verborgen, auch in den
  sieben Tagen nur lesend, in denen kein Abgleich die Kopie mehr erneuert.
- **Register.** Migration `1.15.712` ergänzt die Spalte und ersetzt die
  Funktion; der Down-Pfad stellt die alte Signatur wieder her. Migration
  `1.15.713` ergänzt die Prüfung beim Lesen. Der Eintrag
  `enrollment.holiday_guests` im Datenkatalog nennt die Antworten. Keine neue
  Tabelle, kein neuer `data_objects`-Eintrag.
