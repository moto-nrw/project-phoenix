import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
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

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/rooms/by-category", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/by-category");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches rooms grouped by category", async () => {
    const mockRoomsByCategory = {
      classroom: [
        { id: 1, name: "Room A", category: "classroom" },
        { id: 2, name: "Room B", category: "classroom" },
      ],
      lab: [{ id: 3, name: "Lab 1", category: "lab" }],
      office: [{ id: 4, name: "Office 1", category: "office" }],
    };
    mockApiGet.mockResolvedValueOnce(mockRoomsByCategory);

    const request = createMockRequest("/api/rooms/by-category");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/rooms/by-category",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRoomsByCategory>>(
        response,
      );
    expect(json.data).toEqual(mockRoomsByCategory);
    expect(json.data.classroom).toHaveLength(2);
    expect(json.data.lab).toHaveLength(1);
  });

  it("returns empty object when no rooms", async () => {
    mockApiGet.mockResolvedValueOnce({});

    const request = createMockRequest("/api/rooms/by-category");
    const response = await GET(request, createMockContext());

    const json =
      await parseJsonResponse<ApiResponse<Record<string, unknown[]>>>(response);
    expect(json.data).toEqual({});
  });

  it("handles API errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/rooms/by-category");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
