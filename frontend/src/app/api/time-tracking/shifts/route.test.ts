import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

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
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    () => new Response(JSON.stringify({ error: "err" }), { status: 500 }),
  ),
}));

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

describe("GET /api/time-tracking/shifts", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      createMockRequest("/api/time-tracking/shifts"),
      createMockContext(),
    );
    expect(response.status).toBe(401);
  });

  it("forwards the date range to the backend", async () => {
    const shifts = [
      {
        id: 1,
        staff_id: 7,
        date: "2026-07-06",
        start_time: "08:00",
        end_time: "16:00",
        break_minutes: 30,
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: shifts });

    const response = await GET(
      createMockRequest(
        "/api/time-tracking/shifts?from=2026-07-06&to=2026-07-10",
      ),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/time-tracking/shifts?from=2026-07-06&to=2026-07-10",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: typeof shifts };
    expect(json.data).toEqual(shifts);
  });
});
