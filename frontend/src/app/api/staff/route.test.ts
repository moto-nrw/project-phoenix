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
      : message.includes("(403)")
        ? 403
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

describe("GET /api/staff", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches staff list as array from backend", async () => {
    const mockStaff = [
      {
        id: 1,
        person_id: 10,
        staff_notes: "Test note",
        is_teacher: true,
        teacher_id: 5,
        specialization: "Math",
        role: "Teacher",
        qualifications: "PhD",
        person: {
          id: 10,
          first_name: "John",
          last_name: "Doe",
          email: "john@example.com",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
        was_present_today: true,
      },
    ];
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/staff", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: Array<{ id: string; name: string; firstName: string }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]?.id).toBe("1");
    expect(json.data[0]?.name).toBe("John Doe");
    expect(json.data[0]?.firstName).toBe("John");
  });

  it("fetches staff list with data wrapper from backend", async () => {
    const mockStaff = {
      data: [
        {
          id: 1,
          person_id: 10,
          staff_notes: "Test note",
          is_teacher: false,
          specialization: null,
          role: "Admin",
          qualifications: null,
          person: {
            id: 10,
            first_name: "Jane",
            last_name: "Smith",
            email: "jane@example.com",
            created_at: "2024-01-01T00:00:00Z",
            updated_at: "2024-01-01T00:00:00Z",
          },
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: Array<{ id: string; name: string; role: string | null }>;
    }>(response);
    expect(json.data).toHaveLength(1);
    expect(json.data[0]?.id).toBe("1");
    expect(json.data[0]?.name).toBe("Jane Smith");
    expect(json.data[0]?.role).toBe("Admin");
  });

  it("returns empty array when backend returns null", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ data: unknown[] }>(response);
    expect(json.data).toEqual([]);
  });

  it("forwards query parameters to backend", async () => {
    mockApiGet.mockResolvedValueOnce([]);

    const request = createMockRequest("/api/staff?is_teacher=true");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff?is_teacher=true",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("handles backend errors gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Backend error"));

    const request = createMockRequest("/api/staff");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ data: unknown[] }>(response);
    expect(json.data).toEqual([]);
  });
});

describe("POST /api/staff", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: { person_id: 10, is_teacher: true },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates a new staff member successfully", async () => {
    const mockCreatedStaff = {
      id: 1,
      person_id: 10,
      staff_notes: "New staff",
      is_teacher: true,
      teacher_id: 5,
      specialization: "Science",
      role: "Teacher",
      qualifications: "MSc",
      person: {
        id: 10,
        first_name: "Alice",
        last_name: "Johnson",
        email: "alice@example.com",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedStaff);

    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: {
        person_id: 10,
        is_teacher: true,
        specialization: "Science",
        role: "Teacher",
        qualifications: "MSc",
        staff_notes: "New staff",
      },
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff",
      "test-token",
      expect.objectContaining({
        person_id: 10,
        is_teacher: true,
        specialization: "Science",
      }),
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        id: string;
        name: string;
        specialization: string | null;
      };
    }>(response);
    expect(json.data.id).toBe("1");
    expect(json.data.name).toBe("Alice Johnson");
    expect(json.data.specialization).toBe("Science");
  });

  it("validates person_id is required", async () => {
    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: { is_teacher: true },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("validates person_id must be positive", async () => {
    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: { person_id: -1, is_teacher: true },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("normalizes empty strings to empty strings (not undefined)", async () => {
    const mockCreatedStaff = {
      id: 1,
      person_id: 10,
      staff_notes: "",
      is_teacher: false,
      person: {
        id: 10,
        first_name: "Bob",
        last_name: "Wilson",
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
      created_at: "2024-01-15T10:00:00Z",
      updated_at: "2024-01-15T10:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedStaff);

    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: {
        person_id: 10,
        staff_notes: "",
        specialization: "",
      },
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/staff",
      "test-token",
      expect.objectContaining({
        person_id: 10,
        staff_notes: "",
        specialization: "",
      }),
    );
    expect(response.status).toBe(200);
  });

  it("handles 403 permission errors", async () => {
    const mockError = new Error("Permission denied (403)");
    mockApiPost.mockRejectedValueOnce(mockError);

    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: { person_id: 10 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("handles 400 validation errors for person not found", async () => {
    const mockError = new Error("person not found (400)");
    mockApiPost.mockRejectedValueOnce(mockError);

    const request = createMockRequest("/api/staff", {
      method: "POST",
      body: { person_id: 999 },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
