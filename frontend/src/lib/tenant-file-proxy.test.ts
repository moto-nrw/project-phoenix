import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
  withTenantAuth: (handler: unknown) => handler,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const { proxyFile } = await import("./tenant-file-proxy.server");

function session(token: string): ExtendedSession {
  return { user: { token } } as ExtendedSession;
}

function request(
  path: string,
  init: { method?: string; body?: string } = {},
): NextRequest {
  return new NextRequest(`http://localhost:3000${path}`, {
    ...init,
    headers: { "X-Forwarded-For": "203.0.113.7" },
  });
}

function backendFile(): Response {
  return new Response(new TextEncoder().encode("file-data"), {
    headers: {
      "Content-Type": "application/pdf",
      "Content-Disposition": 'attachment; filename="report.pdf"',
      "Content-Length": "9",
      "X-Content-Type-Options": "nosniff",
      "Set-Cookie": "backend=1",
    },
  });
}

const context = { params: Promise.resolve({ id: "42" }) };
const byId = proxyFile((p) => `/api/students/${String(p.id)}/report`, {
  forwardQuery: ["format", "from"],
});

let fetchMock: ReturnType<typeof vi.fn>;

beforeEach(() => {
  vi.clearAllMocks();
  fetchMock = vi.fn();
  global.fetch = fetchMock as unknown as typeof fetch;
  mockAuth.mockResolvedValue(session("token"));
});

function sentHeaders(call = 0): Headers {
  const init = fetchMock.mock.calls[call]?.[1] as RequestInit;
  return new Headers(init.headers);
}

describe("proxyFile", () => {
  it("answers 401 without a session and never calls the backend", async () => {
    mockAuth.mockResolvedValue(null);

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("streams the file with its file headers only", async () => {
    fetchMock.mockResolvedValue(backendFile());

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("application/pdf");
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="report.pdf"',
    );
    expect(response.headers.get("Content-Length")).toBe("9");
    expect(response.headers.get("X-Content-Type-Options")).toBe("nosniff");
    expect(response.headers.get("Set-Cookie")).toBeNull();
    expect(await response.text()).toBe("file-data");
  });

  it("forwards only the listed query parameters", async () => {
    fetchMock.mockResolvedValue(backendFile());

    await byId(
      request("/api/students/42/report?format=pdf&from=2026-07-01&evil=1"),
      context,
    );

    expect(fetchMock.mock.calls[0]?.[0]).toBe(
      "http://backend:8080/api/students/42/report?format=pdf&from=2026-07-01",
    );
  });

  it("forwards the client identity so the backend rate-limits the visitor", async () => {
    fetchMock.mockResolvedValue(backendFile());

    await byId(request("/api/students/42/report"), context);

    expect(sentHeaders().get("Authorization")).toBe("Bearer token");
    expect(sentHeaders().get("X-Forwarded-For")).toBe("203.0.113.7");
  });

  it("repeats the request once with a refreshed session after a 401", async () => {
    mockUncachedAuth.mockResolvedValue(session("fresh"));
    fetchMock
      .mockResolvedValueOnce(new Response("expired", { status: 401 }))
      .mockResolvedValueOnce(backendFile());

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(sentHeaders(1).get("Authorization")).toBe("Bearer fresh");
  });

  it("forwards the 401 when the refresh brings no new token", async () => {
    mockUncachedAuth.mockResolvedValue(session("token"));
    fetchMock.mockResolvedValue(new Response("expired", { status: 401 }));

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("sends a POST body again on the retry", async () => {
    mockUncachedAuth.mockResolvedValue(session("fresh"));
    fetchMock
      .mockResolvedValueOnce(new Response("expired", { status: 401 }))
      .mockResolvedValueOnce(backendFile());
    const exportFile = proxyFile("/api/students/export", { method: "POST" });

    await exportFile(
      request("/api/students/export", {
        method: "POST",
        body: '{"format":"csv"}',
      }),
      { params: Promise.resolve({}) },
    );

    for (const call of [0, 1]) {
      const init = fetchMock.mock.calls[call]?.[1] as RequestInit;
      expect(init.method).toBe("POST");
      expect(init.body).toBe('{"format":"csv"}');
      expect(sentHeaders(call).get("Content-Type")).toBe("application/json");
    }
  });

  it("keeps a backend error's status, body and content type", async () => {
    const envelope = {
      status: 403,
      detail: "feature_disabled",
      code: "attendance.log_disabled",
    };
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify(envelope), {
        status: 403,
        headers: {
          "Content-Type": "application/problem+json",
          "Retry-After": "30",
        },
      }),
    );

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(403);
    expect(response.headers.get("Content-Type")).toBe(
      "application/problem+json",
    );
    expect(response.headers.get("Retry-After")).toBe("30");
    expect(await response.json()).toEqual(envelope);
  });

  it("answers 502 for a success without a body", async () => {
    fetchMock.mockResolvedValue(new Response(null));

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(502);
  });

  it("answers 500 for a network failure", async () => {
    fetchMock.mockRejectedValue(new Error("network failed"));

    const response = await byId(request("/api/students/42/report"), context);

    expect(response.status).toBe(500);
  });
});
