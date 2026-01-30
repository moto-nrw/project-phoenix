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

describe("GET /api/substitutions/active", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/substitutions/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches active substitutions without date parameter", async () => {
    const mockSubstitutions = [
      {
        id: 1,
        group_id: 5,
        substitute_id: 10,
        start_date: "2024-01-15",
        end_date: "2024-01-15",
      },
      {
        id: 2,
        group_id: 6,
        substitute_id: 11,
        start_date: "2024-01-15",
        end_date: "2024-01-15",
      },
    ];
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockSubstitutions,
      message: "Active substitutions retrieved",
    });

    const request = createMockRequest("/api/substitutions/active");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/substitutions/active",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockSubstitutions>>(response);
    expect(json.data).toEqual(mockSubstitutions);
  });

  it("fetches active substitutions with date parameter", async () => {
    const mockSubstitutions = [
      {
        id: 1,
        group_id: 5,
        substitute_id: 10,
        start_date: "2024-01-20",
        end_date: "2024-01-20",
      },
    ];
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: mockSubstitutions,
      message: "Active substitutions retrieved",
    });

    const request = createMockRequest(
      "/api/substitutions/active?date=2024-01-20",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/substitutions/active?date=2024-01-20",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockSubstitutions>>(response);
    expect(json.data).toEqual(mockSubstitutions);
  });

  it("returns empty array when data is null", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      data: null,
      message: "No active substitutions",
    });

    const request = createMockRequest("/api/substitutions/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns empty array when response has no data property", async () => {
    mockApiGet.mockResolvedValueOnce({
      status: "success",
      message: "No active substitutions",
    });

    const request = createMockRequest("/api/substitutions/active");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});
