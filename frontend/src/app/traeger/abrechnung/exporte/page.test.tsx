import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { ExportRun } from "~/lib/traeger-finance-exports-api";
import TraegerFinanceExportsPage from "./page";

const api = vi.hoisted(() => ({
  fetchExportRuns: vi.fn(),
  fetchExportProtocol: vi.fn(),
  fetchExportSettings: vi.fn(),
  downloadSollstellung: vi.fn(),
}));
const files = vi.hoisted(() => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/traeger-finance-exports-api", () => api);
vi.mock("~/lib/file-download", () => files);
vi.mock("~/lib/traeger-url", () => ({ traegerPath: (path: string) => path }));

const RUN: ExportRun = {
  runKind: "care",
  pathKind: "care-runs",
  runId: "7",
  label: "Monatsbeiträge Oktober 2026",
  issuedOn: "2026-10-30",
  invoiceCount: 42,
  totalCents: 327600,
  lastSollstellung: null,
  exports: 0,
};

beforeEach(() => {
  vi.resetAllMocks();
  api.fetchExportRuns.mockResolvedValue([RUN]);
  api.fetchExportProtocol.mockResolvedValue([]);
  api.fetchExportSettings.mockResolvedValue({ nextFileNumber: 110 });
  api.downloadSollstellung.mockResolvedValue({
    blob: new Blob(["csv"]),
    filename: "csv-export-bewegungsdaten-110.csv",
  });
});

it("downloads the Sollstellung of a run and reads the protocol again", async () => {
  render(
    <ToastProvider>
      <TraegerFinanceExportsPage />
    </ToastProvider>,
  );
  expect(
    (await screen.findAllByText("Noch nicht exportiert")).length,
  ).toBeGreaterThan(0);
  fireEvent.click(
    screen.getAllByRole("button", {
      name: "Sollstellung von Monatsbeiträge Oktober 2026 herunterladen",
    })[0]!,
  );
  await waitFor(() =>
    expect(api.downloadSollstellung).toHaveBeenCalledWith("care-runs", "7"),
  );
  await waitFor(() =>
    expect(files.downloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      "csv-export-bewegungsdaten-110.csv",
    ),
  );
  await waitFor(() => expect(api.fetchExportProtocol).toHaveBeenCalledTimes(2));
});
