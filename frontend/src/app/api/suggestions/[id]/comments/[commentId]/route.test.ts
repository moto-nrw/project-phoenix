import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { Session } from "next-auth";
import { mockSessionData } from "~/test/mocks/next-auth";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({ auth: mockAuth }));
vi.mock("~/lib/api-helpers", () => ({
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { DELETE } from "./route";

function mockValidSession(): void {
  mockAuth.mockResolvedValue(mockSessionData() as ExtendedSession);
}

describe("DELETE /api/suggestions/[id]/comments/[commentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deletes comment successfully", async () => {
    mockValidSession();
    mockApiDelete.mockResolvedValue({ success: true });

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments/42",
    );
    const context = { params: Promise.resolve({ id: "1", commentId: "42" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/suggestions/1/comments/42",
      "test-token",
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments/42",
    );
    const context = { params: Promise.resolve({ id: "1", commentId: "42" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockValidSession();

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments/42",
    );
    const context = {
      params: Promise.resolve({
        id: 123 as unknown as string,
        commentId: "42",
      }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(500);
  });

  it("handles invalid commentId parameter", async () => {
    mockValidSession();

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments/42",
    );
    const context = {
      params: Promise.resolve({ id: "1", commentId: 42 as unknown as string }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(500);
  });

  it("handles non-existent comment", async () => {
    mockValidSession();
    mockApiDelete.mockRejectedValue(new Error("Comment not found"));

    const request = new NextRequest(
      "http://localhost:3000/api/suggestions/1/comments/999",
    );
    const context = { params: Promise.resolve({ id: "1", commentId: "999" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(500);
  });
});
