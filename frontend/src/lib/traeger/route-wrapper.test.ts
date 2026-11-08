import { beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";

const { mockTraegerAuth, mockUncachedAuth, mockFetch } = vi.hoisted(() => ({
  mockTraegerAuth: vi.fn(),
  mockUncachedAuth: vi.fn(),
  mockFetch: vi.fn(),
}));

vi.mock("~/server/auth/traeger", async () => ({
  traegerPortal: (await import("~/test/portal-binding")).testPortalBinding(
    mockTraegerAuth,
    mockUncachedAuth,
  ),
}));
vi.mock("~/lib/server-api-url", () => ({
  getServerApiUrl: () => "http://localhost:8080",
}));
global.fetch = mockFetch as typeof fetch;

import { proxyFile, proxyUpload } from "./route-wrapper.server";

// The factory itself is tested for every portal in
// lib/portal-handlers.server.test.ts; here only the Träger defaults and the
// upload route, which only the Träger portal uses.

const context = { params: Promise.resolve({}) };

const LOGO = { tooLargeCode: "traeger.logo_too_large" } as const;

describe("Träger file proxy defaults (#4655)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  function exportRequest() {
    return new NextRequest(
      "http://localhost:3000/api/traeger/access-log/export?from=2026-09-01&to=2026-09-09",
    );
  }

  it("streams the file with its name and the query forwarded", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response("﻿Datum;Uhrzeit\n", {
        status: 200,
        headers: {
          "Content-Type": "text/csv; charset=utf-8",
          "Content-Disposition": 'attachment; filename="zugriffsprotokoll.csv"',
        },
      }),
    );

    const response = await proxyFile("/traeger/access-log/export")(
      exportRequest(),
      context,
    );

    expect(response.status).toBe(200);
    expect(response.headers.get("content-type")).toBe(
      "text/csv; charset=utf-8",
    );
    expect(response.headers.get("content-disposition")).toContain(
      "zugriffsprotokoll.csv",
    );
    expect(response.headers.get("cache-control")).toBe("no-store");
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/traeger/access-log/export?from=2026-09-01&to=2026-09-09",
      expect.objectContaining({
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });

  it("keeps the backend's cache header for a versioned image on request", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(new Uint8Array([0x89, 0x50]), {
        status: 200,
        headers: {
          "Content-Type": "image/png",
          "Cache-Control": "private, max-age=31536000, immutable",
        },
      }),
    );

    const response = await proxyFile("/traeger/master-data/logo", {
      backendCacheControl: true,
    })(
      new NextRequest(
        "http://localhost:3000/api/traeger/master-data/logo?v=2026-11-05T00%3A00%3A00Z",
      ),
      context,
    );

    expect(response.headers.get("cache-control")).toBe(
      "private, max-age=31536000, immutable",
    );
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/traeger/master-data/logo?v=2026-11-05T00%3A00%3A00Z",
      expect.anything(),
    );
  });

  function previewRequest(body: string) {
    return new NextRequest(
      "http://localhost:3000/api/traeger/billing-runs/7/letters/preview",
      { method: "POST", body, headers: { "Content-Type": "application/json" } },
    );
  }

  it("refuses a body over the limit before calling the backend", async () => {
    const response = await proxyFile(
      "/traeger/billing-runs/7/letters/preview",
      {
        method: "POST",
      },
    )(previewRequest("x".repeat(64 * 1024 + 1)), context);

    expect(response.status).toBe(413);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});

describe("Träger upload proxy (#4653)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockTraegerAuth.mockResolvedValue({ user: { token: "org-token" } });
  });

  function upload(body: BodyInit, headers: Record<string, string>) {
    return new NextRequest(
      "http://localhost:3000/api/traeger/master-data/logo",
      {
        method: "PUT",
        body,
        headers,
      },
    );
  }

  it("uploads a period PDF with POST and encoded route parameters", async () => {
    mockFetch.mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { id: "81" } }), {
        status: 201,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const form = new FormData();
    form.append(
      "document",
      new Blob(["%PDF-1.4"], { type: "application/pdf" }),
      "Hinweis.pdf",
    );
    const request = new NextRequest(
      "http://localhost:3000/api/traeger/holiday-periods/5/form/documents",
      { method: "POST", body: form },
    );
    const response = await proxyUpload(
      (params) =>
        `/traeger/holiday-periods/${encodeURIComponent(String(params.id))}/form/documents`,
      1024,
      { method: "POST", tooLargeCode: "enrollment.legal_document_invalid" },
    )(request, { params: Promise.resolve({ id: "5/1" }) });
    expect(response.status).toBe(201);
    expect(mockFetch).toHaveBeenCalledWith(
      "http://localhost:8080/traeger/holiday-periods/5%2F1/form/documents",
      expect.objectContaining({
        method: "POST",
        headers: expect.objectContaining({ Authorization: "Bearer org-token" }),
      }),
    );
  });
  it("forwards the multipart body once more after a refreshed session", async () => {
    const form = new FormData();
    form.append("logo", new File(["png"], "logo.png", { type: "image/png" }));
    const prepared = new Request("http://localhost", {
      method: "PUT",
      body: form,
    });
    const contentType = prepared.headers.get("content-type")!;
    const bytes = await prepared.arrayBuffer();
    mockFetch
      .mockResolvedValueOnce(new Response(null, { status: 401 }))
      .mockResolvedValueOnce(
        Response.json({ status: "success", data: { width: 1 } }),
      );
    mockUncachedAuth.mockResolvedValue({ user: { token: "fresh-token" } });

    const response = await proxyUpload(
      "/traeger/master-data/logo",
      1024,
      LOGO,
    )(upload(bytes, { "content-type": contentType }), context);

    expect(response.status).toBe(200);
    expect(mockFetch).toHaveBeenCalledTimes(2);
    const [url, init] = mockFetch.mock.calls[1]!;
    expect(url).toBe("http://localhost:8080/traeger/master-data/logo");
    expect(init).toMatchObject({
      method: "PUT",
      headers: expect.objectContaining({
        Authorization: "Bearer fresh-token",
        "Content-Type": contentType,
      }),
    });
    expect(new Uint8Array(init.body as ArrayBuffer)).toEqual(
      new Uint8Array(bytes),
    );
  });

  it("refuses an oversized or non-multipart body without calling the backend", async () => {
    const tooLarge = await proxyUpload(
      "/traeger/master-data/logo",
      10,
      LOGO,
    )(
      upload(new Uint8Array(70 * 1024), {
        "content-type": "multipart/form-data; boundary=x",
      }),
      context,
    );
    expect(tooLarge.status).toBe(400);
    expect(await tooLarge.json()).toMatchObject({
      code: "traeger.logo_too_large",
    });

    // Without a Content-Length the proxy stops reading past the limit.
    const stream = new ReadableStream<Uint8Array>({
      pull(controller) {
        controller.enqueue(new Uint8Array(32 * 1024));
      },
    });
    const streamed = await proxyUpload(
      "/traeger/master-data/logo",
      10,
      LOGO,
    )(
      new NextRequest("http://localhost:3000/api/traeger/master-data/logo", {
        method: "PUT",
        body: stream,
        headers: { "content-type": "multipart/form-data; boundary=x" },
        duplex: "half",
      } as ConstructorParameters<typeof NextRequest>[1]),
      context,
    );
    expect(streamed.status).toBe(400);
    expect(await streamed.json()).toMatchObject({
      code: "traeger.logo_too_large",
    });

    const proof = await proxyUpload("/traeger/proofs", 10, {
      method: "POST",
      tooLargeCode: "enrollment.upload_invalid",
      tooLargeDetails: { max_mb: 10 },
    })(
      upload(new Uint8Array(70 * 1024), {
        "content-type": "multipart/form-data; boundary=x",
      }),
      context,
    );
    expect(proof.status).toBe(400);
    expect(await proof.json()).toMatchObject({
      code: "enrollment.upload_invalid",
      details: { max_mb: 10 },
    });

    const notMultipart = await proxyUpload(
      "/traeger/master-data/logo",
      1024,
      LOGO,
    )(upload("{}", { "content-type": "application/json" }), context);
    expect(notMultipart.status).toBe(400);
    expect(mockFetch).not.toHaveBeenCalled();
  });
});
