#!/usr/bin/env node
// Checks that a pull request body opens with the user TL;DR from
// .github/PULL_REQUEST_TEMPLATE.md and that the section is filled in.
// Reads the body from PR_BODY; pr-body.yml passes it through the environment.
import { fileURLToPath } from 'node:url';

export const heading = '## TL;DR: Was ändert sich für Nutzer?';

export function checkPrBody(body) {
  const lines = (body ?? '').replace(/<!--[\s\S]*?(?:-->|$)/g, '').split(/\r?\n/);
  let start = lines.findIndex(line => line.trim());
  if (start >= 0 && /^#\s/.test(lines[start])) {
    start = lines.findIndex((line, i) => i > start && line.trim());
  }
  if (start < 0 || !/^##\s+TL;DR\b/i.test(lines[start])) {
    return [`The PR body must open with "${heading}".`];
  }
  const end = lines.findIndex((line, i) => i > start && /^#{1,2}\s/.test(line));
  const section = lines.slice(start + 1, end < 0 ? undefined : end).join('\n').trim();
  return section ? [] : [`"${heading}" is empty: say who notices what, or „Keine Änderung für Nutzer“.`];
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const errors = checkPrBody(process.env.PR_BODY);
  for (const error of errors) console.error(`::error title=PR body::${error}`);
  process.exitCode = errors.length ? 1 : 0;
}
