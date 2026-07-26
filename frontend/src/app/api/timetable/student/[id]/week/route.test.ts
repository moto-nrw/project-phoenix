import { beforeEach, describe, expect, it, vi } from "vitest";
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
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const regex = /API error[:\s(]+(\d{3})/;
    const match = error instanceof Error ? regex.exec(error.message) : null;
    const status = match?.[1] ? Number.parseInt(match[1], 10) : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

function createMockRequest(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"));
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

const mockWeek = {
  student_id: 42,
  from: "2026-07-20",
  to: "2026-07-24",
  days: [],
};

describe("GET /api/timetable/student/[id]/week", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);
    const request = createMockRequest(
      "/api/timetable/student/42/week?from=2026-07-20&to=2026-07-24",
    );
    const response = await GET(request, createMockContext({ id: "42" }));
    expect(response.status).toBe(401);
  });

  it("returns 500 when the id parameter is missing", async () => {
    const request = createMockRequest("/api/timetable/student//week");
    const response = await GET(request, createMockContext({}));
    expect(response.status).toBe(500);
    expect(mockApiGet).not.toHaveBeenCalled();
  });

  it("proxies the id and forwards the from/to range to the backend", async () => {
    mockApiGet.mockResolvedValueOnce({ data: mockWeek });
    const request = createMockRequest(
      "/api/timetable/student/42/week?from=2026-07-20&to=2026-07-24",
    );
    const response = await GET(request, createMockContext({ id: "42" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/timetable/student/42/week?from=2026-07-20&to=2026-07-24",
      "test-token",
    );
    expect(response.status).toBe(200);
    const json = await (response.json() as Promise<
      ApiResponse<typeof mockWeek>
    >);
    expect(json.data).toEqual(mockWeek);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));
    const request = createMockRequest(
      "/api/timetable/student/42/week?from=2026-07-20&to=2026-07-24",
    );
    const response = await GET(request, createMockContext({ id: "42" }));
    expect(response.status).toBe(500);
  });
});
