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

## Nachtrag 2026-11-01: Informationsfluss statt Datensilos

Der [Nachtrag zu ADR 0116](0116-ausrichtende-ogs-liest-ihre-teilnehmer-in-der-schul-transaktion.md#nachtrag-2026-11-01-informationsfluss-statt-datensilos)
ersetzt auch hier „keine Gesundheitsangaben“ aus Punkt 4 und dem Nachtrag
vom 28.10.2026 als dauerhaftes Produktziel. Die für die Betreuung benötigten
Angaben aus der Ferienanmeldung sollen dem berechtigten Team am Feriengast
im Betreuungsalltag zur Verfügung stehen, statt nur beim Träger zu bleiben.

Die Umsetzung läuft mit
[#4722](https://github.com/moto-nrw/project-phoenix/issues/4722). Die dort
unterschiedenen Fälle Feriengast und eigenes Kind der ausrichtenden OGS
bleiben getrennt: Angaben für eine Ferienbetreuung überschreiben nicht
stillschweigend die Stammdaten eines eigenen Kindes. Rechtsgrundlage,
Zugriffsschutz und die bestehende Befristung gelten auch für diese Angaben.
Dieser Dokumentationsnachtrag ändert noch keine Kopie, Lesefunktion oder
Löschroutine; die bisherigen Gesundheitsfilter bleiben bis zur
nachgewiesenen Umsetzung wirksam.

## Nachtrag 2026-11-01: Gesundheitsangaben des Feriengasts (#4722)

Mit der Entscheidung des Nutzers vom 01.11.2026 („keine Datensilos“,
Nachtrag zu ADR 0116) gilt Punkt 4 und der Nachtrag vom 28.10.2026 mit
dieser Änderung:

- **Gesundheitsfeld.** Die Kopie trägt die Gesundheitsangaben, die die
  Teilnehmerliste liefert, im Gesundheitsfeld des Kindes (`health_info`),
  geschrieben über den Enrollment-Befehl des People Directory
  (`ApplyEnrollmentProfile`, wie die übrige Kopie). Das Team sieht sie dort,
  wo es alle Kinder sieht: Kinderkarte, Gruppe, Gesundheitsliste. Ein
  späterer Abgleich schreibt das Feld nur neu, wenn sich die Angaben der
  Anmeldung ändern, und überschreibt sonst nicht, was das Team einträgt.
- **Blatt.** `GET /api/holiday-guests/{studentId}` zeigt die Angaben unter
  „Gesundheit“. Die Kopie hält sie verschlüsselt in
  `enrollment.holiday_guests.health_data`, gebunden an Schule und Kind.
- **Ende mit der Woche.** Die Gesundheitsangaben sieht die Schule wie die
  Teilnehmerliste nur bis zum letzten Tag der Woche (Nachtrag zu ADR 0116).
  Ab dem Tag danach zeigen Blatt, Kindakte, Gesundheitsliste und
  Notfallliste sie nicht mehr (People Directory liest das Gesundheitsfeld
  eines Kindes der Klasse `Feriengast` nach seinem Betreuungsende leer, auch
  vor dem Abgleich), und der stündliche
  Abgleich löscht Kopie und Gesundheitsfeld; die sieben Tage nur lesend aus
  Punkt 5 gelten für Kontakte und Antworten, nicht für Gesundheitsangaben.
  Eine Stornierung oder Umbuchung beendet den Gast und löscht beides beim
  nächsten Abgleich.
- Weiter gilt: keine Einwilligungen, keine E-Mail-Adresse, keine Anschrift.
  Ohne Einwilligung der Version 2 bekommt der Gast keine Gesundheitsangaben.
- Die Verschlüsselung des Gesundheitsfelds für alle Kinder der Schule ist
  nicht Teil dieses Nachtrags; die Kopie hat denselben Schutz wie die
  eigenen Kinder der Schule.

## Nachtrag 2026-11-03: Heimatschule im Alltag, Gast erst ab der Ferienwoche (#5630)

Entscheidung des Product Owners nach der Rückmeldung der Gemeinde vom
03.11.2026 ([#5630](https://github.com/moto-nrw/project-phoenix/issues/5630)):
Das Team der ausrichtenden OGS arbeitet mit Ferienkindern wie mit eigenen
Kindern, ein Hinweis nennt die Heimatschule, und die Kinder sind nur in der
Ferienzeit da. Punkt 2 und Punkt 4 gelten mit diesen Änderungen:

- **Zu Punkt 2: erst ab dem ersten Tag der Woche.** Vor dem ersten Tag
  steht ein Feriengast in keiner Live-Liste und nicht in der Kinderliste:
  Kinderliste, Tablet-Liste, Armband-Zuordnung, Kalenderauswahl und die
  Gruppenlisten lassen ein Kind der Klasse `Feriengast` weg, dessen
  `enrolled_from` nach dem Kalendertag des Lesers liegt (People Directory
  `holidayGuestArrived`, neben `currentCare`). Wie beim Ende nach der Woche
  zählt das Betreuungsintervall, nicht der Status, damit die Listen nicht auf
  den stündlichen Abgleich warten. Die laufende Ansicht der Kinderliste
  (`GET /api/students`) überlässt das Ende der Betreuung der Teilnahme von
  Care Plan und fragt das People Directory deshalb ohne Zeitgrenze; sie
  nennt den Tag der Liste im Filterfeld `HolidayGuestsArrivedOn`. Ein Kind,
  das gerade anwesend ist, bleibt dort stehen; die Ansichten aller und
  beendeter Betreuungen bleiben unbeschränkt. Vor der Woche sieht die
  Leitung den Gast nur auf der Seite „Ferienwochen“. Eigene Kinder mit
  späterem Start bleiben, wie sie sind. Ein Test vor der Änderung hat gezeigt, dass ein wartender
  Gast vorher in Kinderliste und Tablet-Liste stand.
- **Zu Punkt 4: Name der Heimatschule an der Kopie.** Die Kopie trägt neben
  `home_school_id` und `guest_school_name` den Namen der Heimatschule
  (`enrollment.holiday_guests.home_school_name`, Migration `1.15.740`). Der
  Abgleich liest die Namen der Schulen des Trägers über den vorhandenen Port
  `HolidayHostSchoolNames` (Organisation & Tenancy, Query, wie die
  Teilnehmerliste aus ADR 0116) in der Tenant-Transaktion der Schule, nur
  wenn ein Gast eine Heimatschule hat, und erneuert ihn bei jedem Abgleich.
  Kein neuer Lesepfad über Owner und keine neue Regel in `policy.json`: Die
  Bindung zieht von `api` nach `roots/owners`, damit auch der Worker sie hat.
  Gelöscht wird der Name mit der Kopie, wie `guest_school_name`.
- **Hinweis „Gast aus …“.** `GET /api/holiday-guests` (Recht
  `students:read`, das Recht der Kinderliste) nennt für jeden Gast, den die
  Schule heute betreut, die Schule des Hinweises; das Blatt nennt sie als
  `school_name`. Die Regel ist die der Teilnehmerliste: Heimatschule des
  Trägers, sonst die von den Eltern genannte Schule, sonst „Feriengast“.
  Gruppe, Check-in, Kinderliste, Aufsicht und Kindakte zeigen ihn. Die Route
  liest nur die Kopie, keine Tabelle des Trägers. Sie gibt nur den Namen
  einer Schule zu einem Kind heraus, das das Team in derselben Liste schon
  mit Klasse `Feriengast` sieht, und schreibt deshalb wie die Kinderliste
  keinen Eintrag ins Zugriffsprotokoll.
- **Rechtsgrundlage und Empfänger.** Die Heimatschule gehört nach Punkt 4 zu
  den Mindestfeldern, die der Träger der ausrichtenden OGS für die Betreuung
  in der Ferienwoche weitergibt. Der Hinweis zeigt sie dem Team, das das Kind
  in dieser Woche betreut, und nur während der Woche. Andere Schulen, auch
  des eigenen Trägers, sehen ihn nicht.
- **Kiosk.** PyrePortal zeigt den Hinweis nicht; ein Gast hat kein Armband
  und wird im Browser an- und abgemeldet. Ein Hinweis am Tablet folgt als
  eigenes Issue, falls die ausrichtende OGS das Tablet nutzt.

## Nachtrag 2026-11-07: Vorgemerkte eigene Kinder erst ab Betreuungsbeginn (#6041)

Rückmeldung der Grundschule Barnstorf vom 06.11.2026
([#6041](https://github.com/moto-nrw/project-phoenix/issues/6041)): Eltern
melden im Dezember für die Betreuung ab Februar an, und bestätigte Kinder
standen sofort in den laufenden Listen. Für die Teilnahme von Care Plan gilt
der Satz „Eigene Kinder mit späterem Start bleiben, wie sie sind“ aus dem
Nachtrag vom 03.11.2026 nicht mehr:

- Ein vorgemerktes Kind (Status `pending`), dessen `enrolled_from` nach dem
  Tag der Liste liegt, nimmt an diesem Tag nicht an der Betreuung teil.
  Care Plan entscheidet das in `ParticipatingOn`
  (`modules/careplan/internal/domain`), also für jede Leserin der
  Teilnahme: laufende Kinderliste mit Zählung und Export
  (`GET /api/students`, `POST /api/students/export`), Gruppenliste und
  Live-Ansicht der Gruppe, Klassenliste, geplante Betreuungstage mit ihren
  Zählern und die Fehlzeitenliste.
- Maßgeblich ist der Betreuungsbeginn, nicht die Anmeldefrist: Ab
  `enrolled_from` zählt das Kind, auch bevor der stündliche Abgleich es
  aktiviert, und auch an einem ausdrücklich gewählten Planungstag ab dem
  Beginn. Die Kinderverwaltung (`include_pending_withdrawals`,
  `AdministrativelyVisibleOn`), die Ansicht aller Kinder und die
  Anmeldeverwaltung zeigen es weiter, damit die Schule den Start vorbereitet.
- Ein sofort aktiviertes Kind (Status `active`) zählt weiter vor seinem
  regulären Beginn, wie bei der Stichtagszahl. Eine laufende
  Wiederanmeldung bleibt aktiv und zählt. Ein tatsächlich anwesendes Kind
  bleibt stehen. Ein vorgemerktes Kind ohne Beginn blendet die Regel nicht
  aus. Das Ende der Betreuung und der Feriengast bleiben, wie oben
  beschrieben.
- Die Live-Listen des People Directory, die ohne Teilnahme von Care Plan
  lesen (`currentCare`), ändert dieser Nachtrag nicht; ob vorgemerkte
  Kinder dort fehlen sollen, klärt
  [#6050](https://github.com/moto-nrw/project-phoenix/issues/6050); die
  Antwort steht im Nachtrag vom 08.11.2026.

### Vergangene Tage: neues und zurückkehrendes Kind (#6041, Review)

Eine Wiederaufnahme (`ResumeStudentCare`) und eine Wiederanmeldung
(`RenewStudentEnrollment`) speichern ein bekanntes Kind wie ein neues:
`pending`, neues `enrolled_from`, früherer Beginn ersetzt. Hielte Care Plan
jedes vorgemerkte Kind an allen Tagen vor dem Beginn zurück, verschwände die
echte frühere Betreuung eines zurückkehrenden Kindes aus der Fehlzeitenliste.
Hielte es nur ab heute zurück, stünde ein im Dezember bestätigtes Kind mit
Start im Februar in der Fehlzeitenliste des laufenden Halbjahres, mit
„nicht erfasst“-Tagen und in Summen und Export, und die Antwort für einen Tag
hinge davon ab, an welchem Tag man ihn liest.

Kein vorhandenes Datum trennt die beiden verlässlich: Den Aktivierungsplan
der Anmeldung stempelt auch eine Wiederanmeldung, die Wiederaufnahme
berührt ihn nicht; `created_at` deckt die Tage zwischen Anlage und heute
nicht ab; fehlende Anwesenheitsdaten beweisen keine fehlende Betreuung.

Darum merkt sich School Membership den frühesten Tag, an dem ein neues Kind
betreut worden sein kann: `users.student_school_memberships.first_care_from`
(Migration 1.15.812,
[ADR 0166](0166-mitgliedschaft-merkt-sich-den-fruehesten-betreuungstag.md)).
Eine neue Genehmigung mit späterem Start (`pending` mit Beginn, keine andere
auch gelöschte Mitgliedschaft) setzt ihn auf den Beginn. Die Schreibvorgänge
von School Membership führen ihn am Tag der Anwendung nach: Ein erreichter
Tag rückt nur noch früher, die Aktivierung am Starttag und eine
Wiederaufnahme behalten ihn, eine Aktivierung vor dem Beginn setzt ihn auf
diesen Tag, unbekannt bleibt unbekannt. Der Nachlauf der Migration setzt ihn
nur für Zeilen, die seit dem Einfügen nachweislich unverändert sind.

Care Plan liest ihn über die Schülerprojektion (`CareStudent.FirstCareFrom`)
in denselben Statements. `AwaitsCareStartOn` hält ein Kind an jedem Tag vor
`first_care_from` zurück, in jedem Status, also auch nach der Aktivierung;
ohne diesen Tag ein vorgemerktes Kind vor seinem Beginn nur ab heute. Ein
zurückkehrendes oder unbekanntes Kind behält seine vergangenen Tage. Echte
Betreuung verschwindet nie: Die Fehlzeitenliste zählt ein zurückgehaltenes
Kind an einem Tag mit, an dem Student Presence eine Anwesenheit oder einen
Raumbesuch gespeichert hat, wie die laufenden Listen ein tatsächlich
anwesendes Kind behalten. Die Lücke zwischen früherem Ende und
Wiederaufnahme eines zurückkehrenden Kindes hält Care Plan seit
[#6073](https://github.com/moto-nrw/project-phoenix/issues/6073) ebenso
zurück ([ADR 0180](0180-mitgliedschaft-merkt-sich-die-betreuungsluecke-eines-zurueckkehrenden-kindes.md)).

## Nachtrag 2026-11-08: Live-Listen des People Directory behalten vorgemerkte eigene Kinder (#6050)

Entscheidung zu
[#6050](https://github.com/moto-nrw/project-phoenix/issues/6050): Die
Live-Listen des People Directory, die ohne Teilnahme von Care Plan lesen
(`currentCare`), zeigen ein vorgemerktes eigenes Kind (Status `pending`,
`enrolled_from` in der Zukunft) weiter vor seinem Beginn. Der Satz „Eigene
Kinder mit späterem Start bleiben, wie sie sind“ aus dem Nachtrag vom
03.11.2026 gilt für diese Leser fort. Keine dieser Listen zählt Betreuung
oder ist eine laufende Betreuungsliste; jede dient der Vorbereitung oder dem
Auflösen schon gespeicherter Daten:

- **Armband-Zuordnung am Kiosk.** `FindAllWithGroups` und
  `FindByTeacherStaffIDsWithGroups` speisen `GET /api/iot/students`, die
  Kinderauswahl, mit der PyrePortal ein Armband zuordnet. Das Team gibt das
  Armband vor dem ersten Tag aus, nicht erst am ersten Morgen.
- **Empfängerauswahl im Kalender.** `FindAllWithGroups` liefert die Suche
  nach Kindern für Kalendereinträge. Eltern eines Kindes mit späterem Start
  brauchen vorher Informationen, etwa zum ersten Tag oder zu einem
  Elternabend. In einer Suche stört der Eintrag nicht.
- **Elternsprechtag.** `FindAllWithGroups` nennt nur die Namen schon
  gebuchter Kinder. Fehlte das Kind dort, stünde eine bestehende Buchung
  ohne Namen.
- **Rückmeldungen der Anmeldephase.** `ListStudentRoster` markiert ein
  vorgemerktes Kind als `NotActive`; Enrollment fragt es nicht nach einer
  Wiederanmeldung, weil seine eigene Anmeldung noch läuft. Keine Änderung.
- `FindByTeacherIDWithGroups` hat keinen Leser außerhalb von Port und
  Dekorator.

Feriengast und Ende der Betreuung bleiben, wie oben beschrieben. Zeigt sich,
dass vorgemerkte Kinder die Armband-Auswahl am Kiosk unübersichtlich machen,
bekommen sie dort einen Hinweis auf ihren Beginn statt einer Ausblendung;
das ändert den Kiosk-Vertrag mit PyrePortal und braucht ein eigenes Issue.

## Nachtrag 2026-11-11: Die Buchung plant die Tage, nicht das Konto der Gruppenwahl

Ersetzt in Punkt 2 „Urheber die Person des Kontos, das die Gruppe gewählt
hat“ und „Hat dieses Konto keinen Personaleintrag (mehr), kommt der Gast
trotzdem in die Gruppe, und nur seine Tage bleiben offen“.

Auf Staging wählte ein Konto die Gruppe, das nur Mitglied einer anderen
Schule desselben Trägers ist und in der ausrichtenden Schule keinen
Personaleintrag hat. Die drei Gäste kamen in die Gruppe, die Wahl meldete
Erfolg, aber keiner ihrer Tage stand in der Tagesplanung. Richtet das
Trägerbüro die Woche für die Schule ein, ist das der Normalfall.

1. **Die Buchung ist Urheber.** Enrollment trägt die gebuchten Tage mit der
   eigenen Quelle `holiday_booking` und ohne Personal-Urheber ein
   (`created_by` bleibt leer, `created_by_guardian` auch); Grund und Zeiten
   bleiben wie in Punkt 2. Migration 1.15.923 erweitert `chk_exception_source`
   und `chk_exception_author` der Tabellen
   `schedule.student_arrival_exceptions` und
   `schedule.student_pickup_exceptions` um diese Quelle. Wer die Gruppe
   gewählt hat, steht weiter im Zugriffsprotokoll der Übertragung.
2. **Die Schule entscheidet den Tag.** Wie eine Zeile des Personals
   überschreibt keine Änderung von Eltern einen solchen Tag, und eine
   genehmigte Abholänderung meldet dort einen Konflikt
   (`careplan.SchoolOwnedException`); das Elternportal zeigt ihn als Tag der
   Schule. Ändert das Team den Tag im Betreuungsplan, bleibt die Quelle
   `holiday_booking`.
3. **Der stündliche Abgleich plant nach.** Hat ein Gast mit gewählter Gruppe
   und gültiger Buchung keine einzige An- oder Abholausnahme, plant der
   Abgleich (`holiday-guest-reconciliation`) seine Tage, ohne dass jemand die
   Gruppe neu wählt. Die Gäste, die vor diesem Nachtrag ohne Tage kopiert
   wurden, bekommen sie so beim nächsten Lauf. Löscht das Team alle Tage
   eines Gasts, plant der nächste Lauf sie ebenso neu; einen Tag ohne Gast
   trägt das Team als Abwesenheit ein. Ein Gast mit mindestens einem
   geplanten Tag behält seinen Plan; neu geplant wird er weiter nur bei einer
   Gruppenwahl oder geänderter Buchung, und die Woche ohne Gruppe behält
   ihren Plan bis zur nächsten Wahl. Ein beendeter Gast verliert seine Tage
   wie bisher (`ClearHolidayGuestDays`).

## Nachtrag 2026-11-11: Feriengruppe vor ihrer Woche

Rückmeldung aus dem Test auf Staging: Eine Schule legte im November die
Gruppe „Ferienbetreuung Januar 2027“ für eine Woche vom 04.01. bis
08.01.2027 an und wählte sie als Feriengruppe. Weil die Feriengäste nach dem
Nachtrag vom 03.11.2026 erst ab dem ersten Tag der Woche in der Gruppe
stehen, zeigte die Gruppenübersicht acht Wochen lang eine leere Gruppe ohne
Erklärung. Der Nachtrag vom 03.11.2026 bleibt unverändert; dazu gilt:

- **Tageslisten.** Die Gruppenlisten von Group Live, also die
  Gruppenauswahl der Seite „OGS-Gruppen“ mit ihrer Navigation
  (`GET /api/students/ogs-group-navigation`) und die Reiter der
  Live-Ansicht (`GET /api/students/ogs-group-live`), lassen eine Gruppe weg,
  solange sie die Feriengruppe einer ausgerichteten Woche ist, die nach dem
  Kalendertag des Lesers beginnt, sie die Feriengruppe keiner heute
  laufenden Woche ist und kein Kind der Gruppe heute an der Betreuung
  teilnimmt (Teilnahme von Care Plan, dieselbe Regel wie der Gruppenkader).
  Ab dem ersten Tag der Woche oder sobald ein Kind der Gruppe heute betreut
  wird, steht sie wieder da. Eine eigene Gruppe mit Kindern, die auch
  Feriengruppe ist, bleibt deshalb immer sichtbar. Ein Link mit der Gruppe
  (`group_id`) öffnet sie weiter.
- **Nach der Woche.** Die Regel kennt nur Wochen, deren letzter Tag heute
  oder später liegt. Nach der letzten Woche ist die Gruppe wieder eine
  gewöhnliche Gruppe und erscheint, auch leer. Die Leitung löscht sie oder
  nutzt sie für die nächste Woche; eine gelöschte Gruppe verliert ihre
  Wahl über den Fremdschlüssel. Das hält die Regel einfach und blendet nie
  eine Gruppe dauerhaft aus.
- **Verwaltung bleibt vollständig.** Gruppenverwaltung, Auswahl der
  Feriengruppe, Dienst- und Betreuungsplan und Personalzuordnung lesen die
  Gruppen weiter über School Structure und zeigen sie immer, damit die
  Leitung Team und Raum vor der Woche zuordnet. `GET /api/holiday-guests/groups`
  (Recht `groups:read`, das Recht der Gruppenlisten) nennt jede Feriengruppe
  mit Woche und Zahl der Feriengäste; die Gruppenverwaltung zeigt daraus den
  Hinweis „Feriengruppe 04.01.–08.01.2027 · 3 Feriengäste ab 04.01.“.
- **Grenzen.** Enrollment besitzt die Wahl und liest sie in der
  Tenant-Transaktion der Schule über die öffentliche Capability
  `HolidayGuestGroupList` (eine Abfrage für die ganze Schule). Group Live
  entscheidet die Regel über eigene Ports (`HolidayGroups`,
  `Roster.GroupMemberIDs`); die Wurzel bindet sie (ADR 0067). Die Kinder
  der wartenden Gruppen liest es nur, wenn eine Gruppe wartet, gesammelt in
  einer Abfrage. Für Leser mit Überblick über alle Gruppen liest Group Live
  die eigenen Gruppen nur noch als IDs aus der Identitätskette; so bleiben
  die Abfrage-Budgets der Gruppenlisten gleich.
- **Kiosk und Aufsicht.** Der Kiosk (`/api/iot/…`) listet keine
  Betreuungsgruppen, die Aufsicht nur die eigenen Gruppen der Person für die
  Raumzuordnung; beide bleiben unverändert.
