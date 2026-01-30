import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { mockSessionData } from "~/test/mocks/next-auth";

// ============================================================================
// Mocks
// ============================================================================

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockApiGet, mockApiPut, mockApiDelete } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockApiGet: vi.fn(),
  mockApiPut: vi.fn(),
  mockApiDelete: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  apiGet: mockApiGet,
  apiPut: mockApiPut,
  apiDelete: mockApiDelete,
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

import { GET, PUT, DELETE } from "./route";

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

function mockValidSession(): void {
  mockAuth.mockResolvedValue(mockSessionData() as ExtendedSession);
}

// ============================================================================
// Tests
// ============================================================================

describe("GET /api/suggestions/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("fetches a single suggestion", async () => {
    mockValidSession();
    mockApiGet.mockResolvedValue({
      data: { id: 42, title: "Test" },
    });

    const req = createMockRequest("/api/suggestions/42");
    const response = await GET(req, {
      params: Promise.resolve({ id: "42" }),
    });
    const json = (await response.json()) as { success: boolean };

    expect(mockApiGet).toHaveBeenCalledWith(
      "/api/suggestions/42",
      "test-token",
    );
    expect(json.success).toBe(true);
  });

  it("returns error for invalid id", async () => {
    mockValidSession();

    const req = createMockRequest("/api/suggestions/invalid");
    const response = await GET(req, {
      params: Promise.resolve({ id: ["a", "b"] }),
    });

    expect(response.status).toBe(500);
  });
});

describe("PUT /api/suggestions/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates a suggestion", async () => {
    mockValidSession();
    mockApiPut.mockResolvedValue({
      data: { id: 1, title: "Updated" },
    });

    const req = createMockRequest("/api/suggestions/1", {
      method: "PUT",
      body: { title: "Updated", description: "Updated desc" },
    });
    const response = await PUT(req, {
      params: Promise.resolve({ id: "1" }),
    });
    const json = (await response.json()) as { success: boolean };

    expect(mockApiPut).toHaveBeenCalledWith(
      "/api/suggestions/1",
      "test-token",
      {
        title: "Updated",
        description: "Updated desc",
      },
    );
    expect(json.success).toBe(true);
  });
});

describe("DELETE /api/suggestions/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("deletes a suggestion", async () => {
    mockValidSession();
    mockApiDelete.mockResolvedValue(undefined);

    const req = createMockRequest("/api/suggestions/1", { method: "DELETE" });
    const response = await DELETE(req, {
      params: Promise.resolve({ id: "1" }),
    });

    expect(mockApiDelete).toHaveBeenCalledWith(
      "/api/suggestions/1",
      "test-token",
    );
    expect(response.status).toBe(204);
  });
});
