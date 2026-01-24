import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import { auth } from "~/server/auth";

// ============================================================================
// Mocks
// ============================================================================

const { mockApiGet } = vi.hoisted(() => ({
  mockApiGet: vi.fn(),
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
}));

// Note: auth() is globally mocked in setup.ts

// ============================================================================
// Test Helpers
// ============================================================================

const TEST_COOKIE_HEADER = "better-auth.session_token=test-session-token";

interface TestResponse {
  session?: {
    userId: string;
    email: string;
    hasSession: boolean;
  };
  backendResponse?: unknown;
  error?: string;
  details?: unknown;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/test", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 401 when not authenticated", async () => {
    // Mock auth to return null
    vi.mocked(auth).mockResolvedValueOnce(null);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as TestResponse;
    expect(json.error).toBe("No active session");
  });

  it("returns 401 when user is undefined in session", async () => {
    vi.mocked(auth).mockResolvedValueOnce({ user: undefined } as never);

    const response = await GET();

    expect(response.status).toBe(401);
    const json = (await response.json()) as TestResponse;
    expect(json.error).toBe("No active session");
  });

  it("returns session and backend response on success", async () => {
    const mockBackendResponse = {
      data: [
        { id: 1, name: "Group A" },
        { id: 2, name: "Group B" },
      ],
    };

    mockApiGet.mockResolvedValueOnce(mockBackendResponse);

    const response = await GET();

    expect(response.status).toBe(200);
    expect(mockApiGet).toHaveBeenCalledWith("/api/groups", TEST_COOKIE_HEADER);

    const json = (await response.json()) as TestResponse;
    expect(json.session).toEqual({
      userId: "test-user-id",
      email: "test@example.com",
      hasSession: true,
    });
    expect(json.backendResponse).toEqual(mockBackendResponse);
  });

  it("handles API errors with Error object", async () => {
    const error = new Error("Backend failed");
    mockApiGet.mockRejectedValueOnce(error);

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as TestResponse;
    expect(json.error).toBe("Backend failed");
  });

  it("handles API errors with response property", async () => {
    const error = Object.assign(new Error("Request failed"), {
      response: { data: { message: "Detailed error" } },
    });
    mockApiGet.mockRejectedValueOnce(error);

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as TestResponse;
    expect(json.error).toBe("Request failed");
    expect(json.details).toEqual({ message: "Detailed error" });
  });

  it("handles unknown error types", async () => {
    mockApiGet.mockRejectedValueOnce("Some string error");

    const response = await GET();

    expect(response.status).toBe(500);
    const json = (await response.json()) as TestResponse;
    expect(json.error).toBe("Unknown error");
    expect(json.details).toBe("Some string error");
  });

  it("calls correct backend endpoint", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    await GET();

    expect(mockApiGet).toHaveBeenCalledWith("/api/groups", TEST_COOKIE_HEADER);
  });
});
