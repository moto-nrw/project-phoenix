import type { NextRequest } from "next/server";
import { NextResponse } from "next/server";
import client from "prom-client";
import {
  backendProxyMetricBuckets,
  backendProxyMetricSnapshot,
} from "./backend-proxy-metrics";

const GLOBAL_KEY = Symbol.for("phoenix.frontend.metrics");

interface MetricsState {
  registry: client.Registry;
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

  const state = { registry };
  globalThis.__phoenixFrontendMetrics = state;
  Reflect.set(globalThis, GLOBAL_KEY, state);
  return state;
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
  const body = `${await state.registry.metrics()}${renderBackendProxyMetrics()}`;
  return new NextResponse(body, {
    status: 200,
    headers: {
      "Content-Type": state.registry.contentType,
      "Cache-Control": "no-store",
    },
  });
}

function renderBackendProxyMetrics(): string {
  const snapshot = backendProxyMetricSnapshot();
  const lines = [
    "# HELP phoenix_frontend_backend_proxy_requests_total Frontend server-side backend proxy requests.",
    "# TYPE phoenix_frontend_backend_proxy_requests_total counter",
  ];

  snapshot.requestSamples.forEach((sample) => {
    lines.push(
      `phoenix_frontend_backend_proxy_requests_total{${labels({
        method: sample.method,
        backend_endpoint: sample.backendEndpoint,
        status_class: sample.statusClass,
        outcome: sample.outcome,
        scope: sample.scope,
        service: "phoenix-frontend",
      })}} ${sample.count}`,
    );
  });

  lines.push(
    "# HELP phoenix_frontend_backend_proxy_request_duration_seconds Frontend server-side backend proxy request duration.",
    "# TYPE phoenix_frontend_backend_proxy_request_duration_seconds histogram",
  );

  snapshot.durationSamples.forEach((sample) => {
    const baseLabels = {
      service: "phoenix-frontend",
      method: sample.method,
      backend_endpoint: sample.backendEndpoint,
      outcome: sample.outcome,
    };
    backendProxyMetricBuckets().forEach((bucket, index) => {
      lines.push(
        `phoenix_frontend_backend_proxy_request_duration_seconds_bucket{${labels(
          {
            ...baseLabels,
            le: bucket.toString(),
          },
        )}} ${sample.buckets[index] ?? 0}`,
      );
    });
    lines.push(
      `phoenix_frontend_backend_proxy_request_duration_seconds_bucket{${labels({
        ...baseLabels,
        le: "+Inf",
      })}} ${sample.count}`,
      `phoenix_frontend_backend_proxy_request_duration_seconds_sum{${labels(
        baseLabels,
      )}} ${sample.sumSeconds}`,
      `phoenix_frontend_backend_proxy_request_duration_seconds_count{${labels(
        baseLabels,
      )}} ${sample.count}`,
    );
  });

  return `${lines.join("\n")}\n`;
}

function labels(values: Record<string, string>): string {
  return Object.entries(values)
    .map(([key, value]) => `${key}="${escapeLabelValue(value)}"`)
    .join(",");
}

function escapeLabelValue(value: string): string {
  return value
    .replace(/\\/g, "\\\\")
    .replace(/\n/g, "\\n")
    .replace(/"/g, '\\"');
}
