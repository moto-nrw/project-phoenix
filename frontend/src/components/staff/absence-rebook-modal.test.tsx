import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AbsenceType } from "~/lib/absence-type-api";
import type { AbsenceRebooking, StaffAbsenceRow } from "~/lib/staff-api";
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
vi.mock("~/contexts/ToastContext", () => ({ useToast: () => stable.toast }));

const mocks = vi.hoisted(() => ({
  getVacationQuota: vi.fn(),
  rebookAbsences: vi.fn(),
  getAllowance: vi.fn(),
}));

vi.mock("~/lib/staff-api", async () => {
  class AbsenceRebookingBlockedError extends Error {}
  return {
    AbsenceRebookingBlockedError,
    staffAbsenceService: {
      getVacationQuota: mocks.getVacationQuota,
      rebookAbsences: mocks.rebookAbsences,
    },
  };
});
vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: { getAllowance: mocks.getAllowance },
}));

import { AbsenceRebookingBlockedError } from "~/lib/staff-api";
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

function renderModal(onSaved = vi.fn().mockResolvedValue(undefined)) {
  render(
    <AbsenceRebookModal
      staff={staff}
      types={[sickLeave]}
      absences={fridays}
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
      new AbsenceRebookingBlockedError("Der August 2026 ist abgeschlossen."),
    );
    renderModal();

    fireEvent.click(screen.getByRole("radio", { name: /Krank-Urlaubstag/ }));

    expect(
      await screen.findByText("Der August 2026 ist abgeschlossen."),
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });

  it("reports a failed preview without enabling the save", async () => {
    mocks.rebookAbsences.mockRejectedValue(new Error("boom"));
    renderModal();

    fireEvent.click(screen.getByRole("radio", { name: /Krank-Urlaubstag/ }));

    expect(
      await screen.findByText(/Die Folgen konnten nicht berechnet werden/),
    ).toBeInTheDocument();
    expect(submitButton()).toBeDisabled();
  });
});
