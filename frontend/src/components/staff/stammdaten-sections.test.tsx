import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StammdatenTab } from "./stammdaten-tab";
import type { StaffStammdaten } from "~/lib/staff-api";

// Section rendering + permission gating of the Stammdaten tab (#1423). The
// SWR mock switches on the cache key so the aggregate, payroll and financial
// requests can carry different fixtures.

const mutate = vi.hoisted(() => vi.fn());
const revealFinancial = vi.hoisted(() => vi.fn());
const swrData = vi.hoisted(() => ({
  current: new Map<string, unknown>(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => ({
    data: [...swrData.current.entries()].find(([prefix]) =>
      key?.startsWith(prefix),
    )?.[1],
    error: undefined,
    isLoading: false,
    isValidating: false,
    mutate,
  }),
}));

vi.mock("~/lib/staff-api", () => ({
  staffPayrollNumberService: { get: vi.fn(), update: vi.fn() },
  staffStammdatenService: {
    get: vi.fn(),
    getFinancial: vi.fn(),
    revealFinancial,
    updatePerson: vi.fn(),
    updateKontakt: vi.fn(),
    updateArbeitsvertrag: vi.fn(),
    updateQualifikationen: vi.fn(),
    updateFinancial: vi.fn(),
  },
}));

const stammdaten: StaffStammdaten = {
  staffId: "42",
  person: {
    firstName: "Mila",
    lastName: "Muster",
    birthday: "1990-04-12",
    gender: "female",
  },
  kontakt: {
    addressStreet: "Musterweg 1",
    addressPostalCode: "48143",
    addressCity: "Münster",
    phone: "+49 251 123456",
    email: "mila@example.com",
    emergencyContactName: "Erik Muster",
    emergencyContactPhone: "+49 170 1",
  },
  arbeitsvertrag: {
    entryDate: "2024-08-01",
    contractEndDate: null,
    probationEndDate: "2025-01-31",
    weeklyHours: 29.5,
    employmentType: "part_time",
  },
  qualifikationen: [
    {
      id: "1",
      name: "Erste-Hilfe-Kurs",
      acquiredOn: "2019-03-10",
      expiresOn: "2020-03-10",
    },
    { id: "2", name: "Schwimmschein", acquiredOn: null, expiresOn: null },
  ],
};

function seedSWR({ financial = false } = {}) {
  swrData.current = new Map<string, unknown>([
    ["staff-stammdaten-financial-", financial ? maskedFinancial : undefined],
    ["staff-stammdaten-", stammdaten],
    ["staff-payroll-number-", "1023"],
  ]);
}

const maskedFinancial = {
  ibanMasked: "•••• 3000",
  taxIdMasked: "••••••••",
  socialSecurityNumberMasked: "••••••••",
};

describe("StammdatenTab Sektionen (#1423)", () => {
  beforeEach(() => {
    mutate.mockReset();
    revealFinancial.mockReset();
  });

  it("zeigt alle Sektionen mit Werten, aber ohne Bearbeiten-Buttons ohne users:update", () => {
    seedSWR();
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
      />,
    );

    expect(screen.getByText("Person")).toBeInTheDocument();
    expect(screen.getByText("Mila")).toBeInTheDocument();
    expect(screen.getByText("12.04.1990")).toBeInTheDocument();
    expect(screen.getByText("Weiblich")).toBeInTheDocument();
    expect(screen.getByText("Musterweg 1, 48143 Münster")).toBeInTheDocument();
    expect(screen.getByText("Unbefristet")).toBeInTheDocument();
    expect(screen.getByText("29,5 Std.")).toBeInTheDocument();
    expect(screen.getByText("Teilzeit")).toBeInTheDocument();
    expect(screen.getByText("Erste-Hilfe-Kurs")).toBeInTheDocument();
    expect(screen.getByText("Abgelaufen")).toBeInTheDocument();

    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();
  });

  it("zeigt Bearbeiten-Buttons mit users:update und öffnet das Personen-Modal", () => {
    seedSWR();
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
        canEditSections
      />,
    );

    const editButtons = screen.getAllByRole("button", { name: "Bearbeiten" });
    expect(editButtons).toHaveLength(4);

    fireEvent.click(editButtons[0]!);
    expect(screen.getByText("Person bearbeiten")).toBeInTheDocument();
    expect(screen.getByLabelText("Vorname")).toHaveValue("Mila");
  });

  it("zeigt ohne staff:financial die Sperre statt der Bank-Sektion", () => {
    seedSWR();
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
      />,
    );

    expect(screen.getByText(/Nicht berechtigt/)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Anzeigen" }),
    ).not.toBeInTheDocument();
  });

  it("zeigt maskierte Bankdaten und lädt Klartext über den Anzeigen-Toggle", async () => {
    seedSWR({ financial: true });
    revealFinancial.mockResolvedValue({
      iban: "DE89370400440532013000",
      taxId: "12345678911",
      socialSecurityNumber: "65170839J003",
    });

    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
        canViewFinancial
      />,
    );

    expect(screen.getByText("•••• 3000")).toBeInTheDocument();
    expect(
      screen.queryByText("DE89370400440532013000"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anzeigen" }));
    await waitFor(() =>
      expect(screen.getByText("DE89370400440532013000")).toBeInTheDocument(),
    );
    expect(revealFinancial).toHaveBeenCalledWith("42");

    fireEvent.click(screen.getByRole("button", { name: "Verbergen" }));
    expect(
      screen.queryByText("DE89370400440532013000"),
    ).not.toBeInTheDocument();
    expect(screen.getByText("•••• 3000")).toBeInTheDocument();
  });

  it("klappt eine Sektion über den Toggle ein", () => {
    seedSWR();
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
      />,
    );

    expect(screen.getByText("Mila")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Person einklappen" }));
    expect(screen.queryByText("Mila")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Person ausklappen" }));
    expect(screen.getByText("Mila")).toBeInTheDocument();
  });
});
