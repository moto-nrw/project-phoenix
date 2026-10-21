import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { ApiResponseError } from "~/lib/api-helpers.server";
import {
  type ExtendedSession,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiPost: mockApiPost,
  apiGet: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
}));

const { POST } = await import("./route");

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string, body?: unknown): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: "POST",
    };

  if (body) {
    requestInit.body = JSON.stringify(body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/auth/password", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Unauthenticated");
  });

  it("returns 400 when currentPassword is missing", async () => {
    const request = createMockRequest("/api/auth/password", {
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("all password fields are required");
  });

  it("returns 400 when newPassword is missing", async () => {
    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("all password fields are required");
  });

  it("returns 400 when passwords do not match", async () => {
    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "New1234!",
      confirmPassword: "Different1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("new passwords do not match");
  });

  it("successfully changes password", async () => {
    mockApiPost.mockResolvedValueOnce({ success: true });

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(mockApiPost).toHaveBeenCalledWith("/auth/password", "test-token", {
      current_password: "Old1234!",
      new_password: "New1234!",
      confirm_password: "New1234!",
    });

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });

  it("replaces a backend error with a legacy message field", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(
        400,
        JSON.stringify({ message: "Aktuelles Passwort ist falsch" }),
        { contentType: "application/json" },
      ),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Wrong1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    expect(await response.json()).toMatchObject({
      status: 400,
      detail: "Bad Request",
      code: "general.input",
    });
  });

  it("replaces an incomplete backend error body", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(
        400,
        JSON.stringify({ detail: "Passwort zu schwach" }),
        { contentType: "application/json" },
      ),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "weak",
      confirmPassword: "weak",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Bad Request");
  });

  it("returns 500 on unexpected error", async () => {
    mockApiPost.mockRejectedValueOnce(new Error("Unexpected error"));

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Unexpected error");
  });

  it("parses serverFetchWithRetry error with JSON body", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(
        401,
        JSON.stringify({
          type: "about:blank",
          title: "Unauthorized",
          status: 401,
          detail: "invalid username or password",
          code: "general.permission",
        }),
      ),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Wrong1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("invalid username or password");
  });

  it("parses serverFetchWithRetry error with complexity message", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(
        400,
        JSON.stringify({
          type: "about:blank",
          title: "Bad Request",
          status: 400,
          detail: "password doesn't meet complexity requirements",
          code: "general.input",
        }),
      ),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "weak",
      confirmPassword: "weak",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("password doesn't meet complexity requirements");
  });

  it("wraps a non-JSON backend error body in the error envelope", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(500, "Internal Server Error", {
        contentType: "text/plain",
      }),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(500);
    expect(response.headers.get("Content-Type")).toBe(
      "application/problem+json",
    );
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
  });
});
