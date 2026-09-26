import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { captureException } = vi.hoisted(() => ({ captureException: vi.fn() }));
vi.mock("@sentry/nextjs", () => ({ captureException }));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));
vi.mock("~/server/auth", () => ({ auth: vi.fn(), uncachedAuth: vi.fn() }));
vi.mock("~/server/auth/tenant-route", () => ({ withTenantAuth: vi.fn() }));
vi.mock("~/server/auth/operator", () => ({
  operatorAuth: vi.fn(),
  uncachedOperatorAuth: vi.fn(),
}));
vi.mock("~/server/auth/operator-route", () => ({ withOperatorAuth: vi.fn() }));

const { createPublicJsonProxy } = await import("./backend-proxy-route.server");
const { handleApiError, ApiResponseError } =
  await import("./api-helpers.server");

const requestId = "0b6f3f4e-5c1d-4a52-9d57-2d3c1b5e8f10";
const request = () =>
  new NextRequest("http://localhost:3000/api/example", {
    headers: { "X-Request-ID": requestId },
  });

const proxy = createPublicJsonProxy({ method: "GET", path: "/api/example" });

describe("BFF Sentry ownership", () => {
  beforeEach(() => vi.clearAllMocks());
  afterEach(() => vi.unstubAllGlobals());

  it("forwards a backend 5xx without opening a BFF event", async () => {
    const body = '{"code":"db_unavailable","request_id":"backend-id"}';
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(body, {
          status: 503,
          headers: { "Content-Type": "application/problem+json" },
        }),
      ),
    );

    const response = await proxy(request());

    expect(response.status).toBe(503);
    expect(await response.text()).toBe(body);
    expect(captureException).not.toHaveBeenCalled();
  });

  it("reports an unreachable backend exactly once with the shared request ID", async () => {
    const failure = new TypeError("fetch failed");
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(failure));

    const response = await proxy(request());

    expect(response.status).toBe(500);
    expect(captureException).toHaveBeenCalledExactlyOnceWith(failure, {
      tags: { request_id: requestId },
    });
  });

  it("rejects malformed JSON before fetch without a BFF event", async () => {
    const send = createPublicJsonProxy({
      method: "POST",
      path: "/api/example",
    });
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const invalid = new NextRequest("http://localhost:3000/api/example", {
      method: "POST",
      body: '{"password":"cleartext-secret"',
    });

    const response = await send(invalid);

    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "Invalid JSON" });
    expect(fetchMock).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });

  it("reports an unexpected BFF exception, not a backend response error", () => {
    const failure = new Error("unexpected BFF exception");
    handleApiError(failure, request());
    expect(captureException).toHaveBeenCalledExactlyOnceWith(failure, {
      tags: { request_id: requestId },
    });
    vi.clearAllMocks();

    handleApiError(new ApiResponseError(500, "backend failure"), request());
    expect(captureException).not.toHaveBeenCalled();
  });

  it("does not include a parsed input fragment in a captured SyntaxError", () => {
    const rawError = new SyntaxError("Unexpected token in password=secret-123");
    handleApiError(rawError, request());

    const captured = captureException.mock.calls[0]?.[0] as SyntaxError;
    expect(captured).toBeInstanceOf(SyntaxError);
    expect(captured.message).toBe("Invalid JSON");
    expect(captured.stack).not.toContain("secret-123");
  });
});
