import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import type { RouteContext } from "~/lib/route-wrapper-utils";

const { mockAuth, mockFetch, mockGetServerApiUrl, mockRevalidateTag } =
  vi.hoisted(() => ({
    mockAuth: vi.fn(),
    mockFetch: vi.fn(),
    mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
    mockRevalidateTag: vi.fn(),
  }));

vi.mock("~/server/auth/operator", () => ({
  operatorAuth: mockAuth,
  uncachedOperatorAuth: mockAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("next/cache", () => ({
  revalidateTag: mockRevalidateTag,
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

  it("busts tenant resolve cache for tenant-affecting keys", async () => {
    // Photo-flag writes return school_slug so the proxy can invalidate the
    // slug-keyed `tenant-${slug}` Next.js tag the [tenant]/layout.tsx fetch
    // populates from /auth/tenant/resolve.
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { school_slug: "musterschule" },
      }),
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
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-musterschule", {
      expire: 0,
    });
  });

  it("does NOT bust cache for non-tenant-resolve-affecting keys", async () => {
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { school_slug: "musterschule" },
      }),
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

    expect(mockRevalidateTag).not.toHaveBeenCalled();
  });

  it("skips cache bust when backend omits school_slug", async () => {
    // Defensive: the backend logs but never propagates a slug-lookup
    // failure (the mutation already committed). The proxy must tolerate the
    // missing slug without crashing — at worst the cache TTL recovers it.
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({ status: "success", data: {} }),
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

  it("busts tenant resolve cache on reset of tenant-affecting keys", async () => {
    // Reset (DELETE) of the photo flag returns school_slug so the proxy can
    // invalidate `tenant-${slug}` here too. Without this, "Zurücksetzen" on
    // operations.student_photos_enabled would keep tenant users staring at
    // the stale resolve payload until cache TTL expiry.
    mockAuth.mockResolvedValue({ user: { token: "valid-token" } });
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        status: "success",
        data: { school_slug: "musterschule" },
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
    await DELETE(request, context);

    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-musterschule", {
      expire: 0,
    });
  });
});
