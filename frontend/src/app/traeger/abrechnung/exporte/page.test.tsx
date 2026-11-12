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
  fetchDatevRunStates: vi.fn(),
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
  api.fetchDatevRunStates.mockResolvedValue({ ready: false, runs: new Map() });
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
    datevAll: false,
    resolvedPositions: 0,
    costCenterFallbacks: [],
    exportedByYou: true,
    exportedByAccount: true,
    exportedByName: "",
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
    screen.getByText("Standard-Erlöskonto: passt nicht"),
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
    expect(api.downloadDatevBatch).toHaveBeenCalledWith("care-runs", "7", {}),
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
it.each(["care", "holiday"] as const)(
  "lists the invoices and correction steps of a refused %s DATEV download",
  async (runKind) => {
    const run: ExportRun =
      runKind === "holiday"
        ? {
            ...RUN,
            runKind,
            pathKind: "holiday-runs",
            label: "Herbstferien 2026",
          }
        : RUN;
    api.fetchExportRuns.mockResolvedValue({
      items: [run],
      total: 1,
      notExported: 1,
      nextOffset: null,
    });
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
            {
              document_number: "MB-2026-0002",
              field: "debtor_number",
              reason: "missing",
            },
            { document_number: 7, field: "debtor_number", reason: "missing" },
          ],
        },
      }),
    );
    renderPage();
    fireEvent.click(
      (
        await screen.findAllByRole("button", {
          name: `DATEV-Datei von ${run.label} herunterladen`,
        })
      )[0]!,
    );
    expect(
      await screen.findByText(`Keine DATEV-Datei für ${run.label}`),
    ).toBeInTheDocument();
    // One line per reason, with every invoice it names; the malformed entry
    // is left out.
    expect(
      screen.getByText("Debitorennummer fehlt: MB-2026-0001, MB-2026-0002"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Liegt in einem anderen Wirtschaftsjahr: ST-2026-0001"),
    ).toBeInTheDocument();
    expect(screen.getAllByRole("listitem")).toHaveLength(2);
    const correction =
      runKind === "holiday"
        ? "Bei Rechnungen ohne Gutschrift: Stornieren Sie die Rechnung. Korrigieren Sie die Debitorennummer in der Ferienanmeldung. Stellen Sie dann die Rechnung neu aus. Danach ist der Lauf wieder frei. Hat die Rechnung schon eine Gutschrift, ist kein Storno möglich. Laden Sie dann die Sollstellung für Ihr Steuerbüro herunter. Es kann die Buchungen daraus manuell in DATEV erfassen. Der DATEV-Download dieses Laufs bleibt gesperrt."
        : "Tragen Sie bei der Familie die richtige Debitorennummer ein. Stornieren Sie dann die Rechnung und stellen Sie sie neu aus. Danach ist der Lauf wieder frei.";
    expect(screen.getByText(/So geht es weiter:/)).toHaveTextContent(
      correction,
    );
    expect(files.downloadBlob).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Hinweis schließen" }));
    await waitFor(() =>
      expect(
        screen.queryByText(`Keine DATEV-Datei für ${run.label}`),
      ).toBeNull(),
    );
  },
);

const LAST_DATEV = {
  id: "12",
  fileNumber: 12,
  format: "datev" as const,
  runKind: "care" as const,
  runId: "7",
  runLabel: "Monatsbeiträge Oktober 2026",
  fileName: "EXTF_Buchungsstapel_12.csv",
  invoiceCount: 42,
  totalCents: 327600,
  reference: "",
  repeatOf: null,
  datevAll: false,
  resolvedPositions: 0,
  costCenterFallbacks: [],
  exportedByYou: true,
  exportedByAccount: true,
  exportedByName: "",
  exportedAt: "2026-11-09T10:00:00Z",
  submittedAt: null,
};

function datevStates(newBookings: number, fiscalYears: string[]) {
  return {
    ready: true,
    runs: new Map([["care/7", { newBookings, fiscalYears }]]),
  };
}

// The standard download holds only new bookings (#6433): the run names
// how many, and the button says it.
it("counts the new DATEV bookings of a run", async () => {
  api.fetchDatevRunStates.mockResolvedValue(datevStates(3, ["2026-01-01"]));
  renderPage();
  expect(
    (await screen.findAllByText("3 neue Buchungen")).length,
  ).toBeGreaterThan(0);
  expect(screen.getAllByText("Noch keine DATEV-Datei").length).toBeGreaterThan(
    0,
  );
  const button = screen.getAllByRole("button", {
    name: "3 neue Buchungen von Monatsbeiträge Oktober 2026 als DATEV-Datei herunterladen",
  })[0]!;
  expect(button).toBeEnabled();
  expect(api.fetchDatevRunStates).toHaveBeenCalledWith([RUN]);
  expect(
    screen.queryByRole("button", {
      name: "Weitere DATEV-Aktionen für Monatsbeiträge Oktober 2026",
    }),
  ).toBeNull();
});

// Without a new booking there is no file to take: the button is off and
// says why; „Alle Buchungen erneut“ warns before it repeats them, in the
// fiscal year the person picks.
it("repeats every DATEV booking only after the warning", async () => {
  api.fetchExportRuns.mockResolvedValue({
    items: [{ ...RUN, lastDatev: LAST_DATEV }],
    total: 1,
    notExported: 1,
    nextOffset: null,
  });
  api.fetchDatevRunStates.mockResolvedValue(
    datevStates(0, ["2026-01-01", "2027-01-01", "2028-01-01", "2029-01-01"]),
  );
  api.downloadDatevBatch.mockResolvedValue({
    blob: new Blob(["extf"]),
    filename: "EXTF_Buchungsstapel_13.csv",
  });
  renderPage();
  expect(
    (await screen.findAllByText("Alles exportiert")).length,
  ).toBeGreaterThan(0);
  const button = screen.getAllByRole("button", DATEV_BUTTON)[0]!;
  expect(button).toBeDisabled();
  const reason = button.getAttribute("aria-describedby");
  expect(reason).toBe("datev-state-care-runs-7");
  expect(document.getElementById(reason!)).toHaveTextContent(
    "Zuletzt Nr. 12 am 09.11.2026",
  );

  fireEvent.click(
    screen.getAllByRole("button", {
      name: "Weitere DATEV-Aktionen für Monatsbeiträge Oktober 2026",
    })[0]!,
  );
  fireEvent.click(
    await screen.findByRole("menuitem", { name: /Alle Buchungen erneut/ }),
  );
  expect(
    await screen.findByText("Alle Buchungen erneut herunterladen?"),
  ).toBeInTheDocument();
  expect(
    screen.getByText("Buchungen können doppelt in DATEV stehen"),
  ).toBeInTheDocument();
  expect(api.downloadDatevBatch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole("button", { name: "2029" }));
  fireEvent.click(screen.getByRole("button", { name: "Erneut herunterladen" }));
  await waitFor(() =>
    expect(api.downloadDatevBatch).toHaveBeenCalledWith("care-runs", "7", {
      all: true,
      fiscalYear: "2029-01-01",
    }),
  );
  expect(
    await screen.findByText(
      "EXTF_Buchungsstapel_13.csv ist heruntergeladen. Sie wiederholt alle Buchungen von Monatsbeiträge Oktober 2026.",
    ),
  ).toBeInTheDocument();
});

it("closes the repeat dialog when DATEV refuses a booking", async () => {
  api.fetchExportRuns.mockResolvedValue({
    items: [{ ...RUN, lastDatev: LAST_DATEV }],
    total: 1,
    notExported: 1,
    nextOffset: null,
  });
  api.fetchDatevRunStates.mockResolvedValue(datevStates(0, ["2026-01-01"]));
  api.downloadDatevBatch.mockRejectedValue(
    new ApiError("Request failed (409)", 409, {
      code: "parent_billing.datev_bookings_invalid",
      details: {
        documents: [
          {
            document_number: "MB-2026-0001",
            field: "debtor_number",
            reason: "invalid",
          },
        ],
      },
    }),
  );
  renderPage();
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "Weitere DATEV-Aktionen für Monatsbeiträge Oktober 2026",
      })
    )[0]!,
  );
  fireEvent.click(
    await screen.findByRole("menuitem", { name: /Alle Buchungen erneut/ }),
  );
  expect(await screen.findByRole("dialog")).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Erneut herunterladen" }));
  // Anker: Der abgelehnte Download nennt die betroffene Rechnung.
  expect(
    await screen.findByText("Debitorennummer passt nicht: MB-2026-0001"),
  ).toBeInTheDocument();
  expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  expect(api.downloadDatevBatch).toHaveBeenCalledWith("care-runs", "7", {
    all: true,
    fiscalYear: "2026-01-01",
  });
  expect(files.downloadBlob).not.toHaveBeenCalled();
});

// A download that found nothing new (another person took the bookings a
// moment earlier) is no fault: a notice says no file and no number.
it("says when a DATEV download found no new booking", async () => {
  api.fetchDatevRunStates.mockResolvedValue(datevStates(1, ["2026-01-01"]));
  api.downloadDatevBatch.mockRejectedValue(
    new ApiError("Request failed (409)", 409, {
      code: "parent_billing.datev_bookings_taken",
    }),
  );
  renderPage();
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "1 neue Buchung von Monatsbeiträge Oktober 2026 als DATEV-Datei herunterladen",
      })
    )[0]!,
  );
  expect(
    await screen.findByText(
      "Keine neuen Buchungen für Monatsbeiträge Oktober 2026",
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByText(
      /Es wurde keine weitere Datei erstellt und keine Nummer verbraucht\./,
    ),
  ).toBeInTheDocument();
  expect(files.downloadBlob).not.toHaveBeenCalled();
});

it("marks a DATEV file of all bookings again in the protocol", async () => {
  api.fetchExportProtocol.mockResolvedValue({
    items: [
      { ...LAST_DATEV, datevAll: true, repeatOf: 12, id: "13", fileNumber: 13 },
    ],
    total: 1,
    nextOffset: null,
  });
  renderPage();
  expect(
    (await screen.findAllByText("Alle erneut: Wiederholung")).length,
  ).toBeGreaterThan(0);
  expect(
    screen.getAllByText("Gleicher Inhalt wie Nr. 12").length,
  ).toBeGreaterThan(0);
});
