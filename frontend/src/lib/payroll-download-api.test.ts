import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "./api-error";
import { downloadPayrollFile } from "./payroll-download-api";

const { downloadBlobMock } = vi.hoisted(() => ({ downloadBlobMock: vi.fn() }));

vi.mock("./file-download", async (importOriginal) => ({
  ...(await importOriginal<typeof import("./file-download")>()),
  downloadBlob: downloadBlobMock,
}));

const fetchMock = vi.fn();
const LINK = "7.AbCdEfGhIjKlMnOpQrStUvWxYz_-0123456789abcd";

describe("downloadPayrollFile (#4805)", () => {
  beforeEach(() => vi.stubGlobal("fetch", fetchMock));
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.resetAllMocks();
  });

  it("posts the link and saves the file under the backend's plain name", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response("a;b", {
        headers: {
          "Content-Type": "text/csv",
          "Content-Disposition":
            "attachment; filename=zeitkonten_2026-09_korrektur-1.csv",
        },
      }),
    );

    await downloadPayrollFile(LINK);

    expect(fetchMock).toHaveBeenCalledWith(
      `/api/public/payroll-download/${LINK}`,
      { method: "POST", cache: "no-store" },
    );
    expect(downloadBlobMock).toHaveBeenCalledWith(
      expect.any(Blob),
      "zeitkonten_2026-09_korrektur-1.csv",
    );
  });

  it("throws the envelope's code for a spent link", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          status: 404,
          detail: "gone",
          code: "workforce.payroll_download_unavailable",
        }),
        {
          status: 404,
          headers: { "Content-Type": "application/problem+json" },
        },
      ),
    );

    const failure = downloadPayrollFile(LINK);
    await expect(failure).rejects.toBeInstanceOf(ApiError);
    await expect(failure).rejects.toMatchObject({
      code: "workforce.payroll_download_unavailable",
    });
    expect(downloadBlobMock).not.toHaveBeenCalled();
  });
});
