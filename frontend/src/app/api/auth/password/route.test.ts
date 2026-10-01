import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { ApiResponseError } from "~/lib/api-helpers.server";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

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

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

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
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Nicht authentifiziert");
  });

  it("returns 400 when currentPassword is missing", async () => {
    const request = createMockRequest("/api/auth/password", {
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Alle Passwortfelder sind erforderlich");
  });

  it("returns 400 when newPassword is missing", async () => {
    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Alle Passwortfelder sind erforderlich");
  });

  it("returns 400 when passwords do not match", async () => {
    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "New1234!",
      confirmPassword: "Different1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Die neuen Passwörter stimmen nicht überein");
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

  it("handles backend error with message field", async () => {
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
    expect(await response.json()).toEqual({
      message: "Aktuelles Passwort ist falsch",
    });
  });

  it("handles backend error with error field", async () => {
    mockApiPost.mockRejectedValueOnce(
      new ApiResponseError(
        400,
        JSON.stringify({ error: "Passwort zu schwach" }),
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
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Passwort zu schwach");
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
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Unexpected error");
  });

  it("parses serverFetchWithRetry error with JSON body", async () => {
    mockApiPost.mockRejectedValueOnce(
      new Error(
        'API error (401): {"status":"error","error":"invalid username or password"}',
      ),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Wrong1234!",
      newPassword: "New1234!",
      confirmPassword: "New1234!",
    });
    const response = await POST(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("invalid username or password");
  });

  it("parses serverFetchWithRetry error with complexity message", async () => {
    mockApiPost.mockRejectedValueOnce(
      new Error(
        'API error (400): {"status":"error","error":"password doesn\'t meet complexity requirements"}',
      ),
    );

    const request = createMockRequest("/api/auth/password", {
      currentPassword: "Old1234!",
      newPassword: "weak",
      confirmPassword: "weak",
    });
    const response = await POST(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("password doesn't meet complexity requirements");
  });

  it("forwards a non-JSON backend error body unchanged", async () => {
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
    expect(response.headers.get("Content-Type")).toBe("text/plain");
    expect(await response.text()).toBe("Internal Server Error");
  });
});
