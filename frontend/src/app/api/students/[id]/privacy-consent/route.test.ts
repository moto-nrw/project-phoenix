import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-client", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
}));

vi.mock("~/lib/api-helpers", () => ({
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

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/students/[id]/privacy-consent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/privacy-consent");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches privacy consent for student", async () => {
    const mockConsent = {
      data: {
        data: {
          id: 1,
          student_id: 123,
          policy_version: "1.0",
          accepted: true,
          accepted_at: "2024-01-01T00:00:00Z",
          duration_days: 30,
          renewal_required: false,
          data_retention_days: 30,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      },
    };
    mockApiGet.mockResolvedValueOnce(mockConsent);

    const request = createMockRequest("/api/students/123/privacy-consent");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/123/privacy-consent",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        student_id: number;
        accepted: boolean;
      };
    }>(response);
    expect(json.data.student_id).toBe(123);
    expect(json.data.accepted).toBe(true);
  });

  it("returns default consent when not found (404)", async () => {
    const mockError = Object.assign(new Error("Not Found"), {
      response: { status: 404 },
    });
    mockApiGet.mockRejectedValueOnce(mockError);

    const request = createMockRequest("/api/students/123/privacy-consent");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        student_id: number;
        accepted: boolean;
        data_retention_days: number;
      };
    }>(response);
    expect(json.data.student_id).toBe(123);
    expect(json.data.accepted).toBe(false);
    expect(json.data.data_retention_days).toBe(30);
  });
});

describe("PUT /api/students/[id]/privacy-consent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {
        policy_version: "1.0",
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates privacy consent successfully", async () => {
    const mockUpdatedConsent = {
      data: {
        data: {
          id: 1,
          student_id: 123,
          policy_version: "1.0",
          accepted: true,
          data_retention_days: 25,
          renewal_required: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-15T10:00:00Z",
        },
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedConsent);

    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {
        policy_version: "1.0",
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/123/privacy-consent",
      { policy_version: "1.0", data_retention_days: 25 },
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("validates data_retention_days range (1-31)", async () => {
    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {
        policy_version: "1.0",
        data_retention_days: 0,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });

  it("requires policy_version and data_retention_days", async () => {
    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {},
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});
