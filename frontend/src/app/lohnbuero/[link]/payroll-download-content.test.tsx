import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PayrollDownloadContent } from "./payroll-download-content";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { downloadMock } = vi.hoisted(() => ({ downloadMock: vi.fn() }));

vi.mock("~/lib/payroll-download-api", () => ({
  downloadPayrollFile: downloadMock,
}));

const LINK = "7.AbCdEfGhIjKlMnOpQrStUvWxYz_-0123456789abcd";

describe("PayrollDownloadContent (#4805)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("opens the link only on the button and confirms the download", async () => {
    downloadMock.mockResolvedValueOnce(undefined);

    render(<PayrollDownloadContent link={LINK} />);

    // Opening the page spends nothing; a mail scanner only sees this page.
    const button = screen.getByRole("button", { name: "Datei herunterladen" });
    expect(downloadMock).not.toHaveBeenCalled();

    fireEvent.click(button);

    expect(
      await screen.findByText(
        "Die Datei ist heruntergeladen. Der Link funktioniert jetzt nicht mehr.",
      ),
    ).toBeInTheDocument();
    expect(downloadMock).toHaveBeenCalledWith(LINK);
    expect(
      screen.queryByRole("button", { name: "Datei herunterladen" }),
    ).not.toBeInTheDocument();
  });

  it("shows the catalog text for a spent link and offers no retry", async () => {
    downloadMock.mockRejectedValueOnce(
      new ApiError("gone", 404, {
        code: "workforce.payroll_download_unavailable",
      }),
    );

    render(<PayrollDownloadContent link={LINK} />);
    fireEvent.click(screen.getByRole("button", { name: "Datei herunterladen" }));

    expect(
      await screen.findByText(
        catalogText("workforce.payroll_download_unavailable", "die Datei"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Datei herunterladen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
  });

  it("offers a retry when the download failed for another reason", async () => {
    downloadMock
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(undefined);

    render(<PayrollDownloadContent link={LINK} />);
    fireEvent.click(screen.getByRole("button", { name: "Datei herunterladen" }));

    expect(
      await screen.findByText(catalogText("general.unavailable", "die Datei")),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => {
      expect(downloadMock).toHaveBeenCalledTimes(2);
    });
  });
});
