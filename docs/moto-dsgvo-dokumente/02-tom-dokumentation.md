# Technische und organisatorische Maßnahmen (TOM) nach Art. 32 DSGVO

| | |
|---|---|
| **Dokument** | 02 Technische und organisatorische Maßnahmen gemäß Art. 32 DSGVO |
| **Auftragsverarbeiter** | moto [RECHTSFORM UND ADRESSE] |
| **Verarbeitungssystem** | moto (internes Projektkürzel "Project Phoenix"), NFC/RFID-gestütztes Anwesenheits- und Raumverwaltungssystem für den Offenen Ganztag (OGS) |
| **Version** | 1.0 |
| **Stand** | 2026-07-07 |
| **Status** | Entwurf zur internen Prüfung |
| **Letzte Überprüfung** | 2026-07-07 (Erstfassung) |
| **Verantwortlich für dieses Dokument** | [NAME DATENSCHUTZKOORDINATOR/IN], Datenschutzkoordination moto |
| **Datenschutzbeauftragte/r** | [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKTDATEN DSB] |

---

## 1. Zweck und Geltungsbereich

Dieses Dokument beschreibt die technischen und organisatorischen Maßnahmen, die moto als Auftragsverarbeiter im Sinne des Art. 28 DSGVO für das System moto ("Project Phoenix") getroffen hat. Es dient als Anlage zum Auftragsverarbeitungsvertrag zwischen dem jeweiligen Schulträger bzw. der Schule als Verantwortlichem (Art. 4 Nr. 7 DSGVO) und moto und erbringt den Nachweis nach Art. 28 Abs. 3 lit. c in Verbindung mit Art. 32 DSGVO sowie im Rahmen der Rechenschaftspflicht nach Art. 5 Abs. 2 DSGVO.

Der Einsatz des Systems erfolgt im Anwendungsbereich der §§ 120 bis 122 SchulG NRW und der VO-DV I NRW. Rechtsgrundlage des Kernbetriebs ist Art. 6 Abs. 1 lit. e DSGVO. § 2 Abs. 1 VO-DV I knüpft die Zulässigkeit der automatisierten Verarbeitung ausdrücklich an das Vorhandensein technischer und organisatorischer Maßnahmen, die die Sicherheit der Verarbeitung gewährleisten. Die hier beschriebenen Maßnahmen sind damit Voraussetzung des zulässigen Betriebs, nicht bloße Ergänzung.

Beschrieben werden ausschließlich Maßnahmen, die im System tatsächlich implementiert sind oder die sich aus dem Hosting-Setup ergeben. Organisatorische Maßnahmen, deren schriftliche Fixierung noch aussteht, sind mit dem Hinweis [ORGANISATORISCH ZU BESTÄTIGEN] gekennzeichnet. Rechtlich oder tatsächlich noch zu verifizierende Punkte sind mit [PRÜFEN] markiert.

## 2. Risikoorientierung (Art. 32 Abs. 1 und Abs. 2 DSGVO)

Die Auswahl der Maßnahmen berücksichtigt Art, Umfang, Umstände und Zwecke der Verarbeitung sowie Eintrittswahrscheinlichkeit und Schwere der Risiken. Für das System ergeben sich insbesondere folgende risikoerhöhende Faktoren:

1. **Betroffene sind überwiegend Kinder** (Grundschulkinder in der OGS-Betreuung), daneben Erziehungsberechtigte und Beschäftigte der Träger.
2. **Gesundheitsdaten nach Art. 9 DSGVO** werden verarbeitet: Freitextfeld zu Gesundheitsinformationen des Kindes, strukturierte Krankmeldungen von Kindern sowie Krankheitsabwesenheiten des Personals.
3. **Anwesenheits- und Aufenthaltsdaten** (Check-in/Check-out, Raumaufenthalte) erlauben Rückschlüsse auf das tagesaktuelle Bewegungsverhalten von Kindern.
4. **Mehrmandantenbetrieb**: Daten mehrerer Schulen werden in einer gemeinsamen Infrastruktur verarbeitet; eine Vermischung oder mandantenübergreifende Offenlegung ist als Hauptrisiko der Architektur adressiert (Abschnitt 3.8).

Die Maßnahmen sind entsprechend auf die Schutzziele Vertraulichkeit, Integrität, Verfügbarkeit und Belastbarkeit (Art. 32 Abs. 1 lit. b DSGVO) sowie auf den Schutz vor Vernichtung, Verlust, Veränderung, unbefugter Offenlegung und unbefugtem Zugang (Art. 32 Abs. 2 DSGVO) ausgerichtet.

## 3. Maßnahmen nach Kontrollbereichen

Die Gliederung folgt den in der Prüfpraxis kommunaler IT-Dienstleister etablierten acht Kontrollbereichen, ergänzt um die DSGVO-spezifischen Kategorien aus Art. 32 Abs. 1. Eine Zuordnungstabelle zu Art. 32 Abs. 1 DSGVO enthält Abschnitt 4.

### 3.1 Zutrittskontrolle (physische Sicherheit)

Ziel: Unbefugten den physischen Zutritt zu Datenverarbeitungsanlagen verwehren.

| Maßnahme | Umsetzung |
|---|---|
| Rechenzentrumsbetrieb | Die Produktions- und Staging-Systeme werden auf dedizierten Servern der Hetzner Online GmbH am Standort Nürnberg (Deutschland) betrieben. moto betreibt keine eigenen Serverräume. |
| Zutrittskontrolle im Rechenzentrum | Die physische Zutrittskontrolle (Zutrittsregelung, Videoüberwachung, Sicherheitsdienst) liegt beim Rechenzentrumsbetreiber. Hetzner ist für den Rechenzentrumsbetrieb nach ISO/IEC 27001 zertifiziert. Der Nachweis erfolgt über den Auftragsverarbeitungsvertrag mit Hetzner und die dort referenzierten Zertifikate. [PRÜFEN: aktuelles ISO-27001-Zertifikat und AVV-Anlage von Hetzner der TOM-Dokumentation beilegen] |
| Endgeräte der Mitarbeitenden | Regelungen zu Verschlüsselung und Absicherung von Arbeitsgeräten der moto-Mitarbeitenden (Festplattenverschlüsselung, Bildschirmsperre) [ORGANISATORISCH ZU BESTÄTIGEN] |
| Kiosk-Geräte an den Schulen | Die NFC-Kioskgeräte (Raspberry Pi) an den Schulen speichern keine personenbezogenen Datenbestände lokal vor; sie kommunizieren über authentifizierte Schnittstellen mit dem Server (Abschnitt 3.2). Die physische Sicherung der Geräte in den Schulräumen obliegt dem Verantwortlichen. |

### 3.2 Zugangskontrolle (Authentifizierung)

Ziel: Nutzung der Systeme durch Unbefugte verhindern.

| Maßnahme | Umsetzung |
|---|---|
| Passwortspeicherung | Passwörter und Personal-PINs werden ausschließlich als Argon2id-Hashes gespeichert (Speicherparameter 64 MB, 3 Iterationen, Parallelität 2, 16 Byte Salt). Die Verifikation erfolgt mit konstantem Zeitvergleich zur Abwehr von Timing-Angriffen. Eine Klartextspeicherung findet an keiner Stelle statt. |
| Sitzungsverwaltung | Die Authentifizierung erfolgt über JWT mit kurzer Gültigkeit: Access-Token 15 Minuten, Refresh-Token 168 Stunden. Ein in der Produktionsumgebung fehlendes oder unsicheres JWT-Secret verhindert den Serverstart (Fail-fast-Prinzip). |
| Mehrfaktor-Authentifizierung (MFA) | Für Schul- und Betreiberkonten steht ein MFA-Subsystem zur Verfügung: E-Mail-Einmalcodes, Wiederherstellungscodes, vertrauenswürdige Geräte (Standardgültigkeit 90 Tage, konfigurierbar 1 bis 180 Tage) sowie Passkeys (WebAuthn). Fehlversuche führen zu einer MFA-Sperre auf Kontoebene. |
| Kontosperrung bei Fehlversuchen | Schwellenwert und Sperrdauer für fehlgeschlagene Anmeldeversuche sind je Schule konfigurierbar (Einstellungsregistrierung, keine hartkodierten Werte). |
| Kiosk-Authentifizierung (NFC-Geräte) | Jedes Kioskgerät authentifiziert sich mit einem gerätespezifischen API-Schlüssel (Bearer-Token); zusätzlich ist für betreuungsrelevante Aktionen die persönliche PIN der aufsichtführenden Fachkraft erforderlich (Zwei-Komponenten-Charakter: Gerätemerkmal plus persönliches Merkmal). Der Schlüsselvergleich erfolgt zeitkonstant; deaktivierte Geräte werden abgewiesen. |
| Ratenbegrenzung | Anfragen werden serverseitig ratenbegrenzt (Token-Bucket je Client-IP). Für Passwort-Zurücksetzungen und öffentliche Anmeldeformulare bestehen zusätzliche, datenbankgestützte Ratenbegrenzungen. |
| Datenbankzugang nach Minimalprinzip | Der Anwendungsserver verbindet sich mit einer eigens angelegten Datenbankrolle mit minimalen, tabellenweise vergebenen Rechten. Administrative Rollen sind hiervon getrennt (Abschnitt 3.8). |
| Portaltrennung beim Login | Schul-, Betreiber- und Elternportal verwenden getrennte Anmeldeinstanzen mit jeweils eigenem Sitzungs-Cookie. Elternkonten werden am Schulportal abgewiesen; das Elternportal akzeptiert ausschließlich Konten mit Erziehungsberechtigten-Rolle. |

### 3.3 Zugriffskontrolle (Autorisierung)

Ziel: Sicherstellen, dass Berechtigte ausschließlich auf die ihrer Aufgabe entsprechenden Daten zugreifen können (Need-to-know).

| Maßnahme | Umsetzung |
|---|---|
| Rollen- und Berechtigungsmodell | Zugriffe werden über ein zentrales Rollen- und Berechtigungsmodell mit einer eigenen Autorisierungskomponente (Policy-Engine) gesteuert. Berechtigungen sind kontobezogen und mandantenbezogen zugeordnet. |
| Zweistufiges Berechtigungssystem | Berechtigungen des Personals (rollenbasiert) und Berechtigungen von Erziehungsberechtigten sind getrennte Systeme. Eltern-Berechtigungen werden je Kind-Beziehung einzeln gespeichert und geprüft. Eine Abholberechtigung oder Notfallkontakt-Kennzeichnung begründet keinen automatischen Portalzugriff auf Kinddaten. |
| Datensichtbarkeit Kinderdaten | Der Lesezugriff auf vollständige Kinderdaten (Profil, Aufenthaltsort, Datenschutzangaben, Abholpläne) ist je Schule konfigurierbar; Voreinstellung ist die Beschränkung auf die Gruppenbetreuung. Der Schreibzugriff bleibt unabhängig davon stets auf die Gruppenbetreuung beschränkt (Trennung von Lese- und Schreibrechten). |
| Anwesenheitsprotokoll standardmäßig deaktiviert | Das rückblickende Anwesenheits- und Raumprotokoll für Mitarbeitende ist in der Voreinstellung abgeschaltet und muss vom Verantwortlichen bewusst aktiviert werden. Sichtbarkeitsfenster (Voreinstellung 30 Tage für An-/Abmeldezeiten, 7 Tage für Raumdetails) und der berechtigte Personenkreis sind konfigurierbar. |
| Zugriffsschutz der Systemeinstellungen | Einstellungen sind zweidimensional geschützt: über Berechtigungen (getrennte Rechte für operative und für datenschutz-/sicherheitsrelevante Einstellungen) und über Sichtbarkeitsrichtlinien (bestimmte Einstellungen nur für Schuladministration, andere nur für den Plattformbetreiber sichtbar). |
| Architektonische Absicherung | Ein automatisierter Architektur-Test in der Build-Pipeline verhindert, dass HTTP-Handler unter Umgehung der Service- und Autorisierungsschicht direkt auf die Datenzugriffsschicht zugreifen. |

### 3.4 Weitergabekontrolle (Transport und Übermittlung)

Ziel: Schutz personenbezogener Daten bei elektronischer Übertragung.

| Maßnahme | Umsetzung |
|---|---|
| Transportverschlüsselung extern | Sämtliche externen Verbindungen zu den Portalen und Schnittstellen werden über einen Reverse Proxy (Caddy) TLS-verschlüsselt terminiert. DNS und vorgeschaltete Auslieferung erfolgen über Cloudflare. |
| Transportverschlüsselung zur Datenbank | Die Verbindung des Anwendungsservers zur PostgreSQL-Datenbank ist in der Produktionsumgebung TLS-verschlüsselt mit Zertifikatsprüfung (sslmode=verify-ca mit hinterlegtem Wurzelzertifikat). |
| Cookie-Härtung | Alle Sitzungs-Cookies sind httpOnly und in der Produktionsumgebung secure. Das Eltern-Portal-Cookie ist zusätzlich mit SameSite=Strict gesetzt (CSRF-Schutz beim Zugriff auf Kinddaten). Betreiber- und Eltern-Cookies sind host-gebunden und damit nicht subdomainübergreifend lesbar. |
| Schutz von Zugangsdaten in Protokollen | Die Zugriffprotokolle des Reverse Proxy maskieren die Header Authorization, Cookie, X-Device-Key und X-Staff-PIN und kürzen IP-Adressen (Maskierung auf /24 bzw. /56). Zugangsdaten gelangen dadurch nicht in Protokolldateien. |
| Secrets-Verwaltung | Zugangsdaten und Konfigurationsgeheimnisse der Staging- und Produktionsumgebung liegen ausschließlich SOPS/age-verschlüsselt in der Versionsverwaltung. Die Build-Pipeline prüft vor jedem Deployment automatisiert, dass keine Klartextwerte enthalten sind. Ein Pre-Commit-Hook (git-secrets) prüft lokal auf versehentlich eingecheckte Zugangsdaten. |
| Datenträgertransport | Ein physischer Transport von Datenträgern mit personenbezogenen Daten findet im Regelbetrieb nicht statt. |

### 3.5 Eingabekontrolle (Protokollierung und Nachvollziehbarkeit)

Ziel: Nachträglich feststellen können, wer wann welche personenbezogenen Daten eingegeben, verändert, eingesehen oder gelöscht hat.

| Maßnahme | Umsetzung |
|---|---|
| Datenzugriffsprotokoll | Lesezugriffe auf sensible Historien- und Anwesenheitsdaten werden protokolliert (handelndes Konto, Rolle, Ressourcentyp, betroffenes Kind, Zeitraum, Zeitpunkt), einschließlich Exporten. Das Protokoll ist mandantengetrennt gespeichert. |
| Anmeldeereignisse | Anmeldungen, Abmeldungen, Fehlversuche und MFA-Ereignisse werden je Konto mit Ereignistyp, Erfolg, IP-Adresse, User-Agent und Zeitpunkt protokolliert. |
| Änderungsprotokolle | Fachliche Änderungen werden in dedizierten, nur anfügbaren Protokolltabellen festgehalten: Änderungen an Kontakt- und Abholdaten der Erziehungsberechtigten, Korrekturen an Zeiterfassungseinträgen (Alt-/Neuwert je Feld, editierende Person), administrative Korrekturen an Anmeldedaten sowie Datenimporte (wer, wann, welche Datei). |
| Löschprotokoll | Löschvorgänge an Kind- und Personaldaten werden mit Löschart, Anzahl gelöschter Datensätze, Löschgrund, ausführender Person und Zeitpunkt protokolliert (Nachweis für Art. 17 DSGVO). |
| Einstellungs-Änderungsprotokoll | Jede Änderung an Systemeinstellungen wird nur anfügbar protokolliert (Aktion, alter und neuer Wert); Passwortwerte werden im Protokoll geschwärzt. |
| Betreiberprotokoll | Aktionen des Plattformbetreibers werden in einem eigenen Audit-Protokoll festgehalten. |
| Datenschutzgerechte Anwendungsprotokolle | Die Anwendung protokolliert strukturiert. Es gilt die kodifizierte Vorgabe, dass Namen von Kindern nicht auf der regulären Protokollstufe erscheinen; verwendet werden numerische Kennungen. |

Hinweis zur Speicherbegrenzung der Protokolle: Für die Audit-Protokolle selbst besteht derzeit keine automatisierte Löschfrist; sie werden als Nachweis-Trail unbegrenzt vorgehalten. [PRÜFEN: Aufbewahrungskonzept für Audit-Protokolle festlegen und mit dem Grundsatz der Speicherbegrenzung (Art. 5 Abs. 1 lit. e DSGVO) abgleichen]

### 3.6 Auftragskontrolle (Weisungsbindung und Unterauftragsverarbeiter)

Ziel: Verarbeitung nur nach dokumentierter Weisung des Verantwortlichen; Kontrolle der eingesetzten weiteren Auftragsverarbeiter.

| Maßnahme | Umsetzung |
|---|---|
| Weisungsbindung | Die Verarbeitung erfolgt ausschließlich zur Erbringung der vertraglich vereinbarten Leistung. Ein dokumentiertes Weisungs- und Kommunikationsverfahren mit den Verantwortlichen [ORGANISATORISCH ZU BESTÄTIGEN] |
| Mandantenkonfiguration durch den Verantwortlichen | Datenschutzrelevante Betriebsparameter (Aufbewahrungsfristen, Protokollumfang, Sichtbarkeitskreise) werden je Schule durch den Verantwortlichen selbst konfiguriert; moto gibt datenschutzfreundliche Voreinstellungen vor (Abschnitt 3.9). |
| Unterauftragsverarbeiter | Siehe nachstehende Übersicht. Die vollständige, vertraglich gepflegte Liste einschließlich AVV-Status wird als eigene Anlage zum Auftragsverarbeitungsvertrag geführt: Dokument 03 (Subprozessorenliste, Anlage 3 zum AVV) |

Eingesetzte Dienste (Ist-Stand):

| Dienst | Anbieter, Sitz | Zweck | Personenbezug | Drittlandbezug |
|---|---|---|---|---|
| Server-Hosting | Hetzner Online GmbH, Deutschland (Rechenzentrum Nürnberg) | Betrieb von Anwendung, Datenbank und Monitoring | Alle im System verarbeiteten Datenkategorien | Nein (Verarbeitung in Deutschland) |
| DNS / vorgeschaltete Auslieferung | Cloudflare, Inc., USA | DNS, CDN, Schutz vor Überlastangriffen | Verbindungsmetadaten (IP-Adressen) der Portalnutzer beim Verbindungsaufbau | Ja. [PRÜFEN: Cloudflare-DPA, EU-Datenlokalisierungsoptionen und Transfermechanismus (EU-US Data Privacy Framework / Standardvertragsklauseln) dokumentieren] |
| Cloudflare Turnstile (Captcha) | Cloudflare, Inc., USA | Bot-Schutz des öffentlichen Anmeldeformulars; in der Voreinstellung deaktiviert, je Schule aktivierbar | IP-Adresse und Browsersignale der ausfüllenden Person | Ja, nur bei Aktivierung. [PRÜFEN: Transfermechanismus; Information der Betroffenen im Anmeldeformular] |
| GitHub / GitHub Container Registry | GitHub, Inc. (Microsoft), USA | Quellcodeverwaltung, Build-Pipeline, Auslieferung der Container-Images | Keine Daten der Endnutzer; nur Anwendungscode und Build-Artefakte | Ja, ohne Endnutzerdaten. [PRÜFEN: DPF-Status von GitHub/Microsoft dokumentieren] |
| E-Mail-Versand (SMTP) | [SMTP-ANBIETER UND SITZ] | Versand von Einladungs-, Passwort-, MFA- und Anmelde-E-Mails | E-Mail-Adressen, Namen in der Anrede, Einladungs- und Zurücksetzungslinks | [PRÜFEN: Anbieter und Serverstandort verifizieren, AVV abschließen] |
| Fehlerprotokollierung (Sentry, optional) | Functional Software, Inc., USA; Konfiguration verweist auf EU-Region | Fehler- und Ausnahmeüberwachung | Stacktraces und Request-Metadaten; implementierte Bereinigung entfernt vor dem Versand Authorization- und Cookie-Header sowie IP-Adresse, E-Mail und Benutzername | [PRÜFEN: tatsächliche Projektregion (EU) im Sentry-Konto verifizieren] |
| Produktnutzungsanalyse (PostHog, optional) | PostHog, Inc.; konfiguriert auf EU-Cloud (eu.i.posthog.com) | Nutzungsanalyse der Weboberfläche | Nutzungsereignisse angemeldeter Nutzer | [PRÜFEN: produktiven Hostwert und Ereignisinhalte verifizieren] |
| Monitoring (Grafana, Loki, Alloy, Prometheus) | Eigenbetrieb auf demselben Hetzner-Server | Protokollaggregation, Metriken, Alarmierung | Anwendungsprotokolle (ohne Kindernamen, IP-maskiert, Zugangsdaten geschwärzt) | Nein (Eigenbetrieb, Deutschland) |

### 3.7 Verfügbarkeitskontrolle und Wiederherstellbarkeit (Art. 32 Abs. 1 lit. b und c)

Ziel: Schutz vor Zerstörung und Verlust; rasche Wiederherstellung nach Zwischenfällen.

| Maßnahme | Umsetzung |
|---|---|
| Datensicherung vor jeder Änderung am Datenbestand | Der automatisierte Deployment-Prozess erstellt vor jeder Datenbankmigration eine vollständige Sicherung: Sicherung der Datenbankrollen und globalen Objekte, vollständiger Datenbank-Dump im wiederherstellbaren Custom-Format sowie Sicherung des Datei-Uploads-Volumes. Bei leerer oder fehlgeschlagener Sicherung wird das Deployment abgebrochen. |
| Automatischer Rollback | Schlägt eine Migration oder der anschließende Funktionstest fehl, stellt der Deployment-Prozess den vorherigen Stand automatisch aus der Sicherung wieder her. Die Ergebniszustände sind über definierte Exit-Codes maschinell auswertbar; ein fehlgeschlagener Rollback wird als kritisches Ereignis behandelt. Für manuelle Rücknahmen existiert ein eigener Rollback-Workflow in der Build-Pipeline. |
| Aufbewahrung der Sicherungen | Sicherungen werden serverseitig vorgehalten und automatisch rotiert (Aufbewahrung: 3 Generationen Staging, 7 Generationen Produktion). Die Sicherungsdatei der Datenbankrollen ist mit restriktiven Dateirechten (nur Eigentümer) geschützt. |
| Regelmäßige, deploymentunabhängige Sicherungen | [PRÜFEN: Sicherungen sind derzeit an Deployments gekoppelt; eine zeitgesteuerte, deploymentunabhängige Sicherung mit dokumentierter Frequenz ist festzulegen: [BACKUP-FREQUENZ UND -AUFBEWAHRUNGSDAUER]] |
| Wiederherstellungstests | [ORGANISATORISCH ZU BESTÄTIGEN: turnusmäßige Wiederherstellungstests der Datenbanksicherungen dokumentieren] |
| Überwachung und Alarmierung | Zentrale Protokoll- und Metriküberwachung (Grafana/Loki/Prometheus) mit Alarmregeln. Der Anwendungsserver stellt einen Health-Endpunkt bereit, der von dedizierten Prüf-Containern für Produktion und Staging minütlich aktiv abgefragt wird; ein Ausfall wird innerhalb weniger Minuten alarmiert. |
| Ressourcenschutz | Konfigurierte Verbindungspool-Grenzen der Datenbankanbindung verhindern Ressourcenerschöpfung; Ratenbegrenzungen (Abschnitt 3.2) begrenzen Lastspitzen durch einzelne Clients. Vorgeschalteter Überlastschutz durch Cloudflare. |
| Getrennte Umgebungen | Staging- und Produktionsumgebung sind vollständig getrennt (eigene Server-Verzeichnisse, eigene verschlüsselte Konfigurationsbestände, eigene Sicherungen). Testdatenbanken laufen in isolierten Netzen; Testdaten-Generatoren sind technisch auf Entwicklungsumgebungen beschränkt. |

### 3.8 Trennungskontrolle (Mandantentrennung)

Ziel: Getrennte Verarbeitung der Daten unterschiedlicher Verantwortlicher (Schulen) im Mehrmandantenbetrieb.

| Maßnahme | Umsetzung |
|---|---|
| Row Level Security auf Datenbankebene | Alle mandantenbezogenen Tabellen (über 58 Tabellen in 15 fachlichen Datenbankschemata) tragen eine Mandantenkennung und sind durch PostgreSQL Row Level Security geschützt. Die Richtlinien sind mit FORCE ROW LEVEL SECURITY erzwungen und binden jede Zeile an die Mandantenkennung der aktuellen Sitzung. Ein Zugriff über Mandantengrenzen hinweg ist damit auf Datenbankebene ausgeschlossen, nicht nur auf Anwendungsebene. |
| Mandantentransaktion je Anfrage | Eine zentrale Middleware wickelt jede mandantenbezogene Anfrage in eine eigene Datenbanktransaktion: Sie setzt die Sitzungsrolle auf die RLS-pflichtige Mandantenrolle und die Mandantenkennung als Sitzungsvariable und rollt die Transaktion bei Serverfehlern automatisch zurück. Teilschreibvorgänge bei Fehlern werden dadurch verhindert. |
| Getrennte Datenbankrollen | Die RLS-pflichtige Mandantenrolle (Zugriff nur auf den eigenen Mandanten) ist von der administrativen Rolle (nur für Betreiberfunktionen, Migrationen) getrennt. Rechte sind tabellenweise minimal vergeben. |
| Mandantenkennung im Datenmodell | Die Mandantenkennung ist als Pflichtfeld im Basisdatenmodell verankert und wird über den gesamten Anfragezyklus im Anwendungskontext mitgeführt und automatisch befüllt. |
| Portal- und Bereichstrennung | Drei getrennte Portale (Schule/Träger, Betreiber, Eltern) mit eigenen Anmeldeinstanzen, eigenen Cookies und eigenen JWT-Geltungsbereichen. Serverseitige Middleware weist Eltern-Token an Schul-Endpunkten und Nicht-Eltern-Token an Eltern-Endpunkten zurück (mehrschichtige Absicherung zusätzlich zur Cookie-Trennung). |
| Betreiberdaten getrennt | Konten und Protokolle des Plattformbetreibers liegen in einer eigenen, nicht mandantengebundenen Tabellenfamilie mit eigenem Portal. |

### 3.9 Pseudonymisierung und Verschlüsselung (Art. 32 Abs. 1 lit. a)

| Maßnahme | Umsetzung |
|---|---|
| RFID-Kennungen ohne Personenbezug auf dem Medium | Die eingesetzten RFID/NFC-Medien enthalten ausschließlich eine technische Kennung (Tag-UID). Die Zuordnung zur Person erfolgt allein serverseitig in der Datenbank. Auf dem Medium selbst sind keine Namen, Gesundheits- oder Anwesenheitsdaten gespeichert. |
| Pseudonyme Kennungen in Protokollen | Anwendungsprotokolle verwenden numerische Kennungen statt Namen; IP-Adressen in Proxy-Protokollen werden maskiert (Abschnitt 3.4 und 3.5). |
| Hashing von Geheimnissen | Passwörter und PINs ausschließlich als Argon2id-Hashes (Abschnitt 3.2). |
| Verschlüsselung bei Übertragung | TLS für alle externen Verbindungen und für die Datenbankanbindung (Abschnitt 3.4). |
| Verschlüsselung von Konfigurationsgeheimnissen | SOPS/age-Verschlüsselung sämtlicher Umgebungsgeheimnisse (Abschnitt 3.4). |
| Verschlüsselung ruhender Nutzdaten | [PRÜFEN: Eine Datenträger- oder Datenbankverschlüsselung der Produktionsserver ist im Systembestand nicht dokumentiert. Ist-Zustand feststellen und entweder nachweisen oder als Maßnahme einführen] |

### 3.10 Datenminimierung und Speicherbegrenzung (Art. 5 Abs. 1 lit. c und e, Art. 25 DSGVO)

| Maßnahme | Umsetzung |
|---|---|
| Automatisierte tägliche Datenbereinigung | Ein Scheduler führt täglich (Voreinstellung 02:00 Uhr) je Schule automatisierte Löschläufe aus. Die Bereinigung ist in der Voreinstellung aktiviert. |
| Aufbewahrung von Anwesenheits- und Aufenthaltsdaten | Voreinstellung 30 Tage, einstellbar 1 bis 31 Tage. Individuelle, je Kind dokumentierte Einwilligungen können innerhalb dieses Rahmens eine eigene Frist setzen. |
| Aufbewahrung von Feedback-Einträgen | Voreinstellung 90 Tage (1 bis 365 Tage). |
| Aufbewahrung abgelehnter Anmeldungen | Voreinstellung 90 Tage (höchstens 730 Tage). |
| Aufbewahrung der Personalzeiterfassung | Voreinstellung 730 Tage (2 Jahre), wählbar bis 2920 Tage; die Stufen orientieren sich an den gesetzlichen Aufbewahrungspflichten (§ 16 Abs. 2 ArbZG, § 41 EStG, § 147 AO). |
| Aufbewahrung abgeschlossener Betreuungsplan-Termine | Voreinstellung 365 Tage (1 bis 1825 Tage). |
| Token-Bereinigung | Abgelaufene Sitzungs-, Zurücksetzungs- und Einladungstoken werden automatisiert gelöscht. Eltern-Einladungslinks sind in der Voreinstellung 48 Stunden gültig (höchstens eine Woche). |
| Datenschutzfreundliche Voreinstellungen | Rückblickendes Anwesenheitsprotokoll in der Voreinstellung deaktiviert; Sichtbarkeit von Kinderdaten in der Voreinstellung auf die Gruppenbetreuung beschränkt; Captcha-Drittdienst in der Voreinstellung deaktiviert (Art. 25 Abs. 2 DSGVO). |
| Löschnachweis | Jeder Löschlauf wird im Löschprotokoll dokumentiert (Abschnitt 3.5). |
| Offene Fristen | Für einzelne Datenbestände (Eltern-Nachrichten, unregistrierte Tag-Scans, Audit-Protokolle) besteht noch keine automatisierte Löschfrist. [PRÜFEN: Löschkonzept um diese Bestände ergänzen] |

### 3.11 Verfahren zur regelmäßigen Überprüfung, Bewertung und Evaluierung (Art. 32 Abs. 1 lit. d)

| Maßnahme | Umsetzung |
|---|---|
| Automatisierte Qualitäts- und Architekturprüfungen | Jede Codeänderung durchläuft eine Build-Pipeline mit automatisierten Tests, statischer Analyse und Architektur-Prüfungen, die sicherheitsrelevante Konventionen erzwingen (u. a. Schichtentrennung, keine Datenbankzugriffe unter Umgehung der Zugriffsschicht, Konfigurations-Synchronisationsprüfung der verschlüsselten Umgebungsdateien). |
| Vier-Augen-Prinzip bei Änderungen | Codeänderungen werden über Pull Requests mit Review eingebracht; direkte Änderungen an Produktionsumgebungen ohne Versionsverwaltung sind durch den SOPS-Prozess ausgeschlossen. |
| Überprüfung dieses Dokuments | Dieses Dokument wird mindestens jährlich sowie anlassbezogen bei Architekturänderungen überprüft und fortgeschrieben. [ORGANISATORISCH ZU BESTÄTIGEN: Überprüfungsturnus verbindlich festlegen und Verantwortlichkeit benennen] |
| Externe Sicherheitsüberprüfungen | [PRÜFEN: Penetrationstests oder externe Sicherheitsaudits sind bislang nicht dokumentiert; Planung und Turnus festlegen] |
| Zertifizierungen (Art. 32 Abs. 3 DSGVO) | moto verfügt derzeit über keine eigene Zertifizierung nach Art. 42 DSGVO und beruft sich nicht auf einen genehmigten Verhaltenskodex nach Art. 40 DSGVO. Der Rechenzentrumsbetreiber Hetzner ist nach ISO/IEC 27001 zertifiziert. [ZERTIFIZIERUNGEN FALLS VORHANDEN] |

### 3.12 Organisatorische Rahmenmaßnahmen und Incident-Response

| Maßnahme | Umsetzung |
|---|---|
| Datenschutzbeauftragte/r | [NAME DATENSCHUTZBEAUFTRAGTER], erreichbar unter [KONTAKTDATEN DSB]. [ORGANISATORISCH ZU BESTÄTIGEN: Bestellung dokumentieren] |
| Verpflichtung auf Vertraulichkeit | Alle Mitarbeitenden von moto mit Zugriff auf personenbezogene Daten werden auf die Vertraulichkeit nach Art. 28 Abs. 3 lit. b, Art. 29 DSGVO verpflichtet. [ORGANISATORISCH ZU BESTÄTIGEN: schriftliche Verpflichtungserklärungen] |
| Schulungen | Mitarbeitende werden zu Datenschutz und Informationssicherheit geschult, Turnus: [SCHULUNGSINTERVALL MITARBEITENDE]. [ORGANISATORISCH ZU BESTÄTIGEN] |
| Meldeverfahren bei Datenschutzverletzungen | Technische Grundlage: aktive Überwachung mit Alarmierung (Abschnitt 3.7) ermöglicht die zeitnahe Erkennung von Störungen und Auffälligkeiten. Ein dokumentiertes Verfahren zur Bewertung und zur Unterstützung des Verantwortlichen bei Meldungen nach Art. 33 und 34 DSGVO (Meldung an den Verantwortlichen ohne unangemessene Verzögerung) [ORGANISATORISCH ZU BESTÄTIGEN] |
| Unterstützung bei Betroffenenrechten | Das System stellt technische Grundlagen für die Erfüllung von Betroffenenrechten bereit: dokumentierte Einwilligungen je Kind, elterninitiierte Datenänderungsanfragen mit Prüfworkflow, protokollierte Löschvorgänge. Das organisatorische Verfahren zur Bearbeitung von Betroffenenanfragen über den Verantwortlichen [ORGANISATORISCH ZU BESTÄTIGEN] |
| Berechtigungsvergabe intern | Verfahren zur Vergabe, Überprüfung und zum Entzug administrativer Zugänge (Server, Versionsverwaltung, SOPS-Schlüssel) bei Ein- und Austritt von Mitarbeitenden [ORGANISATORISCH ZU BESTÄTIGEN]. Technische Grundlage: der SOPS-Altersschlüssel wird ausschließlich über sichere Kanäle geteilt; die Deploy-Zugänge liegen als geschützte Secrets in der Build-Pipeline. |

## 4. Zuordnung zu Art. 32 Abs. 1 DSGVO

| Anforderung Art. 32 Abs. 1 | Abgedeckt durch Abschnitt |
|---|---|
| lit. a Pseudonymisierung und Verschlüsselung | 3.4, 3.9 |
| lit. b Vertraulichkeit | 3.1, 3.2, 3.3, 3.4, 3.8 |
| lit. b Integrität | 3.5, 3.8 (Transaktions-Rollback), 3.11 |
| lit. b Verfügbarkeit und Belastbarkeit | 3.7 |
| lit. c rasche Wiederherstellbarkeit | 3.7 (Sicherung, automatischer Rollback, Rollback-Workflow) |
| lit. d regelmäßige Überprüfung und Evaluierung | 3.11 |
| Abs. 2 Risikoangemessenheit | 2 |
| Abs. 3 Verhaltenskodex/Zertifizierung | 3.11 (nicht zutreffend für moto selbst; ISO 27001 des Rechenzentrumsbetreibers) |

Ergänzend abgedeckt: Art. 25 DSGVO (datenschutzfreundliche Voreinstellungen, Abschnitt 3.10), Art. 28 Abs. 3 lit. b DSGVO (Vertraulichkeitsverpflichtung, Abschnitt 3.12), Art. 33/34 DSGVO (Meldeverfahren, Abschnitt 3.12), Art. 5 Abs. 1 lit. c und e DSGVO (Abschnitt 3.10).

## 5. Nachweise und referenzierte Unterlagen

Die folgenden Unterlagen ergänzen dieses Dokument und werden auf Anforderung des Verantwortlichen vorgelegt, soweit vorhanden:

- Auftragsverarbeitungsvertrag und Zertifikatsnachweise des Rechenzentrumsbetreibers Hetzner [PRÜFEN: beilegen]
- Verzeichnis der Unterauftragsverarbeiter mit AVV-Status: Dokument 03 (Subprozessorenliste)
- Datenbestandsaufnahme (gesondertes internes Dokument) und Löschkonzept (Dokument 05 dieser Reihe)
- Verzeichnis von Verarbeitungstätigkeiten nach Art. 30 Abs. 2 DSGVO (Dokument 04 dieser Reihe)
- Nachweise über Vertraulichkeitsverpflichtungen und Schulungen [ORGANISATORISCH ZU BESTÄTIGEN]

## 6. Änderungshistorie

| Version | Datum | Änderung | Bearbeitung |
|---|---|---|---|
| 1.0 | 2026-07-07 | Erstfassung, Entwurf zur internen Prüfung | Datenschutzkoordination moto |
