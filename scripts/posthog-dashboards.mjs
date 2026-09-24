#!/usr/bin/env node
// Dashboards of the usage analytics (Nutzungsanalyse, #3598 / #3604) in
// PostHog, created and updated through the PostHog EU API.
//
//   node scripts/posthog-dashboards.mjs            create or update everything
//   node scripts/posthog-dashboards.mjs --dry-run  show the plan, write nothing
//   node scripts/posthog-dashboards.mjs --check    run every insight query once
//
// Needs POSTHOG_PERSONAL_API_KEY in the local environment (never in the
// repository), scoped to the project with query:read, insight:read/write, and
// dashboard:read/write. POSTHOG_PROJECT_ID picks the project when the key sees
// more than one. Only the EU API is ever called.
//
// Idempotent: dashboards and insights are found by their exact name and
// updated in place; a run changes nothing that is already current. A renamed
// definition creates a new object; delete the old one in PostHog by hand.
//
// Every insight filters on one deployment, production (`moto-app.de`) or the
// public demo (`demo`); staging and local development never count. No
// insight looks at a single person: the smallest unit is the role or the
// school (`posthog-dashboards.test.mjs` enforces both).
//
// Adding a dashboard or an insight: add it to `dashboards()` below, build its
// query with `hogql()`, `trends()`, or `funnel()` (they add the deployment
// filter), run the tests (`node --test scripts/posthog-dashboards.test.mjs`)
// and then this script. Details: .claude/rules/usage-analytics.md.

import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const POSTHOG_HOST = 'https://eu.posthog.com';

/** `deployment` values (analytics-policy.ts, backend/analytics/analytics.go). */
export const DEPLOYMENTS = Object.freeze({ production: 'moto-app.de', demo: 'demo' });

/** Portals of real schools; `public` covers login, start, and demo pages. */
const PORTALS = ['ogs', 'parents', 'school'];

const ROUTE_LISTS = {
  ogs: ['TRACKED_TENANT_ROUTE_TEMPLATES', 'TRACKED_TENANT_PUBLIC_ROUTE_TEMPLATES'],
  parents: ['TRACKED_PARENT_ROUTE_TEMPLATES'],
  school: ['TRACKED_SCHOOL_ROUTE_TEMPLATES'],
};

const repositoryRoot = resolve(fileURLToPath(import.meta.url), '..', '..');

/**
 * The route templates of every portal, read from analytics-routes.ts, so the
 * page list also names pages nobody opened.
 */
export function readRouteTemplates(
  source = readFileSync(resolve(repositoryRoot, 'frontend/src/lib/analytics-routes.ts'), 'utf8'),
) {
  const routes = [];
  for (const [surface, lists] of Object.entries(ROUTE_LISTS)) {
    for (const list of lists) {
      const body = new RegExp(`export const ${list} = \\[([\\s\\S]*?)\\] as const;`).exec(source)?.[1];
      if (!body) throw new Error(`analytics-routes.ts has no list ${list}`);
      for (const [, template] of body.matchAll(/^\s*"([^"]+)",/gm)) {
        routes.push([surface, template]);
      }
    }
  }
  return routes;
}

/**
 * Events the browser sends itself (`CUSTOM_EVENTS` in analytics-policy.ts).
 * Every other event without `$` is a core action from the backend.
 */
export function readBrowserEvents(
  source = readFileSync(resolve(repositoryRoot, 'frontend/src/lib/analytics-policy.ts'), 'utf8'),
) {
  const body = /const CUSTOM_EVENTS[^=]*= new Set\(\[([\s\S]*?)\]\);/.exec(source)?.[1];
  const events = body ? [...body.matchAll(/^\s*"([a-z_]+)",/gm)].map(([, event]) => event) : [];
  if (events.length === 0) throw new Error('analytics-policy.ts has no CUSTOM_EVENTS');
  return events;
}

// --- query builders ----------------------------------------------------------

const sqlString = (value) => `'${String(value).replaceAll('\\', '\\\\').replaceAll("'", "\\'")}'`;
const sqlList = (values) => `(${values.map(sqlString).join(', ')})`;

/** HogQL condition on the deployment; every HogQL insight uses it. */
const onDeployment = (deployment) => `properties.deployment = ${sqlString(DEPLOYMENTS[deployment])}`;

function deploymentOf(deployment) {
  if (!Object.hasOwn(DEPLOYMENTS, deployment)) throw new Error(`unknown deployment ${deployment}`);
  return deployment;
}

/**
 * A HogQL table (or number). `sql` receives the deployment condition and
 * must use it in every `from events`.
 */
function hogql(deployment, sql, display = 'ActionsTable') {
  const query = sql(onDeployment(deploymentOf(deployment))).trim();
  return {
    deployment,
    query: { kind: 'DataVisualizationNode', source: { kind: 'HogQLQuery', query }, display },
  };
}

function deploymentFilter(deployment) {
  return {
    type: 'AND',
    values: [{
      type: 'AND',
      values: [{ key: 'deployment', type: 'event', operator: 'exact', value: [DEPLOYMENTS[deploymentOf(deployment)]] }],
    }],
  };
}

const series = (event, name, extra = {}) => ({
  kind: 'EventsNode', event, name: event ?? 'All events', custom_name: name, math: 'total', ...extra,
});

function trends(deployment, { series: steps, interval = 'week', dateFrom = '-90d', breakdown, display = 'ActionsLineGraph' }) {
  return {
    deployment,
    query: {
      kind: 'InsightVizNode',
      source: {
        kind: 'TrendsQuery',
        series: steps,
        interval,
        dateRange: { date_from: dateFrom },
        properties: deploymentFilter(deployment),
        filterTestAccounts: false,
        ...(breakdown ? { breakdownFilter: { breakdown, breakdown_type: 'event' } } : {}),
        trendsFilter: { display },
      },
    },
  };
}

function funnel(deployment, { series: steps, dateFrom = '-30d', windowDays = 14 }) {
  return {
    deployment,
    query: {
      kind: 'InsightVizNode',
      source: {
        kind: 'FunnelsQuery',
        series: steps,
        dateRange: { date_from: dateFrom },
        properties: deploymentFilter(deployment),
        filterTestAccounts: false,
        funnelsFilter: { funnelVizType: 'steps', funnelWindowInterval: windowDays, funnelWindowIntervalUnit: 'day' },
      },
    },
  };
}

/** Heatmap of a route template (PostHog's own link format, `urls.heatmapNew`). */
function heatmapLink(projectId, deployment, pathExpression) {
  const url = `concat(${sqlString(`https://${DEPLOYMENTS[deployment]}`)}, ${pathExpression})`;
  return `concat(${sqlString(`${POSTHOG_HOST}/project/${projectId}/heatmaps/new?pageURL=`)}, encodeURLComponent(${url}), '&dataURL=', encodeURLComponent(${url}))`;
}

// --- definitions -------------------------------------------------------------

const portalCondition = `properties.surface in ${sqlList(PORTALS)}`;

function frictionTable(projectId, deployment, surfaces) {
  return hogql(deployment, (where) => `
select
  properties.surface as \`Oberfläche\`,
  properties.$pathname as Seite,
  countIf(event = '$pageview') as Aufrufe,
  countIf(event = '$dead_click') as \`Dead Clicks\`,
  countIf(event = '$rageclick') as \`Rage Clicks\`,
  round(100 * (countIf(event = '$dead_click') + countIf(event = '$rageclick')) / greatest(countIf(event = '$pageview'), 1), 1) as \`Reibung je 100 Aufrufe\`,
  ${heatmapLink(projectId, deployment, 'properties.$pathname')} as Heatmap
from events
where ${where}
  and properties.surface in ${sqlList(surfaces)}
  and event in ('$pageview', '$dead_click', '$rageclick')
  and timestamp > now() - interval 30 day
group by \`Oberfläche\`, Seite
having \`Dead Clicks\` + \`Rage Clicks\` > 0
order by \`Dead Clicks\` + \`Rage Clicks\` desc
limit 50`);
}

/** The five dashboards (#3604). `projectId` feeds the heatmap links. */
export function dashboards({ projectId, routes = readRouteTemplates(), browserEvents = readBrowserEvents() }) {
  const coreAction = `not startsWith(event, '$') and event not in ${sqlList(browserEvents)}`;
  const routeRows = routes.map(([surface, template]) => `(${sqlString(surface)}, ${sqlString(template)})`).join(', ');

  return [
    {
      name: 'Nutzungsanalyse: Demo-Funnel',
      description: 'Wo Interessenten in der öffentlichen Demo aussteigen. Nur deployment = demo. „Start geklickt“ auf der Website misst Umami, nicht PostHog.',
      insights: [
        {
          name: 'Demo-Funnel: Schritte der letzten 30 Tage',
          description: 'Link angefordert und Demo-Schule bereitgestellt kommen aus dem Backend, die übrigen Schritte aus dem Browser. Die Schritte laufen über drei Adressen, deshalb zählt die Tabelle Ereignisse je Schritt statt einzelner Besuche.',
          ...hogql('demo', (where) => `
select Schritt, Anzahl, round(100 * Anzahl / greatest(sum(if(Nr = 1, Anzahl, 0)) over (), 1)) as \`Prozent der Links\`
from (
  select 1 as Nr, 'Link angefordert' as Schritt, countIf(event = 'demo_link_requested') as Anzahl from events where ${where} and timestamp > now() - interval 30 day
  union all
  select 2, 'Link geöffnet (Warteraum)', countIf(event = '$pageview' and properties.surface = 'public' and properties.$pathname = '/demo') from events where ${where} and timestamp > now() - interval 30 day
  union all
  select 3, 'Demo-Schule bereitgestellt (Backend)', countIf(event = 'demo_started') from events where ${where} and timestamp > now() - interval 30 day
  union all
  select 4, 'Eingestiegen', countIf(event = 'demo_entered') from events where ${where} and timestamp > now() - interval 30 day
  union all
  select 5, 'Rolle gewechselt', countIf(event = 'demo_role_switched') from events where ${where} and timestamp > now() - interval 30 day
  union all
  select 6, 'Kostenlos starten', countIf(event = 'demo_start_clicked') from events where ${where} and timestamp > now() - interval 30 day
)
order by Nr`),
        },
        {
          name: 'Demo-Funnel: Schritte pro Woche',
          ...trends('demo', {
            series: [
              series('demo_link_requested', 'Link angefordert'),
              series('demo_started', 'Demo-Schule bereitgestellt'),
              series('demo_entered', 'Eingestiegen'),
              series('demo_role_switched', 'Rolle gewechselt'),
              series('demo_start_clicked', 'Kostenlos starten'),
            ],
          }),
        },
        {
          name: 'Demo-Funnel: Einstieg bis Kostenlos starten',
          description: 'Folgt einem Demo-Zugang (distinct_id ist der Zugang, keine Person) vom Einstieg über den Rollenwechsel bis „Kostenlos starten“.',
          ...funnel('demo', {
            series: [
              series('demo_entered', 'Eingestiegen'),
              series('demo_role_switched', 'Rolle gewechselt'),
              series('demo_start_clicked', 'Kostenlos starten'),
            ],
          }),
        },
        {
          name: 'Demo: Einstiege nach Rolle und Quelle',
          description: 'Demo-Rolle und Kampagnenquelle (src) der letzten 30 Tage.',
          ...hogql('demo', (where) => `
select
  coalesce(properties.demo_role, 'unbekannt') as \`Demo-Rolle\`,
  coalesce(properties.src, 'ohne') as Quelle,
  countIf(event = 'demo_entered') as Einstiege,
  countIf(event = 'demo_role_switched') as Rollenwechsel,
  countIf(event = 'demo_start_clicked') as \`Kostenlos starten\`
from events
where ${where}
  and event in ('demo_entered', 'demo_role_switched', 'demo_start_clicked')
  and timestamp > now() - interval 30 day
group by \`Demo-Rolle\`, Quelle
order by Einstiege desc`),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Nutzung pro Rolle und Oberfläche',
      description: 'Welche Rolle welche Oberfläche wie oft nutzt, in echten Schulen (deployment = moto-app.de). Kleinste Einheit ist die Rolle.',
      insights: [
        {
          name: 'Nutzung: Seitenaufrufe pro Oberfläche und Woche',
          ...trends('production', {
            series: [series('$pageview', 'Seitenaufrufe', {
              properties: [{ key: 'surface', type: 'event', operator: 'exact', value: PORTALS }],
            })],
            breakdown: 'surface',
          }),
        },
        {
          name: 'Nutzung: Rolle und Oberfläche der letzten 30 Tage',
          description: 'Sitzungen zählen Browser-Sitzungen, nicht Personen. Kernaktionen sind die erfolgreichen Schreibvorgänge, die das Backend meldet.',
          ...hogql('production', (where) => `
select
  properties.surface as \`Oberfläche\`,
  coalesce(properties.role, 'ohne Anmeldung') as Rolle,
  countIf(event = '$pageview') as Seitenaufrufe,
  uniqIf(properties.$session_id, event = '$pageview') as Sitzungen,
  countIf(${coreAction}) as Kernaktionen,
  uniq(toString(properties.school_id)) as Schulen
from events
where ${where}
  and ${portalCondition}
  and timestamp > now() - interval 30 day
group by \`Oberfläche\`, Rolle
order by Seitenaufrufe desc`),
        },
        {
          name: 'Nutzung: Kernaktionen nach Oberfläche und Rolle',
          description: 'Erfolgreiche Schreibvorgänge der letzten 30 Tage (backend/analytics/core_actions.go).',
          ...hogql('production', (where) => `
select
  properties.surface as \`Oberfläche\`,
  coalesce(properties.role, 'ohne Anmeldung') as Rolle,
  event as Kernaktion,
  count() as Anzahl,
  uniq(toString(properties.school_id)) as Schulen
from events
where ${where}
  and ${portalCondition}
  and ${coreAction}
  and timestamp > now() - interval 30 day
group by \`Oberfläche\`, Rolle, Kernaktion
order by Anzahl desc
limit 100`),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Seiten',
      description: 'Meistbesuchte und kaum genutzte Seiten als Routenmuster (frontend/src/lib/analytics-routes.ts), in echten Schulen (deployment = moto-app.de).',
      insights: [
        {
          name: 'Seiten: meistbesucht in den letzten 30 Tagen',
          description: 'Verweildauer ist der Median in Sekunden, gemessen beim Verlassen der Seite.',
          ...hogql('production', (where) => `
select
  properties.surface as \`Oberfläche\`,
  properties.$pathname as Seite,
  countIf(event = '$pageview') as Aufrufe,
  uniqIf(properties.$session_id, event = '$pageview') as Sitzungen,
  uniqIf(toString(properties.school_id), event = '$pageview') as Schulen,
  round(quantileIf(0.5)(toFloat(properties.$prev_pageview_duration), event = '$pageleave')) as \`Verweildauer (s)\`
from events
where ${where}
  and ${portalCondition}
  and event in ('$pageview', '$pageleave')
  and timestamp > now() - interval 30 day
group by \`Oberfläche\`, Seite
order by Aufrufe desc
limit 50`),
        },
        {
          name: 'Seiten: kaum oder nie genutzt in den letzten 30 Tagen',
          description: 'Jede Seite aus analytics-routes.ts, aufsteigend nach Aufrufen; 0 heißt nie geöffnet. Das Skript liest die Liste bei jedem Lauf neu.',
          ...hogql('production', (where) => `
select \`Oberfläche\`, Seite, coalesce(Aufrufe, 0) as Aufrufe, coalesce(Schulen, 0) as Schulen
from (
  select tupleElement(route, 1) as \`Oberfläche\`, tupleElement(route, 2) as Seite
  from (select arrayJoin([${routeRows}]) as route)
) as pages
left join (
  select properties.surface as surface, properties.$pathname as path, count() as Aufrufe, uniq(toString(properties.school_id)) as Schulen
  from events
  where ${where}
    and event = '$pageview'
    and ${portalCondition}
    and timestamp > now() - interval 30 day
  group by surface, path
) as seen on pages.\`Oberfläche\` = seen.surface and pages.Seite = seen.path
order by Aufrufe asc, \`Oberfläche\`, Seite
limit 300`),
        },
        {
          name: 'Seiten: Aufrufe ohne Routenmuster',
          description: 'Seiten ohne Muster kommen als /unknown an. Steigt die Zahl, fehlt in analytics-routes.ts ein Eintrag.',
          ...trends('production', {
            series: [series('$pageview', 'Seiten ohne Muster', {
              properties: [{ key: '$pathname', type: 'event', operator: 'exact', value: ['/unknown'] }],
            })],
            breakdown: 'surface',
          }),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Reibung',
      description: 'Dead Clicks (Klick ohne Wirkung) und Rage Clicks (schnelle Mehrfachklicks) nach Seite, mit Link zur Heatmap der Seite. Schulen (moto-app.de) und Demo getrennt.',
      insights: [
        {
          name: 'Reibung: Seiten in Schulen',
          description: 'Letzte 30 Tage. Die Heatmap öffnet das Routenmuster; Seiten mit gleichem Muster in zwei Portalen teilen sich eine Heatmap.',
          ...frictionTable(projectId, 'production', PORTALS),
        },
        {
          name: 'Reibung: Seiten in der Demo',
          description: 'Letzte 30 Tage, öffentliche Demo.',
          ...frictionTable(projectId, 'demo', [...PORTALS, 'public']),
        },
        {
          name: 'Reibung: Dead und Rage Clicks pro Woche in Schulen',
          ...trends('production', {
            series: [series('$dead_click', 'Dead Clicks'), series('$rageclick', 'Rage Clicks')],
          }),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Aktive Schulen',
      description: 'Schulen mit mindestens einem Ereignis (Seitenaufruf, Kernaktion oder Kiosk) pro Woche, in echten Schulen (deployment = moto-app.de).',
      insights: [
        {
          name: 'Aktive Schulen: letzte 7 Tage',
          ...hogql('production', (where) => `
select uniq(toString(properties.school_id)) as \`Aktive Schulen\`
from events
where ${where}
  and isNotNull(properties.school_id)
  and timestamp > now() - interval 7 day`, 'BoldNumber'),
        },
        {
          name: 'Aktive Schulen pro Woche',
          ...trends('production', {
            series: [series(null, 'Aktive Schulen', {
              math: 'hogql',
              math_hogql: "uniqIf(toString(properties.school_id), isNotNull(properties.school_id))",
            })],
            dateFrom: '-180d',
            display: 'ActionsBar',
          }),
        },
        {
          name: 'Aktive Schulen pro Woche und Oberfläche',
          description: 'Wie viele Schulen jede Oberfläche in einer Woche genutzt haben.',
          ...hogql('production', (where) => `
select
  toStartOfWeek(timestamp, 1) as Woche,
  uniq(toString(properties.school_id)) as \`Aktive Schulen\`,
  uniqIf(toString(properties.school_id), properties.surface = 'ogs') as \`OGS-Portal\`,
  uniqIf(toString(properties.school_id), properties.surface = 'parents') as \`Eltern-Portal\`,
  uniqIf(toString(properties.school_id), properties.surface = 'school') as \`Schul-Portal\`
from events
where ${where}
  and isNotNull(properties.school_id)
  and timestamp > now() - interval 12 week
group by Woche
order by Woche desc`),
        },
      ],
    },
  ];
}

// --- sync --------------------------------------------------------------------

/** Minimal PostHog client; `request` is swappable for tests. */
export function posthogClient({ apiKey, request = fetch }) {
  if (!apiKey) throw new Error('POSTHOG_PERSONAL_API_KEY is not set');
  return async (method, path, body) => {
    const response = await request(`${POSTHOG_HOST}${path}`, {
      method,
      headers: { Authorization: `Bearer ${apiKey}`, 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    const text = await response.text();
    if (!response.ok) throw new Error(`PostHog ${method} ${path}: ${response.status} ${text.slice(0, 500)}`);
    return text ? JSON.parse(text) : null;
  };
}

async function listAll(api, path) {
  const results = [];
  let next = path;
  while (next) {
    const page = await api('GET', next);
    results.push(...page.results);
    next = page.next ? new URL(page.next).pathname + new URL(page.next).search : null;
  }
  return results;
}

export async function resolveProjectId(api, configured = process.env.POSTHOG_PROJECT_ID) {
  if (configured) return String(configured);
  const projects = await listAll(api, '/api/projects/');
  if (projects.length !== 1) {
    throw new Error(`The key sees ${projects.length} projects; set POSTHOG_PROJECT_ID`);
  }
  return String(projects[0].id);
}

/**
 * First path where `actual` lacks a value of `expected`, or null. PostHog
 * fills in defaults when it stores a query, so a stored query that contains
 * every value of the definition is current.
 */
export function missingPath(expected, actual, path = 'query') {
  if (expected === null || typeof expected !== 'object') {
    return expected === actual ? null : path;
  }
  if (actual === null || typeof actual !== 'object' || Array.isArray(expected) !== Array.isArray(actual)) return path;
  if (Array.isArray(expected) && expected.length !== actual.length) return path;
  for (const [key, value] of Object.entries(expected)) {
    const missing = missingPath(value, actual[key], `${path}.${key}`);
    if (missing) return missing;
  }
  return null;
}

/**
 * Creates or updates every dashboard and insight. Returns what it did, one
 * line per object; `dryRun` only reports.
 */
export async function sync(api, projectId, definitions, { dryRun = false } = {}) {
  const base = `/api/projects/${projectId}`;
  const log = [];
  const existingDashboards = (await listAll(api, `${base}/dashboards/?limit=200`)).filter((d) => !d.deleted);

  for (const definition of definitions) {
    let dashboard = existingDashboards.find((d) => d.name === definition.name);
    const dashboardBody = { name: definition.name, description: definition.description };
    if (!dashboard) {
      log.push(`create dashboard ${definition.name}`);
      dashboard = dryRun ? { id: null } : await api('POST', `${base}/dashboards/`, dashboardBody);
    } else if (dashboard.description !== definition.description) {
      log.push(`update dashboard ${definition.name}`);
      if (!dryRun) await api('PATCH', `${base}/dashboards/${dashboard.id}/`, dashboardBody);
    }

    for (const insight of definition.insights) {
      const found = (await listAll(api, `${base}/insights/?saved=true&limit=100&search=${encodeURIComponent(insight.name)}`))
        .filter((candidate) => !candidate.deleted && candidate.name === insight.name);
      const current = found[0];
      const body = { name: insight.name, description: insight.description ?? '', query: insight.query, saved: true };
      if (!current) {
        log.push(`create insight ${insight.name}`);
        if (!dryRun) await api('POST', `${base}/insights/`, { ...body, dashboards: [dashboard.id] });
        continue;
      }
      const dashboardIds = current.dashboards ?? [];
      const onDashboard = dashboard.id !== null && dashboardIds.includes(dashboard.id);
      const changed = missingPath(insight.query, current.query);
      if (onDashboard && !changed && (current.description ?? '') === body.description) {
        continue;
      }
      log.push(`update insight ${insight.name}${changed ? ` (${changed})` : ''}`);
      if (!dryRun) {
        await api('PATCH', `${base}/insights/${current.id}/`, {
          ...body,
          dashboards: onDashboard ? dashboardIds : [...dashboardIds, dashboard.id],
        });
      }
    }
  }
  return log;
}

/** Runs every insight query once; a query PostHog rejects fails the run. */
export async function check(api, projectId, definitions) {
  const log = [];
  let failed = false;
  for (const definition of definitions) {
    for (const insight of definition.insights) {
      try {
        const result = await api('POST', `/api/projects/${projectId}/query/`, { query: insight.query.source });
        log.push(`ok   ${insight.name}: ${result.results?.length ?? 0} rows`);
      } catch (error) {
        failed = true;
        log.push(`FAIL ${insight.name}: ${error.message}`);
      }
    }
  }
  return { log, failed };
}

async function main() {
  const dryRun = process.argv.includes('--dry-run');
  const api = posthogClient({ apiKey: process.env.POSTHOG_PERSONAL_API_KEY });
  const projectId = await resolveProjectId(api);
  if (process.argv.includes('--check')) {
    const { log, failed } = await check(api, projectId, dashboards({ projectId }));
    console.log(log.join('\n'));
    process.exitCode = failed ? 1 : 0;
    return;
  }
  const log = await sync(api, projectId, dashboards({ projectId }), { dryRun });
  console.log(log.length ? log.join('\n') : 'Everything is current.');
  const all = await listAll(api, `/api/projects/${projectId}/dashboards/?limit=200`);
  for (const definition of dashboards({ projectId })) {
    const dashboard = all.find((d) => !d.deleted && d.name === definition.name);
    if (dashboard) console.log(`${definition.name}: ${POSTHOG_HOST}/project/${projectId}/dashboard/${dashboard.id}`);
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main().catch((error) => {
    console.error(error.message);
    process.exit(1);
  });
}
