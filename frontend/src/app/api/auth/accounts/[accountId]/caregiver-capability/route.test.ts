import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockAuth, mockUncachedAuth, mockFetch, mockGetServerApiUrl } =
  vi.hoisted(() => ({
    mockAuth: vi.fn(),
    mockUncachedAuth: vi.fn(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  }));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { DELETE, GET, POST } from "./route";

function createContext(accountId?: string) {
  return { params: Promise.resolve(accountId ? { accountId } : {}) };
}

describe("/api/auth/accounts/[accountId]/caregiver-capability", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "session-token" } });
    mockUncachedAuth.mockResolvedValue(null);
  });

  it("returns 401 when the user is not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/auth/accounts/42/caregiver-capability",
      ),
      createContext("42"),
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toEqual({ error: "Unauthorized" });
  });

  it("returns 400 when the account id is missing", async () => {
    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/auth/accounts/caregiver-capability",
      ),
      createContext(),
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      error: "Invalid accountId parameter",
    });
  });

  it("proxies successful GET responses with the backend content type", async () => {
    mockFetch.mockResolvedValue({
      status: 200,
      text: async () => JSON.stringify({ ok: true }),
      headers: new Headers({ "Content-Type": "application/json" }),
    });

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/auth/accounts/42/caregiver-capability",
      ),
      createContext("42"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/accounts/42/caregiver-capability",
      expect.objectContaining({
        method: "GET",
        headers: {
          Authorization: "Bearer session-token",
          "Content-Type": "application/json",
        },
        cache: "no-store",
      }),
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("application/json");
    await expect(response.json()).resolves.toEqual({ ok: true });
  });

  it("retries POST requests with a refreshed token after a 401", async () => {
    mockUncachedAuth.mockResolvedValue({ user: { token: "refreshed-token" } });
    mockFetch
      .mockResolvedValueOnce({
        status: 401,
        text: async () => JSON.stringify({ error: "expired" }),
        headers: new Headers({ "Content-Type": "application/json" }),
      })
      .mockResolvedValueOnce({
        status: 200,
        text: async () => JSON.stringify({ success: true }),
        headers: new Headers({ "Content-Type": "application/json" }),
      });

    const response = await POST(
      new NextRequest(
        "http://localhost:3000/api/auth/accounts/42/caregiver-capability",
        {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ first_name: "Ada" }),
        },
      ),
      createContext("42"),
    );

    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch.mock.calls[1]?.[1]).toEqual(
      expect.objectContaining({
        headers: {
          Authorization: "Bearer refreshed-token",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ first_name: "Ada" }),
      }),
    );
    expect(response.status).toBe(200);
    await expect(response.json()).resolves.toEqual({ success: true });
  });

  it("proxies DELETE requests without a request body", async () => {
    mockFetch.mockResolvedValue({
      status: 204,
      text: async () => "",
      headers: new Headers(),
    });

    const response = await DELETE(
      new NextRequest(
        "http://localhost:3000/api/auth/accounts/42/caregiver-capability",
        {
          method: "DELETE",
        },
      ),
      createContext("42"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/auth/accounts/42/caregiver-capability",
      expect.objectContaining({
        method: "DELETE",
        body: undefined,
      }),
    );
    expect(response.status).toBe(204);
    await expect(response.text()).resolves.toBe("");
  });
});
