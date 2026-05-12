// Coverage for the DELETE handler's 401 retry. The POST handler already
// owns this pattern via createFileUploadHandler — DELETE used to throw a
// generic German error message, which doesn't match the literal "API error
// (401)" string the shared retry wrapper looks for, so an expired-token
// delete fell straight through to a hard failure for idle users. This test
// pins the inline retry: when the first call returns 401, refresh the
// session and retry once with the new token; only fail if the retry also
// 401s (or returns another non-OK status).

import { describe, it, expect, vi, beforeEach, afterAll } from "vitest";
import type { Session } from "next-auth";
import { NextRequest } from "next/server";

interface ExtendedSession extends Session {
  user: Session["user"] & { token?: string };
}

const { mockAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockUncachedAuth: vi.fn<() => Promise<ExtendedSession | null>>(),
  mockFetch: vi.fn<(...args: unknown[]) => Promise<Response>>(),
}));

vi.mock("~/server/auth", () => ({
  auth: mockAuth,
  uncachedAuth: mockUncachedAuth,
}));

vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://backend.test",
}));

const originalFetch = global.fetch;
beforeEach(() => {
  vi.clearAllMocks();
  global.fetch = mockFetch as unknown as typeof fetch;
});

afterAll(() => {
  global.fetch = originalFetch;
});

function createDeleteRequest(studentId: string): NextRequest {
  return new NextRequest(
    new URL(`/api/students/${studentId}/photo`, "http://localhost:3000"),
    { method: "DELETE" },
  );
}

const defaultSession: ExtendedSession = {
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

describe("DELETE /api/students/[id]/photo — 401 retry", () => {
  it("retries with a refreshed token when the backend returns 401", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(refreshedSession);

    // First call: stale-token → 401. Second call: fresh-token → 200.
    mockFetch
      .mockResolvedValueOnce(jsonResponse(401, { error: "expired" }))
      .mockResolvedValueOnce(
        jsonResponse(200, { status: "success", data: null }),
      );

    const { DELETE } = await import("./route");
    const response = await DELETE(createDeleteRequest("42"), {
      params: Promise.resolve({ id: "42" }),
    });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    expect(mockUncachedAuth).toHaveBeenCalledTimes(1);

    // First request used the stale token; second used the refreshed one.
    const firstCallHeaders = (
      mockFetch.mock.calls[0]?.[1] as RequestInit | undefined
    )?.headers as Record<string, string> | undefined;
    const secondCallHeaders = (
      mockFetch.mock.calls[1]?.[1] as RequestInit | undefined
    )?.headers as Record<string, string> | undefined;
    expect(firstCallHeaders?.Authorization).toBe("Bearer stale-token");
    expect(secondCallHeaders?.Authorization).toBe("Bearer fresh-token");
  });

  it("does not retry when uncachedAuth returns the same token", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    // Refresh attempted but yielded the same token (refresh-token cookie
    // also expired). We must fall through and surface the original error
    // instead of looping the same request indefinitely.
    mockUncachedAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(jsonResponse(401, { error: "expired" }));

    const { DELETE } = await import("./route");
    const response = await DELETE(createDeleteRequest("42"), {
      params: Promise.resolve({ id: "42" }),
    });

    expect(response.status).not.toBe(200);
    // Single backend call: 401 → uncachedAuth → token unchanged → fall
    // through (no retry), let createDeleteHandler's outer wrapper map this
    // into the user-facing error.
    expect(mockFetch).toHaveBeenCalledTimes(1);
  });

  it("does not invoke uncachedAuth when the backend returns 200", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(
      jsonResponse(200, { status: "success", data: null }),
    );

    const { DELETE } = await import("./route");
    const response = await DELETE(createDeleteRequest("42"), {
      params: Promise.resolve({ id: "42" }),
    });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    expect(mockUncachedAuth).not.toHaveBeenCalled();
  });
});

// Regression for the "4xx-collapsed-to-500" bug. createDeleteHandler/
// handleApiError only extract the upstream status when the thrown
// Error's message is shaped like "API error (XXX): <body>" — a plain
// Error rendered every backend 4xx as a 500, hiding feature-disabled
// (403), missing-photo (404), and similar caller-shaped errors from the
// frontend. The handler now formats the error envelope correctly; these
// tests pin that contract for both the DELETE branch and (cross-checked
// by inspection) the POST branch which uses the identical pattern.
describe("DELETE /api/students/[id]/photo — backend status preservation", () => {
  it.each([
    [403, "feature disabled"],
    [404, "no photo"],
    [409, "consent withdrawn"],
  ])(
    "surfaces a backend %d as the same status code (not 500)",
    async (status, body) => {
      mockAuth.mockResolvedValue(defaultSession);
      mockFetch.mockResolvedValueOnce(jsonResponse(status, { error: body }));

      const { DELETE } = await import("./route");
      const response = await DELETE(createDeleteRequest("42"), {
        params: Promise.resolve({ id: "42" }),
      });

      expect(response.status).toBe(status);
    },
  );

  it("uses the German fallback when the backend body is empty", async () => {
    // Empty body branch: handler must still format "API error (XXX): <fallback>"
    // so handleApiError preserves the upstream status. Without the fallback
    // the user would see "API error (500): " with a trailing colon.
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(new Response("", { status: 502 }));

    const { DELETE } = await import("./route");
    const response = await DELETE(createDeleteRequest("42"), {
      params: Promise.resolve({ id: "42" }),
    });

    expect(response.status).toBe(502);
  });
});

// POST handler covers a different surface: file validation (magic bytes,
// missing field), the multipart relay to the backend, error status
// propagation, and the consent_acknowledged passthrough. createFile-
// UploadHandler runs MIME/extension/size checks itself before our handler
// callback gets to run, so MIME/extension cases are validated in
// file-upload-wrapper.test.ts; here we exercise the route-specific
// branches that live inside the POST callback.

// Helper builds a multipart NextRequest because NextRequest's Request
// constructor doesn't fully implement formData in the test runtime.
function createUploadRequest(
  studentId: string,
  options: {
    photo?: File | null;
    consentAcknowledged?: boolean;
  } = {},
): NextRequest {
  const fd = new FormData();
  if (options.photo !== null) {
    if (options.photo) {
      fd.append("photo", options.photo);
    }
  }
  if (options.consentAcknowledged) {
    fd.append("consent_acknowledged", "true");
  }
  // Construct the underlying Request with the FormData body, then wrap
  // in NextRequest so request.formData() works inside the handler.
  const baseRequest = new Request(
    `http://localhost:3000/api/students/${studentId}/photo`,
    { method: "POST", body: fd },
  );
  return new NextRequest(baseRequest);
}

// 4-byte JPEG SOI prefix — passes validateImageMagicBytes.
const JPEG_BYTES = new Uint8Array([0xff, 0xd8, 0xff, 0xe0, 0, 0, 0, 0]);
// Random bytes — fails validateImageMagicBytes for any image format.
const NON_IMAGE_BYTES = new Uint8Array([0x00, 0x01, 0x02, 0x03, 0, 0, 0, 0]);

describe("POST /api/students/[id]/photo — body callback branches", () => {
  it("rejects with 415 when magic bytes don't match any allowed image", async () => {
    mockAuth.mockResolvedValue(defaultSession);

    // A real .jpg extension + image/jpeg MIME passes file-upload-wrapper's
    // outer validation, but the body callback's magic-byte check rejects
    // it because the actual bytes are not a JPEG header.
    const fakeFile = new File([NON_IMAGE_BYTES], "fake.jpg", {
      type: "image/jpeg",
    });

    const { POST } = await import("./route");
    const response = await POST(
      createUploadRequest("42", { photo: fakeFile }),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(415);
    // Backend must never have been called — magic-byte check fails fast.
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("relays photo to backend and returns the photo_url envelope", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(
      jsonResponse(200, {
        status: "success",
        data: { photo_url: "/uploads/student-photos/42_abc.jpg" },
      }),
    );

    const realFile = new File([JPEG_BYTES], "real.jpg", {
      type: "image/jpeg",
    });

    const { POST } = await import("./route");
    const response = await POST(
      createUploadRequest("42", {
        photo: realFile,
        consentAcknowledged: true,
      }),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [calledUrl, calledOptions] = mockFetch.mock.calls[0] as [
      string,
      RequestInit,
    ];
    expect(calledUrl).toBe("http://backend.test/api/students/42/photo");
    expect(calledOptions.method).toBe("POST");
    // Body forwarded as FormData with the validated photo + consent flag.
    const body = calledOptions.body as FormData;
    expect(body).toBeInstanceOf(FormData);
    expect(body.get("photo")).toBeInstanceOf(File);
    expect(body.get("consent_acknowledged")).toBe("true");
  });

  it("retries upload with refreshed token on 401, then succeeds", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockUncachedAuth.mockResolvedValue(refreshedSession);
    mockFetch
      .mockResolvedValueOnce(jsonResponse(401, { error: "expired" }))
      .mockResolvedValueOnce(
        jsonResponse(200, {
          status: "success",
          data: { photo_url: "/uploads/x.jpg" },
        }),
      );

    const realFile = new File([JPEG_BYTES], "real.jpg", {
      type: "image/jpeg",
    });

    const { POST } = await import("./route");
    const response = await POST(
      createUploadRequest("42", { photo: realFile }),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    // Second call uses the refreshed token.
    const secondCallHeaders = (
      mockFetch.mock.calls[1]?.[1] as RequestInit | undefined
    )?.headers as Record<string, string> | undefined;
    expect(secondCallHeaders?.Authorization).toBe("Bearer fresh-token");
  });

  it.each([
    [400, "missing consent"],
    [403, "feature disabled"],
    [409, "consent withdrawn"],
  ])(
    "surfaces backend %d as the same status code (not 500)",
    async (status, body) => {
      mockAuth.mockResolvedValue(defaultSession);
      mockFetch.mockResolvedValueOnce(jsonResponse(status, { error: body }));

      const realFile = new File([JPEG_BYTES], "real.jpg", {
        type: "image/jpeg",
      });

      const { POST } = await import("./route");
      const response = await POST(
        createUploadRequest("42", { photo: realFile }),
        { params: Promise.resolve({ id: "42" }) },
      );

      expect(response.status).toBe(status);
    },
  );

  it("returns 500 when backend response lacks photo_url (broken contract)", async () => {
    mockAuth.mockResolvedValue(defaultSession);
    mockFetch.mockResolvedValueOnce(
      // Backend returned 200 but no data.photo_url — this should be
      // surfaced as a 500 because the upstream contract is broken.
      jsonResponse(200, { status: "success", data: {} }),
    );

    const realFile = new File([JPEG_BYTES], "real.jpg", {
      type: "image/jpeg",
    });

    const { POST } = await import("./route");
    const response = await POST(
      createUploadRequest("42", { photo: realFile }),
      { params: Promise.resolve({ id: "42" }) },
    );

    expect(response.status).toBe(500);
  });
});
