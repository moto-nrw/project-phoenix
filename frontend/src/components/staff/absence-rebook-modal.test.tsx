import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AbsenceType } from "~/lib/absence-type-api";
import { ApiError } from "~/lib/api-error";
import type { AbsenceRebooking, StaffAbsenceRow } from "~/lib/staff-api";
import { catalogText } from "~/test/error-catalog-text";
import { suppressConsole } from "~/test/helpers/console";

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    title,
    children,
    footer,
  }: {
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) => (
    <div role="dialog" aria-label={title}>
      {children}
      {footer}
    </div>
  ),
}));

const stable = vi.hoisted(() => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => stable.toast,
  useApiErrorDisplay: () => ({ show: actionErrors.show }),
}));
const actionErrors = vi.hoisted(() => ({ show: vi.fn() }));

const mocks = vi.hoisted(() => ({
  getVacationQuota: vi.fn(),
  rebookAbsences: vi.fn(),
  getAllowance: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    getVacationQuota: mocks.getVacationQuota,
    rebookAbsences: mocks.rebookAbsences,
  },
}));
vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: { getAllowance: mocks.getAllowance },
}));

import {
  AbsenceRebookModal,
  isRebookableAbsence,
} from "./absence-rebook-modal";

const staff = { id: "4", firstName: "Swantje", lastName: "Kollegin" };

const sickLeave: AbsenceType = {
  id: "12",
  name: "Krank-Urlaubstag",
  baseType: "other",
  isActive: true,
  allowanceEnabled: true,
  carryoverUntil: "03-31",
};

function friday(id: number, day: string): StaffAbsenceRow {
  return {
    id,
    staff_id: 4,
    absence_type: "comp_time",
    date_start: day,
    date_end: day,
    half_day: false,
    note: "",
    status: "reported",
  };
}

const fridays = [friday(71, "2026-08-07"), friday(72, "2026-08-14")];

function rebooking(extra: Partial<AbsenceRebooking> = {}): AbsenceRebooking {
  return {
    absences: [],
    days: 2,
    balanceDeltaMinutes: 960,
    allowances: [{ year: 2026, remainingDays: 8, bookingDays: 2 }],
    allowanceExceeded: false,
    vacation: [],
    vacationExceeded: false,
    applied: false,
    ...extra,
  };
}

function renderModal(
  onSaved = vi.fn().mockResolvedValue(undefined),
  absences = fridays,
) {
  render(
    <AbsenceRebookModal
      staff={staff}
      types={[sickLeave]}
      absences={absences}
      onClose={vi.fn()}
      onSaved={onSaved}
    />,
  );
  return onSaved;
}

function submitButton() {
  return screen.getByRole("button", { name: "Art ändern" });
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.getVacationQuota.mockResolvedValue({ remaining_days: 20 });
  mocks.getAllowance.mockResolvedValue({
    remainingDays: 10,
    carriedIn: null,
  });
});

describe("AbsenceRebookModal", () => {
  suppressConsole("error");

  it("offers only reported entries that are no sick reports", () => {
    expect(isRebookableAbsence(fridays[0]!)).toBe(true);
    expect(isRebookableAbsence({ ...fridays[0]!, absence_type: "sick" })).toBe(
      false,
    );
    expect(
      isRebookableAbsence({
        ...fridays[0]!,
        absence_type: "vacation",
        status: "approved",
      }),
    ).toBe(false);
  });

  it("hides the type every entry already has", () => {
    renderModal();
    expect(
      screen.queryByRole("radio", { name: "Freizeitausgleich" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Krank-Urlaubstag/ }),
    ).toBeInTheDocument();
  });

  it("hides every type already used by a mixed selection", () => {
    render(
      <AbsenceRebookModal
        staff={staff}
        types={[sickLeave]}
        absences={[
          { ...fridays[0]!, absence_type: "vacation" },
          { ...fridays[1]!, absence_type: "training" },
        ]}
        onClose={vi.fn()}
        onSaved={vi.fn().mockResolvedValue(undefined)}
      />,
    );

    expect(
      screen.queryByRole("radio", { name: "Urlaub" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("radio", { name: "Fortbildung" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Krank-Urlaubstag/ }),
    ).toBeInTheDocument();
  });

  it("loads and shows allowance accounts for every affected year", async () => {
    mocks.getAllowance.mockImplementation(
      (_type: string, _staffID: string, year: number) =>
        Promise.resolve({
          remainingDays: year === 2025 ? 1 : 3,
          carriedIn: null,
        }),
    );
    renderModal(undefined, [
      { ...fridays[0]!, date_start: "2025-12-31", date_end: "2026-01-02" },
    ]);

    expect(await screen.findByText("2025: noch 1 Tag")).toBeInTheDocument();
    expect(screen.getByText("2026: noch 3 Tage")).toBeInTheDocument();
    expect(mocks.getAllowance).toHaveBeenCalledWith("12", "4", 2025);
    expect(mocks.getAllowance).toHaveBeenCalledWith("12", "4", 2026);
    expect(mocks.getVacationQuota).toHaveBeenCalledWith("4", 2025);
    expect(mocks.getVacationQuota).toHaveBeenCalledWith("4", 2026);
  });

  it("shows the effects and saves with a reason", async () => {
    mocks.rebookAbsences.mockResolvedValue(rebooking());
    const onSaved = renderModal();

    fireEvent.click(screen.getByRole("radio", { name: /Krank-Urlaubstag/ }));

    await waitFor(() =>
      expect(mocks.rebookAbsences).toHaveBeenCalledWith("4", {
        absenceIds: ["71", "72"],
        absenceType: "other",
        absenceTypeId: "12",
        reason: "",
        dryRun: true,
      }),
    );
    expect(await screen.findByText("+16h")).toBeInTheDocument();
    expect(screen.getByText("Diese Änderung")).toBeInTheDocument();

    // Ohne Grund wird nichts gespeichert.
    fireEvent.click(submitButton());
    expect(
      await screen.findByText("Bitte kurz sagen, warum sich die Art ändert."),
    ).toBeInTheDocument();
    expect(mocks.rebookAbsences).toHaveBeenCalledTimes(1);

    fireEvent.change(screen.getByLabelText("Grund (Pflicht)"), {
      target: { value: "Kontingent angelegt" },
    });
    fireEvent.click(submitButton());

    await waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(mocks.rebookAbsences).toHaveBeenLastCalledWith("4", {
      absenceIds: [71, 72],
      absenceType: "other",
      absenceTypeId: "12",
      reason: "Kontingent angelegt",
      dryRun: false,
    });
    expect(stable.toast.success).toHaveBeenCalledWith(
      "2 Einträge sind jetzt Krank-Urlaubstag.",
    );
  });

  it("blocks saving when the allowance is too small", async () => {
    mocks.rebookAbsences.mockResolvedValue(
      rebooking({
        allowanceExceeded: true,
        allowances: [{ year: 2026, remainingDays: -1, bookingDays: 2 }],
      }),
    );
    renderModal();

    fireEvent.click(screen.getByRole("radio", { name: /Krank-Urlaubstag/ }));

    expect(
      await screen.findByText(/Nicht genug Tage: Krank-Urlaubstag reicht/),
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it("shows why a rebooking is blocked and keeps it disabled", async () => {
    mocks.rebookAbsences.mockRejectedValue(
      new ApiError("absence rebooking blocked", 409, {
        code: "workforce.rebooking_into_sick_report",
      }),
    );
    renderModal();

    fireEvent.click(screen.getByRole("radio", { name: /Krank-Urlaubstag/ }));

    const dialog = await screen.findByRole("dialog");
    expect(
      await within(dialog).findByText(
        catalogText(
          "workforce.rebooking_into_sick_report",
          "die Änderung der Art",
        ),
      ),
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it("reports a failed preview without enabling the save", async () => {
    mocks.rebookAbsences.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server", instance: "req-r" }),
    );
    renderModal();

    fireEvent.click(screen.getByRole("radio", { name: /Krank-Urlaubstag/ }));

    expect(
      await screen.findByText(
        catalogText("general.server", "die Änderung der Art"),
      ),
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();

    // Wiederholen rechnet die Folgen neu; danach lässt sich speichern.
    mocks.rebookAbsences.mockResolvedValue(rebooking());
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("+16h")).toBeInTheDocument();
    expect(
      screen.queryByText(catalogText("general.server", "die Änderung der Art")),
    ).not.toBeInTheDocument();
  });
});
