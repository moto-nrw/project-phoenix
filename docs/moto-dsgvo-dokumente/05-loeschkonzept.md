# Löschkonzept und Aufbewahrungsfristen

| | |
|---|---|
| **Dokument** | 05 Löschkonzept und Aufbewahrungsfristen für das Verfahren "moto" (interne Bezeichnung: Project Phoenix) |
| **Version** | 1.0 |
| **Stand** | 2026-07-07 |
| **Status** | Entwurf zur internen Prüfung |
| **Herausgeber** | moto, [RECHTSFORM UND ADRESSE] |
| **Erstellt durch** | Datenschutzkoordination moto |
| **Fachlich geprüft durch** | [NAME DATENSCHUTZBEAUFTRAGTER] (ausstehend) |
| **Freigabe durch** | [NAME GESCHÄFTSFÜHRUNG] (ausstehend) |
| **Nächste Überprüfung** | spätestens 2027-07-07, zusätzlich anlassbezogen |

---

## 1. Zweck und Geltungsbereich

Dieses Löschkonzept beschreibt, nach welchen Regeln personenbezogene Daten im System moto gelöscht werden, wie diese Regeln technisch umgesetzt sind und wie die Löschung nachgewiesen wird. Es konkretisiert den Grundsatz der Speicherbegrenzung (Art. 5 Abs. 1 lit. e DSGVO) und die Löschpflichten aus Art. 17 DSGVO für das Verfahren moto, ein NFC/RFID-gestütztes Anwesenheits- und Raumverwaltungssystem für den Offenen Ganztag (OGS) an Grundschulen in Nordrhein-Westfalen.

Das Konzept gilt für alle personenbezogenen Daten, die moto im Auftrag der jeweiligen Schule bzw. des Schulträgers verarbeitet, einschließlich der Bewegungs-, Protokoll- und Sicherungsdaten. Es ist Bestandteil der Dokumentation der technischen und organisatorischen Maßnahmen nach Art. 32 DSGVO und ergänzt die Anlagen 1 und 2 des Auftragsverarbeitungsvertrags (Art. 28 DSGVO, Dokument 01).

Die Gliederung orientiert sich an der DIN 66398 (Leitlinie zur Entwicklung eines Löschkonzepts): Datenarten-Inventar, Löschklassen, Löschregeln je Datenart, technische Umsetzung, Sonderfälle, Verantwortlichkeiten.

## 2. Rollenverteilung und Löschentscheidung

Die Schule bzw. der Schulträger ist Verantwortlicher im Sinne von Art. 4 Nr. 7 DSGVO. moto ist Auftragsverarbeiter nach Art. 28 DSGVO.

Daraus folgt für die Löschung:

1. **moto trifft keine eigene Löschentscheidung.** moto löscht personenbezogene Daten ausschließlich nach den in diesem Konzept dokumentierten, mit dem Verantwortlichen abgestimmten Fristen oder auf ausdrückliche Weisung des Verantwortlichen.
2. **Die Regellöschfristen für Schülerdaten sind nicht frei verhandelbar.** Der Kernbetrieb stützt sich auf Art. 6 Abs. 1 lit. e DSGVO in Verbindung mit §§ 120 bis 122 SchulG NRW. Die zulässigen Datenarten und Aufbewahrungsfristen ergeben sich aus der VO-DV I. moto darf diese Fristen weder eigenmächtig verkürzen noch verlängern.
3. **Archivrecht geht der Löschung vor.** Nach dem Archivgesetz NRW sind archivwürdige Unterlagen vor der Vernichtung dem zuständigen Archiv anzubieten. Diese Anbietungspflicht trifft die Schule bzw. den Schulträger, nicht moto. Die in diesem Konzept beschriebenen automatischen Löschläufe betreffen ausschließlich Datenarten, für die der Verantwortliche die Löschparameter selbst konfiguriert bzw. bestätigt hat. Löschungen von Stammdatenbeständen erfolgen nur auf Weisung, damit der Verantwortliche seine Anbietungspflicht erfüllen kann.
4. **Betroffenenrechte laufen über den Verantwortlichen.** Löschverlangen von Eltern oder Beschäftigten der Schule werden an die Schule weitergeleitet; moto unterstützt die Umsetzung im Rahmen des Auftragsverarbeitungsvertrags.

## 3. Rechtsgrundlagen

| Norm | Bedeutung für dieses Konzept |
|---|---|
| Art. 5 Abs. 1 lit. e DSGVO | Speicherbegrenzung als Leitprinzip aller Löschregeln |
| Art. 5 Abs. 2 DSGVO | Rechenschaftspflicht: Löschungen müssen nachweisbar sein (Löschprotokoll) |
| Art. 17 DSGVO | Löschpflicht bei Zweckfortfall und auf Betroffenenverlangen |
| Art. 28 Abs. 3 lit. g DSGVO | Löschung oder Rückgabe aller Daten nach Vertragsende, Wahlrecht des Verantwortlichen |
| Art. 32 DSGVO | Löschkonzept als Teil der technischen und organisatorischen Maßnahmen |
| §§ 120 bis 122 SchulG NRW | Landesrechtliche Grundlage der Datenverarbeitung an Schulen in NRW |
| § 9 VO-DV I | Aufbewahrungsfristen für Schülerdaten in NRW (siehe Abschnitt 5) |
| Archivgesetz NRW | Anbietungspflicht vor Vernichtung, Zuständigkeit beim Verantwortlichen |
| § 16 Abs. 2 ArbZG, § 41 EStG, § 147 AO, § 257 HGB | Aufbewahrungspflichten für Arbeitszeit- und lohnrelevante Aufzeichnungen des OGS-Personals |

## 4. Grundsätze

1. **Automatisierung vor Einzelfallentscheidung.** Wiederkehrende Löschungen laufen als automatisierte, täglich ausgeführte Bereinigungsjobs; manuelle Löschungen sind die Ausnahme und werden protokolliert.
2. **Mandantenscharfe Löschung.** Jede Löschung wirkt pro Schule (tenant_id). Die Mandantentrennung wird auf Datenbankebene durch Row Level Security abgesichert; Löschläufe können daher keine Daten anderer Schulen erfassen.
3. **Kürzestmögliche Fristen für Bewegungsdaten.** Anwesenheits- und Aufenthaltsdaten der Kinder werden rollierend nach spätestens 31 Tagen gelöscht, Standard 30 Tage (siehe Abschnitt 7.2).
4. **Konfiguration durch den Verantwortlichen.** Wo das Gesetz Spielräume lässt, legt die Schule die konkrete Frist innerhalb der im System hinterlegten Grenzen selbst fest (mandantenbezogene GDPR-Einstellungen).
5. **Löschung ist Hard-Delete.** Regellöschungen entfernen die Datensätze physisch aus der Produktivdatenbank. Eine Sperrung (Statuswechsel ohne physische Löschung) erfolgt nur in den in Abschnitt 11 beschriebenen Sonderfällen.
6. **Backups laufen aus, sie werden nicht durchsucht.** Gelöschte Daten verbleiben bis zum Ablauf der Backup-Rotation in den Datensicherungen und werden dort nicht einzeln entfernt (siehe Abschnitt 8).

## 5. Fristvorgaben der VO-DV I

Für Schülerdaten in NRW gibt die VO-DV I (§ 9) folgende Aufbewahrungsfristen vor. Fristbeginn ist grundsätzlich das Ende des Kalenderjahres, in dem der Datensatz abgeschlossen wurde, frühestens jedoch das Ende des Kalenderjahres, in dem die Schulpflicht endet.

| Datenkategorie nach VO-DV I | Frist | Relevanz für moto |
|---|---|---|
| Schülerstammdaten (Schülerstammblatt) | 20 Jahre | Die schulische Stammakte führt die Schule. moto hält nur die für den OGS-Betrieb erforderlichen Stammdaten; deren Löschung richtet sich nach dem in Abschnitt 9 beschriebenen Prozess und der Weisung des Verantwortlichen. |
| Zeugnisse, Klassenbucheinträge | 10 Jahre | Nicht relevant, moto verwaltet keine Zeugnisse und keine Klassenbücher. |
| Abgangszeugnisse | bis zu 50 Jahre | Nicht relevant. |
| Übrige Daten (Regelfall, hierunter werden Anwesenheitsnachweise, Atteste und Entschuldigungen typischerweise gefasst) | 5 Jahre | [PRÜFEN] Die Zuordnung von OGS-Anwesenheitsdaten (Check-in/Check-out, Raumwechsel) zu einer Kategorie der VO-DV I ist nicht eindeutig kodifiziert, da die OGS-Betreuung landesrechtlich anders verankert ist als der Pflichtunterricht. Die verbindliche Einordnung ist durch [NAME DATENSCHUTZBEAUFTRAGTER] mit der jeweiligen Schule bzw. dem Schulträger, gegebenenfalls unter Einbindung der zuständigen Bezirksregierung, abzustimmen und schriftlich zu dokumentieren. Bis dahin gilt die im System konfigurierte kurze Löschfrist von maximal 31 Tagen für die elektronischen Bewegungsdaten; sofern die Schule Anwesenheitsnachweise länger benötigt, hat sie diese vor Ablauf der Frist über die vorgesehenen Exportfunktionen in ihre eigene Aktenführung zu übernehmen. |
| Daten auf privaten digitalen Endgeräten | 1 Jahr | Nur mittelbar relevant, da moto zentral hostet. Die Schule stellt sicher, dass Beschäftigte keine Exporte dauerhaft auf privaten Endgeräten speichern. |

**Klarstellung zum Verhältnis der Fristen:** Die kurzen Löschfristen im System (z. B. 30 Tage für Bewegungsdaten) unterschreiten die VO-DV I nicht in unzulässiger Weise. Die VO-DV I begrenzt die Speicherdauer nach oben; sie verpflichtet nicht dazu, elektronische Bewegungsdaten über den Betreuungszweck hinaus vorzuhalten. Soweit die Schule einzelne Nachweise für ihre Aktenführung benötigt, erstellt sie diese vor Fristablauf; die zugehörigen Exportzugriffe werden im System protokolliert (audit.data_access_log).

## 6. Löschklassen und Löschregeln je Datenkategorie

Die Datenkategorien entsprechen der Datenbestandsaufnahme (gesondertes internes Dokument) und dem Verzeichnis von Verarbeitungstätigkeiten (Dokument 04). Alle Tabellenangaben sind schema-qualifiziert (PostgreSQL, 15 Domänen-Schemas).

### 6.1 Löschklassen

| Klasse | Beschreibung | Typische Frist |
|---|---|---|
| **A** Rollierende Kurzfrist | Bewegungs- und Aufenthaltsdaten, tägliche automatische Löschung | 1 bis 31 Tage |
| **B** Operative Mittelfrist | Feedback, abgelehnte Anmeldungen, abgeschlossene Termine | 90 Tage bis 5 Jahre |
| **C** Gesetzliche Aufbewahrung | Zeiterfassung des Personals, Stammdaten nach VO-DV I | 2 bis 20 Jahre |
| **D** Ereignisgebundene Löschung | Tokens, Einladungen, Konten, Karten-Zuordnungen; Auslöser ist ein Ereignis (Ablauf, Austritt, Widerruf), keine Kalenderfrist | bei Ereigniseintritt |
| **E** Aufbewahrung ohne definierte Frist | Audit- und Protokolltabellen ohne automatische Löschung; offener Prüfpunkt, siehe Abschnitt 14 | derzeit unbegrenzt [PRÜFEN] |

### 6.2 Schülerdaten

| Datenkategorie | Speicherort | Klasse | Löschregel |
|---|---|---|---|
| Anwesenheit (Check-in/Check-out) | active.attendance | A | Automatische Löschung nach Ablauf des Aufbewahrungsfensters: Standard 30 Tage, konfigurierbar 1 bis 31 Tage (gdpr.privacy_consent_retention_days). Liegt für ein Kind eine individuelle Einwilligung mit eigenem Wert vor (users.privacy_consents.data_retention_days, ebenfalls 1 bis 31 Tage), geht dieser vor. Täglicher Löschlauf, siehe Abschnitt 7. |
| Aufenthaltsort und Raumwechsel | active.visits | A | Identische Regel wie active.attendance (gleiches Aufbewahrungsfenster, gleicher Löschlauf). |
| Krank-, Entschuldigt- und Ausflugs-Tagesmeldungen | active.student_status_days | E | [PRÜFEN] Für diese Tabelle ist im System derzeit kein eigenes Aufbewahrungsfenster hinterlegt. Da Krankmeldungen Gesundheitsdaten (Art. 9 DSGVO) sind, ist eine dedizierte Löschregel festzulegen und technisch umzusetzen. Bis dahin erfolgt die Löschung mit dem Kind-Datensatz (Abschnitt 9). |
| Stammdaten (Name, Geburtsdatum, Klasse, Adresse, Gruppenzugehörigkeit, Abholregelungen) | users.persons, users.students | C/D | Aufbewahrung für die Dauer des Betreuungsverhältnisses. Nach Schulaustritt: Prozess nach Abschnitt 9. Endgültige Löschung auf Weisung des Verantwortlichen unter Beachtung der VO-DV-I-Fristen und der Archivanbietungspflicht. |
| Gesundheitsangaben (Freitext) | users.students.health_info | D | Löschung bzw. Leerung des Feldes spätestens mit dem Austrittsprozess nach Abschnitt 9. Eine über das Betreuungsende hinausgehende Aufbewahrung von Gesundheitsdaten findet nicht statt. Vorzeitige Löschung jederzeit auf Weisung der Schule oder auf begründetes Verlangen der Erziehungsberechtigten über die Schule. |
| Betreuer-Notizen, Zusatzinformationen | users.students.supervisor_notes, .extra_info | D | Löschung mit dem Kind-Datensatz (Abschnitt 9); vorzeitig auf Weisung. |
| Foto | users.students.photo_path (Einwilligung: .photo_consent_given_at/by) | D | Löschung der Bilddatei und des Pfadverweises unverzüglich bei Widerruf der Einwilligung, spätestens mit dem Austrittsprozess nach Abschnitt 9. |
| Einwilligungsnachweise (Foto, AGB, Datenverarbeitung, E-Mail-Kontakt, Datenschutz-Einwilligung je Kind) | users.students.*_accepted_at, users.privacy_consents | D | Aufbewahrung als Nachweis für die Dauer des Betreuungsverhältnisses (Art. 5 Abs. 2, Art. 7 Abs. 1 DSGVO). Löschung mit dem Kind-Datensatz. [PRÜFEN] Ob Einwilligungsnachweise nach Betreuungsende für eine begrenzte Nachweisfrist (Verjährung) aufzubewahren sind, ist mit [NAME DATENSCHUTZBEAUFTRAGTER] festzulegen. |
| RFID-Karten-Zuordnung | users.persons.tag_id, users.rfid_cards | D | Aufhebung der Zuordnung Tag zu Person mit dem Austrittsprozess (Abschnitt 9). Die Karte wird deaktiviert; die Tag-UID ohne Personenzuordnung ist kein dem Kind zuordenbares Datum mehr. |
| Feedback-Einträge (z. B. Mensa-Bewertung) | feedback.entries | B | Automatische Löschung nach Ablauf des Aufbewahrungsfensters: Standard 90 Tage, konfigurierbar 1 bis 365 Tage (feedback.data_retention_days). |
| Anmeldedaten vor Kind-Anlage (Enrollment) | enrollment.requests, enrollment.request_children, enrollment.request_guardians | B/D | Abgelehnte Anmeldungen: automatische Löschung nach Standard 90 Tagen, konfigurierbar bis 730 Tage (enrollment.rejected_retention_days). Genehmigte Anmeldungen: Übernahme der Daten in den Kind-Datensatz. [PRÜFEN] Für die Ursprungsdatensätze genehmigter Anmeldungen ist derzeit keine eigene automatische Löschfrist hinterlegt; eine Regel ist festzulegen. Status- und Bearbeitungslinks der Anmeldung verfallen nach Standard 365 Tagen, konfigurierbar bis 1825 Tage (enrollment.status_token_ttl_days). |
| Datenänderungsanfragen der Eltern | users.student_data_change_requests | E | [PRÜFEN] Kein eigenes Aufbewahrungsfenster im System hinterlegt; Löschregel ist festzulegen. Bis dahin Löschung mit dem Kind-Datensatz. |
| Betreuungsplan-Termine (abgeschlossen/abgesagt) | schedule (Timetable) | B | Automatische Löschung nach Standard 365 Tagen, konfigurierbar 1 bis 1825 Tage (gdpr.timetable_retention_days). |

### 6.3 Daten der Eltern und Erziehungsberechtigten

| Datenkategorie | Speicherort | Klasse | Löschregel |
|---|---|---|---|
| Profil (Name, E-Mail, Adresse, Telefonnummern, Präferenzen, interne Notizen) | users.guardian_profiles, users.guardian_phone_numbers | D | Aufbewahrung, solange mindestens eine aktive Zuordnung zu einem betreuten Kind besteht (users.students_guardians). Endet die letzte Zuordnung (Austritt des Kindes, Abschnitt 9), wird das Profil einschließlich Telefonnummern und Notizen gelöscht bzw. das zugehörige Portal-Konto deaktiviert und im Austrittsprozess entfernt. |
| Kind-Zuordnung inkl. Abholberechtigung, Notfallkontakt, Portal-Berechtigungen | users.students_guardians | D | Löschung mit der Beendigung der Beziehung bzw. mit dem Kind-Datensatz (Abschnitt 9). |
| Eltern-Portal-Konto | auth.accounts, auth.account_tenants | D | Deaktivierung mit Ende der letzten Kind-Zuordnung, Löschung im Austrittsprozess (Abschnitt 9). Bei Konten mit Zuordnungen zu mehreren Schulen wird nur die Zuordnung zur ausscheidenden Schule beendet. |
| Portal-Einladungen | auth.guardian_invitations | D | Einladungslinks verfallen nach Standard 48 Stunden, konfigurierbar bis 168 Stunden (invitations.guardian_token_expiry_hours). Abgelaufene Einladungen werden durch einen automatischen Bereinigungsjob entfernt. |
| Eltern-Chat mit der Betreuung | users.parent_messages | E | [PRÜFEN] Kein Aufbewahrungsfenster im System hinterlegt. Eine Löschregel (Vorschlag: Löschung mit dem Kind-Datensatz, zusätzlich rollierende Höchstfrist) ist festzulegen und technisch umzusetzen. |
| Änderungsprotokoll Kontakt-/Abholdaten | audit.guardian_changes | E | Append-only-Protokoll, derzeit ohne automatische Löschung (siehe Abschnitt 14). Bei Kontaktänderungen werden die Werte selbst nicht gespeichert, nur bei Abholdaten. |

### 6.4 Daten des Personals

| Datenkategorie | Speicherort | Klasse | Löschregel |
|---|---|---|---|
| Zeiterfassung (Kommen/Gehen, Pausen, Homeoffice), Korrekturen, Abwesenheiten (inkl. Krankmeldungen), Urlaubskontingente | active.work_sessions, active.work_session_breaks, audit.work_session_edits, active.staff_absences, active.staff_vacation_quota | C | Automatische Löschung nach Ablauf des Aufbewahrungsfensters: Standard 730 Tage (2 Jahre), wählbar in Stufen 730/1095/1460/1825/2190/2555/2920 Tage (gdpr.time_tracking_retention_days). Die Bandbreite bildet die gesetzlichen Aufbewahrungspflichten ab: mindestens 2 Jahre nach § 16 Abs. 2 ArbZG, 6 Jahre bei Lohnkonto-Bezug nach § 41 EStG, bis 8 Jahre nach § 147 AO bzw. § 257 HGB. Die konkrete Frist legt die Schule bzw. der Träger entsprechend der eigenen lohnbuchhalterischen Einordnung fest. |
| Stammdaten (Name, Geburtsdatum, Beschäftigungsart, Arbeitszeitmodell, Notizen) | users.persons, users.staff, config.work_time_models, config.staff_work_schedules | D | Aufbewahrung für die Dauer der Beschäftigung. Bei Ausscheiden: Deaktivierung des Kontos, Aufhebung der RFID-Zuordnung, Löschung der Stammdaten auf Weisung des Verantwortlichen nach Ablauf der zeiterfassungsbezogenen Aufbewahrungsfristen. |
| Konto, PIN, Passwort | auth.accounts (password_hash, pin_hash, Argon2id) | D | Deaktivierung bei Ausscheiden, Löschung mit den Stammdaten. Passwörter und PINs liegen zu keinem Zeitpunkt im Klartext vor. |
| Gruppenbetreuung, Vertretungen | active.group_supervisor, education.group_teacher, education.group_substitution | D | Löschung mit der organisatorischen Zuordnung bzw. mit den Stammdaten. |

### 6.5 Konten- und Sicherheitsdaten (alle Kontotypen)

| Datenkategorie | Speicherort | Klasse | Löschregel |
|---|---|---|---|
| Sessions und Refresh-Tokens | auth.tokens | D | Automatische Löschung abgelaufener Tokens durch einen regelmäßigen Bereinigungsjob. |
| Passwort-Reset-Tokens und zugehörige Rate-Limits | auth.password_reset_tokens, auth.password_reset_rate_limits | D | Automatische Löschung abgelaufener Einträge durch regelmäßige Bereinigungsjobs. |
| Einladungstokens (Personal, Operator, E-Mail-Änderung) | auth.invitation_tokens, platform.operator_invitation_token, platform.operator_email_change_token | D | Automatische Löschung abgelaufener Tokens durch regelmäßige Bereinigungsjobs. |
| MFA: vertrauenswürdige Geräte | auth.mfa_trusted_devices | D | Verfall nach Standard 90 Tagen, konfigurierbar 1 bis 180 Tage (security.mfa_trusted_device_days). |
| MFA-Daten, Passkeys | auth.mfa_credentials, auth.mfa_email_challenges, auth.passkey_credentials, auth.passkey_sessions | D | Löschung mit dem Konto bzw. bei Deregistrierung des Merkmals durch die Nutzerin oder den Nutzer. |
| Login-Historie inkl. IP-Adresse | audit.auth_events | E | Derzeit ohne automatische Löschung (siehe Abschnitt 14). |

### 6.6 Audit- und Protokolldaten

| Datenkategorie | Speicherort | Klasse | Löschregel |
|---|---|---|---|
| Datenzugriffsprotokoll (wer hat wann welche sensiblen Daten eingesehen) | audit.data_access_log | E | Bewusst ohne automatische Löschung, dient dem Nachweis nach Art. 5 Abs. 2 und Art. 32 DSGVO. Eine Höchstfrist ist festzulegen (siehe Abschnitt 14). |
| Löschprotokoll | audit.data_deletions | E | Aufbewahrung als Löschnachweis (Art. 5 Abs. 2 DSGVO). Das Protokoll enthält Art, Umfang, Grund, ausführende Person und Zeitpunkt der Löschung, jedoch nicht die gelöschten Inhalte selbst. Eine dauerhafte Aufbewahrung ist als Rechenschaftsnachweis vertretbar; eine Höchstfrist ist gleichwohl festzulegen (Abschnitt 14). |
| Import-Protokoll | audit.data_imports | E | Derzeit ohne automatische Löschung (Abschnitt 14). |
| Zeiterfassungs-Korrekturprotokoll | audit.work_session_edits | C | Wird vom Löschlauf der Zeiterfassung miterfasst (gdpr.time_tracking_retention_days, siehe 6.4). |
| Enrollment-Korrekturprotokoll | audit.enrollment_offering_adjustment | E | Derzeit ohne automatische Löschung (Abschnitt 14). |
| Unregistrierte NFC-Scans (Tag-UID ohne Personenzuordnung) | audit.unregistered_tag_scans | E | [PRÜFEN] Kein Aufbewahrungsfenster hinterlegt. Da die Tag-UID ein pseudonymes, potenziell re-identifizierbares Merkmal ist, ist eine kurze rollierende Löschfrist festzulegen und umzusetzen. |
| Operator-Audit-Log | platform.operator_audit_log | E | Derzeit ohne automatische Löschung (Abschnitt 14). Betrifft ausschließlich Handlungen von Beschäftigten des Plattformbetreibers. |

### 6.7 System-, Log- und Monitoring-Daten

| Datenkategorie | Speicherort | Klasse | Löschregel |
|---|---|---|---|
| Anwendungs- und Zugriffslogs | zentrales Log-System (selbst gehostetes Loki auf dem Hetzner-Server) | A/B | Automatische Löschung nach 30 Tagen (Retention des Log-Systems). Die Anwendung protokolliert nach interner Vorgabe keine Klarnamen von Kindern auf Info-Level, sondern ausschließlich Kennungen; Zugriffs-Logs des Reverse Proxy maskieren IP-Adressen sowie Authentifizierungs-Header. |
| Datenbank-Sicherungen | Backup-Verzeichnis auf dem Produktionsserver | siehe Abschnitt 8 | Rotation, keine Einzellöschung. |

## 7. Technische Umsetzung

### 7.1 Automatisierte Datenbereinigung (Scheduler)

Die Regellöschung wird durch einen serverseitigen Scheduler ausgeführt. Der Bereinigungslauf ist mandantenbezogen konfigurierbar:

| Einstellung | Standardwert | Grenzen | Funktion |
|---|---|---|---|
| gdpr.data_cleanup_enabled | aktiviert | | Automatische Datenbereinigung ein/aus |
| gdpr.data_cleanup_time | 02:00 Uhr | | Uhrzeit des täglichen Bereinigungslaufs |
| gdpr.data_cleanup_timeout_minutes | 30 Minuten | 5 bis 120 | Zeitbegrenzung des Laufs |

Der Löschlauf arbeitet pro Schule (tenant_id) und iteriert über alle aktiven Mandanten. Jede Löschung wird im Löschprotokoll (audit.data_deletions) mit Art, Anzahl der gelöschten Datensätze, Grund und Zeitpunkt festgehalten.

Organisatorische Vorgabe: Das Abschalten der automatischen Bereinigung (gdpr.data_cleanup_enabled) durch eine Schule ist nur nach dokumentierter Abstimmung mit [NAME DATENSCHUTZBEAUFTRAGTER] zulässig, da ansonsten die in diesem Konzept zugesicherten Fristen nicht eingehalten werden. [PRÜFEN] Ob technisch verhindert werden soll, dass Schulen die Bereinigung dauerhaft deaktivieren, ist zu entscheiden.

### 7.2 Konfigurierbare Aufbewahrungsfenster (GDPR-Einstellungen)

Die folgenden Fristen sind je Schule als Einstellung hinterlegt. Der Verantwortliche kann sie innerhalb der Systemgrenzen anpassen; jede Änderung wird in einem append-only Änderungsprotokoll (config.setting_audit) festgehalten.

| Einstellung | Standard | Grenzen | Betroffene Daten |
|---|---|---|---|
| gdpr.privacy_consent_retention_days | 30 Tage | 1 bis 31 | Anwesenheits- und Aufenthaltsdaten der Kinder (active.attendance, active.visits); individuelle Einwilligungswerte je Kind (users.privacy_consents.data_retention_days, ebenfalls 1 bis 31 Tage) gehen vor |
| gdpr.time_tracking_retention_days | 730 Tage | Stufen 730 bis 2920 | Zeiterfassung des Personals inkl. Pausen, Korrekturen, Abwesenheiten |
| gdpr.timetable_retention_days | 365 Tage | 1 bis 1825 | Abgeschlossene und abgesagte Betreuungsplan-Termine |
| feedback.data_retention_days | 90 Tage | 1 bis 365 | Feedback-Einträge der Kinder |
| enrollment.rejected_retention_days | 90 Tage | bis 730 | Abgelehnte Anmeldungen |
| enrollment.status_token_ttl_days | 365 Tage | bis 1825 | Status- und Bearbeitungslinks von Anmeldungen |
| invitations.guardian_token_expiry_hours | 48 Stunden | bis 168 | Eltern-Einladungslinks |
| security.mfa_trusted_device_days | 90 Tage | 1 bis 180 | Vertrauenswürdige Geräte (2FA-Ausnahme) |

Ergänzend begrenzen zwei Sichtbarkeitsfenster den lesenden Zugriff auf noch nicht gelöschte Anwesenheitsdaten (Datenminimierung auf Zugriffsebene, keine Löschregeln im engeren Sinn): gdpr.attendance_visible_days (Standard 30 Tage, 1 bis 365) und gdpr.room_detail_visible_days (Standard 7 Tage, 1 bis 365). Das Anwesenheitsprotokoll ist standardmäßig vollständig deaktiviert (gdpr.attendance_log_enabled, Standard: aus) und der Leserkreis auf Gruppenbetreuende beschränkt (gdpr.attendance_log_scope, gdpr.student_data_scope).

### 7.3 Nicht konfigurierbare Bereinigungsjobs

Unabhängig von den Mandanten-Einstellungen laufen folgende technische Bereinigungen in festen Intervallen:

- Löschung abgelaufener Sitzungs- und Refresh-Tokens (auth.tokens)
- Löschung abgelaufener Passwort-Reset-Tokens und zugehöriger Rate-Limit-Einträge
- Löschung abgelaufener Einladungen (auth.invitation_tokens, auth.guardian_invitations)
- Löschung abgelaufener E-Mail-Änderungs- und Einladungstokens des Betreiberportals
- Technische Aufräumjobs für nicht abgeschlossene Anwesenheits- und Aufsichtszuordnungen (Stale-Attendance, Stale-Supervisor); diese korrigieren Datenqualität und stellen kein eigenes Aufbewahrungsfenster im datenschutzrechtlichen Sinn dar

### 7.4 Kiosk-Geräte (PyrePortal auf Raspberry Pi)

Auf den Kiosk-Geräten in den Schulen werden personenbezogene Daten nicht dauerhaft gespeichert. Das Gerät hält lediglich seinen Geräte-API-Schlüssel und temporäre Sitzungsdaten; alle personenbezogenen Daten liegen ausschließlich in der zentralen Datenbank. Die Löschpflichten dieses Konzepts konzentrieren sich daher auf das Backend. [PRÜFEN] Die Aussage zur fehlenden lokalen Persistenz ist bei wesentlichen Änderungen der Kiosk-Software erneut zu verifizieren; bei Außerbetriebnahme eines Geräts wird der Geräte-API-Schlüssel serverseitig widerrufen.

### 7.5 Mandantenschärfe und Nachvollziehbarkeit

Alle mandantenbezogenen Tabellen tragen eine tenant_id mit Fremdschlüssel auf die Schule; Row Level Security auf Datenbankebene stellt sicher, dass Lösch- wie Leseoperationen die Mandantengrenze nicht überschreiten. Manuelle Löschungen durch Administratorinnen und Administratoren der Schule (z. B. Löschung eines Kind-Datensatzes) werden ebenso wie automatische Löschläufe im Löschprotokoll (audit.data_deletions) erfasst.

## 8. Datensicherungen und ihr Verhältnis zur Löschung

### 8.1 Sicherungsverfahren

Vor jeder Änderung am Produktivsystem (Deployment mit Datenbankmigration) wird eine vollständige Datenbanksicherung erstellt (pg_dump im Custom-Format, zusätzlich Sicherung der Rollen- und Berechtigungsdefinitionen sowie der hochgeladenen Dateien). Die Sicherungen liegen auf demselben Server in einem zugriffsbeschränkten Verzeichnis (Dateiberechtigungen restriktiv gesetzt).

### 8.2 Rotation

Es werden die jeweils letzten sieben Sicherungen der Produktionsumgebung vorgehalten; ältere Sicherungen werden beim Erstellen einer neuen Sicherung automatisch gelöscht (Staging: drei Sicherungen). Da Sicherungen anlassbezogen bei Deployments entstehen, entspricht die Vorhaltezeit von sieben Sicherungen bei der üblichen Deployment-Frequenz einem Zeitraum in der Größenordnung von etwa einer Woche. [PRÜFEN] Ergänzend ist eine kalenderbasierte Höchstvorhaltezeit (z. B. Löschung jeder Sicherung, die älter als [MAXIMALE BACKUP-VORHALTEZEIT IN TAGEN] ist) festzulegen und technisch umzusetzen, damit die Vorhaltezeit auch bei geringer Deployment-Frequenz begrenzt bleibt.

### 8.3 Verhältnis zur Löschung im Produktivsystem

Personenbezogene Daten verbleiben nach ihrer Löschung im Produktivsystem noch bis zum Auslaufen der Rotation in den Datensicherungen. Eine gezielte Einzellöschung aus Sicherungen erfolgt nicht, da dies die Integrität und Wiederherstellbarkeit der Sicherung gefährden würde. Stattdessen gilt:

1. Die maximale Vorhaltezeit der Sicherungen wird auf das technisch und organisatorisch notwendige Minimum begrenzt und dokumentiert (Abschnitt 8.2).
2. Sicherungen werden ausschließlich zur Wiederherstellung nach Störungen verwendet, nicht zur Auswertung.
3. Wird eine Sicherung zurückgespielt, die bereits gelöschte Daten enthält, werden die zwischenzeitlich fällig gewordenen Löschungen unmittelbar nach der Wiederherstellung durch den nächsten Bereinigungslauf nachgeholt; dies wird im Wiederherstellungsprotokoll vermerkt.
4. Nach Vertragsende einer Schule laufen die letzten Sicherungen, die Daten dieser Schule enthalten, mit der Rotation aus (Abschnitt 10).

## 9. Prozess bei Schulaustritt eines Kindes

Auslöser ist das Ende des Betreuungsverhältnisses (Austritt, Schulwechsel, Ende der OGS-Teilnahme). Der Prozess läuft wie folgt:

1. **Statuswechsel durch die Schule.** Die Schule setzt den Kind-Datensatz auf den Status inaktiv bzw. Alumnus und pflegt das Betreuungsende (users.students.status, enrolled_until). Ab diesem Zeitpunkt ist das Kind aus den operativen Ansichten (Anwesenheit, Gruppen, Kiosk) ausgeblendet.
2. **Auslaufen der Bewegungsdaten.** Anwesenheits- und Aufenthaltsdaten (active.attendance, active.visits) laufen über das rollierende Aufbewahrungsfenster (maximal 31 Tage) automatisch aus; spätestens 31 Tage nach dem letzten Betreuungstag existieren keine Bewegungsdaten des Kindes mehr im Produktivsystem.
3. **Aufhebung der RFID-Zuordnung.** Die Zuordnung der Tag-UID zur Person wird aufgehoben, die Karte deaktiviert und für die Wiederverwendung oder Vernichtung an die Schule zurückgegeben.
4. **Gesundheitsdaten und Freitexte.** Gesundheitsangaben (users.students.health_info), Betreuer-Notizen und Abholregelungs-Freitexte werden im Zuge der Austrittsbearbeitung gelöscht; das Foto wird gelöscht.
5. **Eltern-Zuordnungen.** Die Kind-Eltern-Zuordnungen (users.students_guardians) werden beendet. Besteht für einen Erziehungsberechtigten keine weitere aktive Kind-Zuordnung, werden Profil und Portal-Konto deaktiviert und im Rahmen der Stammdatenlöschung entfernt; laufende Chat-Verläufe (users.parent_messages) werden mitgelöscht, sobald die in Abschnitt 6.3 geforderte Löschregel umgesetzt ist [PRÜFEN].
6. **Stammdatenlöschung auf Weisung.** Die endgültige Löschung der Stammdaten (users.persons, users.students und abhängige Datensätze) erfolgt auf Weisung des Verantwortlichen. Die Schule prüft zuvor, ob Daten für die eigene Aktenführung zu übernehmen sind (VO-DV I, Abschnitt 5) und ob die Archivanbietungspflicht greift. moto empfiehlt als Regelweisung die Löschung zum Ende des Kalenderjahres, das auf das Betreuungsende folgt; die verbindliche Festlegung trifft der Verantwortliche. [PRÜFEN] Diese Regelweisung ist je Schulträger im Auftragsverarbeitungsvertrag bzw. in dessen Anlage zu fixieren.
7. **Protokollierung.** Jeder Löschschritt wird im Löschprotokoll (audit.data_deletions) mit Umfang, Grund und ausführender Person festgehalten. Auf Anforderung erhält die Schule eine Löschbestätigung.

Sonderweg vorzeitige Löschung: Verlangen Erziehungsberechtigte die Löschung einzelner Daten (Art. 17 DSGVO), richtet sich das Verlangen an die Schule als Verantwortlichen. Die Schule prüft entgegenstehende Aufbewahrungspflichten und weist moto an; moto setzt die Weisung binnen der im Auftragsverarbeitungsvertrag vereinbarten Frist um und bestätigt die Ausführung.

## 10. Prozess bei Vertragsende der Schule

Endet der Vertrag zwischen moto und der Schule bzw. dem Schulträger, gilt nach Art. 28 Abs. 3 lit. g DSGVO:

1. **Wahlrecht des Verantwortlichen.** Die Schule bzw. der Schulträger entscheidet, ob die Daten gelöscht oder zurückgegeben (exportiert) werden. Das Wahlrecht ist innerhalb von [30 TAGE NACH VERTRAGSENDE] nach Vertragsende auszuüben; übt der Verantwortliche das Wahlrecht nicht fristgerecht aus, erfolgt die Löschung nach vorheriger schriftlicher Ankündigung.
2. **Rückgabe/Export.** Auf Wunsch stellt moto vor der Löschung einen vollständigen Export der Mandantendaten in einem strukturierten, gängigen Format bereit. Der Export wird verschlüsselt übergeben; die Übergabe wird dokumentiert.
3. **Mandantenscharfe Löschung.** Anschließend löscht moto sämtliche personenbezogenen Daten des Mandanten aus dem Produktivsystem. Die Löschung erfolgt anhand der Mandantenkennung (tenant_id) über alle mandantenbezogenen Tabellen; die Mandantentrennung per Row Level Security stellt sicher, dass ausschließlich Daten der betroffenen Schule erfasst werden. Hochgeladene Dateien (z. B. Fotos, Login-Bild) werden mitgelöscht. Der Mandant (Schul-Eintrag) wird deaktiviert und nach Abschluss der Löschung entfernt.
4. **Auslaufen der Sicherungen.** Datensicherungen, die Daten des Mandanten enthalten, werden nicht einzeln bereinigt, sondern laufen mit der Backup-Rotation aus (Abschnitt 8). Bis zum Auslaufen werden sie nicht wiederhergestellt, außer zur Abwehr eines Datenverlusts; in diesem Fall wird die Mandantenlöschung unmittelbar nach der Wiederherstellung wiederholt.
5. **Audit- und Löschprotokolle.** Einträge im Löschprotokoll (audit.data_deletions) sowie mandantenbezogene Einträge in den übrigen Audit-Tabellen werden als Nachweis der ordnungsgemäßen Vertragsabwicklung aufbewahrt, soweit und solange dies zur Erfüllung der Rechenschaftspflicht erforderlich ist. [PRÜFEN] Umfang und Frist dieser Nachweisaufbewahrung sind mit [NAME DATENSCHUTZBEAUFTRAGTER] festzulegen; personenbeziehbare Inhalte, die für den Nachweis nicht erforderlich sind, werden gelöscht.
6. **Löschbestätigung.** moto stellt dem Verantwortlichen nach Abschluss der Löschung und nach Auslaufen der letzten betroffenen Sicherung eine schriftliche Löschbestätigung aus, die Umfang, Zeitpunkte und Verfahren der Löschung benennt.
7. **Unterauftragsverarbeiter.** moto stellt sicher, dass auch bei eingesetzten Unterauftragsverarbeitern (siehe Subprozessoren-Verzeichnis, Dokument 03) keine mandantenbezogenen personenbezogenen Daten über das Vertragsende hinaus verbleiben, soweit solche dort verarbeitet wurden (insbesondere E-Mail-Versand, Fehlerberichte).

## 11. Sonderfälle

1. **Sperrung statt Löschung.** Ist eine Löschung wegen eines laufenden Verfahrens (z. B. Rechtsstreit, aufsichtsbehördliche Prüfung) oder einer entgegenstehenden Aufbewahrungspflicht vorübergehend unzulässig, werden die betroffenen Datensätze gesperrt: Der Zugriff wird auf die für das Verfahren erforderlichen Personen beschränkt, die Datensätze werden von den automatischen Löschläufen ausgenommen und der Vorgang wird dokumentiert. Nach Wegfall des Sperrgrunds wird die Löschung nachgeholt.
2. **Widerspruch und Einwilligungswiderruf.** Beruht eine Verarbeitung auf Einwilligung (insbesondere Fotos), führt der Widerruf zur unverzüglichen Löschung der betroffenen Daten, ohne die Rechtmäßigkeit der bisherigen Verarbeitung zu berühren.
3. **Unvollständige oder fehlerhafte Anmeldungen.** Anmeldedaten, die nie zu einem Betreuungsverhältnis führen, unterliegen der automatischen Löschung abgelehnter Anmeldungen (Abschnitt 6.2).
4. **Nicht zugeordnete RFID-Scans.** Scans unbekannter Tags (audit.unregistered_tag_scans) enthalten keine unmittelbare Personenzuordnung, sind aber pseudonym re-identifizierbar und werden daher der in Abschnitt 14 geforderten kurzen Löschfrist unterworfen [PRÜFEN].
5. **Systemabbau und Migration.** Bei Ablösung von Systemkomponenten werden Altdatenbestände nach erfolgreicher Migration und Verifikation gelöscht; Datenträger werden vor Außerbetriebnahme sicher gelöscht bzw. vernichtet. Für die beim Hosting-Anbieter betriebene Infrastruktur gelten dessen zertifizierte Verfahren zur Datenträgervernichtung [PRÜFEN: Nachweis über Hosting-AVV beibringen].

## 12. Protokollierung und Nachweis

1. Jede automatische wie manuelle Löschung personenbezogener Daten wird im Löschprotokoll (audit.data_deletions) festgehalten: betroffener Datensatztyp, Anzahl der Datensätze, Löschgrund, ausführende Stelle, Zeitpunkt. Die gelöschten Inhalte selbst werden nicht protokolliert.
2. Änderungen an den Aufbewahrungseinstellungen werden im Einstellungs-Änderungsprotokoll (config.setting_audit, append-only) festgehalten.
3. Exporte und lesende Zugriffe auf Anwesenheitshistorien werden im Datenzugriffsprotokoll (audit.data_access_log) festgehalten.
4. Auf Anforderung erhält der Verantwortliche Auszüge aus diesen Protokollen sowie Löschbestätigungen für konkrete Vorgänge.

## 13. Verantwortlichkeiten und Pflege

| Aufgabe | Zuständigkeit |
|---|---|
| Festlegung und Änderung der Aufbewahrungsfenster je Schule | Verantwortlicher (Schulleitung/Trägerverwaltung), technisch über die GDPR-Einstellungen |
| Betrieb und Überwachung der Löschläufe | moto, Betrieb/Entwicklung |
| Prüfung der Löschprotokolle (stichprobenweise, mindestens jährlich) | Datenschutzkoordination moto |
| Freigabe von Ausnahmen (Sperrungen, Abschaltung der Bereinigung) | [NAME DATENSCHUTZBEAUFTRAGTER] |
| Pflege dieses Konzepts, Abgleich mit dem Systemstand | Datenschutzkoordination moto, mindestens jährlich sowie bei jeder Änderung an Löschjobs oder Aufbewahrungseinstellungen |
| Abstimmung der VO-DV-I-Einordnung mit Schulen/Trägern | [NAME DATENSCHUTZBEAUFTRAGTER] |

## 14. Offene Punkte und Prüfaufträge

Die folgenden Punkte sind vor der Freigabe dieses Konzepts zu klären bzw. umzusetzen. Sie ergeben sich aus dem Abgleich mit dem tatsächlichen Systemstand und werden hier bewusst transparent ausgewiesen (Art. 5 Abs. 1 lit. e und Abs. 2 DSGVO):

1. **Audit-Tabellen ohne Löschfrist.** Für audit.auth_events, audit.data_access_log, audit.data_imports, audit.guardian_changes, audit.enrollment_offering_adjustment und platform.operator_audit_log existiert derzeit keine automatische Löschung. Für jede Tabelle ist eine begründete Höchstfrist festzulegen (Vorschlag zur Diskussion: 12 Monate für Login- und Zugriffshistorien, längere Fristen nur, wo der Nachweiszweck dies trägt) und technisch umzusetzen. [PRÜFEN]
2. **Krank-/Entschuldigt-Tageshistorie (active.student_status_days).** Gesundheitsdaten ohne eigenes Aufbewahrungsfenster; dedizierte Löschregel festlegen und umsetzen. [PRÜFEN]
3. **Eltern-Chat (users.parent_messages).** Löschregel festlegen und umsetzen. [PRÜFEN]
4. **Unregistrierte NFC-Scans (audit.unregistered_tag_scans).** Kurze rollierende Löschfrist festlegen und umsetzen. [PRÜFEN]
5. **Genehmigte Anmeldungen und Datenänderungsanfragen.** Löschregeln für die Ursprungsdatensätze genehmigter Anmeldungen sowie für users.student_data_change_requests festlegen. [PRÜFEN]
6. **VO-DV-I-Einordnung der OGS-Anwesenheitsdaten.** Verbindliche Abstimmung mit Schulen/Trägern, gegebenenfalls Bezirksregierung, dokumentieren (Abschnitt 5). [PRÜFEN]
7. **Regelweisung zur Stammdatenlöschung nach Austritt.** Je Schulträger im Auftragsverarbeitungsvertrag fixieren (Abschnitt 9 Nr. 6). [PRÜFEN]
8. **Kalenderbasierte Höchstvorhaltezeit für Backups.** Ergänzend zur anzahlbasierten Rotation festlegen und umsetzen (Abschnitt 8.2). [PRÜFEN]
9. **Nachweisaufbewahrung nach Vertragsende.** Umfang und Frist der aufbewahrten Lösch- und Audit-Nachweise festlegen (Abschnitt 10 Nr. 5). [PRÜFEN]
10. **Einwilligungsnachweise nach Betreuungsende.** Entscheidung über eine begrenzte Nachweisfrist (Abschnitt 6.2). [PRÜFEN]
11. **Deaktivierbarkeit der automatischen Bereinigung.** Entscheidung, ob das dauerhafte Abschalten durch Schulen technisch unterbunden wird (Abschnitt 7.1). [PRÜFEN]
12. **Kiosk-Persistenz.** Verifikation der fehlenden lokalen Speicherung personenbezogener Daten bei wesentlichen Software-Änderungen (Abschnitt 7.4). [PRÜFEN]
13. **Datenträgervernichtung beim Hosting-Anbieter.** Nachweis über den Hosting-Vertrag/AVV beibringen (Abschnitt 11 Nr. 5). [PRÜFEN]

---

*Dieses Dokument ist Teil der DSGVO-Dokumentation von moto. Es ergänzt insbesondere das Verzeichnis der Verarbeitungstätigkeiten (Dokument 04), die Datenbestandsaufnahme (gesondertes internes Dokument), die TOM-Dokumentation (Dokument 02) und das Subprozessoren-Verzeichnis (Dokument 03). Änderungen an Löschjobs, Aufbewahrungseinstellungen oder der Backup-Strategie erfordern eine Aktualisierung dieses Konzepts.*
