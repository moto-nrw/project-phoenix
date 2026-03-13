import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const {
  mockGetRefreshToken,
  mockSetOperatorTokens,
  mockFetch,
  mockGetServerApiUrl,
} = vi.hoisted(() => ({
  mockGetRefreshToken: vi.fn(),
  mockSetOperatorTokens: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/lib/operator/cookies", () => ({
  getOperatorRefreshToken: mockGetRefreshToken,
  setOperatorTokens: mockSetOperatorTokens,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

function mockRequest(): NextRequest {
  return new NextRequest("http://localhost/api/operator/refresh", {
    method: "POST",
    headers: {
      "x-forwarded-for": "203.0.113.1",
      "user-agent": "TestBrowser/1.0",
    },
  });
}

describe("POST /api/operator/refresh", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("successfully refreshes tokens and stores them", async () => {
    mockGetRefreshToken.mockResolvedValue("old-refresh-token");
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        data: {
          access_token: "new-access-token",
          refresh_token: "new-refresh-token",
        },
        message: "Token refreshed",
      }),
    });

    const response = await POST(mockRequest());

    expect(response.status).toBe(200);
    const json = (await response.json()) as { success?: boolean };
    expect(json.success).toBe(true);

    expect(mockSetOperatorTokens).toHaveBeenCalledWith(
      "new-access-token",
      "new-refresh-token",
    );
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/auth/refresh",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({
          Authorization: "Bearer old-refresh-token",
        }) as Record<string, unknown>,
      }),
    );
  });

  it("returns 401 when no refresh token exists", async () => {
    mockGetRefreshToken.mockResolvedValue(undefined);

    const response = await POST(mockRequest());

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("No refresh token");
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns error status when backend rejects refresh", async () => {
    mockGetRefreshToken.mockResolvedValue("expired-refresh-token");
    mockFetch.mockResolvedValue({
      ok: false,
      status: 401,
    });

    const response = await POST(mockRequest());

    expect(response.status).toBe(401);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Token refresh failed");
  });

  it("returns 500 on network error", async () => {
    mockGetRefreshToken.mockResolvedValue("some-refresh-token");
    mockFetch.mockRejectedValue(new Error("Network error"));

    const response = await POST(mockRequest());

    expect(response.status).toBe(500);
    const json = (await response.json()) as { error?: string };
    expect(json.error).toBe("Token refresh failed");
  });

  it("returns expiresAt from access token JWT", async () => {
    const exp = 1700000000;
    const header = Buffer.from(
      JSON.stringify({ alg: "HS256", typ: "JWT" }),
    ).toString("base64url");
    const payload = Buffer.from(JSON.stringify({ sub: "op-1", exp })).toString(
      "base64url",
    );
    const fakeJwt = `${header}.${payload}.fake-sig`;

    mockGetRefreshToken.mockResolvedValue("old-refresh-token");
    mockFetch.mockResolvedValue({
      ok: true,
      json: async () => ({
        status: "success",
        data: {
          access_token: fakeJwt,
          refresh_token: "new-refresh-token",
        },
        message: "Token refreshed",
      }),
    });

    const response = await POST(mockRequest());

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      success?: boolean;
      expiresAt?: number;
    };
    expect(json.success).toBe(true);
    expect(json.expiresAt).toBe(exp * 1000);
  });
});
