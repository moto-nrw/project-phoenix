import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  DEPLOYMENTS,
  FEATURES,
  ON_WORKDAY,
  POSTHOG_HOST,
  SCHOOL_BREAKDOWN,
  dashboards,
  featureExpression,
  featureOfPage,
  missingPath,
  posthogClient,
  publicHolidays,
  readBrowserEvents,
  readPageNames,
  readRouteTemplates,
  sync,
} from './posthog-dashboards.mjs';

const definitions = dashboards({ projectId: '1' });
const insights = definitions.flatMap((dashboard) => dashboard.insights);

test('defines one dashboard per product surface plus the two across schools', () => {
  assert.deepEqual(definitions.map((dashboard) => dashboard.name), [
    'Nutzungsanalyse: Nutzung je Standort',
    'Nutzungsanalyse: OGS-App',
    'Nutzungsanalyse: Eltern-App',
    'Nutzungsanalyse: Schul-App',
    'Nutzungsanalyse: Schule im Verlauf',
    'Nutzungsanalyse: Demo',
  ]);
  const names = insights.map((insight) => insight.name);
  assert.equal(new Set(names).size, names.length, 'insight names are the sync key and must be unique');
});

test('every insight filters on production or the demo, never staging', () => {
  for (const insight of insights) {
    const deployment = DEPLOYMENTS[insight.deployment];
    assert.ok(deployment, `${insight.name} names no known deployment`);
    const source = insight.query.source;
    if (source.kind === 'HogQLQuery') {
      const tables = source.query.match(/\bfrom events\b/g) ?? [];
      const filters = source.query.match(new RegExp(`properties\\.deployment = '${deployment}'`, 'g')) ?? [];
      assert.ok(tables.length > 0, insight.name);
      assert.equal(filters.length, tables.length, `${insight.name}: every events scan filters the deployment`);
      const heatmaps = source.query.match(/\bfrom heatmaps\b/g) ?? [];
      const hosts = source.query.match(new RegExp(`startsWith\\(current_url, 'https://${deployment}/'\\)`, 'g')) ?? [];
      assert.equal(hosts.length, heatmaps.length, `${insight.name}: every heatmaps scan filters the deployment host`);
    } else {
      assert.deepEqual(source.properties.values[0].values,
        [{ key: 'deployment', type: 'event', operator: 'exact', value: [deployment] }], insight.name);
    }
    assert.doesNotMatch(JSON.stringify(insight.query), /staging|localhost/i, insight.name);
  }
});

test('school insights count Monday to Friday without the public holidays of NRW, the demo every day', () => {
  for (const insight of insights) {
    const source = insight.query.source;
    const filters = source.kind === 'HogQLQuery'
      ? source.query.split(ON_WORKDAY).length - 1
      : source.properties.values.filter((group) => group.values.some((value) => value.key === ON_WORKDAY)).length;
    const scans = source.kind === 'HogQLQuery' ? source.query.match(/\bfrom (events|heatmaps)\b/g).length : 1;
    assert.equal(filters, insight.deployment === 'production' ? scans : 0, insight.name);
  }
  const holidays = publicHolidays([2026, 2027]);
  assert.equal(holidays.length, 22);
  for (const day of ['2026-04-03', '2026-04-06', '2026-05-14', '2026-05-25', '2026-06-04', '2026-10-03', '2026-11-01', '2026-12-25', '2027-03-29']) {
    assert.ok(holidays.includes(day), day);
  }
  assert.ok(publicHolidays().includes('2035-12-26'));
});

test('the demo dashboards stay in the demo and the school dashboards in production', () => {
  const isDemo = (dashboard) => dashboard.name.startsWith('Nutzungsanalyse: Demo');
  const demos = definitions.filter(isDemo);
  assert.deepEqual(demos.map((d) => d.name), ['Nutzungsanalyse: Demo']);
  for (const demo of demos) {
    assert.ok(demo.insights.every((insight) => insight.deployment === 'demo'), demo.name);
  }
  const schools = definitions.filter((d) => !isDemo(d));
  for (const dashboard of schools) {
    assert.ok(dashboard.insights.every((insight) => insight.deployment === 'production'), dashboard.name);
  }
});

test('no insight looks at a single person', () => {
  for (const insight of insights) {
    const query = JSON.stringify(insight.query);
    assert.doesNotMatch(query, /distinct_id|\bpersons?\b|person_id|\$user_id|\bpdi\b|email|\bdau\b|weekly_active|monthly_active|unique_users/i, insight.name);
    assert.notEqual(insight.query.source.kind, 'PathsQuery', `${insight.name}: Paths count people`);
    const breakdown = insight.query.source.breakdownFilter?.breakdown;
    if (breakdown) assert.ok(['surface', 'role', SCHOOL_BREAKDOWN].includes(breakdown), insight.name);
  }
});

test('charts name a school from its production group and fall back to its ID', () => {
  assert.match(SCHOOL_BREAKDOWN, /from groups where index = 0 and startsWith\(key, 'moto-app\.de:'\)/);
  assert.match(SCHOOL_BREAKDOWN, /concat\('Schule ', toString\(properties\.school_id\)\)\)$/);
  assert.doesNotMatch(SCHOOL_BREAKDOWN, /demo|staging|localhost/);
});

test('reads the route templates of every portal from analytics-routes.ts', () => {
  const routes = readRouteTemplates();
  for (const route of [['ogs', '/dashboard'], ['ogs', '/demo'], ['parents', '/children/:id'], ['school', '/klasse']]) {
    assert.ok(routes.some(([surface, template]) => surface === route[0] && template === route[1]), route.join(' '));
  }
  assert.throws(() => readRouteTemplates('export const OTHER = [] as const;'), /TRACKED_TENANT_ROUTE_TEMPLATES/);
});

test('reads the browser events from analytics-policy.ts', () => {
  const events = readBrowserEvents();
  assert.ok(events.includes('demo_start_clicked'));
  assert.ok(!events.includes('login_success'), 'a core action comes from the backend');
  assert.throws(() => readBrowserEvents('const OTHER = 1;'), /CUSTOM_EVENTS/);
  const usage = insights.find((insight) => insight.name.startsWith('OGS-App: Was tun'));
  assert.match(usage.query.source.query, /event not in \('login_failed', /);
});

test('every route template belongs to exactly one feature area or is an entry page', () => {
  for (const [surface, template] of readRouteTemplates()) {
    const feature = featureOfPage(surface, template);
    assert.notEqual(feature, 'Sonstiges', `${surface} ${template}: add it to FEATURES or NO_FEATURE_PAGES`);
    const owners = FEATURES.filter((f) => (f.pages?.[surface] ?? []).some((pattern) => pattern.endsWith('/**')
      ? template === pattern.slice(0, -3) || template.startsWith(pattern.slice(0, -2))
      : template === pattern));
    assert.ok(owners.length <= 1, `${surface} ${template} is in ${owners.map((f) => f.name).join(' and ')}`);
  }
  assert.equal(featureOfPage('ogs', '/students/:id/room-history'), 'Kinder und Kindakte');
  assert.equal(featureOfPage('school', '/klasse/kind'), 'Kinder und Kindakte');
  assert.equal(featureOfPage('ogs', '/students-archive'), 'Sonstiges', 'a prefix covers its sub pages only');
  assert.equal(featureOfPage('parents', '/login'), null);
  assert.equal(featureOfPage('parents', '/documents'), 'Weitere Werkzeuge');
});

test('feature areas have unique names, each event belongs to one area, and the kiosk counts', () => {
  const names = FEATURES.map((f) => f.name);
  assert.equal(new Set(names).size, names.length);
  const events = FEATURES.flatMap((f) => f.events ?? []);
  assert.equal(new Set(events).size, events.length);
  assert.ok(events.includes('student_checked_in') && events.includes('student_checked_out'));
  const expression = featureExpression();
  for (const name of names) assert.ok(expression.includes(`'${name}'`), name);
  assert.match(expression, /startsWith\(event, 'demo_'\)/, 'the demo funnel events are no feature use');
});

test('the rarely used page lists name every route template of their portal', () => {
  const ogs = insights.find((insight) => insight.name.startsWith('OGS-App: Welche Seiten öffnet'));
  const parents = insights.find((insight) => insight.name.startsWith('Eltern-App: Welche Seiten öffnet'));
  assert.match(ogs.query.source.query, /'\/students\/:id'/);
  assert.doesNotMatch(ogs.query.source.query, /'\/children\/:id'/);
  assert.match(parents.query.source.query, /'\/children\/:id'/);
});

test('page tables show the navigation name of a page', () => {
  const names = readPageNames();
  assert.equal(names.get('/students/search'), 'Alle Kinder');
  assert.equal(names.get('/home'), 'Startseite');
  assert.throws(() => readPageNames('const OTHER = 1;'), /href\/label/);
  const pages = insights.find((insight) => insight.name === 'OGS-App: Welche Seiten werden am meisten genutzt?');
  assert.match(pages.query.source.query, /'Alle Kinder'/);
  assert.match(pages.query.source.query, /\$prev_pageview_duration/);
});

test('a school counts as active through use, not through a failed login or a dead click', () => {
  const weekly = insights.find((insight) => insight.name === 'Aktive Schulen pro Woche');
  assert.match(JSON.stringify(weekly.query), /event != 'login_failed'/);
  const lastWeek = insights.find((insight) => insight.name === 'Standorte: mit Nutzung in den letzten 7 Tagen');
  assert.match(lastWeek.query.source.query, /uniqIf\(Schule, Nutzung\)/);
  assert.match(lastWeek.query.source.query, /event != 'login_failed'\) as Nutzung|event != 'login_failed' as Nutzung/);
  const oldCondition = "(event = '$pageview' or not startsWith(event, '$'))";
  for (const insight of insights.filter((i) => i.deployment === 'production')) {
    const query = JSON.stringify(insight.query);
    if (!query.includes(oldCondition)) continue;
    assert.match(query, /event != 'login_failed'/, `${insight.name}: use ACTIVE_USE`);
  }
});

test('every portal of a school has its own page, action, and friction tables', () => {
  for (const [dashboard, surface] of [['OGS-App', 'ogs'], ['Eltern-App', 'parents'], ['Schul-App', 'school']]) {
    const tiles = definitions.find((d) => d.name === `Nutzungsanalyse: ${dashboard}`).insights;
    for (const question of ['Welche Seiten werden am meisten genutzt?', 'Welche Seiten öffnet kaum jemand?']) {
      const tile = tiles.find((insight) => insight.name === `${dashboard}: ${question}`);
      assert.match(tile.query.source.query, new RegExp(`properties\\.surface = '${surface}'`), tile.name);
    }
    assert.ok(tiles.some((insight) => insight.name.startsWith(`${dashboard}: Wo klicken`)), dashboard);
  }
});

test('next pages count page changes from the start page, not people', () => {
  const ogs = insights.find((insight) => insight.name.startsWith('OGS-App: Wohin'));
  assert.match(ogs.query.source.query, /properties\.\$prev_pageview_pathname = '\/home'/);
  assert.match(ogs.query.source.query, /properties\.surface = 'ogs'/);
  assert.doesNotMatch(ogs.query.source.query, /session_id/);
});

test('friction tables link the heatmap and read rage clicks from the heatmap data', () => {
  const ogs = dashboards({ projectId: '4711' }).find((d) => d.name === 'Nutzungsanalyse: OGS-App');
  const table = ogs.insights.find((insight) => insight.name.startsWith('OGS-App: Wo klicken')).query.source.query;
  assert.match(table, /'https:\/\/eu\.posthog\.com\/project\/4711\/heatmaps\/new\?pageURL='/);
  assert.match(table, /'https:\/\/moto-app\.de'/);
  assert.match(table, /from heatmaps/);
  assert.match(table, /type = 'rageclick'/);
  assert.match(table, /properties\.surface in \('ogs'\)/);
});

test('the client only calls the EU API and needs a key', async () => {
  assert.throws(() => posthogClient({ apiKey: '' }), /POSTHOG_PERSONAL_API_KEY/);
  const calls = [];
  const api = posthogClient({
    apiKey: 'test-key',
    request: async (url, init) => {
      calls.push({ url, init });
      return new Response('{"ok":true}', { status: 200 });
    },
  });
  await api('GET', '/api/projects/');
  assert.equal(calls[0].url, `${POSTHOG_HOST}/api/projects/`);
  assert.equal(POSTHOG_HOST, 'https://eu.posthog.com');
  assert.equal(calls[0].init.headers.Authorization, 'Bearer test-key');
});

/** In-memory PostHog: dashboards and insights, with the list endpoints. */
function fakePostHog() {
  const store = { dashboards: [], insights: [] };
  const writes = [];
  let nextId = 1;
  const api = async (method, path, body) => {
    const url = new URL(path, POSTHOG_HOST);
    const [, , , , kind, id] = url.pathname.split('/');
    const items = store[kind];
    if (method === 'GET') {
      const search = url.searchParams.get('search');
      return { next: null, results: structuredClone(search ? items.filter((item) => item.name.includes(search)) : items) };
    }
    writes.push(`${method} ${kind}`);
    if (method === 'POST') {
      const item = { id: nextId++, deleted: false, ...structuredClone(body) };
      items.push(item);
      return item;
    }
    const item = items.find((candidate) => String(candidate.id) === id);
    Object.assign(item, structuredClone(body));
    return item;
  };
  return { api, store, writes };
}

test('sync creates everything once and is idempotent', async () => {
  const posthog = fakePostHog();
  const first = await sync(posthog.api, '1', definitions);
  assert.equal(posthog.store.dashboards.length, definitions.length);
  assert.equal(posthog.store.insights.length, insights.length);
  assert.equal(first.length, definitions.length + insights.length);
  for (const dashboard of posthog.store.dashboards) {
    const expected = definitions.find((d) => d.name === dashboard.name).insights.length;
    assert.equal(posthog.store.insights.filter((i) => i.dashboards.includes(dashboard.id)).length, expected);
  }

  posthog.writes.length = 0;
  assert.deepEqual(await sync(posthog.api, '1', definitions), []);
  assert.deepEqual(posthog.writes, []);
});

test('sync updates a changed insight in place and puts it back on its dashboard', async () => {
  const posthog = fakePostHog();
  await sync(posthog.api, '1', definitions);
  const insight = posthog.store.insights[0];
  insight.query = { kind: 'HogQLQuery', query: 'select 1' };
  insight.dashboards = [];

  const log = await sync(posthog.api, '1', definitions);
  assert.deepEqual(log, [`update insight ${insight.name} (query.kind)`]);
  assert.equal(posthog.store.insights.length, insights.length);
  assert.deepEqual(insight.query, insights[0].query);
  assert.deepEqual(insight.dashboards, [posthog.store.dashboards[0].id]);
});

test('defaults PostHog adds to a stored query do not count as a change', () => {
  const expected = { kind: 'TrendsQuery', series: [{ event: 'x', math: 'total' }] };
  const stored = { kind: 'TrendsQuery', series: [{ event: 'x', math: 'total', name: 'x' }], version: 2 };
  assert.equal(missingPath(expected, stored), null);
  assert.equal(missingPath(expected, { ...stored, series: [] }), 'query.series');
  assert.equal(missingPath(expected, { ...stored, series: [{ event: 'y', math: 'total' }] }), 'query.series.0.event');
});

test('a dry run writes nothing', async () => {
  const posthog = fakePostHog();
  const log = await sync(posthog.api, '1', definitions, { dryRun: true });
  assert.equal(log.length, definitions.length + insights.length);
  assert.deepEqual(posthog.writes, []);
});
