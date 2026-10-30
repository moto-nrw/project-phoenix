# Usage analytics: PostHog project and privacy text

## Automatisierter Localhost-Nachweis (3843-c)

`devbox run -- sh -c 'cd frontend && pnpm run test:network'` startet eine
getrennte Playwright-Suite mit dem echten Next.js-Frontend und synthetischer
API auf localhost. Docker, Seed-Daten und echte Konten sind nicht nötig.
Voraussetzungen: installierte Devbox-Pakete, `pnpm install --frozen-lockfile`
in `frontend`, installiertes Playwright-Chromium und freie Ports 13843/18443 (änderbar mit
`NETWORK_E2E_PORT`/`NETWORK_E2E_API_PORT`, siehe
[Synthetic network E2E suites](../agents/operations.md#synthetic-network-e2e-suites)).
Bei fehlendem Browser einmal `cd frontend && pnpm exec playwright install chromium`
ausführen. Die Browserinstallation ist Vorbereitung, nicht Teil des Nachweises.

Die Bereitschaftsprüfung läuft in Node über
`http://127.0.0.1:<Frontend-Port>/api/health` (Standard 13843), nicht über einen Portalhost. Sie benötigt
keine systemweite Auflösung von `*.localhost`. Vor den Tests wird zusätzlich
HTTP 200 ohne Weiterleitung geprüft. Chromium löst die Portalhosts selbst
als Loopback-Adressen auf.

Der härtere lokale Wiederholungslauf entspricht dem CI-Schritt:

```sh
cd frontend
NODE_OPTIONS="--require ./e2e/network/no-subdomain-dns.cjs" pnpm run test:network
```

Der Node-Resolver weist dabei jede `*.localhost`-Auflösung mit `ENOTFOUND`
zurück, sowohl über Callback- als auch Promise-API. `/etc/hosts` bleibt
unverändert. Der Next-Unterprozess behält seine eigene Egress-Sperre;
die Bereitschaftsprüfung des Playwright-Elternprozesses muss unabhängig
davon funktionieren.

Die Suite prüft zwölf Produktfälle in frischen Browserkontexten und eine
zusätzliche Kontrolle der Netzwerksperre:

- OGS (`proof-off.localhost`/`proof-on.localhost`), Elternportal
  (`eltern.localhost`) und Schulportal (`schule.localhost`), jeweils Analyse
  aus/an, vor Anmeldung und nach dem echten Auth.js-Sitzungsübergang.
  Geschützte Prüfseiten: OGS-Profil, Eltern-Einstellungen und
  Schul-Einstellungen. Der API-Stub akzeptiert ausschließlich synthetische
  Testabläufe; er prüft keine echten Berechtigungen oder Zugangsdaten.
  Vor der Anmeldung sendet seit #4901 kein Portal, auch bei „an“. Nach der
  Anmeldung antwortet der Stub bei „an“ mit gespeicherter Einwilligung.
- OGS (`proof-on.localhost`) mit unentschiedenem Konto (#4901): Nach der
  Anmeldung erscheint der Dialog „Nutzungsanalyse erlauben?“. Bis zur Antwort
  gibt es keinen `/ingest`-Versuch. Nach `Erlauben` folgt einer, nach
  `Ablehnen` keiner, auch nicht nach einem neuen Seitenaufruf. Das OGS-Profil
  zeigt danach den gespeicherten Stand des Schalters.
- Öffentliche Online-Anmeldung (`/anmeldung/1`), alle vier Kombinationen
  Analyse aus/an und Captcha aus/an. Ohne Konto gibt es keine Einwilligung:
  seit #4901 ist `/ingest` dort in allen vier Fällen ein Verstoß. Bei ausgeschaltetem Captcha bleibt der
  öffentliche Test-Site-Key absichtlich gesetzt. Das prüft den historischen
  Fehler mit widersprüchlicher Konfiguration.
- Im OGS-Profil wird ein im Browser erzeugtes PNG über die echte Fotoauswahl,
  `compressAvatar` und den Multipart-Upload verarbeitet. Ein Upload muss
  stattfinden, ohne eine jsdelivr-Anfrage auszulösen.
- Die zusätzliche Kontrolle startet einen Dedicated Worker in einer lokalen
  HTML-Testantwort ohne CSP. Dessen jsdelivr-Import muss durch Playwright
  abgebrochen und als Allowlist-Verstoß erkannt werden. Diese bewusst
  abgefangene Anfrage gehört nicht zu den Produktfällen.

### Was die Aufzeichnung erlaubt und blockiert

Erlaubte Origins sind pro Fall die exakte eigene Localhost-Origin,
die konfigurierte Sentry-Platzhalter-Origin und, nur bei eingeschaltetem
Captcha, `https://challenges.cloudflare.com`. Der lokale PostHog-Pfad
`/ingest` ist nur bei eingeschalteter Analyse zulässig. Jede andere Origin,
einschließlich weiterer Localhost-Ports, lässt den Test fehlschlagen.

**Kein externer Dienst wird kontaktiert.** Playwright bricht alle fremden
Origins ab, auch die erlaubten. Ebenso werden `/ingest` und der Sentry-Tunnel
`/monitoring` vor der Weiterleitung durch Next.js abgebrochen. Der
Next.js-Prozess blockiert zusätzlich nicht lokale HTTP-/Fetch-Ziele.
Service Worker sind deaktiviert; WebSockets dürfen nur die eigene Origin
erreichen. Chromium löst fremde Hosts außerdem nicht auf.

Die Suite überschreibt Dienstkonfigurationen mit nicht funktionsfähigen
Platzhaltern. Sitzungs- und Metrikgeheimnisse entstehen zufällig pro Lauf und
werden nicht ausgegeben. Positive Kontrollen erwarten bei Analyse an einen
abgefangenen `/ingest`-Versuch, bei Captcha an den abgefangenen Skriptabruf
und für konfigurierte Fehlerdiagnose nach einem synthetischen Browserfehler
einen abgefangenen `/monitoring`-Versuch. Geladene Formularfelder bzw.
geschützte Seiten dienen als positive Anker für die Aus-Fälle.

`frontend/test-results/network/**/network*.json` enthält ausschließlich
Origin, Pfad ohne Query/Fragment und Inhaltsart (MIME-Typ, sonst
Playwright-Ressourcentyp). Keine Bodies, Headerwerte außer dem MIME-Typ,
Cookies, Tokens, Namen oder Formulareingaben werden gespeichert.
Screenshots, Videos, Traces und automatische DOM-Fehlerabbilder sind aus.
Für die Portale gibt es getrennte Dateien vor der Anmeldung und für den
Sitzungsübergang samt geschützter Seite sowie eine gemeinsame Aufzeichnung.
**PR-CI, nicht nur lokal:** Der separate Workflow **Frontend E2E**
(`.github/workflows/e2e.yml`) führt im Job `e2e` (**Playwright E2E flows**)
den Schritt **Run portal network evidence** mit `pnpm run test:network` aus.
Dieser Schritt verwendet den oben beschriebenen gesperrten Node-Resolver.
Danach folgt **Upload sanitized network evidence**, anschließend
**Run E2E flows** für die bisherigen geseedeten Tests. Die Netzwerksuite
gehört nicht zum Job `lint / frontend-build` des Hauptworkflows.

Der E2E-Workflow ist kein Required Check. Er läuft auf PRs gegen
`development` bei Änderungen an E2E-Dateien, den Playwright-Konfigurationen,
Frontend-Abhängigkeiten, Seed-Zugang oder dem Workflow selbst; zusätzlich
werktags geplant und per `workflow_dispatch`. PR #4862 trifft diese Pfadfilter.

Bereits am Head `9876f3baa147b45dab76b16a4b66de9e2e81b2e7` wurde die
CI-Ausführung bestätigt, nicht nur aus der YAML abgeleitet:
[Joblog, Run 40019687870](https://github.com/moto-nrw/project-phoenix/actions/runs/40019687870/job/120640704783).
Der Log zeigt am 23.10.2026 um 20:32:43 UTC `pnpm run test:network`,
danach `Running 11 tests using 1 worker` und um 20:34:05 UTC
`11 passed (1.4m)`. Auch Artefakt-Upload und nachfolgender E2E-Schritt waren
erfolgreich. Dieser historische Lauf bestätigt noch nicht den späteren
Resolver-Fix; dessen CI-Ergebnis wird nach dem Push separat geprüft.

Happy-DOM-Komponententests laden zusätzlich keine iframe-Seiten,
Stylesheets, externen Skripte oder Bilder aus gerendertem Markup mehr.
Native Navigationen im Hauptfenster, in Unterframes und in neuen Fenstern
öffnen ebenfalls keine HTTP-Verbindungen. Die bisherige reine
Ressourcensperre deckte neue Fenster nicht ab; ein Link mit `target="_blank"`
konnte noch Verbindungen zu `localhost:3000` und damit `ECONNREFUSED` auslösen.
`src/test/resource-isolation.test.ts` schützt diesen Vertrag. Das ersetzt
keine Mocks für explizite Fetch-Aufrufe in Komponententests.
Die ergänzende Prüfung fand solche fehlenden Mocks im Test der
Kind-Detailseite: Familienschutz, Notizen und Notiz-Angebote liefen gegen
`localhost:3000`, teilweise nach Testende. Die drei Lese-APIs sind dort nun
gemockt; die Assertions bleiben unverändert. Der gezielte Lauf mit 95 Tests
zeigte danach keine `ECONNREFUSED`-Diagnosen mehr. Die isolierten 14 Tests
von `external-caregiver-entry.test.tsx` liefen ebenfalls ohne diesen Fehler.

### Aussagegrenzen und Mensch-Schritte

Dieser Code-Nachweis deckt AK1/AK7 von #3843 nur für die beschriebenen
Browserpfade auf localhost ab. Die BFF-/Backend-API und die
Portal-Analysefreigabe sind synthetisch; ihre fachliche Berechnung wird von
den bestehenden Policy-/Integrationstests geprüft. Die Suite bestätigt weder
Backend-Egress noch echte Anmeldung, RLS oder die serverseitige
Turnstile-Verifikation. Abgebrochene Vendor-Aufrufe belegen Ladeversuche,
nicht erfolgreiche Challenges, PostHog-Projekteinstellungen oder
Sitzungsaufzeichnungen. Payload-Inhalte werden bewusst nicht untersucht;
deren Datenminimierung bleibt separat nachzuweisen.

Eine Gegenprobe mit probeweise wieder aktiviertem Kompressions-Worker blieb
ebenfalls ohne CDN-Abruf. Der Netzwerktest prüft tatsächliche Abrufe, nicht
den Optionswert `useWebWorker`. Diesen sichert weiterhin
`frontend/src/lib/image-utils.test.ts`. Die unabhängige Worker-Kontrolle
prüft zusätzlich, dass ein CDN-Import den Mitschnitt und die Sperre erreicht.

**Produktiv nachzuweisen, offen:** Ein berechtigter Mensch prüft nach
Auslieferung dieselben Portale, Konfigurationen und tatsächlichen Empfänger
am ausgelieferten Stand (Slices 3843-f/3843-h2). Dazu gehören Anbieterorte,
PostHog-Projekteinstellungen und die bereinigten übertragenen Inhalte.
AK2/AK3/AK4/AK6 werden durch diese Suite nicht abgeschlossen. Kein Anbieter
entfällt allein aufgrund eines erfolgreichen Localhost-Laufs aus AVV/TOM.

Vorschlag AVV/TOM, keine Änderung im Legal-Repo:

> Der Browser-Code wird auf localhost automatisiert gegen eine explizite
> Origin-Allowlist geprüft. Optionale Analyse und Captcha lösen bei
> ausgeschalteter Testkonfiguration keine entsprechenden Browserabrufe aus;
> die Bildkompression benötigt keinen jsdelivr-Abruf. Der produktive Stand,
> tatsächliche Inhalte, Empfänger und Anbietergrundlagen bleiben gesondert
> zu bestätigen. Dieser Test ist kein Nachweis produktiver Abschaltung.

The code rules of the usage analytics live in `.claude/rules/usage-analytics.md`
and `frontend/src/lib/analytics-policy.ts`. This page lists what lives outside
the repository: the PostHog project settings the code relies on, and the
paragraphs the privacy policy on the website needs (spec #3598, #3603).

## Schulschalter: Implementierung und Nachweis (3843-b)

**Im Code umgesetzt:** `analytics.enabled` ist standardmäßig an. Schuladmins
mit `config:manage` können unter `Einstellungen` → `Datenschutz` →
`Nutzungsanalyse` ausschalten; Operatoren im Schul-Einstellungsformular ebenso.
Der Schalter beendet Nutzungsstatistiken und Sitzungsaufzeichnungen dieser
Schule, nicht die separate Fehlerdiagnose. Eine vorhandene `analytics.freigabe`
bleibt gespeichert, ist bei ausgeschalteter Grundeinstellung aber wirkungslos.
Wiedereinschalten macht sie wieder wirksam. Auch der Operator sieht die
Freigabe nur bei eingeschalteter Grundeinstellung.

Bei „aus“ oder einem Lesefehler starten OGS und öffentliche Anmeldung des
Schulhosts kein PostHog-SDK. Das Schulportal prüft die Schule seiner Sitzung.
Im Elternportal müssen alle aktiven Schulen der Elternperson die Erfassung
erlauben; fehlende Zuordnung oder ein einziger Fehler sperrt sie. Vor Anmeldung
bleiben Eltern- und Schulportal ohne SDK. Demo-Verhalten bleibt unverändert.
Auf dem OGS-Host entscheidet die Schule im Hostnamen, nicht die Schule eines
fremden Anmelde- oder Bearbeitungslinks. Ein eingebetteter Schulkontext darf
nur zusätzlich sperren: Ist dessen Analyse aus oder noch ungeklärt, bleibt die
Erfassung aus. Das gilt auch im Eltern- und Schulportal. Dort bleibt die
Portalentscheidung zusätzlich erforderlich. Eltern-Anmeldeformulare und
Token-Bearbeitungsseiten sperren bereits während des Ladens der Zielschule
und bei Ladefehlern, nicht erst beim Einblenden des Formulars. Bei einer Eltern-Anmeldung muss
also neben allen aktiven Schulzuordnungen auch die Zielschule erlauben.
Anonyme Anmeldeseiten auf dem Elternhost bleiben ohne SDK. Token-Statusseiten
der Anmeldung erfassen außerhalb der Demo auf Eltern- und OGS-Hosts nie,
auch bei angemeldeten Eltern und erlaubenden Schulen. Der Statusvertrag liefert keine Zielschulfreigabe;
die zentrale Policy sperrt deshalb bereits vor dem Laden der Route und bis
zum Verlassen der Statusseite. Das betrifft auch den Sprung nach dem Absenden
(`?submitted=1`). Eine spätere Portal- oder Host-Neulesung hebt die Sperre nicht auf.
Der SDK-Filter prüft die aktuelle Browserroute und die Ereignis-URL, bevor er
ein Ereignis weitergibt. Damit gilt die Sperre auch beim synchronen
History-Update, noch bevor React die Seiteneffekte ausführt, sowie für
Aufzeichnungen ohne eigene Ereignis-URL. Eltern-Anmeldeformulare und
Token-Bearbeitungs-/Anpassungsseiten benötigen eine Freigabe für genau ihre
Route. Solange Routenparameter oder Zielschulmetadaten fehlen, bleibt die
Erfassung gesperrt. Eine Freigabe für einen anderen Token oder eine andere
Anmeldephase reicht nicht. Diese lokalen Routenfreigaben werden nicht übertragen.
Die Oberfläche `public` (Hosts außerhalb Operator-, Eltern-, Schul- und
OGS-Host) erfasst außerhalb der Demo nie. Diese bereits mit PR #4757
wirksame Einschränkung wird hier ausdrücklich nachgetragen.
Backend-Kernaktionen, Presence-Ereignisse und `$groupidentify` prüfen den
Schulschalter vor der Erfassung. Öffentliche und Eltern-Anmeldungen lösen die
Zielschule aus dem Routenparameter auf, nicht aus einer mitgesendeten Sitzung.
`enrollment_submitted` bleibt bei Zielschule „aus“ auch mit `surface=parents`
gesperrt; erlaubende Bestandsschulen ersetzen die Zielschulentscheidung nicht.
Bei erlaubter Eltern-Anmeldung trägt das Ereignis die `school_id` der
Zielschule, nicht die einer Bestandsschule.

Eltern- und Schulportal lesen die Policy beim Sichtbarwerden eines Tabs und
alle fünf Minuten bei sichtbarem Tab neu. Vor jedem Neulesen wird die Erfassung
ausgeschaltet. Lesefehler lassen sie aus; eine überholte Antwort darf sie nicht
wieder einschalten. OGS- und eingebettete Schulkontexte lesen wie bisher bei
Einstellungs- und Sichtbarkeitssignalen neu. Die Neulesung ist keine sofortige
Zustellung einer Änderung an alle Tabs; ohne Signal gilt im sichtbaren Eltern-
oder Schultab das Fünf-Minuten-Intervall. Serverseitige Ereignisse prüfen pro
Erfassung.
Früher erfasste PostHog-Daten werden durch Ausschalten nicht gelöscht.

**Lokal prüfbar:** Das Seed-Profil `manuell` (maßgeblich: `profiles.go`) speichert
`analytics.enabled=false`; andere Schulen behalten den Registry-Standard.
Die Demo-Umgebung ignoriert den Schalter absichtlich, daher für den Nachweis
`APP_ENV=development`, nicht `demo`, verwenden.

1. Lokal migrieren und seeden, keine externen Analyse- oder Diagnoseschlüssel
   verwenden. Für den Browsernachweis darf ein nicht funktionsfähiger
   Test-Platzhalter als `NEXT_PUBLIC_POSTHOG_KEY` dienen. Ausgehenden Verkehr
   zusätzlich auf localhost begrenzen. Keine echten PostHog-Schlüssel benutzen.
2. Schule mit „aus“ im OGS-Portal und ihre öffentliche Anmeldung öffnen.
   Netzwerkaufzeichnung: keine Anfrage an `/ingest`, auch nach Interaktionen.
3. Positive Kontrolle: dieselben Abläufe mit „an“ gegen einen lokalen
   `/ingest`-Stub prüfen; der Stub darf nichts zu PostHog weiterleiten.
4. Backendtests mit Tracker-Stub für Kernaktionen, `student_checked_in/out`,
   `room_transfer` und `$groupidentify` ausführen. Sie prüfen „an“, „aus“,
   Lesefehler, Schule A aus/B an und Eltern mit zwei Schulen, eine aus.
   Tests: `analytics/collection_policy_mock_test.go`, Tenant-Resolve-Tests und
   `lib/analytics-policy.test.ts` / `lib/posthog-client.test.ts`.
5. `components/analytics/portal-analytics-policy.integration.test.tsx` prüft
   echte Policy-, SDK-Client-, Tenant- und Portal-Module gegen ein PostHog-Stub:
   Hostschule A aus, eingebettete Schule B an und B antwortet zuletzt nach
   `visibilitychange`; außerdem A an/B aus und positive Kontrollen.
   Eltern- und Schultabs sperren vor der Neulesung; eine verspätete alte
   Freigabe überschreibt keine neuere Sperre. Eltern-Anmeldungen benötigen
   zusätzlich eine erlaubende Zielschule. Die echten Token-Statusrouten auf
   Eltern- und OGS-Hosts sperren auch bei erlaubender Portal-/Host-Policy,
   während Suspense und nach `visibilitychange`. Beim Verlassen der
   Statusseite ist die erlaubte Erfassung wieder möglich. Dieser Modultest
   ersetzt keinen Netzwerknachweis.
6. Für den lokalen und später menschlich auszuführenden Produktivnachweis
   zusätzlich offene Eltern-/Schultabs, Zielschule aus bei erlaubenden
   Bestandsschulen, fremde Token-Bearbeitungsseite auf einem gesperrten
   OGS-Host, Token-Statusseiten auf Eltern- und OGS-Hosts (auch direkt nach
   Absenden) und `public` außerhalb der Demo prüfen. Statusseiten müssen selbst
   bei erlaubenden Schulen ohne Erfassung bleiben; positive Kontrollen auf den
   benachbarten Formularseiten verwenden; Inhalte und Kennungen nicht als
   unbereinigte HAR-Datei ablegen. Automatisierter Localhost-Lauf: siehe oben,
   Abschnitt 3843-c.

**Lokal geprüft am 23.10.2026:** Nativer Entwicklungsstack, vollständig
synthetische Seed-Daten, Test-Platzhalter statt Analyse-Schlüssel und
abgefangene `/ingest`-Anfragen. Positive Kontrolle mit eingeschalteter Analyse:
Anfragen an den lokalen Stub vorhanden. Gegenprobe: angemeldetes OGS-Portal
von `manuell` und aktives öffentliches Anmeldeformular von `vollbetrieb`
(nach Ausschalten über die Einstellungen), einschließlich Formularinteraktion:
0 `/ingest`-Anfragen. Die öffentliche Gegenprobe verwendete ein frisch
aufgerufenes Formular. Keine PostHog-Anfrage mit echtem Schlüssel.
Schulschalter und getrennte Fehlerdiagnose auf Desktop und Mobilgerät angesehen.
Die automatisierte Wiederholung für alle Portale steht oben (3843-c).

**Produktiv nachzuweisen, offen:** Auslieferung und tatsächlicher Netzwerknachweis
je Portal. Ein lokaler Test bestätigt keine produktive Abschaltung.
Anbieter erst nach bestätigter Abschaltung aus Vertragsunterlagen streichen.
Rechtsgrundlagen, Rollen und § 25 TDDDG bleiben Recht-Slice 3843-e; der folgende
lit.-f-Entwurf bleibt unverändert und ist keine Freigabe.

## Einwilligung der Person (§ 25 TDDDG, #4901)

**Im Code umgesetzt:** Das PostHog-SDK greift auch ohne Cookies auf das Gerät
zu (EDPB-Leitlinien 2/2023, Rn. 33). Es startet deshalb nur, wenn die Schule
die Nutzungsanalyse erlaubt **und** die angemeldete Person eingewilligt hat.
Die Freigabe der Schule ersetzt die Einwilligung nicht. Die Antwort gehört
zum Konto (`auth.accounts.analytics_consent`, Zeitpunkt in
`analytics_consent_decided_at`) und gilt in allen Portalen und Schulen.
`GET`/`PUT /analytics-consent/` bedient jede Portalsitzung; Operator- und
Vorschausitzungen werden abgewiesen.

- Nach dem Login fragt ein Dialog einmalig, solange das Konto nicht
  entschieden hat und die Schule die Analyse erlaubt. Er nennt den Kontobezug
  und im OGS-Portal, dass die Wahl auch für Aufzeichnungen gilt. `Erlauben` und
  `Ablehnen` sind gleich gestaltet, der Fokus liegt auf dem Titel. Schließen
  entscheidet nichts; der Dialog kommt beim nächsten Seitenaufruf wieder.
- Widerruf jederzeit: OGS-Profil, Eltern-Einstellungen und
  Schul-Einstellungen, Karte `Nutzungsanalyse`. Ein Widerruf stoppt Erfassung,
  Aufzeichnung und den Header `X-POSTHOG-SESSION-ID` sofort, noch vor dem
  Speichern; scheitert das Speichern, bleibt diese Seite trotzdem aus. Eine
  Einwilligung wirkt erst nach dem Speichern. Andere offene Tabs lesen die
  Antwort neu, sobald sie wieder sichtbar werden, sonst alle fünf Minuten.
- Ein bereits geladenes SDK bleibt nach Widerruf oder Abmeldung bis zum
  nächsten Seitenaufruf im Speicher. Es ist abgemeldet (`opt_out`), sendet
  nichts und setzt keinen Header mehr.
- Ohne Einwilligung: kein SDK, kein Sitzungs-Header. Das Backend entfernt
  zusätzlich `$session_id` und die pseudonyme Person aus Kernaktionen, deren
  Konto nicht eingewilligt hat; die Kernaktion zählt nur noch für die Schule.
  Das gilt auch für die Sitzungsaufzeichnung mit Analyse-Freigabe.
- Anonyme Seiten (Login, öffentliche Anmeldung, Statusseiten) erfassen
  außerhalb der Demo nichts mehr: Dort gibt es kein Konto, das einwilligen
  kann. Die Trichter der öffentlichen Anmeldung enthalten ab Auslieferung
  keine neuen Browser-Ereignisse; die Kernaktionen des Backends ohne
  Sitzungsbezug bleiben.
- Die öffentliche Demo bleibt unverändert und fragt nicht. Ob die Demo eine
  eigene Einwilligung braucht, ist Teil der Rechtsprüfung (3843-e).

**Lokal geprüft:** Komponenten-, Store-, Richtlinien- und Backend-Tests; die
Netzwerksuite oben mit dem Dialogfall.

**Offen:** Der Datenschutztext unten nennt noch berechtigtes Interesse
(Art. 6 Abs. 1 lit. f DSGVO) und „keine Speicher im Browser“. Für den Zugriff
auf das Gerät ist jetzt die Einwilligung die Grundlage. Den Text passt
Recht-Slice 3843-e an; hier bleibt er unverändert. Produktiver Nachweis je
Portal nach Auslieferung.

## PostHog project settings (EU project)

Session recording starts only when the project allows it (the SDK loads its
remote config through `/ingest`). The client limits recording to the demo and
to the OGS portal of schools with Analyse-Freigabe, and the `before_send`
filter drops every other `$snapshot`; the project switch alone never widens
that.

| Setting | Value | Why |
| --- | --- | --- |
| Session replay › Record user sessions | on | Without it no recording starts at all |
| Session replay › Minimum session duration | 5 seconds | Only sessions of 5 s and longer count against the free quota (#3598). The SDK reads this from the project only; there is no client option |
| Session replay › Sampling | none (100 %) | The client sets the rate: 1 in the demo, `analytics.recording_sample_percent` with Analyse-Freigabe |
| Session replay › Canvas, console logs, network | off | The client turns them off as well; canvas recording is the only part that starts a Blob worker, so the CSP needs no `worker-src blob:` |
| Session replay › Masking | leave the defaults | The client masks per context and wins over the project |
| Discard client IP data | on | No IP, no GeoIP |
| PostHog AI | off | No further subprocessor |

## Dashboards (#3604)

`scripts/posthog-dashboards.mjs` creates and updates the dashboards; how
to add one is in `.claude/rules/usage-analytics.md`. The workflow
`.github/workflows/posthog-dashboards.yml` runs it: `--check` and
`--dry-run` on pull requests, the sync on every merge into development that
touches the script or its inputs. Run it by hand (`workflow_dispatch` on
development) after a failed sync, a new key, or a change in the PostHog UI.

The workflow uses two personal API keys, each used for nothing else and
scoped to project 140838. A pull request runs code from its own branch, so it
gets only the read key; the write key is an environment secret of `posthog`,
which deploys from development only.

| GitHub secret | Where | Scopes |
| --- | --- | --- |
| `POSTHOG_READ_API_KEY` | repository | `query:read`, `insight:read`, `dashboard:read` |
| `POSTHOG_PERSONAL_API_KEY` | environment `posthog` | `query:read`, `insight:read`, `insight:write`, `dashboard:read`, `dashboard:write` |

Set them with `gh secret set POSTHOG_READ_API_KEY` and
`gh secret set POSTHOG_PERSONAL_API_KEY --env posthog`; both read the value
from the prompt. A local key for `--check` and `--dry-run` lives only in the
local environment (for example `~/.config/project-phoenix/local.env`).

One dashboard per product surface plus two across schools. Every tile title
is the question it answers. The team guide (which dashboard for which
question) lives in the dashboard descriptions.

| Dashboard | Deployment | Questions | Link |
| --- | --- | --- | --- |
| Nutzung je Standort (home dashboard, pinned) | `moto-app.de` | How many schools use moto, on which days, for which flows, who slows down | <https://eu.posthog.com/project/140838/dashboard/1046428> |
| OGS-App | `moto-app.de` | Most used pages per day with navigation name and time on page, actions per role, where users go from the start page, feature reach, rarely used pages, dead and rage clicks | created by the sync |
| Eltern-App | `moto-app.de` | The same for parents; parents have no school | created by the sync |
| Schul-App | `moto-app.de` | The same for teachers in the school portal | created by the sync |
| Schule im Verlauf | `moto-app.de` | One school over time (`school_id` filter, do not save it) | <https://eu.posthog.com/project/140838/dashboard/1046854> |
| Demo | `demo` | Conversion to „Kostenlos starten“, time in the demo, what visitors look at and try, where they stop, dead and rage clicks | created by the sync |

A school counts as active through use: page views, core actions, and kiosk
bookings (`ACTIVE_USE`), not a failed login or a dead click. Rage clicks come
from PostHog's heatmap data (`heatmaps` table): autocapture is off in the
project, so there is no `$rageclick` event. Page names come from
`frontend/src/lib/section-navigation.ts`.

Errors are tracked in Sentry (frontend and backend), not in PostHog.

The sync never deletes. After the merge that introduced the per-surface
dashboards, delete the replaced ones by hand, but not their insights: the
new dashboards reuse those with an unchanged name. Replaced: Aktive Schulen (972372), Funktionen in Schulen (1003811), Nutzung pro
Rolle und Oberfläche (972369), Seiten (972370), Reibung (972371), Demo-Funnel
(972368), Demo-Verhalten (1003812). Then delete the insights no definition
uses any more: Aktive Schulen: letzte 7 Tage; Aktive Schulen pro Woche und
Oberfläche; Funktionen: je Schule (30 Tage); Funktionen: nach Rolle und
Oberfläche (30 Tage); Nutzung: Seitenaufrufe pro Oberfläche und Woche;
Nutzung: Rolle und Oberfläche der letzten 30 Tage; Nutzung: Kernaktionen nach
Oberfläche und Rolle; Seiten: meistbesucht in den letzten 30 Tagen; Seiten:
kaum oder nie genutzt in den letzten 30 Tagen; Reibung: Seiten in Schulen;
Reibung: Seiten in der Demo; Reibung: Dead und Rage Clicks pro Woche in
Schulen; Demo-Funnel: Schritte der letzten 30 Tage; Demo-Verhalten:
Sitzungstiefe.

Paths insights stay out: PostHog's Paths count people. „Wohin geht es von
der Startseite?“ counts page changes (`$prev_pageview_pathname`) instead.

The feature areas (Funktionsbereiche) are `FEATURES` in the script; the test
fails for a route template without an area, so a new page needs one.

The demo funnel starts with the link request (`demo_link_requested` from the
backend). The click on „Demo starten" before it happens on the website,
which measures with Umami, not PostHog. So does the intake form behind
„Kostenlos starten" (`moto-ogs.de/start/?src=demo`): Umami events
`start-form-start`, `start-form-step`, `start-form-submit`, and
`start-form-result`, each with `src`.

## Privacy policy (website, `moto-ogs.de/datenschutz`)

The website lives in `moto-nrw/website`, not in this repository. The demo
notice links to its privacy page. The paragraphs below are a draft for that
page and need legal review before they go live; the Analyse-Freigabe also
needs the school's or Träger's written consent (AVV addendum) before the moto
team switches it on for a school.

> **Nutzungsanalyse in moto**
>
> Damit wir moto verbessern können, werten wir aus, wie moto benutzt wird. Wir
> nutzen dafür PostHog (PostHog Inc.; Verarbeitung in der EU, Rechenzentrum
> Frankfurt) als Auftragsverarbeiter. Wir erfassen, welche Seiten aufgerufen
> und welche Schaltflächen angeklickt werden, wie lange eine Seite offen ist
> und wo Klicks ins Leere gehen. Seitenadressen speichern wir nur als Muster
> ohne Kennungen, Texte auf Schaltflächen nur in der Demo. Wir speichern keine
> IP-Adresse und keinen Standort und setzen dafür weder Cookies noch andere
> Speicher im Browser ein. Die Auswertung ist keinem Menschen zugeordnet und
> dient nicht der Bewertung von Beschäftigten. Rechtsgrundlage ist unser
> berechtigtes Interesse an einer funktionierenden, verständlichen Anwendung
> (Art. 6 Abs. 1 lit. f DSGVO).
>
> **Aufzeichnung der Demo**
>
> In der öffentlichen Demo zeichnen wir Besuche ab einer Dauer von fünf
> Sekunden auf. Die Aufzeichnung zeigt, wie Sie durch die Demo gehen. Ihre
> Eingaben sind darin unkenntlich, und das Formular „Kostenlos starten" wird
> nie aufgezeichnet. Die Demo enthält nur ausgedachte Daten. Aufzeichnungen
> löscht PostHog nach einem Monat.
>
> **Analyse-Freigabe einer Schule**
>
> Hat eine Schule oder ihr Träger schriftlich zugestimmt, zeichnen wir auch im
> OGS-Portal dieser Schule Sitzungen auf. Alle Texte, Eingaben und Bilder sind
> dabei unkenntlich. Wiederkehrende Nutzer erkennen wir an einer Kennung, die
> aus Schule und Benutzerkonto berechnet wird und keinen Namen enthält; zu
> dieser Kennung speichern wir nur die Rolle. Das Personal sieht in seinem
> Profil einen Hinweis, solange die Freigabe gilt. Eltern-Portal und
> Schul-Portal werden nie aufgezeichnet. Rechtsgrundlage ist die Vereinbarung
> mit der Schule bzw. dem Träger.
