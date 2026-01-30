import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT, PATCH, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
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

vi.mock("~/lib/student-privacy-helpers", () => ({
  fetchPrivacyConsent: vi.fn(() =>
    Promise.resolve({
      privacy_consent_accepted: true,
      data_retention_days: 30,
    }),
  ),
  updatePrivacyConsent: vi.fn(() => Promise.resolve()),
}));

vi.mock("~/lib/student-helpers", () => ({
  mapStudentResponse: vi.fn(
    (data: {
      id?: number;
      first_name?: string;
      last_name?: string;
      school_class?: string;
      guardian_name?: string;
      guardian_contact?: string;
    }) => ({
      id: data.id?.toString() ?? "1",
      first_name: data.first_name ?? "Test",
      last_name: data.last_name ?? "Student",
      school_class: data.school_class ?? "1a",
      guardian_name: data.guardian_name ?? "Guardian",
      guardian_contact: data.guardian_contact ?? "contact@example.com",
    }),
  ),
  prepareStudentForBackend: vi.fn((data: Record<string, unknown>) => data),
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

describe("GET /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches student by ID with privacy consent", async () => {
    const mockStudent = {
      data: {
        id: 123,
        person_id: 10,
        first_name: "Alice",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        has_full_access: true,
        group_supervisors: [],
      },
    };
    mockApiGet.mockResolvedValueOnce(mockStudent);

    const request = createMockRequest("/api/students/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/students/123", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        id: string;
        has_full_access: boolean;
      };
    }>(response);
    expect(json.data.id).toBe("123");
    expect(json.data.has_full_access).toBe(true);
  });

  it("throws error when student ID is missing", async () => {
    const request = createMockRequest("/api/students/");
    const response = await GET(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: { first_name: "Updated" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates student successfully", async () => {
    const mockUpdatedStudent = {
      data: {
        id: 123,
        person_id: 10,
        first_name: "Updated",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedStudent);

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: { first_name: "Updated" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/123",
      "test-token",
      expect.any(Object),
    );
    expect(response.status).toBe(200);
  });

  it("updates privacy consent when provided", async () => {
    const { updatePrivacyConsent } =
      await import("~/lib/student-privacy-helpers");

    const mockUpdatedStudent = {
      data: {
        id: 123,
        person_id: 10,
        first_name: "Alice",
        last_name: "Smith",
        school_class: "1a",
        guardian_name: "Jane Smith",
        guardian_contact: "jane@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedStudent);

    const request = createMockRequest("/api/students/123", {
      method: "PUT",
      body: {
        first_name: "Alice",
        privacy_consent_accepted: true,
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(updatePrivacyConsent).toHaveBeenCalled();
    expect(response.status).toBe(200);
  });
});

describe("PATCH /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("is an alias for PUT handler", async () => {
    expect(PATCH).toBe(PUT);
  });
});

describe("DELETE /api/students/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes student successfully", async () => {
    mockApiDelete.mockResolvedValueOnce({
      message: "Student deleted successfully",
    });

    const request = createMockRequest("/api/students/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/students/123",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
    }>(response);
    expect(json.success).toBe(true);
  });
});
