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

vi.mock("@/lib/api-helpers", () => ({
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

describe("GET /api/guardians/students/[studentId]/guardians", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/students/5/guardians");
    const response = await GET(request, createMockContext({ studentId: "5" }));

    expect(response.status).toBe(401);
  });

  it("fetches guardians for student from backend", async () => {
    const mockGuardians = [
      {
        id: 1,
        person_id: 100,
        first_name: "John",
        last_name: "Doe",
        relationship: "father",
      },
      {
        id: 2,
        person_id: 101,
        first_name: "Jane",
        last_name: "Doe",
        relationship: "mother",
      },
    ];
    mockApiGet.mockResolvedValueOnce({ data: mockGuardians });

    const request = createMockRequest("/api/guardians/students/5/guardians");
    const response = await GET(request, createMockContext({ studentId: "5" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/guardians/students/5/guardians",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGuardians>>(response);
    expect(json.data).toEqual(mockGuardians);
  });
});

describe("POST /api/guardians/students/[studentId]/guardians", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/students/5/guardians", {
      method: "POST",
      body: { guardian_id: 123, relationship: "father" },
    });
    const response = await POST(request, createMockContext({ studentId: "5" }));

    expect(response.status).toBe(401);
  });

  it("links guardian to student via backend", async () => {
    const linkBody = { guardian_id: 123, relationship: "father" };
    const mockRelationship = {
      id: 1,
      student_id: 5,
      guardian_id: 123,
      relationship: "father",
    };
    mockApiPost.mockResolvedValueOnce({ data: mockRelationship });

    const request = createMockRequest("/api/guardians/students/5/guardians", {
      method: "POST",
      body: linkBody,
    });
    const response = await POST(request, createMockContext({ studentId: "5" }));

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/guardians/students/5/guardians",
      "test-token",
      linkBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRelationship>>(response);
    expect(json.data).toEqual(mockRelationship);
  });
});
