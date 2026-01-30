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

describe("POST /api/persons", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/persons", {
      method: "POST",
      body: {
        first_name: "John",
        last_name: "Doe",
      },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("creates person successfully", async () => {
    const requestBody = {
      first_name: "Alice",
      last_name: "Johnson",
      date_of_birth: "2010-05-15",
      tag_id: "TAG123",
    };

    const mockBackendResponse = {
      id: 500,
      first_name: "Alice",
      last_name: "Johnson",
      date_of_birth: "2010-05-15",
      tag_id: "TAG123",
      created_at: "2024-01-20T10:00:00Z",
      updated_at: "2024-01-20T10:00:00Z",
    };

    mockFetch.mockImplementationOnce(() =>
      createMockFetchResponse(mockBackendResponse, true, 201),
    );

    const request = createMockRequest("/api/persons", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/persons",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          "Content-Type": "application/json",
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
        body: JSON.stringify(requestBody),
      }),
    );

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
      data: { id: number; first_name: string };
    }>(response);
    expect(json.success).toBe(true);
    expect(json.message).toBe("Person created successfully");
    expect(json.data).toEqual(mockBackendResponse);
  });

  it("handles backend error gracefully", async () => {
    const requestBody = {
      first_name: "Bob",
      last_name: "Smith",
    };

    mockFetch.mockImplementationOnce(() =>
      createMockFetchResponse(
        { error: "Validation failed: first_name too short" },
        false,
        400,
      ),
    );

    const request = createMockRequest("/api/persons", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
      data: null;
    }>(response);
    expect(json.success).toBe(false);
    expect(json.message).toContain("Validation failed");
    expect(json.data).toBeNull();
  });

  it("forwards arbitrary body structure to backend", async () => {
    const requestBody = {
      first_name: "Carol",
      last_name: "White",
      custom_field: "custom_value",
      nested: { data: "test" },
    };

    const mockBackendResponse = {
      id: 501,
      first_name: "Carol",
      last_name: "White",
    };

    mockFetch.mockImplementationOnce(() =>
      createMockFetchResponse(mockBackendResponse),
    );

    const request = createMockRequest("/api/persons", {
      method: "POST",
      body: requestBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockFetch).toHaveBeenCalledWith(
      expect.any(String),
      expect.objectContaining({
        body: JSON.stringify(requestBody),
      }),
    );

    expect(response.status).toBe(200);
  });

  it("handles network errors", async () => {
    mockFetch.mockImplementationOnce(() =>
      Promise.reject(new Error("Network error")),
    );

    const request = createMockRequest("/api/persons", {
      method: "POST",
      body: { first_name: "Error", last_name: "Test" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("handles empty response body", async () => {
    mockFetch.mockImplementationOnce(() =>
      createMockFetchResponse(null, false, 500),
    );

    const request = createMockRequest("/api/persons", {
      method: "POST",
      body: { first_name: "Empty", last_name: "Response" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      data: null;
    }>(response);
    expect(json.success).toBe(false);
    expect(json.data).toBeNull();
  });
});
