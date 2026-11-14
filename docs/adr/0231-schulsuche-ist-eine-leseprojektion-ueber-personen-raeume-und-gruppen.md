---
status: accepted
---

# Schulsuche ist eine Leseprojektion über Personen, Räume und Gruppen

Entscheidung vom 2026-11-13 für
[#6954](https://github.com/moto-ogs/project-phoenix/issues/6954). Sie fügt den
Projektions-Owner `school-search-view` hinzu und erweitert Enrollment um die
öffentliche Lesefähigkeit `StudentAnswers`. Sie fügt keine Tabelle, keinen
Index und keine Schreibbefugnis hinzu, aber eine generierte Spalte: den
Suchschlüssel einer Telefonnummer (Punkt 3).

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
   Gemessen (Pläne und Endpunkt, [erste Messung](https://github.com/moto-ogs/project-phoenix/pull/7016#issuecomment-6863948202),
   [Vorher/Nachher](https://github.com/moto-ogs/project-phoenix/pull/7016#issuecomment-6873774013)): Bei Seed-Größe kosteten Planen,
   Ausführen und Roundtrips je etwa gleich viel, bei zehn- und
   fünfzigfacher Größe der Vergleich je Zeile. Nach Punkt 3 und 8 liegt p95
   bei Seed-Größe unter 10 ms, bei zehnfacher unter 30 ms, bei
   fünfzigfacher unter 100 ms.
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
3. **Die Telefonnummer hat einen gespeicherten Suchschlüssel.** Gespeicherte
   Nummern sind Freitext. Suche und Speicher werden auf dieselbe Ziffernfolge
   gebracht (`schoolsearchview.PhoneNumberKey`, die SQL-Funktion
   `public.f_phone_search_key` und `phoneSearchKey` im Frontend, mit denselben
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
   - **Der Schlüssel steht in der Zeile der Nummer:**
     `users.guardian_phone_numbers.search_digits` ist eine generierte Spalte
     über `public.f_phone_search_key(phone_number)` (Migration
     `20261114065713`). Die Datenbank füllt sie bei jedem Schreiben, kein
     Schreiber kann sie veralten lassen, und sie geht mit ihrer Zeile.
     Beim Lesen berechnet, kostete der Schlüssel drei reguläre Ausdrücke je
     Vergleich und war die teuerste Anweisung einer Suche. Die Tabelle
     bleibt bei People Directory; die Projektion liest die Spalte über ihre
     bestehende Lesegrant. Wer die Lesart der Funktion ändert, schreibt die
     Spalte in derselben Migration neu.
   Nur Kontaktpersonen werden über diese Folge gesucht; alle anderen Felder
   vergleichen den Text wie eingegeben, damit Ziffern auch einen Raum „1012“
   finden.
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

8. **Eine Suche ist eine Anweisung, und kein Index trägt den Textvergleich.**
   Die Projektion liest alle Arten, die der Scope öffnet, in einer
   Anweisung; eine Art außerhalb des Scopes ist darin abgeschaltet und liest
   keine Zeile. Die Kinder der
   Schule werden darin einmal verbunden und von Kindertreffern, Kontakten und
   deren Kindern gelesen. Dazu kommen die zwei Lesungen von Enrollment und
   eine für die Kinder der Antworttreffer, nur wenn Plätze frei sind.
   - **Vorfilter:** Der Vergleich je Zeile (`f_unaccent` und `ILIKE`) ist,
     was eine Suche in einer großen Schule kostet. Eine Zeile wird deshalb
     erst als ein zusammengesetzter Text verglichen und nur bei Erfolg Feld
     für Feld. Der Vorfilter verbirgt keinen Treffer: Die Trenner (Leerzeichen,
     Zeilenumbruch) gehören zu keiner `unaccent`-Regel und beenden für
     `lower` ein Wort wie das Feldende; Felder mit der Kollation
     `public.german_name` und Felder der Standardkollation werden getrennt
     zusammengesetzt, weil `ILIKE` nach der Kollation seines Textes
     kleinschreibt. `TestPrefiltersKeepEveryHit` vergleicht beide Lesarten.
   - **Kein Trigramm-Index:** Gemessen nutzt der Planer einen
     `pg_trgm`-Index auf `f_unaccent(...)` unter RLS nicht, auch nicht mit
     abgeschaltetem Seq Scan: `ILIKE` ist nicht leakproof und darf deshalb
     nicht vor der Mandanten-Policy als Indexbedingung laufen. Ohne RLS
     (Superuser) braucht dieselbe Abfrage 0,1 ms statt 19 ms. Ein Index, den
     keine Anfrage der App nutzen kann, wird nicht angelegt. Dasselbe gilt
     für einen Index auf dem Telefon-Schlüssel (`strpos`, `LIKE`).
   - **Anmeldeantworten bleiben in Go gefiltert.** Ein Filter in SQL müsste
     Fragen, Auswahltexte, Zahl- und Datumsformat der Formularversion und
     „die neueste Antwort je Kind und Frage“ nachbilden, und
     `FoldForSearch` ist kein exakter Zwilling von `f_unaccent`. Enrollment
     faltet stattdessen jeden Text nur einmal je Suche.
   Der nächste Hebel für sehr große Schulen wären gespeicherte, gefaltete
   Suchspalten für Namen und Notizen (generiert wie der Telefon-Schlüssel).
   Sie sind nicht umgesetzt: Sie beträfen mehrere Tabellen mehrerer Owner und
   sparen bei Seed-Größe unter einer Millisekunde.

## Nicht entschieden

Gesundheitsangaben liegen versiegelt in der Kinderakte (#5472) und sind ohne
Entschlüsseln aller Kinder nicht durchsuchbar. Notizkarten haben pro Kind
eine eigene Zielgruppe (Gruppe, Aktivität, Klasse des Lesenden). Beide sind
nicht Teil dieser Suche; eine Erweiterung braucht eine eigene Entscheidung.

## Nachweis

Die Tests der Projektion prüfen den Abgleich von Telefonnummern in Go, in der
SQL-Funktion und in der gespeicherten Spalte mit denselben Fällen, die
Vorfilter gegen die Vergleiche, vor denen sie stehen, Scope und
Mandantentrennung. Das Query-Budget `modules.schoolsearchview.search` hält
die Suche bei genau zwei Anweisungen der Projektion. Enrollments Tests prüfen
Antworttexte, Filter und Suche. `scripts/backend-architecture.sh check`
bestätigt Grants, Regeln und die fehlenden Pass-throughs.
