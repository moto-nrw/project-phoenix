# Prozess zur Meldung von Datenschutzverletzungen (Art. 33/34 DSGVO)

| | |
|---|---|
| **Dokument** | 07 Datenpannen-Meldeprozess |
| **Verantwortlich** | [NAME DATENSCHUTZBEAUFTRAGTER], Datenschutzkoordination moto |
| **Version** | 1.0 |
| **Stand** | 2026-07-07 |
| **Status** | Entwurf zur internen Prüfung |
| **Geltungsbereich** | moto [RECHTSFORM UND ADRESSE], Produkt "moto" (internes Projekt "Project Phoenix") |
| **Freigabe** | [NAME GESCHÄFTSFÜHRUNG], Datum: [DATUM FREIGABE] |

---

## 1. Zweck und Geltungsbereich

Dieses Dokument regelt den internen Prozess von moto für den Umgang mit Verletzungen des Schutzes personenbezogener Daten (im Folgenden: Datenschutzverletzung) im Zusammenhang mit dem Betrieb des NFC/RFID-Anwesenheits- und Raumverwaltungssystems "moto" für den Offenen Ganztag (OGS) an Grundschulen in Nordrhein-Westfalen.

moto ist gegenüber den Schulen bzw. Schulträgern **Auftragsverarbeiter** im Sinne von Art. 28 DSGVO. Verantwortlicher im Sinne von Art. 4 Nr. 7 DSGVO ist die jeweilige Schule bzw. der jeweilige Schulträger. Daraus ergibt sich die zentrale Weichenstellung dieses Prozesses:

- moto meldet Datenschutzverletzungen **unverzüglich an den Verantwortlichen** (Art. 33 Abs. 2 DSGVO).
- Die Meldung an die Aufsichtsbehörde (Art. 33 Abs. 1 DSGVO) und die Benachrichtigung betroffener Personen (Art. 34 DSGVO) sind **Pflichten des Verantwortlichen**. moto nimmt **keine eigene Meldung an die Aufsichtsbehörde vor** und benachrichtigt Betroffene nicht in eigenem Namen. moto unterstützt den Verantwortlichen bei diesen Pflichten nach Maßgabe des Auftragsverarbeitungsvertrags (Art. 28 Abs. 3 lit. f DSGVO).

Der Prozess gilt für alle Mitarbeitenden von moto, für alle produktiven und stagingnahen Systeme (Backend, Frontend-Portale, Kiosk-Geräte, Datenbank, Monitoring, Deployment-Infrastruktur) sowie für Vorfälle bei Unterauftragsverarbeitern, soweit sie Daten aus dem Auftrag betreffen.

Dieser Prozess konkretisiert die Meldepflichten aus dem Auftragsverarbeitungsvertrag zwischen moto und dem jeweiligen Verantwortlichen. Bei Widersprüchen gehen die vertraglichen Regelungen des jeweiligen AVV vor. [PRÜFEN: Fristen und Meldewege dieses Dokuments mit der Meldeklausel im AVV-Muster abgleichen und beide konsistent halten.]

## 2. Begriffsbestimmung

Eine **Verletzung des Schutzes personenbezogener Daten** ist nach Art. 4 Nr. 12 DSGVO eine Verletzung der Sicherheit, die, ob unbeabsichtigt oder unrechtmäßig, zur Vernichtung, zum Verlust, zur Veränderung oder zur unbefugten Offenlegung von beziehungsweise zum unbefugten Zugang zu personenbezogenen Daten führt, die übermittelt, gespeichert oder auf sonstige Weise verarbeitet wurden.

Erfasst sind alle drei Schutzziele:

1. **Vertraulichkeit**: unbefugte Offenlegung oder unbefugter Zugang (z. B. Datenabfluss, Fehlversand, Einsicht durch Unbefugte)
2. **Integrität**: unbefugte oder unbeabsichtigte Veränderung von Daten
3. **Verfügbarkeit**: unbeabsichtigter oder unrechtmäßiger Verlust des Zugangs zu Daten oder Vernichtung von Daten (auch vorübergehend, z. B. durch Ransomware oder Datenverlust ohne verwendbares Backup)

### 2.1 Beispiele aus dem moto-Systemkontext

Die folgenden Konstellationen sind als Datenschutzverletzung oder als meldeprozessauslösender Verdachtsfall zu behandeln:

- Kompromittierung oder Abfluss eines Device-API-Keys eines Kiosk-Geräts (PyrePortal auf Raspberry Pi), insbesondere in Verbindung mit einer bekannt gewordenen Personal-PIN
- Diebstahl oder Verlust eines Kiosk-Geräts aus einer Schule
- Fehler in der mandantenbezogenen Zugriffstrennung (PostgreSQL Row Level Security, Tenant-Middleware), durch den Daten einer Schule für eine andere Schule oder für Unbefugte sichtbar werden
- Ausnutzung einer Schwachstelle im Authentifizierungspfad (JWT, MFA, Passkeys, Passwort-Reset), die unbefugten Zugriff auf Konten von Personal, Eltern oder Operator ermöglicht
- Kompromittierung von Zugangsdaten oder Schlüsselmaterial (JWT-Secret, SOPS/age-Schlüssel, Datenbank-Zugangsdaten, SSH-Deploy-Zugänge, GitHub-Zugänge)
- Fehlkonfiguration des Reverse Proxy (Caddy) oder von Cloudflare, durch die interne Endpunkte oder Daten öffentlich erreichbar werden
- Sicherheitsvorfall auf dem Hosting-Server (Hetzner, Nürnberg), z. B. Einbruch, Ransomware, unbefugter Zugriff auf Datenbank, Backups oder das dort mitbetriebene Monitoring (Grafana/Loki)
- Versand personenbezogener Daten an falsche Empfänger (z. B. E-Mail an falsche Adresse mit Kindsdaten, fehlgeleitete Einladungs- oder Reset-Links)
- Unbefugte Einsichtnahme in besonders geschützte Datenbestände, insbesondere Gesundheitsangaben zu Kindern (Freitextfeld Gesundheitsinformationen, strukturierte Krankmeldungen) und Krankmeldungen des Personals
- Sicherheitsvorfall bei einem Unterauftragsverarbeiter (z. B. Hosting-Anbieter, Cloudflare, GitHub/GHCR, E-Mail-Versanddienst), der Daten aus dem Auftrag betrifft
- Verlust oder Nichtwiederherstellbarkeit von Produktivdaten (fehlgeschlagene Migration ohne verwendbares Backup, defekte Datensicherung)

Nicht jeder Sicherheitsvorfall ist eine Datenschutzverletzung (z. B. ein abgewehrter Angriffsversuch ohne Zugriff auf personenbezogene Daten). Die Abgrenzung trifft der Datenschutzbeauftragte gemeinsam mit der technischen Leitung im Rahmen der Bewertung nach Abschnitt 6. Im Zweifel ist der Vorfall als Datenschutzverletzung zu behandeln.

### 2.2 Betroffene Datenkategorien im System

Für die Bewertung und die Meldung ist relevant, welche Datenkategorien das System verarbeitet. Die vollständige Aufstellung enthält das Verzeichnis von Verarbeitungstätigkeiten (Dokument 04) auf Grundlage der Datenbestandsaufnahme (gesondertes internes Dokument). Zusammengefasst:

| Betroffenengruppe | Wesentliche Datenkategorien |
|---|---|
| Schüler:innen | Stammdaten (Name, Geburtsdatum, Adresse, Klasse), Anwesenheits- und Bewegungsdaten (Check-in/Check-out, Raumaufenthalt), Abholregelungen, Foto, Einwilligungen, RFID-Tag-Kennung, Betreuernotizen, **Gesundheitsangaben (Freitext, Krankmeldungen; Art. 9 DSGVO)** |
| Eltern / Erziehungsberechtigte | Name, E-Mail, Adresse, Telefonnummern, Beziehung zum Kind, Abholberechtigung, Notfallkontakt-Kennzeichnung, Portal-Account, Nachrichten mit der Betreuung, Anmeldedaten (Enrollment) |
| Personal | Stammdaten, Beschäftigungsart, Zeiterfassung (Kommen/Gehen, Pausen), **Abwesenheiten inkl. Krankmeldungen (Art. 9 DSGVO)**, RFID-Tag-Kennung, Account- und PIN-Daten (gehasht) |
| Alle Kontoinhaber | E-Mail, Passwort-/PIN-Hashes, Login-Historie inkl. IP-Adresse und User-Agent, MFA- und Passkey-Daten, Sessions/Tokens |
| Nicht identifizierte Dritte | Unregistrierte RFID-Tag-Scans (Tag-UID, Gerät, Zeitpunkt) |

Vorfälle, die Gesundheitsangaben von Kindern, Krankmeldungen, Abholberechtigungen oder Bewegungsdaten von Kindern betreffen, sind stets als potenziell hohes Risiko einzustufen und mit höchster Priorität zu behandeln. Die Betroffenen sind überwiegend Kinder; dies ist bei jeder Bewertung und in jeder Meldung ausdrücklich zu berücksichtigen.

## 3. Rollen und Verantwortlichkeiten

### 3.1 Rollenverteilung nach DSGVO

| Rolle | Wer | Pflichten im Meldeprozess |
|---|---|---|
| Verantwortlicher | Schule bzw. Schulträger | Risikobewertung; Entscheidung über Meldung an die LDI NRW binnen 72 Stunden ab eigener Kenntnis (Art. 33 Abs. 1 DSGVO); Entscheidung über Benachrichtigung der Betroffenen (Art. 34 DSGVO); Dokumentation nach Art. 33 Abs. 5 DSGVO |
| Auftragsverarbeiter | moto | Erkennung, Eindämmung, unverzügliche Meldung an den Verantwortlichen (Art. 33 Abs. 2 DSGVO), technische Aufklärung, Unterstützung des Verantwortlichen (Art. 28 Abs. 3 lit. f DSGVO), eigene Dokumentation |
| Unterauftragsverarbeiter | Hosting-Anbieter, Cloudflare, GitHub/GHCR, E-Mail-Versanddienst, weitere gemäß Subprozessorenliste (Dokument 03) | Meldung an moto gemäß Unterauftragsverarbeitungsvertrag (Kettenmeldepflicht) [PRÜFEN: Meldefristen in allen Unterauftragsverarbeitungsverträgen bzw. DPAs verifizieren und in der Subprozessorenliste vermerken] |

moto trifft im Rahmen von Art. 33 Abs. 2 DSGVO **keine eigene Risikobewertung als Meldevoraussetzung**. Die Meldepflicht des Auftragsverarbeiters an den Verantwortlichen ist voraussetzungslos: Jede bekannt gewordene Datenschutzverletzung, die Daten aus dem Auftrag betrifft, wird gemeldet. Die Bewertung, ob ein Risiko für die Rechte und Freiheiten der Betroffenen besteht und ob eine Meldung an die LDI NRW erfolgt, obliegt allein dem Verantwortlichen. Die interne Klassifizierung nach Abschnitt 6 dient ausschließlich der Priorisierung der Eindämmung und der Qualität der Meldung, nicht der Filterung meldepflichtiger Vorfälle.

### 3.2 Interne Zuständigkeiten bei moto

| Funktion | Person | Aufgaben |
|---|---|---|
| Datenschutzbeauftragter / Datenschutzkoordination | [NAME DATENSCHUTZBEAUFTRAGTER] | Prozessverantwortung, Bewertung, Freigabe und Versand der Meldung an den Verantwortlichen, Dokumentation, Ansprechpartner nach Art. 33 Abs. 3 lit. b DSGVO |
| Technische Leitung / Incident-Verantwortlicher | [NAME TECHNISCHE LEITUNG] | Technische Aufklärung, Eindämmung, Beweissicherung, Wiederherstellung |
| Geschäftsführung | [NAME GESCHÄFTSFÜHRUNG] | Gesamtverantwortung, Freigabe außerordentlicher Maßnahmen (z. B. Abschaltung des Produktivsystems), Kommunikation auf Leitungsebene |
| Alle Mitarbeitenden | alle | Unverzügliche interne Meldung jedes Verdachtsfalls an die Kontaktkette (Abschnitt 11), keine eigenmächtige Außenkommunikation |

Vertretungsregelung: Bei Abwesenheit einer Funktion greift die in Abschnitt 11 hinterlegte Vertretung. Kein Schritt dieses Prozesses darf an der Abwesenheit einer einzelnen Person scheitern.

## 4. Erkennung von Datenschutzverletzungen

Meldeprozessauslösende Erkenntnisse können aus folgenden Quellen stammen. Jede Quelle mündet in denselben Prozess (Abschnitt 5 ff.):

1. **Eigenes Monitoring und Alerting**: selbst gehostetes Grafana/Loki auf der Hosting-Infrastruktur, inklusive Alarmregeln (u. a. Verfügbarkeits-/Healthcheck-Alarme, Fehlerraten, auffällige Log-Muster). Dies ist der primäre technische Erkennungsweg.
2. **Audit- und Protokolldaten des Systems**: Login-Ereignisprotokoll inkl. Fehlversuchen, Datenzugriffsprotokoll für sensible Lesezugriffe, Änderungsprotokolle, Protokoll unregistrierter NFC-Scans. Auffälligkeiten in diesen Protokollen (z. B. gehäufte Fehlversuche, untypische Zugriffs- oder Exportmuster) sind als Verdachtsfall zu behandeln.
3. **Feststellung durch Mitarbeitende**: im Rahmen von Entwicklung, Betrieb, Code-Review oder Deployment (z. B. entdeckte Sicherheitslücke, versehentlich veröffentlichtes Secret, fehlgeschlagene Migration mit Datenverlust).
4. **Meldung durch den Verantwortlichen oder dessen Nutzer**: Support-Anfragen von Schulen, Personal oder Eltern (z. B. "ich sehe Daten eines fremden Kindes").
5. **Externe Hinweise**: Sicherheitsforscher, Responsible-Disclosure-Meldungen, Hinweise Dritter, Presseanfragen.
6. **Meldung eines Unterauftragsverarbeiters**: Sicherheits- oder Datenschutzvorfall bei Hosting, Cloudflare, GitHub/GHCR, E-Mail-Versanddienst oder weiteren Dienstleistern gemäß Subprozessorenliste.

Für die Fristberechnung gilt: **Kenntnis** liegt vor, sobald eine für den Prozess zuständige Person (Abschnitt 3.2) mit hinreichender Sicherheit von einer Datenschutzverletzung weiß. Bei einem Verdachtsfall beginnt unverzüglich die Verifikation; die Verifikationsphase darf nicht dazu genutzt werden, die Meldung hinauszuzögern. Zeitpunkt der Kenntniserlangung ist im Vorfallprotokoll minutengenau festzuhalten.

## 5. Sofortmaßnahmen (Eindämmung und Beweissicherung)

Unmittelbar nach Feststellung eines Vorfalls oder eines konkreten Verdachts ergreift die technische Leitung, soweit einschlägig und ohne die Aufklärung zu gefährden, folgende Maßnahmen:

1. **Eindämmung**
   - Isolierung betroffener Systeme oder Container; im Extremfall Abschaltung des betroffenen Dienstes (Freigabe durch Geschäftsführung, wenn der Betrieb aller Schulen betroffen ist)
   - Sperrung oder Deaktivierung kompromittierter Konten (Personal-, Eltern-, Operator-Konten), Invalidierung aktiver Sessions und Refresh-Tokens
   - Rotation kompromittierter oder potenziell kompromittierter Zugangsdaten und Schlüssel: Device-API-Keys betroffener Kiosk-Geräte, Staff-PINs, JWT-Secret, Datenbank-Zugangsdaten, SOPS/age-Schlüssel, SSH-Deploy-Schlüssel, GitHub-Tokens
   - Deaktivierung betroffener Kiosk-Geräte (bei Geräteverlust oder Key-Kompromittierung)
   - Bei Fehlkonfiguration von Proxy/DNS: sofortige Korrektur der Konfiguration, danach Prüfung, ob und wie lange Daten erreichbar waren
2. **Beweissicherung**
   - Sicherung relevanter Log-Daten (Anwendungslogs, Zugriffs- und Auditprotokolle, Reverse-Proxy-Logs, Monitoring-Daten) auf ein vom Produktivsystem getrenntes Medium, bevor sie durch Rotation oder Retention überschrieben werden
   - Sicherung eines Datenbankstands (Dump) zum Vorfallzeitpunkt, soweit zur Aufklärung erforderlich
   - Festhalten aller Zeitpunkte, Beobachtungen und durchgeführten Maßnahmen im internen Vorfallprotokoll (Abschnitt 9), beginnend mit der ersten Feststellung
3. **Keine vorschnelle Spurenvernichtung**: Systeme werden nicht neu aufgesetzt oder bereinigt, bevor die für die Aufklärung notwendigen Daten gesichert sind, es sei denn, die Eindämmung erfordert es zwingend.

Die Sofortmaßnahmen laufen parallel zur Meldung nach Abschnitt 7. Die Meldung wartet nicht auf den Abschluss der Eindämmung.

## 6. Interne Bewertung und Klassifizierung

Der Datenschutzbeauftragte und die technische Leitung bewerten den Vorfall unverzüglich anhand folgender Fragen:

1. **Liegt eine Datenschutzverletzung im Sinne von Art. 4 Nr. 12 DSGVO vor** oder ein reiner Sicherheitsvorfall ohne Bezug zu personenbezogenen Daten? Im Zweifel: Datenschutzverletzung.
2. **Welche Mandanten (Schulen) sind betroffen?** Zu unterscheiden sind:
   - **mandantenspezifische Vorfälle** (eine Schule betroffen, z. B. Verlust eines Kiosk-Geräts): Meldung an den betroffenen Verantwortlichen
   - **plattformweite Vorfälle** (alle oder mehrere Schulen betroffen, z. B. Kompromittierung des Hosting-Servers, RLS-Fehler): Meldung an alle betroffenen Verantwortlichen, koordiniert und mit identischem Informationsstand
3. **Welche Datenkategorien und Betroffenengruppen sind betroffen?** Besonders zu kennzeichnen: Gesundheitsangaben (Art. 9 DSGVO), Daten von Kindern, Abholberechtigungen und Notfallkontakte, Bewegungs-/Anwesenheitsdaten, Zugangsdaten.
4. **Ungefähre Zahl der betroffenen Personen und Datensätze** (Größenordnung genügt zunächst; Präzisierung per Nachmeldung).
5. **Zeitraum**: Beginn, Entdeckung, Ende bzw. andauernder Zustand.
6. **Schweregrad zur internen Priorisierung** (nicht als Meldefilter):
   - **Kritisch**: Art.-9-Daten oder Kindsdaten in größerem Umfang offengelegt, plattformweiter Vorfall, andauernder unbefugter Zugriff
   - **Hoch**: begrenzte Offenlegung personenbezogener Daten, kompromittierte Einzelkonten, Geräteverlust
   - **Mittel**: Integritäts- oder Verfügbarkeitsverletzung ohne Anhaltspunkt für Offenlegung
   - Bei "Kritisch" informiert der Datenschutzbeauftragte unverzüglich zusätzlich die Geschäftsführung.

Ergebnis der Bewertung ist die Feststellung, **an welche Verantwortlichen** mit **welchem Informationsstand** gemeldet wird. Die Bewertung ersetzt nicht die Risikobewertung des Verantwortlichen und wird in der Meldung ausdrücklich als Sachverhaltsdarstellung, nicht als abschließende Risikoeinschätzung gekennzeichnet.

## 7. Meldung an den Verantwortlichen (Art. 33 Abs. 2 DSGVO)

### 7.1 Frist

Die Meldung an den Verantwortlichen erfolgt **unverzüglich nach Kenntniserlangung**, als interne Zielvorgabe **spätestens innerhalb von 24 Stunden**. Das AVV-Muster (Dokument 01, § 8 Abs. 2) sieht als vertragliche Obergrenze eine Meldung spätestens innerhalb von 48 Stunden vor; die interne Zielvorgabe unterschreitet diese Frist bewusst. [PRÜFEN: bei individuell abweichenden Fristen in einzelnen AVV ist die jeweils vereinbarte Frist maßgeblich.]

Hintergrund: Der Verantwortliche muss seine eigene Meldung an die Aufsichtsbehörde binnen 72 Stunden ab **seiner** Kenntnis abgeben (Art. 33 Abs. 1 DSGVO). Da moto in der Regel Erstentdecker ist, muss die Meldung von moto so früh und so vollständig erfolgen, dass dem Verantwortlichen ausreichend Zeit für Bewertung und Behördenmeldung verbleibt.

Die gesetzliche 72-Stunden-Frist gilt **nicht** für die Meldung von moto an den Verantwortlichen; sie ist keine Rechtfertigung für ein Zuwarten.

### 7.2 Empfänger

Die Meldung geht an die im jeweiligen AVV bzw. in den hinterlegten Kundenstammdaten benannten Stellen des Verantwortlichen, in der Regel:

- Schulleitung der betroffenen Schule: [KONTAKT LAUT KUNDENSTAMMDATEN]
- Datenschutzansprechpartner des Schulträgers bzw. behördlicher Datenschutzbeauftragter: [KONTAKT LAUT KUNDENSTAMMDATEN]
- Bei kommunalem IT-Dienstleister als Betriebspartner des Trägers: zusätzlich dessen benannte Meldestelle [KONTAKT LAUT KUNDENSTAMMDATEN]

Meldeweg: E-Mail an die hinterlegte Meldeadresse mit dem ausgefüllten Formular aus Anhang A, bei kritischen Vorfällen zusätzlich telefonische Vorabinformation. Wird der Versand über die möglicherweise kompromittierte eigene Infrastruktur als unsicher bewertet, erfolgt die Meldung über einen unabhängigen Kanal (Telefon plus alternatives E-Mail-Konto). Der Zugang der Meldung ist zu dokumentieren (Lesebestätigung, Telefonvermerk).

### 7.3 Pflichtinhalte der Meldung

Die Meldung enthält mindestens die folgenden Angaben. Sie orientiert sich an Art. 33 Abs. 3 lit. a bis d DSGVO, damit der Verantwortliche die Inhalte ohne Umformung für seine eigene Meldung an die LDI NRW übernehmen kann:

1. **Art der Verletzung** (Sachverhaltsdarstellung): Was ist wann passiert, welches Schutzziel ist verletzt (Vertraulichkeit, Integrität, Verfügbarkeit), welche Systeme oder Komponenten sind betroffen, ist der Vorfall beendet oder andauernd.
2. **Kategorien und ungefähre Zahl der betroffenen Personen** (Schüler:innen, Erziehungsberechtigte, Personal) **sowie Kategorien und ungefähre Zahl der betroffenen Datensätze**, mit ausdrücklicher Kennzeichnung besonderer Kategorien (Gesundheitsangaben) und des Umstands, dass Kinder betroffen sind.
3. **Name und Kontaktdaten der Anlaufstelle bei moto**: [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKTDATEN DSB].
4. **Beschreibung der wahrscheinlichen Folgen** der Verletzung, soweit aus Sicht von moto als Auftragsverarbeiter erkennbar (Sachverhaltsebene; die rechtliche Risikobewertung obliegt dem Verantwortlichen).
5. **Ergriffene und vorgeschlagene Maßnahmen** zur Behebung der Verletzung und zur Abmilderung möglicher nachteiliger Auswirkungen (Eindämmung, Rotation von Zugangsdaten, Korrekturen, geplante Schritte).
6. Zeitpunkt der Kenntniserlangung bei moto sowie Hinweis, ob und welche Angaben noch ausstehen.

### 7.4 Gestaffelte Meldung

Sind bei Fristablauf noch nicht alle Informationen verfügbar, wird **nicht gewartet**: Die Erstmeldung erfolgt mit dem vorhandenen Kenntnisstand und kennzeichnet offene Punkte ausdrücklich. Fehlende Angaben werden ohne unangemessene Verzögerung nachgemeldet (Rechtsgedanke des Art. 33 Abs. 4 DSGVO). Jede Nachmeldung verweist auf die Vorfallnummer der Erstmeldung.

## 8. Unterstützung des Verantwortlichen (Art. 28 Abs. 3 lit. f DSGVO)

moto unterstützt den Verantwortlichen nach Meldung des Vorfalls insbesondere durch:

1. **Zulieferung für die Behördenmeldung**: Die Angaben aus Anhang A sind so strukturiert, dass der Verantwortliche sie in das Meldeformular der LDI NRW übertragen kann. moto beantwortet Rückfragen des Verantwortlichen und der Aufsichtsbehörde zum technischen Sachverhalt unverzüglich; die Kommunikation mit der Aufsichtsbehörde führt der Verantwortliche.
2. **Unterstützung bei der Benachrichtigung Betroffener (Art. 34 DSGVO)**: Auf Anforderung des Verantwortlichen stellt moto Sachverhaltsbausteine in klarer, einfacher Sprache bereit und unterstützt bei der technischen Umsetzung der Benachrichtigung, etwa per E-Mail-Versand über die Systeminfrastruktur oder Hinweis im Elternportal, soweit die Infrastruktur nicht selbst kompromittiert ist. Die Entscheidung über das Ob, den Inhalt und den Zeitpunkt der Benachrichtigung trifft ausschließlich der Verantwortliche.
3. **Hinweis auf Ausnahmen nach Art. 34 Abs. 3 DSGVO**: moto teilt dem Verantwortlichen mit, welche technischen Schutzmaßnahmen auf die betroffenen Daten angewendet waren (z. B. Verschlüsselung, gehashte Passwörter/PINs), damit der Verantwortliche prüfen kann, ob eine Benachrichtigungspflicht entfällt. Die rechtliche Bewertung nimmt der Verantwortliche vor.

## 9. Dokumentation

1. moto führt ein **internes Vorfallregister**, in dem jede Datenschutzverletzung und jeder ernsthafte Verdachtsfall dokumentiert wird: Vorfallnummer, Zeitpunkte (Feststellung, Kenntnis, Meldung, Abschluss), Sachverhalt, betroffene Mandanten, Datenkategorien und Betroffenenzahlen, ergriffene Maßnahmen, Kommunikationsverlauf mit dem Verantwortlichen und ggf. Unterauftragsverarbeitern, Ergebnis der Nachbereitung.
2. Das Vorfallregister wird **getrennt vom Produktivsystem** geführt (Speicherort: [SPEICHERORT VORFALLREGISTER, z. B. verschlüsselte Ablage außerhalb der Produktivinfrastruktur]), damit die Dokumentation auch bei kompromittierter Infrastruktur verfügbar und beweissicher bleibt.
3. Die Dokumentationspflicht nach Art. 33 Abs. 5 DSGVO trifft den Verantwortlichen. Die interne Dokumentation von moto dient der eigenen Rechenschafts- und Nachweispflicht als Auftragsverarbeiter (Art. 28 Abs. 3 lit. h DSGVO) und wird dem Verantwortlichen auf Anforderung für dessen Dokumentation zur Verfügung gestellt.
4. Auch Vorfälle, die sich nach Prüfung **nicht** als Datenschutzverletzung erweisen, werden mit Begründung im Register festgehalten.
5. Aufbewahrungsdauer der Vorfalldokumentation: [AUFBEWAHRUNGSDAUER, z. B. 5 Jahre ab Abschluss des Vorfalls] [PRÜFEN: Aufbewahrungsdauer festlegen und mit den Verjährungsfristen möglicher Haftungsansprüche sowie den Vorgaben des AVV abstimmen.]

## 10. Nachbereitung

Nach Abschluss jedes Vorfalls, bei kritischen Vorfällen spätestens vier Wochen nach Abschluss:

1. **Ursachenanalyse** (Root Cause): technische und organisatorische Ursache, nicht nur Symptom.
2. **Ableitung von Maßnahmen**: Anpassung technischer und organisatorischer Maßnahmen (TOM-Dokument aktualisieren), Codekorrekturen, Härtung, ggf. Anpassung von Monitoring-Alarmen, damit vergleichbare Vorfälle früher erkannt werden.
3. **Prozessreview**: Prüfung, ob Fristen, Kontaktkette und Formulare funktioniert haben; Anpassung dieses Dokuments bei Bedarf.
4. **Regelprüfung**: Dieser Prozess wird mindestens **jährlich** überprüft und mindestens einmal jährlich in einer Übung (Planspiel anhand eines fiktiven Szenarios, z. B. RLS-Fehler mit mandantenübergreifendem Zugriff) getestet. Verantwortlich: [NAME DATENSCHUTZBEAUFTRAGTER]. Letzte Übung: [DATUM LETZTE ÜBUNG].

## 11. Kontaktkette (Eskalation)

Jeder Verdachtsfall wird sofort an die erste erreichbare Stelle der Kette gemeldet. Erreichbarkeit außerhalb der Geschäftszeiten: [REGELUNG RUFBEREITSCHAFT / ERREICHBARKEIT AUSSERHALB DER GESCHÄFTSZEITEN].

| Stufe | Funktion | Name | Telefon | E-Mail |
|---|---|---|---|---|
| 1 | Datenschutzbeauftragter / Datenschutzkoordination | [NAME DATENSCHUTZBEAUFTRAGTER] | [TELEFON DSB] | [E-MAIL DSB] |
| 1 (parallel) | Technische Leitung / Incident-Verantwortlicher | [NAME TECHNISCHE LEITUNG] | [TELEFON TECHNISCHE LEITUNG] | [E-MAIL TECHNISCHE LEITUNG] |
| 2 | Vertretung DSB | [NAME VERTRETUNG DSB] | [TELEFON VERTRETUNG] | [E-MAIL VERTRETUNG] |
| 3 | Geschäftsführung | [NAME GESCHÄFTSFÜHRUNG] | [TELEFON GESCHÄFTSFÜHRUNG] | [E-MAIL GESCHÄFTSFÜHRUNG] |

Sammel-Meldeadresse für interne und externe Hinweise: [MELDEADRESSE, z. B. datenschutz@moto-domain]

## 12. Zuständige Aufsichtsbehörde

Für die Schulen und Schulträger in Nordrhein-Westfalen als Verantwortliche ist zuständig:

**Landesbeauftragte für Datenschutz und Informationsfreiheit Nordrhein-Westfalen (LDI NRW)**
Kavalleriestraße 2-4, 40213 Düsseldorf
Telefon: 0211 38424-0
E-Mail: poststelle@ldi.nrw.de
Meldung von Datenpannen: über das Online-Meldeformular der LDI NRW (Formularserver, erreichbar über www.ldi.nrw.de)

[PRÜFEN: Kontaktdaten und Formular-Link vor jeder Verwendung auf der Website der LDI NRW auf Aktualität prüfen.]

Die Meldung an die LDI NRW gibt der **Verantwortliche** ab. moto meldet nicht selbst an die LDI NRW. Sollte die LDI NRW moto direkt kontaktieren, informiert moto unverzüglich den betroffenen Verantwortlichen und stimmt die Beantwortung mit ihm ab.

## 13. Mitgeltende Dokumente

- Auftragsverarbeitungsvertrag (AVV, Dokument 01) mit dem jeweiligen Verantwortlichen, insbesondere die Meldeklausel (§ 8)
- Dokument 04: Verzeichnis von Verarbeitungstätigkeiten (Datenkategorien und Speicherorte)
- Dokument 02: Technische und organisatorische Maßnahmen (TOM)
- Subprozessorenliste (Dokument 03) inkl. Meldepflichten der Unterauftragsverarbeiter
- Rechtsrahmen: DSGVO (insb. Art. 4 Nr. 12, Art. 28, 33, 34), §§ 120 bis 122 SchulG NRW, VO-DV I

---

# Anhang A: Meldeformular Datenschutzverletzung (Meldung des Auftragsverarbeiters an den Verantwortlichen nach Art. 33 Abs. 2 DSGVO)

*Dieses Formular wird von moto ausgefüllt und an den Verantwortlichen übermittelt. Die Gliederung folgt Art. 33 Abs. 3 lit. a bis d DSGVO, damit der Verantwortliche die Angaben unmittelbar für seine eigene Meldung an die LDI NRW verwenden kann. Noch nicht bekannte Angaben sind als "wird nachgemeldet" zu kennzeichnen.*

## A.1 Verwaltungsangaben

| Feld | Angabe |
|---|---|
| Vorfallnummer (moto-intern) | _______________________ |
| Meldung | ☐ Erstmeldung  ☐ Nachmeldung zu Vorfallnummer: ______________ |
| Datum und Uhrzeit dieser Meldung | _______________________ |
| Auftragsverarbeiter | moto, [RECHTSFORM UND ADRESSE] |
| Anlaufstelle bei moto (Art. 33 Abs. 3 lit. b) | [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKTDATEN DSB] |
| Verantwortlicher (Empfänger dieser Meldung) | Schule/Schulträger: _______________________ |
| Empfangende Stelle beim Verantwortlichen | _______________________ |
| Betroffene weitere Verantwortliche (bei plattformweitem Vorfall) | ☐ nein  ☐ ja, Anzahl: ______ (jeweils gesonderte Meldung erfolgt) |

## A.2 Art der Verletzung (Art. 33 Abs. 3 lit. a)

| Feld | Angabe |
|---|---|
| Zeitpunkt des Beginns der Verletzung (soweit bekannt) | _______________________ |
| Zeitpunkt der Feststellung / Kenntniserlangung bei moto | _______________________ |
| Dauert die Verletzung an? | ☐ ja  ☐ nein, beendet am: ______________ |
| Verletztes Schutzziel | ☐ Vertraulichkeit  ☐ Integrität  ☐ Verfügbarkeit |
| Art des Vorfalls | ☐ unbefugter Zugriff  ☐ Datenabfluss/Offenlegung  ☐ Fehlversand  ☐ Verlust/Diebstahl Gerät  ☐ Fehlkonfiguration  ☐ Schadsoftware/Angriff  ☐ Datenverlust/-vernichtung  ☐ Vorfall bei Unterauftragsverarbeiter  ☐ Sonstiges: ______________ |
| Betroffene Systeme/Komponenten | _______________________________________________ |
| Sachverhaltsbeschreibung (was ist passiert, wie wurde es entdeckt) | _______________________________________________ _______________________________________________ _______________________________________________ |

## A.3 Betroffene Personen und Datensätze (Art. 33 Abs. 3 lit. a)

| Feld | Angabe |
|---|---|
| Kategorien betroffener Personen | ☐ Schüler:innen (Kinder)  ☐ Erziehungsberechtigte  ☐ Personal  ☐ sonstige: ______________ |
| Ungefähre Zahl betroffener Personen | ______________ (☐ Schätzung, wird präzisiert) |
| Kategorien betroffener Daten | ☐ Stammdaten (Name, Geburtsdatum, Adresse, Klasse) ☐ Kontaktdaten Erziehungsberechtigte (E-Mail, Telefon, Adresse) ☐ Anwesenheits-/Bewegungsdaten (Check-in/-out, Raumaufenthalt) ☐ Abholberechtigungen / Notfallkontakte ☐ **Gesundheitsangaben (Art. 9 DSGVO)**, z. B. Gesundheitsinfo-Freitext, Krankmeldungen ☐ Foto ☐ RFID-Tag-Kennungen ☐ Zugangsdaten (E-Mail, Passwort-/PIN-Hashes, Tokens) ☐ Login-/Protokolldaten (IP-Adressen, Zeitstempel) ☐ Zeiterfassung/Abwesenheiten Personal ☐ Anmeldedaten (Enrollment) ☐ Nachrichten Elternportal ☐ sonstige: ______________ |
| Ungefähre Zahl betroffener Datensätze | ______________ (☐ Schätzung, wird präzisiert) |
| Besondere Kategorien (Art. 9 DSGVO) betroffen? | ☐ ja  ☐ nein  ☐ noch unklar |
| Daten von Kindern betroffen? | ☐ ja  ☐ nein  ☐ noch unklar |
| Waren die betroffenen Daten technisch geschützt (z. B. verschlüsselt, gehasht)? | ☐ ja, wie: ______________  ☐ nein  ☐ teilweise: ______________ |

## A.4 Wahrscheinliche Folgen (Art. 33 Abs. 3 lit. c)

*Sachverhaltsbezogene Einschätzung von moto als Auftragsverarbeiter. Die Risikobewertung im Sinne von Art. 33 Abs. 1 und Art. 34 DSGVO nimmt der Verantwortliche vor.*

| Feld | Angabe |
|---|---|
| Wahrscheinliche Folgen für die Betroffenen aus technischer Sicht | _______________________________________________ _______________________________________________ |
| Anhaltspunkte für tatsächlichen Abruf/Missbrauch der Daten? | ☐ ja: ______________  ☐ nein  ☐ nicht feststellbar |

## A.5 Ergriffene und vorgeschlagene Maßnahmen (Art. 33 Abs. 3 lit. d)

| Feld | Angabe |
|---|---|
| Bereits ergriffene Maßnahmen (mit Zeitpunkt) | _______________________________________________ _______________________________________________ |
| Geplante/vorgeschlagene weitere Maßnahmen | _______________________________________________ |
| Ist die Ursache behoben? | ☐ ja  ☐ nein, voraussichtlich bis: ______________ |
| Empfohlene Maßnahmen auf Seiten des Verantwortlichen (z. B. Information des Kollegiums, Passwortwechsel) | _______________________________________________ |

## A.6 Offene Punkte und Nachmeldung

| Feld | Angabe |
|---|---|
| Noch ausstehende Informationen | _______________________________________________ |
| Voraussichtlicher Zeitpunkt der Nachmeldung | _______________________ |

## A.7 Hinweise für den Verantwortlichen

1. Die Frist für Ihre Meldung an die Aufsichtsbehörde (72 Stunden, Art. 33 Abs. 1 DSGVO) beginnt mit **Ihrer** Kenntnisnahme dieser Meldung, sofern die Verletzung voraussichtlich zu einem Risiko für die Rechte und Freiheiten natürlicher Personen führt.
2. Zuständige Aufsichtsbehörde: Landesbeauftragte für Datenschutz und Informationsfreiheit Nordrhein-Westfalen (LDI NRW), Kavalleriestraße 2-4, 40213 Düsseldorf, Meldung über das Online-Meldeformular der LDI NRW (www.ldi.nrw.de).
3. Die Entscheidung über eine Benachrichtigung der betroffenen Personen (Art. 34 DSGVO) obliegt Ihnen als Verantwortlichem. moto unterstützt Sie auf Anforderung gemäß Art. 28 Abs. 3 lit. f DSGVO.
4. Für Rückfragen: [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKTDATEN DSB].

| Unterschrift / Freigabe der Meldung (moto) | |
|---|---|
| Name, Funktion | _______________________ |
| Datum, Uhrzeit | _______________________ |
