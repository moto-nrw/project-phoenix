import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import {
  deleteAnnouncement,
  downloadDeclarationExport,
  fetchDeclarationStatus,
  isDeclaration,
  type Announcement,
} from "./parent-announcements-api";

const downloadBlobMock = vi.hoisted(() => vi.fn());
vi.mock("~/lib/file-download", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/file-download")>()),
  downloadBlob: downloadBlobMock,
}));

const originalFetch = globalThis.fetch;

afterEach(() => {
  globalThis.fetch = originalFetch;
  vi.clearAllMocks();
});

function mockFetch(response: Response) {
  const fn = vi.fn(() => Promise.resolve(response));
  globalThis.fetch = fn as unknown as typeof globalThis.fetch;
  return fn;
}

describe("Einverständnisse in the staff client (#3430)", () => {
  it("recognizes an Einverständnis by its delivery mode", () => {
    expect(
      isDeclaration({ delivery_mode: "declaration" } as Announcement),
    ).toBe(true);
    expect(isDeclaration({ delivery_mode: "letter" } as Announcement)).toBe(
      false,
    );
  });

  it("loads the status from the proxy route", async () => {
    const fetchMock = mockFetch(
      new Response(
        JSON.stringify({ status: "success", data: { kind: "consent" } }),
        { status: 200, headers: { "Content-Type": "application/json" } },
      ),
    );

    await expect(fetchDeclarationStatus("42")).resolves.toEqual({
      kind: "consent",
    });
    expect(fetchMock).toHaveBeenCalledWith(
      "/api/parent-announcements/42/declaration-status",
      undefined,
    );
  });

  it("carries the backend code on a refused delete", async () => {
    mockFetch(
      new Response(
        JSON.stringify({
          status: "error",
          error: "declaration has submissions",
          code: "declaration_has_submissions",
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );

    const error = await deleteAnnouncement("42").catch((err: unknown) => err);
    expect(error).toBeInstanceOf(ApiError);
    expect(error).toMatchObject({
      status: 409,
      code: "declaration_has_submissions",
      message: "declaration has submissions",
    });
  });

  it("downloads an export under the backend's file name", async () => {
    const fetchMock = mockFetch(
      new Response("a;b", {
        status: 200,
        headers: {
          "Content-Type": "text/csv",
          "Content-Disposition": 'attachment; filename="erklaerung.csv"',
        },
      }),
    );

    await downloadDeclarationExport("42", "csv");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/parent-announcements/42/declaration-export?format=csv",
    );
    expect(downloadBlobMock).toHaveBeenCalledWith(
      expect.any(Blob),
      "erklaerung.csv",
    );
  });

  it("downloads the report as the backend's PDF", async () => {
    const fetchMock = mockFetch(
      new Response("%PDF-1.7", {
        status: 200,
        headers: {
          "Content-Type": "application/pdf",
          "Content-Disposition":
            'attachment; filename="nachweis-erklaerung.pdf"',
        },
      }),
    );

    await downloadDeclarationExport("42", "pdf");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/parent-announcements/42/declaration-export?format=pdf",
    );
    expect(downloadBlobMock).toHaveBeenCalledWith(
      expect.any(Blob),
      "nachweis-erklaerung.pdf",
    );
  });

  it("throws instead of saving an error body as a file", async () => {
    mockFetch(
      new Response(JSON.stringify({ error: "not found" }), {
        status: 404,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(downloadDeclarationExport("42", "csv")).rejects.toThrow(
      "not found",
    );
    expect(downloadBlobMock).not.toHaveBeenCalled();
  });
});
