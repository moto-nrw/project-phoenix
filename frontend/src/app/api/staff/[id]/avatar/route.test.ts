import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest, NextResponse } from "next/server";

const {
  mockAuth,
  mockUncachedAuth,
  mockFetch,
  mockHandleApiError,
  mockGetServerApiUrl,
} = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockHandleApiError: vi.fn((error: unknown) =>
    NextResponse.json(
      { error: error instanceof Error ? error.message : String(error) },
      { status: 500 },
    ),
  ),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  handleApiError: mockHandleApiError,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

describe("GET /api/staff/[id]/avatar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue({ user: { token: "session-token" } });
    mockUncachedAuth.mockResolvedValue({ user: { token: "session-token" } });
  });

  it("returns 401 when unauthenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(401);
    await expect(response.json()).resolves.toEqual({
      error: "Unauthorized",
      success: false,
      message: "Unauthorized",
    });
  });

  it("returns 400 when the staff id is missing", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/avatar"),
      { params: Promise.resolve({}) },
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      error: "Valid staff ID is required",
      success: false,
    });
  });

  it("returns 400 when the staff id is not numeric", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/not-a-number/avatar"),
      { params: Promise.resolve({ id: "not-a-number" }) },
    );

    expect(response.status).toBe(400);
    await expect(response.json()).resolves.toEqual({
      error: "Valid staff ID is required",
      success: false,
    });
  });

  it("proxies avatar bytes without allowing caches to reuse stale avatars", async () => {
    const bytes = new Uint8Array([1, 2, 3, 4]);
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "content-type": "image/png" }),
      arrayBuffer: async () => bytes.buffer,
    });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/api/staff/42/avatar",
      {
        cache: "no-store",
        headers: {
          Authorization: "Bearer session-token",
        },
      },
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/png");
    expect(response.headers.get("Cache-Control")).toBe(
      "private, no-store, max-age=0",
    );
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes);
  });

  it("passes backend error statuses through unchanged", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
    });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(404);
  });

  it("retries once with uncachedAuth when the backend returns 401", async () => {
    const bytes = new Uint8Array([9, 8, 7]);
    mockUncachedAuth.mockResolvedValueOnce({ user: { token: "fresh-token" } });
    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
        headers: new Headers(),
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        headers: new Headers({ "content-type": "image/webp" }),
        arrayBuffer: async () => bytes.buffer,
      });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockFetch).toHaveBeenNthCalledWith(
      1,
      "http://localhost:8080/api/staff/42/avatar",
      {
        cache: "no-store",
        headers: {
          Authorization: "Bearer session-token",
        },
      },
    );
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      "http://localhost:8080/api/staff/42/avatar",
      {
        cache: "no-store",
        headers: {
          Authorization: "Bearer fresh-token",
        },
      },
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/webp");
    expect(new Uint8Array(await response.arrayBuffer())).toEqual(bytes);
  });

  it("delegates thrown errors to handleApiError", async () => {
    const thrown = new Error("network exploded");
    mockFetch.mockRejectedValue(thrown);

    const response = await GET(
      new NextRequest("http://localhost:3000/api/staff/42/avatar"),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(mockHandleApiError).toHaveBeenCalledWith(thrown);
    expect(response.status).toBe(500);
    await expect(response.json()).resolves.toEqual({
      error: "network exploded",
    });
  });
});
