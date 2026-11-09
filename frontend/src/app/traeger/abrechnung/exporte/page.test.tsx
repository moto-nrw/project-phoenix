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
  api.fetchExportRuns.mockResolvedValue({
    items: [RUN],
    total: 1,
    notExported: 1,
    nextOffset: null,
  });
  api.fetchExportProtocol.mockResolvedValue({
    items: [],
    total: 0,
    nextOffset: null,
  });
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

// Older runs stay reachable (review of 9d0709ae69): a list longer than one
// page offers the next page, the status line counts every run, and a
// reload after a download keeps the pages already open.
it("opens older runs page by page and keeps them after a download", async () => {
  const older: ExportRun = {
    ...RUN,
    runId: "3",
    label: "Monatsbeiträge März 2026",
  };
  api.fetchExportRuns.mockImplementation(async (offset = 0) =>
    offset === 0
      ? { items: [RUN], total: 2, notExported: 2, nextOffset: 1 }
      : { items: [older], total: 2, notExported: 2, nextOffset: null },
  );
  render(
    <ToastProvider>
      <TraegerFinanceExportsPage />
    </ToastProvider>,
  );
  expect(
    await screen.findByText("Sie sehen 1 von 2 Läufen."),
  ).toBeInTheDocument();
  expect(screen.getByText("Läufe").parentElement).toHaveTextContent("2Läufe");
  expect(
    screen.getByText("noch nicht exportiert").parentElement,
  ).toHaveTextContent("2noch nicht exportiert");
  expect(
    screen.queryAllByRole("button", {
      name: "Sollstellung von Monatsbeiträge März 2026 herunterladen",
    }),
  ).toHaveLength(0);

  fireEvent.click(screen.getByRole("button", { name: "Ältere anzeigen" }));
  const olderDownload = await screen.findAllByRole("button", {
    name: "Sollstellung von Monatsbeiträge März 2026 herunterladen",
  });
  expect(api.fetchExportRuns).toHaveBeenLastCalledWith(1);
  expect(
    screen.queryByText("Sie sehen 1 von 2 Läufen."),
  ).not.toBeInTheDocument();

  fireEvent.click(olderDownload[0]!);
  await waitFor(() =>
    expect(api.downloadSollstellung).toHaveBeenCalledWith("care-runs", "3"),
  );
  await waitFor(() => expect(api.fetchExportRuns).toHaveBeenCalledTimes(4));
  expect(
    await screen.findAllByRole("button", {
      name: "Sollstellung von Monatsbeiträge März 2026 herunterladen",
    }),
  ).not.toHaveLength(0);
});

it("offers older files of the protocol", async () => {
  const entry = {
    id: "9",
    fileNumber: 9,
    format: "sollstellung" as const,
    runKind: "care" as const,
    runId: "7",
    runLabel: "Monatsbeiträge Oktober 2026",
    fileName: "sollstellung-9.csv",
    invoiceCount: 2,
    totalCents: 0,
    reference: "",
    repeatOf: null,
    exportedByYou: true,
    exportedByAccount: true,
    exportedAt: "2026-11-09T10:00:00Z",
    submittedAt: null,
  };
  api.fetchExportProtocol.mockImplementation(async (offset = 0) =>
    offset === 0
      ? { items: [entry], total: 2, nextOffset: 1 }
      : {
          items: [
            {
              ...entry,
              id: "8",
              fileNumber: 8,
              fileName: "sollstellung-8.csv",
            },
          ],
          total: 2,
          nextOffset: null,
        },
  );
  render(
    <ToastProvider>
      <TraegerFinanceExportsPage />
    </ToastProvider>,
  );
  expect(
    await screen.findByText("Sie sehen 1 von 2 Dateien."),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Ältere anzeigen" }));
  expect(
    (await screen.findAllByText("sollstellung-8.csv")).length,
  ).toBeGreaterThan(0);
  expect(api.fetchExportProtocol).toHaveBeenLastCalledWith(1);
});
