import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, POST } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
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

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
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

describe("GET /api/substitutions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/substitutions");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches substitutions from backend", async () => {
    const mockSubstitutions = {
      status: "success",
      data: [
        {
          id: 1,
          group_id: 10,
          substitute_staff_id: 20,
          original_staff_id: 30,
          date: "2024-01-15",
          notes: "Test substitution",
        },
        {
          id: 2,
          group_id: 11,
          substitute_staff_id: 21,
          original_staff_id: 31,
          date: "2024-01-16",
          notes: null,
        },
      ],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 1,
        total_records: 2,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockSubstitutions);

    const request = createMockRequest("/api/substitutions");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/substitutions", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(2);
  });

  it("fetches substitutions with query parameters", async () => {
    const mockSubstitutions = {
      status: "success",
      data: [],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockSubstitutions);

    const request = createMockRequest(
      "/api/substitutions?date=2024-01-15&group_id=10",
    );
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/substitutions?date=2024-01-15&group_id=10",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when no substitutions found", async () => {
    const mockSubstitutions = {
      status: "success",
      data: [],
      pagination: {
        current_page: 1,
        page_size: 50,
        total_pages: 0,
        total_records: 0,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockSubstitutions);

    const request = createMockRequest("/api/substitutions");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });

  it("returns empty array when data is null", async () => {
    mockApiGet.mockResolvedValueOnce({ data: null });

    const request = createMockRequest("/api/substitutions");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/substitutions", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/substitutions", {
      method: "POST",
      body: {
        group_id: 10,
        substitute_staff_id: 20,
        original_staff_id: 30,
        date: "2024-01-15",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new substitution", async () => {
    const createBody = {
      group_id: 10,
      substitute_staff_id: 20,
      original_staff_id: 30,
      date: "2024-01-15",
      notes: "Emergency substitution",
    };
    const mockCreatedSubstitution = {
      id: 3,
      group_id: 10,
      substitute_staff_id: 20,
      original_staff_id: 30,
      date: "2024-01-15",
      notes: "Emergency substitution",
      created_at: "2024-01-10T10:00:00Z",
      updated_at: "2024-01-10T10:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedSubstitution);

    const request = createMockRequest("/api/substitutions", {
      method: "POST",
      body: createBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/substitutions",
      "test-token",
      createBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: number; group_id: number }>>(
        response,
      );
    expect(json.data.id).toBe(3);
    expect(json.data.group_id).toBe(10);
  });

  it("creates substitution without optional notes", async () => {
    const createBody = {
      group_id: 11,
      substitute_staff_id: 21,
      original_staff_id: 31,
      date: "2024-01-16",
    };
    const mockCreatedSubstitution = {
      id: 4,
      group_id: 11,
      substitute_staff_id: 21,
      original_staff_id: 31,
      date: "2024-01-16",
      notes: null,
      created_at: "2024-01-10T11:00:00Z",
      updated_at: "2024-01-10T11:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedSubstitution);

    const request = createMockRequest("/api/substitutions", {
      method: "POST",
      body: createBody,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
  });
});
