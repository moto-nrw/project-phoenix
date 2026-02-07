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

import { POST } from "./route";

describe("POST /api/operator/announcements/[id]/publish", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("publishes announcement successfully", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    const publishedAnnouncement = {
      id: 1,
      title: "Test",
      published: true,
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: publishedAnnouncement }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/publish",
      {
        method: "POST",
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
      status?: string;
    };
    expect(json.data).toEqual(publishedAnnouncement);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/announcements/1/publish",
      expect.objectContaining({
        method: "POST",
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockGetOperatorToken.mockResolvedValue(undefined);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/publish",
      {
        method: "POST",
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/1/publish",
      {
        method: "POST",
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });

  it("returns 404 for non-existent announcement", async () => {
    mockGetOperatorToken.mockResolvedValue("valid-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "Not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/announcements/999/publish",
      {
        method: "POST",
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "999" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(404);
  });
});
