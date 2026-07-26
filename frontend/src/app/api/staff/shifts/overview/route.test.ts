import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiPatch: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    () => new Response(JSON.stringify({ error: "err" }), { status: 500 }),
  ),
}));

import { GET } from "./route";

function createMockRequest(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"));
}

function createMockContext() {
  return { params: Promise.resolve({}) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

const backendOverview = {
  from: "2026-07-06",
  to: "2026-07-10",
  dienstplan_in_use: true,
  staff: [],
  shifts: [],
  assignments: [],
};

describe("GET /api/staff/shifts/overview", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      createMockRequest("/api/staff/shifts/overview"),
      createMockContext(),
    );

    expect(response.status).toBe(401);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("forwards the date range and returns the backend overview", async () => {
    mockApiGet.mockResolvedValueOnce({ data: backendOverview });

    const response = await GET(
      createMockRequest(
        "/api/staff/shifts/overview?from=2026-07-06&to=2026-07-10",
      ),
      createMockContext(),
    );
    const json = (await response.json()) as { data: unknown };

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff-shifts/overview?from=2026-07-06&to=2026-07-10",
      "test-token",
    );
    expect(response.status).toBe(200);
    expect(json.data).toEqual(backendOverview);
  });
});
