import { NextRequest } from "next/server";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { createFileExportRoute } from "./export-proxy.server";

// The shared body of the export routes that answer with a file. It bypasses
// the JSON route wrapper (that one cannot stream bytes), so the auth, the
// refresh-and-retry, and the pass-through of Content-Disposition are this
// module's own responsibility — and every export route inherits whatever it
// gets wrong.

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
  getServerApiUrl: () => "http://server:8080",
}));

function session(token: string): ExtendedSession {
  return { user: { id: "1", token }, expires: "2099-01-01" };
}

function exportRequest(body: unknown = { from: "2026-07-27" }): NextRequest {
  return new NextRequest("http://localhost:3000/api/staff-shifts/export", {
    method: "POST",
    body: JSON.stringify(body),
  });
}

function fileResponse(init: { disposition?: string | null } = {}) {
  const headers = new Headers({ "content-type": "application/pdf" });
  if (init.disposition !== null) {
    headers.set(
      "content-disposition",
      init.disposition ?? 'attachment; filename="Dienstplan.pdf"',
    );
  }
  return new Response(new Uint8Array([1, 2, 3, 4]), { status: 200, headers });
}

const route = createFileExportRoute({
  backendPath: "/api/staff-shifts/export",
  component: "TestExportRoute",
});

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.mockResolvedValue(session("token-1"));
});

describe("createFileExportRoute", () => {
  it("refuses an unauthenticated request before calling the backend", async () => {
    mockAuth.mockResolvedValue(null);
    const fetchMock = vi.fn();
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const response = await route(exportRequest());

    expect(response.status).toBe(401);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("passes the rendered file and its filename through unchanged", async () => {
    const fetchMock = vi.fn().mockResolvedValue(fileResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const response = await route(exportRequest());

    expect(fetchMock).toHaveBeenCalledWith(
      "http://server:8080/api/staff-shifts/export",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Authorization: "Bearer token-1" }),
      }),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("application/pdf");
    // The filename is the backend's choice; losing it here would save every
    // export as the route's own path name.
    expect(response.headers.get("content-disposition")).toBe(
      'attachment; filename="Dienstplan.pdf"',
    );
    expect(response.headers.get("content-length")).toBe("4");
    expect((await response.arrayBuffer()).byteLength).toBe(4);
  });

  it("falls back to a binary content type when the backend names none", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        new Response(new Uint8Array([1]), { status: 200 }),
      ) as unknown as typeof fetch;

    const response = await route(exportRequest());

    expect(response.headers.get("content-type")).toBe(
      "application/octet-stream",
    );
    expect(response.headers.get("content-disposition")).toBeNull();
  });

  it("relays a backend rejection with its own status", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        new Response("range exceeds 8 weeks", { status: 400 }),
      ) as unknown as typeof fetch;

    const response = await route(exportRequest());

    expect(response.status).toBe(400);
    expect(await response.json()).toEqual({ error: "range exceeds 8 weeks" });
  });

  // A 15-minute access token expires under a planning tab left open all
  // morning; one refreshed retry keeps an export the user is allowed to run
  // from failing on the token rather than on the plan.
  it("retries once with a refreshed token after a 401", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("expired", { status: 401 }))
      .mockResolvedValueOnce(fileResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    mockUncachedAuth.mockResolvedValue(session("token-2"));

    const response = await route(exportRequest());

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    const retry = fetchMock.mock.calls[1]?.[1] as {
      headers: Record<string, string>;
    };
    expect(retry.headers.Authorization).toBe("Bearer token-2");
  });

  it("does not retry when the refresh returns the same token", async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValue(new Response("expired", { status: 401 }));
    globalThis.fetch = fetchMock as unknown as typeof fetch;
    mockUncachedAuth.mockResolvedValue(session("token-1"));

    const response = await route(exportRequest());

    expect(response.status).toBe(401);
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  it("does not retry when the refresh yields no session", async () => {
    globalThis.fetch = vi
      .fn()
      .mockResolvedValue(
        new Response("expired", { status: 401 }),
      ) as unknown as typeof fetch;
    mockUncachedAuth.mockResolvedValue(null);

    expect((await route(exportRequest())).status).toBe(401);
  });

  it("applies the body rewrite before forwarding", async () => {
    const fetchMock = vi.fn().mockResolvedValue(fileResponse());
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const normalizing = createFileExportRoute({
      backendPath: "/api/staff-shifts/export",
      component: "TestExportRoute",
      normalizeBody: (raw) => ({ ...(raw as object), variant: "aushang" }),
    });
    await normalizing(exportRequest({ from: "2026-07-27" }));

    const init = fetchMock.mock.calls[0]?.[1] as { body: string };
    expect(JSON.parse(init.body)).toEqual({
      from: "2026-07-27",
      variant: "aushang",
    });
  });

  // A body the client got wrong is a 400. Reported as a 500 it would read as
  // a broken export and send someone hunting through server logs.
  it("answers a malformed body with a 400", async () => {
    const fetchMock = vi.fn();
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const normalizing = createFileExportRoute({
      backendPath: "/api/staff-shifts/export",
      component: "TestExportRoute",
      normalizeBody: (raw) => raw,
    });
    const request = new NextRequest(
      "http://localhost:3000/api/staff-shifts/export",
      { method: "POST", body: "{not json" },
    );

    const response = await normalizing(request);

    expect(response.status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it("reports an unreachable backend as a server error", async () => {
    globalThis.fetch = vi
      .fn()
      .mockRejectedValue(
        new Error("connect ECONNREFUSED"),
      ) as unknown as typeof fetch;

    const response = await route(exportRequest());

    expect(response.status).toBe(500);
    expect(await response.json()).toEqual({ error: "connect ECONNREFUSED" });
  });
});
