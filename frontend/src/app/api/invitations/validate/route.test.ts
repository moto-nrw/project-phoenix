import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { GET } from "./route";

// ============================================================================
// Mocks
// ============================================================================

const mockFetch = vi.fn();

global.fetch = mockFetch;

// Mock env
vi.mock("~/env", () => ({
  env: {
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// ============================================================================
// Test Helpers
// ============================================================================

function createMockRequest(path: string): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  return new NextRequest(url);
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/invitations/validate", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 400 when token is missing", async () => {
    const request = createMockRequest("/api/invitations/validate");
    const response = await GET(request);

    expect(response.status).toBe(400);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Missing invitation token");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("validates invitation token via backend", async () => {
    const mockInvitation = {
      id: 1,
      email: "teacher@example.com",
      status: "pending",
    };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: new Headers({ "Content-Type": "application/json" }),
      json: async () => mockInvitation,
    });

    const request = createMockRequest("/api/invitations/validate?token=abc123");
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/invitations/abc123",
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse(response);
    expect(json).toEqual(mockInvitation);
  });

  it("handles invalid token (404)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      headers: new Headers({ "Content-Type": "application/json" }),
      json: async () => ({ error: "Invitation not found" }),
    });

    const request = createMockRequest(
      "/api/invitations/validate?token=invalid",
    );
    const response = await GET(request);

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invitation not found");
  });

  it("handles expired token (410)", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 410,
      headers: new Headers({ "Content-Type": "application/json" }),
      json: async () => ({ error: "Invitation expired" }),
    });

    const request = createMockRequest(
      "/api/invitations/validate?token=expired",
    );
    const response = await GET(request);

    expect(response.status).toBe(410);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Invitation expired");
  });

  it("handles non-JSON response from backend", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      headers: new Headers({ "Content-Type": "text/plain" }),
      text: async () => "Internal server error",
      json: async () => {
        throw new Error("Not JSON");
      },
    });

    const request = createMockRequest("/api/invitations/validate?token=test");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal server error");
  });

  it("handles empty non-JSON response", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 503,
      headers: new Headers({ "Content-Type": "text/plain" }),
      text: async () => "",
      json: async () => {
        throw new Error("Not JSON");
      },
    });

    const request = createMockRequest("/api/invitations/validate?token=test");
    const response = await GET(request);

    expect(response.status).toBe(503);
    const json = await parseJsonResponse(response);
    expect(json).toEqual({});
  });

  it("handles fetch errors", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/invitations/validate?token=test");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ error: string }>(response);
    expect(json.error).toBe("Internal Server Error");
  });

  it("encodes special characters in token", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      headers: new Headers({ "Content-Type": "application/json" }),
      json: async () => ({ id: 1 }),
    });

    const request = createMockRequest(
      "/api/invitations/validate?token=abc+def/ghi=",
    );
    await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/invitations/abc%20def%2Fghi%3D",
    );
  });
});
