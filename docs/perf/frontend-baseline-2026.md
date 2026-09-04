# Frontend-Performance-Baseline 2026

Erste systematische Messung des Tenant-Frontends (Issue #2938). Jede Aussage hier hat eine Zahl aus einem der vier Messläufe: Playwright-Traces gegen den Prod-Server, Proxy-Metriken, Turbopack-Bundle-Analyse, react-scan gegen den Dev-Server. Der Mess-Harness liegt unter `frontend/scripts/perf/` und ist wiederholbar (siehe Anhang).

## Kurzfassung

1. **Die Bottom-Navigation lief auf jeder Seite in einer Render-Schleife.** Rund 2.100 Renders pro Sekunde im Leerlauf, auf dem Dashboard 89.771 Renders in 40 Sekunden. Ursache: ein Effekt in `MobileBottomNav`, der bei jedem Render ein neues State-Objekt setzt. Der Fix (State nur bei Änderung setzen) ist in diesem PR; danach 0 Renders im Leerlauf.
2. **Jede Seite feuert vor den Seitendaten 18 Shell-Requests plus viermal `/api/auth/session`.** Die Seitendaten warten auf die Kette Bundle, Session, User-Context, Settings-Schema. Auf der Zeiterfassung startet der erste Seitendaten-Request bei 1.417 ms. Folge-Issue #2973.
3. **Das Client-Bundle ist auf jeder Route 624 bis 837 kB JavaScript (komprimiert, über die Leitung).** 95 kB davon sind zod, weil `~/env` aus Client-Modulen importiert wird; 75 kB PostHog; auf vier Routen 121 bis 135 kB recharts ohne `next/dynamic`. Folge-Issue #2974.
4. **Interaktionen rendern zu viel.** Vier Tastendrücke in der Kindersuche: 25.110 Renders und 120 ms Long Tasks, kein einziger Request. Wochenwechsel in der Zeiterfassung: 7.526 Renders. Folge-Issue #2975.
5. **Der Proxy-Hop ist nicht das Problem.** Backend-Antworten liegen bei p50 3 bis 8 ms, der Hop kostet pro Request 10 bis 30 ms und läuft parallel. Die SWR-Dedupe-Vermutung aus dem Issue ist widerlegt: das einzige Duplikat ist NextAuths Session-Fetch.

## Messaufbau

| Was                   | Wert                                                                                                                                                                             |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Stand                 | Commit `2f728be8d8` (development, 2026-09-03), plus der Nav-Fix aus diesem PR bei den „nachher“-Zahlen                                                                           |
| Maschine              | Apple M3 Max, 64 GB, macOS 26.5.2, Node 24.12.0, pnpm 10.34.4                                                                                                                    |
| Frontend              | Next.js 16.3.0 (Turbopack), React 19.2, SWR 2.5, Playwright 1.62.1, react-scan 0.5.7                                                                                             |
| Server für Traces     | `next build && next start` auf `demo-school.localhost:3000`, Backend im Docker-Compose-Stack                                                                                     |
| Server für react-scan | Docker-Frontend (`next dev`), weil react-scan Fiber-Debuginfos braucht                                                                                                           |
| Daten                 | Seeder-Stand vom 2026-09-02 (100 Kinder, 20 Mitarbeitende, 90 Tage Zeiterfassung) plus `simulate full-day` (100 RFID-Tags, 7 Sessions, 90 Anwesenheiten, 83 Check-ins, 10 krank) |
| Konto                 | Admin `demo1@mail.de` aus `backend/.seed-state.json`                                                                                                                             |
| Browser               | Chromium headless, Desktop Chrome, **4x CPU-Drosselung per CDP**, kalter Cache je Lauf (frischer Context), Service Worker blockiert                                              |
| Wiederholungen        | 3 je Screen, Tabelle zeigt den Median, Spannweite in Klammern                                                                                                                    |
| Ruhe-Kriterium        | 1,5 s ohne Request-Start oder -Ende, SSE/Sentry/PostHog ausgenommen; `networkidle` geht wegen SSE nie                                                                            |

Ohne CPU-Drosselung meldete der M3 Max auf allen Screens 0 Long Tasks und LCP-Werte um 230 bis 380 ms. Schul-Hardware ist deutlich langsamer, deshalb sind alle Zahlen unten mit 4x Drosselung gemessen. Absolute Millisekunden sind auf andere Geräte nicht übertragbar; Verhältnisse und Zähler schon.

## Ergebnisse pro Screen

Kalter Aufruf, Median aus 3 Läufen, 4x CPU-Drosselung. „Settle“ ist die Zeit von der Navigation bis zum Ruhe-Kriterium. „Kette“ ist die längste Folge von API-Requests, bei der jeder erst nach dem Ende des vorherigen startet. „JS“ sind die Bytes aller `/_next/static/*.js`-Antworten über die Leitung.

| Screen              | TTFB  | FCP    | LCP      | Settle kalt                | Settle warm | Requests | davon /api/ | Kette      | JS kalt | Long Tasks | TBT    |
| ------------------- | ----- | ------ | -------- | -------------------------- | ----------- | -------- | ----------- | ---------- | ------- | ---------- | ------ |
| dashboard           | 11 ms | 116 ms | 868 ms   | 2.540 ms (2.523 bis 2.687) | 2.177 ms    | 100      | 24          | 4 (250 ms) | 624 kB  | 2 (293 ms) | 193 ms |
| active-supervisions | 8 ms  | 116 ms | 1.168 ms | 2.804 ms (2.783 bis 2.857) | 2.379 ms    | 108      | 25          | 5 (452 ms) | 699 kB  | 3 (324 ms) | 174 ms |
| students/search     | 19 ms | 112 ms | 1.360 ms | 3.110 ms (3.100 bis 3.153) | 2.719 ms    | 101      | 26          | 4 (621 ms) | 744 kB  | 6 (563 ms) | 263 ms |
| ogs-groups          | 10 ms | 100 ms | 100 ms   | 2.727 ms (2.718 bis 2.994) | 2.292 ms    | 95       | 23          | 4 (323 ms) | 710 kB  | 4 (383 ms) | 183 ms |
| time-tracking       | 8 ms  | 116 ms | 1.208 ms | 3.281 ms (3.244 bis 3.452) | 2.725 ms    | 117      | 40          | 5 (802 ms) | 837 kB  | 4 (593 ms) | 393 ms |
| betreuungsplan      | 10 ms | 128 ms | 1.356 ms | 2.985 ms (2.964 bis 3.036) | 2.532 ms    | 107      | 31          | 3 (351 ms) | 772 kB  | 3 (386 ms) | 236 ms |

Lesart:

- TTFB unter 20 ms und FCP um 110 ms: der Server liefert die leere Hülle sofort. Alles danach ist Client-Arbeit.
- LCP 0,9 bis 1,4 s bei 4x Drosselung, und das auf einer Maschine mit lokalem Backend. Der LCP-Wert von `ogs-groups` (100 ms) ist die Überschrift der Hülle, nicht der Inhalt; der Inhalt kommt erst nach dem Wasserfall.
- Der warme Aufruf (gleicher Context, Bundle im Cache) spart nur 350 bis 560 ms. Der Rest der Settle-Zeit ist Hydration plus API-Wasserfall, nicht Download.
- 2 bis 6 Long Tasks mit 300 bis 600 ms Gesamtdauer pro Aufruf: Hydration und der erste Render der Seite blockieren den Main-Thread. TBT 174 bis 393 ms.

### Interaktionen

| Screen          | Interaktion                        | Zeit bis ruhig | Requests | Long Tasks |
| --------------- | ---------------------------------- | -------------- | -------- | ---------- |
| students/search | 4 Zeichen in „Name suchen…“ tippen | 752 ms         | 0        | 120 ms     |
| time-tracking   | „Vorherige Woche“                  | 1.686 ms       | 6 API    | 0 ms       |
| betreuungsplan  | „Weiter“ (eine Woche)              | 1.685 ms       | 2 API    | 50 ms      |

Das Tippen in der Kindersuche löst keinen Request aus (Debounce plus clientseitige Filterung), kostet aber 120 ms Long Tasks: reine Render-Zeit für 100 Karten pro Tastendruck (siehe Render-Profiling). Der Wochenwechsel in der Zeiterfassung feuert sechs parallele Requests (`shifts`, `history`, `absences`, `schedule-targets`, `holidays`, `closing-days`), die alle in 11 bis 17 ms antworten; die 1,7 s bis zur Ruhe sind SWR-Revalidierung plus Ruhefenster.

## Netzwerk-Wasserfall

Auf allen sechs Screens läuft der kalte Aufruf gleich ab (Zeiten von der Zeiterfassung, 4x Drosselung):

1. 0 bis rund 900 ms: Dokument, 36 bis 44 JS-Chunks, Fonts, Hydration. Kein einziger API-Request.
2. 923 ms: `/api/platform/announcements/unread` und `/api/auth/session`.
3. 1.010 ms: `/api/auth/session` erneut, dann `/api/user-context` (1.317 ms).
4. 1.417 ms: ein Burst aus 20 bis 25 parallelen Requests. Darunter die 18 Shell-Requests, die auf **jeder** Seite identisch sind, plus die ersten Seitendaten.
5. 1.564 ms: Seitendaten, die vom Kontext abhängen (`/api/staff/2/schedule`).

Die 18 Shell-Requests, die auf allen sechs Screens vorkommen:
`/api/user-context`, `/api/me/profile`, `/api/settings/schema`, `/api/reminders`, `/api/platform/announcements/unread`, `/api/messages/unread-count`, `/api/staff-messages/unread-count`, `/api/staff-notices/today`, `/api/staff/absences/pending`, `/api/students/change-requests/pending-count`, `/api/enrollment/admin/change-requests/pending-count`, `/api/students/care-withdrawals?page_size=1`, `/api/groups/context`, `/api/active/supervisors/all`, `/api/active/schulhof/status`, `/api/auth/account-tenants`, `/api/notifications/push/public-key` (antwortet 404), `/api/auth/session`.

Duplikate: auf jedem Screen genau eines, `GET /api/auth/session` mit 4 Aufrufen (Starts bei 725, 771, 845 und 945 ms auf dem Dashboard). Das ist NextAuths `useSession`/`getSession` aus mehreren Providern, kein SWR-Key. Kein einziger SWR-Key wurde doppelt geholt.

Sequenzielle Ketten: die längste Kette ist überall die Shell-Kette Session, User-Context, Settings-Schema, erste Seitendaten (3 bis 5 Glieder, 250 bis 802 ms). Die Seitendaten selbst laufen parallel; die Zeiterfassung holt ihre 20 Requests in einem Burst bei 1.417 ms, und alle antworten in 17 bis 80 ms.

### Nachmessung nach #2973 (2026-09-04)

Derselbe Harness, dieselbe Maschine, dieselbe Drosselung. Das Tenant-Layout
lädt die Shell-Daten jetzt serverseitig und reicht sie als Startwerte in die
Provider und die SWR-Caches. `SessionProvider` bekommt eine rein lesend aus
dem Session-Cookie dekodierte Sitzung als Prop; Refresh-Callbacks laufen
weiterhin nur in Route-Handlern, die ein neues Cookie setzen können.

| Kennzahl (Zeiterfassung, kalt)  | vorher   | nachher  |
| ------------------------------- | -------- | -------- |
| Shell-Requests pro Seitenaufruf | 18       | 1        |
| `GET /api/auth/session`         | 4        | 0        |
| erster Seitendaten-Request      | 1.417 ms | 1.084 ms |
| `GET /api/staff/2/schedule`     | 1.564 ms | 1.314 ms |
| LCP                             | 1.208 ms | 772 ms   |

Der eine verbliebene Shell-Request ist `GET /api/notifications/push/public-key`
(404, weil Web Push lokal nicht eingerichtet ist). Er entfällt, sobald
`syncExistingPushSubscription` erst die Browser-Registrierung liest und den
Schlüssel nur holt, wenn es etwas zu binden gibt. Das ändert eine Zusicherung,
die `push-api.test.ts` seit dem Web-Push-Fix festhält (ohne Registrierung wird
die fehlende VAPID-Konfiguration gemeldet), und gehört deshalb in eine eigene,
bewusst entschiedene Änderung.

Die Requests für Erinnerungen und Ankündigungen sind nicht verschwunden,
sondern serverseitig: sie laufen jetzt parallel, während das HTML gestreamt
wird, statt nach dem Hydrieren im Browser. In den Proxy-Metriken tauchen sie
weiterhin auf.

Abgebrochene Requests: pro Aufruf 10 bis 28 `fetch`-Requests auf Seitenpfade (`/students/search`, `/rooms`, `/ogs-groups`, `/anfragen`, `/settings`, `/staff`, `/time-tracking`, …), die der Browser wieder abbricht. Das sind die Router-Prefetches der `next/link`-Einträge in Seitenleiste und Bottom-Nav. Folge-Issue #2976.

Nachmessung nach #2976 (Shell-Links über `NavLink`: kein Viewport-Prefetch, Prefetch erst bei Hover, Fokus oder Touch-Beginn; gleicher Harness, Stand 2026-09-04, frischer Seed): `cold.requests.failed` ist auf 17 von 18 kalten Aufrufen leer. Der eine Ausreißer (Dashboard, Lauf 3) zeigt 2 abgebrochene Prefetches, `/ogs-groups` und `/students/search`, und die kommen aus den Karten im Seiteninhalt des Dashboards (`InfoCard href`), nicht aus der Seitenleiste. Inhaltslinks nutzen weiter `next/link` mit Standard-Prefetch; das ist gewollt, weil sie wenige sind und auf den nächsten Klick zielen.

## Seitenwechsel statt Seitenaufruf (#2828)

Alles oben misst den **kalten Aufruf**. Die Beschwerde aus dem Testdurchlauf auf Staging galt etwas anderem: dem Wechsel von Seite zu Seite, der sich „wie ein kompletter Reload" anfühlte. Dafür gibt es einen eigenen Harness (`pnpm run perf:nav`, `scripts/perf/navigation.perf.ts`): Klick auf einen Seitenleisten-Link, fünf Ziele im Rundgang, drei Läufe, 4x CPU-Drosselung **und 150 ms Leitungslatenz bei 3 Mbit/s** — ohne Leitungsbremse ist gegen ein lokales Backend jeder Wechsel fertig, bevor überhaupt eine Ladehülle sichtbar wird, und man misst etwas, das keine Schule je erlebt.

Vor dem Klick markiert der Harness die Hülle mit einem Attribut und hängt einen MutationObserver ein; danach steht fest, ob React die Hülle neu aufgebaut hat und welche Ladehüllen zwischendurch zu sehen waren.

Stand `development` fc28be720 (2026-09-04), Median aus 3 Läufen:

| Wechsel nach     | Klick bis Inhalt | Dokument-Requests | Hülle überlebt | API | davon Hüllen-Endpunkte | sichtbar dazwischen         |
| ---------------- | ---------------- | ----------------- | -------------- | --- | ---------------------- | --------------------------- |
| /students/search | 1.583 ms         | 0                 | ja             | 6   | 1                      | eigenes Skelett             |
| /rooms           | 1.005 ms         | 0                 | ja             | 2   | 1                      | eigenes Skelett             |
| /settings        | 910 ms           | 0                 | ja             | 2   | 2                      | **„Lädt…"-Kringel**         |
| /statistics      | 924 ms           | 0                 | ja             | 2   | 1                      | **„Lädt…"-Kringel**, dann Skelett |
| /dashboard       | 326 ms           | 0                 | ja             | 4   | 1                      | nichts                      |

Damit sind die Fragen aus dem Issue beantwortet:

1. **Kein Full-Reload.** Null Dokument-Requests pro Wechsel, in allen drei Läufen. Die App-Shell wird nicht neu aufgebaut: die Marke auf dem Hüllen-Knoten überlebt jeden Wechsel.
2. **2 bis 6 API-Requests pro Wechsel**, praktisch alles Seitendaten. Der einzige Hüllen-Endpunkt, der bei jedem Wechsel erneut läuft, ist `GET /api/platform/announcements/unread`: `useAnnouncements` holt ihn bei jedem Pfadwechsel absichtlich neu, damit eine neue Ankündigung an einem ruhigen Moment erscheint und nicht mitten in einer Eingabe. Das bleibt so. (`/api/settings/schema` taucht einmalig beim ersten Aufruf von /settings auf.)
3. **Layout-Daten werden bereits einmal geladen und behalten** — das ist #2973. Die 18 Hüllen-Requests des kalten Aufrufs wiederholen sich beim Wechsel nicht.
4. **Prefetching ist in Ordnung** — das ist #2976. Der Link wird bei Hover, Fokus oder Touch-Beginn vorgeladen.

Was blieb, war das Sichtbare: die geteilte Ladehülle `(protected)/loading.tsx` zeigte einen allgemeinen Kringel in Inhaltshöhe. Auf jeder Seite ohne eigenes Skelett — 59 der 76 geschützten Seiten — sprang das Layout beim Wechsel erst auf Kringelhöhe zusammen, dann auf das Skelett der Zielseite, dann auf den Inhalt. Drei Zustände für einen Wechsel.

Nachmessung nach #2828 (geteilte Hülle entfernt, Fortschrittsbalken in der Hülle; gleicher Harness, gleiche Maschine, gleiche Drosselung):

| Kennzahl                              | vorher                        | nachher                       |
| ------------------------------------- | ----------------------------- | ----------------------------- |
| „Lädt…"-Kringel je Rundgang           | 2 von 5 Wechseln              | 0                             |
| Zustände zwischen zwei Seiten         | bis zu 3                      | 1 (Balken, dazu ggf. Skelett) |
| Klick bis Inhalt (/settings)          | 910 ms                        | 936 ms                        |
| Klick bis Inhalt (/students/search)   | 1.583 ms                      | 1.601 ms                      |
| Dokument-Requests, Hülle überlebt     | 0 / ja                        | 0 / ja                        |

Die Zeit bis zum Inhalt ändert sich nicht (12 bis 26 ms mehr, gleichmäßig über alle Wechsel — der Melder je Link und der Balken selbst). Was sich ändert, ist, was in dieser Zeit zu sehen ist: die aktuelle Seite bleibt stehen, bis die neue bereit ist, und ein 3 Pixel hoher Balken am oberen Rand zeigt, dass etwas läuft. Er erscheint erst nach 150 ms, sodass schnelle Wechsel ganz ohne Zwischenzustand ablaufen.

## Proxy-Metriken

`/api/internal/metrics` wurde vor und nach dem Trace-Lauf abgezogen; die Tabelle zeigt die Differenz (744 Backend-Aufrufe über 36 Seitenaufrufe, 42 Endpunkte). p50 und p95 sind aus den Histogramm-Buckets interpoliert.

Top nach Anzahl (alle 36 Mal, also auf jedem Seitenaufruf):

| Backend-Endpunkt                                                                                                                                                                                                                                                                                         | Anzahl | Durchschnitt | p50  | p95        |
| -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ | ------------ | ---- | ---------- |
| `GET /api/me/navigation`                                                                                                                                                                                                                                                                                 | 36     | 6 ms         | 4 ms | 16 ms      |
| `GET /api/settings/schema`                                                                                                                                                                                                                                                                               | 36     | 6 ms         | 6 ms | 16 ms      |
| `GET /api/messages/unread-count`                                                                                                                                                                                                                                                                         | 36     | 6 ms         | 4 ms | 16 ms      |
| `GET /api/active/supervisors/all`                                                                                                                                                                                                                                                                        | 36     | 5 ms         | 4 ms | 16 ms      |
| `GET /api/students/change-requests/pending-count`                                                                                                                                                                                                                                                        | 36     | 11 ms        | 9 ms | 23 ms      |
| `GET /api/me/profile`, `/api/reminders`, `/api/platform/announcements/unread`, `/api/staff-notices/today`, `/api/staff/absences/pending`, `/api/active/schulhof/status`, `/api/enrollment/admin/change-requests/pending-count`, `/api/staff-messages/unread-count`, `/api/notifications/push/public-key` | je 36  | 2 bis 4 ms   | 3 ms | 5 bis 9 ms |

Top nach p95:

| Backend-Endpunkt                             | Anzahl | Durchschnitt | p50   | p95    |
| -------------------------------------------- | ------ | ------------ | ----- | ------ |
| `GET /api/students/ogs-group-live`           | 9      | 40 ms        | 21 ms | 182 ms |
| `GET /api/timetable/templates`               | 6      | 30 ms        | 38 ms | 49 ms  |
| `GET /api/students`                          | 18     | 30 ms        | 33 ms | 48 ms  |
| `GET /api/timetable/instances`               | 9      | 30 ms        | 31 ms | 48 ms  |
| `GET /api/enrollment/phases/expiry-warnings` | 6      | 22 ms        | 21 ms | 46 ms  |
| `GET /api/active/analytics/dashboard`        | 6      | 16 ms        | 15 ms | 42 ms  |

Kosten des Proxy-Hops: für dieselben Shell-Endpunkte misst der Browser 10 bis 34 ms (Dashboard, Durchschnitt über 3 Läufe), die Proxy-Metrik 3 bis 6 ms Backend-Zeit. Der Hop plus Browser-Overhead kostet also 10 bis 30 ms pro Request. Weil die Requests parallel laufen, schlägt das auf die Wanduhr mit einem Burst von rund 250 ms durch, nicht mit 18 mal 20 ms. Kein Endpunkt ist langsam genug, um den Wasserfall zu erklären; das erklärt seine Struktur.

## Bundle

Quelle: `pnpm run perf:bundle` (`next experimental-analyze --output`, Turbopack) und `pnpm run perf:bundle-report`, das die JSON-Köpfe unter `.next/diagnostics/analyze/data/**/analyze.data` je Route summiert. 144 Seitenrouten. Größen sind Client-JavaScript (`/_next/static/chunks/*.js`), komprimiert laut Turbopack; die Zahl „über die Leitung“ aus der Trace-Tabelle liegt jeweils etwas darunter, weil nicht jeder Chunk beim ersten Aufruf geladen wird.

Die zehn schwersten Routen:

| Route                                      | Client-JS | Chunks | größte Anteile                                                                                              |
| ------------------------------------------ | --------- | ------ | ----------------------------------------------------------------------------------------------------------- |
| `/[tenant]/staff/[id]`                     | 1.182 kB  | 46     | eigener Code 286 kB, next 250 kB, recharts 135 kB, zod 95 kB, @phosphor-icons/react 77 kB, posthog-js 75 kB |
| `/[tenant]/time-tracking`                  | 1.114 kB  | 43     | next 250 kB, eigener Code 246 kB, recharts 121 kB, zod 95 kB, posthog-js 75 kB                              |
| `/[tenant]/database/students`              | 1.078 kB  | 50     | eigener Code 352 kB, next 252 kB, zod 95 kB, posthog-js 75 kB                                               |
| `/[tenant]/students/[id]`                  | 1.075 kB  | 47     | eigener Code 369 kB, next 250 kB, zod 95 kB, posthog-js 75 kB                                               |
| `/[tenant]/calendar`                       | 1.025 kB  | 46     | eigener Code 323 kB, next 252 kB, zod 95 kB                                                                 |
| `/[tenant]/students/[id]/feedback-history` | 1.017 kB  | 40     | next 250 kB, eigener Code 183 kB, recharts 121 kB                                                           |
| `/[tenant]/students/[id]/room-history`     | 1.008 kB  | 39     | next 250 kB, eigener Code 174 kB, recharts 121 kB                                                           |
| `/[tenant]/betreuungsplan`                 | 1.002 kB  | 44     | eigener Code 303 kB, next 250 kB, zod 95 kB                                                                 |
| `/[tenant]/students/search`                | 989 kB    | 43     | next 250 kB, eigener Code 234 kB, zod 95 kB, posthog-js 75 kB, motion-dom 43 kB                             |
| `/[tenant]/enrollment-phases`              | 975 kB    | 41     | next 250 kB, eigener Code 230 kB, zod 95 kB                                                                 |

Die sechs gemessenen Screens: dashboard 835 kB, active-supervisions 924 kB, students/search 989 kB, ogs-groups 946 kB, time-tracking 1.114 kB, betreuungsplan 1.002 kB.

Geteilte Chunks: drei Chunks mit 166 kB, 95 kB und 76 kB stecken in 133 bis 143 der 144 Routen (Framework, Provider, Shell); zwei weitere mit 67 kB und 60 kB in 125 bzw. 143 Routen. Diese rund 460 kB sind der Sockel, den jede Route trägt.

Auffällig:

- **zod, 95 kB auf jeder Route.** `~/env` (t3 `createEnv` mit den Schemas aus `src/lib/env-validation.js`) wird aus `src/lib/api-url.ts`, `src/lib/api-transport.ts`, `src/lib/analytics.ts`, `src/lib/student-api.ts` und `src/components/auth/tenant-auth-wrapper.tsx` importiert. Der Client braucht davon nur `NEXT_PUBLIC_*`-Strings.
- **posthog-js, 75 kB auf jeder Route**, eager aus `src/instrumentation-client.ts`.
- **recharts, 121 bis 135 kB** auf `time-tracking`, `staff/[id]`, `feedback-history` und `room-history`, statisch importiert. `next/dynamic` gibt es im Repo an zwei Stellen (`calendar/page.tsx`, `database/configs/students.config.tsx`).
- @phosphor-icons/react 72 bis 77 kB trotz `optimizePackageImports`; @sentry/core 44 kB; axios 24 kB.

## Render-Profiling (react-scan, Dev-Server)

react-scan wird per Playwright-Init-Script in den Dev-Server injiziert (`pnpm run perf:render`), die App bleibt unverändert. Gezählt werden Render-Objekte aus dem `onRender`-Callback; react-scan 0.5.7 liefert weder ein `unnecessary`-Urteil noch gefüllte `changes`, deshalb stehen hier nur Zähler. Zähler sind auf Prod übertragbar, Render-Zeiten nicht.

### Vorher: die Bottom-Nav-Schleife

Vor dem Fix in diesem PR, 40 s nach Aufruf bis Ruhe, danach 10 s Leerlauf ohne Interaktion:

| Screen              | Renders bis Ruhe | Renders im Leerlauf (10 s) | Spitzenreiter im Leerlauf                                                       |
| ------------------- | ---------------- | -------------------------- | ------------------------------------------------------------------------------- |
| dashboard           | 89.771           | 21.588                     | `LinkComponent` 3.084, `MobileNavIcon` 3.084, `SSRBase` 2.313, `Presence` 1.542 |
| active-supervisions | 89.032           | 22.246                     | `LinkComponent` 3.176                                                           |
| students/search     | 99.207           | 21.411                     | `LinkComponent` 3.172                                                           |
| ogs-groups          | 85.486           | 19.629                     | `LinkComponent` 2.908                                                           |
| time-tracking       | 94.217           | 21.681                     | `LinkComponent` 3.212                                                           |
| betreuungsplan      | 90.614           | 20.804                     | `LinkComponent` 3.080                                                           |

Rund 2.100 Renders pro Sekunde auf jeder Seite, während nichts passiert. `MobileBottomNav` selbst rendert 3.067 Mal in 40 s und zieht Links, Icons und Radix-`Presence` mit. Ursache in `src/components/dashboard/mobile-bottom-nav.tsx`: zwei `useEffect`s hängen an `displayMainItems`, das jeder Render neu filtert. Der Effekt setzt per `setTimeout` ein frisches `{ left, width }`-Objekt, React rendert, das Array ist neu, der Effekt läuft wieder. Die Nav ist auf Desktop per `lg:hidden` unsichtbar, rendert aber trotzdem.

Fix: `setIndicatorStyle` bekommt einen Updater, der das alte Objekt zurückgibt, wenn sich nichts geändert hat (`keepIfUnchanged`). Die 65 Unit-Tests der Nav laufen unverändert durch.

### Nachher

| Screen              | Komponenten | Renders bis Ruhe | Renders im Leerlauf (10 s) | Interaktion       | Renders Interaktion |
| ------------------- | ----------- | ---------------- | -------------------------- | ----------------- | ------------------- |
| dashboard           | 164         | 3.905            | 0                          |                   |                     |
| active-supervisions | 196         | 4.860            | 14                         |                   |                     |
| students/search     | 188         | 14.766           | 3.520                      | 4 Zeichen tippen  | 25.110              |
| ogs-groups          | 184         | 4.536            | 0                          |                   |                     |
| time-tracking       | 247         | 14.584           | 14                         | „Vorherige Woche“ | 7.526               |
| betreuungsplan      | 201         | 7.340            | 14                         | „Weiter“          | 2.008               |

Die Schleife ist weg (Faktor 20 bis 23 beim Aufruf, Leerlauf bei 0). Was bleibt:

- **Kindersuche, 4 Tastendrücke: 25.110 Renders.** `StudentInfoRow` 4.080, anonyme Komponenten 3.185, `MotoDuotoneIcon` 1.767, `IconBase` 1.766. `searchTerm` ist Page-State (`students/search/page.tsx:1003`); jeder Tastendruck rendert alle 100 Karten, obwohl der SWR-Key erst nach dem Debounce wechselt. `React.memo` kommt im gesamten Frontend nicht vor.
- **Kindersuche, Leerlauf: 3.520 Renders in 10 s**, `StudentInfoRow` 580. `useMinuteClock()` auf Page-Ebene (Zeile 1198) rendert bei jedem Minuten-Tick alle Karten.
- **Zeiterfassung, Wochenwechsel: 7.526 Renders**, davon je 900 für die recharts-Schichten `BarStackClipLayer`, `BarRectangleNeverActive`, `BarRectangle`. Das Chart wird bei jedem Datenwechsel komplett neu aufgebaut.
- Die 14 Leerlauf-Renders auf drei Screens sind ein einzelner `SessionProvider`-Refetch, unauffällig.

## Hypothesen-Check

| Verdacht aus #2938                                                                          | Urteil                    | Beleg                                                                                                                                                                                                                                                                                   |
| ------------------------------------------------------------------------------------------- | ------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Praktisch jede Page ist `"use client"`, Server Components werden fürs Fetching kaum genutzt | **bestätigt, mit Folge**  | 649 Dateien mit `"use client"`, 158 unter `src/app`. TTFB 8 bis 19 ms, aber erster API-Request erst bei 725 bis 923 ms (nach Hydration), LCP 0,9 bis 1,4 s. Der Server liefert eine leere Hülle, alle Daten kommen nach der Hydration.                                                  |
| Der Proxy-Hop über 692 Route-Handler kostet                                                 | **widerlegt als Engpass** | Backend p50 3 bis 8 ms, Hop 10 bis 30 ms pro Request, parallel. Ein Burst aus 20 Requests dauert rund 250 ms. Die Zahl der Requests ist das Problem, nicht der Hop.                                                                                                                     |
| Kein globaler `SWRConfig`, deshalb kein Deduping                                            | **widerlegt**             | Alle 163 `useSWRAuth`-Aufrufe mergen `swrConfig` (`dedupingInterval` 5.000 ms, `keepPreviousData`). In 36 Seitenaufrufen kein einziger doppelter SWR-Key. Das einzige Duplikat, `/api/auth/session` mal 4, ist NextAuth. Ein globaler `SWRConfig` würde nichts ändern; nicht umgesetzt. |
| 0 mal `React.memo`, 2 mal `next/dynamic`, 616 mal `useMemo`                                 | **bestätigt**             | 25.110 Renders für 4 Tastendrücke, 7.526 für einen Wochenwechsel; recharts 121 bis 135 kB in der Erstlast von vier Routen. Dazu ein unerwarteter Befund: die Nav-Schleife mit 2.100 Renders pro Sekunde, die kein `memo` der Welt verhindert hätte, sondern ein stabiler Effekt.        |

## Priorisierte Fix-Liste

| Nr. | Maßnahme                                                                                                                            | Erwarteter Effekt                                                                                           | Aufwand          | Status                |
| --- | ----------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ---------------- | --------------------- |
| 1   | `MobileBottomNav`: Indikator-State nur bei Änderung setzen                                                                          | Leerlauf-Renders 21.588 in 10 s auf 0; Aufruf-Renders auf dem Dashboard 89.771 auf 3.905 (gemessen)         | 15 Zeilen        | **in diesem PR**      |
| 2   | Shell-Daten bündeln (Bootstrap-Endpunkt oder Server-Layout), Session-Fetch auf einen reduzieren, Zähler nach dem ersten Paint laden | 18 Shell-Requests auf 1, Session-Fetches auf 0, Seitendaten 333 ms früher (gemessen, siehe Nachmessung)     | mittel           | #2973 **erledigt**    |
| 3   | Client-Env ohne zod, Charts per `next/dynamic`, PostHog nach dem ersten Paint                                                       | minus 95 kB auf jeder Route, minus 121 bis 135 kB auf vier Routen, minus 75 kB auf dem kritischen Pfad      | klein bis mittel | #2974                 |
| 4   | Kindersuche: Suchfeld-State isolieren, Karten memoisieren, `useMinuteClock` aus der Page ziehen; Zeiterfassung: Chart memoisieren   | Kindersuche unter 1.000 Renders pro Tastendruck (heute rund 6.300), Wochenwechsel unter 2.000 (heute 7.526) | mittel           | #2975                 |
| 5   | Sidebar-Links ohne automatischen Prefetch                                                                                           | 10 bis 28 abgebrochene Requests pro Seitenaufruf weg (gemessen: 0 auf 17 von 18 kalten Aufrufen)            | klein            | **umgesetzt, #2976**  |
| 6   | Bundle-Ratchet und Lighthouse-Report in CI                                                                                          | schützt 1 bis 5                                                                                             |                  | #2939 (bestand schon) |

Bewusst nicht umgesetzt: ein globaler `SWRConfig` (kein Effekt messbar) und `next/dynamic` in diesem PR (das Chart in der Zeiterfassung liegt in einer 4.072-Zeilen-Page und gehört mit den anderen drei Konsumenten in einen Wurf, #2974).

## Anhang: Reproduktion

Der Harness liegt unter `frontend/scripts/perf/`, die Configs daneben. Alle Artefakte landen in `frontend/perf-results/` (gitignored: Traces, Roh-JSON, Session-Cookies).

`PERF_PORT` muss bei jedem Lauf gesetzt sein. Der gewählte Port muss frei sein.

```bash
cd frontend

# 1. Prod-Traces und Proxy-Metriken (Port 3000 muss frei sein)
docker compose -f ../docker-compose.yml stop frontend
PERF_PORT=3000 pnpm run perf:trace # next build && next start, 6 Screens x 3 Läufe, Traces nach perf-results/traces/
docker compose -f ../docker-compose.yml start frontend

# 2. Bundle
pnpm run perf:bundle         # next experimental-analyze --output  ->  .next/diagnostics/analyze
pnpm run perf:bundle-report  # Tabelle je Route nach perf-results/bundle.md

# 3. Render-Zählung gegen den Dev-Server
PERF_PORT=3000 pnpm run perf:render # react-scan per Init-Script, JSON nach perf-results/react-scan/

# 3b. Seitenwechsel (#2828), eigener Port, eigener Server
PERF_PORT=3828 pnpm run perf:nav    # JSON nach perf-results/navigation/
PERF_NAV_SCREENSHOTS=1 PERF_PORT=3828 pnpm run perf:nav # zusätzlich ein Bild kurz nach jedem Klick

# 4. Alles zu Markdown
pnpm run perf:report         # perf-results/report.md
```

Traces öffnen: `pnpm exec playwright show-trace perf-results/traces/<screen>-<lauf>.zip`.

| Datei                             | Zweck                                                                                                                                                                  |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `playwright.perf.config.ts`       | Prod-Runner: build und start, `reuseExistingServer: false`, damit nie versehentlich gegen `next dev` gemessen wird                                                     |
| `playwright.perf-dev.config.ts`   | Dev-Runner für react-scan, nutzt einen laufenden Dev-Server                                                                                                            |
| `scripts/perf/env.mjs`            | Liest `.env.local`, `.env`, `../.env` und reicht nur die nötigen Schlüssel an den Server durch (ein `PORT=8080` aus der Shell würde sonst den Next-Server verschieben) |
| `scripts/perf/access.ts`          | Slug und Admin-Login aus `backend/.seed-state.json`, Login über die Tenant-Wurzel                                                                                      |
| `scripts/perf/targets.ts`         | Die sechs Screens, ihre Ready-Selektoren und Interaktionen                                                                                                             |
| `scripts/perf/recorder.ts`        | Request-Recorder, Ruhe-Kriterium, Vitals-Observer, Auswertung (Duplikate, Ketten, Bytes)                                                                               |
| `scripts/perf/measure.perf.ts`    | Pro Screen kalt, Interaktion, warm; Metrik-Scrape vor und nach dem Lauf                                                                                                |
| `playwright.perf-nav.config.ts`   | Prod-Runner für die Seitenwechsel-Messung (`pnpm run perf:nav`)                                                                                                       |
| `scripts/perf/navigation.perf.ts` | Rundgang über Seitenleisten-Links mit Leitungsbremse; zählt Dokument-, RSC- und API-Requests je Wechsel, prüft, ob die Hülle überlebt, und hält fest, welche Ladehüllen sichtbar waren |
| `scripts/perf/react-scan.perf.ts` | Render-Zählung: Aufruf, 10 s Leerlauf, Interaktion                                                                                                                     |
| `scripts/perf/bundle-report.mjs`  | Per-Route-Tabelle aus der Turbopack-Analyse                                                                                                                            |
| `scripts/perf/report.mjs`         | Median-Tabellen, Wasserfall-Details, Prometheus-Diff, Render-Tabellen                                                                                                  |

Grenzen der Messung:

- Ein Gerät, lokales Backend, ein Konto (Admin). Ein Betreuer-Konto sieht weniger Navigation und andere Shell-Requests.
- Die 4x-Drosselung ist eine Annahme, keine Messung eines Schul-Geräts. Für einen Vergleich mit echten Geräten den Harness dort ausführen und `CPU_THROTTLE` in `measure.perf.ts` auf 1 setzen.
- react-scan misst nur den Dev-Build. Die Render-Zähler stimmen, die Render-Zeiten aus dem Dev-Build stehen deshalb nirgends in diesem Dokument.
- INP wurde nicht gemessen (headless ohne echte Eingabe-Latenz); TBT ist aus Long Tasks genähert.
- Das Bundle-Format von `next experimental-analyze` ist als experimentell markiert; `bundle-report.mjs` liest nur den JSON-Kopf und bricht laut ab, wenn sich das Format ändert.
