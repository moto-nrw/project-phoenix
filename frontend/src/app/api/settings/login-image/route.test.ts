import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const {
  mockAuth,
  mockUncachedAuth,
  mockFetch,
  mockGetServerApiUrl,
  mockRevalidateTag,
} = vi.hoisted(() => ({
  mockAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
  mockRevalidateTag: vi.fn(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

vi.mock("next/cache", () => ({
  revalidateTag: mockRevalidateTag,
}));

vi.mock("~/lib/api-helpers.server", () => ({
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    return new Response(JSON.stringify({ error: message }), { status: 500 });
  }),
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { POST, DELETE, GET } from "./route";

const defaultSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

function createMockContext() {
  return { params: Promise.resolve({}) };
}

function createMockImageFile(): File {
  const bytes = new Uint8Array([
    0xff,
    0xd8,
    0xff,
    0xe0,
    ...new Array<number>(100).fill(0),
  ]);
  return new File([bytes], "test.jpg", { type: "image/jpeg" });
}

describe("POST /api/settings/login-image", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const formData = new FormData();
    formData.append("login_image", createMockImageFile());

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "POST", body: formData },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("successfully uploads a valid image", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        data: { login_image_url: "/uploads/login-image.jpg" },
      }),
    });

    const formData = new FormData();
    formData.append("login_image", createMockImageFile());

    const request = new NextRequest(
      "http://school-a.localhost:3000/api/settings/login-image",
      {
        method: "POST",
        body: formData,
        headers: { host: "school-a.localhost:3000" },
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: { login_image_url: string };
    };
    expect(json.data.login_image_url).toBe("/uploads/login-image.jpg");
  });

  it("rejects request with no file", async () => {
    const formData = new FormData();

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "POST", body: formData },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("No image file provided");
  });

  it("handles backend error", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Internal server error",
      statusText: "Internal Server Error",
    });

    const formData = new FormData();
    formData.append("login_image", createMockImageFile());

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "POST", body: formData },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("calls revalidateTag after successful upload", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        data: { login_image_url: "/uploads/login-image.jpg" },
      }),
    });

    const formData = new FormData();
    formData.append("login_image", createMockImageFile());

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "POST", body: formData },
    );
    request.headers.set("referer", "http://localhost:3000/school-a/settings");
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
  });

  it("retries on 401 with refreshed token", async () => {
    mockUncachedAuth.mockResolvedValueOnce({
      user: { token: "fresh-token" },
    });

    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          data: { login_image_url: "/uploads/login-image.jpg" },
        }),
      });

    const formData = new FormData();
    formData.append("login_image", createMockImageFile());

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      {
        method: "POST",
        body: formData,
        headers: { referer: "http://localhost:3000/school-a/settings" },
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    // First call uses original token
    expect(mockFetch.mock.calls[0]![1]).toEqual(
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );
    // Second call uses refreshed token
    expect(mockFetch.mock.calls[1]![1]).toEqual(
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });
});

describe("DELETE /api/settings/login-image", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "DELETE" },
    );
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("successfully deletes login image", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      text: async () => "",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      {
        method: "DELETE",
        headers: { referer: "http://localhost:3000/school-a/settings" },
      },
    );
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(204);
  });

  it("handles backend error", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Internal server error",
      statusText: "Internal Server Error",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "DELETE" },
    );
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("calls revalidateTag after successful delete", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      text: async () => "",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "DELETE" },
    );
    request.headers.set("referer", "http://localhost:3000/school-a/settings");
    await DELETE(request, createMockContext());

    expect(mockRevalidateTag).toHaveBeenCalledWith("tenant-school-a", {
      expire: 0,
    });
  });
});

describe("GET /api/settings/login-image", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(defaultSession);
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "GET" },
    );
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(401);
  });

  it("returns data from backend", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      status: 200,
      json: async () => ({
        data: { login_image_url: "/uploads/login-image.jpg", can_edit: true },
      }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "GET" },
    );
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: { login_image_url: string; can_edit: boolean };
    };
    expect(json.data.login_image_url).toBe("/uploads/login-image.jpg");
    expect(json.data.can_edit).toBe(true);
  });

  it("handles backend error", async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Internal server error",
      statusText: "Internal Server Error",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "GET" },
    );
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(500);
  });

  it("retries on 401 with refreshed token", async () => {
    mockUncachedAuth.mockResolvedValueOnce({
      user: { token: "fresh-token" },
    });

    mockFetch
      .mockResolvedValueOnce({
        ok: false,
        status: 401,
      })
      .mockResolvedValueOnce({
        ok: true,
        status: 200,
        json: async () => ({
          data: {
            login_image_url: "/uploads/login-image.jpg",
            can_edit: true,
          },
        }),
      });

    const request = new NextRequest(
      "http://localhost:3000/api/settings/login-image",
      { method: "GET" },
    );
    const response = await GET(request, createMockContext());

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch.mock.calls[0]![1]).toEqual(
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer test-token",
        }),
      }),
    );
    expect(mockFetch.mock.calls[1]![1]).toEqual(
      expect.objectContaining({
        headers: expect.objectContaining({
          Authorization: "Bearer fresh-token",
        }),
      }),
    );
  });
});
