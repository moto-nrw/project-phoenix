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
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(403)")
        ? 403
        : 500;
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("GET /api/active/supervisors/all", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/supervisors/all");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns all active supervisions", async () => {
    const mockGroups = [
      { id: 1, name: "Group A", room_id: 10, room: { id: 10, name: "Room A" } },
      { id: 2, name: "Group B", room_id: 20, room: { id: 20, name: "Room B" } },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockGroups });

    const request = createMockRequest("/api/active/supervisors/all");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/active/supervisors/all",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGroups>>(response);
    expect(json.data).toEqual(mockGroups);
    expect(json.data).toHaveLength(2);
  });

  it("handles backend 403 error", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error (403): Forbidden"));

    const request = createMockRequest("/api/active/supervisors/all");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(403);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/active/supervisors/all");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
