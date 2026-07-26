import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiPatch: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    () => new Response(JSON.stringify({ error: "err" }), { status: 500 }),
  ),
}));

import { POST } from "./route";

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

function request(body: unknown): NextRequest {
  return new NextRequest("http://localhost:3000/api/timetable/shift-coverage", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
}

const context = { params: Promise.resolve({}) };

describe("POST /api/timetable/shift-coverage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("requires an authenticated tenant session", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await POST(request({ dates: [] }), context);

    expect(response.status).toBe(401);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("forwards the complete batched request without rewriting dates", async () => {
    const body = {
      dates: ["2026-05-04", "2026-05-06"],
      start_time: "12:00",
      end_time: "13:00",
      staff_ids: [11],
      replan_activity_group_id: 7,
      calendar_period_id: 5,
      week_pattern: 2,
    };
    mockApiPost.mockResolvedValueOnce({ data: { coverage_warnings: [] } });

    const response = await POST(request(body), context);

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/timetable/shift-coverage",
      "test-token",
      body,
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toMatchObject({
      data: { coverage_warnings: [] },
    });
  });
});
