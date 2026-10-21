import { describe, it, expect, vi, beforeEach } from "vitest";
import { POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
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

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: vi.fn(),
    apiPost: mockApiPost,
    apiPut: vi.fn(),
    apiDelete: vi.fn(),
    handleApiError: vi.fn(mockHandleApiError),
  };
});

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/activities/[id]/enroll/[studentId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("enrolls student successfully", async () => {
    mockApiPost.mockResolvedValueOnce({ success: true });

    const request = createMockRequest("/api/activities/1/enroll/100", {
      method: "POST",
      body: {},
    });
    const response = await POST(
      request,
      createMockContext({ id: "1", studentId: "100" }),
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/activities/1/students/100",
      "test-token",
      {},
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{ success: boolean }>(response);
    expect(json.success).toBe(true);
  });
});
