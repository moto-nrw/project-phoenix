import { describe, it, expect, vi, beforeEach } from "vitest";
import { POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: vi.fn(),
  apiPost: mockApiPost,
  apiPut: vi.fn(),
  apiDelete: vi.fn(),
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("POST /api/time-tracking/break/end", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/break/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("ends current break successfully", async () => {
    const mockBreakData = {
      break_id: 99,
      start_time: "2024-01-15T12:00:00Z",
      end_time: "2024-01-15T12:30:00Z",
      duration_minutes: 30,
    };
    mockApiPost.mockResolvedValueOnce({ data: mockBreakData });

    const request = createMockRequest("/api/time-tracking/break/end", {
      method: "POST",
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/time-tracking/break/end",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockBreakData>>(response);
    expect(json.data).toEqual(mockBreakData);
  });
});
