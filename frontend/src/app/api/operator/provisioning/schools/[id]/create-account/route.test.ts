import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockAuth, mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST } from "./route";

describe("POST /api/operator/provisioning/schools/[id]/create-account", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("creates account for school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const accountData = {
      email: "new@school.de",
      first_name: "Max",
      last_name: "Mustermann",
    };
    const createdAccount = { id: 99, email: "new@school.de" };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 201,
      json: async () => ({ status: "success", data: createdAccount }),
    });

    const context: RouteContext = {
      params: Promise.resolve({ id: "42" }),
    };
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/42/create-account",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(accountData),
      },
    );
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(createdAccount);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/42/create-account",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify(accountData),
      }),
    );
  });

  it("passes the correct school id to the backend endpoint", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { id: 1, email: "test@test.de" },
      }),
    });

    const context: RouteContext = {
      params: Promise.resolve({ id: "77" }),
    };
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/77/create-account",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: "test@test.de",
          first_name: "Test",
          last_name: "User",
        }),
      },
    );
    await POST(request, context);

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/77/create-account",
      expect.any(Object),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const context: RouteContext = {
      params: Promise.resolve({ id: "42" }),
    };
    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/42/create-account",
      {
        method: "POST",
        body: JSON.stringify({ email: "test@test.de" }),
      },
    );
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("returns error for invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/bad/create-account",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ email: "test@test.de" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 999 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });
});
