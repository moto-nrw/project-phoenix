# Löschfristen je Datenart

**Stand:** 15.10.2026\
**Quelle für:** Löschkonzept und TOM (Anlage zum AVV). Entscheidungen: [ADR 0048](../adr/0048-loeschfristen-traegt-das-moto-team-je-schule-ein.md). Begründungen der Fristen: [Recherche Aufbewahrungsfristen](../research/aufbewahrungsfristen-2026-10-12.md). Umsetzung: Spec [#3949](https://github.com/moto-nrw/project-phoenix/issues/3949).

Diese Liste nennt jede Datenart mit Personenbezug, ihre Löschfrist, den Startzeitpunkt, wer die Frist einstellt und welcher Owner im Code löscht. Der Wächtertest `TestEveryTableHasARetentionPath` (`backend/database/migrations/retention_coverage_test.go`) hält die Tabellen dazu im Code fest: Jede Tabelle hat einen Löschweg, eine begründete Ausnahme oder steht als offene Lücke mit Ticket. Die Lückenliste darf nur schrumpfen.

## Wie gelöscht wird

- **Stündlicher Token-Aufräumjob.** Einladungen für Eltern, MFA-Codes per E-Mail, Passkey-Sitzungen und abgelaufene Refresh-Tokens löscht der stündliche Aufräumjob für alle Schulen und Operatoren gemeinsam (#3954). Diese Läufe stehen nicht im Löschprotokoll, weil sie keiner Schule zugeordnet sind.
- **Nächtlicher Lauf je Schule.** Zur eingestellten Uhrzeit (`gdpr.data_cleanup_time`, Voreinstellung 02:00) löscht moto in einer Transaktion der Schule alles, was älter als die Frist ist. Wurde ein geplanter Lauf verpasst, weil der letzte erfolgreiche Lauf vor dem Tag des letzten geplanten Termins liegt, holt moto ihn beim nächsten Minutentakt nach; ein fehlgeschlagener Lauf zählt nicht und wird nachgeholt (#3950).
- **Löschprotokoll.** Jede automatische Löschung des nächtlichen Laufs schreibt einen Eintrag in `audit.data_deletions` mit Datenart und Anzahl, ohne die gelöschten Inhalte.
- **Wer die Fristen einstellt.** Fachliche Fristen trägt das moto-Team je Schule im Operator-Dashboard ein, so wie sie im Vertrag stehen; ohne Eintrag gilt die Voreinstellung. Die Schule sieht ihre Fristen unter „Datenschutz“ und kann sie nicht ändern. Technische Daten und Protokolle haben feste Fristen für alle Schulen. Den nächtlichen Lauf hält nur das moto-Team als Notbremse an.

## Fachliche Daten (je Schule, operator_managed)

Keine dieser Fristen ist für OGS-Daten gesetzlich vorgegeben; die Voreinstellungen sind begründete Praxis nach der [Recherche](../research/aufbewahrungsfristen-2026-10-12.md) (Abschnitte 1 bis 6) und vom Verantwortlichen im Vertrag zu bestätigen. Ausnahme: Zeiterfassung mindestens 2 Jahre nach § 16 Abs. 2 ArbZG. AU-Bescheinigungen sind Gesundheitsdaten (Art. 9 DSGVO); ein Jahr deckt die Zwölf-Monats-Frist für eine erneute Arbeitsunfähigkeit wegen derselben Krankheit (§ 3 Abs. 1 S. 2 Nr. 2 EFZG).

| Datenart | Einstellung | Voreinstellung | Rahmen | Startzeitpunkt | Owner | Stand |
|---|---|---|---|---|---|---|
| Raumbesuche ohne Einwilligung | `gdpr.privacy_consent_retention_days` | 30 Tage | 1 bis 31 Tage | Besuch | Student Presence | umgesetzt |
| Betreuungsplan (Termine, Teilnahme, Änderungsprotokoll) | `gdpr.timetable_retention_days` | 365 Tage | 30 Tage bis 5 Jahre | Termin | Timetable | umgesetzt |
| Krank-, Entschuldigt- und Klassenfahrt-Tage | `gdpr.status_day_retention_days` | 365 Tage | 30 Tage bis 3 Jahre | gemeldeter Tag | Care Plan | umgesetzt (#3958) |
| Elternnachrichten | `gdpr.parent_message_retention_days` | 365 Tage | 30 Tage bis 2 Jahre | Versand | Communication | umgesetzt (#3959) |
| Team-Chat | `gdpr.staff_message_retention_days` | 365 Tage | 30 Tage bis 2 Jahre | Versand | Communication | umgesetzt |
| Änderungsprotokoll pro Kind | `gdpr.student_change_log_retention_days` | 90 Tage | 30 bis 365 Tage | Änderung | People Directory | umgesetzt |
| App-Nutzung (Startbildschirm) | `gdpr.pwa_usage_retention_days` | 90 Tage | 30 bis 365 Tage | letzte Nutzung | Delivery | umgesetzt |
| Rückmeldungen | `feedback.data_retention_days` | 90 Tage | 7 bis 365 Tage | Eingang | Feedback | umgesetzt |
| Abgelehnte Anmeldungen | `enrollment.rejected_retention_days` | 90 Tage | 7 bis 730 Tage | Ablehnung | Enrollment | umgesetzt |
| Erledigte Anmeldungen ohne Kind (zurückgezogen, verfallen) samt Einladungen zur Nachmeldung | `gdpr.withdrawn_enrollment_retention_days` | 90 Tage | 30 bis 365 Tage | Erledigung; Einladung: Ablauf oder Einlösung | Enrollment | umgesetzt (#3960) |
| Zeiterfassung (Stempelzeiten, Pausen, Abwesenheiten) | `gdpr.time_tracking_retention_days` | 2 Jahre | 2 bis 8 Jahre | Arbeitstag | Workforce | umgesetzt |
| AU-Bescheinigungen (samt Datei) | `gdpr.staff_sick_note_retention_days` | 365 Tage | 90 Tage bis 3 Jahre | Hochladen | Workforce | umgesetzt (#3961) |
| Tagesanwesenheit | geplant | 365 Tage | 30 Tage bis 3 Jahre | Betreuungstag | Student Presence | offen (#3957) |
| Kinder nach Austritt (vollständig) | geplant | 365 Tage | 30 Tage bis 3 Jahre | letzter Betreuungstag | People Directory, Workflow Kinderlöschung | offen (#3962) |
| Ausgetretenes Personal | Zeiterfassungsfrist | wie Zeiterfassung | wie Zeiterfassung | Personalaustritt | Workforce, Workflow Personalaustritt | offen (#3963) |

Die Uhrzeit des nächtlichen Laufs (`gdpr.data_cleanup_time`) stellt die Schule selbst ein. Den Schalter `gdpr.data_cleanup_enabled` ändert nur das moto-Team (#3951).

## Technische Daten und Protokolle (fest für alle Schulen)

| Datenart | Frist | Startzeitpunkt | Owner | Begründung | Stand |
|---|---|---|---|---|---|
| Nicht registrierte Transponder-Scans | 90 Tage | Scan | Device Fleet | Prüfung durch das moto-Team, danach ohne Zweck | umgesetzt (#3950) |
| Anmeldeereignisse | 90 Tage | Ereignis | Audit Platform | BSI-Mindeststandard „bspw. 90 Tage“ für Sicherheitsprotokolle | umgesetzt (#3956) |
| Zugriffsprotokoll | Ende des Folgejahres | Zugriff | Audit Platform | Analogie § 76 Abs. 4 BDSG | umgesetzt (#3956) |
| Löschprotokoll | 3 Jahre | Löschung | Audit Platform | BayLDA: Nachweis bis 3 Jahre | umgesetzt (#3956) |
| Mail- und Push-Warteschlange, Zustellprotokolle | 30 Tage | Zustellung, endgültiger Fehlschlag oder Abbruch; Zustellprotokoll ohne Mail: Eintrag | Delivery | Inhalt ohne Zweck nach Zustellung | umgesetzt (#3955) |
| Einladungen für Eltern | 30 Tage | Einlösung, sonst Ablauf | Identity Access | Link ohne Zweck nach Ablauf oder Einlösung | umgesetzt (#3954) |
| MFA-Codes per E-Mail, Passkey-Sitzungen (Konten und Operatoren) | 7 Tage | Ablauf | Identity Access | IP-Bezug höchstens 7 Tage | umgesetzt (#3954) |
| Abgelaufene Refresh-Tokens, auch für Operatoren | höchstens 7 Tage; gelöscht bei Ablauf | Ablauf | Identity Access | Sitzung ohne Zweck nach Ablauf | umgesetzt (#3954) |
| Erledigte Aufräumaufträge für Dateien (Dateiablage, Anhänge von Elternmitteilungen, Personal- und Kinderdokumente) | 30 Tage | Löschung der Datei | File Storage, Workforce, Care Plan | Der Auftrag behält nur den Dateinamen; 30 Tage reichen, um einen Fehlschlag nachzuvollziehen. Offene Aufträge bleiben, bis die Datei weg ist | umgesetzt (#4046) |
| Erledigte Austrittsaufträge für Personal | 30 Tage | Abschluss des Auftrags | Workforce | Der Auftrag behält nur die Personal-ID; wie oben | umgesetzt (#4046) |
| Abgeschlossene Betreuungsabmeldungen | 365 Tage | Abschluss (erledigt oder hinfällig) | Care Plan | Die Abschluss-Historie zeigt das letzte Jahr; offene Abmeldungen bleiben, bis die Schule sie abschließt | umgesetzt (#4046) |
| Anmelde-Ratenbegrenzung (IP-Adresse oder E-Mail) | 1 Tag | Beginn des Zählfensters | Enrollment | Das längste Zählfenster ist 24 Stunden; danach zählt der Eintrag nichts mehr | umgesetzt (#4046) |

## Offene Lücken ohne Ticket

Der Wächtertest führt außerdem Tabellen mit Personenbezug, für die es weder einen Löschweg noch ein Ticket der Spec gibt (Klasse `open gap`, Stand 15.10.2026: 24 Tabellen). Sie brauchen eine Entscheidung, bevor der AVV sie zusagen kann:

- **Konten und Stammdaten ohne Löschweg:** Konten (`auth.accounts`, alte Eltern-Konten), Sorgeberechtigte ohne Kind (`users.guardian_profiles`), Operator-Konten des moto-Teams.
- **Weitere Protokolle:** Klassenlisten-Abgleich, Anmeldungs-Löschungen und -Wiederherstellungen, Exporte, Datei-Ereignisse, Änderungen an Finanz-, Personalnummer- und Personaldaten, Kinderlöschungen, Vertretungen, Zeiterfassungs-Löschungen, Einstellungsprotokoll, Operator-Protokoll mit IP-Adresse.
- **Inhalte der Schule:** Dateiablage, Elternmitteilungen, Team-Hinweise, vergangene Termine, Klassenlisten, Jahrgangswechsel.
- **Technische Reste:** Demo-Besucher.

Die Liste darf nur schrumpfen. Die genaue Zuordnung je Tabelle steht in `tableRetentions`.

## Grenzen und Sonderfälle

- **Angenommene Anmeldungen** haben keine eigene Frist; sie gehen mit dem Kind (#3962).
- **Erledigte Anmeldungen ohne Kind** (#3960): Zurückgezogen hat sie ein Elternteil (oder die Schule hat eine Verlängerung gestrichen), verfallen ist eine Verlängerung, die bis zur Frist nicht bestätigt wurde. Zweck entfällt mit der Erledigung; 90 Tage decken Rückfragen danach ab. Erledigung ist der letzte Statuswechsel des Kindes. Hat eine Anmeldung zurückgezogene und abgelehnte Kinder, geht sie erst, wenn beide Fristen abgelaufen sind; ganz abgelehnte Anmeldungen löscht weiter der Lauf für abgelehnte Anmeldungen. Wie dort sagt moto noch geplante Mails der Anmeldung ab; die Mail-Warteschlange löscht sie 30 Tage nach der Absage (siehe Technische Daten). Jede gelöschte Anmeldung steht zusätzlich in `audit.enrollment_deletions`. Eine Anmeldung, aus der ein Kind angelegt wurde, bleibt. Eine verfallene oder gestrichene Verlängerung bleibt außerdem, bis die Betreuungsphase des bisherigen Kindes endet: Bis dahin zeigt nur sie, dass die Familie nicht verlängert hat, sonst warnt moto zum Schuljahresende wieder vor einem unentschiedenen Kind.
- **Einladungen zur Nachmeldung** (`enrollment.late_invites`, #3960): Eine eingelöste Einladung geht mit ihrer Anmeldung. Eine nicht eingelöste geht nach derselben Frist ab ihrem Ablauf.
- **AU-Bescheinigungen** (#3961): Der nächtliche Lauf löscht die Zeile samt Dateinamen und legt die Datei in die Warteschlange `users.staff_document_file_cleanup`. Der Aufräumlauf für Personaldokumente prüft die Warteschlange alle fünf Minuten und entfernt die Datei. Eine schon von der Schule gelöschte AU-Bescheinigung verliert nach der Frist auch ihre Zeile. Die Einträge im Änderungsprotokoll der Personaldaten (`audit.staff_master_data_changes`, Dateiname beim Hochladen und Löschen) bleiben; sie sind eine offene Lücke ohne Ticket.
- **Andere Personaldokumente** haben keine laufende Frist; sie werden beim Personalaustritt entfernt.
- **Offene Anfragen** in Elternnachrichten bleiben, solange die OGS sie nicht entschieden hat. Danach gilt die normale Frist ab Versand, eine entschiedene alte Anfrage geht also beim nächsten Lauf.
- **Willkommensmail für Eltern** (#3534): Sie geht je Schule nur einmal an einen Elternkontakt. Damit das nach der Löschfrist so bleibt, löscht der nächtliche Lauf diesen Eintrag der Mail-Warteschlange nicht, sondern leert die Verbindung zum Elternkontakt, Empfänger, Inhalt und Antwort des Mailservers. Übrig bleibt nur der Schlüssel `guardian_welcome:<ID des Elternkontakts>` mit Status und Zeitstempeln (#3955).
- **Zustellprotokolle von Elternbriefen** verschwinden 30 Tage nach der Zustellung. Danach zeigt der Elternbrief nicht mehr, wer ihn per Mail erhalten hat oder nicht erreichbar war.
- **Essens-Krankhistorie** (`schedule.meal_sickness_status_history`): Grundlage der Essensabrechnung. Sie speichert Krank-Tage ein zweites Mal und hat noch keine Frist. Die Fristlöschung der Krank-Tage schreibt bewusst keine neue Zeile hinein (#3958).
- **Markierungen der Migration 1.15.428** (`auth.parent_declaration_permission_grants`): Sie dienen nur deren Rücknahme und gehen seit 1.15.449 mit der Beziehung zwischen Kind und Elternteil. Markierungen ohne Beziehung hat die Migration entfernt (#4046).
- **Betreuungsabmeldungen eines gelöschten Kindes** behalten bis zum Ablauf der Frist nur „Gelöschtes Kind“ und die Konten, die bestätigt und abgeschlossen haben (#2547).
- **Backups** und die Löschung einer ganzen Schule bei Vertragsende sind nicht Teil dieser Liste (#3836, #3837).
