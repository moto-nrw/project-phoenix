import {
  fireEvent,
  render as renderUi,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactElement } from "react";
import { ToastProvider } from "~/contexts/ToastContext";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { StammdatenTab } from "./stammdaten-tab";
import {
  staffPayrollNumberService,
  staffStammdatenService,
  type StaffStammdaten,
} from "~/lib/staff-api";

// The tab reports a failed reveal as a toast (#2511); the app mounts the
// provider globally.
function render(ui: ReactElement) {
  return renderUi(ui, { wrapper: ToastProvider });
}

// Section rendering + permission gating of the Stammdaten tab (#1423). The
// SWR mock switches on the cache key so the aggregate, payroll and financial
// requests can carry different fixtures.

const mutate = vi.hoisted(() => vi.fn());
const revealFinancial = vi.hoisted(() => vi.fn());
const swrData = vi.hoisted(() => ({
  current: new Map<string, unknown>(),
}));
const swrErrors = vi.hoisted(() => ({
  current: new Map<string, Error>(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => ({
    data: [...swrData.current.entries()].find(([prefix]) =>
      key?.startsWith(prefix),
    )?.[1],
    error: [...swrErrors.current.entries()].find(([prefix]) =>
      key?.startsWith(prefix),
    )?.[1],
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

function seedSWR({ financial = false, financialError = false } = {}) {
  swrData.current = new Map<string, unknown>([
    ["staff-stammdaten-financial-", financial ? maskedFinancial : undefined],
    ["staff-stammdaten-", stammdaten],
    ["staff-payroll-number-", "1023"],
  ]);
  swrErrors.current = new Map(
    financialError
      ? [["staff-stammdaten-financial-", new Error("request failed")]]
      : [],
  );
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
    vi.mocked(staffStammdatenService.updatePerson).mockReset();
    swrErrors.current = new Map();
  });

  it("zeigt alle Sektionen mit Werten, aber ohne Bearbeiten-Buttons ohne staff:stammdaten", () => {
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

  it("zeigt mit staff:stammdaten EIN Bearbeiten für den ganzen Reiter", () => {
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
    expect(editButtons).toHaveLength(1);

    fireEvent.click(editButtons[0]!);
    // Alle Gruppen wechseln gemeinsam in den Bearbeiten-Zustand, mit genau
    // einem Speichern am Ende des Reiters.
    expect(screen.getByLabelText("Vorname")).toHaveValue("Mila");
    expect(screen.getByLabelText("Telefon")).toHaveValue("+49 251 123456");
    expect(screen.getAllByRole("button", { name: "Speichern" })).toHaveLength(
      1,
    );
  });

  it("überschreibt ein vorhandenes Geburtsdatum und sendet den ISO-Wert", async () => {
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

    fireEvent.click(screen.getAllByRole("button", { name: "Bearbeiten" })[0]!);
    const birthdayInput = screen.getByLabelText("Geburtsdatum");
    expect(birthdayInput).toHaveValue("12.04.1990");

    fireEvent.change(birthdayInput, { target: { value: "17.04.1982" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(staffStammdatenService.updatePerson).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ birthday: "1982-04-17" }),
        "",
      ),
    );
  });

  it("meldet einen abgelehnten Abschnitt im Alert oben und markiert die Felder", async () => {
    seedSWR();
    vi.mocked(staffStammdatenService.updatePerson).mockRejectedValue(
      new ApiError("first_name: is required.", 400, {
        code: "general.input",
        errors: [{ field: "first_name", reason: "is required" }],
      }),
    );
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
        canEditSections
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Bearbeiten" })[0]!);
    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "17.04.1982" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Änderung an „Person“ konnte nicht übernommen werden. Bitte prüfen Sie Ihre Angaben.",
      ),
    ).toBeInTheDocument();
    const first = screen.getByLabelText("Vorname");
    await waitFor(() => expect(first).toHaveFocus());
    expect(first).toHaveAttribute("aria-invalid", "true");
    expect(screen.queryByRole("alert", { name: /^Fehler:/ })).toBeNull();
  });

  it("zeigt bei einem Serverfehler die Vorgangskennung zum Kopieren", async () => {
    seedSWR();
    vi.mocked(staffStammdatenService.updatePerson).mockRejectedValue(
      new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-stamm-1",
      }),
    );
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
        canEditSections
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Bearbeiten" })[0]!);
    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "17.04.1982" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("Vorgangskennung: req-stamm-1");
    expect(
      screen.getByText(
        "Die Änderung an „Person“ konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
  });

  it("nennt eine vergebene Personalnummer und schickt gespeicherte Abschnitte nicht erneut", async () => {
    seedSWR();
    vi.mocked(staffStammdatenService.updatePerson).mockResolvedValue(undefined);
    vi.mocked(staffPayrollNumberService.update).mockReset();
    vi.mocked(staffPayrollNumberService.update)
      .mockRejectedValueOnce(
        new ApiError("personnel number taken", 409, {
          code: "workforce.personnel_number_taken",
        }),
      )
      .mockResolvedValueOnce("90002");
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll
        canManagePayrollSettings={false}
        canViewSections
        canEditSections
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Bearbeiten" })[0]!);
    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "17.04.1982" },
    });
    fireEvent.change(screen.getByLabelText("Personalnummer"), {
      target: { value: "90002" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Personalnummer gibt es bereits. Bitte wählen Sie eine andere Angabe.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(staffPayrollNumberService.update).toHaveBeenCalledTimes(2),
    );
    expect(staffStammdatenService.updatePerson).toHaveBeenCalledTimes(1);
  });

  it("hält beim ersten abgelehnten Abschnitt an", async () => {
    seedSWR();
    vi.mocked(staffStammdatenService.updatePerson).mockRejectedValue(
      new ApiError("boom", 503, { code: "general.unavailable" }),
    );
    vi.mocked(staffPayrollNumberService.update).mockReset();
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll
        canManagePayrollSettings={false}
        canViewSections
        canEditSections
      />,
    );

    fireEvent.click(screen.getAllByRole("button", { name: "Bearbeiten" })[0]!);
    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "17.04.1982" },
    });
    fireEvent.change(screen.getByLabelText("Personalnummer"), {
      target: { value: "90002" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Änderung an „Person“ ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(staffPayrollNumberService.update).not.toHaveBeenCalled();
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

  it("meldet ein abgelehntes Anzeigen der Bankdaten als Toast", async () => {
    seedSWR({ financial: true });
    revealFinancial.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.permission" }),
    );
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
        canViewFinancial
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Anzeigen" }));

    expect(
      await screen.findByRole("alert", {
        name: "Fehler: Für die Anzeige der Bank- und Steuerdaten fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("•••• 3000")).toBeInTheDocument();
  });

  it("zeigt einen Ladefehler der Bankdaten mit erneutem Laden statt Leerwerten", () => {
    seedSWR({ financialError: true });
    render(
      <StammdatenTab
        staffId="42"
        canManagePayroll={false}
        canManagePayrollSettings={false}
        canViewSections
        canViewFinancial
      />,
    );

    expect(
      screen.getByText(
        "Die Bank- und Steuerdaten konnten nicht geladen werden.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("•••• 3000")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Bearbeiten" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Erneut laden" }));
    expect(mutate).toHaveBeenCalledTimes(1);
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
