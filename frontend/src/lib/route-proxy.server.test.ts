import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import {
  proxyGet,
  proxyPost,
  proxyPut,
  proxyPatch,
  proxyDelete,
} from "./route-proxy.server";

// ============================================================================
// Types
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

// ============================================================================
// Mocks
// ============================================================================

const {
  mockAuth,
  mockApiGet,
  mockApiPost,
  mockApiPut,
  mockApiPatch,
  mockApiDelete,
} = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPost: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiPatch: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("../server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockAuth,
}));

vi.mock("./api-helpers.server", () => ({
  apiGet: mockApiGet,
  apiPost: mockApiPost,
  apiPut: mockApiPut,
  apiPatch: mockApiPatch,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)") ? 401 : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

// ============================================================================
// Helpers
// ============================================================================

function createMockRequest(
  path: string,
  options: { method?: string; body?: unknown } = {},
): NextRequest {
  const url = new URL(path, "http://localhost:3000");
  const init: { method: string; body?: string; headers?: HeadersInit } = {
    method: options.method ?? "GET",
  };
  if (options.body) {
    init.body = JSON.stringify(options.body);
    init.headers = { "Content-Type": "application/json" };
  }
  return new NextRequest(url, init);
}

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

async function parseJson<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

interface ApiResponse<T> {
  success: boolean;
  message: string;
  data: T;
}

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

beforeEach(() => {
  vi.clearAllMocks();
  mockAuth.mockResolvedValue(defaultSession);
});

// ============================================================================
// Tests
// ============================================================================

describe("proxyGet", () => {
  it("unwraps the backend data envelope by default", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [{ id: 1 }] });

    const handler = proxyGet("/api/items");
    const response = await handler(
      createMockRequest("/api/items"),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith("/api/items", "test-token");
    const json = await parseJson<ApiResponse<{ id: number }[]>>(response);
    expect(json.data).toEqual([{ id: 1 }]);
  });

  it("forwards the query string to the backend", async () => {
    mockApiGet.mockResolvedValueOnce({ data: [] });

    const handler = proxyGet("/api/items");
    await handler(
      createMockRequest("/api/items?page=2&limit=50"),
      createMockContext(),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/items?page=2&limit=50",
      "test-token",
    );
  });

  it("forwards the whole response when raw is set", async () => {
    const whole = { data: [{ id: 1 }], pagination: { total: 1 } };
    mockApiGet.mockResolvedValueOnce(whole);

    const handler = proxyGet("/api/items", { raw: true });
    const response = await handler(
      createMockRequest("/api/items"),
      createMockContext(),
    );

    const json = await parseJson<ApiResponse<typeof whole>>(response);
    expect(json.data).toEqual(whole);
  });

  it("resolves an endpoint builder against route params (id in path)", async () => {
    mockApiGet.mockResolvedValueOnce({ id: 123 });

    const handler = proxyGet((p) => `/api/items/${String(p.id)}/subitems`, {
      raw: true,
    });
    const response = await handler(
      createMockRequest("/api/items/123/subitems"),
      createMockContext({ id: "123" }),
    );

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/items/123/subitems",
      "test-token",
    );
    expect(response.status).toBe(200);
  });

  it("returns 401 when unauthenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);
    const handler = proxyGet("/api/items");
    const response = await handler(
      createMockRequest("/api/items"),
      createMockContext(),
    );
    expect(response.status).toBe(401);
  });
});

describe("proxyPost", () => {
  it("forwards the body and unwraps data by default", async () => {
    const body = { name: "New" };
    mockApiPost.mockResolvedValueOnce({ data: { id: 9 } });

    const handler = proxyPost("/api/items");
    const response = await handler(
      createMockRequest("/api/items", { method: "POST", body }),
      createMockContext(),
    );

    expect(mockApiPost).toHaveBeenCalledWith("/api/items", "test-token", body);
    const json = await parseJson<ApiResponse<{ id: number }>>(response);
    expect(json.data).toEqual({ id: 9 });
  });

  it("forwards the whole response when raw is set", async () => {
    const created = { id: 9, name: "New" };
    mockApiPost.mockResolvedValueOnce(created);

    const handler = proxyPost("/api/items", { raw: true });
    const response = await handler(
      createMockRequest("/api/items", {
        method: "POST",
        body: { name: "New" },
      }),
      createMockContext(),
    );

    const json = await parseJson<ApiResponse<typeof created>>(response);
    expect(json.data).toEqual(created);
  });
});

describe("proxyPut", () => {
  it("resolves the endpoint builder, forwards the body, and can return raw", async () => {
    const body = { name: "Updated" };
    const updated = { id: 5, name: "Updated" };
    mockApiPut.mockResolvedValueOnce(updated);

    const handler = proxyPut<unknown, typeof body>(
      (p) => `/api/items/${String(p.id)}`,
      { raw: true },
    );
    const response = await handler(
      createMockRequest("/api/items/5", { method: "PUT", body }),
      createMockContext({ id: "5" }),
    );

    expect(mockApiPut).toHaveBeenCalledWith("/api/items/5", "test-token", body);
    const json = await parseJson<ApiResponse<typeof updated>>(response);
    expect(json.data).toEqual(updated);
  });
});

describe("proxyPatch", () => {
  it("forwards the body and unwraps data", async () => {
    const body = { active: false };
    mockApiPatch.mockResolvedValueOnce({ data: { id: 3, active: false } });

    const handler = proxyPatch("/api/items/3");
    const response = await handler(
      createMockRequest("/api/items/3", { method: "PATCH", body }),
      createMockContext(),
    );

    expect(mockApiPatch).toHaveBeenCalledWith(
      "/api/items/3",
      "test-token",
      body,
    );
    const json = await parseJson<ApiResponse<{ id: number }>>(response);
    expect(json.data).toEqual({ id: 3, active: false });
  });
});

describe("proxyDelete", () => {
  it("resolves the endpoint builder and returns 204", async () => {
    mockApiDelete.mockResolvedValueOnce(undefined);

    const handler = proxyDelete((p) => `/api/items/${String(p.id)}`);
    const response = await handler(
      createMockRequest("/api/items/7", { method: "DELETE" }),
      createMockContext({ id: "7" }),
    );

    expect(mockApiDelete).toHaveBeenCalledWith("/api/items/7", "test-token");
    expect(response.status).toBe(204);
  });
});
