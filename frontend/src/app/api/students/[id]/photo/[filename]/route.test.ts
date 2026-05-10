// Coverage for the authenticated student photo serve proxy. Mirrors the
// patterns already in ../route.test.ts so the two photo proxy paths share a
// review surface — same hoisted mocks for auth + fetch, same swap-in of
// global.fetch, same shape of response builders.
//
// The route under test streams photo bytes back to the browser (200) or
// preserves the conditional-GET round trip (304) so cache-control,
// etag, and last-modified survive the proxy hop. Without a test the
// caching path was un-pinned: a stray `headers.delete()` or a swap of
// the validators upstream would silently regress every avatar render
// to a full re-download.

import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<(...args: unknown[]) => Promise<Response>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const originalFetch = global.fetch;
beforeEach(() => {
  vi.clearAllMocks();
  global.fetch = mockFetch as unknown as typeof fetch;
});

afterAll(() => {
  global.fetch = originalFetch;
});

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "stale-token", name: "Test User" },
  expires: "2099-01-01",
};

const refreshedSession: ExtendedSession = {
  user: { id: "1", token: "fresh-token", name: "Test User" },
  expires: "2099-01-01",
};

function createGetRequest(
  studentId: string,
  filename: string,
  headers: Record<string, string> = {},
): NextRequest {
  return new NextRequest(
    new URL(
      `/api/students/${studentId}/photo/${filename}`,
      "http://localhost:3000",
    ),
    { method: "GET", headers },
  );
}

function imageResponse(
  status: number,
  body: Uint8Array | null,
  headers: Record<string, string> = {},
): Response {
  return new Response(body ? new Blob([Uint8Array.from(body)]) : null, {
    status,
    headers,
  });
}

describe("GET /api/students/[id]/photo/[filename] — happy path", () => {
  it("streams 200 image bytes through with cache headers preserved", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    const bytes = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10]);
    mockFetch.mockResolvedValueOnce(
      imageResponse(200, bytes, {
        "content-type": "image/jpeg",
        "cache-control": "private, no-cache",
        etag: 'W/"abc123"',
        "last-modified": "Wed, 21 Oct 2026 07:28:00 GMT",
      }),
    );

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("image/jpeg");
    expect(response.headers.get("cache-control")).toBe("private, no-cache");
    expect(response.headers.get("etag")).toBe('W/"abc123"');
    expect(response.headers.get("last-modified")).toBe(
      "Wed, 21 Oct 2026 07:28:00 GMT",
    );

    // Body is streamed through — read it and confirm bytes match.
    const buffer = await response.arrayBuffer();
    expect(new Uint8Array(buffer)).toEqual(bytes);

    // Confirm the backend URL was constructed with encoded path segments.
    const calledUrl = mockFetch.mock.calls[0]?.[0] as string;
    expect(calledUrl).toBe(
      "http://backend.test/api/students/42/photo/p_abc.jpg",
    );
  });
});

describe("GET /api/students/[id]/photo/[filename] — 304 Not Modified", () => {
  it("preserves 304 verbatim with revalidation headers and no body", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(
      imageResponse(304, null, {
        "cache-control": "private, no-cache",
        etag: 'W/"abc123"',
        "last-modified": "Wed, 21 Oct 2026 07:28:00 GMT",
      }),
    );

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    expect(response.status).toBe(304);
    expect(response.headers.get("cache-control")).toBe("private, no-cache");
    expect(response.headers.get("etag")).toBe('W/"abc123"');
    expect(response.headers.get("last-modified")).toBe(
      "Wed, 21 Oct 2026 07:28:00 GMT",
    );
    // 304 must have no body — browser keeps the cached copy.
    expect(response.body).toBeNull();
  });
});

describe("GET /api/students/[id]/photo/[filename] — conditional headers", () => {
  it("forwards If-None-Match and If-Modified-Since to the backend", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(imageResponse(304, null, {}));

    const { GET } = await import("./route");
    const ifModifiedSince = "Wed, 21 Oct 2026 07:28:00 GMT";
    await GET(
      createGetRequest("42", "p_abc.jpg", {
        "if-none-match": 'W/"abc123"',
        "if-modified-since": ifModifiedSince,
      }),
      {
        params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
      },
    );

    const headers = (mockFetch.mock.calls[0]?.[1] as RequestInit | undefined)
      ?.headers as Record<string, string> | undefined;
    expect(headers?.["if-none-match"]).toBe('W/"abc123"');
    expect(headers?.["if-modified-since"]).toBe(ifModifiedSince);
    // Authorization is always there.
    expect(headers?.Authorization).toBe("Bearer stale-token");
  });

  it("does not add conditional headers when the request omits them", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(
      imageResponse(200, new Uint8Array([0xff]), {
        "content-type": "image/jpeg",
      }),
    );

    const { GET } = await import("./route");
    await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    const headers = (mockFetch.mock.calls[0]?.[1] as RequestInit | undefined)
      ?.headers as Record<string, string> | undefined;
    expect(headers).toBeDefined();
    // Only Authorization should be set.
    expect(Object.keys(headers ?? {})).toEqual(["Authorization"]);
  });
});

describe("GET /api/students/[id]/photo/[filename] — 401 retry", () => {
  it("retries with a refreshed token when the backend returns 401", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(refreshedSession);

    const bytes = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);
    mockFetch
      .mockResolvedValueOnce(imageResponse(401, null, {}))
      .mockResolvedValueOnce(
        imageResponse(200, bytes, { "content-type": "image/png" }),
      );

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.png"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.png" }),
    });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockUncachedAuth).toHaveBeenCalledTimes(1);

    const firstHeaders = (
      mockFetch.mock.calls[0]?.[1] as RequestInit | undefined
    )?.headers as Record<string, string> | undefined;
    const secondHeaders = (
      mockFetch.mock.calls[1]?.[1] as RequestInit | undefined
    )?.headers as Record<string, string> | undefined;
    expect(firstHeaders?.Authorization).toBe("Bearer stale-token");
    expect(secondHeaders?.Authorization).toBe("Bearer fresh-token");
  });

  it("does not retry when uncachedAuth returns the same token", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(imageResponse(401, null, {}));

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    // 401 falls through to the non-OK branch — body null, status preserved.
    expect(response.status).toBe(401);
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });
});

describe("GET /api/students/[id]/photo/[filename] — non-OK pass-through", () => {
  it.each([
    [403, "feature disabled"],
    [404, "no photo"],
  ])("surfaces backend %d with a null body", async (status) => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(imageResponse(status, null, {}));

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    expect(response.status).toBe(status);
    expect(response.body).toBeNull();
  });
});

describe("GET /api/students/[id]/photo/[filename] — unauthenticated", () => {
  it("returns 401 without ever hitting the backend when there is no session", async () => {
    mockAuth.mockResolvedValue(null);

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns 401 when the session has no token", async () => {
    mockAuth.mockResolvedValue({
      user: { id: "1", name: "Test" },
      expires: "2099-01-01",
    } as ExtendedSession);

    const { GET } = await import("./route");
    const response = await GET(createGetRequest("42", "p_abc.jpg"), {
      params: Promise.resolve({ id: "42", filename: "p_abc.jpg" }),
    });

    expect(response.status).toBe(401);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
