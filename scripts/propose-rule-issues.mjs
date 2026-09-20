#!/usr/bin/env node
// One-off #3416 proposal only. Historical issue mentions are not cleanup owners.
import { readFileSync } from "node:fs";

if (process.argv.length !== 3) {
  throw new Error("Usage: node scripts/propose-rule-issues.mjs <policy.json>");
}
const policy = JSON.parse(readFileSync(process.argv[2], "utf8"));
for (const rule of policy.rules) {
  if (!rule.description.includes("Compatibility permission") &&
      !rule.description.includes("convert it to exact debt")) continue;
  const mentions = [...new Set([...rule.description.matchAll(/#(\d+)/g)].map(match => Number(match[1])))];
  console.log(JSON.stringify({id: rule.id, proposed_issues: mentions, description: rule.description}));
}
