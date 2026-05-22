import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import client from "prom-client";

const GLOBAL_KEY = Symbol.for("phoenix.frontend.metrics");

interface MetricsState {
  registry: client.Registry;
  backendProxyRequests: client.Counter<string>;
  backendProxyDuration: client.Histogram<string>;
}

declare global {
  // eslint-disable-next-line no-var
  var __phoenixFrontendMetrics: MetricsState | undefined;
}

function getState(): MetricsState {
  if (globalThis.__phoenixFrontendMetrics) {
    return globalThis.__phoenixFrontendMetrics;
  }

  const registry = new client.Registry();
  registry.setDefaultLabels({ service: "phoenix-frontend" });
  client.collectDefaultMetrics({
    register: registry,
    prefix: "phoenix_frontend_",
    eventLoopMonitoringPrecision: 20,
  });

  const backendProxyRequests = new client.Counter({
    name: "phoenix_frontend_backend_proxy_requests_total",
    help: "Frontend server-side backend proxy requests.",
    labelNames: [
      "method",
      "backend_endpoint",
      "status_class",
      "outcome",
      "scope",
    ] as const,
    registers: [registry],
  });

  const backendProxyDuration = new client.Histogram({
    name: "phoenix_frontend_backend_proxy_request_duration_seconds",
    help: "Frontend server-side backend proxy request duration.",
    labelNames: ["method", "backend_endpoint", "outcome"] as const,
    buckets: [0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10],
    registers: [registry],
  });

  const state = { registry, backendProxyRequests, backendProxyDuration };
  globalThis.__phoenixFrontendMetrics = state;
  Reflect.set(globalThis, GLOBAL_KEY, state);
  return state;
}

export function recordBackendProxyMetric(args: {
  method: string;
  backendEndpoint: string;
  status: number;
  durationMs: number;
  outcome: string;
  scope?: string;
}): void {
  const state = getState();
  const statusClass = statusClassFor(args.status);
  const scope = args.scope && args.scope.trim() !== "" ? args.scope : "tenant";
  state.backendProxyRequests
    .labels(args.method, args.backendEndpoint, statusClass, args.outcome, scope)
    .inc();
  state.backendProxyDuration
    .labels(args.method, args.backendEndpoint, args.outcome)
    .observe(args.durationMs / 1000);
}

export async function metricsResponse(
  request: NextRequest,
): Promise<NextResponse> {
  const token = process.env.METRICS_BEARER_TOKEN;
  if (!token) {
    return new NextResponse("metrics are not configured", { status: 503 });
  }
  if (request.headers.get("authorization") !== `Bearer ${token}`) {
    return new NextResponse("Unauthorized", { status: 401 });
  }

  const state = getState();
  return new NextResponse(await state.registry.metrics(), {
    status: 200,
    headers: {
      "Content-Type": state.registry.contentType,
      "Cache-Control": "no-store",
    },
  });
}

function statusClassFor(status: number): string {
  if (!Number.isFinite(status) || status <= 0) return "unknown";
  return `${Math.trunc(status / 100)}xx`;
}
