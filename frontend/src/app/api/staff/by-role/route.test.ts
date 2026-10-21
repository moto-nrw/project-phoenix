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

describe("GET /api/staff/by-role", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches staff by role from backend", async () => {
    const mockStaff = {
      data: [
        {
          id: 1,
          person_id: 100,
          role: "teacher",
          person: { first_name: "John", last_name: "Doe" },
        },
        {
          id: 2,
          person_id: 101,
          role: "teacher",
          person: { first_name: "Jane", last_name: "Smith" },
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/by-role?role=teacher");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/by-role?role=teacher",
      "test-token",
    );
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toHaveLength(2);
  });

  it("throws error when role parameter is missing", async () => {
    const request = createMockRequest("/api/staff/by-role");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Role or roles parameter is required",
    );
  });

  it("fetches staff for different roles", async () => {
    const mockStaff = {
      data: [
        {
          id: 3,
          person_id: 102,
          role: "admin",
          person: { first_name: "Admin", last_name: "User" },
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockStaff);

    const request = createMockRequest("/api/staff/by-role?role=admin");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/staff/by-role?role=admin",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns empty array when no staff found", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const request = createMockRequest("/api/staff/by-role?role=nonexistent");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<ApiResponse<unknown[]>>(response);
    expect(json.data).toEqual([]);
  });
});
