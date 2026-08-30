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

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return { ...actual, apiGet: mockApiGet };
});

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

describe("GET /api/groups/context", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups/context");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns groups for current user", async () => {
    const mockGroups = [
      {
        id: "1",
        name: "Group A",
        via_substitution: false,
        is_personal: true,
      },
      {
        id: "2",
        name: "Group B",
        via_substitution: false,
        is_personal: false,
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockGroups });

    const request = createMockRequest("/api/groups/context");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/ogs-group-navigation",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ groups: typeof mockGroups }>>(
        response,
      );
    expect(json.data.groups).toEqual(mockGroups);
  });

  it("returns empty array when user has no groups", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/groups/context");
    const response = await GET(request, createMockContext());

    const json =
      await parseJsonResponse<ApiResponse<{ groups: unknown[] }>>(response);
    expect(json.data.groups).toEqual([]);
  });

  it("returns an error when the API call fails", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/groups/context");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
