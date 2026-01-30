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

describe("GET /api/staff/by-role", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/by-role?role=teacher");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches staff by role from backend", async () => {
    const mockStaff = {
      data: [
        {
          id: 1,
          person_id: 100,
          role: "teacher",
          person: { first_name: "John", last_name: "Doe" },
        },
        {
          id: 2,
          person_id: 101,
          role: "teacher",
          person: { first_name: "Jane", last_name: "Smith" },
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/by-role?role=teacher");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/by-role?role=teacher",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(2);
  });

  it("throws error when role parameter is missing", async () => {
    const request = createMockRequest("/api/staff/by-role");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Role parameter is required");
  });

  it("fetches staff for different roles", async () => {
    const mockStaff = {
      data: [
        {
          id: 3,
          person_id: 102,
          role: "admin",
          person: { first_name: "Admin", last_name: "User" },
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/by-role?role=admin");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/by-role?role=admin",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when no staff found", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/staff/by-role?role=nonexistent");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});
