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

vi.mock("~/lib/api-helpers.server", async () => {
  const { mockHandleApiError } =
    await import("~/test/helpers/handle-api-error");
  return {
    apiGet: mockApiGet,
    apiPost: vi.fn(),
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

describe("GET /api/groups/[id]/students/room-status", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups/123/students/room-status");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("returns 500 when group ID is missing", async () => {
    const request = createMockRequest("/api/groups//students/room-status");
    const response = await GET(request, createMockContext({}));

    expect(response.status).toBe(500);
  });

  it("fetches room status for students in group", async () => {
    const mockRoomStatus = {
      group_has_room: true,
      student_room_status: {
        1: { in_room: true, room_name: "Room A" },
        2: { in_room: false },
      },
    };
    mockApiGet.mockResolvedValueOnce({ data: mockRoomStatus });

    const request = createMockRequest("/api/groups/123/students/room-status");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/groups/123/students/room-status",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRoomStatus>>(response);
    expect(json.data).toEqual(mockRoomStatus);
  });

  it("handles invalid response format", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/groups/123/students/room-status");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});
