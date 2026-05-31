import { beforeEach, describe, expect, it, vi } from "vitest";
import type { NextRequest } from "next/server";
import {
  recordBackendProxyMetric,
  resetBackendProxyMetricsForTest,
} from "./backend-proxy-metrics";
import { metricsResponse } from "./server-metrics";
import { getMetricsBearerToken } from "./server-runtime-env";

function metricsRequest(token?: string): NextRequest {
  return {
    headers: new Headers(token ? { authorization: `Bearer ${token}` } : {}),
  } as NextRequest;
}

describe("server-metrics", () => {
  beforeEach(() => {
    vi.unstubAllEnvs();
    globalThis.__phoenixFrontendMetrics = undefined;
    resetBackendProxyMetricsForTest();
  });

  it("throws when the metrics token is not configured", async () => {
    await expect(metricsResponse(metricsRequest())).rejects.toThrow(
      "METRICS_BEARER_TOKEN is required",
    );
  });

  it("trims the configured metrics token", () => {
    vi.stubEnv("METRICS_BEARER_TOKEN", " test-token-with-enough-length ");

    expect(getMetricsBearerToken()).toBe("test-token-with-enough-length");
  });

  it("rejects requests without the bearer token", async () => {
    vi.stubEnv("METRICS_BEARER_TOKEN", "test-token-with-enough-length");

    const response = await metricsResponse(metricsRequest("wrong-token"));

    expect(response.status).toBe(401);
    await expect(response.text()).resolves.toBe("Unauthorized");
  });

  it("exports recorded backend proxy metrics", async () => {
    vi.stubEnv("METRICS_BEARER_TOKEN", "test-token-with-enough-length");

    recordBackendProxyMetric({
      method: "GET",
      backendEndpoint: "/api/students/{id}",
      status: 200,
      durationMs: 125,
      outcome: "success",
      scope: "",
    });
    recordBackendProxyMetric({
      method: "POST",
      backendEndpoint: "/api/iot/scan",
      status: 0,
      durationMs: 10,
      outcome: "network_error",
      scope: "operator",
    });

    const response = await metricsResponse(
      metricsRequest("test-token-with-enough-length"),
    );
    const body = await response.text();

    expect(response.status).toBe(200);
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(body).toContain(
      'method="GET",backend_endpoint="/api/students/{id}",status_class="2xx",outcome="success",scope="tenant",service="phoenix-frontend"} 1',
    );
    expect(body).toContain(
      'method="POST",backend_endpoint="/api/iot/scan",status_class="unknown",outcome="network_error",scope="operator",service="phoenix-frontend"} 1',
    );
    expect(body).toContain(
      'phoenix_frontend_backend_proxy_request_duration_seconds_sum{service="phoenix-frontend",method="GET",backend_endpoint="/api/students/{id}",outcome="success"} 0.125',
    );
  });
});
