import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockGetOperatorToken, mockFetch, mockGetServerApiUrl } = vi.hoisted(
  () => ({
    mockGetOperatorToken: vi.fn<() => Promise<string | undefined>>(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }),
);

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorToken: mockGetOperatorToken,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET, POST } from "./route";

const mockContext: RouteContext = { params: Promise.resolve({}) };

describe("GET /api/operator/announcements", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches announcements with default include_inactive=true", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const announcements = [
      { id: 1, title: "Test 1", content: "Content 1" },
      { id: 2, title: "Test 2", content: "Content 2" },
    ];

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: announcements }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(announcements);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements?include_inactive=true",
      expect.any(Object),
    );
  });

  it("respects include_inactive query parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: [] }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements?include_inactive=false",
    );
    await GET(request, mockContext);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements?include_inactive=false",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements",
    );
    const response = await GET(request, mockContext);

    expect(response.status).toBe(401);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.error).toBe("Unauthorized");
  });
});

describe("POST /api/operator/announcements", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates announcement successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const newAnnouncement = {
      title: "New Announcement",
      content: "Test content",
      priority: "high",
    };
    const createdAnnouncement = {
      id: 1,
      ...newAnnouncement,
      created_at: "2024-01-01T00:00:00Z",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ status: "success", data: createdAnnouncement }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newAnnouncement),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(createdAnnouncement);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(newAnnouncement),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements",
      {
        method: "POST",
        body: JSON.stringify({ title: "Test" }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(401);
  });

  it("handles validation errors from backend", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () => JSON.stringify({ error: "Title is required" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content: "Missing title" }),
      },
    );
    const response = await POST(request, mockContext);

    expect(response.status).toBe(400);
  });
});
