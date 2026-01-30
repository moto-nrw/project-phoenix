import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT, DELETE } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks (using vi.hoisted for proper hoisting)
// ============================================================================

const { mockAuth, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("@/lib/api-helpers", () => ({
  apiGet: vi.fn(),
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
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

describe("PUT /api/guardians/[id]/phone-numbers/[phoneId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/123/phone-numbers/456", {
      method: "PUT",
      body: { number: "+49111111111" },
    });
    const response = await PUT(
      request,
      createMockContext({ id: "123", phoneId: "456" }),
    );

    expect(response.status).toBe(401);
  });

  it("updates phone number via backend", async () => {
    const updateBody = { number: "+49111111111" };
    const mockUpdatedPhoneNumber = {
      id: 456,
      guardian_id: 123,
      number: "+49111111111",
      is_primary: false,
    };
    mockApiPut.mockResolvedValueOnce({ data: mockUpdatedPhoneNumber });

    const request = createMockRequest("/api/guardians/123/phone-numbers/456", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(
      request,
      createMockContext({ id: "123", phoneId: "456" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/guardians/123/phone-numbers/456",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<typeof mockUpdatedPhoneNumber>>(
        response,
      );
    expect(json.data.number).toBe("+49111111111");
  });
});

describe("DELETE /api/guardians/[id]/phone-numbers/[phoneId]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/guardians/123/phone-numbers/456", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "123", phoneId: "456" }),
    );

    expect(response.status).toBe(401);
  });

  it("deletes phone number via backend and returns null", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const request = createMockRequest("/api/guardians/123/phone-numbers/456", {
      method: "DELETE",
    });
    const response = await DELETE(
      request,
      createMockContext({ id: "123", phoneId: "456" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/guardians/123/phone-numbers/456",
      "test-token",
    );
    expect(response.status).toBe(204);
  });
});
