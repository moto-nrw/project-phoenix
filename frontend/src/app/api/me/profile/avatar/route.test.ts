import { describe, it, expect, vi, beforeEach } from "vitest";
import { NextRequest } from "next/server";
import { POST, DELETE } from "./route";
import {
  type ExtendedSession,
  createMockContext,
  defaultSession,
} from "~/test/route-test-helpers";

const { mockAuth } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
}));

vi.mock("~/lib/api-helpers.server", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/api-helpers.server")>()),
  handleApiError: vi.fn(
    (await import("~/test/helpers/handle-api-error")).mockHandleApiError,
  ),
}));

// Helper to create a valid image file with proper magic bytes
function createMockImageFile(
  type: "jpeg" | "png" | "gif" | "webp" = "jpeg",
): File {
  const magicBytes: Record<string, number[]> = {
    jpeg: [0xff, 0xd8, 0xff, 0xe0],
    png: [0x89, 0x50, 0x4e, 0x47],
    gif: [0x47, 0x49, 0x46, 0x38],
    webp: [0x52, 0x49, 0x46, 0x46, 0, 0, 0, 0, 0x57, 0x45, 0x42, 0x50],
  };

  const mimeTypes = {
    jpeg: "image/jpeg",
    png: "image/png",
    gif: "image/gif",
    webp: "image/webp",
  };

  const bytes = magicBytes[type] ?? magicBytes.jpeg!;
  const buffer = new Uint8Array([...bytes, ...new Array<number>(100).fill(0)]);
  return new File([buffer], `test.${type}`, { type: mimeTypes[type] });
}

describe("POST /api/me/profile/avatar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    global.fetch = vi.fn();
  });

  it("successfully uploads a valid JPEG avatar", async () => {
    const mockProfile = {
      id: 1,
      first_name: "John",
      avatar_url: "/avatars/1.jpg",
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, data: mockProfile }),
    });

    const formData = new FormData();
    formData.append("avatar", createMockImageFile("jpeg"));

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "POST",
        body: formData,
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
    const json = (await response.json()) as { data: { avatar_url: string } };
    expect(json.data.avatar_url).toBe("/avatars/1.jpg");
  });

  it("successfully uploads a valid PNG avatar", async () => {
    const mockProfile = {
      id: 1,
      avatar_url: "/avatars/1.png",
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, data: mockProfile }),
    });

    const formData = new FormData();
    formData.append("avatar", createMockImageFile("png"));

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "POST",
        body: formData,
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(200);
  });

  it("rejects file with invalid magic bytes", async () => {
    const invalidFile = new File([new Uint8Array([0, 0, 0, 0])], "fake.jpg", {
      type: "image/jpeg",
    });

    const formData = new FormData();
    formData.append("avatar", invalidFile);

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "POST",
        body: formData,
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("Invalid file format");
  });

  it("rejects request with no file", async () => {
    const formData = new FormData();

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "POST",
        body: formData,
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
    const json = (await response.json()) as { detail: string };
    expect(json.detail).toBe("Internal Server Error");
    expect(JSON.stringify(json)).not.toContain("No avatar file provided");
  });

  it("handles backend upload errors", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 500,
      text: async () => "Backend upload failed",
    });

    const formData = new FormData();
    formData.append("avatar", createMockImageFile());

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "POST",
        body: formData,
      },
    );
    const response = await POST(request, createMockContext());

    expect(response.status).toBe(500);
  });
});

describe("DELETE /api/me/profile/avatar", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockAuth.mockResolvedValue(defaultSession);
    global.fetch = vi.fn();
  });

  it("successfully deletes avatar", async () => {
    const mockProfile = {
      id: 1,
      first_name: "John",
      avatar_url: null,
    };

    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: true,
      json: async () => ({ success: true, data: mockProfile }),
    });

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "DELETE",
      },
    );
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(200);
    const json = (await response.json()) as {
      data: { avatar_url: string | null };
    };
    expect(json.data.avatar_url).toBeNull();
  });

  it("handles backend deletion errors", async () => {
    (global.fetch as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      ok: false,
      status: 404,
      text: async () => "Avatar not found",
    });

    const request = new NextRequest(
      "http://localhost:3000/api/me/profile/avatar",
      {
        method: "DELETE",
      },
    );
    const response = await DELETE(request, createMockContext());

    expect(response.status).toBe(500);
  });
});
