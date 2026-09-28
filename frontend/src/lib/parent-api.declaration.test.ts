import { afterEach, describe, expect, it, vi } from "vitest";

import { ParentApiError, downloadDeclarationProofPdf } from "./parent-api";

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

describe("downloadDeclarationProofPdf (#3430)", () => {
  it("loads the PDF through the parent proxy and saves it under the backend's name", async () => {
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

    await downloadDeclarationProofPdf("42", "5");

    expect(fetchMock).toHaveBeenCalledWith(
      "/api/parent/me/news/42/declaration/proof/pdf?student_id=5",
      { method: "GET" },
    );
    expect(downloadBlobMock).toHaveBeenCalledWith(
      expect.any(Blob),
      "nachweis-erklaerung.pdf",
    );
  });

  it("throws a ParentApiError instead of saving an error body", async () => {
    mockFetch(
      new Response(
        JSON.stringify({
          status: "error",
          error: "not found",
          code: "not_found",
        }),
        { status: 404, headers: { "Content-Type": "application/json" } },
      ),
    );

    const error = await downloadDeclarationProofPdf("42", "5").catch(
      (err: unknown) => err,
    );
    expect(error).toBeInstanceOf(ParentApiError);
    expect(error).toMatchObject({ status: 404, code: "not_found" });
    expect(downloadBlobMock).not.toHaveBeenCalled();
  });
});
