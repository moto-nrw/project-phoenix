import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT } from "./route";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

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

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPost: vi.fn(),
  apiPut: mockApiPut,
  apiDelete: vi.fn(),
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(404)")
        ? 404
        : message.includes("(403)")
          ? 403
          : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

vi.mock("~/lib/pin", () => ({
  validatePinOrThrow: vi.fn((pin: string) => {
    if (!/^\d{4}$/.test(pin)) {
      throw new Error("PIN muss aus genau 4 Ziffern bestehen");
    }
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

describe("GET /api/staff/pin", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/pin");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches PIN status from backend", async () => {
    const mockPINStatus = {
      status: "success",
      data: {
        has_pin: true,
        last_changed: "2024-01-15T10:00:00Z",
      },
      message: "PIN status retrieved",
    };
    mockApiGet.mockResolvedValueOnce(mockPINStatus);

    const request = createMockRequest("/api/staff/pin");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/staff/pin", "test-token");
    expect(response.status).toBe(200);

    const json =
      await parseJsonResponse<ApiResponse<{ has_pin: boolean }>>(response);
    expect(json.data.has_pin).toBe(true);
  });

  it("handles 404 error gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Not Found (404)"));

    const request = createMockRequest("/api/staff/pin");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Konto nicht gefunden");
  });

  it("handles 403 error gracefully", async () => {
    mockApiGet.mockRejectedValueOnce(new Error("Forbidden (403)"));

    const request = createMockRequest("/api/staff/pin");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Permission denied");
  });
});

describe("PUT /api/staff/pin", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/staff/pin", {
      method: "PUT",
      body: { new_pin: "1234" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("updates PIN successfully", async () => {
    const updateBody = { current_pin: "0000", new_pin: "1234" };
    const mockPINUpdateResponse = {
      status: "success",
      data: {
        success: true,
        message: "PIN updated successfully",
      },
      message: "PIN updated",
    };
    const mockPINStatus = {
      status: "success",
      data: {
        has_pin: true,
        last_changed: "2024-01-15T10:00:00Z",
      },
      message: "PIN status",
    };

    mockApiPut.mockResolvedValueOnce(mockPINUpdateResponse);
    mockApiGet.mockResolvedValueOnce(mockPINStatus);

    const request = createMockRequest("/api/staff/pin", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext());

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/staff/pin",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);
  });

  it("throws error when new PIN is missing", async () => {
    const request = createMockRequest("/api/staff/pin", {
      method: "PUT",
      body: { current_pin: "0000" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Neue PIN ist erforderlich");
  });

  it("handles incorrect current PIN error", async () => {
    mockApiPut.mockRejectedValueOnce(
      new Error("current pin is incorrect (401)"),
    );

    const request = createMockRequest("/api/staff/pin", {
      method: "PUT",
      body: { current_pin: "9999", new_pin: "1234" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Aktuelle PIN ist falsch");
  });

  it("handles account locked error", async () => {
    mockApiPut.mockRejectedValueOnce(
      new Error("Account temporarily locked due to too many attempts"),
    );

    const request = createMockRequest("/api/staff/pin", {
      method: "PUT",
      body: { current_pin: "0000", new_pin: "1234" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("vorübergehend gesperrt");
  });

  it("handles permission denied error", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Forbidden (403)"));

    const request = createMockRequest("/api/staff/pin", {
      method: "PUT",
      body: { new_pin: "1234" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toContain("Zugriff verweigert");
  });
});
