import { describe, it, expect, vi, beforeEach } from "vitest";
import { GET } from "./route";
import { createMockRequest } from "~/test/route-test-helpers";

// ============================================================================
// Mocks
// ============================================================================

const mockFetch = vi.fn();

global.fetch = mockFetch;

// Mock env
vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NEXT_PUBLIC_API_URL: "http://localhost:8080",
  },
}));

// ============================================================================
// Test Helpers
// ============================================================================

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
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Missing invitation token");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("validates invitation token via backend", async () => {
    const mockInvitation = {
      id: 1,
      email: "teacher@example.com",
      status: "pending",
    };
    mockFetch.mockResolvedValueOnce(Response.json(mockInvitation));

    const request = createMockRequest("/api/invitations/validate?token=abc123");
    const response = await GET(request);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://server:8080/auth/invitations/abc123",
    );
    expect(response.status).toBe(200);
    const json = await parseJsonResponse(response);
    expect(json).toEqual(mockInvitation);
  });

  it("handles invalid token (404)", async () => {
    mockFetch.mockResolvedValueOnce(
      Response.json({ detail: "Invitation not found" }, { status: 404 }),
    );

    const request = createMockRequest(
      "/api/invitations/validate?token=invalid",
    );
    const response = await GET(request);

    expect(response.status).toBe(404);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invitation not found");
  });

  it("handles expired token (410)", async () => {
    mockFetch.mockResolvedValueOnce(
      Response.json({ detail: "Invitation expired" }, { status: 410 }),
    );

    const request = createMockRequest(
      "/api/invitations/validate?token=expired",
    );
    const response = await GET(request);

    expect(response.status).toBe(410);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Invitation expired");
  });

  it("replaces a non-JSON backend server error with a safe problem envelope", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("Internal server error", {
        status: 500,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/invitations/validate?token=test");
    const response = await GET(request);

    expect(response.status).toBe(500);
    expect(await response.json()).toMatchObject({
      status: 500,
      detail: "Internal Server Error",
      code: "general.server",
    });
  });

  it("replaces an empty backend server error with a safe problem envelope", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("", {
        status: 503,
        headers: { "Content-Type": "text/plain" },
      }),
    );

    const request = createMockRequest("/api/invitations/validate?token=test");
    const response = await GET(request);

    expect(response.status).toBe(503);
    expect(await response.json()).toMatchObject({
      status: 503,
      detail: "Service Unavailable",
      code: "general.unavailable",
    });
  });

  it("handles fetch errors", async () => {
    mockFetch.mockRejectedValueOnce(new Error("Network error"));

    const request = createMockRequest("/api/invitations/validate?token=test");
    const response = await GET(request);

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<{ detail: string }>(response);
    expect(json.detail).toBe("Internal Server Error");
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
      "http://server:8080/auth/invitations/abc%20def%2Fghi%3D",
    );
  });
});
