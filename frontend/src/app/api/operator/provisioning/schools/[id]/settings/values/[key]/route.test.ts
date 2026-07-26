import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils.server";

const {
  mockAuth,
  mockFetch,
  mockGetServerApiUrl,
  mockRevalidateTag,
  mockRevalidatePath,
} = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockRevalidateTag: vi.fn(),
  mockRevalidatePath: vi.fn(),
}));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("next/cache", () => ({
  revalidateTag: (...args: unknown[]) => mockRevalidateTag(...args),
  revalidatePath: (...args: unknown[]) => mockRevalidatePath(...args),
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { PUT, DELETE } from "./route";

describe("PUT /api/operator/provisioning/schools/[id]/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("proxies valid setting update to backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: null }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/operations.session_end_time",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: "18:30" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "operations.session_end_time",
      }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/settings/values/operations.session_end_time",
      expect.objectContaining({ method: "PUT" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: "x" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "foo.bar" }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(401);
  });

  it("rejects invalid key pattern (uppercase not allowed)", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/BAD.KEY",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: "x" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "BAD.KEY" }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects non-string id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: "x" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: 10 as unknown as string,
        key: "foo.bar",
      }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(500);
  });

  it("revalidates tenant cache for keys that ride on /auth/tenant/resolve", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    // First fetch: the backend PUT. Second fetch: the slug lookup via
    // /operator/schools/summaries that revalidateTenantIfNeeded issues.
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "success", data: null }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: [
            { id: 10, slug: "school-ten" },
            { id: 11, slug: "school-eleven" },
          ],
        }),
      });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/operations.student_photos_enabled",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: true }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "operations.student_photos_enabled",
      }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/settings/values/operations.student_photos_enabled",
      expect.objectContaining({ method: "PUT" }),
    );
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/summaries",
      expect.objectContaining({ method: "GET" }),
    );
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-ten", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-ten", "layout");
  });

  it("revalidates tenant metadata after an operator changes grade_level_max", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "success", data: null }),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: [{ id: 10, slug: "school-ten" }],
        }),
      });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/enrollment.grade_level_max",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: 13 }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "enrollment.grade_level_max",
      }),
    };

    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-ten", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-ten", "layout");
  });

  it("skips revalidation when the key does not affect /auth/tenant/resolve", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: null }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/operations.session_end_time",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: "18:30" }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "operations.session_end_time",
      }),
    };
    await PUT(request, context);

    // Only the backend PUT, never the summaries lookup.
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockRevalidateTag).not.toHaveBeenCalled();
    expect(mockRevalidatePath).not.toHaveBeenCalled();
  });

  it("skips revalidation when the slug lookup cannot resolve the school", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "success", data: null }),
      })
      // summaries lookup returns an empty list — no matching id.
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({ status: "success", data: [] }),
      });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/operations.student_photos_enabled",
      {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ value: false }),
      },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "operations.student_photos_enabled",
      }),
    };
    const response = await PUT(request, context);

    expect(response.status).toBe(200);
    expect(mockRevalidateTag).not.toHaveBeenCalled();
    expect(mockRevalidatePath).not.toHaveBeenCalled();
  });
});

describe("DELETE /api/operator/provisioning/schools/[id]/settings/values/[key]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("proxies reset to backend", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 204,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar",
      { method: "DELETE" },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "foo.bar" }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/operator/schools/10/settings/values/foo.bar",
      expect.objectContaining({ method: "DELETE" }),
    );
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValue(null);

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar",
      { method: "DELETE" },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "foo.bar" }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(401);
  });

  it("rejects invalid key pattern", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/bad key",
      { method: "DELETE" },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "bad key" }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects non-string id parameter", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar",
      { method: "DELETE" },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: 10 as unknown as string,
        key: "foo.bar",
      }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(500);
  });

  it("revalidates tenant cache for keys that ride on /auth/tenant/resolve", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch
      // Backend DELETE — 204 No Content, parseResponse short-circuits.
      .mockResolvedValueOnce({ ok: true, status: 204 })
      // Slug lookup.
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          status: "success",
          data: [{ id: 10, slug: "school-ten" }],
        }),
      });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/operations.student_photos_enabled",
      { method: "DELETE" },
    );
    const context: RouteContext = {
      params: Promise.resolve({
        id: "10",
        key: "operations.student_photos_enabled",
      }),
    };
    const response = await DELETE(request, context);

    expect(response.status).toBe(204);
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-ten", {
      expire: 0,
    });
    expect(mockRevalidatePath).toHaveBeenCalledWith("/school-ten", "layout");
  });

  it("skips revalidation when the key does not affect /auth/tenant/resolve", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({ ok: true, status: 204 });

    const request = new NextRequest(
      "http://localhost:3000/api/operator/provisioning/schools/10/settings/values/foo.bar",
      { method: "DELETE" },
    );
    const context: RouteContext = {
      params: Promise.resolve({ id: "10", key: "foo.bar" }),
    };
    await DELETE(request, context);

    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockRevalidateTag).not.toHaveBeenCalled();
    expect(mockRevalidatePath).not.toHaveBeenCalled();
  });
});
