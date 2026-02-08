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

import { GET, PUT, DELETE } from "./route";

describe("GET /api/operator/announcements/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches announcement by id", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const announcement = {
      id: 1,
      title: "Test Announcement",
      content: "Test content",
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: announcement }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(announcement);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements/1",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(401);
  });

  it("returns 404 for non-existent announcement", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "Not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/999",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "999" }) };
    const response = await GET(request, context);

    expect(response.status).toBe(404);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await GET(request, context);

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/operator/announcements/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates announcement successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const updateData = {
      title: "Updated Title",
      content: "Updated content",
    };
    const updatedAnnouncement = {
      id: 1,
      ...updateData,
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: updatedAnnouncement }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updateData),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(updatedAnnouncement);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements/1",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(updateData),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
      {
        method: "PUT",
        body: JSON.stringify({ title: "Test" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(401);
  });
});

describe("DELETE /api/operator/announcements/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deletes announcement successfully with 204 response", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements/1",
      expect.objectContaining({
        method: "DELETE",
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(401);
  });

  it("returns 404 for non-existent announcement", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "Not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/999",
    );
    const context: RouteContext = { params: Promise.resolve({ id: "999" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(404);
  });
});
