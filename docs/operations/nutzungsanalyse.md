# Usage analytics: PostHog project and privacy text

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
   unbereinigte HAR-Datei ablegen. Netzwerknachweis bleibt Slice 3843-c.

**Lokal geprüft am 23.10.2026:** Nativer Entwicklungsstack, vollständig
synthetische Seed-Daten, Test-Platzhalter statt Analyse-Schlüssel und
abgefangene `/ingest`-Anfragen. Positive Kontrolle mit eingeschalteter Analyse:
Anfragen an den lokalen Stub vorhanden. Gegenprobe: angemeldetes OGS-Portal
von `manuell` und aktives öffentliches Anmeldeformular von `vollbetrieb`
(nach Ausschalten über die Einstellungen), einschließlich Formularinteraktion:
0 `/ingest`-Anfragen. Die öffentliche Gegenprobe verwendete ein frisch
aufgerufenes Formular. Keine PostHog-Anfrage mit echtem Schlüssel.
Schulschalter und getrennte Fehlerdiagnose auf Desktop und Mobilgerät angesehen.
Der Netzwerknachweis aller Portale bleibt Slice 3843-c.

**Produktiv nachzuweisen, offen:** Auslieferung und tatsächlicher Netzwerknachweis
je Portal. Ein lokaler Test bestätigt keine produktive Abschaltung.
Anbieter erst nach bestätigter Abschaltung aus Vertragsunterlagen streichen.
Rechtsgrundlagen, Rollen und § 25 TDDDG bleiben Recht-Slice 3843-e; der folgende
lit.-f-Entwurf bleibt unverändert und ist keine Freigabe.

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

| Dashboard | Deployment | Link |
| --- | --- | --- |
| Demo-Funnel | `demo` | <https://eu.posthog.com/project/140838/dashboard/972368> |
| Nutzung pro Rolle und Oberfläche | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/972369> |
| Seiten (meistbesucht, kaum genutzt) | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/972370> |
| Reibung (Dead und Rage Clicks, Heatmaps) | `moto-app.de` and `demo`, separate tables | <https://eu.posthog.com/project/140838/dashboard/972371> |
| Aktive Schulen | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/972372> |
| Funktionen in Schulen (Reichweite, je Schule, nach Rolle, pro Woche) | `moto-app.de` | <https://eu.posthog.com/project/140838/dashboard/1003811> |
| Demo-Verhalten (letzte Seite, Sitzungstiefe, Funktionen, Aktionen) | `demo` | <https://eu.posthog.com/project/140838/dashboard/1003812> |

The feature areas (Funktionsbereiche) are `FEATURES` in the script; the test
fails for a route template without an area, so a new page needs one.

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
