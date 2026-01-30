import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";
import { GET } from "./route";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers", () => ({
  handleApiError: vi.fn((error: unknown) => {
    const message =
      error instanceof Error ? error.message : "Internal Server Error";
    const status = message.includes("(401)")
      ? 401
      : message.includes("(400)")
        ? 400
        : 500;
    return new Response(JSON.stringify({ error: message }), { status });
  }),
}));

const defaultSession: ExtendedSession = {
  user: { id: "1", token: "test-token", name: "Test User" },
  expires: "2099-01-01",
};

function createMockContext(
  params: Record<string, string | string[] | undefined> = {},
) {
  return { params: Promise.resolve(params) };
}

describe("GET /api/me/profile/avatar/[filename]", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    global.fetch = vi.fn();
  });

  it("returns 401 when not authenticated", async () => {
    mockAuth.mockResolvedValueOnce(null);

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/test.jpg",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "test.jpg" }),
    );

    expect(response.status).toBe(401);
  });

  it("returns 400 when filename is missing", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/",
    );
    const response = await GET(
      request,
      createMockContext({ filename: undefined }),
    );

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("Filename is required");
  });

  it("rejects filename with path traversal attack", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/../../../etc/passwd",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "../../../etc/passwd" }),
    );

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("Invalid filename");
  });

  it("rejects filename with forward slashes", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/foo/bar.jpg",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "foo/bar.jpg" }),
    );

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("Invalid filename");
  });

  it("rejects filename with backslashes", async () => {
    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/foo\\bar.jpg",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "foo\\bar.jpg" }),
    );

    expect(response.status).toBe(400);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("Invalid filename");
  });

  it("successfully fetches JPEG avatar", async () => {
    const mockImageBuffer = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0, 0]);

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      headers: new Map([["content-type", "image/jpeg"]]),
      arrayBuffer: async () => mockImageBuffer.buffer,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/avatar123.jpg",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "avatar123.jpg" }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("image/jpeg");
    expect(response.headers.get("cache-control")).toBe(
      "private, max-age=86400",
    );

    const buffer = await response.arrayBuffer();
    expect(new Uint8Array(buffer)).toEqual(mockImageBuffer);
  });

  it("successfully fetches PNG avatar", async () => {
    const mockImageBuffer = new Uint8Array([0x89, 0x50, 0x4e, 0x47]);

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      headers: new Map([["content-type", "image/png"]]),
      arrayBuffer: async () => mockImageBuffer.buffer,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/avatar456.png",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "avatar456.png" }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("image/png");
  });

  it("uses default content-type when backend doesn't provide one", async () => {
    const mockImageBuffer = new Uint8Array([0, 0, 0, 0]);

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      headers: new Map(),
      arrayBuffer: async () => mockImageBuffer.buffer,
    });

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/test.jpg",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "test.jpg" }),
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe("image/jpeg");
  });

  it("handles backend fetch errors", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 404,
      text: async () => "Avatar not found",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar/nonexistent.jpg",
    );
    const response = await GET(
      request,
      createMockContext({ filename: "nonexistent.jpg" }),
    );

    expect(response.status).toBe(404);
    const json = (await response.json()) as { error: string };
    expect(json.error).toContain("Avatar not found");
  });
});
