import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";

const { mockFetch, mockGetServerApiUrl } = vi.hoisted(() => ({
  mockFetch: vi.fn(),
  mockGetServerApiUrl: vi.fn(() => "http://localhost:8080"),
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: mockGetServerApiUrl,
}));

global.fetch = mockFetch as unknown as typeof fetch;

import { GET } from "./route";

function createMockContext(filename: string) {
  return { params: Promise.resolve({ filename }) };
}

describe("GET /api/public/login-image/[filename]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("returns 404 when filename contains path traversal (..) ", async () => {
    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/public/login-image/..%2Fetc%2Fpasswd",
      ),
      createMockContext("../etc/passwd"),
    );

    expect(response.status).toBe(404);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns 404 when filename contains a slash", async () => {
    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/public/login-image/sub/file.png",
      ),
      createMockContext("sub/file.png"),
    );

    expect(response.status).toBe(404);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("returns 404 when filename is empty", async () => {
    const response = await GET(
      new NextRequest("http://localhost:3000/api/public/login-image/"),
      createMockContext(""),
    );

    expect(response.status).toBe(404);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("proxies image bytes with Cache-Control header on success", async () => {
    const imageData = new ArrayBuffer(10);
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "Content-Type": "image/png" }),
      arrayBuffer: async () => imageData,
    });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/public/login-image/logo.png"),
      createMockContext("logo.png"),
    );

    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/public/login-image/logo.png",
      { signal: expect.any(AbortSignal) },
    );
    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("image/png");
    expect(response.headers.get("Cache-Control")).toBe("public, max-age=86400");
    expect(await response.arrayBuffer()).toEqual(imageData);
  });

  it("returns 404 when backend returns 404", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 404,
      headers: new Headers(),
    });

    const response = await GET(
      new NextRequest(
        "http://localhost:3000/api/public/login-image/missing.png",
      ),
      createMockContext("missing.png"),
    );

    expect(response.status).toBe(404);
  });

  it("returns 502 when backend returns non-image content type", async () => {
    mockFetch.mockResolvedValue({
      ok: true,
      status: 200,
      headers: new Headers({ "Content-Type": "text/html" }),
      arrayBuffer: async () => new ArrayBuffer(0),
    });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/public/login-image/bad.png"),
      createMockContext("bad.png"),
    );

    expect(response.status).toBe(502);
  });

  it("returns 502 when fetch throws a network error", async () => {
    mockFetch.mockRejectedValue(new Error("ECONNREFUSED"));

    const response = await GET(
      new NextRequest("http://localhost:3000/api/public/login-image/logo.png"),
      createMockContext("logo.png"),
    );

    expect(response.status).toBe(502);
  });

  it("returns 502 when backend returns a non-404 error (e.g. 500)", async () => {
    mockFetch.mockResolvedValue({
      ok: false,
      status: 500,
      headers: new Headers(),
    });

    const response = await GET(
      new NextRequest("http://localhost:3000/api/public/login-image/logo.png"),
      createMockContext("logo.png"),
    );

    expect(response.status).toBe(502);
  });
});
