#!/usr/bin/env node
// Dashboards of the usage analytics (Nutzungsanalyse, #3598 / #3604) in
// PostHog, created and updated through the PostHog EU API.
//
//   node scripts/posthog-dashboards.mjs            create or update everything
//   node scripts/posthog-dashboards.mjs --dry-run  show the plan, write nothing
//   node scripts/posthog-dashboards.mjs --check    run every insight query once
//
// .github/workflows/posthog-dashboards.yml runs the sync on every merge into
// development, and --check and --dry-run on pull requests; a local run uses
// only the two read-only modes. Needs POSTHOG_PERSONAL_API_KEY (a GitHub
// secret in CI, the local environment otherwise, never the repository),
// scoped to the project with query:read, insight:read/write, and
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
// and open a pull request. Details: .claude/rules/usage-analytics.md.

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

/** Easter Sunday of a year (Gregorian, anonymous algorithm), as UTC date. */
function easterSunday(year) {
  const a = year % 19;
  const b = Math.floor(year / 100);
  const c = year % 100;
  const f = Math.floor((b + 8) / 25);
  const g = Math.floor((b - f + 1) / 3);
  const h = (19 * a + b - Math.floor(b / 4) - g + 15) % 30;
  const l = (32 + 2 * (b % 4) + 2 * Math.floor(c / 4) - h - (c % 4)) % 7;
  const m = Math.floor((a + 11 * h + 22 * l) / 451);
  const month = Math.floor((h + l - 7 * m + 114) / 31);
  const day = ((h + l - 7 * m + 114) % 31) + 1;
  return new Date(Date.UTC(year, month - 1, day));
}

/** First and last year whose public holidays the school charts leave out. */
const HOLIDAY_YEARS = [2026, 2035];

/**
 * Statutory public holidays of North Rhine-Westphalia (YYYY-MM-DD), the state
 * of every moto school (the backend's DE-NW calendar). School holidays and
 * closing days are per school and unknown to PostHog.
 */
export function publicHolidays([from, to] = HOLIDAY_YEARS) {
  const dates = [];
  for (let year = from; year <= to; year++) {
    const easter = easterSunday(year);
    const fromEaster = (days) => new Date(easter.getTime() + days * 86_400_000);
    const fixed = (month, day) => new Date(Date.UTC(year, month - 1, day));
    dates.push(
      fixed(1, 1), fromEaster(-2), fromEaster(1), fixed(5, 1), fromEaster(39), fromEaster(50), fromEaster(60),
      fixed(10, 3), fixed(11, 1), fixed(12, 25), fixed(12, 26),
    );
  }
  return dates.map((date) => date.toISOString().slice(0, 10)).sort();
}

/**
 * HogQL condition: the event fell on a school day, Monday to Friday and no
 * public holiday (project time zone Europe/Berlin). The OGS is closed on the
 * other days, so every school chart leaves them out.
 */
export const ON_WORKDAY = `toDayOfWeek(toDate(timestamp)) <= 5 and toString(toDate(timestamp)) not in ${sqlList(publicHolidays())}`;

/**
 * HogQL condition on the deployment; every HogQL insight uses it. Production
 * counts school days only (ON_WORKDAY); the demo is open every day.
 */
/** Note on every dashboard with school data. */
const WORKDAYS_NOTE = 'Gezählt werden nur Werktage: Montag bis Freitag ohne gesetzliche Feiertage in NRW.';

const onDeployment = (deployment) => deployment === 'production'
  ? `properties.deployment = ${sqlString(DEPLOYMENTS[deployment])} and ${ON_WORKDAY}`
  : `properties.deployment = ${sqlString(DEPLOYMENTS[deployment])}`;

function deploymentOf(deployment) {
  if (!Object.hasOwn(DEPLOYMENTS, deployment)) throw new Error(`unknown deployment ${deployment}`);
  return deployment;
}

/**
 * A HogQL table (or number). `sql` receives the deployment condition and
 * must use it in every `from events`.
 */
function hogql(deployment, sql, display = 'ActionsTable', chartSettings) {
  const query = sql(onDeployment(deploymentOf(deployment))).trim();
  return {
    deployment,
    query: {
      kind: 'DataVisualizationNode',
      source: { kind: 'HogQLQuery', query },
      display,
      ...(chartSettings ? { chartSettings } : {}),
    },
  };
}

function deploymentFilter(deployment) {
  return {
    type: 'AND',
    values: [{
      type: 'AND',
      values: [{ key: 'deployment', type: 'event', operator: 'exact', value: [DEPLOYMENTS[deploymentOf(deployment)]] }],
    }, ...(deployment === 'production' ? [{ type: 'AND', values: [{ key: ON_WORKDAY, type: 'hogql' }] }] : [])],
  };
}

const series = (event, name, extra = {}) => ({
  kind: 'EventsNode', event, name: event ?? 'All events', custom_name: name, math: 'total', ...extra,
});

/**
 * HogQL expression: the name of the event's school. The backend publishes
 * every school as a group of type `school` (index 0) with the key
 * `<deployment>:<school_id>` and the property `name`; a school without one
 * reads „Schule 6“.
 */
export function schoolLabel(deployment) {
  const prefix = `${DEPLOYMENTS[deploymentOf(deployment)]}:`;
  const groups = `from groups where index = 0 and startsWith(key, ${sqlString(prefix)}) and notEmpty(toString(properties.name))`;
  return `transform(toString(properties.school_id), (select groupArray(replaceOne(key, ${sqlString(prefix)}, '')) ${groups}), (select groupArray(toString(properties.name)) ${groups}), concat('Schule ', toString(properties.school_id)))`;
}

/** Breakdown by school name (`schoolLabel`) for charts of real schools. */
export const SCHOOL_BREAKDOWN = schoolLabel('production');

function trends(deployment, { series: steps, interval = 'week', dateFrom = '-90d', breakdown, breakdownType = 'event', display = 'ActionsLineGraph', compare = false }) {
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
        ...(breakdown ? { breakdownFilter: { breakdown, breakdown_type: breakdownType } } : {}),
        ...(compare ? { compareFilter: { compare: true } } : {}),
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

// --- feature areas -----------------------------------------------------------

/**
 * Feature areas (Funktionsbereiche): which pages and which events belong to
 * one thing a school does with moto. A page ending in `/**` covers itself and
 * every page below it. The first matching area wins; the test makes sure
 * every route template of analytics-routes.ts lands in exactly one area or in
 * NO_FEATURE_PAGES, so a new page needs a decision here.
 */
export const FEATURES = Object.freeze([
  { name: 'Ein- und Auschecken', events: ['student_checked_in', 'student_checked_out'] },
  { name: 'Startseite', pages: { ogs: ['/home', '/dashboard'], parents: ['/'], school: ['/'] } },
  {
    name: 'Kinder und Kindakte',
    pages: { ogs: ['/students/**'], parents: ['/children/**'], school: ['/klasse'] },
    events: ['student_note_created', 'photo_consent_changed', 'day_hint_changed'],
  },
  { name: 'Gruppen', pages: { ogs: ['/ogs-groups', '/ogs-groups/overview'] }, events: ['group_created', 'group_updated'] },
  {
    name: 'Aufsichten und Räume',
    pages: { ogs: ['/active-supervisions', '/rooms/**', '/activities'], school: ['/aufsichten'] },
    events: ['supervision_started', 'supervision_completed', 'supervision_attendance_recorded'],
  },
  {
    name: 'Abwesenheiten',
    pages: { ogs: ['/absences'] },
    events: ['absence_reported', 'absence_request_submitted', 'arrival_exception_changed', 'class_trip_reported'],
  },
  {
    name: 'Elternanfragen',
    pages: { ogs: ['/anfragen', '/admin/change-requests', '/admin/guardian-approvals'] },
    events: ['pickup_change_requested', 'pickup_change_decided', 'offering_change_requested', 'offering_change_decided', 'master_data_change_submitted'],
  },
  {
    name: 'Nachrichten und Elternbriefe',
    pages: {
      ogs: ['/messages/**', '/team-chat/**', '/parent-announcements/**'],
      parents: ['/messages/**', '/news/**'],
      school: ['/nachrichten/**'],
    },
    events: [
      'parent_message_sent', 'staff_message_sent', 'parent_message_marked_unread', 'parent_messages_marked_all_read',
      'parent_message_count_scope_changed', 'parent_declaration_submitted', 'staff_notice_acknowledged',
    ],
  },
  {
    name: 'Tagesplan und Tagesinfos',
    pages: { ogs: ['/tagesplan', '/tagesinformationen', '/day-log', '/betreuungsplan', '/planung', '/timetables'], school: ['/tagesinformationen'] },
  },
  { name: 'Kalender und Speiseplan', pages: { ogs: ['/calendar', '/calendar-periods', '/meal-plan'], parents: ['/calendar', '/meal-plan'] } },
  {
    name: 'Personal und Dienstplan',
    pages: { ogs: ['/staff/**', '/dienstplan', '/mein-dienstplan', '/payroll', '/invitations'] },
    events: ['staff_invited', 'user_invited'],
  },
  { name: 'Zeiterfassung', pages: { ogs: ['/time-tracking'] } },
  { name: 'Vertretung', pages: { ogs: ['/substitutions', '/vertretung', '/vertretungsplan'] }, events: ['substitution_assigned', 'substitution_ended'] },
  {
    name: 'Anmeldung neuer Kinder',
    pages: {
      ogs: ['/admin/enrollments/**', '/enrollment-form', '/enrollment-phases/**', '/care-offerings', '/anmeldung/**'],
      parents: ['/anmeldung/**'],
    },
    events: ['enrollment_submitted'],
  },
  { name: 'Stammdaten und Export', pages: { ogs: ['/database/**', '/eltern/bankverbindungen'] }, events: ['data_exported'] },
  { name: 'Einstellungen', pages: { ogs: ['/settings', '/profile'], parents: ['/settings'], school: ['/einstellungen'] }, events: ['settings_changed'] },
  { name: 'Weitere Werkzeuge', pages: { ogs: ['/lists', '/reminders', '/emergency', '/info-displays', '/dateien', '/statistics'] } },
]);

/** Login, invitation, and demo entry pages: getting in, not using a feature. */
export const NO_FEATURE_PAGES = Object.freeze({
  ogs: ['/', '/demo', '/display', '/invite', '/reset-password'],
  parents: ['/login', '/invite', '/reset-password', '/demo', '/accept-guardian-invite/:token'],
  school: ['/login', '/invite', '/reset-password'],
});

/** Events that are no feature use: sessions, devices, and the demo funnel. */
const NO_FEATURE_EVENTS = ['login_success', 'login_failed', 'tenant_switched', 'guardian_invite_accepted', 'pwa_installed', 'pwa_install_prompt_shown', 'page_viewed'];

/** Area unmatched pages and events fall into; growing means FEATURES lacks an entry. */
const OTHER_FEATURE = 'Sonstiges';

/** Demo roles a visitor picks; transient pages before the pick carry none. */
const DEMO_ROLES = ['caregiver', 'lead', 'parent', 'all'];

const matchesPage = (pattern, template) => pattern.endsWith('/**')
  ? template === pattern.slice(0, -3) || template.startsWith(pattern.slice(0, -2))
  : template === pattern;

const matchesPages = (pages, surface, template) => (pages?.[surface] ?? []).some((pattern) => matchesPage(pattern, template));

/** The feature area of a page, `null` for NO_FEATURE_PAGES, OTHER_FEATURE if unmapped. */
export function featureOfPage(surface, template) {
  if (matchesPages(NO_FEATURE_PAGES, surface, template)) return null;
  return FEATURES.find((feature) => matchesPages(feature.pages, surface, template))?.name ?? OTHER_FEATURE;
}

function pagesCondition(pages) {
  const bySurface = Object.entries(pages).map(([surface, patterns]) => {
    const exact = patterns.map((pattern) => pattern.replace(/\/\*\*$/, ''));
    const prefixes = patterns.filter((pattern) => pattern.endsWith('/**')).map((pattern) => pattern.slice(0, -2));
    const path = [`properties.$pathname in ${sqlList(exact)}`, ...prefixes.map((prefix) => `startsWith(properties.$pathname, ${sqlString(prefix)})`)];
    return `(properties.surface = ${sqlString(surface)} and (${path.join(' or ')}))`;
  });
  return `event = '$pageview' and (${bySurface.join(' or ')})`;
}

/**
 * HogQL expression: the feature area of an event, '' for anything that is no
 * feature use (other `$` events, logins, the demo funnel's own events).
 */
export function featureExpression() {
  const branches = [[pagesCondition(NO_FEATURE_PAGES), "''"]];
  for (const feature of FEATURES) {
    const conditions = [];
    if (feature.pages) conditions.push(pagesCondition(feature.pages));
    if (feature.events) conditions.push(`event in ${sqlList(feature.events)}`);
    branches.push([conditions.map((condition) => `(${condition})`).join(' or '), sqlString(feature.name)]);
  }
  branches.push([
    `event = '$pageview' or not (startsWith(event, '$') or startsWith(event, 'demo_') or event in ${sqlList(NO_FEATURE_EVENTS)})`,
    sqlString(OTHER_FEATURE),
  ]);
  return `multiIf(${branches.map(([condition, value]) => `${condition}, ${value}`).join(', ')}, '')`;
}

const quoted = (name) => `\`${name}\``;

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

/** The dashboards of #3604 plus feature areas and demo behaviour. `projectId` feeds the heatmap links. */
export function dashboards({ projectId, routes = readRouteTemplates(), browserEvents = readBrowserEvents() }) {
  const coreAction = `not startsWith(event, '$') and event not in ${sqlList(browserEvents)}`;
  const feature = featureExpression();
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
      description: `Welche Rolle welche Oberfläche wie oft nutzt, in echten Schulen (deployment = moto-app.de). Kleinste Einheit ist die Rolle. ${WORKDAYS_NOTE}`,
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
      description: `Meistbesuchte und kaum genutzte Seiten als Routenmuster (frontend/src/lib/analytics-routes.ts), in echten Schulen (deployment = moto-app.de). ${WORKDAYS_NOTE}`,
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
      description: `Dead Clicks (Klick ohne Wirkung) und Rage Clicks (schnelle Mehrfachklicks) nach Seite, mit Link zur Heatmap der Seite. Schulen (moto-app.de) und Demo getrennt. ${WORKDAYS_NOTE}`,
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
      description: `Schulen mit mindestens einem Ereignis (Seitenaufruf, Kernaktion oder Kiosk) pro Woche, in echten Schulen (deployment = moto-app.de). ${WORKDAYS_NOTE}`,
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
    {
      name: 'Nutzungsanalyse: Funktionen in Schulen',
      description: `Welche Funktionsbereiche echte Schulen nutzen (deployment = moto-app.de): Seitenaufrufe und Aktionen, einschließlich Ein- und Auschecken am Kiosk. Die Zuordnung steht in FEATURES in scripts/posthog-dashboards.mjs. Das Elternportal kennt keine Schule und erscheint nur in der Tabelle nach Rolle. ${WORKDAYS_NOTE}`,
      insights: [
        {
          name: 'Funktionen: Reichweite in Schulen (30 Tage)',
          description: 'Je Funktionsbereich: wie viele Schulen ihn genutzt haben und welcher Anteil der aktiven Schulen das ist. Aktionen sind erfolgreiche Schreibvorgänge und Kiosk-Buchungen.',
          ...hogql('production', (where) => `
select
  Funktion,
  uniq(Schule) as Schulen,
  round(100 * uniq(Schule) / greatest(any(t.Aktiv), 1)) as ${quoted('Anteil aktiver Schulen in Prozent')},
  countIf(event = '$pageview') as Seitenaufrufe,
  countIf(event != '$pageview') as Aktionen,
  uniqIf(Sitzung, event = '$pageview') as Sitzungen
from (${schoolFeatureEvents(where, 30)}) as e
cross join (
  select uniq(toString(properties.school_id)) as Aktiv
  from events
  where ${where}
    and isNotNull(properties.school_id)
    and timestamp > now() - interval 30 day
) as t
where Funktion != ''
group by Funktion
order by Schulen desc, Seitenaufrufe + Aktionen desc`),
        },
        {
          name: 'Funktionen: je Schule (30 Tage)',
          description: 'Eine Zeile je Schule (Schul-ID), eine Spalte je Funktionsbereich: Seitenaufrufe plus Aktionen. Zeigt, welche Schule was nutzt und was sie auslässt.',
          ...hogql('production', (where) => `
select
  Schule as ${quoted('Schule (ID)')},
  count() as Gesamt,
  ${FEATURES.map((f) => `countIf(Funktion = ${sqlString(f.name)}) as ${quoted(f.name)}`).join(',\n  ')}
from (${schoolFeatureEvents(where, 30)})
where Funktion != ''
group by Schule
order by Gesamt desc`),
        },
        {
          name: 'Funktionen: nach Rolle und Oberfläche (30 Tage)',
          description: 'Wer welchen Funktionsbereich nutzt, einschließlich Elternportal. Ohne Oberfläche sind Ereignisse vom Kiosk und aus dem Backend.',
          ...hogql('production', (where) => `
select
  Funktion,
  ${quoted('Oberfläche')},
  Rolle,
  countIf(event = '$pageview') as Seitenaufrufe,
  countIf(event != '$pageview') as Aktionen,
  uniq(Schule) as Schulen
from (
  select
    ${feature} as Funktion,
    coalesce(properties.surface, 'ohne (Kiosk, Backend)') as ${quoted('Oberfläche')},
    coalesce(properties.role, '-') as Rolle,
    toString(properties.school_id) as Schule,
    event
  from events
  where ${where}
    and timestamp > now() - interval 30 day
)
where Funktion != ''
group by Funktion, ${quoted('Oberfläche')}, Rolle
order by Seitenaufrufe + Aktionen desc
limit 100`),
        },
        {
          name: 'Funktionen: Schulen pro Woche',
          description: 'Wie viele Schulen jeden Funktionsbereich in einer Woche genutzt haben, letzte 12 Wochen. Zeigt, ob eine Funktion sich ausbreitet oder einschläft.',
          ...hogql('production', (where) => `
select
  toStartOfWeek(timestamp, 1) as Woche,
  uniq(Schule) as ${quoted('Aktive Schulen')},
  ${FEATURES.map((f) => `uniqIf(Schule, Funktion = ${sqlString(f.name)}) as ${quoted(f.name)}`).join(',\n  ')}
from (
  select ${feature} as Funktion, toString(properties.school_id) as Schule, timestamp
  from events
  where ${where}
    and isNotNull(properties.school_id)
    and timestamp > now() - interval 12 week
)
where Funktion != ''
group by Woche
order by Woche desc`),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Nutzung je Standort',
      description: 'Je Schule (Schul-ID): an welchen Tagen sie moto nutzt und wofür, in echten Schulen (deployment = moto-app.de). Gezählt werden nur Werktage: Montag bis Freitag ohne gesetzliche Feiertage in NRW. Ferien und Schließtage kennt PostHog nicht, sie sind nicht abgezogen. Ein- und Auschecken wird erst seit dem 25.09.2026 erfasst. Das Elternportal kennt keine Schule und fehlt hier.',
      insights: [
        {
          name: 'Standorte: mit Anwesenheit in den letzten 7 Tagen',
          description: 'Schulen mit mindestens einer Ein- oder Auscheck-Buchung.',
          ...hogql('production', (where) => `
select uniqIf(Schule, Anwesenheit) as Standorte
from (${schoolDays(where, 7)})`, 'BoldNumber'),
        },
        {
          name: 'Standorte: mit Nutzung in den letzten 7 Tagen',
          description: 'Schulen mit mindestens einem Ereignis: Seitenaufruf, Kernaktion oder Kiosk.',
          ...hogql('production', (where) => `
select uniq(Schule) as Standorte
from (${schoolDays(where, 7)})`, 'BoldNumber'),
        },
        {
          name: 'Standorte: Ein- und Auscheck-Buchungen in den letzten 7 Tagen',
          description: 'Alle Schulen zusammen. Buchungen sind keine Kinder: ein Kind kann mehrmals am Tag buchen.',
          ...hogql('production', (where) => `
select countIf(Anwesenheit) as Buchungen
from (${schoolDays(where, 7)})`, 'BoldNumber'),
        },
        {
          name: 'Standorte: Anwesenheitsquote (60 Tage)',
          description: 'Anteil der Werktage (Montag bis Freitag ohne Feiertage) mit Ein- oder Auscheck-Buchung, gezählt ab der ersten Buchung bis gestern. Ferien und Schließtage sind nicht abgezogen, eine Quote unter 100 % kann also Ferien sein. Schulen ohne Buchung fehlen.',
          ...hogql('production', (where) => `
select Standort, round(Mit / greatest(Seit, 1), 2) as Anwesenheitsquote
from (
  select
    any(Standort) as Standort,
    uniqIf(Tag, Anwesenheit and Tag < today()) as Mit,
    ${workdaysSinceFirstAttendance} as Seit
  from (${schoolDays(where, 60)})
  group by Schule
  having countIf(Anwesenheit) > 0
)
order by Anwesenheitsquote desc, Standort`, 'ActionsBarValue', {
            xAxis: { column: 'Standort' },
            yAxis: [{ column: 'Anwesenheitsquote', settings: { formatting: { style: 'percent', decimalPlaces: 0 } } }],
          }),
        },
        {
          name: 'Standorte: Anwesenheit pro Tag',
          description: 'Ein- und Auscheck-Buchungen je Schule und Tag, letzte 60 Tage. Je dunkler, desto mehr Buchungen; Wochenenden und Feiertage bleiben immer leer, weitere leere Felder sind Tage ohne Anwesenheit (Ferien, Pause). Erfasst seit dem 25.09.2026.',
          ...trends('production', {
            series: [series(null, 'Ein- und Auscheck-Buchungen', {
              properties: [{ key: `event in ${sqlList(ATTENDANCE_EVENTS)}`, type: 'hogql' }],
            })],
            interval: 'day',
            dateFrom: '-60d',
            breakdown: SCHOOL_BREAKDOWN,
            breakdownType: 'hogql',
            display: 'TwoDimensionalHeatmap',
          }),
        },
        {
          name: 'Standorte: Nutzung pro Tag',
          description: 'Seitenaufrufe, Kernaktionen und Kiosk-Buchungen je Schule und Tag, letzte 60 Tage. Zeigt auch Schulen, die moto ohne Anwesenheitserfassung nutzen.',
          ...trends('production', {
            series: [series(null, 'Seitenaufrufe und Aktionen', {
              properties: [{ key: "isNotNull(properties.school_id) and (event = '$pageview' or not startsWith(event, '$'))", type: 'hogql' }],
            })],
            interval: 'day',
            dateFrom: '-60d',
            breakdown: SCHOOL_BREAKDOWN,
            breakdownType: 'hogql',
            display: 'TwoDimensionalHeatmap',
          }),
        },
        {
          name: 'Standorte: Nutzung im Vergleich zum eigenen Schnitt',
          description: 'Jede Schule an sich selbst gemessen: Seitenaufrufe und Aktionen einer Woche gegenüber ihrem eigenen Wochenschnitt der letzten 12 abgeschlossenen Wochen (100 %). Über 100 % heißt mehr Nutzung als sonst, unter 100 % weniger, etwa in den Ferien. Eine Woche ohne jede Nutzung fehlt als Punkt. In der Legende lassen sich Schulen ein- und ausblenden.',
          ...hogql('production', (where) => `
select Woche, Standort, round(Anzahl / greatest(avg(Anzahl) over (partition by Schule), 1), 2) as ${quoted('Anteil am eigenen Schnitt')}
from (
  select toStartOfWeek(Tag, 1) as Woche, Schule, any(Standort) as Standort, countIf(event = '$pageview' or not startsWith(event, '$')) as Anzahl
  from (${schoolDays(where, 91)})
  where Tag >= addDays(toStartOfWeek(today(), 1), -84) and Tag < toStartOfWeek(today(), 1)
  group by Woche, Schule
)
order by Woche, Standort`, 'ActionsLineGraph', {
            xAxis: { column: 'Woche' },
            yAxis: [{ column: 'Anteil am eigenen Schnitt', settings: { formatting: { style: 'percent', decimalPlaces: 0 } } }],
            seriesBreakdownColumn: 'Standort',
            goalLines: [{ label: 'eigener Schnitt', value: 1 }],
          }),
        },
        {
          name: 'Standorte: genutzte Abläufe (60 Tage)',
          description: `Je Schule ein Balken, je Ablauf ein Abschnitt: seine Länge ist die Zahl der Tage, an denen die Schule den Ablauf genutzt hat. Ein hoher Balken heißt also breite und regelmäßige Nutzung. „Weitere“ fasst die übrigen Funktionsbereiche zusammen (FEATURES in scripts/posthog-dashboards.mjs).`,
          ...hogql('production', (where) => `
select Standort, Ablauf, uniq(Tag) as Tage
from (
  select Standort, Tag, multiIf(Funktion in ${sqlList(KEY_FLOWS)}, Funktion, Funktion in ('', 'Startseite'), '', 'Weitere') as Ablauf
  from (${schoolDays(where, 60)})
)
where Ablauf != ''
group by Standort, Ablauf
order by Standort, Ablauf`, 'ActionsStackedBar', {
            xAxis: { column: 'Standort' },
            yAxis: [{ column: 'Tage' }],
            seriesBreakdownColumn: 'Ablauf',
          }),
        },
        {
          name: 'Standorte: Überblick (60 Tage)',
          description: 'Die Zahlen hinter den Grafiken. Anwesenheit heißt: mindestens eine Ein- oder Auscheck-Buchung an dem Tag. Der heutige Tag zählt nicht mit, er ist noch unvollständig. Buchungen sind keine Kinder: ein Kind kann mehrmals am Tag buchen.',
          ...hogql('production', (where) => `
select
  any(Standort) as Standort,
  Schule as ${quoted('Schul-ID')},
  uniqIf(Tag, Anwesenheit and Tag < today()) as ${quoted('Werktage mit Anwesenheit')},
  ${workdaysSinceFirstAttendance} as ${quoted('Werktage seit erster Anwesenheit')},
  if(countIf(Anwesenheit) = 0, null, minIf(Tag, Anwesenheit)) as ${quoted('Erste Anwesenheit')},
  if(countIf(Anwesenheit) = 0, null, maxIf(Tag, Anwesenheit)) as ${quoted('Letzte Anwesenheit')},
  uniqIf(Tag, Tag < today()) as ${quoted('Werktage mit Nutzung')},
  uniqIf(Tag, event = 'login_success') as ${quoted('Tage mit Login')},
  countIf(Anwesenheit) as ${quoted('Ein- und Auscheck-Buchungen')},
  countIf(${coreAction}) as Kernaktionen,
  countIf(event = '$pageview') as Seitenaufrufe
from (${schoolDays(where, 60)})
group by Schule
order by ${quoted('Werktage mit Anwesenheit')} desc, ${quoted('Werktage mit Nutzung')} desc`),
        },
        {
          name: 'Standorte: Tage je Funktionsbereich (60 Tage)',
          description: 'An wie vielen Tagen eine Schule einen Funktionsbereich genutzt hat, mit Seitenaufruf oder Aktion. Die Zuordnung steht in FEATURES in scripts/posthog-dashboards.mjs.',
          ...hogql('production', (where) => `
select
  any(Standort) as Standort,
  Schule as ${quoted('Schul-ID')},
  uniq(Tag) as ${quoted('Tage mit Nutzung')},
  uniqIf(Tag, event = 'login_success') as Logins,
  ${FEATURES.map((f) => `uniqIf(Tag, Funktion = ${sqlString(f.name)}) as ${quoted(f.name)}`).join(',\n  ')}
from (${schoolDays(where, 60)})
group by Schule
order by ${quoted('Tage mit Nutzung')} desc`),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Schule im Verlauf',
      description: `Eine Schule über die Zeit, verglichen mit ihr selbst statt mit anderen Schulen. Oben unter „Filter“ die Ereigniseigenschaft school_id auf eine Schul-ID setzen (die IDs stehen im Dashboard „Nutzung je Standort“, Tabelle Überblick). Ohne Filter zeigen die Grafiken alle Schulen zusammen. Die gestrichelte Linie ist der Zeitraum davor. Nur echte Schulen (deployment = moto-app.de). ${WORKDAYS_NOTE}`,
      insights: [
        {
          name: 'Verlauf: Tage mit Nutzung pro Woche',
          description: 'An wie vielen Tagen der Woche die Schule moto genutzt hat (Seitenaufruf, Kernaktion oder Kiosk). 5 heißt jeden Werktag.',
          ...trends('production', {
            series: [series(null, 'Tage mit Nutzung', {
              math: 'hogql',
              math_hogql: 'uniq(toDate(timestamp))',
              properties: [{ key: "isNotNull(properties.school_id) and (event = '$pageview' or not startsWith(event, '$'))", type: 'hogql' }],
            })],
            dateFrom: '-180d',
            display: 'ActionsBar',
          }),
        },
        {
          name: 'Verlauf: Ein- und Auscheck-Buchungen pro Tag',
          description: 'Buchungen pro Tag, letzte 60 Tage, gestrichelt die 60 Tage davor. Buchungen sind keine Kinder. Erfasst seit dem 25.09.2026.',
          ...trends('production', {
            series: [series(null, 'Ein- und Auscheck-Buchungen', {
              properties: [{ key: `event in ${sqlList(ATTENDANCE_EVENTS)}`, type: 'hogql' }],
            })],
            interval: 'day',
            dateFrom: '-60d',
            compare: true,
          }),
        },
        {
          name: 'Verlauf: Seitenaufrufe und Aktionen pro Woche',
          description: 'Wie viel die Schule mit moto arbeitet, gestrichelt die 90 Tage davor.',
          ...trends('production', {
            series: [series(null, 'Seitenaufrufe und Aktionen', {
              properties: [{ key: "isNotNull(properties.school_id) and (event = '$pageview' or not startsWith(event, '$'))", type: 'hogql' }],
            })],
            compare: true,
          }),
        },
        {
          name: 'Verlauf: Abläufe pro Woche',
          description: 'Seitenaufrufe und Aktionen je Ablauf. Zeigt, welche Abläufe die Schule neu aufnimmt oder liegen lässt.',
          ...trends('production', {
            series: KEY_FLOWS.map((flow) => series(null, flow, {
              properties: [{ key: `isNotNull(properties.school_id) and ${featureExpression()} = ${sqlString(flow)}`, type: 'hogql' }],
            })),
            display: 'ActionsBar',
          }),
        },
        {
          name: 'Verlauf: Logins pro Woche',
          description: 'Erfolgreiche Anmeldungen im OGS- und Schulportal, gestrichelt die 90 Tage davor.',
          ...trends('production', {
            series: [series('login_success', 'Logins', {
              properties: [{ key: 'isNotNull(properties.school_id)', type: 'hogql' }],
            })],
            compare: true,
          }),
        },
      ],
    },
    {
      name: 'Nutzungsanalyse: Demo-Verhalten',
      description: 'Was Besucher in der öffentlichen Demo ansehen und ausprobieren und wo sie aufhören. Nur deployment = demo, nur Seiten nach der Rollenwahl. Eine Sitzung endet auch beim Neuladen oder in einem neuen Tab (kein Cookie), „aufgehört“ heißt deshalb: letzte Seite einer Sitzung.',
      insights: [
        {
          name: 'Demo-Verhalten: letzte Seite vor dem Aufhören',
          description: 'Letzte 30 Tage. Auf welcher Seite Demo-Sitzungen enden, wie viele Seiten und Sekunden die Sitzung bis dahin hatte.',
          ...hogql('demo', (where) => `
select
  Funktion,
  Seite,
  count() as ${quoted('Sitzungen endeten hier')},
  round(100 * count() / sum(count()) over (), 1) as ${quoted('Anteil in Prozent')},
  round(median(Seiten)) as ${quoted('Median Seiten bis dahin')},
  round(median(Dauer)) as ${quoted('Median Dauer (s)')}
from (
  select
    Sitzung,
    argMax(Funktion, timestamp) as Funktion,
    argMax(Seite, timestamp) as Seite,
    count() as Seiten,
    dateDiff('second', min(timestamp), max(timestamp)) as Dauer
  from (${demoPages(where)})
  group by Sitzung
)
group by Funktion, Seite
order by ${quoted('Sitzungen endeten hier')} desc
limit 30`),
        },
        {
          name: 'Demo-Verhalten: Sitzungstiefe',
          description: 'Letzte 30 Tage. Wie weit Besucher kommen: Sitzungen nach Zahl der Seiten, mit Dauer und Zahl der angesehenen Funktionsbereiche.',
          ...hogql('demo', (where) => `
select
  multiIf(Seiten = 1, '1 Seite', Seiten <= 3, '2-3 Seiten', Seiten <= 6, '4-6 Seiten', Seiten <= 10, '7-10 Seiten', Seiten <= 20, '11-20 Seiten', 'über 20 Seiten') as ${quoted('Tiefe')},
  count() as Sitzungen,
  round(100 * count() / sum(count()) over (), 1) as ${quoted('Anteil in Prozent')},
  round(median(Dauer)) as ${quoted('Median Dauer (s)')},
  round(avg(Funktionen), 1) as ${quoted('Funktionsbereiche (Mittel)')}
from (
  select
    Sitzung,
    count() as Seiten,
    dateDiff('second', min(timestamp), max(timestamp)) as Dauer,
    uniqIf(Funktion, Funktion != '') as Funktionen
  from (${demoPages(where)})
  group by Sitzung
)
group by ${quoted('Tiefe')}
order by min(Seiten)`),
        },
        {
          name: 'Demo-Verhalten: angesehene Funktionen nach Rolle',
          description: 'Letzte 30 Tage. Welcher Anteil der Sitzungen einer Demo-Rolle einen Funktionsbereich geöffnet hat.',
          ...hogql('demo', (where) => `
select
  f.Rolle as Rolle,
  f.Funktion as Funktion,
  f.Sitzungen as Sitzungen,
  round(100 * f.Sitzungen / greatest(r.Gesamt, 1)) as ${quoted('Anteil der Sitzungen in Prozent')},
  f.Aufrufe as Seitenaufrufe
from (
  select Rolle, Funktion, uniq(Sitzung) as Sitzungen, count() as Aufrufe
  from (${demoPages(where)})
  where Funktion != ''
  group by Rolle, Funktion
) as f
join (
  select Rolle, uniq(Sitzung) as Gesamt
  from (${demoPages(where)})
  group by Rolle
) as r on f.Rolle = r.Rolle
order by Rolle, Sitzungen desc`),
        },
        {
          name: 'Demo-Verhalten: ausprobierte Aktionen',
          description: 'Letzte 30 Tage. Was Besucher selbst gespeichert oder abgeschickt haben. Nur Aktionen mit Browser-Sitzung: die Daten, die beim Bereitstellen der Demo-Schule entstehen, zählen nicht.',
          ...hogql('demo', (where) => `
select
  Funktion,
  event as Aktion,
  count() as Anzahl,
  uniq(Sitzung) as Sitzungen
from (
  select ${feature} as Funktion, event, properties.$session_id as Sitzung
  from events
  where ${where}
    and event != '$pageview'
    and isNotNull(properties.$session_id)
    and timestamp > now() - interval 30 day
)
where Funktion != ''
group by Funktion, Aktion
order by Anzahl desc`),
        },
        {
          name: 'Demo-Verhalten: Funktionen vor „Kostenlos starten“',
          description: 'Letzte 30 Tage. Welcher Anteil der Sitzungen mit und ohne Klick auf „Kostenlos starten“ einen Funktionsbereich gesehen hat. Ein großer Abstand deutet auf eine Funktion, die überzeugt. Wenige Klicks bedeuten große Zufallsschwankung.',
          ...hogql('demo', (where) => `
select
  Funktion,
  countIf(Gestartet = 1) as ${quoted('Sitzungen mit Klick')},
  round(100 * countIf(Gestartet = 1) / greatest(max(MitKlick), 1)) as ${quoted('Anteil mit Klick in Prozent')},
  round(100 * countIf(Gestartet = 0) / greatest(max(OhneKlick), 1)) as ${quoted('Anteil ohne Klick in Prozent')}
from (
  select Sitzung, Gestartet, arrayJoin(Funktionen) as Funktion, MitKlick, OhneKlick
  from (
    select
      Sitzung,
      Gestartet,
      Funktionen,
      sum(Gestartet) over () as MitKlick,
      count() over () - sum(Gestartet) over () as OhneKlick
    from (
      select
        properties.$session_id as Sitzung,
        max(event = 'demo_start_clicked') as Gestartet,
        groupUniqArrayIf(${feature}, event = '$pageview' and coalesce(properties.demo_role, properties.role) in ${sqlList(DEMO_ROLES)}) as Funktionen
      from events
      where ${where}
        and isNotNull(properties.$session_id)
        and timestamp > now() - interval 30 day
      group by Sitzung
    )
    where length(Funktionen) > 0
  )
)
where Funktion != ''
group by Funktion
order by ${quoted('Anteil mit Klick in Prozent')} desc, ${quoted('Anteil ohne Klick in Prozent')} desc`),
        },
      ],
    },
  ];
}

/** Production events of schools with their feature area (school_id set: no parents portal). */
function schoolFeatureEvents(where, days) {
  return `
  select ${featureExpression()} as Funktion, toString(properties.school_id) as Schule, event, properties.$session_id as Sitzung
  from events
  where ${where}
    and isNotNull(properties.school_id)
    and timestamp > now() - interval ${days} day`;
}

const ATTENDANCE_EVENTS = FEATURES.find((f) => f.name === 'Ein- und Auschecken').events;

/** Feature areas the per-school chart shows on their own (at most eight segments with „Weitere“). */
const KEY_FLOWS = [
  'Ein- und Auschecken', 'Aufsichten und Räume', 'Abwesenheiten', 'Elternanfragen',
  'Nachrichten und Elternbriefe', 'Zeiterfassung', 'Kinder und Kindakte',
];

/** School days (ON_WORKDAY) from a school's first attendance in `schoolDays` until yesterday. */
const workdaysSinceFirstAttendance = `if(countIf(Anwesenheit) = 0, 0, length(arrayFilter(d -> toDayOfWeek(d) <= 5 and toString(d) not in ${sqlList(publicHolidays())}, arrayMap(i -> addDays(minIf(Tag, Anwesenheit), i), range(toInt(greatest(dateDiff('day', minIf(Tag, Anwesenheit), today()), 0)))))))`;

/** Production events of schools with their day (project time zone) and feature area. */
function schoolDays(where, days) {
  return `
  select
    toString(properties.school_id) as Schule,
    ${SCHOOL_BREAKDOWN} as Standort,
    toDate(timestamp) as Tag,
    event in ${sqlList(ATTENDANCE_EVENTS)} as Anwesenheit,
    ${featureExpression()} as Funktion,
    event
  from events
  where ${where}
    and isNotNull(properties.school_id)
    and timestamp > now() - interval ${days} day`;
}

/** Demo page views after the role pick, with their feature area. */
function demoPages(where) {
  return `
  select
    properties.$session_id as Sitzung,
    ${featureExpression()} as Funktion,
    properties.$pathname as Seite,
    coalesce(properties.demo_role, properties.role) as Rolle,
    timestamp
  from events
  where ${where}
    and event = '$pageview'
    and coalesce(properties.demo_role, properties.role) in ${sqlList(DEMO_ROLES)}
    and isNotNull(properties.$session_id)
    and timestamp > now() - interval 30 day`;
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
