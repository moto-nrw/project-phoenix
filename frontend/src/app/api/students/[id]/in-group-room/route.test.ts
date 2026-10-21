import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

// ============================================================================
// Test Helpers
// ============================================================================

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/students/[id]/in-group-room", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches group room status from backend", async () => {
    const mockStatus = {
      data: {
        in_group_room: true,
        group_id: 5,
        room_id: 10,
      },
    };
    mockApiGet.mockResolvedValueOnce(mockStatus);

    const request = createMockRequest("/api/students/123/in-group-room");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/123/in-group-room",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        in_group_room: boolean;
        group_id: number;
      };
    }>(response);
    expect(json.data.in_group_room).toBe(true);
    expect(json.data.group_id).toBe(5);
  });

  it("throws error when student ID is missing", async () => {
    const request = createMockRequest("/api/students//in-group-room");
    const response = await GET(request, createMockContext({ id: undefined }));

    expect(response.status).toBe(500);
  });

  it("throws error when response is invalid", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/in-group-room");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});
