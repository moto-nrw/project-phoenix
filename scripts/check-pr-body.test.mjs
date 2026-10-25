import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import test from 'node:test';
import { checkPrBody, heading } from './check-pr-body.mjs';

const template = readFileSync(new URL('../.github/PULL_REQUEST_TEMPLATE.md', import.meta.url), 'utf8');

test('accepts a filled TL;DR at the top, also below the template title', () => {
  assert.deepEqual(checkPrBody(`${heading}\nEltern sehen jetzt, wer die Anfrage gestellt hat.\n\n## Description\nx`), []);
  assert.deepEqual(checkPrBody(template.replace(`${heading}\n`, `${heading}\nKeine Änderung für Nutzer, nur CI.\n`)), []);
});

test('rejects a body without the TL;DR at the top', () => {
  assert.equal(checkPrBody('## Description\nx').length, 1);
  assert.equal(checkPrBody(`Teil von #1.\n\n${heading}\nEltern sehen mehr.`).length, 1);
  assert.equal(checkPrBody('').length, 1);
  assert.equal(checkPrBody(undefined).length, 1);
});

test('rejects the unfilled template and an empty section', () => {
  assert.match(checkPrBody(template)[0], /is empty/);
  assert.match(checkPrBody(`${heading}\n\n## Description\nx`)[0], /is empty/);
});
