# Übersicht der DSGVO-Dokumentation "moto"

| | |
|---|---|
| **Dokument** | 00 Übersicht und konsolidierte Offene-Punkte-Liste |
| **Version** | 1.0 |
| **Stand** | 2026-07-07 |
| **Status** | Arbeitsdokument, wird bei jeder Änderung der Reihe fortgeschrieben |

---

## 1. Dokumentenreihe

Rollenverteilung in allen Dokumenten einheitlich: Die Schule bzw. der Schulträger ist **Verantwortlicher** (Art. 4 Nr. 7 DSGVO), moto ist **Auftragsverarbeiter** (Art. 4 Nr. 8, Art. 28 DSGVO). Alle Dokumente liegen in Version 1.0, Stand 2026-07-07, Status "Entwurf zur internen Prüfung" vor.

| Nr. | Datei | Inhalt | Funktion |
|---|---|---|---|
| 01 | `01-auftragsverarbeitungsvertrag.md` | Auftragsverarbeitungsvertrag (AVV) nach Art. 28 Abs. 3 DSGVO | Vertragliche Grundlage; Anlage 1 = Dokument 04, Anlage 2 = Dokument 02, Anlage 3 = Dokument 03, Anlage 4 = Ansprechpartner (noch zu erstellen) |
| 02 | `02-tom-dokumentation.md` | Technische und organisatorische Maßnahmen (Art. 32 DSGVO) | Anlage 2 zum AVV; Maßnahmen nach Kontrollbereichen mit Umsetzungsstand |
| 03 | `03-subprozessorenliste.md` | Subprozessorenliste (Art. 28 Abs. 2 und 4 DSGVO) | Anlage 3 zum AVV; Hetzner, Cloudflare, SMTP-Dienst, Sentry, PostHog als Subunternehmer; GitHub nachrichtlich ohne Datenzugriff |
| 04 | `04-verzeichnis-verarbeitungstaetigkeiten.md` | Verzeichnis von Verarbeitungstätigkeiten (Art. 30 Abs. 2 DSGVO) | Anlage 1 zum AVV (Beschreibung der Verarbeitung); Datenkategorien mit Tabellenbezug |
| 05 | `05-loeschkonzept.md` | Löschkonzept und Aufbewahrungsfristen | Ergänzt Anlagen 1 und 2; Löschklassen, Fristen, Austritts- und Vertragsendeprozesse |
| 06 | `06-dsfa-zuarbeit.md` | Zuarbeit zur Datenschutz-Folgenabschätzung (Art. 35 DSGVO) | Unterstützung des Verantwortlichen nach Art. 28 Abs. 3 lit. f DSGVO; Risikoanalyse R1 bis R10 |
| 07 | `07-datenpannen-meldeprozess.md` | Datenpannen-Meldeprozess (Art. 33/34 DSGVO) | Interner Prozess; Meldung an den Verantwortlichen unverzüglich, Ziel 24 h (AVV-Obergrenze 48 h) |

Hinweis: Die mehrfach referenzierte **Datenbestandsaufnahme** (tabellengenaue Aufstellung aller Datenfelder) ist ein gesondertes internes Arbeitsdokument und nicht Teil der nummerierten Reihe 01 bis 07.

## 2. Konsolidierte offene Punkte

Alle offenen Platzhalter und [PRÜFEN]-Punkte der Reihe, thematisch gruppiert und dedupliziert. Fundstellen in Klammern.

### 2.1 Firmen-Stammdaten und Personen (vor Freigabe ausfüllen)

- [ ] Rechtsform und Adresse von moto: `[RECHTSFORM UND ADRESSE]` bzw. `[RECHTSFORM, z. B. GmbH / UG (haftungsbeschränkt)]`, `[STRASSE, HAUSNUMMER]`, `[PLZ, ORT]`, `[REGISTERGERICHT, REGISTERNUMMER]` (Dok 01, 02, 03, 04, 05, 06, 07)
- [ ] Firmenkontaktdaten: `[TELEFONNUMMER]`, `[E-MAIL-ADRESSE FÜR DATENSCHUTZANFRAGEN]`, `[WEBSITE]`, `[ADRESSE]` (Dok 04)
- [ ] Vertretungsberechtigte: `[GESCHÄFTSFÜHRUNG / VERTRETUNGSBERECHTIGTE PERSON]` (Dok 01), `[NAME GESCHÄFTSFÜHRUNG]` / `[NAME GESCHÄFTSFÜHRUNG/VORSTAND]` (Dok 03, 04, 05, 07), `[NAME, FUNKTION VERTRETUNGSBERECHTIGTE PERSON MOTO]` (Dok 01)
- [ ] Datenschutzkoordination: `[NAME DATENSCHUTZKOORDINATOR/IN]` bzw. `[NAME DATENSCHUTZKOORDINATOR MOTO]` (Dok 01, 02, 03, 04, 06)
- [ ] Datenschutzbeauftragte/r moto: `[NAME DATENSCHUTZBEAUFTRAGTER]` mit `[KONTAKTDATEN DSB]` / `[KONTAKT DATENSCHUTZBEAUFTRAGTER]` / `[E-MAIL, TELEFON]` (Dok 01, 02, 03, 04, 05, 06, 07); zuvor Benennungspflicht nach Art. 37 DSGVO / § 38 BDSG abschließend bewerten (Dok 01 Vertragsparteien, Dok 04 Abschn. 2) und Bestellung dokumentieren (Dok 02 Abschn. 3.12)
- [ ] Technische Leitung / Incident-Verantwortlicher und Vertretung DSB mit Telefon und E-Mail (Dok 07 Abschn. 3.2 und 11), Sammel-Meldeadresse `[MELDEADRESSE]` (Dok 07 Abschn. 11)
- [ ] Trägerseite je Vertrag: `[NAME DES SCHULTRÄGERS / OGS-TRÄGERS]`, Rechtsform, Anschrift, Vertretung, Schulen/OGS-Standorte, DSB des Trägers (Dok 01 Vertragsparteien), `[NAME SCHULTRÄGER]` (Dok 06 Abschn. 9)

### 2.2 Vertrag und Dokumentenverwaltung

- [ ] `[DATUM HAUPTVERTRAG]` (Dok 01 Präambel) und `[DATUM AVV]` (Dok 06 Abschn. 1 und 9)
- [ ] `[GERICHTSSTAND]` festlegen; bei kommunalen Trägern Verwaltungsrechtsweg und Vergaberecht berücksichtigen (Dok 01 § 14)
- [ ] **Anlage 4 zum AVV** (weisungsberechtigte/-empfangende Personen, Kontaktstelle für Meldungen nach § 8, Erreichbarkeiten) vor Unterzeichnung erstellen (Dok 01 § 3 Abs. 4 und Anlagenverzeichnis)
- [ ] Vergütungs-, Kosten- und Haftungsregelungen (Weisungs-Mehraufwände, anlassbezogene Audits, Freistellungs-/Haftungsbegrenzungsklauseln) mit dem Hauptvertrag abstimmen (Dok 01 § 3 Abs. 7, § 10 Abs. 6, § 12 Abs. 3)
- [ ] Trägerkonstellation klären: Wer ist Verantwortlicher, ggf. Vereinbarung über gemeinsame Verantwortlichkeit nach Art. 26 DSGVO zwischen Schule und Träger (Dok 01 Präambel)
- [ ] `[VERSION/DATUM AV-VERTRAGSMUSTER]` in Dok 04 Abschn. 3 eintragen; `[ORT], [DATUM]` und Unterschrift in Dok 04
- [ ] Bereitstellungsweg für aktualisierte Subprozessorenliste festlegen (Dok 03 Abschn. 6 Nr. 5)
- [ ] Freigaben nachholen: fachliche Prüfung und Freigabe Dok 05 (Kopfzeile), Freigabe Dok 03 (Fußzeile), `[DATUM FREIGABE]` Dok 07

### 2.3 Subprozessoren und Drittlandtransfer

- [ ] **SMTP-Anbieter benennen**: Name, Sitz, Serverstandort aus der Produktivkonfiguration ermitteln, AVV/DPA abschließen und referenzieren, Drittlandbewertung nachziehen (Dok 01 § 6, Dok 02 Abschn. 3.6, Dok 03 Nr. 4 und Abschn. 4.4, Dok 04 Abschn. 5/6, Dok 06 Punkt 4)
- [ ] **Hetzner**: aktuelles ISO-27001-Zertifikat und AVV-Anlage beschaffen, beilegen und in Dokument 03 referenzieren; vertragliche Beschränkung auf deutsche Standorte dokumentieren; Datum der letzten Sichtung inkl. Subunternehmeranhang eintragen; Nachweis Datenträgervernichtung (Dok 01 § 10 Abs. 4, Dok 02 Abschn. 3.1, Dok 03 Abschn. 4.1, Dok 05 Abschn. 11 Nr. 5, Dok 06 Punkt 10)
- [ ] **Cloudflare**: konkrete Vertragspartei (Inc. oder Germany GmbH) eintragen, DPF-Zertifizierungsstatus mit Prüfdatum dokumentieren, Nutzung der EU-Datenlokalisierung (Data Localization Suite) verifizieren, Betroffeneninformation im Anmeldeformular bei aktiviertem Turnstile (Dok 01 § 6, Dok 02, Dok 03 Abschn. 4.2, Dok 04, Dok 06 Punkt 5)
- [ ] **Sentry**: produktiven Einsatz bestätigen (sonst Eintrag streichen), tatsächliche Projektregion (EU) im Konto verifizieren, DPA-Fassung sichten und ablegen (Dok 01 § 6, Dok 02, Dok 03 Abschn. 4.5, Dok 04, Dok 06 Punkt 6)
- [ ] **PostHog**: produktiven Einsatz und konfigurierten Host (EU-Cloud) bestätigen, Event-Inhalte auf Kennungen/Namen von Kindern auditieren, DPA-Fassung sichten (Dok 01 § 6, Dok 02, Dok 03 Abschn. 4.6, Dok 04, Dok 06 Punkt 7)
- [ ] **GitHub/Microsoft**: DPF-Zertifizierungsstatus mit Prüfdatum dokumentieren, SCC-Rückfallmechanismus im GitHub-DPA verifizieren (Dok 02, Dok 03 Abschn. 4.3 und 5, Dok 04, Dok 06 Punkt 8)
- [ ] Adress-Platzhalter in Dok 03 füllen: `[ADRESSE CLOUDFLARE GERMANY]`, `[ADRESSE POSTHOG, USA]`; Anlagenverzeichnis Dok 03: alle `[DATUM DER SICHTUNG]` eintragen, DPA-Links verifizieren
- [ ] DPF-Rechtsentwicklung beobachten (Trump v. Slaughter, Rechtssache C-703/25 P); jährliche Prüfung der DPF-Zertifizierungen (Dok 03 Abschn. 5)
- [ ] Meldefristen (Kettenmeldepflicht bei Datenpannen) in allen Subprozessoren-DPAs verifizieren und in Dok 03 vermerken (Dok 07 Abschn. 3.1)
- [ ] Schriftarten-Auslieferung: Ausschluss eines Laufzeit-Rückfalls auf externe Google-Server bestätigen (Dok 06 Punkt 9)

### 2.4 Löschfristen und Speicherbegrenzung

- [ ] Aufbewahrungsfristen für Audit-Tabellen ohne Löschfrist festlegen und technisch umsetzen: `audit.auth_events`, `audit.data_access_log`, `audit.data_deletions`, `audit.data_imports`, `audit.guardian_changes`, `audit.enrollment_offering_adjustment`, `platform.operator_audit_log` (Dok 02 Abschn. 3.5, Dok 04 Abschn. 4.7c, Dok 05 Abschn. 14 Nr. 1, Dok 06 R9)
- [ ] Löschregel für Krank-/Entschuldigt-Tagesmeldungen `active.student_status_days` (Gesundheitsdaten) festlegen (Dok 05 Abschn. 14 Nr. 2)
- [ ] Löschregel für Eltern-Nachrichten `users.parent_messages` festlegen (Dok 05 Abschn. 14 Nr. 3, Dok 06 R9)
- [ ] Kurze rollierende Löschfrist für unregistrierte NFC-Scans `audit.unregistered_tag_scans` festlegen (Dok 05 Abschn. 14 Nr. 4, Dok 06 R5)
- [ ] Löschregeln für genehmigte Anmeldungen (Ursprungsdatensätze) und `users.student_data_change_requests` festlegen (Dok 05 Abschn. 14 Nr. 5)
- [ ] VO-DV-I-Einordnung der OGS-Anwesenheitsdaten verbindlich mit Schulen/Trägern, ggf. Bezirksregierung, abstimmen (Dok 01 § 9 Abs. 2, Dok 05 Abschn. 5 und 14 Nr. 6)
- [ ] Regelweisung zur Stammdatenlöschung nach Schulaustritt je Schulträger im AVV fixieren (Dok 05 Abschn. 14 Nr. 7)
- [ ] Kalenderbasierte Höchstvorhaltezeit für Backups festlegen (`[MAXIMALE BACKUP-VORHALTEZEIT IN TAGEN]`) und zeitgesteuerte, deploymentunabhängige Sicherung mit `[BACKUP-FREQUENZ UND -AUFBEWAHRUNGSDAUER]` einführen (Dok 02 Abschn. 3.7, Dok 05 Abschn. 14 Nr. 8)
- [ ] Umfang und Frist der Nachweisaufbewahrung nach Vertragsende festlegen (Dok 05 Abschn. 14 Nr. 9)
- [ ] Nachweisfrist für Einwilligungsnachweise nach Betreuungsende entscheiden (Dok 05 Abschn. 14 Nr. 10)
- [ ] Entscheiden, ob das dauerhafte Abschalten der automatischen Bereinigung durch Schulen technisch unterbunden wird (Dok 05 Abschn. 14 Nr. 11)
- [ ] Kiosk-Persistenz (keine lokale Speicherung) bei wesentlichen Software-Änderungen erneut verifizieren (Dok 01 § 9 Abs. 6, Dok 05 Abschn. 14 Nr. 12)

### 2.5 Rechtliche Klärungen (überwiegend Verantwortlicher)

- [ ] Erlaubnistatbestand nach Art. 9 Abs. 2 DSGVO für Gesundheitsangaben (Kinderprofil-Freitext, Krankmeldungen, Anmeldeformular) dokumentieren, ggf. Einwilligung einholen; Eingaberichtlinie (Dienstanweisung) für Freitextfelder erstellen (Dok 01 § 2 Abs. 5, Dok 04 Abschn. 3, Dok 06 R8 und Punkte 2, 12)
- [ ] Abgleich der Standard-Datenfelder mit dem Datenkatalog der VO-DV I; Rechtsgrundlage für Foto, Gesundheitsangaben, Feedback klären (Dok 04 Abschn. 8, Dok 06 Punkt 2)
- [ ] DSFA-Erforderlichkeit klären: aktuelle LDI-NRW-Positivliste beschaffen und einschlägige Positionen dokumentieren; Durchführung obliegt dem Verantwortlichen (Dok 01 § 8 Abs. 5, Dok 06 Abschn. 2 und Punkt 1)
- [ ] LDI-NRW-Kontaktdaten und Formular-Link vor jeder Verwendung auf Aktualität prüfen (Dok 07 Abschn. 12)

### 2.6 TOM und Sicherheit (moto)

- [ ] Verschlüsselung ruhender Nutzdaten: Ist-Zustand feststellen, nachweisen oder als Maßnahme einführen (Dok 02 Abschn. 3.9)
- [ ] Turnus und Verfahren der internen Wirksamkeitsprüfung der TOM festlegen und in Dokument 02 aufnehmen (Dok 01 § 5 Abs. 4, Dok 02 Abschn. 3.11)
- [ ] Externe Sicherheitsüberprüfungen / Penetrationstests planen, Turnus festlegen, Nachweise führen (Dok 02 Abschn. 3.11, Dok 06 R3 und Punkt 11)
- [ ] Turnusmäßige Wiederherstellungstests der Datenbanksicherungen dokumentieren (Dok 02 Abschn. 3.7)
- [ ] `[ZERTIFIZIERUNGEN FALLS VORHANDEN]` ergänzen oder Feld streichen (Dok 02 Abschn. 3.11)

### 2.7 Organisatorische Nachweise ([ORGANISATORISCH ZU BESTÄTIGEN], Dok 02)

- [ ] Regelungen zur Absicherung der Endgeräte der Mitarbeitenden (Festplattenverschlüsselung, Bildschirmsperre) schriftlich fixieren (Abschn. 3.1)
- [ ] Dokumentiertes Weisungs- und Kommunikationsverfahren mit den Verantwortlichen (Abschn. 3.6)
- [ ] Schriftliche Vertraulichkeitsverpflichtungen aller Mitarbeitenden mit Datenzugriff (Abschn. 3.12, Abschn. 5)
- [ ] Schulungen: `[SCHULUNGSINTERVALL MITARBEITENDE]` festlegen und Nachweise führen (Abschn. 3.12)
- [ ] Dokumentiertes Meldeverfahren bei Datenschutzverletzungen bestätigen; Verweis auf Dokument 07 herstellen (Abschn. 3.12)
- [ ] Organisatorisches Verfahren zur Bearbeitung von Betroffenenanfragen (Abschn. 3.12)
- [ ] Verfahren zur Vergabe, Überprüfung und zum Entzug administrativer Zugänge bei Ein-/Austritt (Abschn. 3.12)
- [ ] Überprüfungsturnus des TOM-Dokuments verbindlich festlegen und Verantwortlichkeit benennen (Abschn. 3.11)

### 2.8 Datenpannen-Prozess (Dok 07)

- [ ] `[SPEICHERORT VORFALLREGISTER]` festlegen (verschlüsselte Ablage außerhalb der Produktivinfrastruktur)
- [ ] `[AUFBEWAHRUNGSDAUER]` der Vorfalldokumentation festlegen und mit Verjährungsfristen und AVV abstimmen (Abschn. 9)
- [ ] `[REGELUNG RUFBEREITSCHAFT / ERREICHBARKEIT AUSSERHALB DER GESCHÄFTSZEITEN]` festlegen (Abschn. 11)
- [ ] Kontakte laut Kundenstammdaten je Verantwortlichem hinterlegen (`[KONTAKT LAUT KUNDENSTAMMDATEN]`, Abschn. 7.2)
- [ ] Jährliche Prozessübung durchführen und `[DATUM LETZTE ÜBUNG]` pflegen (Abschn. 10)
- [ ] Fristen und Meldewege bei jeder AVV-Änderung mit der Meldeklausel (Dok 01 § 8) konsistent halten (Abschn. 1 und 7.1)

## 3. Pflegehinweis

Dieses Dokument wird bei jeder inhaltlichen Änderung eines Dokuments der Reihe sowie bei Erledigung offener Punkte aktualisiert. Erledigte Punkte werden abgehakt und mit Datum versehen, nicht gelöscht.
