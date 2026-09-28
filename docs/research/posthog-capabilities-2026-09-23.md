# PostHog: Möglichkeiten, Grenzen und Folgen für moto (Recherche zu #3578)

Stand: 23.09.2026. Grundlage: Repository-Stand `development` (6713d56a53), installierte SDKs,
Primärquellen von PostHog (Docs, Pricing, Legal, GitHub). Jede externe Aussage ist verlinkt.
Kennzeichnung: **[verifiziert]** = in Quelle oder Code nachgelesen, **[Schluss]** = eigene Ableitung
aus verifizierten Fakten, **[offen]** = nicht verifizierbar.

Rechtliche Bewertung (DSGVO, Kinderdaten, Auftragsverarbeitung mit Schulträgern, TDDDG) ist
**nicht** Gegenstand dieser Recherche. Die Doku-Aussagen von PostHog sind Herstellerangaben,
keine Rechtsauskunft. Offene Fragen für DSB/Juristen stehen am Ende.

## Kurzfazit

1. **Replay ist heute technisch dreifach blockiert, nicht nur durch `disable_session_recording`.**
   (a) `advanced_disable_flags: true` verhindert das Laden der Remote-Config, und das SDK startet Replay
   nur, wenn die Remote-Config `enabled` meldet. (b) Die CSP in `frontend/src/proxy.ts` erlaubt nur
   den `api_host` in `connect-src` und nichts Externes in `script-src`; `recorder.js` und die
   Remote-Config kommen aber von `eu-assets.i.posthog.com`. (c) `before_send` verwirft `$snapshot`.
   **[verifiziert im SDK-Code 1.415.2]**
2. **Ein Reverse Proxy unter eigener Domain (`/ingest` per Next.js-Rewrite) löst das CSP-Problem
   am saubersten** und umgeht Adblocker. Er existiert heute nicht. Die Matcher-Regel von
   `proxy.ts` muss den Pfad dann ausnehmen. **[verifiziert]**
3. **Die Gruppe `school` funktioniert heute nicht.** Laut PostHog werden Events mit
   `$process_person_profile: false` nicht mit Gruppen verknüpft. moto sendet `$groups` immer
   zusammen mit `$process_person_profile: false`. Group Analytics ist außerdem ein kostenpflichtiges
   Add-on. Die Auswertung pro Schule muss also über die Property `school_id` laufen, oder man
   entscheidet sich bewusst für Personenprofile plus Add-on. **[verifiziert]**
4. **Projekte:** Free = 1 Projekt, Pay-as-you-go = 6 Projekte (gleicher Free-Tier, Karte nötig,
   pro Produkt Billing-Limit setzbar), Boost ($250/Monat) = unbegrenzt. Für Demo/Staging/Production
   getrennt reicht also Pay-as-you-go mit Limit 0 USD Mehrkosten. **[verifiziert, Pricing-Seite]**
5. **Free-Tier deckt moto-Volumen voraussichtlich:** 1 Mio. Events, 5.000 Web-Replays, 1 Mio.
   Flag-Requests, 100.000 Exceptions, 1.500 Survey-Antworten pro Monat. Heatmap-Daten kosten nichts
   extra. Replay-Aufbewahrung: Free 1 Monat, Pay-as-you-go 90 Tage. **[verifiziert]**
6. **Vollmaskierung für Schulbildschirme ist per SDK konfigurierbar** (`maskAllInputs: true`,
   `maskTextSelector: "*"`, Bilder per `blockSelector`, Netzwerk-URLs per
   `maskCapturedNetworkRequestFn`, Canvas/Console aus). Masking läuft im Browser, maskierte Daten
   verlassen das Gerät nicht. Restrisiko: Layout, Klickpfade, URLs und Bilder ohne expliziten Block.
   **[verifiziert + Schluss]**
7. **Mit `persistence: "memory"` läuft Replay, aber jede harte Navigation erzeugt neue
   `distinct_id` und neue Session** (SDK-Fallback auf Memory). Client-Navigation in Next.js bleibt
   eine Session. Cookieless-Modus `"always"` schließt Replay komplett aus. **[verifiziert im SDK-Code]**
8. **Backend kann `posthog-go` weiterhin nicht nutzen:** `posthog-go` v1.27.0 hängt weiterhin von
   `hashicorp/golang-lru/v2` (MPL-2.0) ab. `deployment` und `$session_id` lassen sich aber im
   eigenen Tracker (`backend/analytics/analytics.go`) mit wenigen Zeilen ergänzen. **[verifiziert]**
9. **PyrePortal-Kiosk:** Die Geräte-URL enthält `?key=<DEVICE_API_KEY>`. Jede PostHog-Einbindung
   würde den Schlüssel ohne Redaktion in `$current_url` und in Replays schreiben. Kiosk-Analytics
   daher, wenn überhaupt, nur mit URL-Redaktion und ohne Replay. **[verifiziert im Code + Schluss]**
10. **KI-Zugriff:** Offizieller MCP-Server `https://mcp.posthog.com/mcp` (OAuth, EU-Routing,
    `readonly=true`, Projekt-Pinning). Der Preset „MCP Server“ für Personal API Keys vergibt
    Schreibrechte auf fast alle Scopes; für Agenten besser OAuth mit Read-only oder ein eigener
    Key mit `query:read`, `insight:read`, `dashboard:read`, `session_recording:read`. **[verifiziert]**

## Ist-Stand im Code

**Installierte Versionen** **[verifiziert]**

| Paket | Version | Quelle |
|---|---|---|
| `posthog-js` | `^1.415.2`, installiert 1.415.2 (npm aktuell 1.434.11) | `frontend/package.json:58`, `frontend/node_modules/posthog-js/package.json:3` |
| `@posthog/types` (transitiv) | 1.402.2 | `frontend/node_modules/.pnpm/@posthog+types@1.402.2` |
| `posthog-node` | nicht installiert (npm aktuell 5.53.0) | `frontend/package.json` |
| `posthog-go` | **nicht verwendet**, eigener HTTP-Tracker | `backend/go.mod` (kein Eintrag), `backend/analytics/analytics.go:7-10` |
| `next` | `^16.3.3` | `frontend/package.json:55` |
| `@sentry/nextjs` | `^10.70.0` (Error-Tracking läuft bereits über Sentry) | `frontend/package.json:46` |

**Frontend**

- `frontend/src/lib/posthog-client.ts:19-40`: Init-Optionen. `defaults: "2026-01-30"`,
  `autocapture`, `rageclick`, `capture_pageview`, `capture_pageleave`, `capture_performance`,
  `capture_heatmaps`, `capture_dead_clicks`, `capture_exceptions` alle `false`;
  `disable_scroll_properties: true`, `disable_session_recording: true`, `persistence: "memory"`,
  `disable_persistence: true`, `person_profiles: "never"`, `save_referrer`/`save_campaign_params: false`,
  `disable_surveys: true`, `advanced_disable_flags: true`, `before_send: sanitizePostHogEvent`.
- `frontend/src/lib/posthog-client.ts:93-127`: Lazy-Init per dynamischem `import("posthog-js")`
  im `requestIdleCallback`; Operationen vor Init werden gepuffert.
- `frontend/src/app/providers.tsx:22`: `useEffect(schedulePostHogInitialization, [])` in den
  Root-Providern. Kein `instrumentation-client.ts`-Init für PostHog; diese Datei existiert, lädt aber
  nur Sentry und den PWA-Prompt (`frontend/src/instrumentation-client.ts`).
- `frontend/src/lib/posthog-privacy.ts:4-22`: Event-Allowlist (16 Events inkl. 4 Demo-Events).
  `:32-37`: erlaubte SDK-Properties nur `token`, `distinct_id`, `$lib`, `$lib_version`.
  `:108-109`: erzwingt `$geoip_disable: true` und `$process_person_profile: false`.
  `:114-123`: `before_send` baut ein neues Event nur aus `uuid`, `event`, `timestamp`, gefilterten
  Properties. Folge: `$session_id`, `$window_id`, `$current_url`, `$pathname` werden verworfen,
  ebenso jedes `$snapshot`, `$$heatmap`, `$autocapture`, `$pageview`, `$pageleave`, `$dead_click`,
  `$rageclick`, `$exception`.
- `frontend/src/lib/analytics.ts:46-68` (`trackTenantEvent`) und `:70-82` (`trackPageView`):
  setzen `deployment`, `school_id` und `$groups: { school }`. Pageviews sind ein eigenes Event
  `page_viewed` mit `view_id` statt `$pageview`.
- `frontend/src/lib/analytics.ts:93-101`: Demo-Events nutzen die Demo-Access-ID als `distinct_id`.
- `frontend/src/lib/analytics-deployment.ts:8-10`: `deployment` = `"demo"` oder
  `NEXT_PUBLIC_TENANT_DOMAIN`.
- `frontend/src/components/auth/tenant-auth-wrapper.tsx:49-63`: registriert Schulkontext als
  Super Properties (`register`) und räumt beim Logout/Tenant-Wechsel per `unregister` + `reset` auf.
- `frontend/src/proxy.ts:52-69`: leitet aus `NEXT_PUBLIC_POSTHOG_HOST` den Origin ab;
  `:73-82`: CSP `script-src 'self' 'unsafe-inline'`, `connect-src 'self' <PostHog-Origin>`,
  kein `worker-src` (fällt auf `default-src 'self'` zurück). `:828-833`: Proxy-Matcher greift auf
  fast alle Pfade. **Kein Reverse Proxy für PostHog**, `frontend/next.config.js` enthält keine
  `rewrites`.
- Formulare mit echten Personendaten: `frontend/src/app/start/page.tsx:22-29` (Name, Kontakt,
  Organisation usw.). Ob das im Ticket gemeinte „Demo-Formular“ dieses oder `DemoSetupScreen` ist,
  habe ich nicht abschließend geprüft **[offen]**.
- Tests pinnen die Konfiguration: `frontend/src/lib/posthog-client.test.ts`,
  `posthog-privacy.test.ts`, `analytics.test.ts`, `app/providers.test.tsx`,
  `components/auth/tenant-auth-wrapper.test.tsx`, `proxy.test.ts`.

**Backend**

- `backend/analytics/analytics.go:7-10`: begründet den Verzicht auf `posthog-go` mit der
  MPL-2.0-Abhängigkeit. `:58-76`: `New` liefert No-op ohne `POSTHOG_API_KEY`.
  `:116-149`: `Capture` sendet je Event einen eigenen `/batch/`-Request in einer Goroutine (keine
  echte Bündelung). `:122-126`: erzwingt `$lib`, `$geoip_disable`, `$process_person_profile: false`.
  Kein `deployment`, kein `$session_id`.
- `backend/services/factory.go:363-364, 395-396, 683-687`: Konfiguration über Viper
  `posthog_api_key`/`posthog_host`.
- Einzige Aufrufstelle: `backend/modules/studentpresence/internal/application/presence/active_service.go:198-214`
  (`distinct_id = "school:<id>"`, `$groups`, `school_id`, nach Commit). Events:
  `student_checked_in`, `student_checked_out`, `room_transfer`.

**Deployment/Env** **[verifiziert, nur Schlüsselnamen]**

- `environments/{staging,production,demo}.compose.yml`: `POSTHOG_API_KEY`, `POSTHOG_HOST`,
  `NEXT_PUBLIC_POSTHOG_KEY`, `NEXT_PUBLIC_POSTHOG_HOST` als Pflichtvariablen (staging `:92-93, :139-140`,
  production `:111-112, :158-159`, demo `:97-98, :144-145`). `${VAR?msg}` bricht nur bei *fehlender*,
  nicht bei *leerer* Variable ab.
- `environments/runtime-env-allowlist.json:29-30, 72-73`: alle vier Schlüssel freigegeben.
- `docs/operations/demo-environment.md:104-109`: Demo nutzt dasselbe Projekt; Backend-Key bleibt leer.

**PyrePortal**

- Keine Analytics-Bibliothek in `PyrePortal/src` oder `package.json` (Treffer nur „plausible UID“).
- `PyrePortal/src/platform/webAdapterBase.ts:107-118`: Geräte-Key kommt aus `?key=` der URL und
  bleibt dort stehen; Router ist `BrowserRouter` (`PyrePortal/src/App.tsx:75`).

## 1. Produktkatalog PostHog Cloud EU

Free-Tier-Werte gelten monatlich und sind laut Pricing-Seite auf Free und Pay-as-you-go gleich
([Pricing](https://posthog.com/pricing)). Preise in USD, erste Stufe nach dem Free-Tier.

| Produkt | Was es tut | Nötige Konfiguration | Free-Tier / Preis |
|---|---|---|---|
| Product Analytics | Trends, Funnels, Retention, Pfade, SQL | Events per `capture` | 1 Mio. Events, dann $0,00005/Event, fallend bis $0,000009 ([Pricing PA](https://posthog.com/product-analytics/pricing)) |
| Web Analytics | Dashboard für Besucher, Pageviews, Sessions, Bounce, Quellen; basiert auf `$pageview` und Sessions ([Docs](https://posthog.com/docs/web-analytics)) | `$pageview` (`capture_pageview: "history_change"` bzw. `defaults`), `$session_id` | wie Product Analytics; „Anonymous events cost 10x less than identified“ ([Pricing WA](https://posthog.com/web-analytics/pricing)) |
| Session Replay Web/Mobile | rrweb-Aufzeichnung inkl. Konsole/Netzwerk optional | Projekt-Schalter + SDK, Remote-Config nötig (siehe 2) | Web 5.000, dann $0,0050/Aufnahme; Mobile 2.500, dann $0,0100; Aufbewahrung Free 1 Monat, bezahlt 3 Monate ([Pricing SR](https://posthog.com/session-replay/pricing)) |
| Heatmaps, Scrollmaps, Clickmaps | Maus/Klick/Scroll-Positionen; Clickmap braucht Autocapture, Scrollmap braucht Pageleave ([Heatmaps](https://posthog.com/docs/toolbar/heatmaps)) | `capture_heatmaps`/`enable_heatmaps`, Projekt-Schalter | Heatmap-Daten „doesn't contribute to your bill“ ([Heatmaps](https://posthog.com/docs/toolbar/heatmaps)) |
| Dead/Rage Clicks | Klicks ohne Seitenänderung; 3 Klicks in 30 px und 1 s | `capture_dead_clicks`, `rageclick` | Dead-Click-Events als normale Events bepreist; im Heatmap enthalten kostenlos ([Autocapture](https://posthog.com/docs/product-analytics/autocapture)) |
| Autocapture | Klicks, Form-Submits, Änderungen, Elementtext | `autocapture` bzw. Projekt-Schalter | als Events bepreist |
| Toolbar | Overlay auf der eigenen Seite für Heatmaps, Actions, Flag-Overrides ([Toolbar](https://posthog.com/docs/toolbar)) | Authorized URLs, CSP für `img-src`/`style-src`/`font-src` ([CSP](https://posthog.com/docs/advanced/content-security-policy)) | kostenlos **[Schluss]** |
| Surveys | Pop-up-Umfragen, Targeting über Events/Flags ([Docs](https://posthog.com/docs/surveys/installation)) | `disable_surveys: false`, Flags/Remote-Config | 1.500 Antworten, dann $0,10/Antwort ([Pricing](https://posthog.com/surveys/pricing)) |
| Feature Flags | Rollouts, Targeting, Remote Config | `/flags`-Requests; Personen-/Gruppen-Targeting braucht Profile | 1 Mio. Requests, dann $0,0001/Request ([Pricing](https://posthog.com/feature-flags/pricing)); lokale Evaluation zählt als 10 Requests ([Kosten](https://posthog.com/docs/billing/estimating-usage-costs)) |
| Experiments | A/B-Tests auf Basis von Flags | Flags + Metrik-Events | über Flag-Requests abgerechnet ([Pricing](https://posthog.com/experiments/pricing)) |
| Error Tracking | `$exception`-Events mit Stacktraces | `capture_exceptions` / Go: `NewDefaultException` | 100.000, dann $0,00037/Exception ([Pricing](https://posthog.com/error-tracking/pricing)) |
| LLM Analytics (heißt jetzt „AI Observability“) | Traces, Kosten, Latenz von LLM-Aufrufen | SDK-Wrapper | 100.000 Events, dann $0,00035 ([Pricing](https://posthog.com/ai-observability/pricing)) |
| Data Warehouse | SQL über Events, Personen, Sessions plus externe Quellen ([Docs](https://posthog.com/docs/data-warehouse/start-here)) | Quellen verknüpfen | 1 Mio. Rows + historische Daten frei ([Pricing](https://posthog.com/pricing)); Folgepreis **[offen]** |
| Data Pipelines/CDP | Realtime-Destinations, Batch-Exports, Quellen ([Docs](https://posthog.com/docs/cdp/start-here)) | Destinations einrichten | 10.000 Events + 1 Mio. Rows frei ([Pricing](https://posthog.com/pricing)); Folgepreis **[offen]** |
| Logs | Log-Ingestion (OTel) | OTel-Exporter | 10 GB frei, dann $0,25/GB; Aufbewahrung 14 Tage ([Logs Pricing](https://posthog.com/docs/logs/pricing)) |
| Revenue Analytics | Dediziertes Dashboard wurde entfernt; Umsatz-Properties an Personen/Gruppen ([Docs](https://posthog.com/docs/revenue-analytics)) | Zahlungsquellen/Events | für moto irrelevant **[Schluss]** |
| Group Analytics | Auswertung pro Organisation statt Person; max. 5 Gruppentypen je Projekt ([Docs](https://posthog.com/docs/product-analytics/group-analytics)) | `posthog.group()`, identifizierte Events | **bezahltes Add-on**, dann werden alle identifizierten Events des Projekts zusätzlich abgerechnet; Preis nicht auf statischer Seite **[offen]** |
| Persons/Identify | Personenprofile, Kohorten, Lifecycle, Targeting ([Docs](https://posthog.com/docs/data/anonymous-vs-identified-events)) | `identify`, `person_profiles` | anonyme Events „up to 4x cheaper“ laut Docs, „10x“ laut WA-Pricing (widersprüchlich) **[offen]** |

Plattform: Free = 1 Projekt, 1 Jahr Datenaufbewahrung, Community-Support; Pay-as-you-go =
6 Projekte, 7 Jahre, E-Mail-Support; Managed Reverse Proxy 2 (Free) bzw. 10 (PAYG)
([Pricing](https://posthog.com/pricing), [PA-Pricing](https://posthog.com/product-analytics/pricing)).
Boost $250/Monat, Scale $750/Monat, Enterprise individuell, alle „unlimited projects“; Boost bringt
u. a. SSO-Enforcement, 2FA-Enforcement, HIPAA BAA, Replay-Aufbewahrung 12 Monate; RBAC erst Enterprise
([Platform packages](https://posthog.com/platform-packages)). Billing-Limits pro Produkt; über dem
Limit gehen Daten verloren ([Limits](https://posthog.com/docs/billing/limits-alerts)).

## 2. Session Replay: Datenschutz-Stellschrauben (posthog-js)

**Grundsatz:** „Our privacy controls run in the browser … masked data is never sent over the network“
([Privacy controls](https://posthog.com/docs/session-replay/privacy)). **[verifiziert]**

| Mittel | Default | Wirkung |
|---|---|---|
| `session_recording.maskAllInputs` | `true` | alle Eingaben maskiert; Passwörter immer ([Privacy](https://posthog.com/docs/session-replay/privacy)) |
| `maskInputOptions`, `maskInputFn` | – | feiner steuern (nur sinnvoll, wenn man entmaskieren will) |
| `maskTextSelector: "*"` | Text **nicht** maskiert | maskiert allen Nicht-Input-Text („Mask all text“) |
| `maskTextClass` | `ph-mask` | einzelne Elemente maskieren ([Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)) |
| `maskTextFn` | – | eigene Textmaskierung |
| `blockClass` / `ph-no-capture` | `ph-no-capture` | Element wird als graue Box gleicher Größe ersetzt, auch kein Autocapture |
| `blockSelector` | `null` | CSS-Selektor für Blockierung, auch als Projekt-Einstellung; Client-Wert hat Vorrang ([Privacy](https://posthog.com/docs/session-replay/privacy)) |
| `maskAllElementAttributes` | `false` | maskiert alle String-Attribute (`class`, `src`, `href`, `style` …), reduziert Wiedergabetreue ([Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)) |
| Netzwerk | nur URL + Timing; Header/Bodies aus; Auth-Header und Cookies werden gescrubbt ([Network](https://posthog.com/docs/session-replay/network-recording)) | `recordHeaders`/`recordBody`, `maskCapturedNetworkRequestFn` |
| Seiten-URL im Replay | wird aufgezeichnet | `maskCapturedNetworkRequestFn` redigiert auch die Snapshot-URL ([Privacy](https://posthog.com/docs/session-replay/privacy)) |
| Canvas | aus; nicht durch DOM-Masking abgedeckt ([Canvas](https://posthog.com/docs/session-replay/canvas-recording)) | `captureCanvas`, `canvasCapture.maskRegionsFn` |
| Console-Logs | aus („we do not capture these logs automatically“) ([Console](https://posthog.com/docs/session-replay/console-log-recording)) | `enable_recording_console_log` |
| Sampling, Mindestdauer, URL-/Event-Trigger, Linked Flag | Projekt-Einstellungen, Trigger-Gruppen ANY/ALL; Web puffert bis 1 Min. vor Trigger ([Steuerung](https://posthog.com/docs/session-replay/how-to-control-which-sessions-you-record)) | `sampleRate`, `strictMinimumDuration` |
| Programmatisch | `posthog.startSessionRecording(override?)`, `stopSessionRecording()`, `disable_session_recording` ([Steuerung](https://posthog.com/docs/session-replay/how-to-control-which-sessions-you-record)) | Override für `sampling`, `linked_flag`, `url_trigger`, `event_trigger` |

**Vollmaskierung für Schulbildschirme [Schluss aus obigen Quellen]:**

```ts
session_recording: {
  maskAllInputs: true,
  maskTextSelector: "*",
  blockSelector: "img, video, canvas, svg image, [data-ph-block]",
  maskAllElementAttributes: true, // optional: auch href/src/title/alt; senkt Treue stark
  recordHeaders: false,
  recordBody: false,
  maskCapturedNetworkRequestFn: (req) => ({ ...req, name: stripQueryAndIds(req.name) }),
},
enable_recording_console_log: false,
```

Restrisiken trotz Vollmaskierung **[Schluss]**: Textlänge und Layout bleiben sichtbar (z. B. Anzahl
Kinder in einer Liste), Pfade mit IDs (`/students/123`) in URL und Netzwerk müssen redigiert
werden, `aria-label`/`title`/`alt` sind Attribute (nur `maskAllElementAttributes` deckt sie ab),
Projekt-Einstellungen im PostHog-UI können Canvas/Netzwerk/Console einschalten, sofern der Client
sie nicht explizit überschreibt (für Canvas ausdrücklich dokumentiert:
„Allows local config to override remote canvas recording settings“,
[Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).

**Replay mit `persistence: "memory"` und `person_profiles: "never"`** **[verifiziert im SDK-Code 1.415.2]**

- Der Recorder prüft `enabled_server_side && enabled_client_side && !optedOut`
  (`node_modules/posthog-js/lib/src/extensions/replay/session-recording.js:66-72`;
  Quelle: [session-recording.ts](https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/extensions/replay/session-recording.ts)).
  `enabled_server_side` kommt nur aus der Remote-Config.
- `advanced_disable_flags: true` führt zu „Remote config is disabled. Falling back to local config.“
  (`lib/src/remote-config.js`; [remote-config.ts](https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/remote-config.ts)).
  Die Config-Doku bestätigt: „Disabling this will also prevent remote configuration from loading“
  ([Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).
  **Folge: Replay startet mit heutiger Konfiguration nie, auch nach `startSessionRecording()`.**
  Alternative: `advanced_disable_feature_flags: true` lädt die Config, wertet aber keine Flags aus.
- `SessionIdManager._canUseSessionStorage()` gibt bei `persistence === "memory"` oder
  `disable_persistence` `false` zurück; Session- und Window-ID liegen dann nur im Speicher
  („won't persist page loads“) (`lib/src/sessionid.js:129-131`;
  [sessionid.ts](https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/sessionid.ts)).
  Replay funktioniert, aber jeder Full-Reload startet neue Session und neue anonyme `distinct_id`.
- Im Replay-Code fand ich keine Abhängigkeit von `person_profiles` **[Schluss]**. Replays hängen an
  `distinct_id` und `$session_id`, nicht an Personenprofilen.
- `cookieless_mode: "always"` ist mit Replay unvereinbar: Der Konstruktor wirft
  „cannot be used with cookieless_mode="always"“ (`session-recording.js:54-55`, `sessionid.js:58-59`).

**`before_send` und `$snapshot`** **[verifiziert im SDK-Code]**: Der Recorder sendet Daten über
`instance.capture('$snapshot', { $snapshot_data, $session_id, $window_id, … }, { _url: …/s/, _batchKey: 'recordings' })`
(`lazy-loaded-session-recorder.js:1794-1801`). `capture()` ruft vor dem Versand
`_runBeforeSend` auf (`posthog-core.js:1258-1266`;
[posthog-core.ts](https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/posthog-core.ts)).
Gibt `before_send` `null` zurück, wird der Snapshot verworfen. Genauso laufen `$$heatmap`
(`heatmaps.js:286`), `$dead_click`, `$rageclick`, `$pageleave` durch `before_send`. Der heutige
Sanitizer müsste `$snapshot` unverändert durchlassen (inkl. `$snapshot_data`, `$session_id`,
`$window_id`, `$snapshot_bytes`, `$snapshot_host`), denn er baut Properties neu auf.

**Laden des Recorders und CSP** **[verifiziert im SDK-Code]**: `loadExternalDependency` lädt
`/static/recorder.js` über `requestRouter.endpointFor("assets", …)`; bei EU-Host ergibt das
`https://eu-assets.i.posthog.com/static/…`, bei eigenem Proxy-Host den `api_host` selbst
([request-router.ts](https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/utils/request-router.ts)).
Die Remote-Config lädt zuerst als Skript, dann als JSON von `/array/<token>/config`. PostHog nennt
als CSP `script-src` und `connect-src` `https://*.posthog.com` und `worker-src 'self' blob: data:`;
mit Reverse Proxy genügt die eigene Domain
([CSP](https://posthog.com/docs/advanced/content-security-policy)). moto blockiert beides heute.
Ob der Recorder ohne `worker-src blob:` degradiert oder scheitert, habe ich nicht getestet **[offen]**.

## 3. Heatmaps und Autocapture

- Flags: `autocapture` (bool oder Objekt mit `url_allowlist`, `url_ignorelist`,
  `dom_event_allowlist`, `element_allowlist`, `css_selector_allowlist`/`ignorelist`,
  `element_attribute_ignorelist`, `capture_copied_text`), `capture_heatmaps`/`enable_heatmaps`,
  `capture_dead_clicks` (inkl. `css_selector_ignorelist`, Klasse `ph-no-deadclick`), `rageclick`,
  `capture_pageleave` (`true` | `"if_capture_pageview"`), `capture_pageview`
  (`true` | `"history_change"`) ([JS-Config](https://posthog.com/docs/libraries/js/config),
  [Autocapture](https://posthog.com/docs/product-analytics/autocapture),
  [Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).
- Next.js App Router: `defaults: '2025-05-24'` oder später setzt `capture_pageview: 'history_change'`
  ([JS-Config](https://posthog.com/docs/libraries/js/config)). Aktuelle Snapshots:
  `2026-05-30` (Next.js-Doku), `2026-08-29` (JS-Config). moto nutzt `2026-01-30`.
- Heatmaps erzeugen keine zusätzlichen Events („No additional events are created“), Clickmap braucht
  Autocapture, Scrollmap braucht Pageleave ([Autocapture](https://posthog.com/docs/product-analytics/autocapture),
  [Heatmaps](https://posthog.com/docs/toolbar/heatmaps)). moto setzt `disable_scroll_properties: true`;
  ob das die Scrollmap leert, ist nicht dokumentiert **[offen, wahrscheinlich ja]**.
- Datenschutz Autocapture: Formularwerte werden nicht erfasst („we do not automatically capture form
  values“), Elementtext aber schon (`Element text`); `mask_all_text` verhindert `textContent`,
  `mask_all_element_attributes` verhindert Attributnamen/-werte; `.ph-no-autocapture` bzw.
  `data-ph-no-autocapture` schließt Elemente aus ([Autocapture](https://posthog.com/docs/product-analytics/autocapture),
  [Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).
  Für Schulen **[Schluss]**: Buttontexte wie Kindername in Listen würden ohne `mask_all_text` als
  `$el_text` gesendet.

## 4. Identität und Personen

- Anonym vs. identifiziert: identifizierte Events erzeugen Personenprofile; nur sie erlauben
  Personen-Properties, Kohorten, Lifecycle, Flag-Targeting nach Person
  ([Docs](https://posthog.com/docs/data/anonymous-vs-identified-events)).
- `person_profiles`: `identified_only` (Default), `always`, `never`. Die Doku-Seite nennt `never` nur
  für Mobile; das Web-SDK 1.415.2 unterstützt `never` laut Typdefinition
  („All events (including `$identify`) will be sent with `$process_person_profile: False`“)
  ([Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).
- Persistenz: `localStorage+cookie` (Default), `localStorage`, `sessionStorage`, `cookie`, `memory`
  ([JS-Config](https://posthog.com/docs/libraries/js/config)). Cookieless: `cookieless_mode:
  "always" | "on_reject"`, Server-Hash, Projekt-Schalter „Cookieless server hash mode“ nötig;
  `identify` in `always` nicht empfohlen, Alias-Events werden verworfen
  ([Cookieless](https://posthog.com/docs/tutorials/cookieless-tracking)).
- Kosten: anonyme Events „up to 4x cheaper“ ([Docs](https://posthog.com/docs/data/anonymous-vs-identified-events));
  WA-Pricing sagt „10x less“ ([WA-Pricing](https://posthog.com/web-analytics/pricing)). Auf den
  statischen Preisseiten steht nur ein Event-Preis; der Aufschlag für identifizierte Events ist dort
  nicht ausgewiesen **[offen]**.
- **Schule als Gruppe:** technisch naheliegend (`posthog.group('school', id)`), aber:
  „If `$process_person_profile` is set to `false`, the event won't link to the group“ und
  Group Analytics ist ein bezahltes Add-on, das alle identifizierten Events zusätzlich abrechnet
  ([Group Analytics](https://posthog.com/docs/product-analytics/group-analytics)). moto sendet heute
  `$groups` mit `$process_person_profile: false` (Frontend `posthog-privacy.ts:108-109`, Backend
  `analytics.go:126`). **Die Gruppenzuordnung ist damit wirkungslos; `school_id` als Property trägt
  die Auswertung.**
- Pseudonyme IDs: PostHog empfiehlt eindeutige, stabile Strings, bei Multi-Tenant z. B.
  `identify(`${customerId}:${user.id}`)` ([Embedded-Analytics-Projekte](https://posthog.com/docs/data/embedded-analytics-projects)).
  Eine Empfehlung für Hashing fand ich nicht **[offen]**. PostHog selbst stuft eine persistente
  `distinct_id` als personenbezogen ein ([Cookieless](https://posthog.com/docs/tutorials/cookieless-tracking)).
  **[Schluss]**: Ein Hash der Account-ID bleibt pseudonym, nicht anonym.
- Löschen: Person im UI oder per API, `delete_events=true`, `delete_recordings=true`; asynchron
  ([Persons](https://posthog.com/docs/data/persons)). Ohne Personenprofil fehlt der Hebel, Replays
  einer bestimmten Person zu löschen **[Schluss]**; dann bleibt Löschen einzelner Recordings oder
  des ganzen Projekts.

## 5. Projekte und Umgebungen

- Free: 1 Projekt; Pay-as-you-go: 6 Projekte; Boost/Scale/Enterprise: unbegrenzt
  ([Pricing](https://posthog.com/pricing), [Platform packages](https://posthog.com/platform-packages)).
  Der Free-Tier ist laut Pricing „Same free tier every month“; ob er pro Organisation oder pro Projekt
  gilt, steht dort nicht explizit **[offen, vermutlich pro Organisation]**.
- PostHog empfiehlt drei Projekte (Local, Staging, Production)
  ([Projects](https://posthog.com/docs/settings/projects),
  [Tutorial](https://posthog.com/tutorials/multiple-environments)). Nachteil: Dashboards, Insights,
  Actions lassen sich nicht direkt zwischen Projekten kopieren (Tutorial).
- „Environments“ innerhalb eines Projekts existieren als API-Konzept
  ([Environments API](https://posthog.com/docs/api/environments); PR
  [#97325](https://github.com/PostHog/posthog/pull/97325) erwähnt „several environments per project“).
  Eine Produktdoku mit Status und Plan-Zuordnung fand ich nicht **[offen]**.
- Filterung in einem Projekt: `host`-Property oder Super Properties per `register`
  ([Projects](https://posthog.com/docs/settings/projects)). `internal_or_test_user_hostname`
  markiert ab `defaults: '2026-01-30'` localhost automatisch als Test-User und **aktiviert dafür
  Personenverarbeitung** ([JS-Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts));
  greift bei moto nur lokal.
- Replay kennt keine Projekt-interne Trennung außer Filtern und URL-Triggern **[Schluss]**: Wer
  Replay nur in der Demo will, braucht bei einem Projekt clientseitiges Gating (Build-Flag
  `isDemoBuild()`), sonst ein eigenes Demo-Projekt mit eigenem Token.

## 6. Backend (Go)

- `posthog-go` v1.27.0 (22.09.2026), MIT, hängt aber von `github.com/hashicorp/golang-lru/v2`
  ab ([go.mod](https://github.com/PostHog/posthog-go/blob/master/go.mod)); die Lizenzbegründung in
  `analytics.go:7-10` gilt weiter **[verifiziert]**.
- Funktionsumfang laut [Go-Doku](https://posthog.com/docs/libraries/go) und
  [config.go](https://github.com/PostHog/posthog-go/blob/master/config.go): `Enqueue(Capture/Alias/GroupIdentify)`,
  `Groups`, `$process_person_profile`, Flags mit `EvaluateFlags`, lokale Evaluation über
  Personal/Secret Key, Error Tracking (`NewDefaultException`, `NewSlogCaptureHandler`),
  `NewRequestContextMiddleware` liest `X-PostHog-Distinct-Id` und `X-PostHog-Session-Id`.
  `DefaultEventProperties` („merged into every Capture event“) entspricht Super Properties.
  Batching: `BatchSize` 100, `Interval` 5 s, `MaxQueueSize` 10.000, `ShutdownTimeout`; `Close()` flusht.
- Für den eigenen Tracker **[Schluss]**: `deployment` als festes Feld beim Erzeugen (analog
  `DefaultEventProperties`), optional `$session_id` aus Header `X-POSTHOG-SESSION-ID`, echte
  Bündelung über Channel + Ticker, `/batch/` akzeptiert mehrere Events pro Request
  ([Capture API](https://posthog.com/docs/api/capture)). Damit ist `POSTHOG_API_KEY` in der Demo setzbar.

## 7. Zentrale Einbindung in Next.js App Router

- Offizielle Empfehlung: Init in `instrumentation-client.ts` mit `defaults: '2026-05-30'`
  ([Next.js-Doku](https://posthog.com/docs/libraries/next-js)). moto initialisiert lazy im
  `Providers`-Effect; das ist zulässig, verzögert aber den Start und damit den Replay-Beginn **[Schluss]**.
- Reverse Proxy per Rewrites (EU):
  `/ingest/static/:path*` → `https://eu-assets.i.posthog.com/static/:path*`,
  `/ingest/array/:path*` → `https://eu-assets.i.posthog.com/array/:path*`,
  `/ingest/:path*` → `https://eu.i.posthog.com/:path*`, `skipTrailingSlashRedirect: true`,
  `api_host: '/ingest'`, `ui_host: 'https://eu.posthog.com'`. Bestehende Middleware/Proxy-Matcher
  müssen den Pfad ausnehmen ([Proxy Next.js](https://posthog.com/docs/advanced/proxy/nextjs)). Bei moto:
  Negativ-Lookahead in `frontend/src/proxy.ts:831` ergänzen; `proxy.ts:52-69` erwartet heute eine
  absolute URL und müsste einen relativen `api_host` zulassen. Managed Reverse Proxy ist als
  Plattformfunktion gelistet (2 Proxies Free) ([PA-Pricing](https://posthog.com/product-analytics/pricing)).
- Server: `posthog-node` mit `flushAt: 1`, `flushInterval: 0`, `await posthog.shutdown()`
  ([Next.js-Doku](https://posthog.com/docs/libraries/next-js)). Für moto nur nötig, wenn Next-Route-Handler
  selbst Events senden sollen; heute gehen serverseitige Events über das Go-Backend **[Schluss]**.
- Session-Verknüpfung: `tracing_headers: ['<api-host>']` fügt `X-POSTHOG-DISTINCT-ID`,
  `X-POSTHOG-SESSION-ID`, `X-POSTHOG-WINDOW-ID` an Requests; nur Hostnamen
  ([JS-Config](https://posthog.com/docs/libraries/js/config),
  [Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).
  Alternativ `posthog.get_session_id()` weiterreichen und als `$session_id` setzen
  ([Sessions](https://posthog.com/docs/data/sessions)). Da moto den Browser-Traffic über
  Next-Route-Handler zum Backend proxyt, müssten diese die Header durchreichen **[Schluss]**.
- Flag-Bootstrapping: nur wirksam, wenn Flags vor dem Rendern (serverseitig) ausgewertet werden
  ([Next.js-Doku](https://posthog.com/docs/libraries/next-js)); Option `bootstrap`
  ([Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)).

## 8. Kiosk / PyrePortal

- PostHog hat keine Kiosk-spezifische Doku **[offen]**. React/Vite-Setup: `posthog-js` +
  `@posthog/react`, `VITE_POSTHOG_*`-Variablen ([React-Doku](https://posthog.com/docs/libraries/react)).
- Caveats **[Schluss, gestützt auf Quellen]**:
  1. **Geräte-Key in der URL** (`webAdapterBase.ts:107-118`): landet ohne Gegenmaßnahme in
     `$current_url`, `$pathname`-Kontext und Replay-URL. Gegenmittel: `before_send`-Redaktion,
     `mask_personal_data_properties` + `custom_personal_data_properties: ['key']`
     ([Config-Typen](https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts)),
     `maskCapturedNetworkRequestFn` für Replay ([Privacy](https://posthog.com/docs/session-replay/privacy)).
  2. **Lang laufende Tabs:** Sessions enden nach 30 Min. Inaktivität oder 24 h
     ([Sessions](https://posthog.com/docs/data/sessions)); PostHogs eigene Desktop-App hatte bei
     dauerhaft laufendem Replay Speicherwachstum bis zum Absturz
     ([PR #92040](https://github.com/PostHog/posthog/pull/92040)). Replay-Daten werden im Speicher
     gepuffert und können bei Reload verloren gehen
     ([posthog-js #1099](https://github.com/PostHog/posthog-js/issues/1099)).
  3. **Kinderdaten auf dem Kiosk** (Namen beim Scan) machen Replay dort zum Hochrisiko-Fall.
  4. **Offline/Netz:** Analytics-Requests konkurrieren mit NFC-Scan-Requests; Rate-Limiting und
     Fehlerverhalten des SDK sind fire-and-forget, aber ungetestet auf GKT **[offen]**.
  5. PyrePortal meldet dieselben Vorgänge (Check-in/-out) bereits indirekt über Backend-Events
     (`active_service.go`). Für Nutzungszahlen reicht das möglicherweise **[Schluss]**.

## 9. Recht und DSGVO (nur Fakten aus PostHog-Quellen)

- Hosting EU: Frankfurt ([GDPR](https://posthog.com/docs/privacy/gdpr-compliance),
  [Data storage](https://posthog.com/docs/privacy/data-storage)); Subprocessor AWS „Germany (PostHog EU
  Cloud)“; weitere Core-Subprocessors Wiz (DE/FR), PlanetScale, Modal Labs (DE für EU), Cloudflare
  („Global edge locations … for data in transit“) ([Subprocessors](https://posthog.com/subprocessors)).
  Eigene Abschnitte „AI Subprocessors (Only if AI Features are Enabled)“ und „Internal Subprocessors“;
  deren Einträge konnte ich nicht auslesen **[offen]**.
- Vertragspartner ist PostHog, Inc., San Francisco; DPA inkorporiert EU-SCC; Subprocessor-Änderungen
  mit 14 Tagen Vorankündigung und Widerspruchsmöglichkeit; Abschluss self-serve unter
  `app.posthog.com/legal`, Signatur über PandaDoc ([DPA](https://posthog.com/dpa),
  [Privacy](https://posthog.com/docs/privacy)).
- Consent: „if you use PostHog with cookies on your website (for logged out users), you should also use
  a cookie banner“ ([GDPR](https://posthog.com/docs/privacy/gdpr-compliance)); Werkzeuge:
  `opt_out_capturing_by_default`, `opt_in_capturing()`, `cookieless_mode`
  ([Data collection](https://posthog.com/docs/privacy/data-collection)). GDPR-Seite erwähnt Kinder nur
  mit „Children under 13 can only give consent with permission from their parent“.
- Aufbewahrung: Events Free 1 Jahr, bezahlt 7 Jahre, **nicht verkürzbar** („a shorter period is not
  available on request“) ([Retention](https://posthog.com/docs/data/events-retention)); Replays Free
  1 Monat, PAYG 90 Tage, Boost/Scale bis 1 Jahr, Enterprise bis 5 Jahre; Logs 14 Tage
  ([Pricing FAQ](https://posthog.com/pricing)).
- Löschung: Personen inkl. Events/Recordings per API; Recordings per Crypto-Shredding, Metadaten nach
  10 Tagen ([Replay-Privacy](https://posthog.com/docs/session-replay/privacy)); Event-Löschungen
  laufen asynchron in Nebenzeiten ([Data storage](https://posthog.com/docs/privacy/data-storage)).
  IP-Verwerfen als Projekt-Einstellung (nicht im Cookieless-Hash-Modus) (ebd.).

## 10. Programmatischer Zugriff für einen KI-Agenten

- API-Hosts EU: privat `https://eu.posthog.com`, öffentlich (Ingestion) `https://eu.i.posthog.com`
  ([API](https://posthog.com/docs/api)). Query-Endpoint `/api/projects/:project_id/query/` mit
  `HogQLQuery`, Scope „Query Read“; Limits 2.400/h, 240/min, 3 parallel, 10 s Laufzeit
  ([Queries](https://posthog.com/docs/api/queries)).
- Scopes haben das Format `<objekt>:read|write`
  ([scopes.py](https://github.com/PostHog/posthog/blob/master/posthog/scopes.py)). Relevant für
  Dashboards/Analyse: `query:read` („Covers query and events endpoints“), `insight:read`,
  `dashboard:read`, `session_recording:read`, `session_recording_playlist:read`, `person:read`,
  `cohort:read`, `event_definition:read`, `property_definition:read`, `heatmap:read`,
  `web_analytics:read`, `feature_flag:read`; zum Anlegen von Dashboards zusätzlich
  `insight:write`, `dashboard:write`. Löschen von Personen: `person:write` bzw. `data_deletion`.
- MCP: offizieller Server `https://mcp.posthog.com/mcp` (das eigenständige Repo
  [PostHog/mcp](https://github.com/PostHog/mcp) ist archiviert, Code liegt in
  [posthog/services/mcp](https://github.com/PostHog/posthog/tree/master/services/mcp)).
  Claude Code: `claude mcp add --transport http posthog https://mcp.posthog.com/mcp -s user`,
  alternativ `npx @posthog/wizard mcp add` oder `claude plugin install posthog`
  ([Claude-Code-Doku](https://posthog.com/docs/model-context-protocol/claude-code)).
  Auth: OAuth (empfohlen) oder Personal API Key; Region wird automatisch geroutet
  ([FAQ](https://posthog.com/docs/model-context-protocol/faq)); das README nennt für EU-OAuth zusätzlich
  `mcp-eu.posthog.com` ([README](https://github.com/PostHog/posthog/blob/master/services/mcp/README.md))
  (Widerspruch, beide Quellen von PostHog).
  Einschränkungen: `readonly=true` bzw. Header `x-posthog-read-only: true`, Projekt-Pinning per
  `project_id` / `x-posthog-project-id` ([FAQ](https://posthog.com/docs/model-context-protocol/faq)),
  Tool-Filter `?features=insights,dashboards,sql,replay,...` oder `?tools=...`
  ([README](https://github.com/PostHog/posthog/blob/master/services/mcp/README.md)).
  Feature-Gruppen u. a. `insights`, `dashboards`, `sql`, `replay`, `persons`, `flags`, `experiments`,
  `surveys`, `error_tracking`, `web_analytics`, `docs`, `workspace`.
- Achtung: Der Key-Preset „MCP Server“ vergibt `:write` auf alle nicht privilegierten Scopes mit
  `access_type: 'all'` ([scopes.tsx](https://github.com/PostHog/posthog/blob/master/frontend/src/lib/scopes.tsx));
  die FAQ sagt dagegen, er sei auf ein Projekt beschränkt. **[Schluss]**: Für einen Agenten OAuth
  mit `readonly=true` und `project_id` nutzen oder einen eigenen Key nur mit Read-Scopes erstellen.

## Übersicht: Feature-Bewertung für moto

| Feature | Nutzen für moto | Nötige Config-Änderung | Kosten/Free-Tier | Datenschutz-Risiko |
|---|---|---|---|---|
| Custom Events (heute) | Demo-Funnel, Nutzung pro Rolle/Schule | `deployment` im Backend; Allowlist pflegen | im 1-Mio.-Tier | gering (Allowlist) |
| `$pageview` + Web Analytics | Meistbesuchte Seiten ohne eigenes `page_viewed` | `capture_pageview: "history_change"`, `$pageview`, `$current_url` redigiert (IDs), `$session_id` durchlassen | Events im Tier | mittel: URLs enthalten IDs |
| Session Replay Demo | Demo-Nutzung verstehen | Remote-Config (`advanced_disable_flags` weg), Reverse Proxy oder CSP, `disable_session_recording: false` nur in Demo, `$snapshot` in `before_send`, Masking inkl. `/start`-Formular | 5.000/Monat, 1 Monat Aufbewahrung | mittel: echte Interessenten-Daten in Formularen |
| Session Replay Schulen | UX-Probleme in echten Abläufen | wie Demo + Vollmaskierung, Bild-Block, URL-Redaktion, ggf. Sampling/URL-Trigger | teilt Tier mit Demo | **hoch**: Kinderdaten, Layout-Rückschlüsse |
| Heatmaps | Klick-/Scrollverhalten | `capture_heatmaps: true`, `$$heatmap` durchlassen; Scrollmap braucht Pageleave | kostenlos | gering bis mittel (Koordinaten + URL) |
| Autocapture | Klickpfade ohne Code | `autocapture` mit `mask_all_text`, `mask_all_element_attributes`, Allowlists | Events, kann Tier füllen | mittel: Elementtext |
| Dead/Rage Clicks | UI-Fehler finden | `capture_dead_clicks`, `rageclick` | Dead Clicks als Events | gering bis mittel |
| Pageleave | Verweildauer, Scrollmap | `capture_pageleave: "if_capture_pageview"`, `disable_scroll_properties: false` | Events | gering |
| Feature Flags/Experiments | Rollouts pro Schule | Flags aktivieren; Targeting nach Person braucht Profile | 1 Mio. Requests | gering; stabil nur mit Persistenz |
| Surveys | Feedback von Personal | `disable_surveys: false`, Flags | 1.500 Antworten | mittel (Freitext) |
| Error Tracking | – (Sentry vorhanden) | `capture_exceptions` | 100.000 | mittel (Stacktraces, Messages) |
| Group Analytics | Auswertung pro Schule als Gruppe | identifizierte Events + Add-on | kostenpflichtig | mittel (Personenprofile nötig) |
| Persons/Identify | Kohorten, Retention pro Nutzer | `identify`, `person_profiles` ≠ `never`, Persistenz | teurere Events | hoch (Personenbezug, Consent-Frage) |
| Backend-Events mit Session | Kiosk/Server-Vorgänge mit Frontend-Session verbinden | `tracing_headers`, Header durchreichen, `$session_id` im Tracker | Events | gering |
| Logs | Alternative/Ergänzung zu bestehendem Logging | OTel-Exporter | 10 GB | mittel |
| MCP/API für Agent | Dashboards anlegen, Auswertungen | OAuth + `readonly`, Projekt-Pin | kostenlos | mittel (Agent sieht Replays) |
| Kiosk-Analytics | Nutzung an Geräten | neues SDK in PyrePortal, Key-Redaktion | Events | hoch ohne Redaktion (API-Key) |

## Offene Fragen für die Entscheidung (#3578)

**Technisch**

1. Reverse Proxy (`/ingest` per Rewrite) oder CSP um `eu.i.posthog.com` und `eu-assets.i.posthog.com`
   (inkl. `worker-src blob:`) erweitern? Proxy erfordert Anpassung von `proxy.ts:52-69` und `:831`.
2. `advanced_disable_flags: true` durch `advanced_disable_feature_flags: true` ersetzen (Config ja,
   Flags nein)? Welche Projekt-Einstellungen im PostHog-UI (Replay, Canvas, Netzwerk, Console,
   Autocapture, Heatmaps) sollen per Client-Config hart überschrieben werden?
3. Replay nur per Build (`isDemoBuild()`) oder per Projekt trennen? Bei einem Projekt: URL-Trigger
   reichen nicht als Sicherheitsgrenze, Client-Gating ist Pflicht.
4. Soll die Allowlist in `posthog-privacy.ts` SDK-Events (`$snapshot`, `$$heatmap`, `$pageview`,
   `$pageleave`, `$autocapture`, `$dead_click`, `$rageclick`) mit je eigener Property-Allowlist
   durchlassen, und welche URL-Redaktion gilt für `$current_url`/`$pathname`?
5. `persistence: "memory"` beibehalten (Sessions reißen bei Reload ab) oder `sessionStorage`
   (Session über Reload, keine Cookies)? Folgen für Consent siehe unten.
6. Gruppenfeld `$groups` entfernen oder bewusst auf identifizierte Events umstellen?
7. Backend: `deployment` fest pro Instanz, `$session_id` aus Tracing-Header, echte Bündelung?
8. Kiosk: überhaupt Frontend-Analytics in PyrePortal, oder reichen Backend-Events?
9. Init auf `instrumentation-client.ts` umziehen (früherer Replay-Start) oder Lazy-Init behalten?

**Kosten**

1. Pay-as-you-go (Karte, 6 Projekte, Billing-Limits je Produkt auf Free-Tier-Niveau) für getrennte
   Projekte Demo/Staging/Production akzeptabel?
2. Gilt der Free-Tier pro Organisation oder pro Projekt? (auf Pricing-Seite nicht eindeutig)
3. Reicht 1 Monat Replay-Aufbewahrung (Free) bzw. 90 Tage (PAYG)?
4. Group Analytics Add-on: Preis nicht öffentlich auf statischen Seiten; nur bei Bedarf anfragen.
5. Erwartetes Volumen: Replays pro Monat bei allen Schulen vs. 5.000 Freikontingent; Sampling nötig?

**Datenschutz/Recht (für DSB/Juristen)**

1. Rechtsgrundlage für Replay auf Schulbildschirmen mit Kinderdaten, auch bei Vollmaskierung
   (Layout, URLs, Zeitpunkte bleiben personenbeziehbar?).
2. Verhältnis moto ↔ Schule/Schulträger: Ist PostHog ein weiterer Unterauftragsverarbeiter in den
   AV-Verträgen mit den Schulträgern, und müssen Schulen vorab informiert werden (14-Tage-Frist gilt
   nur zwischen PostHog und moto)?
3. Drittlandbezug: Vertragspartner PostHog, Inc. (USA), EU-SCC im DPA, Cloudflare-Edge weltweit.
   Reicht das? Ist PostHog unter dem EU-US Data Privacy Framework zertifiziert (nicht geprüft)?
4. TDDDG § 25: Braucht `sessionStorage`/`localStorage`-Nutzung für Analytics eine Einwilligung,
   im Portal (Personal), im Elternportal, in der Demo (Interessenten)? Heute: `memory`, keine Speicherung.
5. Nicht verkürzbare Event-Aufbewahrung (1 bzw. 7 Jahre) vs. Speicherbegrenzung; reicht Löschen per API?
6. Datenschutzerklärung und Demo-Hinweis: welche Kategorien (Replay, Heatmaps, Pseudonym-IDs)
   müssen genannt werden?
7. KI-Funktionen von PostHog (PostHog AI, Replay Vision) nutzen weitere Subprocessors; sollen sie für
   das moto-Projekt deaktiviert bleiben?
8. DPA mit PostHog bereits abgeschlossen? (in `app.posthog.com/legal` bzw. EU-Pendant prüfen)

## Quellen

**PostHog Pricing und Legal**
- https://posthog.com/pricing
- https://posthog.com/product-analytics/pricing
- https://posthog.com/web-analytics/pricing
- https://posthog.com/session-replay/pricing
- https://posthog.com/feature-flags/pricing
- https://posthog.com/experiments/pricing
- https://posthog.com/surveys/pricing
- https://posthog.com/error-tracking/pricing
- https://posthog.com/ai-observability/pricing
- https://posthog.com/docs/logs/pricing
- https://posthog.com/platform-packages
- https://posthog.com/docs/billing/limits-alerts
- https://posthog.com/docs/billing/estimating-usage-costs
- https://posthog.com/dpa
- https://posthog.com/subprocessors
- https://posthog.com/docs/privacy
- https://posthog.com/docs/privacy/gdpr-compliance
- https://posthog.com/docs/privacy/data-storage
- https://posthog.com/docs/privacy/data-collection
- https://posthog.com/docs/data/events-retention

**PostHog Docs**
- https://posthog.com/docs/session-replay/privacy
- https://posthog.com/docs/session-replay/how-to-control-which-sessions-you-record
- https://posthog.com/docs/session-replay/network-recording
- https://posthog.com/docs/session-replay/console-log-recording
- https://posthog.com/docs/session-replay/canvas-recording
- https://posthog.com/docs/toolbar/heatmaps
- https://posthog.com/docs/toolbar
- https://posthog.com/docs/product-analytics/autocapture
- https://posthog.com/docs/product-analytics/group-analytics
- https://posthog.com/docs/data/anonymous-vs-identified-events
- https://posthog.com/docs/data/persons
- https://posthog.com/docs/data/sessions
- https://posthog.com/docs/data/embedded-analytics-projects
- https://posthog.com/docs/settings/projects
- https://posthog.com/tutorials/multiple-environments
- https://posthog.com/docs/api/environments
- https://posthog.com/docs/tutorials/cookieless-tracking
- https://posthog.com/docs/web-analytics
- https://posthog.com/docs/revenue-analytics
- https://posthog.com/docs/data-warehouse/start-here
- https://posthog.com/docs/cdp/start-here
- https://posthog.com/docs/surveys/installation
- https://posthog.com/docs/libraries/js/config
- https://posthog.com/docs/libraries/next-js
- https://posthog.com/docs/libraries/react
- https://posthog.com/docs/libraries/go
- https://posthog.com/docs/advanced/proxy/nextjs
- https://posthog.com/docs/advanced/content-security-policy
- https://posthog.com/docs/api
- https://posthog.com/docs/api/queries
- https://posthog.com/docs/api/capture
- https://posthog.com/docs/model-context-protocol
- https://posthog.com/docs/model-context-protocol/claude-code
- https://posthog.com/docs/model-context-protocol/faq

**GitHub (PostHog)**
- https://github.com/PostHog/posthog-js/blob/main/packages/types/src/posthog-config.ts
- https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/posthog-core.ts
- https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/sessionid.ts
- https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/remote-config.ts
- https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/extensions/replay/session-recording.ts
- https://github.com/PostHog/posthog-js/blob/main/packages/browser/src/utils/request-router.ts
- https://github.com/PostHog/posthog-js/issues/1099
- https://github.com/PostHog/posthog-go/blob/master/go.mod
- https://github.com/PostHog/posthog-go/blob/master/config.go
- https://github.com/PostHog/posthog/blob/master/services/mcp/README.md
- https://github.com/PostHog/posthog/blob/master/posthog/scopes.py
- https://github.com/PostHog/posthog/blob/master/frontend/src/lib/scopes.tsx
- https://github.com/PostHog/posthog/pull/97325
- https://github.com/PostHog/posthog/pull/92040
- https://github.com/PostHog/mcp (archiviert)

**Lokal geprüfter SDK-Code (posthog-js 1.415.2, `frontend/node_modules/posthog-js/lib/src/`)**
- `extensions/replay/session-recording.js:54-72`
- `extensions/replay/external/lazy-loaded-session-recorder.js:1725-1801`
- `posthog-core.js:1258-1266, 1339, 3120-3146, 3865-3890`
- `sessionid.js:58-59, 129-131`
- `remote-config.js:24-60`
- `heatmaps.js:286`
- `dist/module.js` (`endpointFor("assets")` → `https://<region>-assets.i.posthog.com`)
