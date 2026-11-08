import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";
import { NextRequest } from "next/server";
import type { ExtendedSession } from "~/test/route-test-helpers";

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<(...args: unknown[]) => Promise<Response>>(),
}));

vi.mock("~/server/auth", async () =>
  (await import("~/test/portal-binding")).mockPortalModule("tenant", {
    auth: mockAuth,
    uncachedAuth: mockUncachedAuth,
    withTenantAuth: (handler: unknown) => handler,
  }),
);

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const originalFetch = global.fetch;
beforeEach(() => {
  vi.clearAllMocks();
  global.fetch = mockFetch as unknown as typeof fetch;
  mockAuth.mockResolvedValue(staleSession);
});

afterAll(() => {
  global.fetch = originalFetch;
});

const staleSession: ExtendedSession = {
  user: { id: "1", token: "stale-token", name: "Test User" },
  expires: "2099-01-01",
};

const refreshedSession: ExtendedSession = {
  user: { id: "1", token: "fresh-token", name: "Test User" },
  expires: "2099-01-01",
};

function jsonResponse(status: number, body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const context = { params: Promise.resolve({ id: "42" }) };

// NextRequest's own constructor does not implement formData in the test
// runtime, so the multipart body goes through a plain Request first.
function uploadRequest(
  photo: File,
  consentAcknowledged?: boolean,
): NextRequest {
  const fd = new FormData();
  fd.append("photo", photo);
  if (consentAcknowledged) fd.append("consent_acknowledged", "true");
  return new NextRequest(
    new Request("http://localhost:3000/api/students/42/photo", {
      method: "POST",
      body: fd,
    }),
  );
}

const JPEG_BYTES = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0]);
const NON_IMAGE_BYTES = new Uint8Array([0x00, 0x01, 0x02, 0x03, 0, 0, 0, 0]);

describe("POST /api/students/[id]/photo", () => {
  it("rejects with 415 when the bytes are no JPEG, PNG or WebP", async () => {
    const { POST } = await import("./route");
    const response = await POST(
      uploadRequest(
        new File([NON_IMAGE_BYTES], "fake.jpg", { type: "image/jpeg" }),
      ),
      context,
    );

    expect(response.status).toBe(415);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("rejects a GIF before the backend sees it", async () => {
    const { POST } = await import("./route");
    const response = await POST(
      uploadRequest(new File([JPEG_BYTES], "a.gif", { type: "image/gif" })),
      context,
    );

    expect(response.status).toBe(415);
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("relays the photo with the consent flag and returns the photo_url", async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse(200, {
        status: "success",
        data: { photo_url: "/uploads/student-photos/42_abc.jpg" },
      }),
    );

    const { POST } = await import("./route");
    const response = await POST(
      uploadRequest(
        new File([JPEG_BYTES], "real.jpg", { type: "image/jpeg" }),
        true,
      ),
      context,
    );

    expect(response.status).toBe(200);
    expect(await response.json()).toMatchObject({
      data: { photo_url: "/uploads/student-photos/42_abc.jpg" },
    });
    const [url, init] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("http://backend.test/api/students/42/photo");
    const body = init.body as FormData;
    expect(body.get("photo")).toBeInstanceOf(File);
    expect(body.get("consent_acknowledged")).toBe("true");
  });
});

describe("DELETE /api/students/[id]/photo", () => {
  it("asks the backend at most twice after an expired session", async () => {
    mockUncachedAuth.mockResolvedValue(refreshedSession);
    mockFetch
      .mockResolvedValueOnce(jsonResponse(401, { detail: "expired" }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const { DELETE } = await import("./route");
    const response = await DELETE(
      new NextRequest("http://localhost:3000/api/students/42/photo", {
        method: "DELETE",
      }),
      context,
    );

    expect(response.status).toBe(204);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockFetch.mock.calls[0]?.[0]).toBe(
      "http://backend.test/api/students/42/photo",
    );
  });
});
