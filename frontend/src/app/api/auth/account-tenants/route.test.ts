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

describe("GET /api/auth/account-tenants", () => {
  const originalFetch = global.fetch;

  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = vi.fn();
    mockAuth.mockResolvedValue(defaultSession);
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("proxies tenant list from backend", async () => {
    const backendData = {
      data: [
        { tenant_id: 1, slug: "school-a", name: "School A" },
        { tenant_id: 2, slug: "school-b", name: "School B" },
      ],
    };

    vi.mocked(global.fetch).mockResolvedValueOnce(
      new Response(JSON.stringify(backendData), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/auth/account-tenants");
    const response = await GET(request);

    expect(global.fetch).toHaveBeenCalledWith(
      "http://server:8080/auth/account/tenants",
      {
        headers: {
          "Content-Type": "application/json",
          Authorization: "Bearer test-token",
        },
      },
    );

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<typeof backendData>(response);
    expect(json.data).toHaveLength(2);
  });
});
