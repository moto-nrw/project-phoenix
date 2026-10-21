import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiResponseError } from "~/lib/api-response-error.server";
import { GET, PUT, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  createMockRequest,
  defaultSession,
} from "~/test/route-test-helpers";

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

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
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

describe("GET /api/rooms/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("fetches room by ID from backend (wrapped response)", async () => {
    const mockRoom = {
      id: 123,
      name: "Room A",
      building: "Main",
      floor: 2,
      capacity: 30,
      is_occupied: false,
    };
    mockApiGet.mockResolvedValueOnce({ data: mockRoom });

    const request = createMockRequest("/api/rooms/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockApiGet).toHaveBeenCalledWith("/api/rooms/123", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRoom>>(response);
    expect(json.data.id).toBe(123);
    expect(json.data.name).toBe("Room A");
  });

  it("fetches room by ID from backend (direct response)", async () => {
    const mockRoom = {
      id: 456,
      name: "Room B",
      is_occupied: true,
      student_count: 15,
    };
    mockApiGet.mockResolvedValueOnce(mockRoom);

    const request = createMockRequest("/api/rooms/456");
    const response = await GET(request, createMockContext({ id: "456" }));

    const json =
      await parseJsonResponse<ApiResponse<typeof mockRoom>>(response);
    expect(json.data.id).toBe(456);
    expect(json.data.student_count).toBe(15);
  });

  it("handles 404 room not found", async () => {
    mockApiGet.mockRejectedValueOnce(new ApiResponseError(404, "Not found"));

    const request = createMockRequest("/api/rooms/999");
    const response = await GET(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });

  // The room form renders "Offener Raum" from this field. Dropping it made a
  // released Schulhof look unreleased in the form (#3276).
  it("keeps the release and system flags (wrapped response)", async () => {
    mockApiGet.mockResolvedValueOnce({
      data: {
        id: 54,
        name: "Schulhof",
        is_occupied: false,
        is_system: true,
        is_open_room: true,
      },
    });

    const request = createMockRequest("/api/rooms/54");
    const response = await GET(request, createMockContext({ id: "54" }));

    const json =
      await parseJsonResponse<
        ApiResponse<{ is_system: boolean; is_open_room: boolean }>
      >(response);
    expect(json.data.is_system).toBe(true);
    expect(json.data.is_open_room).toBe(true);
  });

  it("keeps the release and system flags (direct response)", async () => {
    mockApiGet.mockResolvedValueOnce({
      id: 54,
      name: "Schulhof",
      is_occupied: false,
      is_system: true,
      is_open_room: false,
    });

    const request = createMockRequest("/api/rooms/54");
    const response = await GET(request, createMockContext({ id: "54" }));

    const json =
      await parseJsonResponse<
        ApiResponse<{ is_system: boolean; is_open_room: boolean }>
      >(response);
    expect(json.data.is_system).toBe(true);
    expect(json.data.is_open_room).toBe(false);
  });
});

describe("PUT /api/rooms/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: { name: "Updated Room" },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("updates room via backend", async () => {
    const updateBody = { name: "Updated Room", capacity: 40 };
    const mockUpdatedRoom = {
      id: 123,
      name: "Updated Room",
      capacity: 40,
      is_occupied: false,
    };
    mockApiPut.mockResolvedValueOnce(mockUpdatedRoom);

    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/rooms/123",
      "test-token",
      expect.objectContaining({
        name: "Updated Room",
        capacity: 40,
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedRoom>>(response);
    expect(json.data.name).toBe("Updated Room");
  });

  it("rejects update with invalid capacity", async () => {
    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: { capacity: -5 },
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain(
      "Capacity must be greater than 0",
    );
  });

  it("handles camelCase to snake_case field mapping", async () => {
    const updateBody = { deviceId: "device-123" };
    const mockUpdatedRoom = { id: 123, name: "Room", device_id: "device-123" };
    mockApiPut.mockResolvedValueOnce(mockUpdatedRoom);

    const request = createMockRequest("/api/rooms/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/rooms/123",
      "test-token",
      expect.objectContaining({
        device_id: "device-123",
      }),
    );
    expect(response.status).toBe(200);
  });

  // Regression for #3276: unticking "Offener Raum" sends is_open_room=false.
  // The proxy used to rebuild the body without it, so the backend kept the
  // standing release and the revocation was a silent no-op.
  it("forwards a release revocation (is_open_room=false)", async () => {
    mockApiPut.mockResolvedValueOnce({
      id: 54,
      name: "Schulhof",
      is_open_room: false,
    });

    const request = createMockRequest("/api/rooms/54", {
      method: "PUT",
      body: { name: "Schulhof", is_open_room: false },
    });
    const response = await PUT(request, createMockContext({ id: "54" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/rooms/54",
      "test-token",
      expect.objectContaining({ is_open_room: false }),
    );
    expect(response.status).toBe(200);
  });

  it("forwards a release (is_open_room=true)", async () => {
    mockApiPut.mockResolvedValueOnce({ id: 7, name: "Turnhalle" });

    const request = createMockRequest("/api/rooms/7", {
      method: "PUT",
      body: { name: "Turnhalle", is_open_room: true },
    });
    await PUT(request, createMockContext({ id: "7" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/rooms/7",
      "test-token",
      expect.objectContaining({ is_open_room: true }),
    );
  });

  // An unrelated edit (for example the colour) must not touch the release:
  // the backend treats an omitted field as "leave it as it is".
  it("omits is_open_room when the client did not send it", async () => {
    mockApiPut.mockResolvedValueOnce({ id: 54, name: "Schulhof" });

    const request = createMockRequest("/api/rooms/54", {
      method: "PUT",
      body: { name: "Schulhof", color: "#76BB40" },
    });
    await PUT(request, createMockContext({ id: "54" }));

    expect(mockApiPut).toHaveBeenCalledOnce();
    const forwarded = mockApiPut.mock.calls[0]?.[2] as Record<string, unknown>;
    expect(forwarded).toMatchObject({ name: "Schulhof" });
    expect(forwarded).not.toHaveProperty("is_open_room");
  });
});

describe("DELETE /api/rooms/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/rooms/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(response.status).toBe(401);
  });

  it("deletes room via backend and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/rooms/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockApiDelete).toHaveBeenCalledWith("/api/rooms/123", "test-token");
    expect(response.status).toBe(204);
  });

  it("handles 404 room not found", async () => {
    mockApiDelete.mockRejectedValueOnce(
      new ApiResponseError(404, "Room not found"),
    );

    const request = createMockRequest("/api/rooms/999", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "999" }));

    expect(response.status).toBe(404);
  });
});
