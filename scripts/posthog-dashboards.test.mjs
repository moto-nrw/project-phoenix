import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  DEPLOYMENTS,
  POSTHOG_HOST,
  dashboards,
  missingPath,
  posthogClient,
  readBrowserEvents,
  readRouteTemplates,
  sync,
} from './posthog-dashboards.mjs';

const definitions = dashboards({ projectId: '1' });
const insights = definitions.flatMap((dashboard) => dashboard.insights);

test('defines the five dashboards of #3604', () => {
  assert.deepEqual(definitions.map((dashboard) => dashboard.name), [
    'Nutzungsanalyse: Demo-Funnel',
    'Nutzungsanalyse: Nutzung pro Rolle und Oberfläche',
    'Nutzungsanalyse: Seiten',
    'Nutzungsanalyse: Reibung',
    'Nutzungsanalyse: Aktive Schulen',
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
    } else {
      assert.deepEqual(source.properties.values[0].values,
        [{ key: 'deployment', type: 'event', operator: 'exact', value: [deployment] }], insight.name);
    }
    assert.doesNotMatch(JSON.stringify(insight.query), /staging|localhost/i, insight.name);
  }
});

test('the demo funnel stays in the demo and the school dashboards in production', () => {
  const [demo, ...schools] = definitions;
  assert.ok(demo.insights.every((insight) => insight.deployment === 'demo'));
  for (const dashboard of schools.filter((d) => d.name !== 'Nutzungsanalyse: Reibung')) {
    assert.ok(dashboard.insights.every((insight) => insight.deployment === 'production'), dashboard.name);
  }
});

test('no insight looks at a single person', () => {
  for (const insight of insights) {
    const query = JSON.stringify(insight.query);
    assert.doesNotMatch(query, /distinct_id|\bpersons?\b|person_id|\$user_id|\bpdi\b|email|\bdau\b|weekly_active|monthly_active|unique_users/i, insight.name);
    const breakdown = insight.query.source.breakdownFilter?.breakdown;
    if (breakdown) assert.ok(['surface', 'role'].includes(breakdown), insight.name);
  }
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
  const usage = insights.find((insight) => insight.name.startsWith('Nutzung: Kernaktionen'));
  assert.match(usage.query.source.query, /event not in \('login_failed', /);
});

test('the page list names every route template', () => {
  const pages = insights.find((insight) => insight.name.startsWith('Seiten: kaum'));
  assert.match(pages.query.source.query, /\('parents', '\/children\/:id'\)/);
});

test('heatmap links point at the EU app and the route template', () => {
  const friction = dashboards({ projectId: '4711' }).find((d) => d.name === 'Nutzungsanalyse: Reibung');
  const table = friction.insights[0].query.source.query;
  assert.match(table, /'https:\/\/eu\.posthog\.com\/project\/4711\/heatmaps\/new\?pageURL='/);
  assert.match(table, /'https:\/\/moto-app\.de'/);
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
  assert.equal(posthog.store.dashboards.length, 5);
  assert.equal(posthog.store.insights.length, insights.length);
  assert.equal(first.length, 5 + insights.length);
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
  assert.equal(log.length, 5 + insights.length);
  assert.deepEqual(posthog.writes, []);
});
