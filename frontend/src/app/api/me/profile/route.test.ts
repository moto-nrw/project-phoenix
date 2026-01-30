import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET, PUT } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

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

describe("GET /api/me/profile", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/me/profile");
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("fetches current user profile from backend", async () => {
    const mockProfile = {
      id: 1,
      first_name: "John",
      second_name: "Doe",
      email: "john@example.com",
      phone: "+123456789",
      avatar_url: "/avatars/1.jpg",
    };
    mockApiGet.mockResolvedValueOnce({ data: mockProfile });

    const request = createMockRequest("/api/me/profile");
    const response = await GET(request, createMockContext());

    expect(mockApiGet).toHaveBeenCalledWith("/api/me/profile", "test-token");
    expect(response.status).toBe(200);

    const json = (await response.json()) as { data: unknown };
    expect(json.data).toEqual(mockProfile);
  });
});

describe("PUT /api/me/profile", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = createMockRequest("/api/me/profile", {
      method: "PUT",
      body: { first_name: "Jane" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("updates user profile via backend", async () => {
    const updateBody = {
      first_name: "Jane",
      second_name: "Smith",
      phone: "+987654321",
    };
    const mockUpdatedProfile = {
      id: 1,
      first_name: "Jane",
      second_name: "Smith",
      email: "jane@example.com",
      phone: "+987654321",
    };
    mockApiPut.mockResolvedValueOnce({ data: mockUpdatedProfile });

    const request = createMockRequest("/api/me/profile", {
      method: "PUT",
      body: updateBody,
    });
    const response = await PUT(request, createMockContext());

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/me/profile",
      "test-token",
      updateBody,
    );
    expect(response.status).toBe(200);

    const json = (await response.json()) as {
      data: { first_name: string; phone: string };
    };
    expect(json.data.first_name).toBe("Jane");
    expect(json.data.phone).toBe("+987654321");
  });

  it("handles validation errors from backend", async () => {
    mockApiPut.mockRejectedValueOnce(new Error("Backend error (500)"));

    const request = createMockRequest("/api/me/profile", {
      method: "PUT",
      body: { phone: "invalid" },
    });
    const response = await PUT(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
