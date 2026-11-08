import { describe, it, expect, vi, beforeEach } from "vitest";
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

const { mockAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<typeof fetch>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
  }),
);

global.fetch = mockFetch;

function jsonResponse(data: unknown, status = 200): Response {
  return new Response(JSON.stringify(data), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

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

describe("GET /api/guardians/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("fetches guardian by ID from backend", async () => {
    const mockGuardian = {
      id: 123,
      person_id: 100,
      first_name: "John",
      last_name: "Doe",
    };
    mockFetch.mockResolvedValueOnce(jsonResponse({ data: mockGuardian }));

    const request = createMockRequest("/api/guardians/123");
    const response = await GET(request, createMockContext({ id: "123" }));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/guardians/123",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockGuardian>>(response);
    expect(json.data).toEqual(mockGuardian);
  });
});

describe("PUT /api/guardians/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("updates guardian via backend", async () => {
    const updateBody = { first_name: "Jane" };
    const mockUpdatedGuardian = {
      id: 123,
      person_id: 100,
      first_name: "Jane",
      last_name: "Doe",
    };
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ data: mockUpdatedGuardian }),
    );

    const request = createMockRequest("/api/guardians/123", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext({ id: "123" }));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/guardians/123",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(updateBody),
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
          "Content-Type": "application/json",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedGuardian>>(
        response,
      );
    expect(json.data.first_name).toBe("Jane");
  });
});

describe("DELETE /api/guardians/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("deletes guardian via backend and returns null", async () => {
    mockFetch.mockResolvedValueOnce(new Response(null, { status: 204 }));

    const request = createMockRequest("/api/guardians/123", {
      method: "DELETE",
    });
    const response = await DELETE(request, createMockContext({ id: "123" }));

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/api/guardians/123",
      expect.objectContaining({
        method: "DELETE",
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }) as Record<string, string>,
      }),
    );
    expect(response.status).toBe(204);
  });
});
