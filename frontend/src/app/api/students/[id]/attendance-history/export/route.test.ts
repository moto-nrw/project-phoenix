import { beforeEach, describe, expect, it, vi } from "vitest";
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

const { GET } = await import("./route");

function session(token: string): ExtendedSession {
  return { user: { token } } as ExtendedSession;
}

function request(query = ""): NextRequest {
  return new NextRequest(
    `http://localhost:3000/api/students/42/attendance-history/export${query}`,
  );
}

function backendFile(contentType: string, filename: string): Response {
  return new Response(new TextEncoder().encode("file-data"), {
    headers: {
      "Content-Type": contentType,
      "Content-Disposition": `attachment; filename="${filename}"`,
      "Content-Length": "9",
    },
  });
}

const context = { params: Promise.resolve({ id: "42" }) };

describe("GET /api/students/[id]/attendance-history/export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(session("token"));
  });

  it("returns 401 without a tenant session", async () => {
    mockAuth.mockResolvedValue(null);

    const response = await GET(request(), context);

    expect(response.status).toBe(401);
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it.each([
    ["pdf", "application/pdf", "anwesenheit.pdf"],
    [
      "docx",
      "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
      "anwesenheit.docx",
    ],
    [
      "xlsx",
      "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
      "anwesenheit.xlsx",
    ],
  ])(
    "streams %s with download headers",
    async (format, contentType, filename) => {
      global.fetch = vi
        .fn()
        .mockResolvedValue(backendFile(contentType, filename));

      const response = await GET(
        request(`?format=${format}&from=2026-07-01&to=2026-07-15&ignored=x`),
        context,
      );

      expect(global.fetch).toHaveBeenCalledWith(
        `http://backend:8080/api/students/42/attendance-history/export?format=${format}&from=2026-07-01&to=2026-07-15`,
        {
          headers: { Authorization: "Bearer token" },
          cache: "no-store",
        },
      );
      expect(response.status).toBe(200);
      expect(response.headers.get("Content-Type")).toBe(contentType);
      expect(response.headers.get("Content-Disposition")).toBe(
        `attachment; filename="${filename}"`,
      );
      expect(response.headers.get("Content-Length")).toBe("9");
      expect(response.headers.get("Cache-Control")).toBe("no-store");
      expect(await response.text()).toBe("file-data");
    },
  );

  it("retries once with a refreshed token after backend 401", async () => {
    mockAuth.mockResolvedValue(session("stale"));
    mockUncachedAuth.mockResolvedValue(session("fresh"));
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response("expired", { status: 401 }))
      .mockResolvedValueOnce(backendFile("application/pdf", "export.pdf"));
    global.fetch = fetchMock;

    const response = await GET(request("?format=pdf"), context);

    expect(response.status).toBe(200);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock.mock.calls[1]?.[1]).toMatchObject({
      headers: { Authorization: "Bearer fresh" },
    });
  });

  it("preserves backend errors", async () => {
    global.fetch = vi
      .fn()
      .mockResolvedValue(new Response("feature_disabled", { status: 403 }));

    const response = await GET(request("?format=pdf"), context);

    expect(response.status).toBe(403);
    expect(await response.text()).toBe("feature_disabled");
  });

  it("returns 502 for a successful response without a body", async () => {
    global.fetch = vi.fn().mockResolvedValue(new Response(null));

    const response = await GET(request("?format=pdf"), context);

    expect(response.status).toBe(502);
  });

  it("returns 500 for network failures", async () => {
    global.fetch = vi.fn().mockRejectedValue(new Error("network failed"));

    const response = await GET(request("?format=pdf"), context);

    expect(response.status).toBe(500);
    expect(await response.text()).toBe("Internal server error");
  });
});
