import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { StaffVacationQuotaSummary } from "~/lib/staff-api";

// Same stub as stundenkonto-panel.test.tsx: the kit picker opens a calendar
// overlay, the native input keeps the Stichtag settable via fireEvent.change
// and forwards min/max so the computed bounds stay assertable.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

// Reduced to the confirm path: the two-step gate is the shared component's own
// concern and is covered where that component lives.
vi.mock("~/components/ui/confirm-delete-modal", () => ({
  ConfirmDeleteModal: ({
    isOpen,
    title,
    onConfirm,
  }: {
    isOpen: boolean;
    title: string;
    onConfirm: () => void;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <button type="button" onClick={onConfirm}>
          Bestätigen
        </button>
      </div>
    ) : null,
}));

// Both hooks return stable identities on purpose: the tab's reload callback
// depends on the toast object, so a fresh object per render would re-run the
// load effect forever and keep the tab in its loading state.
const stable = vi.hoisted(() => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
  mutateMatching: vi.fn().mockResolvedValue(undefined),
  swrMutate: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => stable.toast,
}));

// Without an SWRConfig provider `useSWRConfig()` hands back a fresh `mutate`
// on every render, which would re-run the tab's load effect endlessly.
vi.mock("swr", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useSWRConfig: () => ({ mutate: stable.swrMutate }),
}));

vi.mock("~/lib/swr", () => ({
  useTenantMutateMatching: () => stable.mutateMatching,
}));

const mocks = vi.hoisted(() => ({
  getVacationQuota: vi.fn(),
  getAbsences: vi.fn(),
  setVacationOpening: vi.fn(),
  deleteVacationOpening: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    getVacationQuota: mocks.getVacationQuota,
    getAbsences: mocks.getAbsences,
    setVacationOpening: mocks.setVacationOpening,
    deleteVacationOpening: mocks.deleteVacationOpening,
    approve: vi.fn(),
    deleteAbsence: vi.fn(),
  },
}));

import { AbwesenheitenTab } from "./abwesenheiten-tab";

const year = new Date().getFullYear();

function quota(
  overrides: Partial<StaffVacationQuotaSummary> = {},
): StaffVacationQuotaSummary {
  return {
    staff_id: 4,
    year,
    entitled_days: 30,
    carryover_days: 2,
    taken_before_days: 0,
    taken_days: 0,
    reserved_days: 0,
    remaining_days: 32,
    ...overrides,
  };
}

const existingOpening = {
  id: "7",
  staff_id: "4",
  year,
  effective_date: `${year}-02-28`,
  taken_before_days: 6,
  entered_remaining_days: 26,
  note: "Übernahme aus Urlaubsliste",
  decided_by: "9",
  decided_at: `${year}-03-01T08:00:00Z`,
};

function renderTab() {
  return render(<AbwesenheitenTab staffId="4" canEdit canManageSickReports />);
}

describe("AbwesenheitenTab vacation takeover", () => {
  it("books a takeover from the entered Resturlaub", async () => {
    mocks.getVacationQuota.mockResolvedValue(quota());
    mocks.getAbsences.mockResolvedValue([]);
    mocks.setVacationOpening.mockResolvedValue(existingOpening);
    renderTab();

    fireEvent.click(
      await screen.findByRole("button", { name: "Urlaubs-Übernahme" }),
    );

    const date = screen.getByLabelText("Stichtag");
    fireEvent.change(date, { target: { value: `${year}-02-28` } });
    fireEvent.change(screen.getByLabelText("Resturlaub zum Stichtag (Tage)"), {
      target: { value: "26,5" },
    });
    fireEvent.change(screen.getByLabelText("Begründung (Pflicht)"), {
      target: { value: "Übernahme aus Urlaubsliste" },
    });

    // Anspruch 30 + Übertrag 2 − Rest 26,5 = 5,5, wie der Server rechnet.
    expect(screen.getByText("5,5 Tage")).toBeInTheDocument();
    // Der Stichtag bleibt im angezeigten Jahr.
    expect(date).toHaveAttribute("min", `${year}-01-01`);

    fireEvent.click(screen.getByRole("button", { name: "Übernahme buchen" }));

    await waitFor(() => {
      expect(mocks.setVacationOpening).toHaveBeenCalledWith("4", {
        effectiveDate: `${year}-02-28`,
        remainingDays: 26.5,
        note: "Übernahme aus Urlaubsliste",
      });
    });
  });

  it("rejects malformed and over-precise vacation opening values", async () => {
    mocks.getVacationQuota.mockResolvedValue(quota());
    mocks.getAbsences.mockResolvedValue([]);
    renderTab();

    fireEvent.click(
      await screen.findByRole("button", { name: "Urlaubs-Übernahme" }),
    );
    fireEvent.change(screen.getByLabelText("Stichtag"), {
      target: { value: `${year}-02-28` },
    });
    fireEvent.change(screen.getByLabelText("Begründung (Pflicht)"), {
      target: { value: "Übernahme aus Urlaubsliste" },
    });

    const remainingInput = screen.getByLabelText(
      "Resturlaub zum Stichtag (Tage)",
    );
    for (const value of ["14,5 Tage", "12abc", "12,25"]) {
      fireEvent.change(remainingInput, { target: { value } });

      expect(
        screen.getByRole("button", { name: "Übernahme buchen" }),
      ).toBeDisabled();
      expect(
        screen.getByText(
          "Bitte eine Zahl zwischen -999 und 999 mit höchstens einer Nachkommastelle eingeben.",
        ),
      ).toBeInTheDocument();
    }
  });

  it("shows the booked takeover and deletes it instead of editing", async () => {
    mocks.getVacationQuota.mockResolvedValue(
      quota({
        taken_before_days: 6,
        remaining_days: 26,
        opening: existingOpening,
      }),
    );
    mocks.getAbsences.mockResolvedValue([]);
    mocks.deleteVacationOpening.mockResolvedValue(undefined);
    renderTab();

    expect(
      await screen.findByText("6 Tage vor Einführung genommen"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Urlaubs-Übernahme" }));
    expect(
      screen.queryByLabelText("Resturlaub zum Stichtag (Tage)"),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Übernahme löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    await waitFor(() => {
      expect(mocks.deleteVacationOpening).toHaveBeenCalledWith("4", year);
    });
  });

  it("hides the takeover entry point without edit permission", async () => {
    mocks.getVacationQuota.mockResolvedValue(quota());
    mocks.getAbsences.mockResolvedValue([]);
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit={false}
        canManageSickReports={false}
      />,
    );

    expect(await screen.findByText("Historie " + year)).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Urlaubs-Übernahme" }),
    ).not.toBeInTheDocument();
  });
});
