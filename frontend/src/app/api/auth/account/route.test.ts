import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  type ExtendedSession,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

const { GET } = await import("./route");

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/auth/account", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(defaultSession);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("fetches account information successfully", async () => {
    const accountData = {
      id: 1,
      email: "user@example.com",
      role: "teacher",
      staff_id: 10,
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(accountData), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/account");
    const response = await GET(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://server:8080/auth/account",
      {
        headers: {
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        },
      },
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof accountData>(response);
    expect(json.email).toBe("user@example.com");
    expect(json.role).toBe("teacher");
  });
});
