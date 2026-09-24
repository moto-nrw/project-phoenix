# Usage analytics: PostHog project and privacy text

The code rules of the usage analytics live in `.claude/rules/usage-analytics.md`
and `frontend/src/lib/analytics-policy.ts`. This page lists what lives outside
the repository: the PostHog project settings the code relies on, and the
paragraphs the privacy policy on the website needs (spec #3598, #3603).

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

`scripts/posthog-dashboards.mjs` creates and updates the five dashboards; how
to add one is in `.claude/rules/usage-analytics.md`. The script needs a
personal API key in `POSTHOG_PERSONAL_API_KEY`, kept in the local
environment only (for example `~/.config/project-phoenix/local.env`), scoped
to the project with `query:read`, `insight:read`, `insight:write`,
`dashboard:read`, and `dashboard:write`.

| Dashboard | Deployment | Link |
| --- | --- | --- |
| Demo-Funnel | `demo` | <https://eu.posthog.com/project/140838/dashboard/972368> |
| Nutzung pro Rolle und Oberfläche | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/972369> |
| Seiten (meistbesucht, kaum genutzt) | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/972370> |
| Reibung (Dead und Rage Clicks, Heatmaps) | `moto-app.de` and `demo`, separate tables | <https://eu.posthog.com/project/140838/dashboard/972371> |
| Aktive Schulen | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/972372> |

The demo funnel starts with the link request (`demo_link_requested` from the
backend). The click on „Demo starten" before it happens on the website,
which measures with Umami, not PostHog.

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
