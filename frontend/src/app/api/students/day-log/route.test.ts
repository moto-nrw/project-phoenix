import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockUncachedAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/server/auth/tenant-route", () => ({
  withTenantAuth: <T>(handler: T) => handler,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: mockApiGet,
}));

const session: ExtendedSession = {
  user: { id: "1", token: "expired-token", name: "Test User" },
  expires: "2099-01-01",
};

function request(path: string): NextRequest {
  return new NextRequest(new URL(path, "http://localhost:3000"));
}

describe("GET /api/students/day-log", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(session);
  });

  it("refreshes an expired backend token and retries once", async () => {
    mockApiGet
      .mockRejectedValueOnce(new Error("API error (401): expired"))
      .mockResolvedValueOnce({ data: { groups: [] } });
    mockUncachedAuth.mockResolvedValue({
      ...session,
      user: { ...session.user, token: "refreshed-token" },
    });

    const response = await GET(
      request("/api/students/day-log?date=2026-07-24"),
    );

    expect(mockUncachedAuth).toHaveBeenCalledOnce();
    expect(mockApiGet).toHaveBeenNthCalledWith(
      1,
      "/api/students/day-log?date=2026-07-24",
      "expired-token",
    );
    expect(mockApiGet).toHaveBeenNthCalledWith(
      2,
      "/api/students/day-log?date=2026-07-24",
      "refreshed-token",
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({
      status: "success",
      data: { groups: [] },
    });
  });
});
