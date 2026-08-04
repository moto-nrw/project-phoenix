// Regression for the DOCX MIME gate. A .docx is an OOXML ZIP container, so
// browsers without the OOXML mapping report File.type as "application/zip"
// (or "application/x-zip-compressed"). The proxy's allowedMimeTypes used to
// list only the OOXML type, so those uploads died with 415 in Next.js before
// the backend — which validates the actual OOXML parts — ever saw them.
// These tests pin that the ZIP labels pass the proxy while the extension
// gate still rejects a bare .zip.

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
  mockAuth.mockResolvedValue({
    user: { id: "1", token: "test-token", name: "Test User" },
    expires: "2099-01-01",
  });
});

afterAll(() => {
  global.fetch = originalFetch;
});

// Minimal ZIP local-file-header prefix — the proxy never inspects bytes, the
// backend does, so the payload only has to be non-empty.
const ZIP_BYTES = new Uint8Array([0x50, 0x4b, 0x03, 0x04, 0, 0, 0, 0]);

function createUploadRequest(file: File, category = "sonstiges"): NextRequest {
  const fd = new FormData();
  fd.append("file", file);
  fd.append("category", category);
  return new NextRequest(
    new Request("http://localhost:3000/api/staff/42/documents", {
      method: "POST",
      body: fd,
    }),
  );
}

function successResponse(): Response {
  return new Response(
    JSON.stringify({
      status: "success",
      data: { id: 7, category: "sonstiges" },
    }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
}

describe("POST /api/staff/[id]/documents — DOCX MIME labels", () => {
  it.each([
    ["application/zip"],
    ["application/x-zip-compressed"],
    ["application/vnd.openxmlformats-officedocument.wordprocessingml.document"],
    [""],
  ])("forwards a .docx labelled %s to the backend", async (mimeType) => {
    mockFetch.mockResolvedValueOnce(successResponse());

    const file = new File([ZIP_BYTES], "vertrag.docx", { type: mimeType });

    const { POST } = await import("./route");
    const response = await POST(createUploadRequest(file), {
      params: Promise.resolve({ id: "42" }),
    });

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(1);
    const [calledUrl] = mockFetch.mock.calls[0] as [string, RequestInit];
    expect(calledUrl).toBe("http://backend.test/api/staff/42/documents");
  });

  it("still rejects a bare .zip before it reaches the backend", async () => {
    const file = new File([ZIP_BYTES], "archiv.zip", {
      type: "application/zip",
    });

    const { POST } = await import("./route");
    const response = await POST(createUploadRequest(file), {
      params: Promise.resolve({ id: "42" }),
    });

    expect(response.status).toBe(415);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
