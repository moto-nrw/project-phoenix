import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockFetch, mockGetServerApiUrl, mockUncachedAuth } =
  vi.hoisted(() => ({
    mockAuth: vi.fn(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
    mockUncachedAuth: vi.fn(),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), warn: vi.fn() }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

describe("POST /api/enrollment/admin/reports/class-roster/export", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
    mockUncachedAuth.mockResolvedValue({ user: { token: "refresh-token" } });
  });

  it("proxies class roster exports and preserves download headers", async () => {
    mockFetch.mockResolvedValue(
      new Response("xlsx-bytes", {
        status: 200,
        headers: {
          "Content-Type":
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          "Content-Disposition": 'attachment; filename="klassenliste.xlsx"',
        },
      }),
    );
    const body = {
      format: "xlsx",
      filters: { phase_id: "42", school_class: "1a" },
    };
    const request = new NextRequest(
      "http://localhost:3000/api/enrollment/admin/reports/class-roster/export",
      {
        method: "POST",
        body: JSON.stringify(body),
      },
    );

    const response = await POST(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/enrollment/admin/reports/class-roster/export",
      expect.objectContaining({
        method: "POST",
        headers: {
          Authorization: "Bearer access-token",
          "Content-Type": "application/json",
        },
        body: JSON.stringify(body),
        cache: "no-store",
      }),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Disposition")).toBe(
      'attachment; filename="klassenliste.xlsx"',
    );
    expect(await response.text()).toBe("xlsx-bytes");
  });
});
