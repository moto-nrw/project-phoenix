// Fasst perf-results/ zu Markdown-Tabellen zusammen (pnpm run perf:report, #2938).
// Liest die JSONs aus measure.perf.ts (Median über die Wiederholungen), die
// react-scan-JSONs und die Prometheus-Scrapes vor/nach dem Lauf (Differenz).
import { existsSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";

const OUT_DIR = join(process.cwd(), "perf-results");
// Muss zu BUCKETS in src/lib/backend-proxy-metrics.ts passen.
const BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10];

/** @param {number[]} values */
function median(values) {
  const sorted = values
    .filter((v) => v !== null && !Number.isNaN(v))
    .sort((a, b) => a - b);
  if (sorted.length === 0) return null;
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2;
}

/** @param {number[]} values */
function range(values) {
  const sorted = values.filter((v) => v !== null).sort((a, b) => a - b);
  if (sorted.length === 0) return "";
  return `${fmt(sorted[0])}–${fmt(sorted[sorted.length - 1])}`;
}

function fmt(value) {
  if (value === null || value === undefined) return "–";
  return Math.round(value).toLocaleString("de-DE");
}

function kb(bytes) {
  return bytes === null ? "–" : `${(bytes / 1024).toFixed(0)} kB`;
}

// ---------------------------------------------------------------- measure
function loadMeasurements() {
  if (!existsSync(OUT_DIR)) return new Map();
  const byTarget = new Map();
  for (const file of readdirSync(OUT_DIR)) {
    const match = /^(.+)\.(\d+)\.json$/.exec(file);
    if (!match) continue;
    const data = JSON.parse(readFileSync(join(OUT_DIR, file), "utf8"));
    const list = byTarget.get(match[1]) ?? [];
    list.push(data);
    byTarget.set(match[1], list);
  }
  return byTarget;
}

function screenTable(byTarget) {
  const rows = [
    "| Screen | TTFB | FCP | LCP | DCL | Settle (kalt) | Settle (warm) | Requests | /api/ | Duplikate | Längste Kette | JS kalt | JS warm | Long Tasks | TBT |",
    "|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|",
  ];
  for (const [target, runs] of byTarget) {
    const cold = runs.map((r) => r.cold);
    const warm = runs.map((r) => r.warm);
    const m = (list, pick) => fmt(median(list.map(pick)));
    rows.push(
      `| ${target} | ${m(cold, (c) => c.vitals.ttfbMs)} ms | ${m(cold, (c) => c.vitals.fcpMs)} ms | ${m(cold, (c) => c.vitals.lcpMs)} ms | ${m(cold, (c) => c.vitals.domContentLoadedMs)} ms | ${m(cold, (c) => c.settleMs)} ms (${range(cold.map((c) => c.settleMs))}) | ${m(warm, (c) => c.settleMs)} ms | ${m(cold, (c) => c.requests.requests)} | ${m(cold, (c) => c.requests.apiRequests)} | ${m(cold, (c) => c.requests.duplicateApi.length)} | ${m(cold, (c) => c.requests.longestChain.length)} (${m(cold, (c) => c.requests.longestChain.spanMs)} ms) | ${kb(median(cold.map((c) => c.requests.jsBytes)))} | ${kb(median(warm.map((c) => c.requests.jsBytes)))} | ${m(cold, (c) => c.vitals.longTaskCount)} (${m(cold, (c) => c.vitals.longTaskTotalMs)} ms) | ${m(cold, (c) => c.vitals.totalBlockingMs)} ms |`,
    );
  }
  return rows.join("\n");
}

function interactionTable(byTarget) {
  const rows = [
    "| Screen | Interaktion | Dauer bis ruhig | Requests | /api/ | Duplikate | Long Tasks |",
    "|---|---|---|---|---|---|---|",
  ];
  for (const [target, runs] of byTarget) {
    const list = runs.map((r) => r.interaction).filter(Boolean);
    if (list.length === 0) continue;
    const m = (pick) => fmt(median(list.map(pick)));
    rows.push(
      `| ${target} | ${list[0].label} | ${m((i) => i.durationMs)} ms | ${m((i) => i.requests.requests)} | ${m((i) => i.requests.apiRequests)} | ${m((i) => i.requests.duplicateApi.length)} | ${m((i) => i.longTasksMs)} ms |`,
    );
  }
  return rows.join("\n");
}

function detailSections(byTarget) {
  const out = [];
  for (const [target, runs] of byTarget) {
    const run = runs[0];
    out.push(`### ${target} (${run.path}, Lauf 1 von ${runs.length})`);
    out.push("");
    out.push(
      `Settle: ${run.cold.settledBy}. API-Requests des kalten Aufrufs in Startreihenfolge:`,
    );
    out.push("");
    out.push("| Start | Dauer | Status | Bytes | Request |");
    out.push("|---|---|---|---|---|");
    for (const r of run.cold.requests.api) {
      out.push(
        `| ${fmt(r.startMs)} ms | ${fmt(r.durationMs)} ms | ${r.status ?? "–"} | ${kb(r.bytes)} | \`${r.method} ${r.path}\` |`,
      );
    }
    if (run.cold.requests.duplicateApi.length > 0) {
      out.push("");
      out.push("Duplikate (gleiche Methode + URL):");
      for (const d of run.cold.requests.duplicateApi) {
        out.push(
          `- \`${d.key}\` ×${d.count} (Starts bei ${d.startsMs.join(", ")} ms)`,
        );
      }
    }
    if (run.cold.requests.longestChain.length > 1) {
      out.push("");
      out.push(
        `Längste sequenzielle Kette (${run.cold.requests.longestChain.length} Requests, ${fmt(run.cold.requests.longestChain.spanMs)} ms):`,
      );
      for (const s of run.cold.requests.longestChain.steps) {
        out.push(
          `- ${fmt(s.startMs)}–${fmt(s.endMs)} ms \`${s.method} ${s.path}\``,
        );
      }
    }
    if (run.interaction) {
      out.push("");
      out.push(
        `Interaktion „${run.interaction.label}“: ${run.interaction.requests.apiRequests} API-Requests, ${run.interaction.requests.duplicateApi.length} Duplikate.`,
      );
      for (const r of run.interaction.requests.api) {
        out.push(
          `- ${fmt(r.startMs)} ms, ${fmt(r.durationMs)} ms \`${r.method} ${r.path}\``,
        );
      }
    }
    out.push("");
  }
  return out.join("\n");
}

// ---------------------------------------------------------------- metrics
/** Prometheus-Textformat -> Map<seriesKey, value> */
function parsePrometheus(text) {
  const series = new Map();
  for (const line of text.split("\n")) {
    if (!line || line.startsWith("#")) continue;
    const match = /^([a-zA-Z_:][a-zA-Z0-9_:]*)(\{[^}]*\})?\s+(\S+)/.exec(line);
    if (!match) continue;
    series.set(`${match[1]}${match[2] ?? ""}`, Number(match[3]));
  }
  return series;
}

function parseLabels(labelText) {
  const labels = {};
  for (const m of labelText.matchAll(/([a-zA-Z_]+)="((?:[^"\\]|\\.)*)"/g)) {
    labels[m[1]] = m[2];
  }
  return labels;
}

function diffSeries(final, baseline) {
  const diff = new Map();
  for (const [key, value] of final) {
    diff.set(key, value - (baseline.get(key) ?? 0));
  }
  return diff;
}

function quantile(buckets, count, q) {
  // buckets: kumulative Zähler je BUCKETS-Grenze (+Inf am Ende).
  const target = q * count;
  let previous = 0;
  let previousBound = 0;
  for (let i = 0; i < BUCKETS.length; i += 1) {
    const cumulative = buckets[i];
    if (cumulative >= target) {
      const inBucket = cumulative - previous;
      if (inBucket === 0) return BUCKETS[i];
      return (
        previousBound +
        ((target - previous) / inBucket) * (BUCKETS[i] - previousBound)
      );
    }
    previous = cumulative;
    previousBound = BUCKETS[i];
  }
  return Number.POSITIVE_INFINITY;
}

function metricsTables() {
  const baselinePath = join(OUT_DIR, "metrics-baseline.txt");
  const finalPath = join(OUT_DIR, "metrics-final.txt");
  if (!existsSync(baselinePath) || !existsSync(finalPath))
    return "_keine Proxy-Metriken (metrics-*.txt fehlt)_";
  const diff = diffSeries(
    parsePrometheus(readFileSync(finalPath, "utf8")),
    parsePrometheus(readFileSync(baselinePath, "utf8")),
  );
  const endpoints = new Map();
  for (const [key, value] of diff) {
    const name = key.split("{")[0];
    const labels = parseLabels(key.slice(name.length));
    if (
      !name.startsWith(
        "phoenix_frontend_backend_proxy_request_duration_seconds",
      )
    )
      continue;
    const id = `${labels.method} ${labels.backend_endpoint}`;
    const entry = endpoints.get(id) ?? {
      id,
      count: 0,
      sum: 0,
      buckets: new Array(BUCKETS.length + 1).fill(0),
    };
    if (name.endsWith("_count")) entry.count += value;
    else if (name.endsWith("_sum")) entry.sum += value;
    else if (name.endsWith("_bucket")) {
      const index =
        labels.le === "+Inf"
          ? BUCKETS.length
          : BUCKETS.indexOf(Number(labels.le));
      if (index >= 0) entry.buckets[index] += value;
    }
    endpoints.set(id, entry);
  }
  const list = [...endpoints.values()].filter((e) => e.count > 0);
  const total = list.reduce((s, e) => s + e.count, 0);
  const row = (e) =>
    `| \`${e.id}\` | ${e.count} | ${fmt((e.sum / e.count) * 1000)} ms | ${fmt(quantile(e.buckets, e.count, 0.5) * 1000)} ms | ${fmt(quantile(e.buckets, e.count, 0.95) * 1000)} ms |`;
  const header =
    "| Backend-Endpunkt | Anzahl | Ø | p50 ≈ | p95 ≈ |\n|---|---|---|---|---|";
  const byCount = [...list]
    .sort((a, b) => b.count - a.count)
    .slice(0, 15)
    .map(row)
    .join("\n");
  const byP95 = [...list]
    .sort(
      (a, b) =>
        quantile(b.buckets, b.count, 0.95) - quantile(a.buckets, a.count, 0.95),
    )
    .slice(0, 15)
    .map(row)
    .join("\n");
  return `Proxy-Requests im Messlauf gesamt: ${total} (Differenz final − baseline, ${list.length} Endpunkte).\n\n**Top 15 nach Anzahl**\n\n${header}\n${byCount}\n\n**Top 15 nach p95**\n\n${header}\n${byP95}`;
}

// ---------------------------------------------------------------- react-scan
function renderTables() {
  const dir = join(OUT_DIR, "react-scan");
  if (!existsSync(dir)) return "_keine react-scan-Daten_";
  const out = [];
  out.push(
    "| Screen | Komponenten | Renders bis ruhig (Fenster) | Renders im Leerlauf (10 s) | Renders je Sekunde Leerlauf | Interaktion | Renders Interaktion |",
  );
  out.push("|---|---|---|---|---|---|---|");
  const details = [];
  const line = (e) =>
    `- \`${e.component}\`: ${e.renders} Renders (${e.updates} Updates), ${e.timeMs} ms Dev-Renderzeit`;
  for (const file of readdirSync(dir)
    .filter((f) => f.endsWith(".json"))
    .sort()) {
    const data = JSON.parse(readFileSync(join(dir, file), "utf8"));
    const i = data.interaction;
    const m = data.mount;
    const idle = data.idle;
    out.push(
      `| ${data.target} | ${m.components} | ${m.renders} (${m.windowSeconds} s) | ${idle.renders} | ${fmt(idle.renders / idle.windowSeconds)} | ${i ? i.label : "–"} | ${i ? i.renders : "–"} |`,
    );
    details.push(
      `### ${data.target}\n\nTop-Komponenten im Leerlauf (10 s, Seite steht):\n`,
    );
    for (const e of idle.top.slice(0, 10)) details.push(line(e));
    if (i) {
      details.push(
        `\nTop-Komponenten bei „${i.label}“ (${i.windowSeconds} s):\n`,
      );
      for (const e of i.top.slice(0, 10)) details.push(line(e));
    }
    details.push("");
  }
  return `${out.join("\n")}\n\n${details.join("\n")}`;
}

// ---------------------------------------------------------------- main
const byTarget = loadMeasurements();
const report = [
  "## Ergebnisse pro Screen (Median über die Wiederholungen, kalter Aufruf)",
  "",
  screenTable(byTarget),
  "",
  "## Interaktionen",
  "",
  interactionTable(byTarget),
  "",
  "## Wasserfall-Details",
  "",
  detailSections(byTarget),
  "## Proxy-Metriken",
  "",
  metricsTables(),
  "",
  "## Render-Profiling (react-scan, Dev-Server)",
  "",
  renderTables(),
  "",
].join("\n");

writeFileSync(join(OUT_DIR, "report.md"), report);
process.stdout.write(report);
