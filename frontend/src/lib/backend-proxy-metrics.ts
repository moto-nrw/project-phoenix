export interface BackendProxyMetricArgs {
  method: string;
  backendEndpoint: string;
  status: number;
  durationMs: number;
  outcome: string;
  scope?: string;
}

interface BackendProxyMetricState {
  requests: Map<string, number>;
  durations: Map<
    string,
    { count: number; sumSeconds: number; buckets: number[] }
  >;
}

export interface BackendProxyMetricSnapshot {
  requestSamples: Array<{
    method: string;
    backendEndpoint: string;
    statusClass: string;
    outcome: string;
    scope: string;
    count: number;
  }>;
  durationSamples: Array<{
    method: string;
    backendEndpoint: string;
    outcome: string;
    count: number;
    sumSeconds: number;
    buckets: number[];
  }>;
}

const BUCKETS = [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10];
const GLOBAL_KEY = "__phoenixFrontendBackendProxyMetrics";

declare global {
  // eslint-disable-next-line no-var
  var __phoenixFrontendBackendProxyMetrics: BackendProxyMetricState | undefined;
}

export function recordBackendProxyMetric(args: BackendProxyMetricArgs): void {
  const state = getBackendProxyMetricState();
  const scope = args.scope && args.scope.trim() !== "" ? args.scope : "tenant";
  const statusClass = statusClassFor(args.status);
  const requestKey = encodeKey([
    args.method,
    args.backendEndpoint,
    statusClass,
    args.outcome,
    scope,
  ]);
  state.requests.set(requestKey, (state.requests.get(requestKey) ?? 0) + 1);

  const durationKey = encodeKey([
    args.method,
    args.backendEndpoint,
    args.outcome,
  ]);
  const duration = state.durations.get(durationKey) ?? {
    count: 0,
    sumSeconds: 0,
    buckets: Array.from({ length: BUCKETS.length }, () => 0),
  };
  const durationSeconds = args.durationMs / 1000;
  duration.count += 1;
  duration.sumSeconds += durationSeconds;
  BUCKETS.forEach((bucket, index) => {
    if (durationSeconds <= bucket) {
      duration.buckets[index] = (duration.buckets[index] ?? 0) + 1;
    }
  });
  state.durations.set(durationKey, duration);
}

export function backendProxyMetricSnapshot(): BackendProxyMetricSnapshot {
  const state = getBackendProxyMetricState();
  return {
    requestSamples: Array.from(state.requests.entries()).map(([key, count]) => {
      const [method, backendEndpoint, statusClass, outcome, scope] =
        decodeRequestKey(key);
      return { method, backendEndpoint, statusClass, outcome, scope, count };
    }),
    durationSamples: Array.from(state.durations.entries()).map(
      ([key, sample]) => {
        const [method, backendEndpoint, outcome] = decodeDurationKey(key);
        return {
          method,
          backendEndpoint,
          outcome,
          count: sample.count,
          sumSeconds: sample.sumSeconds,
          buckets: [...sample.buckets],
        };
      },
    ),
  };
}

export function resetBackendProxyMetricsForTest(): void {
  globalThis.__phoenixFrontendBackendProxyMetrics = undefined;
}

export function backendProxyMetricBuckets(): readonly number[] {
  return BUCKETS;
}

function getBackendProxyMetricState(): BackendProxyMetricState {
  if (!globalThis.__phoenixFrontendBackendProxyMetrics) {
    globalThis.__phoenixFrontendBackendProxyMetrics = {
      requests: new Map(),
      durations: new Map(),
    };
    Reflect.set(
      globalThis,
      GLOBAL_KEY,
      globalThis.__phoenixFrontendBackendProxyMetrics,
    );
  }
  return globalThis.__phoenixFrontendBackendProxyMetrics;
}

function statusClassFor(status: number): string {
  if (!Number.isFinite(status) || status <= 0) return "unknown";
  return `${Math.trunc(status / 100)}xx`;
}

function encodeKey(parts: string[]): string {
  return JSON.stringify(parts);
}

function decodeRequestKey(
  key: string,
): [string, string, string, string, string] {
  const [method, backendEndpoint, statusClass, outcome, scope] = JSON.parse(
    key,
  ) as string[];
  return [
    method ?? "unknown",
    backendEndpoint ?? "unknown",
    statusClass ?? "unknown",
    outcome ?? "unknown",
    scope ?? "unknown",
  ];
}

function decodeDurationKey(key: string): [string, string, string] {
  const [method, backendEndpoint, outcome] = JSON.parse(key) as string[];
  return [
    method ?? "unknown",
    backendEndpoint ?? "unknown",
    outcome ?? "unknown",
  ];
}
