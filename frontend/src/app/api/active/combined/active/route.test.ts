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

describe("GET /api/active/combined/active", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/active/combined/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches active combined groups from backend", async () => {
    const mockActiveGroups = [
      {
        id: 1,
        name: "Active Combined Group A",
        description: "Currently active",
        room_id: 5,
        started_at: "2024-01-15T09:00:00Z",
      },
      {
        id: 2,
        name: "Active Combined Group B",
        description: "Another active group",
        room_id: 10,
        started_at: "2024-01-15T10:00:00Z",
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockActiveGroups);

    const request = createMockRequest("/api/active/combined/active");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/active/combined/active",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockActiveGroups>>(response);
    expect(json.data).toEqual(mockActiveGroups);
    expect(json.data).toHaveLength(2);
  });

  it("handles case when no active combined groups exist", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/active/combined/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("filters active groups correctly", async () => {
    const mockActiveGroups = [
      {
        id: 1,
        name: "Active Group",
        started_at: "2024-01-15T09:00:00Z",
        ended_at: null,
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockActiveGroups);

    const request = createMockRequest("/api/active/combined/active");
    const response = await GET(request, createMockContext());

    const json =
      await parseJsonResponse<ApiResponse<typeof mockActiveGroups>>(response);
    expect(json.data.every((group) => group.ended_at === null)).toBe(true);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/active/combined/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("handles unauthorized access", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Unauthorized (401)"));

    const request = createMockRequest("/api/active/combined/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });
});
