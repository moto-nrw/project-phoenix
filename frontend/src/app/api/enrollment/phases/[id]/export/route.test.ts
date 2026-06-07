import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockUncachedAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend:8080",
}));

const { POST } = await import("./route");

function session(token: string): ExtendedSession {
  return { user: { token } } as ExtendedSession;
}

function backendFile(): Response {
  const headers = new Headers({
    "content-type": "application/pdf",
    "content-disposition": 'attachment; filename="Anmeldungen.pdf"',
  });
  return {
    ok: true,
    status: 200,
    headers,
    arrayBuffer: async () => new TextEncoder().encode("PDF").buffer,
  } as unknown as Response;
}

function req(): NextRequest {
  return new NextRequest(
    "http://localhost:3000/api/enrollment/phases/5/export",
    {
      method: "POST",
      body: JSON.stringify({ format: "pdf" }),
    },
  );
}

const ctx = { params: Promise.resolve({ id: "5" }) };

describe("POST /api/enrollment/phases/[id]/export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when there is no session token", async () => {
    mockAuth.mockResolvedValue(null);
    const res = await POST(req(), ctx);
    expect(res.status).toBe(401);
  });

  it("streams the backend file through with its headers", async () => {
    mockAuth.mockResolvedValue(session("tok"));
    global.fetch = vi.fn(async () => backendFile()) as unknown as typeof fetch;

    const res = await POST(req(), ctx);

    expect(res.status).toBe(200);
    expect(res.headers.get("Content-Type")).toBe("application/pdf");
    expect(res.headers.get("Content-Disposition")).toBe(
      'attachment; filename="Anmeldungen.pdf"',
    );
    // The phase id is forwarded to the backend export endpoint.
    const calledUrl = (global.fetch as ReturnType<typeof vi.fn>).mock
      .calls[0]![0] as string;
    expect(calledUrl).toBe(
      "http://backend:8080/api/enrollment/phases/5/export",
    );
  });

  it("forwards a backend error with its status and body", async () => {
    mockAuth.mockResolvedValue(session("tok"));
    global.fetch = vi.fn(
      async () =>
        ({
          ok: false,
          status: 400,
          headers: new Headers(),
          text: async () => "phase too large",
        }) as unknown as Response,
    ) as unknown as typeof fetch;

    const res = await POST(req(), ctx);
    expect(res.status).toBe(400);
    expect(await res.json()).toEqual({ error: "phase too large" });
  });

  it("refreshes the token and retries once on a 401 from the backend", async () => {
    mockAuth.mockResolvedValue(session("stale"));
    mockUncachedAuth.mockResolvedValue(session("fresh"));
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        headers: new Headers(),
        text: async () => "expired",
      } as unknown as Response)
      .mockResolvedValueOnce(backendFile());
    global.fetch = fetchFn as unknown as typeof fetch;

    const res = await POST(req(), ctx);

    expect(res.status).toBe(200);
    expect(fetchFn).toHaveBeenCalledTimes(2);
    // Second attempt uses the refreshed token.
    const secondInit = fetchFn.mock.calls[1]![1] as RequestInit;
    expect((secondInit.headers as Record<string, string>).Authorization).toBe(
      "Bearer fresh",
    );
  });

  it("returns 500 when an unexpected error is thrown", async () => {
    mockAuth.mockRejectedValue(new Error("auth exploded"));
    const res = await POST(req(), ctx);
    expect(res.status).toBe(500);
    expect(await res.json()).toEqual({ error: "auth exploded" });
  });

  it("gives up when the refreshed token is unchanged", async () => {
    mockAuth.mockResolvedValue(session("stale"));
    mockUncachedAuth.mockResolvedValue(session("stale"));
    global.fetch = vi.fn(
      async () =>
        ({
          ok: false,
          status: 401,
          headers: new Headers(),
          text: async () => "expired",
        }) as unknown as Response,
    ) as unknown as typeof fetch;

    const res = await POST(req(), ctx);
    expect(res.status).toBe(401);
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });
});
