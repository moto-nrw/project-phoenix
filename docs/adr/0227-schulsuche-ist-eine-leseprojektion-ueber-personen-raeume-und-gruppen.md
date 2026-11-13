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
   `StudentAnswers` an: Filterfragen mit Antworten und Anzahl, die Kinder zu
   einer Antwort, und eine Textsuche über Frage und Antwort. Gelesen werden
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
   sind Freitext. Suche und Speicher werden auf dieselbe Ziffernfolge ohne
   Verkehrsausscheidungs- und Ländervorwahl gebracht
   (`schoolsearchview.PhoneSearchKey` und die gleichlautende SQL-Konstante).
   Teilnummern treffen als Teil dieser Folge. Nur Kontaktpersonen werden
   über diese Folge gesucht; alle anderen Felder vergleichen den Text wie
   eingegeben, damit Ziffern auch einen Raum „1012“ finden. Eine generierte Spalte oder ein
   Funktionsindex lohnt bei einigen hundert Nummern je Schule nicht und
   bräuchte eine Migration im Owner People Directory.
4. **Die Suche zeigt nur, was das Konto ohne Suche lesen darf.** Die Route
   leitet aus den Rechten einen Scope ab und sucht nur in den Arten und
   Feldern, die er öffnet: Kinder mit `users:read` oder `students:read`;
   Elternnotizen, Anmeldeantworten und Kontaktpersonen mit Telefon und
   E-Mail nur mit `users:read`; beides nur für Personal der Schule oder
   Admins (#2329); Mitarbeitende mit dem Recht der Personalliste, Räume mit
   `rooms:read`, Gruppen mit `groups:read` und einem Kinder-Leserecht, weil
   ein Gruppen-Treffer „Alle Kinder“ mit dieser Gruppe öffnet. Ohne Kontaktrecht sucht die
   Projektion keine Telefonnummer; die Antwort hat dieselbe Form mit leeren
   Listen. Der Filter nach Anmeldeangaben in Liste und Export verlangt
   `users:read`.
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
