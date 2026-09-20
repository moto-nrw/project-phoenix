// One-off review artifact. Does not modify policy or create issues.
import { readFileSync, writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";

const root = "backend/architecture/rule-backfill-3416";
const sha = "4aa36c315fc19e8ae8dda50193ffa34843e25575";
const base = JSON.parse(execFileSync("git", ["show", `${sha}:backend/architecture/policy.json`], {encoding:"utf8"}));
const stale = new Set(readFileSync(`${root}/stale-rules.txt`, "utf8").trim().split("\n"));
const newTickets = {"class-day-tests":3444,"file-storage":3445,"identity-behavior":3446,"presence-http":3447,"settings-test-dates":3448};
const cleanup = {
  3217: [2750, "Root composition removes the retained import route wiring."],
  2707: ["file-storage", "Replace seven retained File Storage composition and test dependencies."],
  2701: ["class-day-tests", "Replace retained repositories passed to schedule services in class-day integration tests."],
  3364: ["identity-behavior", "Remove cross-owner implementation dependencies from the relocated Identity behavior suites."],
  3350: [3427, "Dissolve carelifecycle, switch its consumers and replace retained care-exit dependencies."],
  3224: [2725, "Identity carrier owns retained usercontext providers and their consumer switch."],
  3219: [3418, "Dissolve shiftplanning and retire inbound-staff-shifts legacy permissions."],
  3232: [2736, "Operator/auth carrier owns retained operator handler dependencies after relocation."],
  3218: [3424, "Dissolve timetableplanning and switch retained timetable consumers."],
  3220: [3351, "Dissolve careschedule and switch retained schedule consumers."],
  3214: [3422, "Dissolve Presence legacy providers and switch their remaining consumers."],
  3229: [3421, "Explicit inbound-parent compatibility conversion and removal ticket."],
};

function resolve(rule) {
  if (stale.has(rule.id)) return {id:rule.id, action:"delete", reason:"rules.stale: no allowed edge in any scope"};
  const mentions = [...rule.description.matchAll(/#(\d+)/g)].map(match => Number(match[1]));
  // Specific relocation wins over carrier mentions; these are proposed review decisions.
  const origin = [3350,3364,2707,2701,3217,3207,3229,3224,3219,3232,3214,3218,3220].find(n => mentions.includes(n));
  let assignment = cleanup[origin];
  if (origin === 3207) {
    assignment = rule.source_owner === "root-composition"
      ? [2750,"Root composition removes retained presence route wiring."]
      : rule.source_owner === "settings-platform"
        ? ["settings-test-dates","Settings test support must stop exposing the retained date type."]
        : ["presence-http","Replace shared helper, date-type and settings implementation imports in Presence HTTP and adapter tests."];
  }
  if (origin === 3229 && rule.source_owner === "root-composition") assignment = [2750,"Root composition removes retained parent adapter wiring."];
  if (origin === 3229 && rule.source_owner === "test-support") assignment = [2748,"Shared test composition removes retained parent adapter wiring."];
  if (origin === 3214 && rule.source_owner === "inbound-students") assignment = [3352,"Students Presence front ticket explicitly owns this consumer."];
  if (origin === 3214 && rule.source_owner === "inbound-parent") assignment = [3421,"Explicit inbound-parent compatibility conversion and removal ticket."];
  if (origin === 3220 && !rule.description.includes("careschedule") && rule.source_owner === "inbound-timetable" && rule.target_owner !== "inbound-schedules") assignment = [3424,"Retained Timetable implementation dependency, not a care-schedule consumer edge."];
  if (!assignment) throw new Error(`Unresolved rule ${rule.id}`);
  const [owner, reason] = assignment;
  return {id:rule.id, action:"assign", issue:`https://github.com/moto-nrw/project-phoenix/issues/${typeof owner === "number" ? owner : newTickets[owner]}`, ...(typeof owner === "number" ? {} : {new_ticket:owner}), reason};
}

const rows = base.rules.filter(rule => rule.description.includes("Compatibility permission") || rule.description.includes("convert it to exact debt")).map(resolve);
writeFileSync(`${root}/resolution.jsonl`, rows.map(row => JSON.stringify(row)).join("\n")+"\n");
const counts = new Map();
for (const row of rows) {
  const owner = row.issue ?? row.new_ticket ?? "delete";
  counts.set(owner, (counts.get(owner) ?? 0)+1);
}
const lines = ["# Rule issue backfill review (#3416)","","**Status: assignments approved by the user in this implementation session. Cleanup issues #3444–#3448 created.**","",`Evidence commit: \`${sha}\`.`,"","Baseline: 2,040 rules, 504 compatibility descriptions, 682 legacy entries, policy epoch 17. The fixed-build graph reports 167 stale rules, including 19 compatibility rules. Deletion leaves 1,873 rules and 485 temporary permissions. The legacy manifest is unchanged.","","## Review decisions","","The user approved the cleanup mappings and five new tickets below. Every compatibility rule has a row and rationale in [resolution.jsonl](resolution.jsonl). Its original description and extracted historical issue numbers are in [proposal.jsonl](proposal.jsonl). These historical mentions are not cleanup assignments. All 167 deleted IDs are in [stale-rules.txt](stale-rules.txt).","","| Cleanup owner | Rules |","|---|---:|"];
for (const [owner,count] of [...counts].sort()) lines.push(`| ${owner} | ${count} |`);
lines.push("","## Created cleanup tickets","","Each new ticket is a child of #2580 and follows up #3416. Tickets stay open until all listed permissions and any exact debt converted from them are removed.");
for (const owner of Object.keys(newTickets).sort()) {
  const assigned = rows.filter(row => row.new_ticket === owner);
  lines.push("",`### ${owner} (#${newTickets[owner]})`,"",assigned[0].reason,"","Exit criteria: remove these implementation dependencies through public capabilities or consumer-owned ports; delete the permissions; preserve behavior and tenant isolation; pass affected behavior tests, the architecture check and issue audit. Any conversion to exact debt must retain this ticket as owner until no rule or legacy entry references it.","","Rules:","",...assigned.map(row => `- \`${row.id}\``));
}
lines.push("","## Reproduction","","Run `scripts/propose-rule-issues.mjs` against the evidence commit's policy. Run `scripts/resolve-rule-issues-3416.mjs` to reproduce the reviewed resolution. Neither script edits policy or creates tickets.","","Schema changes to 3; policy epoch stays 17. Existing cleanup tickets were confirmed open during preparation. The final audit must recheck the union of rule and legacy issues.","");
writeFileSync(`${root}/README.md`, lines.join("\n"));
console.log(Object.fromEntries(counts));
