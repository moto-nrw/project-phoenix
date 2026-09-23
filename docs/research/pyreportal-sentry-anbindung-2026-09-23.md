# PyrePortal im GKT-Browser an Sentry anbinden (Stand 23.09.2026)

Recherche für #3591, Teil der Karte #3586. Sie trägt Fakten und Varianten
zusammen, wählt aber keine Variante. Die Entscheidung fällt in einem eigenen
Grilling-Ticket.

- **Codestand:** PyrePortal `development` bei `e4761ee` (Version 1.7.3),
  project-phoenix `origin/development` bei `c59dd0abd9`. Pfade ohne Präfix
  beziehen sich auf PyrePortal, Pfade mit `phoenix/` auf project-phoenix.
- **Sentry-SDK-Quellen:** `getsentry/sentry-javascript` bei Commit
  `f8f196ef4a` (Branch `develop`, 23.09.2026), dazu die Sentry-Doku.
- **Kennzeichnung:** **[belegt]** heißt: im Code, in der Konfiguration oder per
  `gh`/`npm view` nachgesehen. **[abgeleitet]** heißt: aus belegten Fakten
  geschlossen, nicht beobachtet. **[Doku]** heißt: laut Sentry-Primärquelle.
- **Nicht gemacht:** keine Requests an moto-Domains, kein Zugriff auf ein
  GKT-Gerät, kein Blick in das Sentry-Konto. Die Faktenbasis der Karte
  ([Sentry Ist-Stand](https://github.com/moto-nrw/project-phoenix/blob/research/sentry-ist-stand/docs/research/sentry-ist-stand-2026-09-23.md),
  Lücke 10) gilt weiter: PyrePortal meldet heute keinen Fehler aus der Ferne.

## Kurzfazit

1. **Das SDK läuft technisch.** `@sentry/react` braucht laut Doku ES2020 und
   Chrome 80. Der heutige PyrePortal-Build setzt bereits mehr voraus
   (Top-Level-`await`, Vite-Ziel Chrome 111). Läuft die App auf dem GKT, reicht
   die WebView auch für Sentry. **[abgeleitet]** Die genaue WebView-Version ist
   nirgends dokumentiert.
2. **DSN und Sourcemaps passen in den bestehenden Build.** Der DSN kommt wie
   `VITE_API_BASE_URL` als Build-Variable aus `deploy-gkt.yml`. Sourcemaps lädt
   `@sentry/vite-plugin` hoch und löscht sie vor dem `rsync`. Dafür fehlen im
   PyrePortal-Repo heute `SENTRY_AUTH_TOKEN`, Org und Projekt. **[belegt]**
3. **Offline gehen Events ohne Zusatz verloren.** Mit
   `makeBrowserOfflineTransport` landen sie in IndexedDB (Standard: 30) und
   werden mit Backoff von 5 s bis 1 h sowie beim `online`-Event nachgesendet.
   **[Doku, Quellcode]**
4. **Der Device-API-Key steckt in der URL und das SDK sendet URLs.**
   `event.request.url` (ungefiltert, auch unter v11 `dataCollection`) und der
   Navigation-Breadcrumb `from: "/?key=..."` enthalten ihn. Sentrys
   serverseitiger Scrubber kennt `key` nicht. Ein eigener `beforeSend` und
   `beforeBreadcrumb` sind also Pflicht. **[belegt, Quellcode, Doku]**
5. **Eine Gerätezuordnung ohne Key gibt es schon.** `POST /api/iot/ping`
   liefert `device_id` und `device_name`, `GET /api/iot/status` ebenso.
   PyrePortal verwirft die Antwort heute. **[belegt]**
6. **Vier Transportwege sind realistisch:** direkt an Sentry, über den
   bestehenden Next.js-Tunnel `/monitoring`, über einen Proxy auf dem
   Kiosk-Webserver oder über ein neues Relay im Go-Backend. Abwägung unten.

## 1. Browser-Engine der GKT/GKTL-Geräte

**Was belegt ist:**

- GKT läuft in einer nativen Android-WebView mit JS-Brücke `GKTKiosk`, dazu
  `system.js` von Gekartel (`system_receivername = 'de.gekartel.VALUECHANGED'`,
  Version `v0.4`) (`public/system.js:1-2`, `vite.config.ts:18-26`,
  `docs/kiosk-deployment.md:9`). **[belegt]**
- Commit `6cf0087` nennt die Hardware: „Android WebView on RK3568 renders CSS
  `background-clip: text` as hollow outlines“. **[belegt]**
- Android-Version, WebView-/Chromium-Version und die Update-Politik von
  Gekartel stehen in keinem der beiden Repos und in keinem Issue.
  `rg -i "webview|android|userAgent|chromium"` findet nur die obigen Stellen.
  **[belegt]**

**Was sich daraus ableiten lässt:**

- `src/main.tsx:24-31` nutzt Top-Level-`await`. Das braucht Chrome 89 oder
  neuer. **[abgeleitet]**
- `vite.config.ts` setzt kein `build.target`. Vite 8.3.0 baut damit für
  `baseline-widely-available` = `chrome111, edge111, firefox114, safari16.4,
  ios16.4` (`node_modules/vite/dist/node/chunks/node.js:699-705`,
  Default in Zeile 33834). Syntax, die Chrome 111 kann, bleibt also
  unübersetzt. **[belegt]**
- Wenn Version 1.7.3 auf den GKT-Geräten läuft, ist deren WebView damit
  mindestens etwa Chrome 111. Das liegt deutlich über Sentrys Minimum.
  **[abgeleitet, nicht auf dem Gerät geprüft]**

**Sentry-Anforderung:** ES2020, Chrome 80, Edge 80, Safari 15, Firefox 74,
Samsung Internet 13, außerdem die `fetch`-API
([Supported Browsers](https://docs.sentry.io/platforms/javascript/troubleshooting/supported-browsers/)).
v11 hat nur Safari 14 gestrichen
([MIGRATION.md, „Upgrading from 10.x to 11.x“](https://github.com/getsentry/sentry-javascript/blob/develop/MIGRATION.md)).
**[Doku]** Für die Wedge-Geräte (iPad) ist Safari 15 die Grenze; Vites
Ziel `ios16.4` liegt schon darüber. **[abgeleitet]**

**Wie man die Version auf dem Gerät erfährt:**

1. Einmalig `navigator.userAgent` loggen. Auf GKT geht `logger.warn/error` über
   `adapter.persistLog` an `SYSTEM.log2('PyrePortal', ...)`
   (`src/platform/gkt/index.ts:87-94`), also ins Gekartel-Gerätelog. Ein
   `logger.info` beim Start (`src/App.tsx:41-44`) würde ab `persistLevel` INFO
   ebenfalls persistiert (`src/utils/logger.ts:43-44`).
2. Das Access-Log des Kiosk-Webservers auswerten (`DEPLOY_HOST`,
   `/var/www/pyreportal`). Übliche nginx-Formate enthalten den User-Agent. Die
   Serverkonfiguration liegt nicht im Repo (`docs/kiosk-deployment.md:42-43`).
   **[abgeleitet]**
3. Nach einer Sentry-Anbindung zeigt Sentry Browser und Version pro Event.

Offen bleibt, ob IndexedDB in der Gekartel-WebView verfügbar und dauerhaft ist.
localStorage funktioniert, denn die App speichert Sitzungen dort
(`src/platform/shared/localStorageSession.ts:14`). IndexedDB ist davon
getrennt und muss auf dem Gerät geprüft werden. **[abgeleitet]**

## 2. Welches SDK

- **Paket:** `@sentry/react`, `latest` = 11.0.0 (veröffentlicht 23.09.2026),
  `v10` = 10.75.3 (`npm view @sentry/react dist-tags`; GitHub-Releases).
  Peer-Dependencies von 11.0.0: `react 17.x || 18.x || 19.x`, `react-router
  6.x || 7.x || 8.x`. PyrePortal nutzt React `^19.3.0` und React Router
  `^8.3.1` (`package.json:35-37`). **[belegt]**
- **React 19:** Die Doku empfiehlt `Sentry.reactErrorHandler()` in den
  `createRoot`-Optionen `onUncaughtError`, `onCaughtError`,
  `onRecoverableError`
  ([React-Guide](https://docs.sentry.io/platforms/javascript/guides/react/)).
  PyrePortal ruft `createRoot` in `src/main.tsx:33-37` ohne Optionen auf und hat
  eine eigene `ErrorBoundary` (`src/App.tsx:58`,
  `src/utils/errorBoundary.tsx:32-40`). **[Doku, belegt]**
- **Init-Zeitpunkt:** Laut Doku muss `Sentry.init` vor allen anderen Imports
  laufen. In PyrePortal läuft `initializeApi()` per Top-Level-`await` vor dem
  Render (`src/main.tsx:24-31`), und `getDeviceApiKey` wirft, wenn `?key=`
  fehlt (`src/platform/webAdapterBase.ts:107-118`). Fehler dort erfasst Sentry
  nur, wenn es vorher initialisiert ist. **[Doku, belegt]**
- **Globale Handler:** PyrePortal hat heute kein `window.onerror` und kein
  `unhandledrejection`. Die Default-Integration `globalHandlersIntegration`
  deckt beides ab (`packages/browser/src/sdk.ts:35-46`). **[belegt, Quellcode]**
- **v10 oder v11:** Für PyrePortal wichtig sind drei v11-Änderungen
  (MIGRATION.md): `sendDefaultPii` wird durch `dataCollection` ersetzt, mit
  großzügigeren Defaults (u. a. `userInfo: true`, `httpBodies` alle).
  `attachStacktrace` ist jetzt standardmäßig `true`. Browser-Sessions nutzen
  `lifecycle: 'page'` statt `'route'`; beim Kiosk, der den ganzen Tag ohne
  Reload läuft, entsteht damit eine Session pro Seitenaufruf. **[Doku]**
  Das Next.js-Frontend steht auf `@sentry/nextjs` 10.70.0
  (`phoenix/frontend/package.json:46`). Ob PyrePortal auf v10 oder v11 geht,
  hängt an der Frage, ob beide Oberflächen dieselbe Major-Linie fahren sollen.

## 3. DSN in den Build bringen

- `build:gkt` ist nur ein Alias für `tsc && vite build`
  (`package.json:9-11`). Es gibt einen gemeinsamen Build für GKT und Wedge
  (`docs/kiosk-deployment.md:1-5`). **[belegt]**
- Der Deploy-Workflow setzt heute nur `VITE_API_BASE_URL` aus einer
  GitHub-Variable (`.github/workflows/deploy-gkt.yml:28-30, 49-52`) und kennt
  schon `DEPLOY_ENV` = `production` oder `staging` (Zeile 30). **[belegt]**
- Im PyrePortal-Repo gibt es die Secrets `DEPLOY_SSH_KEY`,
  `SLACK_DEPLOY_WEBHOOK`, `SONAR_TOKEN` und die Variablen `DEPLOY_HOST`,
  `GKT_PROD_API_URL`, `GKT_STAGING_API_URL` (`gh secret list`,
  `gh variable list`). Nichts für Sentry. **[belegt]**
- **Weg:** `VITE_SENTRY_DSN` und `VITE_SENTRY_ENVIRONMENT` (aus `DEPLOY_ENV`)
  im Schritt „Build unified kiosk“ setzen, im Code über `import.meta.env`
  lesen. Ohne DSN läuft `Sentry.init` nicht, lokale Builds bleiben wie sie
  sind. **[abgeleitet]**
- **Geheimhaltung:** Laut Sentry ist ein DSN öffentlich unkritisch. Er erlaubt
  nur das Einsenden von Events, kein Lesen
  ([DSN Explainer](https://docs.sentry.io/concepts/key-terms/dsn-explainer/)).
  Er landet ohnehin im ausgelieferten Bundle. PyrePortal ist ein öffentliches
  Repo, dessen Regeln Schlüssel im Quelltext verbieten (`CLAUDE.md`,
  „Releasing“). Deshalb gehört der DSN in eine GitHub-Variable oder ein
  Secret, nicht in den Code. **[Doku, belegt]**
- **Release:** `__APP_VERSION__` aus `package.json` gibt es schon
  (`vite.config.ts:9, 29-31`). Das Vite-Plugin würde ohne `release.name` den
  Git-SHA nehmen und per `release.inject` (Default `true`) ins Bundle
  schreiben
  ([bundler-plugins, Optionstabelle](https://github.com/getsentry/sentry-javascript-bundler-plugins/blob/main/packages/dev-utils/src/generate-documentation-table.ts)).
  Welcher Release-Name gilt, entscheidet #3589. **[belegt, Doku]**

## 4. Offline und instabiles Netz

- **Ohne Offline-Transport:** „Transports will drop an event if it fails to
  send due to a lack of connection.“
  ([Options, transport](https://docs.sentry.io/platforms/javascript/configuration/options/)).
  **[Doku]**
- **Mit `makeBrowserOfflineTransport(makeFetchTransport)`**
  ([Offline Caching](https://docs.sentry.io/platforms/javascript/best-practices/offline-caching/)):
  - Speicher: IndexedDB. Optionen `maxQueueSize` (Default 30),
    `flushAtStartup` (Default `false`), `shouldStore`, `shouldSend`,
    `dbName`, `storeName`. **[Doku]**
  - Gespeichert wird nur, wenn `fetch` wirft (Netzfehler). Antworten mit
    Status ab 400 werden nicht gespeichert. Retry mit Backoff ab 5 s,
    verdoppelnd bis 1 h. Nach erfolgreichem Senden wird die Queue geleert
    (`packages/core/src/transports/offline.ts:11-13, 154-189`). **[Quellcode]**
  - Das `online`-Event des Fensters löst zusätzlich einen Flush aus
    (`packages/browser/src/transports/offline.ts:159-169`). **[Quellcode]**
    `system.js` warnt selbst, dass `navigator.onLine` auf dem Gerät „buggy“
    sei (`public/system.js:251-254`). Darum ist der Backoff der verlässlichere
    Weg. PyrePortal nutzt für den eigenen Netzstatus bewusst nur den
    Health-Check (`src/hooks/useNetworkStatus.ts:29-36`). **[belegt]**
  - Ist die Queue voll, werden weitere Events verworfen
    (`packages/browser/src/transports/offline.ts:51-75`). **[Quellcode]**
- **Falle mit Tunnel über fremde Origin:** Wirft `fetch`, weil eine
  CORS-Antwort fehlt, obwohl der Server das Event angenommen hat, speichert der
  Offline-Transport es und sendet es später erneut. Das ergäbe Duplikate. Das
  betrifft nur Varianten mit Cross-Origin-Tunnel ohne passende CORS-Header.
  **[abgeleitet]**
- **Netz in den Schulen:** Ob Schulnetze `*.ingest.sentry.io` (bzw.
  `*.ingest.de.sentry.io`) erreichen, ist unbekannt. Sicher erreichbar ist
  nur die API-Domain, die PyrePortal schon nutzt. **[abgeleitet]**

## 5. Sourcemaps für den GKT-Build

- Anleitung: `@sentry/vite-plugin` als letztes Plugin, `build.sourcemap:
  "hidden"`, `authToken: process.env.SENTRY_AUTH_TOKEN`,
  `sourcemaps.filesToDeleteAfterUpload` für `*.map`
  ([Vite-Guide](https://docs.sentry.io/platforms/javascript/guides/react/sourcemaps/uploading/vite/)).
  Das Plugin läuft nur im Produktions-Build. **[Doku]**
- Aktuell: `@sentry/vite-plugin` 5.4.0, `engines.node >= 18`
  (`npm view`). Der Workflow nutzt Node 22 (`deploy-gkt.yml:37-39`).
  Vite 8 nutzt Rolldown; ob Plugin 5.4.0 dort ohne Einschränkung läuft, ist
  nicht geprüft. **[belegt, offen]**
- `vite.config.ts` erzeugt heute keine Sourcemaps (kein `build.sourcemap`;
  Vite-Default `false`, `node_modules/vite/dist/node/cli.js:765`). Der Deploy
  kopiert `dist/` per `rsync --delete` (`deploy-gkt.yml:91-93`). Werden die
  Maps nach dem Upload gelöscht, landen sie nicht auf dem Server. **[belegt]**
- Da PyrePortal öffentlich ist, würden ausgelieferte Maps keinen geheimen Code
  preisgeben. Sie wären aber unnötige Last für die Kiosk-Geräte.
  **[abgeleitet]**
- Neu nötig: `SENTRY_AUTH_TOKEN` (Secret), `SENTRY_ORG`, `SENTRY_PROJECT`
  (Variablen) im PyrePortal-Repo. Im Phoenix-Repo gibt es dieselben Namen
  (Faktenbasis, Abschnitt Environments & Releases). **[belegt]**

## 6. Gerät zuordnen, ohne den Device-API-Key zu senden

### Wo der Key heute sichtbar ist

- Der Key kommt als `?key=...` in die Kiosk-URL und wird beim Start gelesen
  und zwischengespeichert (`src/platform/webAdapterBase.ts:107-118`).
  `restartApp` baut die URL mit Key neu (`webAdapterBase.ts:132-138`). Die
  Landing-Page `/` trägt ihn also in der Adresse, nach `navigate('/pin')`
  (`src/pages/LandingPage.tsx:26`) ist er aus der URL verschwunden.
  **[belegt]**
- API-Requests senden den Key nur im `Authorization`-Header
  (`CLAUDE.md`, „Authentication Pattern“), nicht in der URL. **[belegt]**

### Wo das SDK ihn mitnehmen würde

| Stelle | Verhalten | Quelle |
|---|---|---|
| `event.request.url` | `httpContextIntegration` setzt `location.href` ein. Kommentar im Code: „The URL isn't gated by `dataCollection`“. Ein Fehler auf `/` enthält damit `?key=`. | `packages/browser/src/integrations/httpcontext.ts:25-42` **[Quellcode]** |
| Navigation-Breadcrumb | `from`/`to` sind `parsedUrl.relative`, also Pfad plus Query. Der Wechsel `/` nach `/pin` ergibt `from: "/?key=..."`. | `packages/browser/src/integrations/breadcrumbs.ts:290-320`, `packages/core/src/utils/url.ts:292` **[Quellcode]** |
| `dataCollection.urlQueryParams` (v11) | Default `true`. Werte „sensibler“ Parameter werden `[Filtered]`, Parameternamen immer gesendet. Die Default-Denyliste (`forwarded`, `-ip`, `remote-`, `via`, `-user`) enthält `key` nicht. | MIGRATION.md, Abschnitt `dataCollection`; [Options](https://docs.sentry.io/platforms/javascript/configuration/options/) **[Doku]** |
| Serverseitiges Scrubbing | Default-Felder: `password, secret, passwd, api_key, apikey, auth, credentials, mysql_pwd, privatekey, private_key, token, bearer`. Ein Parameter `key` fällt nicht darunter. | [Server-Side Scrubbing](https://docs.sentry.io/security-legal-pii/scrubbing/server-side-scrubbing/) **[Doku]** |
| Tracing-Spans/Transaktionsnamen | Nur relevant, falls Tracing aktiviert wird (auf der Karte offen). | [Sensitive Data](https://docs.sentry.io/platforms/javascript/data-management/sensitive-data/) **[Doku]** |

Folgerung: Ein clientseitiger Filter ist Pflicht. `beforeSend` muss `key` aus
`event.request.url` entfernen, `beforeBreadcrumb` aus Navigation-Breadcrumbs
(`data.from`, `data.to`). Sentrys Doku empfiehlt genau diese Hooks, damit
Daten „never leaves the local environment“. Als zweite Linie kann `key` in
Sentry unter „Additional Sensitive Fields“ eingetragen werden. Das greift aber
erst nach dem Versand. **[abgeleitet, Doku]**

### Weitere Daten, die ungefragt mitkämen

- **Konsolen-Breadcrumbs:** Default-Integration `consoleIntegration`
  (`packages/browser/src/sdk.ts:40`; MIGRATION.md: in v11 eigene Integration).
  `system.js` schreibt jede NFC-Nutzlast mit Armband-UID per `console.log` in
  die Konsole (`public/system.js:79-80, 340, 360, 376`). Die UIDs landen damit
  als Breadcrumbs in jedem späteren Event. Der PyrePortal-Logger schreibt in
  Produktion erst ab WARN in die Konsole (`src/utils/logger.ts:42`), und die
  `tagId`-Logs sind INFO/DEBUG (`src/hooks/useRfidScanning.ts:173-182, 308`).
  **[belegt, abgeleitet]**
- **Nutzerdaten:** Unter v11 ist `dataCollection.userInfo` standardmäßig
  `true` (IP-Ableitung). Unter v10 ist das Gegenstück `sendDefaultPii: false`.
  **[Doku]**

### Wie ein Event trotzdem einem Gerät zugeordnet werden kann

- `POST /api/iot/ping` antwortet mit `device_id` und `device_name`
  (`phoenix/backend/api/iot/checkin/handlers.go:14-29`). PyrePortal ruft es bei
  der PIN-Prüfung auf und verwirft die Antwort (`src/services/api.ts:287-306`,
  mit festem `deviceName: 'OGS Device'`). **[belegt]**
- `GET /api/iot/status` liefert `device.id`, `device_id`, `device_type`,
  `name` (`phoenix/backend/api/iot/checkin/handlers.go:33-52`). Die Route liegt
  in der `DeviceAuthenticator`-Gruppe, die den PIN laut Kommentar nur „when
  supplied“ bindet (`phoenix/backend/api/iot/compose/compose.go:113-132`).
  Ob sie ohne PIN antwortet, ist nicht getestet. **[belegt, abgeleitet]**
- `device_id` ist die Gerätekennung, der API-Key ein getrenntes Feld, das nie
  in JSON erscheint (`phoenix/backend/models/iot/device.go:36-40`). **[belegt]**
- Der Schulname ist nach dem Start bekannt (`src/services/schoolName.ts:41-48`),
  also wäre auch ein Schul-Tag möglich. Welche Kontexte erlaubt sind, regelt
  die Datenschutzentscheidung der Karte. **[belegt]**
- Mögliche Kennungen, jeweils mit `Sentry.setTag`/`initialScope`:
  1. `device_id` aus Ping/Status: lesbar, stabil, braucht einen Request und
     fehlt für Fehler vor dem ersten erfolgreichen Aufruf.
  2. Kurzer SHA-256-Hash des Keys, sofort beim Start verfügbar, ohne
     Backend-Aufruf: pseudonym, ändert sich bei Key-Rotation, und in Sentry
     braucht man eine Zuordnungstabelle. `crypto.subtle` gibt es nur in
     sicheren Kontexten (siehe Hinweis in `src/utils/logger.ts:99-107`).
  3. Serverseitig ergänzt, falls ein Backend-Relay genutzt wird (Variante D).
  **[abgeleitet]**
- Plattform (`gkt` oder `wedge`) steht schon in `adapter.platform` und wird
  beim Start geloggt (`src/services/apiClient.ts:67-71`). **[belegt]**

## 7. Transportwege

Alle Varianten nutzen dasselbe SDK, denselben Scrubbing-Filter und denselben
Sourcemap-Upload. Sie unterscheiden sich nur darin, wohin der Browser die
Envelopes schickt.

### A. Direkt an Sentry

- Browser sendet an die Ingest-URL aus dem DSN. Laut Tunnel-Doku hängt der
  `sentry_key` dabei als Query-Parameter an, und Ad-Blocker blockieren solche
  Requests eher. Auf einem Kiosk ohne Ad-Blocker ist das kaum relevant.
  **[Doku, abgeleitet]**
- **Pro:** kein Code außerhalb von PyrePortal, keine Kopplung an Phoenix.
- **Contra:** braucht ausgehenden Zugriff der Schulnetze auf Sentry
  (unbekannt). Sendet die IP des Geräts direkt an Sentry (Projektschalter
  „Prevent Storing of IP Addresses“, siehe Faktenbasis).
- CSP des Kiosk-Servers: unbekannt, nicht im Repo, `index.html` hat keine
  CSP-Meta. **[belegt]**

### B. Über den bestehenden Next.js-Tunnel `/monitoring`

- `phoenix/frontend/next.config.js:98` setzt `tunnelRoute: "/monitoring"`.
  `@sentry/nextjs` 10.70.0 legt dafür nur eine Rewrite-Regel an:
  `/monitoring?o=<orgid>&p=<projectid>[&r=<region>]` wird zu
  `https://o<orgid>.ingest[.<region>].sentry.io/api/<projectid>/envelope/`
  (`phoenix/frontend/node_modules/@sentry/nextjs/build/cjs/config/withSentryConfig/tunnel.js:19-71`).
  Der Proxy lässt `/monitoring` auf allen Host-Typen durch und setzt nur
  Security-Header (`phoenix/frontend/src/proxy.ts:115-121, 226, 337, 426, 711`).
  **[belegt]**
- PyrePortal könnte `tunnel: "https://<frontend-host>/monitoring?o=...&p=..."`
  setzen. **[abgeleitet]**
- **Pro:** kein neuer Dienst, die Frontend-Domain ist in Schulnetzen
  vermutlich erreichbar.
- **Contra:** Cross-Origin vom Kiosk-Host. Ob die CORS-Header von Sentry durch
  den Rewrite zurückkommen, ist nicht geprüft; fehlen sie, drohen mit
  Offline-Transport Duplikate (Abschnitt 4). Die Regel prüft Org und Projekt
  nicht gegen eine Liste, die Sentry-Doku empfiehlt das aber
  ([Troubleshooting, tunnel](https://docs.sentry.io/platforms/javascript/troubleshooting/)).
  Das gilt heute schon. Die Kiosk-Fehlermeldung hängt dann an
  Frontend-Deploys. v11 schickt Tunnel-Requests durch die Middleware
  (MIGRATION.md, Zeile 1276). **[belegt, Doku, offen]**

### C. Same-Origin-Proxy auf dem Kiosk-Webserver

- Der Webserver unter `DEPLOY_HOST` leitet z. B. `/monitoring` an Sentry
  weiter. PyrePortal nutzt dann einen relativen `tunnel`. Laut Doku löst eine
  relative Tunnel-URL keinen CORS-Preflight aus. **[Doku, abgeleitet]**
- **Pro:** kein CORS-Thema, kein Phoenix-Code, Kiosk bleibt unabhängig.
- **Contra:** Serverkonfiguration liegt nicht im Repo
  (`docs/kiosk-deployment.md:42-43`), also Betrieb außerhalb von Code-Review.
  Projekt-ID-Prüfung muss der Proxy selbst leisten. **[belegt, abgeleitet]**

### D. Relay im Go-Backend unter `/api/iot/...`

- Neuer Endpunkt in der Device-Gruppe nimmt Envelopes an und leitet sie an
  Sentry weiter. Die Tunnel-Doku beschreibt den Ablauf: DSN aus dem
  Envelope-Header lesen, Projekt-ID gegen eine Liste prüfen, an
  `https://{host}/api/{projectId}/envelope/` senden. **[Doku]**
- **Pro:** gleiche Domain wie alle API-Aufrufe, also sicher erreichbar.
  Das Backend kennt Gerät und Schule aus dem Key und kann sie ergänzen, ohne
  dass der Client etwas mitschickt. Missbrauch des Endpunkts ist an einen
  gültigen Device-Key gebunden.
- **Contra:** neuer IoT-Endpunkt, also Vertrag in beiden Repos
  (`CLAUDE.md` des Ökosystems, „Cross-Repo Impact“). Das Backend müsste
  Envelopes weiterreichen oder umbauen. Das SDK sendet ohne eigene Header;
  der Device-Key müsste über `transportOptions.headers` mitgehen. Fällt das
  Backend aus, gehen auch Fehlermeldungen darüber verloren, obwohl gerade
  dann Kiosk-Fehler interessant sind (der Offline-Transport puffert nur bei
  Netzfehlern, nicht bei 5xx). **[abgeleitet, Quellcode]**

### Übersicht

| | A direkt | B Next-Tunnel | C Kiosk-Proxy | D Go-Relay |
|---|---|---|---|---|
| Neuer Code außerhalb PyrePortal | nein | nein | Serverkonfig | ja, Backend |
| Erreichbarkeit aus Schulnetz | unbekannt | wahrscheinlich | ja (Kiosk-Host) | ja (API-Host) |
| CORS/Duplikat-Risiko | gering | offen | keins | gering (eigene Header) |
| Gerätezuordnung | Client | Client | Client | Server möglich |
| Abhängig von | Sentry | Frontend-Deploy | Kiosk-Server | Backend |

## Offene Punkte

1. WebView-Version und IndexedDB auf GKT/GKTL (Abschnitt 1, Messweg dort).
2. Ob Schulnetze Sentry direkt erreichen (Variante A).
3. Ob die CORS-Header über den Next-Rewrite zurückkommen (Variante B); nur
   lokal oder auf Staging prüfbar.
4. Ob `GET /api/iot/status` ohne PIN antwortet.
5. Ob `@sentry/vite-plugin` 5.4.0 mit Vite 8 (Rolldown) ohne Einschränkung
   läuft.
6. Eigenes Sentry-Projekt für PyrePortal oder gemeinsam mit dem Frontend,
   Release-Name, Environments: hängt an #3589, Konto und Region an #3587.
7. SDK-Linie v10 oder v11 und, bei v11, welche `dataCollection`-Werte gelten.

## Quellen

- PyrePortal (lokal, `e4761ee`): `package.json`, `vite.config.ts`,
  `src/main.tsx`, `src/App.tsx`, `src/utils/logger.ts`,
  `src/utils/errorBoundary.tsx`, `src/platform/webAdapterBase.ts`,
  `src/platform/gkt/index.ts`, `src/services/apiClient.ts`,
  `src/services/api.ts`, `src/services/schoolName.ts`,
  `src/hooks/useNetworkStatus.ts`, `public/system.js`,
  `docs/kiosk-deployment.md`, `.github/workflows/deploy-gkt.yml`.
- project-phoenix (`c59dd0abd9`): `frontend/next.config.js`,
  `frontend/src/proxy.ts`, `frontend/package.json`,
  `backend/api/iot/compose/compose.go`, `backend/api/iot/checkin/handlers.go`,
  `backend/models/iot/device.go`; `@sentry/nextjs` 10.70.0 aus
  `frontend/node_modules`.
- getsentry/sentry-javascript `f8f196ef4a`:
  [`packages/browser/src/integrations/httpcontext.ts`](https://github.com/getsentry/sentry-javascript/blob/f8f196ef4a9d783abf6d1cc2c58c1d7e302bf04d/packages/browser/src/integrations/httpcontext.ts),
  [`packages/browser/src/integrations/breadcrumbs.ts`](https://github.com/getsentry/sentry-javascript/blob/f8f196ef4a9d783abf6d1cc2c58c1d7e302bf04d/packages/browser/src/integrations/breadcrumbs.ts),
  [`packages/browser/src/sdk.ts`](https://github.com/getsentry/sentry-javascript/blob/f8f196ef4a9d783abf6d1cc2c58c1d7e302bf04d/packages/browser/src/sdk.ts),
  [`packages/browser/src/transports/offline.ts`](https://github.com/getsentry/sentry-javascript/blob/f8f196ef4a9d783abf6d1cc2c58c1d7e302bf04d/packages/browser/src/transports/offline.ts),
  [`packages/core/src/transports/offline.ts`](https://github.com/getsentry/sentry-javascript/blob/f8f196ef4a9d783abf6d1cc2c58c1d7e302bf04d/packages/core/src/transports/offline.ts),
  [`MIGRATION.md`](https://github.com/getsentry/sentry-javascript/blob/f8f196ef4a9d783abf6d1cc2c58c1d7e302bf04d/MIGRATION.md).
- Sentry-Doku (abgerufen 23.09.2026):
  [Supported Browsers](https://docs.sentry.io/platforms/javascript/troubleshooting/supported-browsers/),
  [Offline Caching](https://docs.sentry.io/platforms/javascript/best-practices/offline-caching/),
  [Vite Sourcemaps](https://docs.sentry.io/platforms/javascript/guides/react/sourcemaps/uploading/vite/),
  [React-Guide](https://docs.sentry.io/platforms/javascript/guides/react/),
  [Options](https://docs.sentry.io/platforms/javascript/configuration/options/),
  [Troubleshooting/Tunnel](https://docs.sentry.io/platforms/javascript/troubleshooting/),
  [Sensitive Data](https://docs.sentry.io/platforms/javascript/data-management/sensitive-data/),
  [Server-Side Scrubbing](https://docs.sentry.io/security-legal-pii/scrubbing/server-side-scrubbing/),
  [DSN Explainer](https://docs.sentry.io/concepts/key-terms/dsn-explainer/).
- npm (`npm view`, 23.09.2026): `@sentry/react` dist-tags und
  Peer-Dependencies, `@sentry/vite-plugin` 5.4.0.
