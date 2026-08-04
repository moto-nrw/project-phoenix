import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  apiGet: vi.fn(),
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

function createContext(params: Record<string, string>) {
  return { params: Promise.resolve(params) };
}

function createRequest(): NextRequest {
  return new NextRequest(
    new URL("/api/activities/categories/7/restore", "http://localhost:3000"),
    { method: "POST" },
  );
}

describe("POST /api/activities/categories/[id]/restore", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await POST(createRequest(), createContext({ id: "7" }));

    expect(response.status).toBe(401);
    expect(mockApiPost).not.toHaveBeenCalled();
  });

  it("calls the backend restore endpoint", async () => {
    mockApiPost.mockResolvedValueOnce({
      data: { id: 7, name: "Essen", archived_at: undefined },
    });

    const response = await POST(createRequest(), createContext({ id: "7" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/categories/7/restore",
      "test-token",
    );
    expect(response.status).toBe(200);
  });
});
