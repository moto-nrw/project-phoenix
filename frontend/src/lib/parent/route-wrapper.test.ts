import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const {
  mockFetch,
  mockGetServerApiUrl,
  mockParentAuth,
  mockOperatorAuth,
  mockTenantAuth,
} = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockParentAuth: vi.fn(),
  mockOperatorAuth: vi.fn(),
  mockTenantAuth: vi.fn(),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("~/server/auth/parent", () => ({
  parentAuth: mockParentAuth,
  uncachedParentAuth: mockParentAuth,
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockOperatorAuth,
  uncachedOperatorAuth: mockOperatorAuth,
}));

vi.mock("~/server/auth", () => ({
  auth: mockTenantAuth,
  uncachedAuth: mockTenantAuth,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { proxyGet, proxyPost, proxyPut } from "./route-wrapper.server";

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

const defaultSession = { user: { token: "parent-token" } };

function createRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: options.method ?? "GET",
  };
  if (options.body !== undefined) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(`http://localhost:3000${path}`, init);
}

function createContext(
  params: Record<string, string | string[] | undefined> = {},
): RouteContext {
  return { params: Promise.resolve(params) };
}

async function parseJson<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("parent proxy factories", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParentAuth.mockResolvedValue(defaultSession);
  });

  it("uses parent auth, forwards query strings, and wraps already-unwrapped GET data once", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: [{ id: 1, name: "Child" }],
      }),
    });

    const handler = proxyGet("/parent/me/children");
    const response = await handler(
      createRequest("/api/parent/me/children?include=guardians"),
      createContext(),
    );

    expect(mockParentAuth).toHaveBeenCalledTimes(1);
    expect(mockOperatorAuth).not.toHaveBeenCalled();
    expect(mockTenantAuth).not.toHaveBeenCalled();
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/parent/me/children?include=guardians",
      expect.objectContaining({
        method: "GET",
        headers: expect.objectContaining({
          Authorization: "Bearer parent-token",
        }) as Record<string, unknown>,
      }),
    );
    await expect(parseJson<ApiResponse<unknown[]>>(response)).resolves.toEqual({
      success: true,
      message: "Success",
      data: [{ id: 1, name: "Child" }],
    });
  });

  it("forwards POST bodies through the parent proxy", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { request_id: "req-1" },
      }),
    });

    const handler = proxyPost("/parent/enrollments/test/submit");
    const response = await handler(
      createRequest("/api/parent/enrollments/test/submit", {
        method: "POST",
        body: { child_name: "Ada" },
      }),
      createContext(),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/parent/enrollments/test/submit",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ child_name: "Ada" }),
      }),
    );
    await expect(
      parseJson<ApiResponse<{ request_id: string }>>(response),
    ).resolves.toMatchObject({
      data: { request_id: "req-1" },
    });
  });

  it("resolves params and forwards PUT bodies through the parent proxy", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { updated: true },
      }),
    });

    const handler = proxyPut(
      (params) => `/parent/me/children/${String(params.studentId)}/notes`,
    );
    const response = await handler(
      createRequest("/api/parent/me/children/7/notes", {
        method: "PUT",
        body: { note: "Bring jacket" },
      }),
      createContext({ studentId: "7" }),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/parent/me/children/7/notes",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify({ note: "Bring jacket" }),
      }),
    );
    await expect(response.json()).resolves.toMatchObject({
      data: { updated: true },
    });
  });

  it("returns 401 when no parent session is present", async () => {
    mockParentAuth.mockResolvedValue(null);

    const handler = proxyGet("/parent/me/children");
    const response = await handler(
      createRequest("/api/parent/me/children"),
      createContext(),
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toEqual({ error: "Unauthorized" });
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("retries proxy requests with a refreshed parent token after backend 401", async () => {
    mockParentAuth
      .mockResolvedValueOnce({ user: { token: "old-token" } })
      .mockResolvedValueOnce({ user: { token: "new-token" } });
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        text: async () => "Unauthorized",
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: { refreshed: true },
        }),
      });

    const handler = proxyGet("/parent/me/profile");
    const response = await handler(
      createRequest("/api/parent/me/profile"),
      createContext(),
    );

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8080/parent/me/profile",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer old-token",
        }) as Record<string, unknown>,
      }),
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/parent/me/profile",
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer new-token",
        }) as Record<string, unknown>,
      }),
    );
    await expect(response.json()).resolves.toMatchObject({
      data: { refreshed: true },
    });
  });
});
