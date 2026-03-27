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

import { PUT, DELETE } from "./route";

describe("PUT /api/operator/provisioning/schools/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("updates school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    const updateData = {
      organization_id: 1,
      name: "Updated School",
      slug: "updated-school",
      subdomain: "updated-school",
      address: "Street 1",
      city: "Berlin",
      zip: "10115",
      phone: "030123456",
      email: "school@example.com",
      active: true,
    };
    const updatedSchool = { id: 10, ...updateData };

    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: updatedSchool }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(updateData),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data?: unknown;
      error?: string;
    };
    expect(json.data).toEqual(updatedSchool);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10",
      expect.objectContaining({
        method: "PUT",
        body: JSON.stringify(updateData),
      }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10",
      {
        method: "PUT",
        body: JSON.stringify({ name: "Test" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(401);
  });

  it("returns error for invalid id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/bad",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Test" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: 999 as unknown as string }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(500);
  });

  it("forwards 404 not found from backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "School not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/999",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ name: "Test", slug: "test", subdomain: "test" }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "999" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(404);
  });

  it("forwards 409 conflict from backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: "subdomain already exists" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: "Test",
          slug: "test",
          subdomain: "taken",
        }),
      },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "10" }) };
    const response = await PUT(request, context);

    expect(response.status).toBe(409);
  });
});

describe("DELETE /api/operator/provisioning/schools/[id]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("soft-deletes school successfully", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: null,
        message: "School soft-deleted successfully",
      }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/55",
      { method: "DELETE" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "55" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/55",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/55",
      { method: "DELETE" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "55" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(401);
  });

  it("forwards 404 not found from backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      text: async () => JSON.stringify({ error: "School not found" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/999",
      { method: "DELETE" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "999" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(404);
  });

  it("forwards 409 already deleted from backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: false,
      status: 409,
      text: async () => JSON.stringify({ error: "School is already deleted" }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/55",
      { method: "DELETE" },
    );
    const context: RouteContext = { params: Promise.resolve({ id: "55" }) };
    const response = await DELETE(request, context);

    expect(response.status).toBe(409);
  });
});
