import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { PUT } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiPut } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiPut: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: vi.fn(),
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
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

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

interface ApiErrorResponse {
  error: string;
  code?: string;
}

async function parseJsonResponse<T>(response: Response): Promise<T> {
  return (await response.json()) as T;
}

describe("PUT /api/settings/system/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/settings/system/test.key", {
      method: "PUT",
      body: { value: "new-value" },
    });
    const response = await PUT(request, createMockContext({ key: "test.key" }));

    expect(response.status).toBe(401);
  });

  it("updates system setting successfully", async () => {
    const mockData = { key: "test.key", value: "new-value" };
    mockApiPut.mockResolvedValueOnce(mockData);

    const request = createMockRequest("/api/settings/system/test.key", {
      method: "PUT",
      body: { value: "new-value" },
    });
    const response = await PUT(request, createMockContext({ key: "test.key" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/system/test.key",
      "test-token",
      { value: "new-value" },
    );
    expect(response.status).toBe(200);
  });

  it("handles boolean values", async () => {
    const mockData = { key: "feature.enabled", value: true };
    mockApiPut.mockResolvedValueOnce(mockData);

    const request = createMockRequest("/api/settings/system/feature.enabled", {
      method: "PUT",
      body: { value: true },
    });
    const response = await PUT(request, createMockContext({ key: "feature.enabled" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/system/feature.enabled",
      "test-token",
      { value: true },
    );
    expect(response.status).toBe(200);
  });

  it("handles integer values", async () => {
    const mockData = { key: "max.retries", value: 5 };
    mockApiPut.mockResolvedValueOnce(mockData);

    const request = createMockRequest("/api/settings/system/max.retries", {
      method: "PUT",
      body: { value: 5 },
    });
    const response = await PUT(request, createMockContext({ key: "max.retries" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/system/max.retries",
      "test-token",
      { value: 5 },
    );
    expect(response.status).toBe(200);
  });

  it("throws error when key parameter is missing", async () => {
    const request = createMockRequest("/api/settings/system/test.key", {
      method: "PUT",
      body: { value: "new-value" },
    });
    const response = await PUT(request, createMockContext({}));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid key parameter");
  });

  it("throws error when key parameter is an array", async () => {
    const request = createMockRequest("/api/settings/system/test.key", {
      method: "PUT",
      body: { value: "new-value" },
    });
    const response = await PUT(request, createMockContext({ key: ["test", "key"] }));

    expect(response.status).toBe(500);
    const json = await parseJsonResponse<ApiErrorResponse>(response);
    expect(json.error).toContain("Invalid key parameter");
  });

  it("handles keys with dots and special characters", async () => {
    const mockData = { key: "app.config.feature-flag_v2", value: "enabled" };
    mockApiPut.mockResolvedValueOnce(mockData);

    const request = createMockRequest("/api/settings/system/app.config.feature-flag_v2", {
      method: "PUT",
      body: { value: "enabled" },
    });
    const response = await PUT(request, createMockContext({ key: "app.config.feature-flag_v2" }));

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/settings/system/app.config.feature-flag_v2",
      "test-token",
      { value: "enabled" },
    );
    expect(response.status).toBe(200);
  });
});
