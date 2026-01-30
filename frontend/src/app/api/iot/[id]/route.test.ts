import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-client", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
}));

vi.mock("@/lib/route-wrapper", async () => {
  const actual = await vi.importActual("@/lib/route-wrapper");
  return actual;
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

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const requestInit: { method: string; body?: string; headers?: HeadersInit } =
    {
      method: options.method ?? "GET",
    };

  if (options.body) {
    requestInit.body = JSON.stringify(options.body);
    requestInit.headers = { "Content-Type": "application/json" };
  }

  return new NextRequest(url, requestInit);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

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

describe("GET /api/iot/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/iot/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("fetches device by ID from backend", async () => {
    const mockDevice = {
      data: {
        status: "success",
        data: {
          id: 1,
          device_id: "DEVICE001",
          description: "Test Device",
          is_active: true,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-01T00:00:00Z",
        },
      },
    };
    mockApiGet.mockResolvedValueOnce(mockDevice);

    const request = createMockRequest("/api/iot/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/iot/1", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ id: string; deviceId: string }>>(
        response,
      );
    expect(json.data.id).toBe("1");
  });

  it("throws error when device not found", async () => {
    mockApiGet.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/iot/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Device not found");
  });

  it("throws error when response format is invalid", async () => {
    mockApiGet.mockResolvedValueOnce({ data: { unexpected: "structure" } });

    const request = createMockRequest("/api/iot/1");
    const response = await GET(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid response format");
  });
});

describe("PUT /api/iot/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/iot/1", {
      method: "PUT",
      body: { description: "Updated Device" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("updates device via backend", async () => {
    const updateBody = { description: "Updated Device", is_active: false };
    const mockUpdatedDevice = {
      data: {
        status: "success",
        data: {
          id: 1,
          device_id: "DEVICE001",
          description: "Updated Device",
          is_active: false,
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-15T10:00:00Z",
        },
      },
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedDevice);

    const request = createMockRequest("/api/iot/1", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/iot/1",
      updateBody,
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("throws error when device update fails", async () => {
    mockApiPut.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/iot/1", {
      method: "PUT",
      body: { description: "Updated" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Failed to update device");
  });

  it("throws error when response format is invalid", async () => {
    mockApiPut.mockResolvedValueOnce({ data: { unexpected: "structure" } });

    const request = createMockRequest("/api/iot/1", {
      method: "PUT",
      body: { description: "Updated" },
    });
    const response = await PUT(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Invalid response format");
  });
});

describe("DELETE /api/iot/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/iot/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(response.status).toBe(401);
  });

  it("deletes device via backend and returns success", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/iot/1", { method: "DELETE" });
    const response = await DELETE(request, createMockContext({ id: "1" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/iot/1", "test-token");
    expect(response.status).toBe(200);

    const json = await parseJsonResponse<{
      success: boolean;
      message: string;
    }>(response);
    expect(json.success).toBe(true);
    expect(json.message).toContain("deleted successfully");
  });
});
