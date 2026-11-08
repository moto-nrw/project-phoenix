import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<typeof fetch>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

vi.stubGlobal("fetch", mockFetch);

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

describe("GET /api/feedback/student/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 400 when student ID is missing from path", async () => {
    const request = createMockRequest("/api/feedback/other/path");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invalid id parameter");
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
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify(backendResponse), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const request = createMockRequest("/api/feedback/student/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/feedback/student/123",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
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
});
