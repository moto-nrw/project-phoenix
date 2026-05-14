import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockAuth,
}));

vi.mock("../server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiPost: mockApiPost,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { POST } from "./route";

describe("POST /api/active/visits/transit/assign", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "access-token" } });
  });

  it("forwards assignments to the backend and unwraps data", async () => {
    mockApiPost.mockResolvedValue({
      data: {
        assigned: [42],
        skipped: [],
        active_group_id: 99,
        room_id: 77,
      },
    });
    const body = { student_ids: [42], active_group_id: 99 };
    const request = new NextRequest(
      "http://localhost:3000/api/active/visits/transit/assign",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      },
    );

    const response = await POST(request, { params: Promise.resolve({}) });

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/active/visits/transit/assign",
      "access-token",
      body,
    );
    await expect(response.json()).resolves.toEqual({
      success: true,
      data: {
        assigned: [42],
        skipped: [],
        active_group_id: 99,
        room_id: 77,
      },
      message: "Success",
    });
  });
});
