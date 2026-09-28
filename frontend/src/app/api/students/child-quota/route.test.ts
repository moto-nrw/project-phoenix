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
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

describe("GET /api/students/child-quota", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({
      user: { id: "1", token: "test-token", name: "Test User" },
      expires: "2099-01-01",
    });
  });

  it("passes the school's Kinderkontingent through with the caller's token", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: { limited: true, booked_places: 50, occupied_places: 48 },
    });

    const response = await GET(
      new NextRequest(new URL("/api/students/child-quota", "http://localhost")),
      { params: Promise.resolve({}) },
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/child-quota",
      "test-token",
    );
    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      data: { limited: true, booked_places: 50, occupied_places: 48 },
    });
  });

  it("refuses without a session", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      new NextRequest(new URL("/api/students/child-quota", "http://localhost")),
      { params: Promise.resolve({}) },
    );

    expect(response.status).toBe(401);
    expect(mockApiGet).not.toHaveBeenCalled();
  });
});
