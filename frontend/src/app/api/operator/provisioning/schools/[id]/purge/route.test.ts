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

describe("POST /api/operator/provisioning/schools/[id]/purge", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("purges a soft-deleted school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const purgeResult = {
      tenant_id: 10,
      tables_processed: 5,
      total_rows_deleted: 120,
      details: [{ table: "users.students", rows_deleted: 80 }],
    };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: purgeResult }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/purge",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: "PURGE" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data?: unknown };
    expect(json.data).toEqual(purgeResult);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/purge",
      expect.objectContaining({
        method: "POST",
        body: JSON.stringify({ confirm: "PURGE" }),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/purge",
      {
        method: "POST",
        body: JSON.stringify({ confirm: "PURGE" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(401);
  });

  it("handles invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/purge",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: "PURGE" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 123 as unknown as string }),
    };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });

  it("returns 400 when confirmation is missing", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 400,
      text: async () =>
        JSON.stringify({
          error: 'confirmation required: set confirm to "PURGE"',
        }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/purge",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({}),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(400);
  });

  it("returns 409 when school is not soft-deleted", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: "School is not deleted" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/purge",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: "PURGE" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(409);
  });

  it("returns 500 when purge stored procedure fails", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      text: async () =>
        JSON.stringify({ error: "Failed to purge tenant data" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/purge",
      {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ confirm: "PURGE" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await POST(request, context);

    expect(response.status).toBe(500);
  });
});
