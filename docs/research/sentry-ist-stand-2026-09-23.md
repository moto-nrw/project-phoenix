# Sentry: Ist-Stand und Fähigkeiten (Stand 23.09.2026)

Recherche als Grundlage für #3586. Keine Entscheidung, keine Umsetzung.

- **Codestand:** `origin/development` bei `6713d56a53`. PR #3585 ist offen und
  nicht gemergt. Wo er etwas ändert, steht das ausdrücklich als „mit #3585“ dabei.
- **Kennzeichnung:** **[belegt]** heißt: im Code, in der Konfiguration oder
  per `gh` nachgesehen. **[abgeleitet]** heißt: aus belegten Fakten
  geschlossen, aber nicht beobachtet. **[Doku]** heißt: laut Sentry-Primärquelle.
- **Nicht gelesen:** `environments/*.sops.env`. Ein Repo-Hook blockiert
  jeden Zugriff, und die Werte sind verschlüsselt. Welche Werte dort für
  `SENTRY_ENVIRONMENT` und `SENTRY_DSN` stehen, ist deshalb offen.

## Kurzfazit: die 10 größten Lücken

| # | Lücke | Beleg |
|---|---|---|
| 1 | **Demo-Frontend hat gar keinen Sentry-DSN.** Der Build liest für Demo das Repo-Secret `DEMO_NEXT_PUBLIC_SENTRY_DSN`, das nicht existiert. Damit ist der DSN leer, und `Sentry.init` läuft nicht. #3586 geht von einem „eigenen DSN“ aus, das stimmt für das Frontend nicht. | `.github/workflows/build.yml:378`; `gh secret list` zeigt nur `NEXT_PUBLIC_SENTRY_DSN` und `SENTRY_AUTH_TOKEN`, auch in den GitHub-Environments `demo`/`production` keine Sentry-Secrets **[belegt]**; `frontend/src/sentry.client.config.ts:7`; `docs/operations/demo-environment.md:109-112` |
| 2 | **Kein Release, weder im Frontend noch im Backend.** Ohne Release gibt es keine Regressionserkennung, keine Zuordnung zu Deploys und keine Suspect Commits. | FE: `sentry.client.config.ts:8-17` ohne `release`, `build.yml:370-380` ohne `SENTRY_RELEASE`; BE: `backend/cmd/serve.go:46-52` ohne `Release`, `backend/Dockerfile:14-18` ohne `.git` im Kontext und ohne Build-Arg **[belegt]**. Dass wirklich kein Release ankommt, ist **[abgeleitet]**, siehe Abschnitt Environments & Releases |
| 3 | **Minifizierte Stacktraces im Frontend.** Sourcemap-Upload ist abgeschaltet. | `frontend/next.config.js:88-90` (`sourcemaps.disable: true`) **[belegt]**; mit #3585 nur, wenn `SENTRY_AUTH_TOKEN` gesetzt ist |
| 4 | **Behandelte Fehler im Frontend landen nie in Sentry.** 824 `logger.error`-Aufrufe in 395 Dateien gehen nur nach Loki. Auch BFF-Routen, die 500 liefern, loggen nur, denn `onRequestError` greift nur bei unbehandelten Fehlern. | `rg -c "logger\.error\("` **[belegt]**; `frontend/src/lib/api-helpers.server.ts:396-402` und `:416-428`; `frontend/src/instrumentation.ts:16` |
| 5 | **Client-Logs gehen außerhalb des Tenant-Portals verloren.** Eltern: 401. Schul-Portal: 404, schon im Proxy. Dazu kommen Operator-, Read-only- und nicht angemeldete Sitzungen. Im Browser puffert der Logger 5 s und sendet beim Schließen oder Absturz nichts nach. | `frontend/src/app/api/logs/route.ts:62-74`; `frontend/src/proxy.ts:423-433`; `frontend/src/lib/logger.ts:228-229, 316-356` **[belegt]** |
| 6 | **Nur ein einziger Error-Boundary meldet an Sentry.** Es gibt keine `error.tsx`, nur `global-error.tsx`. `SSEErrorBoundary` schreibt nur `error.message` ohne Stack in den Logger. | `fd error.tsx` ohne Treffer; `frontend/src/app/global-error.tsx:41-43`; `frontend/src/components/sse/SSEErrorBoundary.tsx:30-35` **[belegt]** |
| 7 | **Backend-Events tragen keinen Kontext:** kein Tag für Schule, Portal oder Route, kein User, kein Release. Ein Teil der 5xx-Pfade umgeht die Sentry-Erfassung. | Keine `SetTag`/`SetUser`/`ConfigureScope` im Backend; `http.Error(..., 500)` u. a. in `backend/modules/delivery/http/sse/api.go:71,109`, `backend/api/common/tenant_middleware.go:246,262`, `backend/api/request_id.go:17` **[belegt]** |
| 8 | **Hintergrundjobs melden nur Panics.** Scheitert ein Worker-Job oder ein Befehl darin, entstehen nur ein Log und eine Metrik. Der Attendance-Sync verwandelt Panics in Fehler, ohne an Sentry zu melden. | `backend/observability/tracer.go:54-69`; `backend/api/server.go:283-285`; `backend/services/scheduler/scheduler.go:703-725`; `backend/modules/timetable/legacy/timetableplanning/attendance_sync_service.go:81-88` (und sieben weitere `recover()` dort) **[belegt]** |
| 9 | **Weder slog-Anbindung noch Tracing.** Das Frontend setzt `tracesSampleRate: 0`, das Backend hat kein `EnableTracing`, und `sentry-go/slog` ist nicht eingebunden. | `frontend/src/sentry.*.config.ts:12`; `backend/cmd/serve.go:46-52`; `backend/go.mod:9` (nur `sentry-go`) **[belegt]** |
| 10 | **PyrePortal hat keinerlei Fernmeldung von Fehlern.** Logs gehen auf dem GKT-Gerät an `SYSTEM.log2`, sonst an die Konsole. Es gibt keinen globalen `onerror`-Handler. | `PyrePortal/src/platform/gkt/index.ts:87-94`; `PyrePortal/src/utils/errorBoundary.tsx:32-40`; `rg sentry` ohne Treffer **[belegt]** |

Querschnitt: Für harte Browser-Abstürze (WebKit/PWA) gibt es heute keinen
Mechanismus. Die CSP hat kein `report-uri`/`report-to`
(`frontend/src/proxy.ts:73-82`). Was Sentry dabei überhaupt kann, steht im
Abschnitt „Harte Abstürze“.

## Installierte Versionen

| Paket | Version | Quelle |
|---|---|---|
| `@sentry/nextjs` | 10.70.0 (`^10.70.0`) | `frontend/package.json:46`, `frontend/pnpm-lock.yaml:2491` |
| `@sentry/cli` (Build-Plugin) | 2.58.6, plattformspezifisch als `@sentry/cli-linux-x64` u. a. | `frontend/pnpm-lock.yaml:2450, 2474` |
| `@sentry/webpack-plugin` / `bundler-plugin-core` | 5.4.0 / 5.3.0 | `frontend/pnpm-lock.yaml:2411, 2552` |
| `github.com/getsentry/sentry-go` | v0.49.0 | `backend/go.mod:9` |
| Next.js | `^16.3.3` | `frontend/package.json:55` |
| PyrePortal | kein Sentry-Paket | `PyrePortal/package.json` |

Laut GitHub-Releases ist sentry-go v0.49.0 (26.08.2026) aktuell. In der
JavaScript-Linie gibt es v10.75.3 und ganz frisch v11.0.0
(https://github.com/getsentry/sentry-go/releases,
https://github.com/getsentry/sentry-javascript/releases) **[Doku, v11 nicht
selbst geprüft]**.

## Ist-Stand pro Oberfläche

### Backend (Go, Projekt `go`)

- **Init:** Nur im Befehl `serve` (`backend/cmd/serve.go:44-58`). Die DSN kommt
  aus `SENTRY_DSN`, das Environment aus `SENTRY_ENVIRONMENT`, beide über viper
  (`serve.go:150`). Ist `SENTRY_DSN` gesetzt, muss auch `SENTRY_ENVIRONMENT`
  gesetzt sein, sonst startet der Server nicht (`serve.go:179-180`, Test
  `serve_test.go:75-84`). Nicht gesetzt sind `Release`, `SampleRate` (Default
  1.0), `EnableTracing`, `TracesSampleRate`, `EnableLogs` und `SendDefaultPII`
  (Default false). **[belegt]**
- **BeforeSend:** `scrubSentryEvent` (`serve.go:77-113`) schwärzt Feed-Tokens
  in Message, Transaction, Breadcrumbs und URL, leert `Request.Data` und
  filtert die Header `Authorization`, `X-Staff-PIN`, `X-Staff-Id`,
  `X-Staff-Auth-PIN` und `X-Device-Key`. **[belegt]**
- **Middleware:** `sentryhttp.New(sentryhttp.Options{Repanic: true})` in
  `backend/api/base.go:993-994`, direkt nach `middleware.Recoverer`
  (`base.go:992`). Chi schachtelt in `Use`-Reihenfolge, also fängt Sentry den
  Panic zuerst, meldet ihn und wirft ihn erneut, danach antwortet
  `Recoverer` mit 500 **[abgeleitet aus chi-Semantik]**. Laut Quelltext klont
  die Middleware pro Request einen Hub und ruft `ContinueTrace` mit
  `sentry-trace`/`baggage` auf
  (https://github.com/getsentry/sentry-go/blob/master/http/sentryhttp.go)
  **[Doku]**.
- **5xx:** `common.RenderError` (`backend/api/common/errors.go:21-36`) meldet
  bei Status ≥ 500 `errResp.Err` an den Request-Hub. Das ist der Hauptpfad
  mit 2.273 Aufrufen. `common.RespondWithError`
  (`backend/api/common/response.go:53-63`) meldet `errors.New(errorMsg)`.
  Dort entstehen Typ und Stack also an der Meldestelle, nicht an der
  Fehlerquelle. **[belegt]** Ohne Sentry-Meldung bleiben u. a.:
  `http.Error(..., 500)` in den SSE-Handlern
  (`modules/delivery/http/sse/api.go:71,109`, `school_api.go:103,121`,
  `parent_api.go:59`), in `api/common/tenant_middleware.go:246,262`,
  `api/request_id.go:17`, `api/base.go:2107` und
  `modules/dataimport/inbound/templates.go:22`, außerdem `render.Status(r, 500)`
  in `api/enrollment/legal_document_handlers.go:49,81,94` und
  `modules/identityaccess/inbound/me/profile.go:134`. **[belegt]**
- **Panics in Goroutinen:** Gemeldet werden sie über
  `sentry.CurrentHub().Recover` plus `Flush(2s)` in
  `services/scheduler/scheduler.go:622-630, 664-672, 999-1007`,
  `email/dispatcher.go:143-146, 242-246` und
  `services/mfa_composition.go:300-305`. Weil `CurrentHub()` genutzt wird und
  kein geklonter Hub, fehlt jeder Job-Kontext. Nach einem Panic endet die
  Polling-Goroutine des Schedulers und läuft bis zum Neustart nicht mehr
  **[abgeleitet aus `scheduler.go:619-654`]**.
- **Ohne Sentry-Meldung:** `recover()` in `tenant/runtime.go:285-291`, das
  erneut panict und dann über die Middleware gemeldet wird **[abgeleitet]**.
  Außerdem `scheduler.go:710-716`, das erneut panict und wohl von
  `runMinutePolling`/`runIntervalPolling` gemeldet wird **[abgeleitet]**.
  Schließlich 8× `attendance_sync_service.go`, wo ein Panic zum Fehler
  wird und nur im Log steht. **[belegt]**
- **Worker-Fehler ohne Panic:** `Tracer.Failure` schreibt eine ERROR-Zeile
  „runtime operation failed“ und eine Prometheus-Metrik
  (`backend/observability/tracer.go:54-69`). Der Fehlertext bleibt bewusst auf
  Debug-Level. An Sentry geht nichts. **[belegt]**
- **slog:** `applog` hat keinen Sentry-Handler. Request-Logs laufen über
  `slog-chi` (`base.go:972-991`). **[belegt]**
- **Flush:** `defer sentry.Flush(2 * time.Second)` in `serve.go:56`. Das
  greift nur bei normalem Rückkehren aus `RunE`. **[belegt]**
- **Weitere Befehle** (`migrate`, `seed`, `cleanup` …) initialisieren Sentry
  nicht. **[belegt, nur `serve.go` ruft `sentry.Init`]**

### Frontend (Next.js, Projekt `javascript-nextjs`)

Tenant-, Eltern-, Schul- und Operator-Portal laufen im selben Next.js-Build
und mit derselben Sentry-Konfiguration.

- **Init:** `instrumentation-client.ts:2` lädt `sentry.client.config.ts`.
  `instrumentation.ts:3-13` lädt die Server- und Edge-Konfiguration,
  `instrumentation.ts:16` setzt `onRequestError = Sentry.captureRequestError`,
  und `instrumentation-client.ts:4` setzt `onRouterTransitionStart`.
  **[belegt]**
- **Optionen, in allen drei Configs gleich:** DSN aus
  `NEXT_PUBLIC_SENTRY_DSN`, Environment aus `NEXT_PUBLIC_SENTRY_ENVIRONMENT`,
  `tracesSampleRate: 0`, `beforeSend: scrubEvent`. Nur im Client zusätzlich
  `replaysSessionSampleRate: 0` und `replaysOnErrorSampleRate: 0`. Die Replay
  Integration wird nicht hinzugefügt. Ohne DSN läuft keine Initialisierung.
  Nicht gesetzt sind `release`, `dist`, `sendDefaultPii`, `ignoreErrors`,
  `denyUrls`, `beforeBreadcrumb`, `integrations`, `enableLogs` und
  `tracePropagationTargets`.
  (`frontend/src/sentry.client.config.ts:4-18`, `sentry.server.config.ts:4-16`,
  `sentry.edge.config.ts:4-16`) **[belegt]**
- **Scrubbing** (`frontend/src/sentry.shared.ts:27-74`): verwirft den bekannten
  RSC-Router-State-Fehler, löscht die Header `Authorization`/`Cookie`, leert
  Cookies, entfernt `ip_address`, `email` und `username` vom User und
  schwärzt Feed-Tokens in URL, Transaction, `request_path` und Breadcrumbs.
  **[belegt]**
- **Build:** `withSentryConfig` mit `silent: true`,
  `sourcemaps.disable: true` und `tunnelRoute: "/monitoring"`
  (`frontend/next.config.js:85-93`). Der Tunnel ist auf allen Hosts
  freigeschaltet (`proxy.ts:224-226, 335-337, 426, 711`). Die CSP lässt nur
  `connect-src 'self'` zu (`proxy.ts:78`), deshalb braucht es den Tunnel.
  `@sentry/cli` steht in `ignoredBuiltDependencies`
  (`frontend/pnpm-workspace.yaml:45`), die Plattform-Binaries kommen als
  optionale Pakete (`pnpm-lock.yaml:2432-2450`). **[belegt]** `next build` ohne
  Flag baut unter Next 16 mit Turbopack. Das installierte SDK bringt dafür
  `handleRunAfterProductionCompile.js` und `config/turbopack/` mit
  (`node_modules/@sentry/nextjs/build/cjs/config/`) **[belegt]**. Ob der
  Upload mit #3585 unter Turbopack im Docker-Build tatsächlich läuft, ist
  **nicht verifiziert** (so steht es auch im PR).
- **Error-Boundaries:** Es gibt keine `error.tsx`. `app/global-error.tsx:41-43`
  ruft `Sentry.captureException(error)` auf. Das ist neben
  `captureRequestError` der einzige explizite Sentry-Aufruf im Frontend.
  `SSEErrorBoundary.tsx:30-35` loggt nur. Unbehandelte Fehler im Browser
  erfasst das SDK über seine globalen Handler **[Doku-Standardverhalten,
  nicht im Code konfiguriert]**. **[belegt]**
- **Logger:** Auf dem Server schreibt `logger` JSON nach stdout
  (`frontend/src/lib/logger.ts:217`). Im Client sammelt er Einträge und
  schickt sie alle 5 s oder ab 10 Einträgen an `/api/logs`
  (`logger.ts:228-229, 296, 316-356`). In Produktion greift er ab Level
  `info` (`logger.ts:131-143`). Scheitert der Versand, bleibt nur ein
  `console.warn`, und der Batch ist weg (`logger.ts:349-356`). Einen Flush
  bei `pagehide`/`visibilitychange` gibt es nicht. **[belegt]**
- **Kontext:** Kein `portal`-Tag, keine Schule, kein User. Mit #3585 kommt ein
  `portal`-Tag über `initialScope`, abgeleitet aus dem Host. **[belegt]**
- **PostHog:** `capture_exceptions: false`
  (`frontend/src/lib/posthog-client.ts:29`). Es gibt also keinen zweiten
  Kanal für Fehler. **[belegt]**

| Portal | Unbehandelte Fehler → Sentry | `logger.*` → Loki | Besonderheit |
|---|---|---|---|
| Tenant | ja | ja, nur bei nicht schreibgeschützter Tenant-Session (`route.ts:67-74`) | |
| Eltern | ja | **nein**: 401, da `validateSessionToken(..., "tenant")` (`route.ts:67`). Mit #3585 über `/api/parent/logs` | Anlass von #3586 |
| Schul-Portal | ja | **nein**: der Proxy antwortet auf `/api/logs` mit 404 (`proxy.ts:423-433`). Auch mit #3585 nicht gelöst | |
| Operator | ja | vermutlich nein: `/api/` wird durchgereicht (`proxy.ts:224`), aber die Route verlangt eine Tenant-Session **[abgeleitet]** | |
| Nicht angemeldet (Login, Einladung) | ja | nein: 401 (`route.ts:62-65`) | |

### PyrePortal (Kiosk)

- Kein Sentry, kein anderer Dienst für Fehlermeldungen (`rg -i
  "sentry|bugsnag|rollbar|datadog|posthog|logrocket"` ohne Treffer). **[belegt]**
- Der Logger schreibt je nach Plattform: auf GKT an `SYSTEM.log2('PyrePortal',
  entry)`, sonst an die Konsole (`src/platform/gkt/index.ts:87-94`,
  `wedge/index.ts:93-96`, `browser/index.ts:71-74`). Ab `INFO` wird
  persistiert (`src/utils/logger.ts:43-44`). **[belegt]**
- `ErrorBoundary` umschließt die App (`src/App.tsx:58, 184`) und loggt nur
  lokal (`src/utils/errorBoundary.tsx:32-40`). Es gibt keine Handler für
  `window.onerror` oder `unhandledrejection`. **[belegt]**
- Die einzige Rückmeldung ans Backend ist `/api/iot/ping`
  (`src/services/api.ts:285-294`), ohne Fehlerinhalt. **[belegt]**
- Folge: Fehler auf einem Gerät sieht nur, wer die Logs dieses Geräts
  ausliest. **[abgeleitet]**

### Demo

- **Frontend:** DSN aus `secrets.DEMO_NEXT_PUBLIC_SENTRY_DSN`
  (`build.yml:378`). Das Secret existiert nicht, also bleibt der DSN leer und
  Sentry aus. **[belegt per `gh secret list`]** Genau so beschreibt es
  `docs/operations/demo-environment.md:109-112` („Sentry and Web Push are
  disabled“).
- **Backend:** `scripts/create-demo-env.py:43-44` legt `SENTRY_DSN=""` und
  `SENTRY_ENVIRONMENT=demo` an. Ob der SOPS-Wert seitdem geändert wurde, ist
  **unbekannt**.
- Mit #3585 bekäme ein Demo-Build `SENTRY_PROJECT=javascript-nextjs` und würde
  Sourcemaps in das Prod-Projekt hochladen, obwohl Demo dort keine Events
  sendet **[abgeleitet aus dem PR-Diff `build.yml`]**.

## Environments & Releases

| Wert | Frontend | Backend |
|---|---|---|
| DSN | Build-Arg `NEXT_PUBLIC_SENTRY_DSN` aus dem Repo-Secret (Prod und Staging teilen sich einen DSN), Demo aus dem nicht vorhandenen `DEMO_NEXT_PUBLIC_SENTRY_DSN` (`build.yml:378`) | `SENTRY_DSN` aus `environments/<env>.sops.env` über `environments/<env>.compose.yml` (production `:109`, staging `:90`, demo `:95`) |
| Environment | Build-Arg `NEXT_PUBLIC_SENTRY_ENVIRONMENT = needs.check-environment.outputs.environment` (`build.yml:379`), also `production` (Push auf `main`), `staging` (Push auf `development`) oder `demo` (manuell) (`build.yml:56-80`) | `SENTRY_ENVIRONMENT` aus SOPS, **unabhängig von `APP_ENV`**. Werte nicht lesbar. `.env.example:84` empfiehlt `development`, `staging` oder `production`. |
| Release | nicht gesetzt | nicht gesetzt |
| dist | nicht gesetzt | nicht gesetzt |

- **Build-Zeit statt Laufzeit:** `NEXT_PUBLIC_*` wird beim Build in Client-
  und Server-Bundle eingesetzt. Die gleichnamigen Compose/SOPS-Keys sind für
  das Frontend wirkungslos (`build.yml:363-369`, `production.compose.yml:163-164`).
  **[belegt]**
- **Frontend-Release ohne #3585:** `withSentryConfig` löst den Namen über
  `release.name ?? getSentryRelease() ?? git rev-parse HEAD` auf
  (`node_modules/@sentry/nextjs/build/cjs/config/withSentryConfig/getFinalConfigObjectUtils.js:15-17`).
  Im Docker-Build (`context: ./frontend`, `build.yml:358`) fehlen
  `SENTRY_RELEASE`, `GITHUB_SHA` und `.git`. Deshalb ist vermutlich kein
  Release gesetzt **[abgeleitet]**. Mit #3585 kommt
  `SENTRY_RELEASE=${{ github.sha }}` als Build-Arg dazu.
- **Backend-Release:** sentry-go v0.49.0 fällt zurück auf `SENTRY_RELEASE`,
  dann CI-Variablen (u. a. `GITHUB_SHA`), dann `debug.ReadBuildInfo()` mit
  `vcs.revision`, dann `git describe`
  (`~/go/pkg/mod/github.com/getsentry/sentry-go@v0.49.0/util.go:40-85`).
  Keine dieser Quellen ist im Container vorhanden: keine Variable in den
  Compose-Dateien, `.git` nicht im Build-Kontext `./backend`, kein `git` im
  Laufzeit-Image (`backend/Dockerfile:14-18, 21-22`). **[abgeleitet]**
- **Risiko gleicher oder fehlender Environments:**
  - Prod und Staging senden im Frontend in dasselbe Projekt und unterscheiden
    sich nur über `environment` **[belegt]**.
  - Im Backend hängt die Trennung allein am SOPS-Wert `SENTRY_ENVIRONMENT`.
    Ein Kopierfehler würde Staging als `production` melden, und keine Prüfung
    vergleicht ihn mit `APP_ENV` **[abgeleitet; Werte nicht verifiziert]**.
  - Ein leeres Environment ist im Backend ausgeschlossen (`serve.go:179-180`).
    Im Frontend kann es nur leer sein, wenn der DSN gesetzt ist, der
    Workflow-Output aber nicht. Das ist nach `build.yml:56-80` nicht möglich.
    **[belegt]**
  - Lokal: `.env.example:173-174` lässt beide Frontend-Werte leer, also läuft
    lokal kein Frontend-Sentry **[belegt]**.
- **Env-Var-Namen:**
  - `SENTRY_DSN` und `SENTRY_ENVIRONMENT`: Backend, gelesen in
    `serve.go:150`; geführt in `environments/runtime-env-allowlist.json:80-81`,
    `scripts/check-runtime-env.py:19`, `backend/dev.env.example:70-72` und
    `.env.example:83-85`.
  - `NEXT_PUBLIC_SENTRY_DSN` und `NEXT_PUBLIC_SENTRY_ENVIRONMENT`: Frontend,
    gelesen in `frontend/src/env.js:43-44`, validiert in
    `lib/env-validation.js:46-47, 90-92`, außerdem in
    `runtime-env-allowlist.json:32-33` und `Dockerfile.prod:43-47`.
  - GitHub-Secrets: `NEXT_PUBLIC_SENTRY_DSN`, `SENTRY_AUTH_TOKEN` (angelegt am
    23.09.2026). GitHub-Variablen: `SENTRY_ORG`, `SENTRY_PROJECT`.
  - Nur mit #3585 gelesen werden `SENTRY_AUTH_TOKEN`, `SENTRY_ORG`,
    `SENTRY_PROJECT` und `SENTRY_RELEASE`.

## Verschluckte Fehler

| Ort | Verhalten | Folge |
|---|---|---|
| `frontend/src/app/api/logs/route.ts:62-74` | Nur Tenant-Sessions, die nicht schreibgeschützt sind. Alles andere bekommt 401. | Client-Logs aus Eltern-, Operator- und Login-Seiten sowie aus schreibgeschützten Sitzungen fehlen in Loki. |
| `frontend/src/proxy.ts:423-433` | Der Schul-Host blockt `/api/*` außer `/api/school/*` mit 404. | Client-Logs aus dem Schul-Portal fehlen, auch mit #3585. |
| `frontend/src/lib/logger.ts:326-356` | Der Batch wird vor dem Senden geleert. Bei Fehlern bleibt nur `console.warn`. Beim Verlassen der Seite wird nicht geflusht. | Log-Einträge der letzten bis zu 5 s vor Absturz oder Navigation gehen verloren. |
| `frontend/src/lib/api-helpers.server.ts:382-402, 416-428` | BFF-Fehler: bei 5xx oder unbekanntem Fehler `logger.error` und 500-JSON, kein `throw` | Nur in Loki, nicht in Sentry (`onRequestError` greift nicht). Mit #3585 als Sentry-Message. |
| 824× `logger.error` in 395 Dateien (Frontend) | Nur Loki | Fehler, die behandelt und angezeigt werden, sieht Sentry nicht. Mit #3585 werden sie Sentry-Events, außer `sse connection error` und `parent login failed`. |
| `frontend/src/components/sse/SSEErrorBoundary.tsx:30-35` | `logger.error` nur mit `message` | Kein Stack, kein Sentry-Event |
| 80 leere `catch {}` in 66 Dateien, 36 × `.catch(() => {})` in 18 Dateien (Frontend) | Bewusst ignoriert | Stichprobe (`parent-messages-api.ts:69`, `pwa-usage-api.ts:41,59`, `pwa-install-prompt.ts:311,321`): alle kommentiert und fachlich begründet. Vollständig geprüft ist das **nicht**. |
| `backend/observability/tracer.go:54-69` | Worker- und Request-Fehler: ERROR-Zeile ohne Fehlertext plus Metrik | Nicht in Sentry. Die Details stehen nur auf Debug-Level. |
| `backend/modules/timetable/legacy/timetableplanning/attendance_sync_service.go:81-88` (+7) | `recover()` wird zu Fehler plus Log mit Stack | Nicht in Sentry. Landet der Fehler später in einer 5xx-Antwort, erscheint er ohne den ursprünglichen Panic-Stack. |
| `backend/services/scheduler/scheduler.go:619-654` | Panic wird gemeldet, danach endet die Goroutine | Der Job läuft bis zum Neustart nicht mehr. Sentry zeigt den Panic nur einmal. **[abgeleitet]** |
| `http.Error`/`render.Status` 500 in SSE-, Tenant-Middleware-, Request-ID-, Legal-Document- und Profil-Handlern (siehe Backend) | Kein `RenderError` | 5xx ohne Sentry-Event |
| `backend/api/common/response.go:59-62` | `errors.New(errorMsg)` an der Meldestelle | Sentry gruppiert nach dem Aufrufstack von `RespondWithError`, nicht nach der Ursache **[abgeleitet]** |
| `PyrePortal/src/utils/errorBoundary.tsx:32-40`, `platform/gkt/index.ts:87-94` | Nur lokales Log auf dem Gerät | Aus der Ferne unsichtbar |

## Live-Daten Sentry: nicht zugänglich

- `sentry-cli` ist nicht installiert (`which sentry-cli` ohne Treffer). Es gibt
  weder `~/.sentryclirc` noch eine `SENTRY_*`-Variable in der Shell.
  `frontend/node_modules/.bin/sentry-cli` fehlt, passend zu
  `ignoredBuiltDependencies`.
- Ein Aufruf der Org-URL mit `agent-browser` (neue Session) leitete auf
  `https://sentry.io/auth/login/ganztagshelden/` um. Es gab keine gespeicherte
  Sentry-Anmeldung, und ein Login-Versuch fand nicht statt.
- Die DSN-Werte liegen verschlüsselt in SOPS bzw. als GitHub-Secret und
  lokal nicht in `.env` / `backend/dev.env` / `frontend/.env.local`.
  Deshalb ist auch die **Datenregion (US oder EU) unbekannt**.
- Offen bleiben dadurch: Projektliste, tatsächliche Environment-Namen (vor
  allem im Backend), ob Releases ankommen, Event-Mengen und Typen, Alerts,
  Slack-Integration, Spike Protection, Inbound Filters,
  Datenschutzeinstellungen und die Datenregion. #3586 nennt als einzige Zahl
  „Frontend-Projekt: 6 Errors“.

## Sentry-Fähigkeiten laut Doku, nach offener Frage aus #3586

Die Quellen sind docs.sentry.io und getsentry-Repos. Punkte, die die
Recherche nicht wörtlich bestätigen konnte, sind markiert.

### Was soll Sentry beantworten, was bleibt bei Loki?

- **Sentry Logs:** Im JS-SDK über `enableLogs: true` und
  `Sentry.logger.*` (https://docs.sentry.io/platforms/javascript/guides/nextjs/logs/),
  in Go über `EnableLogs: true` ab sentry-go 0.33
  (https://docs.sentry.io/platforms/go/logs/). Enthalten sind 5 GB pro Monat,
  darüber 0,50 $/GB nur als Pay-as-you-go
  (https://docs.sentry.io/pricing/). Ein Vergleich mit Loki oder eine
  Anleitung zu „erwarteten Fehlern“ fand sich auf docs.sentry.io nicht.
- **`sentry-go/slog`** (ab 0.34) leitet slog-Records **in das Logs-Produkt**,
  nicht in Events oder Breadcrumbs. Bestätigte Optionen: `LogLevel`,
  `AttrFromContext`, `AddSource`, `ReplaceAttr`
  (https://docs.sentry.io/platforms/go/logs/slog/,
  https://github.com/getsentry/sentry-go/tree/master/slog). Eine eigene Option,
  die slog-Fehler als Events meldet, wurde **nicht gefunden**.
- Im Repo gilt ADR 0006 mit #2501/#2512: Sentry bekommt Serverfehler,
  Netzabbruch und Absturz, gruppiert per Fingerprint über den Fehlercode.
  Alles Erwartbare geht nach Loki (`docs/adr/0006-fehleridentitaet-ist-der-code.md:46, 99`).
  #3585 schickt vorerst jeden `logger.error` an Sentry, und der Kommentar in
  #2512 hält fest, das später auf die Klassen Serverfehler, Netzabbruch und
  Absturz einzuschränken.
- Schon vorhanden: Grafana-Alarme auf Loki, nämlich „Server error spike“ bei
  mehr als 10 ERROR-Zeilen in 5 min und „Postgres errors“
  (`monitoring/runbooks/server-errors.md:1-8`,
  `monitoring/grafana/provisioning/alerting/server-errors.yml`).

### Wer ist betroffen: Konto-ID und Schule am Event?

- In Go gibt es `scope.SetTag`, `scope.SetUser(sentry.User{ID: ...})`
  (https://docs.sentry.io/platforms/go/enriching-events/scopes/). Für
  Goroutinen empfiehlt die Doku `sentry.CurrentHub().Clone()` und die
  Methoden des lokalen Hubs (https://docs.sentry.io/platforms/go/concurrency/).
- `SendDefaultPII` ist in Go per Default aus und wird durch die feinere Option
  `DataCollection` abgelöst (https://docs.sentry.io/platforms/go/configuration/options/).
  Im JS-SDK schickt v9 ohne `sendDefaultPii: true` keine IP mehr
  (https://docs.sentry.io/platforms/javascript/guides/nextjs/migration/v8-to-v9/).
  Die genaue Formulierung des v10-Defaults ist **nicht wörtlich bestätigt**.
- Serverseitig ist der Data Scrubber per Default an und erkennt Namen wie
  password, secret, token und Kreditkartenmuster. Advanced Data Scrubbing
  erlaubt eigene Pfadregeln (https://docs.sentry.io/security-legal-pii/scrubbing/server-side-scrubbing/).
  Eigene Felder wie `student_name` erfasst er nur mit einer ausdrücklichen
  Regel, wie #2501/T5 schon festhält.
- „Prevent Storing of IP Addresses“ ist ein eigener Projektschalter
  (https://docs.sentry.io/platforms/go/enriching-events/identify-user/).
- **EU-Datenhaltung:** Frankfurt oder USA wird beim Anlegen der Org gewählt
  und lässt sich danach nicht mehr ändern. EU-Orgs senden an
  `o<nr>.ingest.de.sentry.io` (https://docs.sentry.io/organization/data-storage-location/).
  Der Name `ganztagshelden.sentry.io` sagt über die Region nichts. Dazu gibt
  es den DPA unter https://sentry.io/legal/dpa/ und die Subprozessoren unter
  https://sentry.io/legal/subprocessors/.

### Session Replay und Datenschutz

- Defaults: `maskAllText: true`, `maskAllInputs: true`, `blockAllMedia: true`.
  Netzwerkdetails (Bodies, Header) sind opt-in (`networkDetailAllowUrls: []`).
  Die Maskierung passiert im Browser vor dem Senden.
  (https://docs.sentry.io/platforms/javascript/session-replay/privacy/,
  https://docs.sentry.io/platforms/javascript/session-replay/configuration/,
  https://docs.sentry.io/security-legal-pii/scrubbing/protecting-user-privacy/)
- `replaysOnErrorSampleRate` puffert etwa 1 min vor einem Fehler. Sentry
  empfiehlt, je nach Kundentyp eine Einwilligung zu prüfen (gleiche Quelle).
- Heute gilt: beide Raten stehen auf 0, und die Integration fehlt
  (`sentry.client.config.ts:13-14`).

### Harte Browser- und App-Abstürze (WebKit/PWA)

- **Kann Sentry nicht:** Ein abgestürzter Tab führt kein JS mehr aus. Eine
  Doku-Stelle, die das ausdrücklich sagt, fand sich nicht, es folgt aber aus
  der Architektur **[abgeleitet]**.
- **Release Health:** Browser-Sessions kennen die Zustände healthy, errored,
  crashed und abnormal. „Abnormal“ entsteht nur als Schluss beim nächsten
  Start, wenn eine saubere Beendigung fehlt
  (https://docs.sentry.io/product/releases/health/). Ob das Browser-SDK einen
  abgestürzten Tab beim Neuladen als abnormal meldet, ist **nicht
  dokumentiert**.
- **Security Policy Reporting:** Sentry nimmt CSP-Reports über
  `report-uri`/`report-to`/`Reporting-Endpoints` an, aber **nur
  CSP-Verstöße**, keine Crash-Reports der Reporting API
  (https://docs.sentry.io/platforms/javascript/guides/express/security-policy-reporting/).
  NEL wird in der Doku nicht erwähnt.
- **Offline-Transport** (`makeBrowserOfflineTransport`): IndexedDB-Queue, die
  bei Netzabbruch puffert und beim nächsten Online-Zustand nachsendet
  (https://docs.sentry.io/platforms/javascript/configuration/transports/).
  Gegen Abstürze hilft sie nicht, es sei denn, das Event war vorher schon
  erfasst.
- Für uns heißt das: Einen harten Absturz sieht Sentry höchstens indirekt,
  über Breadcrumbs und Events **vor** dem Absturz (mit #3585) oder über einen
  Neustart-Marker, den wir selbst bauen **[abgeleitet]**.

### Backend: Release, Tags, Hintergrundjobs, slog

- sentry-go liest `SENTRY_RELEASE` und `SENTRY_ENVIRONMENT` selbst
  (https://docs.sentry.io/platforms/go/configuration/options/; siehe auch
  `util.go:40-85`, `client.go:356-366` im Modul-Cache).
- Für Panics in Goroutinen gibt es `sentry.Recover()` /
  `sentry.RecoverWithContext(ctx)` (https://docs.sentry.io/platforms/go/usage/panics/).
  `Flush` wartet höchstens bis zum Timeout
  (https://docs.sentry.io/platforms/go/configuration/draining/).
- `sentryhttp` meldet von sich aus **nur Panics**, keine 5xx
  (https://github.com/getsentry/sentry-go/blob/master/http/sentryhttp.go).
  Deshalb braucht es `RenderError`.
- v0.49.0 hat die Rahmung von Panic-Stacks geändert (Recover-Frames werden
  übersprungen), was die Gruppierung beeinflussen kann
  (https://github.com/getsentry/sentry-go/releases).
- Release-Namen: `paket@version+build` oder Commit-SHA. Releases gelten
  org-weit, ein Projektpräfix wird empfohlen
  (https://docs.sentry.io/product/releases/naming-releases/).

### Tracing Frontend → Backend und Performance

- Über die Header `sentry-trace` und `baggage`
  (https://docs.sentry.io/platforms/javascript/guides/nextjs/tracing/trace-propagation/).
  Im Browser propagiert das SDK per Default nur an dieselbe Origin. Für den
  Server gibt es zwei Aussagen: „all outgoing requests“, Fremd-Origins aber
  per `tracePropagationTargets`
  (https://docs.sentry.io/platforms/javascript/guides/nextjs/configuration/options/).
  Die Doku ist hier nicht eindeutig. Am echten BFF-Request zu `API_URL`
  prüfen.
- `sentryhttp` setzt eingehende Traces automatisch über `ContinueTrace` fort.
  Trace-Propagation ohne Performance-Sampling ist umgesetzt
  (https://github.com/getsentry/sentry-javascript/issues/8352). Ab welcher
  `@sentry/nextjs`-Version, ist **nicht festgestellt**.
- In Go stehen `EnableTracing` und `TracesSampleRate` per Default auf
  false bzw. 0 (Go-Optionsseite oben).

### Abdeckung: Schul-Portal, PyrePortal, Demo und Sourcemaps

- Debug-IDs sind der empfohlene Weg für Sourcemaps. Seit v9 werden Maps
  standardmäßig erzeugt und nach dem Upload gelöscht, ein Release ist für die
  Zuordnung nicht mehr nötig, wohl aber für Regressionen und Deploys
  (https://docs.sentry.io/platforms/javascript/sourcemaps/troubleshooting_js/debug-ids/,
  https://docs.sentry.io/platforms/javascript/guides/nextjs/migration/v8-to-v9/).
- Build-Optionen: `org`, `project`, `authToken`, `sourcemaps.disable`,
  `sourcemaps.deleteSourcemapsAfterUpload`, `release.name`,
  `widenClientFileUpload` (https://docs.sentry.io/platforms/javascript/guides/nextjs/configuration/build/).
  Für den Token genügt der Scope `org:ci` (https://docs.sentry.io/api/permissions/).
- **Ein Build, mehrere Projekte:** Innerhalb derselben Org kann das Plugin
  mehrere Projekte bedienen. Mehrere Orgs gehen nur über separate
  `sentry-cli`-Uploads
  (https://github.com/getsentry/sentry-javascript-bundler-plugins/issues/868,
  **nur Issue-Quelle**).
- Für PyrePortal (React/Vite) gibt es kein Doku-Detail aus dieser Recherche.
  Offen bleibt auch die CSP im Kiosk.

### Auswertung im Alltag: Alerts, Slack, Quota, Rauschen

- **Issue Alerts** feuern pro Ereignis (neues Issue, Regression, Eskalation).
  **Metric Alerts** feuern auf aggregierte Werte. Filter pro Environment sind
  möglich, Default ist „alle“
  (https://docs.sentry.io/product/alerts-notifications/issue-alerts/,
  https://docs.sentry.io/product/alerts/create-alerts/issue-alert-config/).
- Slack: unter Settings → Integrations → Slack, danach als Aktion in einer
  Alert-Regel (https://docs.sentry.io/product/integrations/notification-incidents/slack/).
- Die Priorität hat die Stufen High, Medium und Low. Eskalation hebt sie an
  (https://docs.sentry.io/product/issues/issue-priority/).
- **Spike Protection:** Ein dynamisches Stundenlimit auf Basis der
  Vergangenheit verwirft Events über der Schwelle. Benachrichtigungen dafür
  sind per Default aus (https://docs.sentry.io/product/accounts/quotas/spike-protection/).
  Quotas lassen sich pro Org, Projekt und DSN setzen, je Datenkategorie
  (https://docs.sentry.io/product/accounts/quotas/).
- **Inbound Filters** (werden nicht auf die Quota angerechnet):
  Browser-Erweiterungen, Legacy-Browser, localhost, Crawler,
  React-Hydration-Fehler, ChunkLoadError, bestimmte IPs, Meldungen und
  Releases (https://docs.sentry.io/concepts/data-management/filtering/).
  SDK-seitig gibt es `ignoreErrors` und `denyUrls`. `denyUrls` prüft
  Stack-Frames, nicht die Seiten-URL
  (https://docs.sentry.io/platforms/javascript/configuration/filtering/).
- Environments lassen sich nicht löschen, nur ausblenden, und ausgeblendete
  zählen weiter zur Quota (https://docs.sentry.io/concepts/key-terms/environments/).

## Offene Punkte und Unsicherheiten

1. **Live-Zustand unbekannt:** Environments, Releases, Alerts, Slack,
   Quotas, Inbound Filters, Datenschutzeinstellungen und Datenregion. Um das
   zu klären, braucht jemand Zugang (Login im Browser oder ein Token mit
   Leserechten).
2. **SOPS-Werte ungeprüft:** `SENTRY_ENVIRONMENT` und `SENTRY_DSN` im Backend
   für staging, production und demo. Frage: Meldet Staging als `staging`,
   und ist Demo wirklich leer?
3. **Release tatsächlich leer?** Das ist aus Code und Build-Kontext
   abgeleitet. Bestätigen lässt es sich nur in Sentry.
4. **#3585, ungeprüft:** Läuft der Upload unter Turbopack im Docker-Build
   mit BuildKit-Secret? Reichen die optionalen `@sentry/cli-linux-*`-Pakete
   trotz `ignoredBuiltDependencies`? Wie hoch ist das Volumen mit etwa 55
   Events pro Tag (Schätzung im PR)?
5. **Operator-Portal-Logs:** Das 401 ist abgeleitet, nicht beobachtet.
6. **Trace-Propagation im BFF:** Ob Node-Fetches zu `API_URL` heute schon
   `sentry-trace` tragen, ist nicht geprüft. Die Doku ist uneinheitlich.
7. **Browser-Absturz als „abnormal“ in Release Health** ist für das
   Browser-SDK nicht dokumentiert.
8. **Doku-Punkte ohne wörtliche Bestätigung:** JS-Default für
   `sendDefaultPii`/`dataCollection` in v10, Prioritätsschwellen, GA-Datum
   von Sentry Logs (nur Blog), Sourcemaps über mehrere Orgs (nur
   GitHub-Issue).
9. **Leere `catch`-Blöcke** im Frontend sind nur stichprobenartig geprüft.
