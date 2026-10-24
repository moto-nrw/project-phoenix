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

describe("POST /api/time-tracking/check-out", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("checks out user successfully", async () => {
    const mockCheckOutData = {
      session_id: 123,
      check_out_time: "2024-01-15T17:00:00Z",
    };
    mockApiPost.mockResolvedValueOnce({ data: mockCheckOutData });

    const request = createMockRequest("/api/time-tracking/check-out", {
      method: "POST",
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/time-tracking/check-out",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockCheckOutData>>(response);
    expect(json.data).toEqual(mockCheckOutData);
  });
});

describe("POST /api/time-tracking/check-out with F9 reason", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("forwards the deviation reason to the backend", async () => {
    mockApiPost.mockResolvedValueOnce({ data: { session_id: 123 } });

    const request = createMockRequest("/api/time-tracking/check-out", {
      method: "POST",
      body: { reason: "Elterngespräch lief länger" },
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/time-tracking/check-out",
      "test-token",
      { reason: "Elterngespräch lief länger" },
    );
    expect(response.status).toBe(200);
  });
});
