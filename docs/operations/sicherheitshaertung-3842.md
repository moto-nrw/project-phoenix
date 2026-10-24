# Sicherheitshärtung #3842: Ergebnis und offene Nachweise

Slice 3842-f im Epic #3846. Stand: 23.10.2026.
Repository-Basis: `origin/development`, Commit `e42cabdc5bd8`.
AK3 ergänzt durch Slice 3842-d am 24.10.2026, Repo-Basis `755ccc706520`.

## Aussagekraft

Dies ist ein Repository-Abgleich der acht Akzeptanzkriterien von
[#3842](https://github.com/moto-nrw/project-phoenix/issues/3842), keine
Bestätigung des produktiven Stands und keine Freigabe der Verarbeitung.
Es gab keine Requests an Produktions-/Staging- oder Monitoring-Hosts,
keinen Serverzugriff und keine Änderung an Umgebungen oder Secrets.
Die Vorarbeiten wurden gelesen, ihre Tests in diesem Doku-Slice nicht erneut
ausgeführt. Ein gemergter PR beweist weder Rollout noch wirksamen Betrieb.

Maßstab sind die Kriterien vom 23.10.2026 und die
[rechtliche Einordnung](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/research/14-dsgvo-mindestumfang.md),
nicht pauschale Technikvorgaben aus dem Audit vom 03.10.2026.
Das Legal-Repo wurde ausschließlich gelesen.

## Stand aller Akzeptanzkriterien

| AK | Im Repository belegt | Offen und erforderlicher Mensch-Nachweis |
|---|---|---|
| **1: Produktions-/Test-/Demodaten und Zugänge trennen** | Getrennte Compose-Dateien, Netze und umgebungsbezogene Log-Tags; Dienst-/Secret-Grenzen in [Runtime environment boundaries](runtime-environment-boundaries.md), Test-DB-Verfahren in [Operations](../agents/operations.md#test-database), [Demo-Betrieb](demo-environment.md). Das ist kein Beleg tatsächlich getrennter Zugangsdaten. | **Offen, Betrieb zu #3842; Rechteanteil 3842-c.** Effektive DBs, Netze, Credentials, Hostrechte und Datenherkunft je Umgebung bestätigen; keine identifizierbaren Produktivdaten in Test/Demo. Isolation beim gemeinsamen Host und erreichbare Monitoring-Brücken bewerten. Für den umfassenden Isolationsnachweis ist im vorliegenden Slice-Auftrag keine eigene Slice-ID benannt; Zuordnung durch den Orchestrator offen. |
| **2: Mandantentrennung und neue Tabellen prüfen** | **Im Code umgesetzt:** [PR #4823](https://github.com/moto-nrw/project-phoenix/pull/4823), Slice **3842-a**: `TestEveryTenantTableHasRLS`, Migration `1.15.476` und Regressionstests für fünf Tabellen. [Nachweis und Grenzen](rls-coverage-3842.md). | **Produktiv offen, 3842-a:** Migration und Policies unter Request-Rolle nachweisen. Wächter prüft Struktur und Policy-Anwendbarkeit, nicht beliebige Policy-Semantik; begründete Ausnahmen und indirekte Schulbezüge bleiben gesondert zu prüfen. |
| **3: DB-Übertragung und Gegenstelle bewerten** | **Repo-Bewertung in 3842-d erstellt:** [DB-Transportschutz und Mensch-Prüfung](db-transportschutz-3842.md). Produktions-App: `require` ohne Gegenstellenprüfung; Staging/Demo: `disable`. Alle drei DBs haben Loopback-Portfreigaben. Frontend und teils Prometheus teilen das DB-Netz. Migration/Demo-DSN bleiben ohne Secret-Zugriff unbekannt; Sicherungen verwenden im Repo lokale Unix-Sockets. | **Produktiv offen, 3842-d/Betrieb:** `verify-full` für Produktions-TCP und externe Diagnose herstellen und negativ testen oder gleichwertigen Same-Host-Schutz anhand effektiver Grenzen nachweisen. Netzteilnehmer, Ports, CA/SAN, tatsächliche Clientpfade einschließlich kurzlebiger Jobs und Vertragsabweichung belegen. Keine Freigabe durch diese Bewertung. |
| **4: Browser-Angriffswege und CSP** | **Teil im Code umgesetzt/bewertet:** [PR #4841](https://github.com/moto-nrw/project-phoenix/pull/4841), Slice **3842-e**: `object-src 'none'` ergänzt; `base-uri 'self'` und `form-action 'self'` bereits vorhanden; `unsafe-eval` bereits nur Entwicklung. [Browser-Bewertung](browser-csp-3842.md). | **Produktiv offen, 3842-e:** ausgelieferte Header einschließlich Caddy/HSTS und Login-/Formularabläufe nachweisen. `unsafe-inline` bleibt ein Inline-XSS-Restrisiko; keine vollständige Nonce-Migration allein wegen eines Entwurfs, aber notwendige Anwendungskorrekturen bleiben erforderlich. |
| **5: Privilegierte Zugriffe und Logausfälle** | Operator-Protokoll mit bestehender Retention ist in der [Fristenliste](../reference/loeschfristen.md) erfasst. Monitoring enthält Fehlerregeln; [Störungsablauf und Erkennungslücken](stoerungen.md) sind dokumentiert. Das beweist keine vollständige Operator-Leseprotokollierung oder Ausfallsperre. | **Offen, 3842-b:** relevante privilegierte Lese-/Schreibpfade und Sicherheitsänderungen abdecken, Integrität/Zugriff und Alarmzustellung prüfen; Logfehler je Risiko behandeln, kritische unprotokollierte Zugriffe gegebenenfalls sperren. Keine pauschale Sperre sämtlicher Aktionen nötig. |
| **6: Deploy-, Diagnose- und Schlüsselrechte** | Dienstbezogene Env-Allowlist begrenzt Containergeheimnisse. `.github/workflows/build.yml` enthält weiterhin SOPS-Entschlüsselung in Actions sowie Deploy-Aufrufe mit `root` für Staging, Produktion und Demo. Das ist ein aktueller Codebefund, kein nur historischer Auditpunkt. | **Offen, 3842-c:** erforderliche Deploy-/Schlüsselrechte, Widerruf und getrennten, beschränkten regulären Diagnosezugang umsetzen bzw. gleichwertigen Ablauf belegen. Docker-/Root-Rechte nicht als read-only einstufen. Tatsächliche Schlüssel und Hostrechte wurden nicht gelesen. |
| **7: Betriebsprotokolle: Fristen, Zugriff, Minimierung** | **Doku in 3842-f erstellt:** [Quellen und Fristvorschläge](../reference/loeschfristen.md#betriebsprotokolle), [lesende Checkliste](betriebsprotokolle-pruefen-3842.md). Caddy-Beispiel enthält bereits Rotation/Filter; Mail-Outbox hat 30-Tage-Löschweg. journald/Loki/Sentry/PostHog-/Actions-Log-Retention nicht im Repo festgelegt. | **Nicht vollständig erfüllt, 3842-f/Betrieb:** Vorschläge begründen und freigeben, effektive Fristen, Rechte, Filter, Kopien und Löschung nachweisen; nötige Konfiguration separat umsetzen. Caddy-Archivrotation allein begrenzt keine aktive Datei; Artefaktfrist ist keine Actions-Logfrist. Vertragsabweichung 90 Tage gegenüber sechs Monaten unter **3842-i** klären. |
| **8: Ergebnisse, Abhilfe und Restrisiken dokumentieren** | **Dokumentiert in 3842-f:** dieser Bericht, Quelleninventar und Mensch-Prüfablauf; Vorarbeiten 3842-a/e verlinkt. Keine neue Infrastruktur als pauschale Pflicht eingeführt. | **Produktiver Abschluss offen, 3842-f/Betrieb:** Nachweise und Abhilfeentscheidungen je AK ergänzen; Bericht nach Abschluss weiterer Slices aktualisieren. Dokumentation ersetzt keine notwendige Maßnahme. |

## Restrisiken und notwendige Abhilfe

1. **Unbelegte Aufbewahrung und Kopien:** Größenlimits verhindern keine lange
   Speicherung bei geringem Volumen. Loki, Exporte oder Anbieter können Daten
   über die Quelle hinaus halten. Vor Abschluss von AK7 Quelle, aktive und
   archivierte Dateien, Spools und Speicherlöschung nach der Checkliste
   belegen. Fristvorschläge sind keine Übergangsfreigabe für unbegrenzte Logs.
2. **Personenbezug und Geheimnisse in Diagnosedaten:** Konto-/Schul-/Geräte-IDs,
   Pfade und freie Fehler-/Providertexte bleiben potenziell zuordenbar.
   Caddys Beispiel entfernt Queries, nicht alle geheimen Pfadsegmente.
   Frontend-Scrubbing belegt nicht automatisch den Inhalt von Backend-, DB-,
   Geräte- oder Anbieterkopien. Datenarme Feldverträge und Filter mit
   synthetischen Proben nachweisen; bestätigte Lecks vor weiterer betroffener
   Verarbeitung beheben und als möglichen Vorfall bewerten.
3. **Privilegien und Nachvollziehbarkeit:** Root-Deployment und unvollständig
   nachgewiesene Operator-/Diagnoseprotokollierung können weitreichenden,
   schlecht nachvollziehbaren Zugriff ermöglichen. 3842-b/c sind notwendige
   Abhilfen, soweit kein gleichwertiger Schutz belegt ist, nicht pauschal
   optionale Folgearbeiten. Bestehende Alarmregeln beweisen keinen Alarm bei
   Ausfall der gesamten Protokollkette.
4. **Isolation und DB-TLS:** Die [Bewertung 3842-d](db-transportschutz-3842.md)
   belegt Loopback-Ports und gemeinsame Netzteilnehmer, keinen gleichwertigen
   Schutz ohne Gegenstellenprüfung. Produktions-TCP benötigt `verify-full`
   oder einen gesonderten Wirksamkeitsnachweis der Same-Host-Alternative.
   3842-c/d und der Betriebsnachweis zu AK1 müssen die tatsächlichen Grenzen
   belegen; ein eigener Server ist nicht allein wegen des Audits Pflicht.
5. **Browser und Telemetrie:** Inline-XSS-Risiko bleibt nach 3842-e; weitere
   Bewertung bei neuen Sinks/Quellen erforderlich. Sentry-/PostHog-Fristen
   legitimieren keine Erfassung. Rechtsgrundlagen, tatsächliche Datenflüsse,
   Anbieter, Aufzeichnungen und Abschaltung bleiben unter #3843 zu klären.

Die Betriebsleitung entscheidet mit Datenschutzrolle und technischer
Verantwortung **vor der betroffenen Verarbeitung** über erforderliche Abhilfe
oder deren wirksame Beschränkung/Unterbrechung. Dieser Bericht erklärt kein
offenes Risiko für akzeptiert. Entscheidungen und Nachweise gehören ohne
Rohdaten in die geschützte Betriebsablage.

## Mensch-Schritte und Abschluss

1. Je AK verantwortliche Rolle, Termin und interne Nachweis-ID festlegen;
   insbesondere den noch nicht zugeordneten Isolationsnachweis aus AK1 klären.
2. Die [lesende Checkliste](betriebsprotokolle-pruefen-3842.md) je Umgebung
   durchführen. Vorschläge freigeben oder begründet ändern. Abhilfe ist ein
   separater Umsetzungsschritt, nicht Ergebnis der Lesekontrolle.
3. Die Produktionsprüfungen aus 3842-a/d/e durchführen; 3842-b/c und den
   Vertragsabgleich 3842-i abschließen. Für bereits laufende Verarbeitung
   erforderliche Schutzmaßnahmen nicht bis zum formalen Issue-Abschluss aufschieben.
4. Tatsächlichen Rollout, Wirksamkeitsbelege, Abweichungen und Entscheidungen
   im Bericht referenzieren. Keine Rohlogs, Personen-/Kundenkennungen, Tokens,
   Secrets oder ungeschützten Konfigurationsexporte in GitHub ablegen.
5. #3842 erst nach vollständigen Kriterien und menschlichen Nachweisen
   abschließen; dieser Slice verwendet deshalb `Refs #3842`, nicht `Closes`.

## Vorschlag AVV/TOM, nicht im Legal-Repo umgesetzt

Gelesene Basis: [AVV Anhang 1 Ziff. 5/6](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/03-avv.md)
und [interne Checkliste](https://github.com/moto-nrw/legal/blob/7a21f6742782bcf6141470a47333ae5be6c3182f/vertragspaket/intern/checkliste.md).

Textvorschlag für Ziff. 6, **erst nach Fristentscheidung und Wirksamkeitsnachweis**:

> Das Protokollierungskonzept unterscheidet fachliche Auditnachweise,
> Anmeldeereignisse und technische Betriebsprotokolle. Für Betriebsprotokolle
> dokumentiert moto je Quelle Zweck, erforderliche Datenfelder, berechtigte
> Rollen, Fristbeginn und Höchstaufbewahrung einschließlich technischer
> Löschverzögerungen und weitergeleiteter Kopien. Die wirksamen Fristen und
> Löschwege sind im Löschkonzept festgehalten. Zugriffe und Exporte sind auf
> erforderliche Aufgaben beschränkt. Eine vorfallbezogene Beweissicherung
> erfolgt getrennt mit dokumentiertem Umfang, Zweck und Löschtermin.

Den Widerspruch „Anmelde- und Sicherheitsprotokolle sechs Monate“ zu
90 Tagen für Anmeldeereignisse separat unter 3842-i auflösen: tatsächliche
Datenarten und Zweck prüfen, danach Vertragswert oder Code begründet
angleichen. Weder die Diagnosevorschläge noch diese Formulierung ändern den
Vertrag oder bestätigen bereits zugesagte TOM.
