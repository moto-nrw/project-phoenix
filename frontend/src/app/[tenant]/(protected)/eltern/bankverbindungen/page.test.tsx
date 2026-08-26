import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";

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
      expect(screen.getByText("Mia Schneider")).toBeInTheDocument(),
    );
    expect(screen.getByText("Nicht zugeordnet")).toBeInTheDocument();
    expect(screen.getByText("Fehlt")).toBeInTheDocument();
    expect(screen.getByText("Ohne IBAN (1)")).toBeInTheDocument();
  });

  it("narrows the list to the children still missing an IBAN", async () => {
    mockFetchOverview.mockResolvedValue([
      row({}),
      row({ studentId: "2", studentName: "Lea Wolf", ibanMasked: "" }),
    ]);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(screen.getByText("Mia Schneider")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "Ohne IBAN (1)" }));

    expect(screen.queryByText("Mia Schneider")).not.toBeInTheDocument();
    expect(screen.getByText("Lea Wolf")).toBeInTheDocument();
  });

  it("downloads in the chosen format", async () => {
    mockFetchOverview.mockResolvedValue([row({})]);
    mockExport.mockResolvedValue(undefined);

    render(<BankverbindungenPage />);
    await waitFor(() =>
      expect(screen.getByText("Mia Schneider")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByRole("button", { name: "PDF" }));
    fireEvent.click(
      screen.getByRole("button", { name: /Liste herunterladen/ }),
    );

    await waitFor(() => expect(mockExport).toHaveBeenCalledWith("pdf"));
  });

  it("warns that the downloaded file carries full IBANs", async () => {
    render(<BankverbindungenPage />);

    await waitFor(() =>
      expect(screen.getByText(/enthält die ganzen IBANs/)).toBeInTheDocument(),
    );
  });
});
