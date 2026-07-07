# Liste der Subunternehmer (Subprozessoren)

**Dokument 03 der Datenschutzdokumentation — Anlage 3 zum Vertrag über die Auftragsverarbeitung (Art. 28 DSGVO)**

| | |
|---|---|
| Auftragsverarbeiter | moto, [RECHTSFORM UND ADRESSE] |
| Produkt | moto (NFC/RFID-gestütztes Anwesenheits- und Raumverwaltungssystem für den Offenen Ganztag) |
| Verantwortlicher | Die jeweilige Schule bzw. der jeweilige Schulträger gemäß Hauptvertrag |
| Datenschutzbeauftragter des Auftragsverarbeiters | [NAME DATENSCHUTZBEAUFTRAGTER], [KONTAKTDATEN DATENSCHUTZBEAUFTRAGTER] |
| Version | 1.0 |
| Stand | 2026-07-07 |
| Status | Entwurf zur internen Prüfung |

---

## 1. Zweck und Geltungsbereich

Diese Liste benennt gemäß Art. 28 Abs. 2 und Abs. 4 DSGVO alle weiteren Auftragsverarbeiter (Subunternehmer), die moto zur Erbringung der vertraglich vereinbarten Leistungen einsetzt. Sie ist Bestandteil des Vertrages über die Auftragsverarbeitung zwischen dem Verantwortlichen (Schule/Schulträger) und moto und konkretisiert die dort erteilte allgemeine schriftliche Genehmigung im Sinne des Art. 28 Abs. 2 Satz 1 DSGVO.

moto legt jedem Subunternehmer durch Vertrag dieselben Datenschutzpflichten auf, die im Auftragsverarbeitungsvertrag zwischen dem Verantwortlichen und moto vereinbart sind (Art. 28 Abs. 4 DSGVO). Kommt ein Subunternehmer seinen Datenschutzpflichten nicht nach, haftet moto gegenüber dem Verantwortlichen für die Einhaltung der Pflichten dieses Subunternehmers.

Die Beauftragung von Subunternehmern erfolgt im Einklang mit § 2 Abs. 3 VO-DV I NRW: Die Verarbeitung durch Subunternehmer erfolgt ausschließlich weisungsgebunden und zweckgebunden für die jeweilige Schule.

## 2. Grundsatz der Datenhaltung

Sämtliche produktiven Nutzdaten (Stammdaten von Schülerinnen und Schülern, Erziehungsberechtigten und Personal, Anwesenheits- und Aufenthaltsdaten, Einwilligungen, Protokolldaten) werden ausschließlich in der PostgreSQL-Datenbank auf Servern der Hetzner Online GmbH am Standort Nürnberg (Deutschland) gespeichert. Kein anderer in dieser Liste genannter Dienstleister erhält dauerhaften Zugriff auf diese Datenbank.

Die übrigen Subunternehmer sind vorgelagerte Infrastruktur- bzw. Werkzeugdienste mit deutlich reduziertem oder ohne Personenbezug. Diese Abgrenzung ist in der Übersichtstabelle (Spalte "Datenkategorien") und in den Einzeldarstellungen (Abschnitt 4) ausgewiesen.

## 3. Übersichtstabelle

| Nr. | Anbieter | Sitz | Zweck der Verarbeitung | Datenkategorien | Drittlandtransfer / Transfergarantie | AVV/DPA des Anbieters |
|---|---|---|---|---|---|---|
| 1 | Hetzner Online GmbH | Industriestraße 25, 91710 Gunzenhausen, Deutschland; Rechenzentrum: Nürnberg, Deutschland | Hosting der gesamten Systeminfrastruktur (Backend, Frontend, Datenbank, selbst gehostetes Monitoring), Datenbank-Backups | Alle im System verarbeiteten personenbezogenen Daten, einschließlich Gesundheitsangaben in Freitextfeldern (Art. 9 DSGVO), Anwesenheits- und Bewegungsdaten, Zugangsdaten (nur als Hashes) | Nein. Verarbeitung ausschließlich in deutschen Rechenzentren. [PRÜFEN: vertragliche Beschränkung auf deutsche Standorte im Hetzner-AVV dokumentieren] | https://www.hetzner.com/AV/DPA_de.pdf (Abschluss über das Hetzner-Kundenportal) |
| 2 | Cloudflare, Inc. bzw. Cloudflare Germany GmbH [PRÜFEN: konkrete Vertragspartei laut Cloudflare-DPA] | Cloudflare, Inc., 101 Townsend St, San Francisco, CA 94107, USA; EU-Gesellschaft: Cloudflare Germany GmbH, [ADRESSE CLOUDFLARE GERMANY] | DNS-Auflösung, Content Delivery, DDoS-Schutz (dem Hosting vorgelagert); zusätzlich Bot-Schutz (Turnstile-Captcha) für das öffentliche Anmeldeformular, sofern je Schule aktiviert | Verbindungs- und Metadaten aller Portalnutzer (IP-Adressen, HTTP-Metadaten); bei aktiviertem Turnstile zusätzlich IP-Adresse und Browsersignale der Erziehungsberechtigten. Keine dauerhafte Speicherung von Inhaltsdaten | Ja, USA. Transfergarantie: Angemessenheitsbeschluss (EU) 2023/1795 (EU-US Data Privacy Framework), DPF-Zertifizierung von Cloudflare [PRÜFEN: Zertifizierungsstatus unter dataprivacyframework.gov, Prüfdatum eintragen]; hilfsweise Standardvertragsklauseln gemäß Durchführungsbeschluss (EU) 2021/914, im Cloudflare-DPA enthalten. Risikohinweis siehe Abschnitt 5 | https://www.cloudflare.com/cloudflare-customer-dpa/ (Version 6.4, April 2026); Subprozessoren: https://www.cloudflare.com/gdpr/subprocessors/ |
| 3 | GitHub, Inc. bzw. GitHub B.V. (Microsoft-Konzern) | GitHub, Inc., 88 Colin P Kelly Jr St, San Francisco, CA 94107, USA | Quellcodeverwaltung, CI/CD-Pipeline (Build und Deployment), Container-Registry (GHCR), Verwaltung verschlüsselter Deploy-Zugangsdaten | Keine personenbezogenen Daten von Schülerinnen und Schülern, Erziehungsberechtigten oder Schulpersonal. Verarbeitet werden ausschließlich Entwicklungsartefakte (Quellcode, Container-Images, Build-Protokolle) sowie Daten der Entwickler von moto | Ja, USA. Transfergarantie: Angemessenheitsbeschluss (EU) 2023/1795 (EU-US Data Privacy Framework), DPF-Zertifizierung von GitHub [PRÜFEN: Zertifizierungsstatus unter dataprivacyframework.gov, Prüfdatum eintragen]. Risikohinweis siehe Abschnitt 5 | https://github.com/customer-terms/github-data-protection-agreement (Fassung Oktober 2025); Subprozessoren: https://docs.github.com/en/site-policy/privacy-policies/github-subprocessors |
| 4 | [NAME SMTP-ANBIETER] [PRÜFEN: Anbieter, Sitz und Serverstandort klären] | [SITZ SMTP-ANBIETER] | Versand von System-E-Mails (Einladungen, Passwort-Zurücksetzung, MFA-Codes, Anmeldebestätigungen, Benachrichtigungen) | E-Mail-Adressen, Namen in der Anrede, Einladungs- und Zurücksetzungs-Token; in Anmeldebestätigungen ggf. der Name des Kindes | [PRÜFEN: abhängig vom Anbieter und Serverstandort] | [PRÜFEN: AVV/DPA des Anbieters referenzieren] |
| 5 | Functional Software, Inc. (Sentry) [PRÜFEN: produktiver Einsatz bestätigen] | 45 Fremont Street, San Francisco, CA 94105, USA; Datenhaltung laut Konfiguration: EU-Region | Fehler- und Ausnahmeprotokollierung (Error-Tracking) für Backend und Frontend | Fehlermeldungen, Stacktraces, technische Request-Metadaten. Vor Übermittlung werden Authentifizierungs-Header, Cookies, IP-Adressen, E-Mail-Adressen und Benutzernamen technisch entfernt (Scrubbing im Quellcode implementiert) | Datenhaltung in der EU-Region von Sentry vorgesehen [PRÜFEN: tatsächliche Projekt-Region im Sentry-Konto verifizieren]. Bei EU-Region kein regelmäßiger Drittlandtransfer der Ereignisdaten; Restrisiko Konzernzugriff USA, hilfsweise DPF/SCC laut Sentry-DPA [PRÜFEN: aktuelle Fassung sichten] | https://sentry.io/legal/dpa/ [PRÜFEN: Link und Fassung verifizieren] |
| 6 | PostHog, Inc. [PRÜFEN: produktiver Einsatz und produktiv konfigurierter Host bestätigen] | [ADRESSE POSTHOG, USA]; Datenhaltung laut Konfiguration: EU Cloud (eu.i.posthog.com) | Produkt- und Nutzungsanalyse des Web-Frontends (nur bei gesetztem Analyse-Schlüssel aktiv) | Nutzungsereignisse und Interaktionsdaten angemeldeter Nutzer (Personal, ggf. Eltern-Portal). [PRÜFEN: Event-Inhalte auditieren, insbesondere ob Namen oder Kennungen von Kindern in Ereignis-Eigenschaften gelangen] | Datenhaltung in der EU-Cloud-Instanz vorgesehen [PRÜFEN: produktiven Konfigurationswert bestätigen]. Bei EU-Cloud kein regelmäßiger Drittlandtransfer; Restrisiko Konzernzugriff USA, hilfsweise DPF/SCC laut PostHog-DPA [PRÜFEN: aktuelle Fassung sichten] | https://posthog.com/dpa [PRÜFEN: Link und Fassung verifizieren] |

## 4. Einzeldarstellung

### 4.1 Hetzner Online GmbH (Hosting)

Hetzner betreibt die für moto genutzte Serverinfrastruktur ausschließlich im Rechenzentrum Nürnberg (Deutschland). Auf dieser Infrastruktur laufen sämtliche Systemkomponenten: das Go-Backend, das Next.js-Frontend, die PostgreSQL-Datenbank (mit Transportverschlüsselung, Row Level Security und mandantengetrennter Datenhaltung je Schule), der Reverse Proxy (Caddy) sowie das selbst betriebene Monitoring (Grafana, Loki). Ein Drittlandtransfer findet insoweit nicht statt.

Hetzner erhält keinen inhaltlichen Zugriff auf die verarbeiteten Daten; die Verarbeitung beschränkt sich auf die Bereitstellung und den Betrieb der Infrastruktur. Datenbank-Backups verbleiben auf derselben Infrastruktur. Der Abschluss der Auftragsverarbeitungsvereinbarung erfolgt über das Hetzner-Kundenportal; die im Anhang des Hetzner-AVV benannten eigenen Subunternehmer von Hetzner (Konzerngesellschaften, Supportdienstleister) sind bei der jährlichen Überprüfung dieser Liste mitzusichten. [PRÜFEN: Datum der letzten Sichtung des Hetzner-AVV inkl. Subunternehmeranhang eintragen]

Hinweis zum Monitoring: Grafana, Loki und die zugehörigen Komponenten werden von moto selbst auf der Hetzner-Infrastruktur betrieben und begründen keinen weiteren Subunternehmer. In den Zugriffsprotokollen des Reverse Proxy werden IP-Adressen maskiert; Authentifizierungsdaten (Authorization-Header, Cookies, Geräteschlüssel, Personal-PINs) werden vor der Protokollierung entfernt.

### 4.2 Cloudflare (DNS, CDN, DDoS-Schutz, Turnstile)

Cloudflare ist der Hetzner-Infrastruktur als Netzwerkdienst vorgelagert und erbringt DNS-Auflösung, Content Delivery und Schutz vor Überlastungsangriffen. Der Web-Verkehr aller drei Portale (Schule/Träger, Betreiber, Eltern) wird über das Cloudflare-Netz geleitet. Cloudflare verarbeitet dabei Verbindungsmetadaten, insbesondere IP-Adressen und HTTP-Metadaten der Portalnutzer. Eine dauerhafte Speicherung von Inhaltsdaten bei Cloudflare erfolgt nicht.

Zusätzlich wird der Cloudflare-Dienst Turnstile als Bot-Schutz für das öffentliche Online-Anmeldeformular eingesetzt. Turnstile ist standardmäßig deaktiviert und wird je Schule einzeln aktiviert. Bei aktiviertem Turnstile wird das Prüf-Skript direkt im Browser der Erziehungsberechtigten von Cloudflare geladen; Cloudflare erhält in diesem Fall die IP-Adresse und technische Browsersignale der ausfüllenden Person. Zusätzlich übermittelt das Backend zur Verifikation die IP-Adresse an den Cloudflare-Prüfdienst.

Drittlandtransfer: Cloudflare, Inc. ist ein Unternehmen mit Sitz in den USA. Rechtsgrundlage des Transfers ist der Angemessenheitsbeschluss (EU) 2023/1795 der EU-Kommission (EU-US Data Privacy Framework) in Verbindung mit der DPF-Zertifizierung von Cloudflare; hilfsweise gelten die im Cloudflare-DPA enthaltenen Standardvertragsklauseln gemäß Durchführungsbeschluss (EU) 2021/914. Zum Risikohinweis bezüglich des Fortbestands des Angemessenheitsbeschlusses siehe Abschnitt 5.

[PRÜFEN: Cloudflare bietet mit der Data Localization Suite (Regional Services, Customer Metadata Boundary) die Möglichkeit, TLS-Terminierung und Metadatenspeicherung auf die EU zu beschränken. Ob diese Beschränkung für die moto-Konfiguration aktiviert ist, ist zu verifizieren. Ohne aktivierte Datenlokalisierung ist von einer Verarbeitung mit potenziellem US-Bezug auszugehen; diese Liste geht bis zur Verifikation vom Standardfall ohne Datenlokalisierung aus.]

[PRÜFEN: Konkrete Vertragspartei (Cloudflare, Inc. oder Cloudflare Germany GmbH) laut abgeschlossenem Vertrag eintragen.]

### 4.3 GitHub (Quellcodeverwaltung, CI/CD, Container-Registry)

GitHub wird ausschließlich als Entwicklungs- und Auslieferungswerkzeug genutzt: Quellcodeverwaltung, automatisierte Build- und Deploy-Prozesse sowie die Bereitstellung der Container-Images über die GitHub Container Registry (GHCR). Zugangsdaten für die Zielserver liegen ausschließlich verschlüsselt vor.

GitHub hat zu keinem Zeitpunkt Zugriff auf produktive personenbezogene Daten von Schülerinnen und Schülern, Erziehungsberechtigten oder Schulpersonal. Die Produktivdatenbank ist von GitHub aus nicht erreichbar. Betroffen sind ausschließlich Entwicklungsartefakte sowie personenbezogene Daten der moto-Entwickler (Accounts, Commit-Metadaten). Diese Kategorisierung ist für die Risikobewertung maßgeblich: GitHub ist ein Werkzeugdienstleister ohne Klardatenzugriff. Gemäß § 6 Abs. 4 des Auftragsverarbeitungsvertrags gilt GitHub damit nicht als Unterauftragsverhältnis im Sinne des Art. 28 Abs. 4 DSGVO und wird in dieser Liste nachrichtlich als Dienst ohne Zugriff auf personenbezogene Daten der betroffenen Personen geführt.

Drittlandtransfer: GitHub, Inc. gehört zum Microsoft-Konzern (USA). Rechtsgrundlage des Transfers ist der Angemessenheitsbeschluss (EU) 2023/1795 (EU-US Data Privacy Framework) in Verbindung mit der DPF-Zertifizierung von GitHub. Zum Risikohinweis siehe Abschnitt 5. [PRÜFEN: DPF-Zertifizierungsstatus von GitHub unter dataprivacyframework.gov mit Prüfdatum dokumentieren.]

### 4.4 SMTP-Dienstleister (E-Mail-Versand)

Das System versendet Transaktions-E-Mails (Portal-Einladungen, Passwort-Zurücksetzung, MFA-Codes, Bestätigungen und Statusinformationen zur Online-Anmeldung, interne Benachrichtigungen). Übermittelt werden dabei E-Mail-Adressen, Namen in der Anrede, zeitlich befristete Token sowie in Anmeldebestätigungen ggf. der Name des angemeldeten Kindes.

[PRÜFEN: Der konkrete SMTP-Anbieter ist zu benennen. Die Zugangsdaten liegen ausschließlich verschlüsselt in der Deployment-Konfiguration vor; Anbietername, Sitz, Serverstandort und AVV sind zu ermitteln und hier einzutragen. Bis zur Klärung kann keine Aussage zum Drittlandtransfer getroffen werden.]

### 4.5 Sentry (Error-Tracking)

Sentry dient der Fehler- und Ausnahmeprotokollierung in Backend und Frontend. Vor der Übermittlung an Sentry werden im Quellcode nachweisbar Authentifizierungs-Header und Cookies entfernt sowie IP-Adresse, E-Mail-Adresse und Benutzername aus den Ereignisdaten gelöscht. Übermittelt werden damit im Regelbetrieb technische Fehlerdaten ohne direkte Identifikatoren; ein Restrisiko personenbezogener Inhalte in Fehlermeldungstexten verbleibt.

Die Konfiguration verweist auf die EU-Region von Sentry. [PRÜFEN: Die tatsächliche Region des produktiven Sentry-Projekts ist im Sentry-Konto zu verifizieren, da die Regionswahl beim Anbieter erfolgt und nicht technisch im Quellcode erzwungen ist. Ebenso ist zu bestätigen, ob Sentry produktiv überhaupt aktiviert ist; die Anbindung ist optional und nur bei gesetzter Konfiguration aktiv. Ist Sentry nicht produktiv im Einsatz, ist dieser Eintrag zu streichen.]

### 4.6 PostHog (Produktanalyse)

PostHog dient der Analyse der Nutzung des Web-Frontends und ist nur aktiv, wenn der zugehörige Analyse-Schlüssel in der Konfiguration gesetzt ist. Die Konfiguration verweist auf die EU-Cloud-Instanz (eu.i.posthog.com).

[PRÜFEN: Der produktiv konfigurierte Host ist zu bestätigen (der Wert ist überschreibbar). Zusätzlich ist zu auditieren, welche Ereignisse und Eigenschaften konkret erfasst werden, insbesondere ob Namen oder Kennungen von Kindern in Ereignis-Eigenschaften gelangen können. Ist PostHog nicht produktiv im Einsatz, ist dieser Eintrag zu streichen.]

## 5. Risikohinweis zum EU-US Data Privacy Framework

Die Drittlandtransfers zu Cloudflare und GitHub stützen sich primär auf den Angemessenheitsbeschluss (EU) 2023/1795 der EU-Kommission (EU-US Data Privacy Framework, DPF). Hierzu besteht derzeit erhöhte Rechtsunsicherheit:

1. Der US Supreme Court hat am 29.06.2026 in der Sache Trump v. Slaughter entschieden, dass die gesetzlich verankerte Unabhängigkeit der US Federal Trade Commission (FTC) mit der US-Verfassung nicht vereinbar ist. Die Unabhängigkeit der FTC ist eine der tragenden Annahmen des Angemessenheitsbeschlusses.
2. Gegen den Angemessenheitsbeschluss ist beim Gerichtshof der Europäischen Union ein Rechtsmittel anhängig (Rechtssache C-703/25 P).

Der Angemessenheitsbeschluss gilt formal fort, bis die EU-Kommission ihn zurücknimmt oder der EuGH ihn für nichtig erklärt. Eine Berufung auf das DPF ist daher gegenwärtig weiterhin zulässig. moto trifft folgende Vorkehrungen:

- [NAME DATENSCHUTZBEAUFTRAGTER] beobachtet die Rechtsentwicklung fortlaufend und prüft den DPF-Zertifizierungsstatus der betroffenen Anbieter mindestens jährlich sowie anlassbezogen unter dataprivacyframework.gov.
- Als Rückfallebene werden die Standardvertragsklauseln gemäß Durchführungsbeschluss (EU) 2021/914 vorgehalten. Bei Cloudflare sind diese Bestandteil des DPA. [PRÜFEN: SCC-Rückfallmechanismus im GitHub Data Protection Agreement verifizieren.]
- Die betroffenen Dienste verarbeiten keine bzw. nur minimale personenbezogene Daten der betroffenen Kinder (GitHub: keine; Cloudflare: Verbindungsmetadaten ohne dauerhafte Inhaltsspeicherung). Die Nutzdaten verbleiben vollständig in Deutschland (Abschnitt 2).

Entfällt der Angemessenheitsbeschluss, informiert moto den Verantwortlichen unverzüglich über die dann maßgebliche Transfergrundlage.

## 6. Verfahren bei Änderungen (Art. 28 Abs. 2 Satz 2 DSGVO)

1. moto informiert den Verantwortlichen in Textform (E-Mail an die in Anlage 4 zum Auftragsverarbeitungsvertrag benannte Kontaktstelle) über jede beabsichtigte Hinzufügung oder Ersetzung eines Subunternehmers.
2. Die Information erfolgt mindestens 30 Kalendertage vor der geplanten Produktivsetzung. Diese Frist entspricht den Ankündigungsfristen, die die eingesetzten Vorlieferanten (Cloudflare, GitHub) ihrerseits in ihren Subprozessorenlisten anwenden, sodass moto Ankündigungen der eigenen Subunternehmer fristgerecht weiterreichen kann.
3. Der Verantwortliche kann der Änderung innerhalb der Frist aus wichtigem datenschutzrechtlichem Grund in Textform widersprechen. Im Fall eines Widerspruchs stimmen die Parteien eine zumutbare Lösung ab; kommt eine solche nicht zustande, ist der Verantwortliche zur außerordentlichen Kündigung des Hauptvertrags berechtigt, soweit die Leistung ohne den betreffenden Subunternehmer nicht erbracht werden kann (§ 6 Abs. 2 des Auftragsverarbeitungsvertrags).
4. Erfolgt innerhalb der Frist kein Widerspruch, gilt die Änderung als genehmigt.
5. Diese Liste wird bei jeder Änderung mit neuer Versionsnummer und neuem Stand fortgeschrieben. Die jeweils aktuelle Fassung wird dem Verantwortlichen zur Verfügung gestellt. [PRÜFEN: Bereitstellungsweg festlegen, z. B. Kundenbereich oder Versand per E-Mail.]

## 7. Anlagenverzeichnis

| Anlage | Dokument | Fundstelle | Letzte Sichtung |
|---|---|---|---|
| A | Hetzner Auftragsverarbeitungsvereinbarung | https://www.hetzner.com/AV/DPA_de.pdf | [DATUM DER SICHTUNG] |
| B | Cloudflare Data Processing Addendum, Version 6.4 (April 2026) | https://www.cloudflare.com/cloudflare-customer-dpa/ | [DATUM DER SICHTUNG] |
| C | GitHub Data Protection Agreement (Fassung Oktober 2025) | https://github.com/customer-terms/github-data-protection-agreement | [DATUM DER SICHTUNG] |
| D | AVV/DPA des SMTP-Anbieters | [PRÜFEN: nach Klärung des Anbieters ergänzen] | [DATUM DER SICHTUNG] |
| E | Sentry Data Processing Addendum | https://sentry.io/legal/dpa/ [PRÜFEN: Link verifizieren] | [DATUM DER SICHTUNG] |
| F | PostHog Data Processing Agreement | https://posthog.com/dpa [PRÜFEN: Link verifizieren] | [DATUM DER SICHTUNG] |

---

*Erstellt durch: [NAME DATENSCHUTZKOORDINATOR], moto, [RECHTSFORM UND ADRESSE]*
*Freigabe durch: [NAME GESCHÄFTSFÜHRUNG] (ausstehend, Status: Entwurf zur internen Prüfung)*
