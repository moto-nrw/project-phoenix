import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return { ...actual, apiGet: mockApiGet };
});

const { GET } = await import("./route");

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
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

describe("GET /api/feedback/student/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(401);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Unauthorized");
  });

  it("returns 401 when session has no token", async () => {
    mockAuth.mockResolvedValueOnce({
      user: { id: "1", name: "Test User" },
      expires: "2099-01-01",
    } as ExtendedSession);

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(401);
  });

  it("returns 400 when student ID is missing from path", async () => {
    const request = createMockRequest("/api/feedback/other/path");
    const response = await GET(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invalid id parameter");
  });

  it("returns feedback data on success", async () => {
    const backendResponse = {
      data: [
        {
          id: 10,
          value: "positive",
          day: "2026-04-01",
          time: "15:30:23",
          student_id: 123,
          is_mensa_feedback: false,
          created_at: "2026-04-01T15:30:23Z",
          updated_at: "2026-04-01T15:30:23Z",
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(backendResponse);

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(200);
    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/feedback/student/123",
      "test-token",
    );

    const json = await parseJsonResponse<{
      status: string;
      data: unknown[];
    }>(response);
    expect(json.status).toBe("success");
    expect(json.data).toHaveLength(1);

    // Verify Cache-Control header
    expect(response.headers.get("Cache-Control")).toBe("no-store");
  });

  it("returns 403 when backend returns feature_disabled", async () => {
    mockApiGet.mockRejectedValueOnce(
      new Error("API error (403): feature_disabled"),
    );

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(403);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("feature_disabled");
  });

  it("returns 404 when backend returns not found", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error (404): not found"));

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("not_found");
  });

  it("returns 500 on generic backend error", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Network failure"));

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Network failure");
  });

  it("extracts status code from API error message", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("API error (502): Bad Gateway"));

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(502);
  });

  it("handles non-Error thrown values", async () => {
    mockApiGet.mockRejectedValueOnce("string error");

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("string error");
  });
});
