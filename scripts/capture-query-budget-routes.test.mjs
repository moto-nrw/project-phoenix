import { test } from 'node:test';
import assert from 'node:assert/strict';
import { backfillBudget, capturedRoutes } from './capture-query-budget-routes.mjs';

const budget = { scenario: 'api.students.list', max: 3, exact: true, description: 'three children' };
const observe = route => new Map([[budget.scenario, route]]);

test('table-driven description needs no route; only route is added', () => {
  const after = backfillBudget(budget, observe('GET /api/students/'));
  assert.deepEqual(after, { ...budget, route: 'GET /api/students/' });
});
test('capture preserves resolved mount prefix and parameter pattern', () => {
  assert.equal(backfillBudget(budget, observe('GET /api/students/{id}')).route, 'GET /api/students/{id}');
});
test('a renamed route never silently overwrites an authored binding', () => {
  assert.throws(() => backfillBudget({ ...budget, route: 'GET /api/students/' }, observe('GET /api/pupils/')), /Refusing to overwrite/);
});
test('missing, conflicting and malformed observations fail closed', () => {
  assert.throws(() => backfillBudget(budget, new Map()), /No observation/);
  assert.throws(() => capturedRoutes('{"scenario":"x","route":"GET /a"}\n{"scenario":"x","route":"GET /b"}'), /Conflicting/);
  assert.throws(() => backfillBudget(budget, observe('multiple HTTP routes')), /Invalid chi route/);
});
test('only explicitly marked non-HTTP budgets may lack a route', () => {
  assert.throws(() => backfillBudget(budget, observe('')), /Non-HTTP reason/);
  const exceptions = { [budget.scenario]: 'Directory use case called directly, no request.' };
  assert.deepEqual(backfillBudget(budget, observe(''), exceptions), budget);
  assert.throws(() => backfillBudget(budget, observe('GET /api/students/'), exceptions), /HTTP measurement cannot claim/);
});

test('multiple requests retain their sorted deduplicated route set', () => {
  const route = ['PUT /api/students/{id}', 'GET /api/students/', 'GET /api/students/{id}', 'GET /api/students/'];
  const after = backfillBudget(budget, observe(route));
  assert.deepEqual(after.route, ['GET /api/students/', 'GET /api/students/{id}', 'PUT /api/students/{id}']);
  assert.throws(() => backfillBudget(after, observe(['GET /api/students/', 'GET /api/pupils/{id}', 'PUT /api/students/{id}'])), /Refusing to overwrite/);
});
