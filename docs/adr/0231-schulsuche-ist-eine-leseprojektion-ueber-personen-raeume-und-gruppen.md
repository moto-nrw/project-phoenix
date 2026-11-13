---
status: accepted
---

# Schulsuche ist eine Leseprojektion über Personen, Räume und Gruppen

Entscheidung vom 2026-11-13 für
[#6954](https://github.com/moto-ogs/project-phoenix/issues/6954). Sie fügt den
Projektions-Owner `school-search-view` hinzu und erweitert Enrollment um die
öffentliche Lesefähigkeit `StudentAnswers`. Sie fügt keine Tabelle, keinen
Index und keine Schreibbefugnis hinzu.

## Kontext

Das Büro einer OGS will zu einer Telefonnummer die Familie finden, die
Leitung Kinder nach Angaben aus der Online-Anmeldung („Seepferdchen
vorhanden“). Beides sind Suchen über Daten mehrerer Owner: Namen und Klassen
liegen bei People Directory und School Membership, Gruppen bei School
Structure, Räume bei Facilities, Kontaktpersonen und Telefonnummern bei People
Directory, die Antworten der Anmeldung bei Enrollment. Statt zwei
Einzellösungen in „Alle Kinder“ entsteht eine Suche über die ganze Schul-App.

Drei Fragen waren offen:

1. **Kommen die Antworten eigener Formularfelder in der Kinderakte an?** Nein.
   Ein Feld ohne Ziel (`FormField.Target == ""`) bleibt in
   `enrollment.request_children.custom_data`. Die Genehmigung kopiert nur
   Felder mit Ziel auf die Kinderakte. Die Kinderakte liest die übrigen
   Antworten schon heute live über die Verknüpfung `created_student_id`
   (`GET /students/{id}/enrollment-extra-fields`, Recht `users:read`,
   Büro-Zuordnungen ausgenommen).
2. **Reichen Abfragen auf den bestehenden Tabellen?** Ja. Eine Schule hat
   einige hundert Kinder, Kontaktpersonen und Telefonnummern. Jede Abfrage
   filtert auf die Schule und begrenzt ihr Ergebnis; RLS gilt zusätzlich.
   Gemessen (Pläne und Endpunkt, [Messung im PR](https://github.com/moto-ogs/project-phoenix/pull/7016#issuecomment-6863948202)):
   bei zehnfacher Seed-Größe p95 höchstens 64 ms, bei fünfzigfacher bis
   200 ms, weil Namen, Notizen und Nummern je Zeile verglichen werden.
3. **Welcher Owner stellt die Suche bereit?** Keiner der Domänen-Owner: die
   Abfragen verbinden Tabellen mehrerer Owner in einer Anweisung.

## Entscheidung

1. **Die Leseprojektion `school-search-view`** (`modules/schoolsearchview`)
   stellt `POST /api/search` bereit. Sie besitzt keine Daten und schreibt
   nichts. Ihre Lesegrant nennt genau `users.persons`,
   `users.student_profiles`, `users.student_school_memberships`,
   `education.groups`, `users.guardian_profiles`,
   `users.guardian_phone_numbers`, `users.student_guardian_relationships`,
   `users.staff_school_memberships` und `facilities.rooms`
   (ADR 0066, Punkt 6). Sie liest in der Schul-Transaktion der Anfrage, jede
   Anweisung filtert zusätzlich auf deren Schule.
2. **Die Anmeldeantworten bleiben bei Enrollment.** Enrollment bietet
   `StudentAnswers` an: Filterfragen mit Antworten und den Kindern je
   Antwort, die Kinder zu einer Antwort, und eine Textsuche über Frage und
   Antwort. Enrollment kennt nur die Verknüpfung, nicht, wer noch an der
   Schule ist: Die Kinderliste zählt je Antwort die Kinder, die sie selbst
   zeigt (laufende Betreuung, ohne Ehemalige), damit Zahl und Liste gleich
   sind. Die Suche fragt alle Treffer ab und nimmt die ersten Kinder, die
   noch an der Schule sind. Gelesen werden
   dieselben Antworten wie auf der Kinderakte (neueste Anmeldung je Kind und
   Frage). Gezählt werden nur freie Kinderfragen der Typen Ja/Nein, Auswahl,
   Text, Zahl und Datum. Fragen mit Ziel stehen schon in der Kinderakte;
   Dateien und strukturierte Typen tragen keinen Antworttext. Die Projektion
   und die Kinderliste (`presence-legacy-view`) binden die Fähigkeit über
   eigene Ports; die Root verdrahtet sie. Eine Kopie der Antworten in die
   Kinderakte oder eine Projektion mit eigenem Speicher würde eine zweite
   Quelle schaffen, die bei Korrekturen und Löschungen mitgepflegt werden
   müsste.
3. **Die Telefonnummer wird beim Lesen normalisiert.** Gespeicherte Nummern
   sind Freitext. Suche und Speicher werden auf dieselbe Ziffernfolge
   gebracht (`schoolsearchview.PhoneNumberKey`, die gleichlautende
   SQL-Konstante und `phoneSearchKey` im Frontend, mit denselben
   Testfällen): Der Text wird auf Ziffern, Plus und Klammern verdichtet,
   eine eingeklammerte Null fällt weg, gleich welche Trenner in der Klammer
   standen; dann fallen 00, die 49 einer internationalen Nummer und die
   deutsche Verkehrsausscheidungsziffer weg. Jedes Unicode-Leerzeichen und
   jeder Strich trennt, weil kopierte Nummern aus Signaturen und Telefon-Apps
   sie enthalten. Eine Suche braucht danach mindestens vier Ziffern.
   Teilnummern treffen als Teil der gespeicherten Folge.
   - **Eine 49 ohne + oder 00 bleibt stehen**, bei gespeicherten Nummern
     immer: „4921 1234567“ ist ebenso oft Emdens 04921 ohne 0 wie eine
     Ländervorwahl. Nur eine Suche, deren Ziffern so mit 49 beginnen und
     mindestens elf Ziffern haben, sucht zusätzlich mit der Folge ohne 49
     (`PhoneSearchAlternativeKey`). Was die unveränderte Folge findet, bleibt
     gefunden.
   - **Durchwahl nur als Rückfall:** Findet keine Nummer eines Kontakts eines
     Kindes der Schule die Suche direkt, liest die Abfrage die Suche als
     gespeicherte Nummer mit Durchwahl (mindestens sieben gespeicherte
     Ziffern, ein bis fünf mehr) und nimmt nur die längste passende
     gespeicherte Nummer. Eine exakte Nummer bringt so keine kürzere Nachbarin
     mit.
   Nur Kontaktpersonen werden über diese Folge gesucht; alle anderen Felder
   vergleichen den Text wie eingegeben, damit Ziffern auch einen Raum „1012“
   finden. Eine generierte Spalte oder ein Funktionsindex bräuchte eine
   Migration im Owner People Directory und lohnt erst bei Schulen jenseits der
   gemessenen zehnfachen Größe.
4. **Die Suche zeigt nur, was das Konto ohne Suche lesen darf.** Die Route
   leitet aus den Rechten einen Scope ab und sucht nur in den Arten und
   Feldern, die er öffnet: Kinder mit `users:read` oder `students:read`;
   Elternnotizen, Anmeldeantworten und Kontaktpersonen mit Telefon und
   E-Mail nur mit `users:read`; beides nur für Personal der Schule oder
   Admins (#2329); Mitarbeitende mit dem Recht der Personalliste, Räume mit
   `rooms:read`, Gruppen mit `groups:read` und einem Kinder-Leserecht, weil
   ein Gruppen-Treffer „Alle Kinder“ mit dieser Gruppe öffnet. Ohne Kontaktrecht sucht die
   Projektion keine Telefonnummer; die Antwort hat dieselbe Form mit leeren
   Listen. Die Antwort nennt den Scope des Kontos, damit der Dialog seine
   Hinweise aus der Entscheidung des Servers ableitet; der Scope hängt nur
   vom Konto ab, nicht von den Treffern. Der Filter nach Anmeldeangaben, seine
   Auswahl und der gefilterte Export verlangen dasselbe wie die Angaben auf
   der Kinderakte: `users:read` und einen Personaleintrag der Schule, oder
   Admin. Archivierte Räume fehlen wie in der Raumliste. Eine Elternnotiz und
   eine lange Anmeldeantwort kommen als Ausschnitt um den Treffer,
   höchstens 120 Zeichen.
5. **Der Suchbegriff bleibt aus URLs und Protokollen.** Er reist im Body
   eines `POST`, nie als Query-Parameter. Fehler nennen die gescheiterte
   Lesung, nie den Text. bun schreibt gebundene Werte in das SQL, das das
   Abfrage-Log (langsame und fehlerhafte Anweisungen) erhält. Jede Anweisung,
   die den Text trägt, beginnt deshalb mit `protocol.RedactLiteralsInLog` von
   Transaction Runtime; das Log ersetzt dann alle ihre Textliterale. Der
   Suchdialog trägt `data-analytics-block`, eine Sitzungsaufzeichnung enthält
   ihn nicht, und kein Link des Dialogs trägt den Text. Offen bleibt das
   Statement-Log des Datenbankservers: Ist dort `log_min_duration_statement`
   oder `log_statement` gesetzt, sieht es das SQL mit eingesetzten Werten,
   wie bei jeder bestehenden Namenssuche. Das betrifft den Betrieb, nicht
   diese Projektion. Die Route ist eine Lesung und kein Kernvorgang der
   Nutzungsanalyse; die Mitarbeiter-Vorschau darf sie aufrufen.

6. **Der Suchdialog ist ein Kit-Baustein mit einem Verwender.**
   `CommandPalette` steht in `components/ui/`, obwohl ADR 0149 für neue
   Bausteine drei Verwender in zwei Bereichen verlangt. Eine lokale Lösung
   ist hier nicht möglich: Combobox, Listbox und Optionen dürfen nach der
   harten Regel `ui-kit/no-hand-rolled-menu` nur im Kit stehen, und kein
   bestehender Baustein trägt gruppierte Treffer mit Symbol und zweiter Zeile
   (`SuggestInput` kennt nur Textvorschläge, `ListboxDropdown` nur eine
   Auswahl hinter einem Auslöser). Der Baustein enthält keine Fachlogik: Er
   bekommt Gruppen, Text und Zustände über Props und lädt nichts. Diese
   Ausnahme gilt nur für ihn; ein zweiter Verwender nutzt ihn, statt eine
   weitere Liste zu bauen.
7. **Kompositionsbudget.** `modules/enrollment/compose` wächst um den
   Konstruktor von `StudentAnswers` (+5 Zeilen), `modules/schoolsearchview/compose`
   kommt mit 38 Zeilen hinzu; das Budget in
   `backend/test/module_legacy_budget_ratchet_test.go` nennt diese
   Entscheidung.

## Nicht entschieden

Gesundheitsangaben liegen versiegelt in der Kinderakte (#5472) und sind ohne
Entschlüsseln aller Kinder nicht durchsuchbar. Notizkarten haben pro Kind
eine eigene Zielgruppe (Gruppe, Aktivität, Klasse des Lesenden). Beide sind
nicht Teil dieser Suche; eine Erweiterung braucht eine eigene Entscheidung.

## Nachweis

Die Tests der Projektion prüfen den Abgleich von Telefonnummern in Go und SQL
mit denselben Fällen, Scope und Mandantentrennung. Enrollments Tests prüfen
Antworttexte, Filter und Suche. `scripts/backend-architecture.sh check`
bestätigt Grants, Regeln und die fehlenden Pass-throughs.
