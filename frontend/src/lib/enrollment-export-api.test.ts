import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { exportPhaseRegistrations } from "./enrollment-export-api";

// exportPhaseRegistrations POSTs to the Next.js proxy and streams the
// returned blob to a browser download. These tests stub fetch + the DOM
// download primitives to assert the request shape, the filename source,
// and the error contract — no network, no real DOM navigation.

describe("exportPhaseRegistrations", () => {
  let clickSpy: ReturnType<typeof vi.fn>;
  let anchor: {
    href: string;
    download: string;
    click: ReturnType<typeof vi.fn>;
    remove: ReturnType<typeof vi.fn>;
  };

  beforeEach(() => {
    clickSpy = vi.fn();
    anchor = { href: "", download: "", click: clickSpy, remove: vi.fn() };
    vi.spyOn(document, "createElement").mockReturnValue(
      anchor as unknown as HTMLAnchorElement,
    );
    vi.spyOn(document.body, "append").mockImplementation(() => undefined);
    // jsdom has no object-URL implementation.
    URL.createObjectURL = vi.fn(() => "blob:fake");
    URL.revokeObjectURL = vi.fn();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  function mockFetchOk(
    disposition: string | null,
    body = "PDFBYTES",
  ): ReturnType<typeof vi.fn> {
    const headers = new Headers();
    if (disposition) headers.set("content-disposition", disposition);
    const fn = vi.fn(
      async () =>
        ({
          ok: true,
          headers,
          blob: async () => new Blob([body]),
        }) as unknown as Response,
    );
    global.fetch = fn as unknown as typeof fetch;
    return fn;
  }

  it("posts the format to the proxy and downloads with the server filename", async () => {
    const fetchFn = mockFetchOk(
      'attachment; filename="Anmeldungen Phase A.pdf"',
    );

    await exportPhaseRegistrations("123", "pdf");

    expect(fetchFn).toHaveBeenCalledTimes(1);
    const [url, init] = fetchFn.mock.calls[0]!;
    expect(url).toBe("/api/enrollment/phases/123/export");
    expect(init).toMatchObject({ method: "POST" });
    expect(JSON.parse((init as RequestInit).body as string)).toEqual({
      format: "pdf",
    });
    // Filename comes from the Content-Disposition header.
    expect(anchor.download).toBe("Anmeldungen Phase A.pdf");
    expect(clickSpy).toHaveBeenCalledTimes(1);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith("blob:fake");
  });

  it("includes child_status in the body only when given", async () => {
    const fetchFn = mockFetchOk(null);

    await exportPhaseRegistrations("7", "xlsx", "approved");
    expect(
      JSON.parse((fetchFn.mock.calls[0]![1] as RequestInit).body as string),
    ).toEqual({
      format: "xlsx",
      child_status: "approved",
    });

    fetchFn.mockClear();
    await exportPhaseRegistrations("7", "xlsx");
    expect(
      JSON.parse((fetchFn.mock.calls[0]![1] as RequestInit).body as string),
    ).toEqual({
      format: "xlsx",
    });
  });

  it("falls back to a default filename when no disposition is present", async () => {
    mockFetchOk(null);

    await exportPhaseRegistrations("9", "xlsx");

    expect(anchor.download).toBe("anmeldungen.xlsx");
  });

  it("posts docx exports and uses a docx fallback filename", async () => {
    const fetchFn = mockFetchOk(null, "DOCXBYTES");

    await exportPhaseRegistrations("9", "docx");

    expect(
      JSON.parse((fetchFn.mock.calls[0]![1] as RequestInit).body as string),
    ).toEqual({
      format: "docx",
    });
    expect(anchor.download).toBe("anmeldungen.docx");
  });

  it("throws the backend error text on a non-ok response", async () => {
    global.fetch = vi.fn(
      async () =>
        ({
          ok: false,
          text: async () => "phase has too many registrations",
        }) as unknown as Response,
    ) as unknown as typeof fetch;

    await expect(exportPhaseRegistrations("1", "pdf")).rejects.toThrow(
      "phase has too many registrations",
    );
    expect(clickSpy).not.toHaveBeenCalled();
  });
});
