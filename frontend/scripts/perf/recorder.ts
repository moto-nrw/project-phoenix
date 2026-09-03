import type { Page, Request } from "@playwright/test";

export interface RequestRecord {
  url: string;
  /** Pfad + Query, ohne Host. */
  path: string;
  method: string;
  resourceType: string;
  status: number | null;
  startMs: number;
  endMs: number | null;
  durationMs: number | null;
  /** Response-Body-Bytes über die Leitung (Playwright `request.sizes()`). */
  bytes: number | null;
  /** SSE, Sentry-Tunnel, PostHog, Log-Shipping: aufgezeichnet, aber nie fürs Settle gezählt. */
  noise: boolean;
  failed: boolean;
}

const NOISE = [
  /\/api\/(?:[a-z]+\/)?sse\//,
  /\/monitoring/,
  /posthog/i,
  /\/api\/logs$/,
];

function isNoise(url: string, resourceType: string): boolean {
  if (resourceType === "eventsource") return true;
  return NOISE.some((pattern) => pattern.test(url));
}

function pathOf(url: string): string {
  try {
    const parsed = new URL(url);
    return `${parsed.pathname}${parsed.search}`;
  } catch {
    return url;
  }
}

/**
 * Zeichnet jeden Request einer Page auf. `start()` setzt den Nullpunkt;
 * `waitForQuiet()` ersetzt `networkidle`, das wegen SSE nie eintritt.
 */
export class RequestRecorder {
  records: RequestRecord[] = [];
  private t0 = 0;
  private lastActivity = 0;
  private readonly pending = new Map<Request, RequestRecord>();

  constructor(page: Page) {
    page.on("request", (request) => {
      if (this.t0 === 0) return;
      const record: RequestRecord = {
        url: request.url(),
        path: pathOf(request.url()),
        method: request.method(),
        resourceType: request.resourceType(),
        status: null,
        startMs: this.now(),
        endMs: null,
        durationMs: null,
        bytes: null,
        noise: isNoise(request.url(), request.resourceType()),
        failed: false,
      };
      this.records.push(record);
      this.pending.set(request, record);
      if (!record.noise) this.lastActivity = record.startMs;
    });
    page.on("response", (response) => {
      const record = this.pending.get(response.request());
      if (record) record.status = response.status();
    });
    page.on("requestfinished", (request) => {
      void this.finish(request, false);
    });
    page.on("requestfailed", (request) => {
      void this.finish(request, true);
    });
  }

  private now(): number {
    return performance.now() - this.t0;
  }

  private async finish(request: Request, failed: boolean): Promise<void> {
    const record = this.pending.get(request);
    if (!record) return;
    this.pending.delete(request);
    record.failed = failed;
    record.endMs = this.now();
    if (!record.noise) this.lastActivity = record.endMs;
    try {
      const timing = request.timing();
      record.durationMs =
        timing.responseEnd >= 0
          ? timing.responseEnd
          : record.endMs - record.startMs;
    } catch {
      record.durationMs = record.endMs - record.startMs;
    }
    try {
      const sizes = await request.sizes();
      // Aus dem Cache bediente Antworten melden -1.
      record.bytes =
        sizes.responseBodySize >= 0 ? sizes.responseBodySize : null;
    } catch {
      record.bytes = null;
    }
  }

  start(): void {
    this.records = [];
    this.pending.clear();
    this.t0 = performance.now();
    this.lastActivity = 0;
  }

  /** Index-Marke, um später nur die Requests ab hier auszuwerten. */
  mark(): number {
    return this.records.length;
  }

  since(mark: number): RequestRecord[] {
    return this.records.slice(mark);
  }

  /**
   * Wartet, bis `quietMs` lang kein Nicht-Noise-Request startete oder endete,
   * spätestens bis `capMs` nach dem letzten `start()`.
   */
  async waitForQuiet(options: {
    quietMs: number;
    capMs: number;
  }): Promise<"quiet" | "cap"> {
    for (;;) {
      const now = this.now();
      const openRequests = [...this.pending.values()].some((r) => !r.noise);
      if (!openRequests && now - this.lastActivity >= options.quietMs) {
        return "quiet";
      }
      if (now >= options.capMs) return "cap";
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
  }
}

export interface Vitals {
  ttfbMs: number | null;
  fcpMs: number | null;
  lcpMs: number | null;
  domContentLoadedMs: number | null;
  cls: number;
  longTaskCount: number;
  longTaskTotalMs: number;
  /** Summe (Dauer - 50 ms) über alle Long Tasks, die TBT-Näherung. */
  totalBlockingMs: number;
}

interface PerfWindow {
  __perf?: { lcp: number; cls: number; longTasks: number[] };
}

/**
 * Muss VOR `page.goto()` registriert werden: läuft dann in jedem neuen
 * Dokument vor App-Code, `buffered: true` holt frühe Einträge nach.
 */
export async function installVitalsObserver(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const state = { lcp: 0, cls: 0, longTasks: [] as number[] };
    (window as PerfWindow).__perf = state;
    const observe = (
      type: string,
      handler: (entry: PerformanceEntry) => void,
    ) => {
      try {
        new PerformanceObserver((list) => {
          for (const entry of list.getEntries()) handler(entry);
        }).observe({ type, buffered: true });
      } catch {
        // Entry-Typ nicht unterstützt: Wert bleibt 0.
      }
    };
    observe("largest-contentful-paint", (entry) => {
      state.lcp = entry.startTime;
    });
    observe("layout-shift", (entry) => {
      const shift = entry as PerformanceEntry & {
        hadRecentInput?: boolean;
        value?: number;
      };
      if (!shift.hadRecentInput) state.cls += shift.value ?? 0;
    });
    observe("longtask", (entry) => {
      state.longTasks.push(entry.duration);
    });
  });
}

export async function readVitals(page: Page): Promise<Vitals> {
  return page.evaluate(() => {
    const perf = (window as PerfWindow).__perf ?? {
      lcp: 0,
      cls: 0,
      longTasks: [],
    };
    const nav = performance.getEntriesByType("navigation")[0] as
      PerformanceNavigationTiming | undefined;
    const fcp = performance
      .getEntriesByType("paint")
      .find((entry) => entry.name === "first-contentful-paint");
    return {
      ttfbMs: nav ? nav.responseStart : null,
      fcpMs: fcp ? fcp.startTime : null,
      lcpMs: perf.lcp > 0 ? perf.lcp : null,
      domContentLoadedMs: nav ? nav.domContentLoadedEventEnd : null,
      cls: perf.cls,
      longTaskCount: perf.longTasks.length,
      longTaskTotalMs: perf.longTasks.reduce((sum, d) => sum + d, 0),
      totalBlockingMs: perf.longTasks.reduce(
        (sum, d) => sum + Math.max(0, d - 50),
        0,
      ),
    };
  });
}

interface ChainStep {
  path: string;
  method: string;
  startMs: number;
  endMs: number;
}

export interface RequestSummary {
  requests: number;
  apiRequests: number;
  staticRequests: number;
  noiseRequests: number;
  failedRequests: number;
  /** Fehlgeschlagene oder abgebrochene Requests, `resourceType path`, dedupliziert. */
  failed: string[];
  jsBytes: number;
  cssBytes: number;
  apiBytes: number;
  duplicateApi: Array<{ key: string; count: number; startsMs: number[] }>;
  longestChain: { length: number; spanMs: number; steps: ChainStep[] };
  slowestApi: Array<{ path: string; method: string; durationMs: number }>;
  api: Array<{
    path: string;
    method: string;
    status: number | null;
    startMs: number;
    durationMs: number | null;
    bytes: number | null;
  }>;
}

function isApi(record: RequestRecord): boolean {
  return !record.noise && record.path.startsWith("/api/");
}

function isStatic(record: RequestRecord): boolean {
  return record.path.startsWith("/_next/static/");
}

/** B folgt A, wenn B kurz nach dem Ende von A startet: ein Wasserfall-Schritt. */
const CHAIN_MAX_GAP_MS = 300;

/**
 * Längste Kette sequenzieller API-Requests: B hängt an A, wenn B frühestens
 * mit dem Ende von A und höchstens CHAIN_MAX_GAP_MS danach startet. Requests
 * eines parallelen Bursts (gleicher Start) zählen damit nicht als Kette.
 * Dynamische Programmierung über die nach Start sortierte Liste.
 */
function longestChain(api: RequestRecord[]): RequestSummary["longestChain"] {
  const sorted = api
    .filter((r): r is RequestRecord & { endMs: number } => r.endMs !== null)
    .sort((a, b) => a.startMs - b.startMs);
  const best = sorted.map(() => 1);
  const prev = sorted.map(() => -1);
  for (let i = 0; i < sorted.length; i += 1) {
    for (let j = 0; j < i; j += 1) {
      const a = sorted[j]!;
      const b = sorted[i]!;
      const gap = b.startMs - a.endMs;
      if (gap >= 0 && gap <= CHAIN_MAX_GAP_MS && best[j]! + 1 > best[i]!) {
        best[i] = best[j]! + 1;
        prev[i] = j;
      }
    }
  }
  let end = -1;
  for (let i = 0; i < sorted.length; i += 1) {
    if (end === -1 || best[i]! > best[end]!) end = i;
  }
  if (end === -1) return { length: 0, spanMs: 0, steps: [] };
  const steps: ChainStep[] = [];
  for (let i = end; i !== -1; i = prev[i]!) {
    const r = sorted[i]!;
    steps.unshift({
      path: r.path,
      method: r.method,
      startMs: r.startMs,
      endMs: r.endMs,
    });
  }
  return {
    length: steps.length,
    spanMs: steps[steps.length - 1]!.endMs - steps[0]!.startMs,
    steps,
  };
}

export function summarizeRequests(records: RequestRecord[]): RequestSummary {
  const api = records.filter(isApi);
  const byKey = new Map<string, number[]>();
  for (const r of api) {
    const key = `${r.method} ${r.path}`;
    const starts = byKey.get(key) ?? [];
    starts.push(Math.round(r.startMs));
    byKey.set(key, starts);
  }
  const sum = (list: RequestRecord[]) =>
    list.reduce((total, r) => total + (r.bytes ?? 0), 0);
  return {
    requests: records.filter((r) => !r.noise).length,
    apiRequests: api.length,
    staticRequests: records.filter(isStatic).length,
    noiseRequests: records.filter((r) => r.noise).length,
    failedRequests: records.filter((r) => r.failed && !r.noise).length,
    failed: [
      ...new Set(
        records
          .filter((r) => r.failed && !r.noise)
          .map((r) => `${r.resourceType} ${r.path.replace(/\?.*$/, "")}`),
      ),
    ].slice(0, 20),
    jsBytes: sum(
      records.filter((r) => isStatic(r) && /\.js(\?|$)/.test(r.path)),
    ),
    cssBytes: sum(
      records.filter((r) => isStatic(r) && /\.css(\?|$)/.test(r.path)),
    ),
    apiBytes: sum(api),
    duplicateApi: [...byKey.entries()]
      .filter(([, starts]) => starts.length > 1)
      .map(([key, starts]) => ({
        key,
        count: starts.length,
        startsMs: starts,
      })),
    longestChain: longestChain(api),
    slowestApi: [...api]
      .filter((r) => r.durationMs !== null)
      .sort((a, b) => (b.durationMs ?? 0) - (a.durationMs ?? 0))
      .slice(0, 10)
      .map((r) => ({
        path: r.path,
        method: r.method,
        durationMs: Math.round(r.durationMs ?? 0),
      })),
    api: api.map((r) => ({
      path: r.path,
      method: r.method,
      status: r.status,
      startMs: Math.round(r.startMs),
      durationMs: r.durationMs === null ? null : Math.round(r.durationMs),
      bytes: r.bytes,
    })),
  };
}
