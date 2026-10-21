import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { GET, PUT } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    apiGet: mockApiGet,
    apiPut: mockApiPut,
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

describe("GET /api/students/[id]/privacy-consent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/privacy-consent");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches privacy consent for student", async () => {
    const mockConsent = {
      data: {
        id: 1,
        student_id: 123,
        policy_version: "1.0",
        accepted: true,
        accepted_at: "2024-01-01T00:00:00Z",
        duration_days: 30,
        renewal_required: false,
        data_retention_days: 30,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-01T00:00:00Z",
      },
    };
    mockApiGet.mockResolvedValueOnce(mockConsent);

    const request = createMockRequest("/api/students/123/privacy-consent");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/students/123/privacy-consent",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        student_id: number;
        accepted: boolean;
      };
    }>(response);
    expect(json.data.student_id).toBe(123);
    expect(json.data.accepted).toBe(true);
  });

  it("returns default consent when not found (404)", async () => {
    mockApiGet.mockRejectedValueOnce(new ApiResponseError(404, "Not found"));

    const request = createMockRequest("/api/students/123/privacy-consent");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      data: {
        student_id: number;
        accepted: boolean;
        data_retention_days: number;
      };
    }>(response);
    expect(json.data.student_id).toBe(123);
    expect(json.data.accepted).toBe(false);
    expect(json.data.data_retention_days).toBe(30);
  });
});

describe("PUT /api/students/[id]/privacy-consent", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {
        policy_version: "1.0",
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates privacy consent successfully", async () => {
    const mockUpdatedConsent = {
      data: {
        id: 1,
        student_id: 123,
        policy_version: "1.0",
        accepted: true,
        data_retention_days: 25,
        renewal_required: false,
        created_at: "2024-01-01T00:00:00Z",
        updated_at: "2024-01-15T10:00:00Z",
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedConsent);

    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {
        policy_version: "1.0",
        data_retention_days: 25,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/students/123/privacy-consent",
      "test-token",
      { policy_version: "1.0", data_retention_days: 25 },
    );
    expect(response.status).toBe(200);
  });

  it("validates data_retention_days range (1-31)", async () => {
    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {
        policy_version: "1.0",
        data_retention_days: 0,
      },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });

  it("requires policy_version and data_retention_days", async () => {
    const request = createMockRequest("/api/students/123/privacy-consent", {
      method: "PUT",
      body: {},
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
  });
});
