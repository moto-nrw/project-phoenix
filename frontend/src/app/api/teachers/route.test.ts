import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { POST } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
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

// Mock global fetch
global.fetch = mockFetch;

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

function createMockFetchResponse(data: unknown, ok = true, status = 200) {
  return Promise.resolve({
    ok,
    status,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  } as Response);
}

describe("POST /api/teachers", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/teachers", {
      method: "POST",
      body: {
        first_name: "John",
        last_name: "Doe",
        email: "john.doe@example.com",
        password: "SecurePass123!",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates teacher with account successfully", async () => {
    const requestBody = {
      first_name: "Jane",
      last_name: "Smith",
      email: "jane.smith@example.com",
      password: "SecurePass123!",
      specialization: "Mathematics",
      role: "Lead Teacher",
      qualifications: "PhD in Education",
      tag_id: "RFID123",
      staff_notes: "Experienced educator",
    };

    const mockAccountResponse = {
      data: {
        id: "100",
        email: "jane.smith@example.com",
        username: "jane.smith",
        name: "Jane Smith",
      },
    };

    const mockPersonResponse = {
      data: {
        id: 200,
        first_name: "Jane",
        last_name: "Smith",
        account_id: 100,
        tag_id: "RFID123",
        created_at: "2024-01-20T10:00:00Z",
        updated_at: "2024-01-20T10:00:00Z",
      },
    };

    const mockStaffResponse = {
      data: {
        id: 300,
        person_id: 200,
        staff_notes: "Experienced educator",
        is_teacher: true,
        teacher_id: 400,
        specialization: "Mathematics",
        role: "Lead Teacher",
        qualifications: "PhD in Education",
        created_at: "2024-01-20T10:00:00Z",
        updated_at: "2024-01-20T10:00:00Z",
      },
    };

    mockFetch
      .mockImplementationOnce(() =>
        createMockFetchResponse(mockAccountResponse),
      )
      .mockImplementationOnce(() => createMockFetchResponse(mockPersonResponse))
      .mockImplementationOnce(() => createMockFetchResponse(mockStaffResponse));

    const request = createMockRequest("/api/teachers", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledTimes(3);
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: { id: string; name: string; email: string };
    }>(response);
    expect(json.success).toBe(true);
    expect(json.data.id).toBe("300");
    expect(json.data.name).toBe("Jane Smith");
    expect(json.data.email).toBe("jane.smith@example.com");
  });

  it("trims empty optional fields before sending", async () => {
    const requestBody = {
      first_name: "Bob",
      last_name: "Jones",
      email: "bob.jones@example.com",
      password: "Pass123!",
      specialization: "   ",
      role: "",
      qualifications: null,
      staff_notes: "   Notes   ",
    };

    const mockAccountResponse = {
      data: {
        id: "101",
        email: "bob@example.com",
        username: "bob",
        name: "Bob",
      },
    };
    const mockPersonResponse = {
      data: { id: 201, first_name: "Bob", last_name: "Jones", account_id: 101 },
    };
    const mockStaffResponse = {
      data: {
        id: 301,
        person_id: 201,
        staff_notes: "Notes",
        is_teacher: true,
      },
    };

    mockFetch
      .mockImplementationOnce(() =>
        createMockFetchResponse(mockAccountResponse),
      )
      .mockImplementationOnce(() => createMockFetchResponse(mockPersonResponse))
      .mockImplementationOnce(() => createMockFetchResponse(mockStaffResponse));

    const request = createMockRequest("/api/teachers", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);

    // Verify staff creation call excluded empty fields
    const staffCall = mockFetch.mock.calls[2] as
      | [string, { body: string }]
      | undefined;
    const staffBody = JSON.parse(staffCall?.[1]?.body ?? "{}") as Record<
      string,
      unknown
    >;
    expect(staffBody.specialization).toBeUndefined();
    expect(staffBody.role).toBeUndefined();
    expect(staffBody.qualifications).toBeUndefined();
    expect(staffBody.staff_notes).toBe("Notes");
  });

  it("handles account creation failure", async () => {
    mockFetch.mockImplementationOnce(() =>
      createMockFetchResponse({ error: "Email already exists" }, false, 400),
    );

    const request = createMockRequest("/api/teachers", {
      method: "POST",
      body: {
        first_name: "Duplicate",
        last_name: "User",
        email: "exists@example.com",
        password: "Pass123!",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Account creation failed");
  });

  it("handles person creation failure", async () => {
    const mockAccountResponse = {
      data: {
        id: "102",
        email: "test@example.com",
        username: "test",
        name: "Test",
      },
    };

    mockFetch
      .mockImplementationOnce(() =>
        createMockFetchResponse(mockAccountResponse),
      )
      .mockImplementationOnce(() =>
        createMockFetchResponse({ error: "Person creation error" }, false, 500),
      );

    const request = createMockRequest("/api/teachers", {
      method: "POST",
      body: {
        first_name: "Error",
        last_name: "Test",
        email: "error@example.com",
        password: "Pass123!",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Person creation failed");
  });

  it("handles staff creation failure", async () => {
    const mockAccountResponse = {
      data: {
        id: "103",
        email: "test@example.com",
        username: "test",
        name: "Test",
      },
    };
    const mockPersonResponse = {
      data: { id: 203, first_name: "Test", last_name: "User", account_id: 103 },
    };

    mockFetch
      .mockImplementationOnce(() =>
        createMockFetchResponse(mockAccountResponse),
      )
      .mockImplementationOnce(() => createMockFetchResponse(mockPersonResponse))
      .mockImplementationOnce(() =>
        createMockFetchResponse({ error: "Staff creation error" }, false, 500),
      );

    const request = createMockRequest("/api/teachers", {
      method: "POST",
      body: {
        first_name: "Staff",
        last_name: "Error",
        email: "staff@example.com",
        password: "Pass123!",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Staff creation failed");
  });
});
