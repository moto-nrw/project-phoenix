import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { ExportRun } from "~/lib/traeger-finance-exports-api";
import TraegerFinanceExportsPage from "./page";

const api = vi.hoisted(() => ({
  fetchExportRuns: vi.fn(),
  fetchExportProtocol: vi.fn(),
  fetchExportSettings: vi.fn(),
  downloadSollstellung: vi.fn(),
  downloadDatevBatch: vi.fn(),
}));
const files = vi.hoisted(() => ({ downloadBlob: vi.fn() }));
vi.mock("~/lib/traeger-finance-exports-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...api,
}));
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
  lastDatev: null,
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
  api.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: { ready: true, missingSettings: [], invalidSettings: [] },
  });
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

function renderPage() {
  render(
    <ToastProvider>
      <TraegerFinanceExportsPage />
    </ToastProvider>,
  );
}

const DATEV_BUTTON = {
  name: "DATEV-Datei von Monatsbeiträge Oktober 2026 herunterladen",
};

// The DATEV file waits for the values of the tax office (#4694): the page
// names each missing or wrong value, links to the settings and keeps the
// button off, with the notice as its description.
it("names the missing DATEV values and keeps the DATEV download off", async () => {
  api.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: {
      ready: false,
      missingSettings: ["parent_billing.datev_consultant_number"],
      invalidSettings: ["parent_billing.datev_revenue_account"],
    },
  });
  renderPage();
  expect(
    await screen.findByText("Die DATEV-Datei fehlt noch"),
  ).toBeInTheDocument();
  expect(screen.getByText("Beraternummer: fehlt")).toBeInTheDocument();
  expect(
    screen.getByText("Erlöskonto für Elternbeiträge: passt nicht"),
  ).toBeInTheDocument();
  expect(
    screen.getByRole("link", { name: "DATEV-Werte eintragen" }),
  ).toHaveAttribute("href", "/traeger/einstellungen?bereich=datev");
  const button = screen.getAllByRole("button", DATEV_BUTTON)[0]!;
  expect(button).toBeDisabled();
  expect(button).toHaveAttribute("aria-describedby", "datev-readiness");
  expect(
    screen.getAllByRole("button", {
      name: "Sollstellung von Monatsbeiträge Oktober 2026 herunterladen",
    })[0],
  ).toBeEnabled();
});

it("downloads the DATEV file of a run once the values are complete", async () => {
  api.downloadDatevBatch.mockResolvedValue({
    blob: new Blob(["extf"]),
    filename: "EXTF_Buchungsstapel_110.csv",
  });
  renderPage();
  const button = (await screen.findAllByRole("button", DATEV_BUTTON))[0]!;
  expect(screen.queryByText("Die DATEV-Datei fehlt noch")).toBeNull();
  fireEvent.click(button);
  await waitFor(() =>
    expect(api.downloadDatevBatch).toHaveBeenCalledWith("care-runs", "7"),
  );
  await waitFor(() =>
    expect(files.downloadBlob).toHaveBeenCalledWith(
      expect.any(Blob),
      "EXTF_Buchungsstapel_110.csv",
    ),
  );
  expect(api.downloadSollstellung).not.toHaveBeenCalled();
});

// A refused download names the invoices DATEV would refuse and what to do.
it("lists the invoices a refused DATEV download names", async () => {
  api.downloadDatevBatch.mockRejectedValue(
    new ApiError("Request failed (409)", 409, {
      code: "parent_billing.datev_bookings_invalid",
      details: {
        documents: [
          {
            document_number: "MB-2026-0001",
            field: "debtor_number",
            reason: "missing",
          },
          {
            document_number: "ST-2026-0001",
            field: "document_date",
            reason: "out_of_period",
          },
          { document_number: 7, field: "debtor_number", reason: "missing" },
        ],
      },
    }),
  );
  renderPage();
  fireEvent.click((await screen.findAllByRole("button", DATEV_BUTTON))[0]!);
  expect(
    await screen.findByText(
      "Keine DATEV-Datei für Monatsbeiträge Oktober 2026",
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByText("MB-2026-0001: Debitorennummer fehlt"),
  ).toBeInTheDocument();
  expect(
    screen.getByText("ST-2026-0001: liegt in einem anderen Wirtschaftsjahr"),
  ).toBeInTheDocument();
  expect(screen.getAllByRole("listitem")).toHaveLength(2);
  expect(files.downloadBlob).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "Hinweis schließen" }));
  await waitFor(() =>
    expect(
      screen.queryByText("Keine DATEV-Datei für Monatsbeiträge Oktober 2026"),
    ).toBeNull(),
  );
});
