import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { mockSessionData } from "~/test/mocks/next-auth";

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiPost: mockApiPost,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { POST } from "./route";
import type { ExtendedSession } from "~/test/route-test-helpers";

function mockValidSession(): void {
  mockAuth.mockResolvedValue(mockSessionData() as ExtendedSession);
}

describe("POST /api/platform/announcements/[id]/seen", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("marks announcement as seen successfully", async () => {
    mockValidSession();
    mockApiPost.mockResolvedValue({ success: true });

    const request = new NextRequest(
      "http://localhost:3000/api/platform/announcements/1/seen",
      {
        method: "POST",
      },
    );
    const context = { params: Promise.resolve({ id: "1" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/platform/announcements/1/seen",
      "test-token",
    );
  });
});
