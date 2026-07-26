import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), warn: vi.fn() }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

describe("GET /api/enrollment/admin/reports/care-usage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
    mockFetch.mockResolvedValue(
      new Response(JSON.stringify({ data: { rows: [] } }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
  });

  it("forwards weekday and pickup_time filters to the backend", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/enrollment/admin/reports/care-usage?phase_id=42&weekday=mon&pickup_time=14%3A30&search=Ada",
    );

    await GET(request);

    const [url, init] = mockFetch.mock.calls[0] ?? [];
    expect(url?.toString()).toBe(
      "http://localhost:8080/api/enrollment/admin/reports/care-usage?phase_id=42&weekday=mon&pickup_time=14%3A30&search=Ada",
    );
    expect(init).toEqual({
      headers: { Authorization: "Bearer access-token" },
      cache: "no-store",
    });
  });
});
