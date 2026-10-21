import { describe, it, expect, vi, beforeEach } from "vitest";
import { PUT } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: mockApiPut,
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

describe("PUT /api/time-tracking/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: { status: "home_office" },
    });
    const response = await PUT(request, createMockContext({ id: "42" }));

    expect(response.status).toBe(401);
  });

  it("updates work session successfully", async () => {
    const mockUpdatedSession = {
      id: 42,
      status: "home_office",
      notes: "Working from home",
    };
    mockApiPut.mockResolvedValueOnce({ data: mockUpdatedSession });

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: {
        status: "home_office",
        notes: "Working from home",
      },
    });
    const response = await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/42",
      "test-token",
      {
        check_in_time: undefined,
        check_out_time: undefined,
        break_minutes: undefined,
        status: "home_office",
        notes: "Working from home",
      },
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedSession>>(response);
    expect(json.data).toEqual(mockUpdatedSession);
  });

  it("converts camelCase to snake_case for backend", async () => {
    mockApiPut.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: {
        checkInTime: "2024-01-15T08:00:00Z",
        checkOutTime: "2024-01-15T17:00:00Z",
        breakMinutes: 30,
      },
    });
    await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/42",
      "test-token",
      {
        check_in_time: "2024-01-15T08:00:00Z",
        check_out_time: "2024-01-15T17:00:00Z",
        break_minutes: 30,
        status: undefined,
        notes: undefined,
      },
    );
  });

  it("converts break IDs from string to int", async () => {
    mockApiPut.mockResolvedValueOnce({ data: {} });

    const request = createMockRequest("/api/time-tracking/42", {
      method: "PUT",
      body: {
        breaks: [
          { id: "1", durationMinutes: 15 },
          { id: "2", durationMinutes: 30 },
        ],
      },
    });
    await PUT(request, createMockContext({ id: "42" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/time-tracking/42",
      "test-token",
      {
        check_in_time: undefined,
        check_out_time: undefined,
        break_minutes: undefined,
        status: undefined,
        notes: undefined,
        breaks: [
          { id: 1, duration_minutes: 15 },
          { id: 2, duration_minutes: 30 },
        ],
      },
    );
  });
});
