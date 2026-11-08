import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { NextRequest } from "next/server";

const { captureException } = vi.hoisted(() => ({ captureException: vi.fn() }));
vi.mock("@sentry/nextjs", () => ({ captureException }));

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

const { setTenantSession } = vi.hoisted(() => ({
  setTenantSession: vi.fn().mockResolvedValue(undefined),
}));
vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    setSession: setTenantSession,
    withSessionResponse: (handler: unknown) => handler,
  }),
);

const { POST } = await import("./route");

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "POST",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = {
      "Content-Type": "application/json",
      Origin: "http://localhost:3000",
    };
  }

  return new NextRequest(url, requestInit);
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/login", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("forwards credentials and returns only a session confirmation", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };
    const backendResponse = {
      access_token: "jwt-token",
      refresh_token: "refresh-token",
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);
    const [, requestInit] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];

    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(vi.mocked(global.fetch).mock.calls[0]?.[0]).toBe(
      "http://server:8080/auth/login",
    );
    expect(requestInit).toMatchObject({
      method: "POST",
      cache: "no-store",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(loginPayload),
    });
    // Identity headers travel only when the request carries them (no
    // "unknown" User-Agent placeholder any more).
    expect(requestInit.headers).not.toHaveProperty("User-Agent");

    expect(response.status).toBe(200);
    expect(await response.json()).toEqual({ status: "authenticated" });
    expect(setTenantSession).toHaveBeenCalledWith("jwt-token", "refresh-token");
  });

  it("returns 401 when credentials are invalid", async () => {
    const loginPayload = { email: "test@example.com", password: "wrong" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify({ detail: "Invalid credentials" }), {
        status: 401,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid credentials");
  });

  it("replaces a non-JSON backend 5xx response with a safe envelope", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("Server Error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    expect(response.headers.get("Content-Type")).toBe(
      "application/problem+json",
    );
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
    expect(captureException).not.toHaveBeenCalled();
  });

  it("handles JSON parse error from backend", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response("invalid json", {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({ code: "general.server" });
    expect(setTenantSession).not.toHaveBeenCalled();
  });

  it("rejects an empty success body without starting a session", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({ code: "general.server" });
    expect(setTenantSession).not.toHaveBeenCalled();
  });

  it("replaces an empty backend 5xx response with a safe envelope", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(null, {
        status: 502,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    expect(response.status).toBe(502);
    expect(await response.json()).toMatchObject({
      status: 502,
      detail: "Bad Gateway",
      code: "general.unavailable",
    });
  });

  // The backend reads only its remember-device cookie at login; the BFF's
  // own session cookies never travel to it.
  it("forwards the trusted-device cookie and no other cookie", async () => {
    vi.mocked(global.fetch).mockResolvedValueOnce(
      Response.json({ status: "mfa_required", challenge_token: "c" }),
    );
    const request = new NextRequest("http://localhost:3000/api/auth/login", {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Origin: "http://localhost:3000",
        cookie: "authjs.session-token=secret; mfa_trust_device=device-1",
      },
      body: JSON.stringify({ email: "a@b.de", password: "x" }),
    });
    // Anker: the request carries both cookies.
    expect(request.cookies.get("authjs.session-token")?.value).toBe("secret");

    await POST(request);

    const [, init] = vi.mocked(global.fetch).mock.calls[0] as [
      string,
      RequestInit,
    ];
    expect(new Headers(init.headers).get("cookie")).toBe(
      "mfa_trust_device=device-1",
    );
  });

  it("returns 503 when fetch throws an error", async () => {
    const loginPayload = { email: "test@example.com", password: "Test1234!" };

    const failure = new Error("Network error");
    vi.mocked(global.fetch).mockRejectedValueOnce(failure);

    const request = createMockRequest("/api/auth/login", {
      body: loginPayload,
    });
    const response = await POST(request);

    // A network failure answers 503 general.unavailable (was 500).
    expect(response.status).toBe(503);
    const json = await parseJsonResponse<{ code: string }>(response);
    expect(json.code).toBe("general.unavailable");
    expect(JSON.stringify(json)).not.toContain(failure.message);
    expect(captureException).toHaveBeenCalledExactlyOnceWith(failure, {});
  });

  it("rejects malformed request JSON without reporting its password", async () => {
    const secret = "cleartext-password-123";
    const request = new NextRequest("http://localhost:3000/api/auth/login", {
      method: "POST",
      headers: { Origin: "http://localhost:3000" },
      body: `{not-json ${secret}`,
    });

    const response = await POST(request);

    expect(response.status).toBe(400);
    expect(await response.text()).not.toContain(secret);
    expect(global.fetch).not.toHaveBeenCalled();
    expect(captureException).not.toHaveBeenCalled();
  });

  it("returns 400 when request parsing throws a non-Error value", async () => {
    const request = {
      json: vi.fn().mockRejectedValueOnce("bad payload"),
      headers: new Headers({ Origin: "http://localhost:3000" }),
      nextUrl: new URL("http://localhost:3000/api/auth/login"),
    } as unknown as NextRequest;

    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid JSON");
    expect(captureException).not.toHaveBeenCalled();
  });
});
