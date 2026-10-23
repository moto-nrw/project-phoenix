import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  within,
} from "@testing-library/react";

import { ApiError } from "~/lib/api-error";
import type { PaymentOverviewRow } from "~/lib/guardian-payment-api";
import { catalogText } from "~/test/error-catalog-text";

const mockFetchOverview = vi.fn();
const mockExport = vi.fn();
vi.mock("~/lib/guardian-payment-api", () => ({
  fetchPaymentOverview: () => mockFetchOverview(),
  exportPaymentOverview: (...args: unknown[]) => mockExport(...args),
}));

let mockPermissions: string[] = ["guardians:financial"];
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: mockPermissions } },
    status: "authenticated",
  }),
}));

const mockPush = vi.fn();
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mockPush, replace: mockPush }),
}));

// Der Export-Dialog ist ein SlideOver; Vaul rendert in jsdom nichts.
vi.mock("vaul", () => import("~/test/mocks/vaul"));

// Load and export errors run through the real hooks (#2517, #4748).
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

import BankverbindungenPage from "./page";

function openExportDialog() {
  fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
  fireEvent.click(screen.getByRole("menuitem", { name: "Exportieren" }));
}

function row(overrides: Partial<PaymentOverviewRow>): PaymentOverviewRow {
  return {
    studentId: "1",
    studentName: "Mia Schneider",
    schoolClass: "1a",
    guardianId: "10",
    guardianName: "Sabine Schneider",
    relationshipType: "parent",
    accountHolder: "Sabine Schneider",
    ibanMasked: "•••• 3000",
    ...overrides,
  };
}

describe("BankverbindungenPage", () => {
  beforeEach(() => {
    mockPermissions = ["guardians:financial"];
    mockFetchOverview.mockReset();
    mockExport.mockReset();
    mockFetchOverview.mockResolvedValue([]);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("refuses the page without the bank permission", () => {
    mockPermissions = ["users:read", "users:update"];

    render(<BankverbindungenPage />);

    expect(
      screen.getByText("Kein Zugriff auf Bankverbindungen"),
    ).toBeInTheDocument();
    expect(mockFetchOverview).not.toHaveBeenCalled();
  });

  // jsdom applies no CSS, so the stacked and the table layout both render.
  // Scoping every row query keeps the assertions unambiguous.
  const table = () => within(screen.getByTestId("data-table-table"));
  const stacked = () => within(screen.getByTestId("data-table-stacked"));

  it("shows children without bank details as a gap, not as a blank cell", async () => {
    mockFetchOverview.mockResolvedValue([
      row({}),
      row({
        studentId: "2",
        studentName: "Lea Wolf",
        guardianId: null,
        guardianName: "",
        accountHolder: "",
        ibanMasked: "",
      }),
    ]);

    render(<BankverbindungenPage />);

    await waitFor(() =>
      expect(table().getByText("Mia Schneider")).toBeInTheDocument(),
    );
    expect(table().getByText("Nicht zugeordnet")).toBeInTheDocument();
    expect(table().getByText("Fehlt")).toBeInTheDocument();
    expect(screen.getByText("Ohne IBAN (1)")).toBeInTheDocument();
  });

  it("narrows the list to the children still missing an IBAN", async () => {
    mockFetchOverview.mockResolvedValue([
      row({}),
      row({ studentId: "2", studentName: "Lea Wolf", ibanMasked: "" }),
    ]);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(table().getByText("Mia Schneider")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Ohne IBAN (1)" }));

    expect(table().queryByText("Mia Schneider")).not.toBeInTheDocument();
    expect(table().getByText("Lea Wolf")).toBeInTheDocument();
    expect(stacked().getByText("Lea Wolf")).toBeInTheDocument();
  });

  it("downloads in the chosen format", async () => {
    mockFetchOverview.mockResolvedValue([row({})]);
    mockExport.mockResolvedValue(undefined);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(table().getByText("Mia Schneider")).toBeInTheDocument(),
    );

    openExportDialog();
    fireEvent.click(screen.getByRole("button", { name: "PDF" }));
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    await waitFor(() => expect(mockExport).toHaveBeenCalledWith("pdf"));
  });

  it("prints the PDF in the tab opened from the dialog", async () => {
    const target = {} as Window;
    vi.spyOn(globalThis, "open").mockReturnValue(target);
    mockFetchOverview.mockResolvedValue([row({})]);
    mockExport.mockResolvedValue(undefined);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(table().getByText("Mia Schneider")).toBeInTheDocument(),
    );
    openExportDialog();
    fireEvent.click(screen.getByRole("button", { name: "Drucken" }));

    await waitFor(() => expect(mockExport).toHaveBeenCalledWith("pdf", target));
  });

  // The defect this layout exists to fix: in the table the IBAN is a fourth
  // column that a phone viewport cannot show.
  it("carries the IBAN as a labelled field in the stacked layout", async () => {
    mockFetchOverview.mockResolvedValue([row({})]);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(stacked().getByText("Mia Schneider")).toBeInTheDocument(),
    );

    expect(stacked().getByText("IBAN")).toBeInTheDocument();
    expect(stacked().getByText("Kontoinhaber")).toBeInTheDocument();
    expect(stacked().getByText("•••• 3000")).toBeInTheDocument();
    expect(stacked().getByText("1a")).toBeInTheDocument();
  });

  it("reveals the rest of a long list on demand instead of rendering it all", async () => {
    mockFetchOverview.mockResolvedValue(
      Array.from({ length: 30 }, (_, i) =>
        row({ studentId: String(i), studentName: `Kind ${i}` }),
      ),
    );

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(stacked().getByText("Kind 0")).toBeInTheDocument(),
    );

    // Rows render in the table's sort order (by name), so the tail of the
    // first page is "Kind 4" — "Kind 9" is on the second.
    expect(stacked().queryByText("Kind 9")).not.toBeInTheDocument();

    fireEvent.click(
      stacked().getByRole("button", { name: "Mehr laden (25 von 30)" }),
    );

    expect(stacked().getByText("Kind 9")).toBeInTheDocument();
  });

  it("warns that the downloaded file carries full IBANs", async () => {
    render(<BankverbindungenPage />);

    await waitFor(() =>
      expect(screen.getByText(/enthält die ganzen IBANs/)).toBeInTheDocument(),
    );
  });

  // #2517: a failed load replaces the list, never "Noch keine Kinder".
  it("shows a failed load instead of the empty list and retries", async () => {
    mockFetchOverview
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce([row({})]);

    render(<BankverbindungenPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Bankverbindungen"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Kinder in der Liste"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() =>
      expect(table().getByText("Mia Schneider")).toBeInTheDocument(),
    );
  });

  // #4748: der Export läuft über den gemeinsamen Export-Dialog; ein Fehler
  // bleibt dort stehen, mit Katalogtext und Wiederholen.
  it("keeps a failed download in the export dialog", async () => {
    mockFetchOverview.mockResolvedValue([row({})]);
    mockExport
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce(undefined);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(table().getByText("Mia Schneider")).toBeInTheDocument(),
    );
    openExportDialog();
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));

    const message = catalogText(
      "general.server",
      "das Herunterladen der Bankverbindungen",
    );
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.getByRole("dialog")).toContainElement(
      screen.getByText(message),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockExport).toHaveBeenCalledTimes(2));
    expect(mockExport).toHaveBeenLastCalledWith("xlsx");
  });
});
