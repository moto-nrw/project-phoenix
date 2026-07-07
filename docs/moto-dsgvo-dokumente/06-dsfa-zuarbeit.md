# Zuarbeit zur Datenschutz-Folgenabschätzung gemäß Art. 35 DSGVO

**System: moto (NFC/RFID-gestütztes Anwesenheits- und Raumverwaltungssystem für den Offenen Ganztag)**

| | |
|---|---|
| Dokument | 06 Zuarbeit des Auftragsverarbeiters zur Datenschutz-Folgenabschätzung |
| Version | 1.0 |
| Stand | 2026-07-07 |
| Status | Entwurf zur internen Prüfung |
| Ersteller | [NAME DATENSCHUTZKOORDINATOR], moto [RECHTSFORM UND ADRESSE] |
| Datenschutzbeauftragter des Auftragsverarbeiters | [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKT DATENSCHUTZBEAUFTRAGTER] |
| Adressat | Schulleitung und Schulträger als Verantwortliche, deren behördliche Datenschutzbeauftragte |
| Vertraulichkeit | Intern / zur Weitergabe an den Verantwortlichen bestimmt |

---

## 1. Zweck und Geltungsbereich dieses Dokuments

Dieses Dokument unterstützt die datenschutzrechtlich verantwortliche Stelle (Schule bzw. Schulträger) bei der Erstellung der Datenschutz-Folgenabschätzung (DSFA) gemäß Art. 35 DSGVO für den Einsatz des Systems moto im Offenen Ganztag (OGS).

Es wird ausdrücklich klargestellt:

1. Die **Durchführung, Bewertung und Freigabe der DSFA obliegt ausschließlich dem Verantwortlichen** im Sinne von Art. 4 Nr. 7, Art. 35 Abs. 1 DSGVO, also der Schule bzw. dem Schulträger. Dieses Dokument ist keine DSFA und ersetzt keine DSFA.
2. moto handelt als **Auftragsverarbeiter** gemäß Art. 28 DSGVO. Grundlage dieser Zuarbeit ist die Unterstützungspflicht aus **Art. 28 Abs. 3 lit. f DSGVO**, wonach der Auftragsverarbeiter den Verantwortlichen unter Berücksichtigung der Art der Verarbeitung und der ihm zur Verfügung stehenden Informationen bei der Einhaltung der Pflichten aus Art. 32 bis 36 DSGVO unterstützt. Die entsprechende vertragliche Verankerung findet sich in § 8 Abs. 1 und 5 des Auftragsverarbeitungsvertrags (Dokument 01) vom [DATUM AVV].
3. Der Umfang dieser Zuarbeit ist zweifach begrenzt: auf die Verarbeitungsvorgänge, die moto tatsächlich im Auftrag durchführt, und auf die Informationen, die moto aus technischer Sicht zur Verfügung stehen (Systemarchitektur, Datenkategorien, Datenflüsse, technische und organisatorische Maßnahmen, Unterauftragsverarbeiter). Die rechtliche Prüfung von Notwendigkeit und Verhältnismäßigkeit sowie die abschließende Risikoentscheidung bleiben dem Verantwortlichen vorbehalten. Dieses Dokument enthält keine Rechtsberatung.
4. Sämtliche Risikoeinstufungen in Abschnitt 7 sind **Einschätzungsvorschläge aus technischer Anbietersicht**. Der Verantwortliche hat sie unter Berücksichtigung seiner örtlichen Gegebenheiten (Anzahl der Kinder, Personalstruktur, Räumlichkeiten, Aufstellung der Geräte) eigenständig zu prüfen und gegebenenfalls anzupassen.

Grundlage der technischen Angaben ist der Softwarestand des Systems zum 07.07.2026. Ergänzend wird auf folgende Dokumente der Dokumentenreihe verwiesen: Datenbestandsaufnahme (Datenkategorien und Speicherfristen), TOM-Inventar (Anlage zum AVV) und Unterauftragsverarbeiterliste.

---

## 2. Anlass: Warum eine DSFA voraussichtlich erforderlich ist

Die Entscheidung, ob eine DSFA durchzuführen ist, trifft der Verantwortliche. Aus Sicht des Auftragsverarbeiters sprechen die folgenden Gesichtspunkte dafür, dass die Schwelle des Art. 35 Abs. 1 DSGVO (voraussichtlich hohes Risiko) erreicht wird:

Nach den von den deutschen Aufsichtsbehörden übernommenen Leitlinien der Art.-29-Datenschutzgruppe (WP 248 rev.01) gilt als Faustregel, dass eine DSFA regelmäßig erforderlich ist, wenn zwei oder mehr der dort genannten neun Kriterien zutreffen. Auf den Einsatz von moto treffen mindestens zu:

1. **Daten besonders schutzbedürftiger Betroffener** (Kriterium 7): Die Kernbetroffenen sind Grundschulkinder. Kinder werden in WP 248 ausdrücklich als schutzbedürftige Betroffene genannt.
2. **Systematische Überwachung** (Kriterium 3): Das System erfasst Anwesenheits- und Aufenthaltsdaten der Kinder systematisch und fortlaufend über den gesamten Betreuungszeitraum jedes Betreuungstages.
3. **Innovative Nutzung neuer Technologien** (Kriterium 8): Der Einsatz von NFC/RFID-Kiosk-Geräten zur Anwesenheitserfassung von Kindern in der Schule ist eine im Schulumfeld neuartige Technologie.
4. Gegebenenfalls **Verarbeitung in großem Umfang** (Kriterium 5), wenn mehrere Schulen eines Trägers auf derselben Instanz betrieben werden. Die Einschätzung hierzu obliegt dem Verantwortlichen.

Ergänzend ist auf die Positivlisten nach Art. 35 Abs. 4 DSGVO hinzuweisen:

- Für die Schule als **öffentliche Stelle in NRW** ist primär die Liste der LDI NRW für den öffentlichen Bereich maßgeblich. [PRÜFEN: Der aktuelle Wortlaut der LDI-NRW-Liste ist vor Fertigstellung der DSFA direkt unter ldi.nrw.de abzurufen und die einschlägigen Positionen sind durch den Verantwortlichen zu dokumentieren.]
- Als Auslegungshilfe kann die abgestimmte Liste der Datenschutzkonferenz (DSK) für den nicht-öffentlichen Bereich (Version 1.1, Stand 17.10.2018) herangezogen werden. Dort sind unter Nr. 4 die umfangreiche Verarbeitung von Aufenthaltsdaten natürlicher Personen und unter Nr. 8 die Erstellung von Bewegungsprofilen per RFID (dort am Beispiel Beschäftigter) als DSFA-pflichtige Verarbeitungen genannt. Beide Regelbeispiele liegen inhaltlich nahe an der hier betrachteten Verarbeitung; bei Kindern ist der Schutzbedarf höher anzusetzen als beim Referenzbeispiel Beschäftigte.

Das Fehlen einer Verarbeitung auf einer Positivliste entbindet nicht von der eigenständigen Prüfung nach Art. 35 Abs. 1 DSGVO.

---

## 3. Systematische Beschreibung der Verarbeitung (Zuarbeit zu Art. 35 Abs. 7 lit. a DSGVO)

### 3.1 Systemüberblick

moto ist ein mandantenfähiges Anwesenheits- und Raumverwaltungssystem für den Offenen Ganztag an Grundschulen. Es besteht aus folgenden Komponenten:

| Komponente | Beschreibung |
|---|---|
| Backend | Serveranwendung (Go), zentrale Geschäftslogik und Schnittstellen |
| Datenbank | PostgreSQL 17 mit Row Level Security (RLS) zur Mandantentrennung, 15 fachliche Schemata |
| Web-Frontend | Browseranwendung (Next.js) mit drei strikt getrennten Portalen: Schulportal (Personal/Verwaltung), Betreiberportal (moto), Elternportal |
| Kiosk-Gerät (PyrePortal) | Raspberry-Pi-Gerät mit NFC/RFID-Leser in den Betreuungsräumen bzw. am Eingang; authentifiziert sich gegenüber dem Backend mit einem gerätespezifischen API-Schlüssel, Bedienhandlungen des Personals erfordern zusätzlich eine persönliche PIN |
| Hosting | Server bei Hetzner (Rechenzentrum Nürnberg, Deutschland); TLS-Terminierung über den Reverse Proxy Caddy; DNS/CDN über Cloudflare; Monitoring über selbst gehostetes Grafana/Loki auf demselben Server |

Der Serverbetrieb erfolgt vollständig auf Infrastruktur in Deutschland. Auf dem Kiosk-Gerät selbst werden keine personenbezogenen Datenbestände dauerhaft gespeichert; das Gerät dient als Erfassungs- und Anzeigegerät gegenüber dem Backend.

### 3.2 Ablauf eines RFID-Scans (Kernverarbeitung)

1. Jedes Kind erhält einen passiven RFID/NFC-Tag (Karte oder Anhänger). Auf dem Tag ist ausschließlich die herstellerseitige Tag-Kennung (UID) enthalten; es werden keine Namen, Fotos oder sonstigen Daten auf den Tag geschrieben.
2. Beim Vorhalten des Tags an ein Kiosk-Gerät liest das Gerät die Tag-UID und übermittelt sie zusammen mit der Gerätekennung über eine TLS-verschlüsselte Verbindung an das Backend.
3. Das Backend ordnet die Tag-UID der hinterlegten Person zu (Tabelle `users.rfid_cards` über `users.persons.tag_id`) und erzeugt bzw. schließt einen Anwesenheitsdatensatz: Check-in bzw. Check-out mit Datum, Uhrzeit, erfassendem Gerät und gegebenenfalls der bestätigenden Betreuungskraft (`active.attendance`: `student_id`, `date`, `check_in_time`, `check_out_time`, `checked_in_by`, `checked_out_by`, `device_id`, `yard_since`).
4. Bei Raumwechseln innerhalb der Betreuung wird der aktuelle Aufenthalt als Besuchsdatensatz geführt (`active.visits`: `student_id`, `active_group_id`, `entry_time`, `exit_time`). Dieser Datensatz beantwortet die betriebliche Frage, in welchem Betreuungsangebot bzw. Raum sich ein Kind gerade befindet.
5. Wird ein Tag gescannt, der keiner Person zugeordnet ist, wird der Scan mit Tag-UID, Gerät und Zeitpunkt in einer gesonderten Tabelle protokolliert (`audit.unregistered_tag_scans`), damit Fehlzuordnungen und verwaiste Tags aufgeklärt werden können.

Das System kennt zwei vom Betreiber je Schule konfigurierbare Erfassungsmodi: einen detaillierten Modus mit Raumzuordnung und einen binären Modus (nur anwesend/abwesend am Eingang, ohne Raumauswahl am Kiosk). Der binäre Modus reduziert die erfasste Datentiefe und ist im Rahmen der Verhältnismäßigkeitsprüfung des Verantwortlichen relevant (siehe Abschnitt 6).

Abgrenzung, die für die Risikobeschreibung wesentlich ist: Das System führt **keine Ortung** durch. Es gibt keine GPS-, Funkzellen- oder sonstige kontinuierliche Standortverfolgung. Erfasst werden ausschließlich diskrete Ereignisse (Scan an einem stationären Gerät zu einem Zeitpunkt). Die Tag-UID ist ein technischer Identifikator und **kein biometrisches Merkmal**; biometrische Daten im Sinne von Art. 9 Abs. 1 DSGVO werden nicht verarbeitet.

### 3.3 Zwecke der Verarbeitung

Zweck der Kernverarbeitung ist die Wahrnehmung der Aufsichtspflicht und die ordnungsgemäße Organisation der Betreuung im Offenen Ganztag, insbesondere:

- verlässliche Feststellung, welche Kinder anwesend sind (Anwesenheitskontrolle, Vermisstenfall, Evakuierung),
- Zuordnung der Kinder zu Betreuungsangeboten und Räumen zur Sicherstellung der Aufsicht,
- Abwicklung der Abholung entsprechend den hinterlegten Abholregelungen und Abholberechtigungen,
- Kommunikation mit Erziehungsberechtigten (Krankmeldungen, Nachrichten, Anmeldeverfahren),
- Personaleinsatz und Arbeitszeiterfassung des Betreuungspersonals,
- Nachweis- und Protokollzwecke (Zugriffs-, Änderungs- und Löschprotokolle).

Die abschließende Zweckfestlegung obliegt dem Verantwortlichen.

### 3.4 Betroffenengruppen

1. Schülerinnen und Schüler (Kinder in der Betreuung, besonders schutzbedürftige Betroffene),
2. Eltern und Erziehungsberechtigte (teils mit Elternportal-Zugang),
3. Personal (pädagogische Mitarbeitende, Verwaltung, Vertretungen, Gäste/externe Kursleitungen),
4. Mitarbeitende des Plattformbetreibers (eigener Kontotyp, nicht mandantengebunden),
5. natürliche Personen mit nicht zugeordnetem RFID-Tag (unregistrierte Scans, potentiell re-identifizierbar über die Tag-UID).

### 3.5 Datenkategorien (Zusammenfassung)

Die vollständige, tabellengenaue Aufstellung enthält die Datenbestandsaufnahme (gesondertes Dokument der Reihe). Zusammengefasst:

**Kinder:** Stammdaten (Name, Geburtsdatum, Klasse, Adresse), Gruppen- und Raumzugehörigkeit, Abholregelungen je Wochentag einschließlich Freitextangaben zur Abholung, Krank-/Entschuldigt-Status, Betreuernotizen, Foto (nur mit dokumentierter Einwilligung), Einwilligungsdatensätze, RFID-Tag-Kennung, Anwesenheits- und Aufenthaltsdaten (Check-in/Check-out, Raumbesuche), Feedback-Einträge (z. B. Mensa-Bewertung), Anmeldedaten aus dem Aufnahmeverfahren sowie **Gesundheitsangaben als Freitext** (`users.students.health_info`, siehe 3.6).

**Erziehungsberechtigte:** Name, E-Mail, Adresse, Telefonnummern, Beziehung zum Kind, Abholberechtigung und Notfallkontakt-Kennzeichnung, je Kind-Beziehung gespeicherte Portalberechtigungen, Portalnachrichten mit der Betreuung, Anmeldedaten.

**Personal:** Stammdaten, Beschäftigungsart und Arbeitszeitmodell, Zeiterfassung (Kommen/Gehen, Pausen), Abwesenheiten einschließlich Krankmeldungen, interne Notizen, RFID-Kennung, Konto- und Anmeldedaten.

**Konto- und Protokolldaten (alle Kontoinhaber):** E-Mail, Benutzername, Passwort- und PIN-Hash (Argon2id, nie im Klartext), MFA-Zustand, Passkeys, Anmeldehistorie einschließlich IP-Adresse und User-Agent (`audit.auth_events`), Sitzungs- und Einladungstoken.

**Protokolldaten (Auditschema):** Datenzugriffsprotokoll (`audit.data_access_log`: wer hat wann welche sensiblen Daten welchen Kindes eingesehen), Löschprotokoll, Importprotokoll, Änderungsprotokolle zu Erziehungsberechtigten- und Zeiterfassungsdaten, unregistrierte Tag-Scans.

### 3.6 Besondere Kategorien personenbezogener Daten (Art. 9 DSGVO)

Das System verarbeitet Gesundheitsdaten in folgendem Umfang:

| Datum | Fundstelle | Einordnung |
|---|---|---|
| Gesundheitsangaben zum Kind (Freitext, z. B. Allergien, Medikation) | `users.students.health_info`; kann bereits im Anmeldeformular abgefragt werden (Formularfeld `student.health_info`) | Gesundheitsdaten, Art. 9 |
| Krankmeldungen des Kindes (strukturiert, tagesbezogen) | `active.student_status_days` mit Status `sick`, optional mit Freitextnotiz; Altbestand `users.students.sick/sick_since` | Gesundheitsdaten, Art. 9 |
| Krankmeldungen des Personals | `active.staff_absences` mit `absence_type = sick`, Freitextfelder `note`/`decision_note` | Gesundheitsdaten eines Beschäftigten, Art. 9 |

Die Freitextfelder unterliegen keiner strukturierten inhaltlichen Validierung; welcher Detailgrad tatsächlich erfasst wird, hängt von den Eingaben des Personals bzw. der Eltern ab. Der Verantwortliche sollte den zulässigen Inhalt dieser Felder durch Dienstanweisung eingrenzen (siehe Risiko R8 und Abschnitt 8). Weitere Kategorien nach Art. 9 (biometrische Daten, ethnische Herkunft, Religion u. a.) werden nicht verarbeitet.

### 3.7 Zugriffsmodell: wer sieht was

Der Datenzugriff ist mehrstufig beschränkt. Die folgenden Mechanismen sind im System implementiert und in der TOM-Anlage im Einzelnen beschrieben:

**Mandantenebene:** Jede Schule ist ein eigener Mandant. Die Trennung ist auf Datenbankebene durch Row Level Security erzwungen (Policies auf allen mandantenbezogenen Tabellen, dedizierte Datenbankrollen mit minimalen Rechten, transaktionsgebundene Mandantenkennung je Anfrage). Personal einer Schule kann technisch nicht auf Daten einer anderen Schule zugreifen.

**Portalebene:** Drei getrennte Portale mit jeweils eigener Sitzung, eigenem Cookie und eigenem Berechtigungsumfang (Schulportal, Betreiberportal, Elternportal). Eltern-Sitzungen werden auf Schulportal-Routen serverseitig zurückgewiesen und umgekehrt.

**Rollen- und Rechteebene innerhalb der Schule:** Rollen- und Berechtigungsmodell für Personal und Verwaltung; zusätzlich zwei datenschutzspezifische, je Schule konfigurierbare Sichtbarkeitsregeln:

| Einstellung | Standardwert | Wirkung |
|---|---|---|
| `gdpr.attendance_log_enabled` | **aus** | Historisches Anwesenheitsprotokoll/Raumverlauf ist für Mitarbeitende standardmäßig gar nicht einsehbar; die Freischaltung ist eine bewusste Entscheidung der Schule |
| `gdpr.attendance_log_scope` | nur Gruppenbetreuende | Bei Freischaltung sehen nur die Betreuungskräfte der jeweiligen Gruppe das Protokoll |
| `gdpr.attendance_visible_days` | 30 Tage (max. 365) | Sichtbarkeitsfenster für An-/Abmeldezeiten |
| `gdpr.room_detail_visible_days` | 7 Tage (max. 365) | Kürzeres Sichtbarkeitsfenster für Raumdetails |
| `gdpr.student_data_scope` | nur Gruppenbetreuende | Wer vollständige Kinderdaten (Profil, Aufenthaltsort, Abholpläne, Datenschutzangaben) lesen darf; Schreibzugriff bleibt stets auf Gruppenbetreuende beschränkt |

**Elternportal:** Die Berechtigung eines Elternkontos ist je Kind-Beziehung gespeichert (`users.students_guardians.permissions`). Eine Person kann für ein Kind volle Portalrechte und für ein anderes Kind nur eine Abholberechtigung besitzen. Operative Kennzeichen (Abholberechtigung, Notfallkontakt) begründen keinen Portalzugriff.

**Kiosk-Gerät:** Das Gerät zeigt im Regelbetrieb nur den unmittelbaren Scan-Kontext. Bedienhandlungen des Personals erfordern die persönliche PIN.

**Betreiberportal (moto):** Zugriff nur für Betreiberpersonal mit eigenem Kontotyp, MFA und gesondertem Audit-Log (`platform.operator_audit_log`). Betreiberzugriffe erfolgen ausschließlich zu Betriebs-, Wartungs- und Supportzwecken im Rahmen des AVV.

**Zugriffsprotokollierung:** Lesezugriffe auf Anwesenheits- und Historiendaten werden im Datenzugriffsprotokoll (`audit.data_access_log`) festgehalten (handelndes Konto, Rolle, betroffenes Kind, Datenart, Zeitraum, Zeitpunkt).

### 3.8 Speicher- und Löschfristen

Die Löschung erfolgt automatisiert durch einen täglichen Bereinigungslauf (standardmäßig aktiviert, 02:00 Uhr). Die wesentlichen Fristen sind je Schule konfigurierbar; Standardwerte:

| Datenart | Frist (Standard) | Konfigurationsrahmen |
|---|---|---|
| Besuchs- und Anwesenheitsdaten (`active.visits`, `active.attendance`) | 30 Tage | 1 bis 31 Tage; individuelle Einwilligung je Kind kann eine eigene Frist im selben Rahmen setzen |
| Feedback-Einträge | 90 Tage | 1 bis 365 Tage |
| Zeiterfassung Personal | 730 Tage (2 Jahre) | wählbar bis 2920 Tage, orientiert an arbeits- und steuerrechtlichen Aufbewahrungspflichten |
| Abgeschlossene Betreuungsplan-Termine | 365 Tage | 1 bis 1825 Tage |
| Abgelehnte Anmeldungen | 90 Tage | bis 730 Tage |
| Eltern-Einladungslinks | 48 Stunden | bis 168 Stunden |
| Abgelaufene Sitzungs-, Reset- und Einladungstoken | laufende technische Bereinigung | fest |

**Offener Punkt Speicherbegrenzung:** Für die Tabellen des Auditschemas (`audit.auth_events`, `audit.data_access_log`, `audit.data_deletions`, `audit.data_imports`, `audit.guardian_changes`, `audit.unregistered_tag_scans`) sowie für Elternportal-Nachrichten (`users.parent_messages`) besteht derzeit **keine automatisierte Löschfrist**; `audit.work_session_edits` wird dagegen vom Löschlauf der Zeiterfassung miterfasst (Dokument 05, Abschnitt 6.6). Diese Protokolle dienen der Nachvollziehbarkeit (Eingabekontrolle, Art. 5 Abs. 2 DSGVO), die zeitlich unbegrenzte Aufbewahrung ist jedoch gegen den Grundsatz der Speicherbegrenzung (Art. 5 Abs. 1 lit. e DSGVO) abzuwägen. [PRÜFEN: Festlegung angemessener Aufbewahrungsfristen für Audit-Protokolle und Elternnachrichten, gemeinsam durch Verantwortlichen und Auftragsverarbeiter; Umsetzung im System ist als Maßnahme einzuplanen.] Siehe Risiko R9.

### 3.9 Empfänger und Unterauftragsverarbeiter

Empfänger innerhalb des Verantwortungsbereichs der Schule sind das Betreuungspersonal und die Verwaltung nach Maßgabe des Zugriffsmodells (3.7) sowie die Erziehungsberechtigten für die Daten ihres Kindes über das Elternportal.

Unterauftragsverarbeiter und Dienstleister (Details in der gesonderten Unterauftragsverarbeiterliste):

| Dienst | Rolle | Drittlandbezug |
|---|---|---|
| Hetzner (Nürnberg, Deutschland) | Serverhosting, gesamte Datenhaltung | Nein; Nachweis über AVV und Zertifikate des Rechenzentrumsbetreibers [PRÜFEN: aktueller Hetzner-AVV und Zertifikatsstand beizulegen] |
| Cloudflare | DNS/CDN vor dem System; zusätzlich optionaler Bot-Schutz (Turnstile) für das öffentliche Anmeldeformular, standardmäßig deaktiviert | Ja (US-Anbieter); bei aktiviertem Turnstile erhält Cloudflare IP-Adresse und Browsersignale der Erziehungsberechtigten [PRÜFEN: Cloudflare-DPA und Nutzung der EU-Datenlokalisierung] |
| GitHub/GHCR (Microsoft) | Quellcode, Build-Artefakte, Container-Images; kein Zugriff auf personenbezogene Produktivdaten | Ja (US-Anbieter) [PRÜFEN: DPF-Zertifizierungsstatus] |
| SMTP-Versanddienst | Versand von Einladungs-, Reset-, MFA- und Benachrichtigungs-E-Mails (E-Mail-Adressen, Namen, Token, ggf. Kindername in Anmeldebenachrichtigungen) | [PRÜFEN: konkreter SMTP-Anbieter und Serverstandort sind zu benennen] |
| Sentry (Fehlerdiagnose, optional) | Fehler- und Ausnahmeberichte; im System ist eine Bereinigung implementiert, die Autorisierungs-Header, Cookies, IP-Adresse, E-Mail und Benutzername vor Versand entfernt | [PRÜFEN: tatsächliche Nutzung der EU-Region im Sentry-Konto] |
| PostHog (Nutzungsanalyse, optional) | Nutzungsereignisse des Personals im Frontend; als EU-Cloud konfiguriert | [PRÜFEN: produktiver Konfigurationswert und Event-Inhalte, insbesondere Ausschluss von Kinderdaten in Ereignis-Eigenschaften] |
| Grafana/Loki/Prometheus (selbst gehostet) | Monitoring und Protokollaggregation auf eigener Infrastruktur; IP-Maskierung und Entfernung von Zugangsdaten aus Zugriffslogs implementiert | Nein |

Anwendungsprotokolle enthalten auf Info-Ebene keine Klarnamen von Kindern (nur technische Kennungen); dies ist als Projektregel im Entwicklungsprozess verankert.

### 3.10 Rechtsgrundlage (Kontextinformation für den Verantwortlichen)

Die Bestimmung der Rechtsgrundlage obliegt dem Verantwortlichen. Nach dem Einsatzkonzept ist für den Kernbetrieb (Anwesenheitserfassung, Raumzuordnung, Abholorganisation) folgende Grundlage vorgesehen:

- **Art. 6 Abs. 1 lit. e DSGVO** (Wahrnehmung einer im öffentlichen Interesse liegenden Aufgabe) in Verbindung mit **§§ 120 bis 122 SchulG NRW**; die Zulässigkeit der einzelnen Datenkategorien richtet sich nach der **VO-DV I** (Verordnung über die zur Verarbeitung zugelassenen Daten von Schülerinnen und Schülern sowie Eltern). [PRÜFEN: Abgleich der in Abschnitt 3.5 aufgeführten Datenkategorien mit dem Datenkatalog der VO-DV I durch den Verantwortlichen; insbesondere für Foto, Gesundheitsangaben und Feedback-Einträge ist zu klären, ob eine Einwilligung nach Art. 6 Abs. 1 lit. a bzw. Art. 9 Abs. 2 lit. a DSGVO als ergänzende Grundlage erforderlich ist.]
- Für die Verarbeitung von **Gesundheitsdaten** (3.6) ist zusätzlich eine Ausnahme nach **Art. 9 Abs. 2 DSGVO** erforderlich. [PRÜFEN: Einschlägige lit. (insbesondere lit. a Einwilligung oder landesrechtliche Grundlage) durch den Verantwortlichen festlegen und dokumentieren.]
- Einzelne Funktionen stützen sich bereits systemseitig auf dokumentierte **Einwilligungen** (Foto: `photo_consent_given_at/by`; Datenschutz-Einwilligung mit individueller Aufbewahrungsfrist: `users.privacy_consents`).
- Für die Zeiterfassung des Personals kommt Art. 6 Abs. 1 lit. b und c DSGVO in Verbindung mit arbeitsrechtlichen Vorschriften in Betracht; die Einordnung obliegt dem Verantwortlichen bzw. dem Träger als Arbeitgeber.

Da die Kernverarbeitung auf Art. 6 Abs. 1 lit. e DSGVO gestützt werden soll, ist das Widerspruchsrecht nach Art. 21 DSGVO in der Betroffeneninformation zu berücksichtigen; die Bewertung obliegt dem Verantwortlichen.

---

## 4. Angaben zur Notwendigkeit und Verhältnismäßigkeit (Zuarbeit zu Art. 35 Abs. 7 lit. b DSGVO)

Die Bewertung von Notwendigkeit und Verhältnismäßigkeit ist Sache des Verantwortlichen. Aus technischer Sicht werden folgende Gestaltungsmerkmale des Systems zur Verfügung gestellt, die in diese Bewertung einfließen können:

1. **Abstufbare Datentiefe:** Der binäre Erfassungsmodus (nur anwesend/abwesend) steht als datensparsamere Alternative zum detaillierten Modus mit Raumzuordnung zur Verfügung. Die Schule kann den Modus wählen, der ihrem Aufsichtskonzept entspricht.
2. **Standardmäßig restriktive Sichtbarkeit:** Das historische Anwesenheitsprotokoll ist ab Werk deaktiviert; Sichtbarkeitsfenster und berechtigter Personenkreis sind eng voreingestellt und nur bewusst erweiterbar (3.7).
3. **Kurze Regelaufbewahrung der Bewegungsdaten:** Besuchs- und Anwesenheitsdaten werden standardmäßig nach 30 Tagen automatisiert gelöscht; die Obergrenze liegt bei 31 Tagen (3.8). Eine langfristige Historie der Aufenthalte entsteht im Regelbetrieb nicht.
4. **Datenarme Tags:** Auf dem RFID-Tag selbst befinden sich keine personenbezogenen Daten; die Zuordnung erfolgt ausschließlich serverseitig. Ein verlorener Tag offenbart für sich genommen keine Daten und kann im System deaktiviert werden.
5. **Zweckgetrennte Portale und Berechtigungen:** Eltern sehen nur die eigenen Kinder gemäß gespeicherter Beziehung; Personal sieht standardmäßig nur die eigene Gruppe; der Betreiber hat keinen fachlichen Regelzugriff.
6. **Alternativenbetrachtung:** Die Alternative zur elektronischen Erfassung ist die papiergebundene Anwesenheitsliste. Deren Schwächen (fehlende Zugriffskontrolle, keine Löschautomatik, keine Protokollierung der Einsichtnahme, Verlustrisiko) sollten in der Verhältnismäßigkeitsabwägung des Verantwortlichen den Risiken der elektronischen Verarbeitung gegenübergestellt werden. Die Abwägung selbst trifft der Verantwortliche.

---

## 5. Methodik der Risikoanalyse

Die Risiken werden aus der **Sicht der betroffenen Personen** betrachtet (Erwägungsgründe 75 und 76 DSGVO), nicht aus Sicht der Organisation. Bewertet werden je Risiko:

- **Eintrittswahrscheinlichkeit** unter Berücksichtigung der implementierten Maßnahmen: gering / mittel / hoch,
- **Schwere** der möglichen Folgen für die Betroffenen: gering / mittel / hoch,
- die **implementierten Abhilfemaßnahmen** aus dem TOM-Inventar (Anlage zum AVV, dort mit Fundstellen im Quellcode belegt),
- das aus technischer Sicht verbleibende **Restrisiko**.

Die Einstufungen sind Vorschläge des Auftragsverarbeiters auf Grundlage der Systemarchitektur. Örtliche Faktoren (z. B. Geräteaufstellung, Personalfluktuation, Schulgröße) kann nur der Verantwortliche bewerten.

---

## 6. Risikoanalyse aus Betroffenensicht mit Abhilfemaßnahmen (Zuarbeit zu Art. 35 Abs. 7 lit. c und d DSGVO)

### R1: Erstellung von Bewegungs- und Verhaltensprofilen von Kindern

**Beschreibung:** Die fortlaufende Erfassung von Check-in, Check-out und Raumbesuchen könnte über längere Zeiträume zu einem Profil verdichtet werden (wann kommt das Kind, mit welchen Angeboten verbringt es Zeit, wann wird es abgeholt). Ein solches Profil beträfe ein Kind und wäre geeignet, Rückschlüsse auf Familienverhältnisse und Verhalten zu ermöglichen.

**Betroffene:** Kinder (besonders schutzbedürftig). **Schwere:** hoch. **Eintrittswahrscheinlichkeit (mit Maßnahmen):** gering.

**Implementierte Abhilfemaßnahmen:**

- Automatisierte Löschung der Besuchs- und Anwesenheitsdaten nach standardmäßig 30, maximal 31 Tagen; eine langfristige Profilbildung ist damit datenseitig ausgeschlossen, sofern die Bereinigung aktiv ist (Standard: aktiv).
- Historisches Anwesenheitsprotokoll standardmäßig vollständig deaktiviert (`gdpr.attendance_log_enabled = aus`); bei Aktivierung Sichtbarkeit nur für Gruppenbetreuende und nur innerhalb der konfigurierten Fenster (30 Tage An-/Abmeldezeiten, 7 Tage Raumdetails).
- Binärer Erfassungsmodus als datensparsamere Betriebsart ohne Raumzuordnung wählbar.
- Keine Ortungstechnik; ausschließlich diskrete Scan-Ereignisse an stationären Geräten (3.2).
- Jeder Lesezugriff auf Historiendaten wird im Datenzugriffsprotokoll festgehalten und ist damit nachträglich kontrollierbar.

**Restrisiko:** gering, solange die Standardkonfiguration beibehalten bzw. Abweichungen dokumentiert begründet werden. Die Entscheidung über Erfassungsmodus und Protokollfreischaltung ist eine risikorelevante Konfigurationsentscheidung des Verantwortlichen und sollte in der DSFA ausdrücklich festgehalten werden.

### R2: Unberechtigte Kenntnisnahme durch internes Personal

**Beschreibung:** Betreuungs- oder Verwaltungspersonal könnte Daten von Kindern einsehen, für die keine dienstliche Erforderlichkeit besteht (Kinder anderer Gruppen, Gesundheitsangaben, Abholregelungen, Historie).

**Betroffene:** Kinder, Erziehungsberechtigte. **Schwere:** mittel bis hoch (bei Gesundheits- und Sorgerechtskontexten hoch). **Eintrittswahrscheinlichkeit (mit Maßnahmen):** gering bis mittel.

**Implementierte Abhilfemaßnahmen:**

- Rollen- und Berechtigungsmodell; Sichtbarkeit vollständiger Kinderdaten standardmäßig auf Gruppenbetreuende beschränkt (`gdpr.student_data_scope`), Schreibzugriff stets auf Gruppenbetreuende beschränkt.
- Datenzugriffsprotokoll (`audit.data_access_log`) über Lesezugriffe auf sensible Daten mit handelndem Konto, Rolle, Kind und Zeitraum; abschreckende und aufklärende Wirkung.
- Persönliche Konten mit Passwort (Argon2id-Hashing), optional Passkeys; MFA-Subsystem; konfigurierbare Kontosperre nach Fehlversuchen; PIN-Pflicht für Personalaktionen am Kiosk.
- Änderungsprotokolle für Erziehungsberechtigten- und Zeiterfassungsdaten (append-only).

**Restrisiko:** mittel. Technische Maßnahmen begrenzen den Zugriff, ersetzen aber nicht organisatorische Maßnahmen des Verantwortlichen: Vertraulichkeitsverpflichtung nach Art. 29, 32 Abs. 4 DSGVO, Dienstanweisung zur Nutzung, regelmäßige Kontrolle des Zugriffsprotokolls, zeitnahe Deaktivierung ausscheidender Mitarbeitender. Diese Maßnahmen sind in der DSFA als Pflichten des Verantwortlichen aufzuführen.

### R3: Unberechtigter Zugriff von außen (Angriff, Diebstahl von Zugangsdaten)

**Beschreibung:** Externe Angreifer könnten über gestohlene Zugangsdaten, Schwachstellen oder Brute-Force-Angriffe Zugriff auf Kinder- und Elterndaten erlangen. Wegen der Datenkategorien (Kinderdaten, Abholregelungen, Adressen, Gesundheitsangaben) wären die Folgen erheblich.

**Betroffene:** alle Gruppen. **Schwere:** hoch. **Eintrittswahrscheinlichkeit (mit Maßnahmen):** gering.

**Implementierte Abhilfemaßnahmen:**

- Transportverschlüsselung durchgehend (TLS am Reverse Proxy; Datenbankverbindungen mit SSL und Zertifikatsprüfung in Produktion).
- Passwort-Hashing mit Argon2id (64 MB Speicher, 3 Iterationen, Salt, zeitkonstanter Vergleich); PINs ausschließlich als Hash.
- MFA für Schul- und Betreiberportal einschließlich E-Mail-Challenges, Recovery und begrenzt gültiger vertrauenswürdiger Geräte; Kontosperre nach konfigurierbarer Fehlversuchsschwelle.
- Kurzlebige JWT-Zugriffstoken (15 Minuten) mit Refresh-Mechanismus; kein statisches Sitzungsgeheimnis im Code, Startabbruch bei fehlender sicherer Konfiguration.
- Rate Limiting auf Anmelde-, Passwort-Reset- und Anmeldeformular-Endpunkten.
- Kiosk-Geräte authentifizieren sich mit gerätespezifischem API-Schlüssel (zeitkonstanter Vergleich, Statusprüfung); zusätzlich Personal-PIN für Bedienaktionen.
- Least-Privilege-Datenbankrollen; der Anwendungsserver verbindet sich nicht als Superuser.
- Secrets ausschließlich verschlüsselt verwaltet (SOPS/age), keine Klartext-Zugangsdaten im Code; Anmeldehistorie mit IP und Ereignistyp (`audit.auth_events`) zur Angriffserkennung.
- Sicherheitsrelevante Header (Autorisierung, Cookies, Geräteschlüssel, PIN) werden aus Zugriffsprotokollen entfernt, IP-Adressen dort maskiert.

**Restrisiko:** gering bis mittel. Kein System ist gegen Angriffe vollständig geschützt; das Restrisiko wird zusätzlich durch Meldeprozesse (Art. 33, 34 DSGVO, im AVV geregelt) und Wiederherstellungsfähigkeit (R10) begrenzt. [PRÜFEN: Turnus externer Sicherheitsüberprüfungen/Penetrationstests festlegen und dokumentieren; im Repository ist kein solcher Nachweis enthalten.]

### R4: Mandantenübergreifender Datenzugriff (Vermischung zwischen Schulen)

**Beschreibung:** Auf einer gemeinsam betriebenen Instanz könnten Daten einer Schule für Personal einer anderen Schule sichtbar werden (Programmierfehler, fehlerhafte Abfrage).

**Betroffene:** Kinder, Eltern, Personal aller Schulen der Instanz. **Schwere:** hoch. **Eintrittswahrscheinlichkeit (mit Maßnahmen):** gering.

**Implementierte Abhilfemaßnahmen:**

- Row Level Security auf Datenbankebene mit erzwungenen Policies (`FORCE ROW LEVEL SECURITY`) auf allen mandantenbezogenen Tabellen; die Mandantentrennung hängt damit nicht allein von der Korrektheit des Anwendungscodes ab.
- Getrennte Datenbankrollen: die reguläre Anwendungsrolle unterliegt zwingend der RLS; eine RLS-umgehende Rolle ist auf Betreiber-, Migrations- und Wartungspfade beschränkt.
- Je Anfrage transaktionsgebundene Mandantenkennung mit automatischem Rollback bei Serverfehlern; Mandantenkennung als Pflichtfeld auf allen betroffenen Datenmodellen.
- Portaltrennung mit eigenem Sitzungs-Cookie je Portal; Token mit Eltern-Geltungsbereich werden auf Schulrouten serverseitig zurückgewiesen (mehrstufige Absicherung).
- Automatisierte Architektur-Prüfungen im Entwicklungsprozess, die das Umgehen der Datenzugriffsschicht verhindern.

**Restrisiko:** gering. Die Kombination aus datenbankseitiger Erzwingung und Anwendungslogik gilt als belastbare Trennung; verbleibendes Risiko sind Implementierungsfehler in Betreiber-Funktionen, die durch das Betreiber-Audit-Log nachvollziehbar bleiben.

### R5: Verknüpfbarkeit und Re-Identifizierung über die RFID-Tag-Kennung

**Beschreibung:** Die Tag-UID ist ein pseudonymes, dauerhaftes Kennzeichen. Wer die UID eines Kindes kennt und Lesezugriff auf Scandaten erlangt, kann Ereignisse einem Kind zuordnen. UIDs handelsüblicher Tags können zudem von Dritten mit eigenem Lesegerät im Vorbeigehen ausgelesen werden. Nicht zugeordnete Scans (`audit.unregistered_tag_scans`) betreffen potentiell unbeteiligte Dritte und sind derzeit ohne Löschfrist gespeichert.

**Betroffene:** Kinder, Personal, Dritte mit RFID-Tags. **Schwere:** mittel. **Eintrittswahrscheinlichkeit:** gering.

**Implementierte Abhilfemaßnahmen:**

- Auf dem Tag befinden sich keine personenbezogenen Daten; die Zuordnung UID zu Person existiert nur serverseitig und unterliegt dem Zugriffsmodell (3.7) und der Mandantentrennung (R4).
- Die Zuordnungstabelle ist nur für berechtigtes Verwaltungspersonal zugänglich; Scans ohne Zuordnung werden getrennt gespeichert und erst durch berechtigte Auflösung einer Person zugeordnet.
- Verlorene oder entwendete Tags können deaktiviert und neu ausgegeben werden, ohne dass Altdaten auf dem Tag verbleiben.
- Die kurze Aufbewahrung der Bewegungsdaten (R1) begrenzt den Wert einer erbeuteten UID erheblich.

**Restrisiko:** gering bis mittel. Das Auslesen der bloßen UID durch Dritte lässt sich bei passiven Tags technisch nicht ausschließen, führt aber ohne Zugriff auf die serverseitige Zuordnung nicht zu einer Identifizierung. [PRÜFEN: Löschfrist für `audit.unregistered_tag_scans` festlegen; siehe R9.]

### R6: Datenabfluss an Dritte und Drittlandübermittlung

**Beschreibung:** Über eingebundene Dienstleister könnten personenbezogene Daten in Drittländer gelangen oder von Dritten eingesehen werden (E-Mail-Versand, CDN, optionale Diagnose- und Analysedienste, Bot-Schutz beim öffentlichen Anmeldeformular).

**Betroffene:** vor allem Erziehungsberechtigte (E-Mail, IP-Adresse), mittelbar Kinder (Namen in Benachrichtigungen). **Schwere:** mittel. **Eintrittswahrscheinlichkeit:** mittel (dienstabhängig).

**Implementierte Abhilfemaßnahmen:**

- Primäre Datenhaltung vollständig auf Servern in Deutschland (Hetzner Nürnberg); Monitoring und Protokollaggregation selbst gehostet, kein Log-Versand an Dritte.
- Bot-Schutz (Cloudflare Turnstile) standardmäßig deaktiviert und nur je Schule aktivierbar; Diagnose- (Sentry) und Analysediensten (PostHog) sind optional; für Sentry ist eine automatische Entfernung von IP-Adresse, E-Mail, Benutzername und Autorisierungsdaten vor Versand implementiert; PostHog ist auf die EU-Cloud konfiguriert.
- Schriftarten werden zur Bauzeit eingebettet und selbst ausgeliefert; zur Laufzeit erfolgt kein Abruf bei Google.
- Unterauftragsverarbeiter sind in der gesonderten Liste mit Zweck, Datenkategorien und Drittlandbewertung dokumentiert; Einbindung neuer Unterauftragsverarbeiter nach Maßgabe des AVV (Informations- und Widerspruchsverfahren).

**Restrisiko:** mittel, bis die in der Unterauftragsverarbeiterliste markierten Punkte geklärt sind: [PRÜFEN: SMTP-Anbieter und Serverstandort; Cloudflare-Datenlokalisierung; Sentry-Projektregion; produktive PostHog-Konfiguration und Event-Inhalte; DPF-Status von GitHub/Microsoft; Ausschluss eines CDN-Rückfalls bei der Schriftarten-Auslieferung.] Der Verantwortliche sollte die Aktivierung der optionalen Dienste (Turnstile, Sentry, PostHog) als eigene Entscheidung in der DSFA dokumentieren.

### R7: Missbrauch oder Verlust des Kiosk-Geräts

**Beschreibung:** Ein Kiosk-Gerät könnte entwendet, manipuliert oder durch Unbefugte (auch Kinder) bedient werden; der Geräteschlüssel könnte extrahiert und für API-Zugriffe missbraucht werden.

**Betroffene:** Kinder. **Schwere:** mittel. **Eintrittswahrscheinlichkeit:** gering.

**Implementierte Abhilfemaßnahmen:**

- Keine dauerhafte lokale Speicherung personenbezogener Datenbestände auf dem Gerät; das Gerät zeigt nur den unmittelbaren Scan-Kontext.
- Gerätespezifischer API-Schlüssel mit serverseitiger Statusprüfung: ein kompromittiertes Gerät kann zentral deaktiviert werden; der Schlüssel berechtigt nur zu den Kiosk-Endpunkten, nicht zum Verwaltungszugriff.
- Bedienaktionen des Personals (über den reinen Scan hinaus) erfordern die persönliche PIN; PIN-Vergleich zeitkonstant, PIN-Speicherung nur als Hash.
- Geräteaktivität wird serverseitig protokolliert (letzter Kontakt, ohne personenbezogene Inhalte im Log).

**Restrisiko:** gering. Die physische Sicherung der Geräte (Aufstellort, Befestigung, Beaufsichtigung) liegt im Verantwortungsbereich der Schule und ist in der DSFA als organisatorische Maßnahme des Verantwortlichen zu ergänzen.

### R8: Unbefugte Offenlegung von Gesundheitsdaten (Art. 9)

**Beschreibung:** Gesundheitsangaben zum Kind (Freitext, Krankmeldungen) und Krankmeldungen des Personals sind besonders sensibel. Risiken sind die Einsichtnahme durch nicht erforderliche Personen und die unkontrollierte Anreicherung der Freitextfelder mit übermäßigen Details.

**Betroffene:** Kinder, Personal. **Schwere:** hoch. **Eintrittswahrscheinlichkeit (mit Maßnahmen):** gering bis mittel.

**Implementierte Abhilfemaßnahmen:**

- Zugriff auf vollständige Kinderdaten einschließlich Gesundheitsangaben standardmäßig nur für Gruppenbetreuende (`gdpr.student_data_scope`); Lesezugriffe werden protokolliert.
- Krankmeldungen des Kindes sind tagesbezogen strukturiert; das Elternportal erlaubt die Meldung nur für Kinder mit entsprechender gespeicherter Berechtigung der meldenden Person.
- Personal-Abwesenheiten unterliegen dem Rollenmodell; Genehmigungsvermerke werden personenbezogen protokolliert.
- Mandantentrennung, Verschlüsselung und Zugangskontrollen gemäß R3/R4 gelten uneingeschränkt auch für diese Daten.

**Restrisiko:** mittel. Freitextfelder lassen sich technisch nicht auf das Erforderliche begrenzen. Der Verantwortliche sollte per Dienstanweisung festlegen, welche Angaben in `health_info` und in Notizfeldern zulässig sind (Datenminimierung bei der Eingabe), und die Rechtsgrundlage nach Art. 9 Abs. 2 DSGVO dokumentieren (3.10). [PRÜFEN: Rechtsgrundlage Art. 9 Abs. 2 und Eingaberichtlinie des Verantwortlichen.]

### R9: Überlange Speicherung von Protokoll- und Nachrichtendaten

**Beschreibung:** Audit-Protokolle (Anmeldehistorie mit IP-Adressen, Datenzugriffs-, Änderungs- und Löschprotokolle, unregistrierte Tag-Scans) sowie Elternportal-Nachrichten haben derzeit keine automatisierte Löschfrist. Aus Betroffenensicht entsteht das Risiko einer zeitlich unbegrenzten Nachvollziehbarkeit von Verhalten (Anmeldezeiten des Personals, Kommunikationsinhalte der Eltern).

**Betroffene:** Personal, Eltern, mittelbar Kinder (Nachrichteninhalte). **Schwere:** mittel. **Eintrittswahrscheinlichkeit:** hoch (der Zustand besteht, solange keine Frist definiert ist).

**Implementierte Abhilfemaßnahmen:**

- Die betroffenen Tabellen unterliegen der Mandantentrennung, dem Rollenmodell und sind append-only; ein fachlicher Regelzugriff auf sie besteht nicht.
- Anmeldeprotokolle enthalten keine Klartext-Zugangsdaten; Anwendungslogs enthalten auf Info-Ebene keine Kindernamen; IP-Maskierung in Zugriffslogs des Reverse Proxy.
- Für alle übrigen Datenarten bestehen automatisierte, konfigurierbare Löschfristen (3.8), der Bereinigungsmechanismus ist vorhanden und erprobt.

**Restrisiko:** mittel und derzeit nicht abschließend behandelt. Dies ist der wesentliche bekannte offene Punkt dieser Zuarbeit. [PRÜFEN: Verantwortlicher und Auftragsverarbeiter legen Aufbewahrungsfristen für das Auditschema und Elternnachrichten fest (Vorschlagsbasis: Erforderlichkeit für Nachweiszwecke nach Art. 5 Abs. 2 DSGVO gegen Speicherbegrenzung nach Art. 5 Abs. 1 lit. e DSGVO abwägen); die technische Umsetzung wird von moto als Maßnahme eingeplant und der Umsetzungsstand dem Verantwortlichen mitgeteilt.]

### R10: Verlust von Verfügbarkeit oder Integrität (Auswirkung auf die Aufsicht)

**Beschreibung:** Fällt das System aus oder werden Daten verfälscht, kann die Schule sich bei der Aufsicht nicht auf die Anwesenheitsdaten verlassen. Aus Betroffenensicht droht mittelbar eine Gefährdung des Kindeswohls (Kind gilt fälschlich als anwesend oder abwesend).

**Betroffene:** Kinder. **Schwere:** hoch (mittelbar). **Eintrittswahrscheinlichkeit:** gering.

**Implementierte Abhilfemaßnahmen:**

- Automatisierte Datenbanksicherung vor jeder Softwareaktualisierung mit definiertem Rollback-Verfahren; Abbruch der Aktualisierung bei fehlgeschlagener Sicherung; Sicherungsaufbewahrung mit Rotationsregel; restriktive Dateiberechtigungen auf Sicherungen.
- Aktive Verfügbarkeitsüberwachung (Health-Checks im Minutentakt, Alarmierung), selbst gehostetes Monitoring.
- Konsistenzmechanismen auf Datenbankebene (Transaktionen je Anfrage mit automatischem Rollback bei Fehlern); Änderungsprotokolle ermöglichen die Nachvollziehbarkeit nachträglicher Korrekturen.

**Restrisiko:** gering. Organisatorische Rückfallebene des Verantwortlichen (Verfahren bei Systemausfall, z. B. papiergebundene Notliste) sollte im Betreuungskonzept beschrieben und in der DSFA erwähnt werden.

### Risikoübersicht

| Nr. | Risiko | Schwere | Eintrittswahrscheinlichkeit (mit Maßnahmen) | Restrisiko (Vorschlag) |
|---|---|---|---|---|
| R1 | Bewegungs-/Verhaltensprofile von Kindern | hoch | gering | gering |
| R2 | Unberechtigte interne Kenntnisnahme | mittel bis hoch | gering bis mittel | mittel |
| R3 | Externer Angriff / Zugangsdatendiebstahl | hoch | gering | gering bis mittel |
| R4 | Mandantenübergreifender Zugriff | hoch | gering | gering |
| R5 | Verknüpfbarkeit über Tag-UID | mittel | gering | gering bis mittel |
| R6 | Datenabfluss an Dritte / Drittland | mittel | mittel | mittel (bis Klärung der Prüfpunkte) |
| R7 | Missbrauch/Verlust Kiosk-Gerät | mittel | gering | gering |
| R8 | Offenlegung von Gesundheitsdaten | hoch | gering bis mittel | mittel |
| R9 | Überlange Speicherung von Protokolldaten | mittel | hoch | mittel (offener Punkt) |
| R10 | Verfügbarkeits-/Integritätsverlust | hoch | gering | gering |

---

## 7. Restrisiko und Konsultation (Art. 36 DSGVO)

Nach Einschätzung des Auftragsverarbeiters lässt sich bei Beibehaltung der restriktiven Standardkonfiguration und Umsetzung der beim Verantwortlichen liegenden organisatorischen Maßnahmen kein fortbestehendes hohes Restrisiko erkennen. Diese Einschätzung ersetzt nicht die eigene Bewertung des Verantwortlichen. Kommt der Verantwortliche in seiner DSFA zu einem fortbestehenden hohen Restrisiko, ist vor Aufnahme bzw. Fortführung der Verarbeitung die Aufsichtsbehörde zu konsultieren (Art. 36 DSGVO). Zuständige Aufsichtsbehörde für die Schule ist die Landesbeauftragte für Datenschutz und Informationsfreiheit Nordrhein-Westfalen (LDI NRW). moto unterstützt eine etwaige Konsultation im Rahmen von Art. 28 Abs. 3 lit. f DSGVO mit den erforderlichen technischen Auskünften.

---

## 8. Beim Verantwortlichen liegende Maßnahmen und Entscheidungen

Für die Vollständigkeit der DSFA weist der Auftragsverarbeiter auf folgende Punkte hin, die nur der Verantwortliche regeln kann:

1. Einbindung des behördlichen Datenschutzbeauftragten der Schule bzw. des Trägers (Art. 35 Abs. 2 DSGVO) und Entscheidung über die Einholung des Standpunkts betroffener Personen bzw. ihrer Vertretungen, etwa der Schulkonferenz oder Elternvertretung (Art. 35 Abs. 9 DSGVO).
2. Abgleich der Datenkategorien mit der VO-DV I und Dokumentation der Rechtsgrundlagen einschließlich Art. 9 Abs. 2 DSGVO für Gesundheitsdaten (3.10).
3. Entscheidung und Begründung zu risikorelevanten Konfigurationen: Erfassungsmodus (binär/detailliert), Freischaltung und Reichweite des Anwesenheitsprotokolls, Sichtbarkeitsumfang der Kinderdaten, Aufbewahrungsfristen innerhalb der konfigurierbaren Rahmen, Aktivierung optionaler Dienste (Bot-Schutz, Fehlerdiagnose, Nutzungsanalyse).
4. Organisatorische Maßnahmen vor Ort: Vertraulichkeitsverpflichtungen des Personals, Dienstanweisung zur Systemnutzung und zu zulässigen Inhalten von Freitextfeldern, Verfahren zur zeitnahen Deaktivierung ausscheidender Nutzerkonten, regelmäßige Auswertung des Datenzugriffsprotokolls, physische Sicherung der Kiosk-Geräte, Ausfallverfahren.
5. Betroffeneninformation nach Art. 13/14 DSGVO einschließlich Hinweis auf das Widerspruchsrecht nach Art. 21 DSGVO sowie Verfahren zur Bedienung der Betroffenenrechte (Auskunft, Berichtigung, Löschung); moto stellt hierfür die technischen Funktionen bereit (u. a. Datenänderungsanfragen der Eltern, dokumentierte Löschung mit Löschprotokoll).
6. Aufnahme der Verarbeitung in das Verzeichnis der Verarbeitungstätigkeiten des Verantwortlichen (Art. 30 Abs. 1 DSGVO).
7. Überprüfung und Fortschreibung der DSFA bei wesentlichen Änderungen (Art. 35 Abs. 11 DSGVO). moto informiert den Verantwortlichen über wesentliche Systemänderungen, die die Beschreibung in Abschnitt 3 berühren, und aktualisiert diese Zuarbeit entsprechend.

---

## 9. Anlagen und referenzierte Dokumente

1. Auftragsverarbeitungsvertrag zwischen [NAME SCHULTRÄGER] und moto [RECHTSFORM UND ADRESSE] vom [DATUM AVV], einschließlich TOM-Anlage
2. Datenbestandsaufnahme (Datenkategorien, Tabellen, Speicherfristen), gesondertes internes Dokument; Speicher- und Löschfristen zusätzlich in Dokument 05 (Löschkonzept) der Reihe
3. TOM-Inventar mit Quellcode-Fundstellen, Dokument 02 der Reihe
4. Unterauftragsverarbeiterliste, Dokument 03 der Reihe
5. Verzeichnis der Verarbeitungstätigkeiten, Dokument 04 der Reihe
6. Liste der LDI NRW nach Art. 35 Abs. 4 DSGVO für den öffentlichen Bereich [AKTUELLE FASSUNG BEIFÜGEN]
7. DSK-Kurzpapier Nr. 5 (Datenschutz-Folgenabschätzung, Stand 17.12.2018) und DSK-Liste für den nicht-öffentlichen Bereich (Version 1.1, Stand 17.10.2018) als methodische Referenz

---

## 10. Offene Punkte (Sammelübersicht)

| Nr. | Punkt | Zuständig |
|---|---|---|
| 1 | Aktuellen Wortlaut der LDI-NRW-Positivliste (öffentlicher Bereich) beschaffen und einschlägige Positionen dokumentieren | Verantwortlicher, mit Unterstützung moto |
| 2 | Abgleich der Datenkategorien mit der VO-DV I; Rechtsgrundlage für Foto, Gesundheitsangaben, Feedback klären (Art. 6 Abs. 1 lit. a, Art. 9 Abs. 2 DSGVO) | Verantwortlicher |
| 3 | Aufbewahrungsfristen für Auditschema (`audit.*`) und Elternnachrichten festlegen; technische Umsetzung einplanen | gemeinsam; Umsetzung moto |
| 4 | SMTP-Anbieter und Serverstandort benennen und in der Unterauftragsverarbeiterliste nachtragen | moto |
| 5 | Cloudflare: DPA-Konditionen und Nutzung der EU-Datenlokalisierung prüfen (relevant bei aktiviertem Turnstile) | moto |
| 6 | Sentry: tatsächliche Projektregion (EU) im Konto verifizieren | moto |
| 7 | PostHog: produktiven Konfigurationswert (EU-Cloud) bestätigen und Event-Inhalte auf Ausschluss von Kinderdaten prüfen | moto |
| 8 | GitHub/GHCR: DPF-Zertifizierungsstatus dokumentieren | moto |
| 9 | Schriftarten-Auslieferung: Ausschluss eines Laufzeit-Rückfalls auf externe Google-Server in der Build-Pipeline bestätigen | moto |
| 10 | Hetzner-AVV und aktuelle Rechenzentrumszertifikate als Anlage beifügen | moto |
| 11 | Turnus externer Sicherheitsüberprüfungen/Penetrationstests festlegen und Nachweise führen | moto |
| 12 | Dienstanweisung zu Freitextfeldern (Gesundheitsangaben, Notizen) erstellen | Verantwortlicher |
| 13 | Firmen-Stammdaten und Ansprechpartner in diesem Dokument vervollständigen ([RECHTSFORM UND ADRESSE], [NAME DATENSCHUTZKOORDINATOR], [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKT DATENSCHUTZBEAUFTRAGTER], [DATUM AVV], [NAME SCHULTRÄGER]) | moto |

---

*Ende des Dokuments. Version 1.0, Stand 2026-07-07, Status: Entwurf zur internen Prüfung.*
