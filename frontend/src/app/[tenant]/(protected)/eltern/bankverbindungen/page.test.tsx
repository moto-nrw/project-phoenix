import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  within,
} from "@testing-library/react";

import type { PaymentOverviewRow } from "~/lib/guardian-payment-api";

const mockFetchOverview = vi.fn();
const mockExport = vi.fn();
vi.mock("~/lib/guardian-payment-api", () => ({
  fetchPaymentOverview: () => mockFetchOverview(),
  exportPaymentOverview: (format: string) => mockExport(format),
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

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

import BankverbindungenPage from "./page";

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
  const table = () => within(screen.getByTestId("payment-list-table"));
  const stacked = () => within(screen.getByTestId("payment-list-stacked"));

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

    fireEvent.click(screen.getByRole("button", { name: "PDF" }));
    fireEvent.click(screen.getByRole("button", { name: /Herunterladen/ }));

    await waitFor(() => expect(mockExport).toHaveBeenCalledWith("pdf"));
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

    expect(stacked().queryByText("Kind 25")).not.toBeInTheDocument();

    fireEvent.click(stacked().getByRole("button", { name: /Weitere 5/ }));

    expect(stacked().getByText("Kind 25")).toBeInTheDocument();
  });

  it("warns that the downloaded file carries full IBANs", async () => {
    render(<BankverbindungenPage />);

    await waitFor(() =>
      expect(screen.getByText(/enthält die ganzen IBANs/)).toBeInTheDocument(),
    );
  });
});
