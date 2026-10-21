import { describe, it, expect, vi, beforeEach } from "vitest";
import { POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers.server", async () => {
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

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("POST /api/guardians/[id]/phone-numbers/[phoneId]/set-primary", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest(
      "/api/guardians/123/phone-numbers/456/set-primary",
      {
        method: "POST",
      },
    );
    const response = await POST(
      request,
      createMockContext({ id: "123", phoneId: "456" }),
    );

    expect(response.status).toBe(401);
  });

  it("sets phone number as primary via backend", async () => {
    const mockUpdatedPhoneNumber = {
      id: 456,
      guardian_id: 123,
      number: "+49123456789",
      is_primary: true,
    };
    mockApiPost.mockResolvedValueOnce({ data: mockUpdatedPhoneNumber });

    const request = createMockRequest(
      "/api/guardians/123/phone-numbers/456/set-primary",
      {
        method: "POST",
      },
    );
    const response = await POST(
      request,
      createMockContext({ id: "123", phoneId: "456" }),
    );

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/guardians/123/phone-numbers/456/set-primary",
      "test-token",
      {},
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedPhoneNumber>>(
        response,
      );
    expect(json.data.is_primary).toBe(true);
  });
});
