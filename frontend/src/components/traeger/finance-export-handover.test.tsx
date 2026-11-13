import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import TraegerFinanceExportsPage from "~/app/traeger/abrechnung/exporte/page";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type {
  ExportRecipient,
  ExportRecipientsPage,
} from "~/lib/traeger-export-handovers-api";
import type { ExportRun } from "~/lib/traeger-finance-exports-api";

// The handover of a run's file to the recipient of its format (#6824, ADR
// 0218; #6911, ADR 0221) on the export page: the menu „Übergeben“ per
// format, the result of a handover, the question before a second handover of
// the same content, and the way to the recipients when a file has none.

const exportsApi = vi.hoisted(() => ({
  fetchExportRuns: vi.fn(),
  fetchExportProtocol: vi.fn(),
  fetchExportSettings: vi.fn(),
  fetchDatevRunStates: vi.fn(),
}));
const handoverApi = vi.hoisted(() => ({
  fetchExportRecipients: vi.fn(),
  handOverRunFile: vi.fn(),
}));
vi.mock("~/lib/traeger-finance-exports-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...exportsApi,
}));
vi.mock("~/lib/traeger-export-handovers-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...handoverApi,
}));
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

const READY: ExportRecipient = {
  id: "7",
  name: "Kämmerei",
  enabled: true,
  ready: true,
  host: "dateien.kaemmerei-beispiel.de",
  port: 22,
  username: "moto",
  remoteDirectory: "/eingang",
  hostKeyFingerprint: "SHA256:AbCdEfGhIjKlMnOpQrStUvWxYz0123456789+/AbCdE",
  passwordSet: true,
  missingFields: [],
  formats: ["sollstellung", "datev", "pain008"],
  lastHandover: null,
  updatedAt: "2026-11-12T08:00:00Z",
};

/** Every format goes to the given recipient. */
function pageOf(...recipients: ExportRecipient[]): ExportRecipientsPage {
  return {
    recipients,
    assignments: (["sollstellung", "datev", "pain008"] as const).map(
      (format) => ({
        format,
        recipientId:
          recipients.find((recipient) => recipient.formats.includes(format))
            ?.id ?? null,
      }),
    ),
    handovers: [],
  };
}

beforeEach(() => {
  vi.resetAllMocks();
  exportsApi.fetchExportRuns.mockResolvedValue({
    items: [RUN],
    total: 1,
    notExported: 1,
    nextOffset: null,
  });
  exportsApi.fetchExportProtocol.mockResolvedValue({
    items: [],
    total: 0,
    nextOffset: null,
  });
  exportsApi.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: { ready: false, missingSettings: ["datev"], invalidSettings: [] },
  });
  exportsApi.fetchDatevRunStates.mockResolvedValue({
    ready: false,
    runs: new Map(),
  });
  handoverApi.fetchExportRecipients.mockResolvedValue(pageOf(READY));
});

function renderPage() {
  render(
    <ToastProvider>
      <TraegerFinanceExportsPage />
    </ToastProvider>,
  );
}

async function chooseSollstellung() {
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "Dateien von Monatsbeiträge Oktober 2026 an die Empfänger übergeben",
      })
    )[0]!,
  );
  fireEvent.click(
    screen.getByRole("menuitem", { name: /Sollstellung übergeben/ }),
  );
}

it("hands the Sollstellung over and names where it arrived", async () => {
  handoverApi.handOverRunFile.mockResolvedValue({
    fileName: "sollstellung-110.csv",
    state: "transferred",
    reason: "",
    recipientName: "Kämmerei",
    targetHost: "dateien.kaemmerei-beispiel.de",
    targetDirectory: "/eingang",
  });
  renderPage();
  await chooseSollstellung();
  await waitFor(() =>
    expect(handoverApi.handOverRunFile).toHaveBeenCalledWith(
      "sollstellung",
      "care-runs",
      "7",
      {},
    ),
  );
  expect(
    await screen.findByText(
      "sollstellung-110.csv ist bei Kämmerei angekommen.",
    ),
  ).toBeInTheDocument();
  await waitFor(() =>
    expect(exportsApi.fetchExportProtocol).toHaveBeenCalledTimes(2),
  );
});

it("shows a handover that did not arrive with its reason and a retry", async () => {
  handoverApi.handOverRunFile.mockResolvedValue({
    fileName: "sollstellung-110.csv",
    state: "failed",
    reason: "host_key_mismatch",
    targetHost: "dateien.kaemmerei-beispiel.de",
    targetDirectory: "/eingang",
  });
  renderPage();
  await chooseSollstellung();
  expect(
    await screen.findByText(
      "Sollstellung von Monatsbeiträge Oktober 2026 ist nicht angekommen",
    ),
  ).toBeInTheDocument();
  expect(
    screen.getByText(/Bitte prüfen Sie den Fingerabdruck beim Empfänger/),
  ).toBeInTheDocument();
  fireEvent.click(
    screen.getByRole("button", { name: "Noch einmal übergeben" }),
  );
  await waitFor(() =>
    expect(handoverApi.handOverRunFile).toHaveBeenCalledTimes(2),
  );
});

it("asks before the same file goes to the recipient again", async () => {
  handoverApi.handOverRunFile
    .mockRejectedValueOnce(
      new ApiError("conflict", 409, {
        code: "parent_billing.export_already_handed_over",
      }),
    )
    .mockResolvedValueOnce({
      fileName: "sollstellung-111.csv",
      state: "transferred",
      reason: "",
      targetHost: "dateien.kaemmerei-beispiel.de",
      targetDirectory: "/eingang",
    });
  renderPage();
  await chooseSollstellung();
  expect(
    await screen.findByText("Diese Datei hat der Empfänger schon"),
  ).toBeInTheDocument();
  fireEvent.click(
    screen.getByRole("button", { name: "Noch einmal übergeben" }),
  );
  await waitFor(() =>
    expect(handoverApi.handOverRunFile).toHaveBeenLastCalledWith(
      "sollstellung",
      "care-runs",
      "7",
      {
        repeat: true,
      },
    ),
  );
});

it("names the recipient of each file and leads to the recipients for a file without a ready one", async () => {
  exportsApi.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: { ready: true, missingSettings: [], invalidSettings: [] },
  });
  handoverApi.fetchExportRecipients.mockResolvedValue(
    pageOf(
      { ...READY, formats: ["sollstellung", "pain008"] },
      {
        ...READY,
        id: "8",
        name: "Steuerbüro",
        ready: false,
        missingFields: ["password"],
        formats: ["datev"],
      },
    ),
  );
  renderPage();
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "Dateien von Monatsbeiträge Oktober 2026 an die Empfänger übergeben",
      })
    )[0]!,
  );
  expect(
    screen.getByRole("menuitem", { name: /Sollstellung übergeben/ }),
  ).toHaveTextContent("Kämmerei");
  expect(
    screen.queryByRole("menuitem", { name: /DATEV-Datei übergeben/ }),
  ).not.toBeInTheDocument();
  const assign = screen.getByRole("menuitem", { name: /Empfänger zuordnen/ });
  expect(assign).toHaveAttribute(
    "href",
    "/traeger/abrechnung/exporte/empfaenger",
  );
  expect(assign).toHaveTextContent("Steuerbüro: Es fehlen noch Angaben.");
});

it("leads to the recipients while no file has a ready one and keeps the download", async () => {
  handoverApi.fetchExportRecipients.mockResolvedValue(
    pageOf({
      ...READY,
      ready: false,
      passwordSet: false,
      missingFields: ["password"],
    }),
  );
  renderPage();
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "Dateien von Monatsbeiträge Oktober 2026 an die Empfänger übergeben",
      })
    )[0]!,
  );
  expect(
    screen.getByRole("menuitem", { name: /Empfänger einrichten/ }),
  ).toHaveAttribute("href", "/traeger/abrechnung/exporte/empfaenger");
  expect(
    screen.queryByRole("menuitem", { name: /Sollstellung übergeben/ }),
  ).not.toBeInTheDocument();
  expect(
    screen.getAllByRole("button", {
      name: "Sollstellung von Monatsbeiträge Oktober 2026 herunterladen",
    }).length,
  ).toBeGreaterThan(0);
});

it("says that nothing is new when a DATEV handover has no new bookings", async () => {
  exportsApi.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: { ready: true, missingSettings: [], invalidSettings: [] },
  });
  handoverApi.handOverRunFile.mockRejectedValue(
    new ApiError("conflict", 409, {
      code: "parent_billing.datev_no_new_bookings",
    }),
  );
  renderPage();
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "Dateien von Monatsbeiträge Oktober 2026 an die Empfänger übergeben",
      })
    )[0]!,
  );
  fireEvent.click(
    screen.getByRole("menuitem", { name: /DATEV-Datei übergeben/ }),
  );
  expect(
    await screen.findByText(
      "Keine neuen Buchungen für Monatsbeiträge Oktober 2026",
    ),
  ).toBeInTheDocument();
  expect(handoverApi.handOverRunFile).toHaveBeenCalledWith(
    "datev",
    "care-runs",
    "7",
    {},
  );
});

it("hands every DATEV booking over again for a lost file, in the chosen fiscal year", async () => {
  exportsApi.fetchExportRuns.mockResolvedValue({
    items: [{ ...RUN, lastDatev: "2026-11-01T08:00:00Z" }],
    total: 1,
    notExported: 0,
    nextOffset: null,
  });
  exportsApi.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: { ready: true, missingSettings: [], invalidSettings: [] },
  });
  exportsApi.fetchDatevRunStates.mockResolvedValue({
    ready: true,
    runs: new Map([
      ["care/7", { newBookings: 0, fiscalYears: ["2026-01-01"] }],
    ]),
  });
  handoverApi.handOverRunFile.mockResolvedValue({
    fileName: "EXTF_Buchungsstapel-110.csv",
    state: "transferred",
    reason: "",
    targetHost: "dateien.kaemmerei-beispiel.de",
    targetDirectory: "/eingang",
  });
  renderPage();
  fireEvent.click(
    (
      await screen.findAllByRole("button", {
        name: "Dateien von Monatsbeiträge Oktober 2026 an die Empfänger übergeben",
      })
    )[0]!,
  );
  fireEvent.click(
    screen.getByRole("menuitem", {
      name: /Alle DATEV-Buchungen erneut übergeben/,
    }),
  );
  expect(
    await screen.findByText("Alle Buchungen erneut übergeben?"),
  ).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Erneut übergeben" }));
  await waitFor(() =>
    expect(handoverApi.handOverRunFile).toHaveBeenCalledWith(
      "datev",
      "care-runs",
      "7",
      { all: true, fiscalYear: "2026-01-01" },
    ),
  );
});

it("names the invoices DATEV would refuse when a DATEV handover makes no file", async () => {
  exportsApi.fetchExportSettings.mockResolvedValue({
    nextFileNumber: 110,
    datev: { ready: true, missingSettings: [], invalidSettings: [] },
  });
  handoverApi.handOverRunFile.mockRejectedValue(
    new ApiError("unprocessable", 422, {
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
        name: "Dateien von Monatsbeiträge Oktober 2026 an die Empfänger übergeben",
      })
    )[0]!,
  );
  fireEvent.click(
    screen.getByRole("menuitem", { name: /DATEV-Datei übergeben/ }),
  );
  expect(
    await screen.findByText("Debitorennummer passt nicht: MB-2026-0001"),
  ).toBeInTheDocument();
});
