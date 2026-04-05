import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const {
  mockOperatorAuth,
  mockUncachedOperatorAuth,
  mockFetch,
  mockGetServerApiUrl,
} = vi.hoisted(() => ({
  mockOperatorAuth: vi.fn(),
  mockUncachedOperatorAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockOperatorAuth,
  uncachedOperatorAuth: mockUncachedOperatorAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { DELETE, GET, POST } from "./route";

function createContext(id?: string, accountId?: string) {
  return { params: Promise.resolve(id && accountId ? { id, accountId } : {}) };
}

describe("/api/operator/provisioning/schools/[id]/accounts/[accountId]/caregiver-capability", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockOperatorAuth.mockResolvedValue({ user: { token: "operator-token" } });
    mockUncachedOperatorAuth.mockResolvedValue(null);
  });

  it("returns 401 when the operator is not authenticated", async () => {
    mockOperatorAuth.mockResolvedValueOnce(null);

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/operator/provisioning/schools/10/accounts/42/caregiver-capability",
      ),
      createContext("10", "42"),
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toEqual({ error: "Unauthorized" });
  });

  it("returns 400 when school or account params are invalid", async () => {
    const response = await POST(
      new NextRequest(
        "http://localhost:3000/api/operator/provisioning/schools/10/accounts/42/caregiver-capability",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ first_name: "Ada" }),
        },
      ),
      createContext("10"),
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      error: "Invalid school or account parameter",
    });
  });

  it("proxies GET requests to the operator backend endpoint", async () => {
    mockFetch.mockResolvedValue({
      status: 200,
      text: async () => JSON.stringify({ ok: true }),
      headers: new Headers({ "Content-Type": "application/json" }),
    });

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/operator/provisioning/schools/10/accounts/42/caregiver-capability",
      ),
      createContext("10", "42"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/accounts/42/caregiver-capability",
      expect.objectContaining({
        method: "GET",
        headers: {
          Authorization: "Bearer operator-token",
          "Content-Type": "application/json",
        },
      }),
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ ok: true });
  });

  it("retries DELETE requests when the operator token was refreshed", async () => {
    mockUncachedOperatorAuth.mockResolvedValue({
      user: { token: "operator-token-refreshed" },
    });
    mockFetch
      .mockResolvedValueOnce({
        status: 401,
        text: async () => JSON.stringify({ error: "expired" }),
        headers: new Headers({ "Content-Type": "application/json" }),
      })
      .mockResolvedValueOnce({
        status: 200,
        text: async () => JSON.stringify({ deleted: true }),
        headers: new Headers({ "Content-Type": "application/json" }),
      });

    const response = await DELETE(
      new NextRequest(
        "http://localhost:3000/api/operator/provisioning/schools/10/accounts/42/caregiver-capability",
        { method: "DELETE" },
      ),
      createContext("10", "42"),
    );

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch.mock.calls[1]?.[1]).toEqual(
      expect.objectContaining({
        headers: {
          Authorization: "Bearer operator-token-refreshed",
          "Content-Type": "application/json",
        },
      }),
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ deleted: true });
  });
});
