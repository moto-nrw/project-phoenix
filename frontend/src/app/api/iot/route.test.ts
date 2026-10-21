import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET, POST } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPost } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers.server", async (importOriginal) => {
  const actual = await importOriginal<Record<string, unknown>>();
  return {
    ...actual,
    apiGet: mockApiGet,
    apiPost: mockApiPost,
  };
});

vi.mock("@/lib/iot-helpers", () => ({
  mapDeviceResponse: vi.fn(
    (device: {
      id: number;
      device_id: string;
      description: string;
      is_active: boolean;
      created_at: string;
      updated_at: string;
    }) => ({
      id: String(device.id),
      deviceId: device.device_id,
      description: device.description,
      isActive: device.is_active,
      createdAt: new Date(device.created_at),
      updatedAt: new Date(device.updated_at),
    }),
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

describe("GET /api/iot", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches devices from backend and returns paginated response", async () => {
    const mockDevices = {
      status: "success",
      data: [
        {
          id: 1,
          device_id: "DEVICE001",
          description: "Test Device",
          is_active: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      ],
    };
    mockApiGet.mockResolvedValueOnce(mockDevices);

    const request = createMockRequest("/api/iot");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/iot", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<
      ApiResponse<{
        data: unknown[];
        pagination: { total_records: number };
      }>
    >(response);
    expect(json.data.data).toHaveLength(1);
    expect(json.data.pagination.total_records).toBe(1);
  });

  it("returns empty paginated response when API returns null", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/iot");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<
      ApiResponse<{
        data: unknown[];
        pagination: { total_records: number };
      }>
    >(response);
    expect(json.data.data).toEqual([]);
    expect(json.data.pagination.total_records).toBe(0);
  });

  it("returns empty paginated response when response has unexpected structure", async () => {
    mockApiGet.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/iot");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = await parseJsonResponse<
      ApiResponse<{
        data: unknown[];
        pagination: { total_records: number };
      }>
    >(response);
    expect(json.data.data).toEqual([]);
  });
});

describe("POST /api/iot", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("creates a new device", async () => {
    const createBody = {
      device_id: "DEVICE002",
      description: "New Device",
      is_active: true,
    };
    const mockCreatedDevice = {
      status: "success",
      data: {
        id: 2,
        device_id: "DEVICE002",
        description: "New Device",
        is_active: true,
        created_at: "2024-01-02T00:00:00Z",
        updated_at: "2024-01-02T00:00:00Z",
      },
    };
    mockApiPost.mockResolvedValueOnce(mockCreatedDevice);

    const request = createMockRequest("/api/iot", {
      method: "POST",
      body: createBody,
    });
    const response = await POST(request, createMockContext());

    expect(mockApiPost).toHaveBeenCalledWith(
      "/api/iot",
      "test-token",
      createBody,
    );
    expect(response.status).toBe(200);
  });

  it("throws error when response format is invalid", async () => {
    mockApiPost.mockResolvedValueOnce({ unexpected: "structure" });

    const request = createMockRequest("/api/iot", {
      method: "POST",
      body: { device_id: "DEVICE003" },
    });
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid response format");
  });
});
