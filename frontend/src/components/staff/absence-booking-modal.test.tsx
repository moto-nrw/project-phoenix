import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AbsenceType } from "~/lib/absence-type-api";
import { suppressConsole } from "~/test/helpers/console";

vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

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
  getAbsences: vi.fn(),
  createAbsence: vi.fn(),
  getCompTimePreview: vi.fn(),
  getAllowance: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    getVacationQuota: mocks.getVacationQuota,
    getAbsences: mocks.getAbsences,
    createAbsence: mocks.createAbsence,
    getCompTimePreview: mocks.getCompTimePreview,
  },
}));
vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: { getAllowance: mocks.getAllowance },
}));

import { AbsenceBookingModal } from "./absence-booking-modal";

// Test clock: Wednesday, 2026-09-09.
const staff = { id: "4", firstName: "Rena", lastName: "Generation" };

const regeneration: AbsenceType = {
  id: "12",
  name: "Regenerationstag",
  baseType: "other",
  isActive: true,
  allowanceEnabled: true,
};
const conversion: AbsenceType = {
  id: "13",
  name: "Umwandlungstag",
  baseType: "other",
  isActive: false,
  allowanceEnabled: true,
};
const trip: AbsenceType = {
  id: "14",
  name: "Dienstreise",
  baseType: "other",
  isActive: true,
  allowanceEnabled: false,
};

function vacationQuota(year: number, remaining: number) {
  return {
    staff_id: 4,
    year,
    entitled_days: remaining,
    carryover_days: 0,
    taken_before_days: 0,
    taken_days: 0,
    reserved_days: 0,
    remaining_days: remaining,
  };
}

function allowance(year: number, remaining: number) {
  return {
    staffId: "4",
    absenceTypeId: "12",
    year,
    entitledDays: 2,
    takenDays: 2 - remaining,
    reservedDays: 0,
    remainingDays: remaining,
  };
}

function renderModal(types: AbsenceType[] = [regeneration, conversion, trip]) {
  const onSaved = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  render(
    <AbsenceBookingModal
      staff={staff}
      types={types}
      onClose={onClose}
      onSaved={onSaved}
    />,
  );
  return { onSaved, onClose };
}

function choose(label: string) {
  fireEvent.click(screen.getByRole("radio", { name: new RegExp(`^${label}`) }));
}

const submit = () => screen.getByRole("button", { name: "Eintragen" });

describe("AbsenceBookingModal", () => {
  suppressConsole("error");

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getVacationQuota.mockImplementation((_id: string, year: number) =>
      Promise.resolve(vacationQuota(year, 12)),
    );
    mocks.getAbsences.mockResolvedValue([]);
    mocks.getAllowance.mockImplementation(
      (_type: string, _id: string, year: number) =>
        Promise.resolve(allowance(year, 1)),
    );
    mocks.createAbsence.mockResolvedValue({});
    mocks.getCompTimePreview.mockResolvedValue({
      currentBalanceMinutes: 600,
      deductionMinutes: 480,
      realizedDeductionMinutes: 0,
      futureCommitmentMinutes: 0,
      futureAdjustmentMinutes: 0,
      projectedBalanceMinutes: 120,
    });
  });

  it("shows how many days each account has left and offers only active arts", async () => {
    renderModal();

    expect(await screen.findByText("noch 12 Tage")).toBeInTheDocument();
    expect(screen.getByText("noch 1 Tag")).toBeInTheDocument();
    expect(screen.getByText("vom Stundenkonto")).toBeInTheDocument();
    expect(screen.getAllByText("ohne Kontingent")).toHaveLength(3);
    expect(
      screen.queryByRole("radio", { name: /Umwandlungstag/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("radio", { name: /Dienstreise/ }),
    ).toBeInTheDocument();
    // Nothing is preselected, so nothing is booked by accident.
    expect(submit()).toBeDisabled();
  });

  it("books vacation directly and shows what is left afterwards", async () => {
    const { onSaved } = renderModal();
    await screen.findByText("noch 12 Tage");

    choose("Urlaub");
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-09-14" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-18" },
    });

    expect(screen.getByText("Danach übrig")).toBeInTheDocument();
    expect(screen.getByText("7 Tage")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Das Tagessoll wird gutgeschrieben. Die Überstunden bleiben gleich.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(submit());

    await waitFor(() =>
      expect(mocks.createAbsence).toHaveBeenCalledWith("4", {
        absence_type: "vacation",
        date_start: "2026-09-14",
        date_end: "2026-09-18",
        half_day: undefined,
        note: undefined,
      }),
    );
    expect(onSaved).toHaveBeenCalled();
    expect(stable.toast.success).toHaveBeenCalledWith("Urlaub eingetragen.");
  });

  it("blocks a booking beyond the account and says what to do", async () => {
    renderModal();
    await screen.findByText("noch 1 Tag");

    choose("Regenerationstag");
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-10" },
    });

    expect(
      screen.getByText(
        "Nicht genug Tage: Regenerationstag hat noch 1 Tag, diese Eintragung braucht 2 Tage. Ändern Sie zuerst den Anspruch oder wählen Sie eine andere Art.",
      ),
    ).toBeInTheDocument();
    expect(submit()).toBeDisabled();

    fireEvent.click(screen.getByRole("checkbox", { name: "Halber Tag" }));
    expect(screen.queryByText(/Nicht genug Tage/)).not.toBeInTheDocument();
    expect(submit()).toBeEnabled();
    fireEvent.click(submit());
    await waitFor(() =>
      expect(mocks.createAbsence).toHaveBeenCalledWith("4", {
        absence_type: "other",
        absence_type_id: "12",
        date_start: "2026-09-09",
        date_end: "2026-09-09",
        half_day: true,
        note: undefined,
      }),
    );
  });

  it("charges only the part of an extension that is not already booked", async () => {
    mocks.getAbsences.mockResolvedValue([
      {
        id: 31,
        staff_id: 4,
        absence_type: "other",
        absence_type_id: "12",
        date_start: "2026-09-09",
        date_end: "2026-09-09",
        half_day: false,
        note: "",
        status: "reported",
      },
    ]);
    renderModal();
    await screen.findByText("noch 1 Tag");

    choose("Regenerationstag");
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2026-09-10" },
    });

    expect(screen.queryByText(/Nicht genug Tage/)).not.toBeInTheDocument();
    expect(screen.getByText("0 Tage")).toBeInTheDocument();
    expect(submit()).toBeEnabled();
  });

  it("checks every calendar year a booking touches", async () => {
    mocks.getVacationQuota.mockImplementation((_id: string, year: number) =>
      Promise.resolve(vacationQuota(year, year === 2027 ? 0 : 12)),
    );
    renderModal();
    await screen.findByText("noch 12 Tage");

    choose("Urlaub");
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: "2026-12-30" },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: "2027-01-04" },
    });

    await waitFor(() =>
      expect(mocks.getVacationQuota).toHaveBeenCalledWith("4", 2027),
    );
    expect(
      await screen.findByText(/Nicht genug Tage: Urlaub hat noch 0 Tage/),
    ).toBeInTheDocument();
    expect(screen.getByText("Urlaub 2026: noch übrig")).toBeInTheDocument();
    expect(screen.getByText("Urlaub 2027: noch übrig")).toBeInTheDocument();
    expect(submit()).toBeDisabled();
  });

  it("shows the server's reason when the booking is refused", async () => {
    mocks.createAbsence.mockRejectedValue(
      new Error("An diesen Tagen ist schon eine Abwesenheit eingetragen."),
    );
    const { onSaved } = renderModal();
    await screen.findByText("noch 12 Tage");

    choose("Fortbildung");
    fireEvent.click(submit());

    expect(
      await screen.findByText(
        "An diesen Tagen ist schon eine Abwesenheit eingetragen.",
      ),
    ).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
  });

  describe("Freizeitausgleich (#2873)", () => {
    it("shows the Stundenkonto preview and books comp time", async () => {
      renderModal();
      await screen.findByText("noch 12 Tage");
      choose("Freizeitausgleich");

      expect(
        screen.getByText(
          "Freizeitausgleich zieht das Tagessoll vom Stundenkonto ab.",
        ),
      ).toBeInTheDocument();
      expect(
        await screen.findByText("Stundenkonto aktuell"),
      ).toBeInTheDocument();
      expect(screen.getByText("Stundenkonto danach")).toBeInTheDocument();
      expect(
        screen.queryByText("Bereits geplante Buchungen"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByText(/fällt damit unter null/),
      ).not.toBeInTheDocument();

      await waitFor(() => expect(submit()).toBeEnabled());
      fireEvent.click(submit());
      await waitFor(() =>
        expect(mocks.createAbsence).toHaveBeenCalledWith(
          "4",
          expect.objectContaining({ absence_type: "comp_time" }),
        ),
      );
    });

    it("requires an explicit confirmation when the projection is negative", async () => {
      mocks.getCompTimePreview.mockResolvedValue({
        currentBalanceMinutes: 120,
        deductionMinutes: 480,
        realizedDeductionMinutes: 0,
        futureCommitmentMinutes: 240,
        futureAdjustmentMinutes: -60,
        projectedBalanceMinutes: -660,
      });
      renderModal();
      await screen.findByText("noch 12 Tage");
      choose("Freizeitausgleich");

      expect(
        await screen.findByText(/fällt damit unter null/),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Bereits geplanter Freizeitausgleich"),
      ).toBeInTheDocument();
      expect(
        screen.getByText("Bereits geplante Buchungen"),
      ).toBeInTheDocument();
      expect(submit()).toBeDisabled();

      fireEvent.click(
        screen.getByRole("checkbox", {
          name: "Ich trage den Freizeitausgleich trotzdem ein.",
        }),
      );
      expect(submit()).toBeEnabled();
    });

    it("still allows booking when the preview fails to load", async () => {
      mocks.getCompTimePreview.mockRejectedValue(new Error("preview down"));
      renderModal();
      await screen.findByText("noch 12 Tage");
      choose("Freizeitausgleich");

      await waitFor(() => expect(submit()).toBeEnabled());
    });

    it("blocks the submit while a changed range's preview reloads (#2885)", async () => {
      renderModal();
      await screen.findByText("noch 12 Tage");
      choose("Freizeitausgleich");
      expect(
        await screen.findByText("Stundenkonto danach"),
      ).toBeInTheDocument();
      await waitFor(() => expect(submit()).toBeEnabled());

      let resolvePreview!: (value: unknown) => void;
      mocks.getCompTimePreview.mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolvePreview = resolve;
          }),
      );
      fireEvent.change(screen.getByLabelText("Bis"), {
        target: { value: "2026-09-11" },
      });

      expect(
        await screen.findByText(/Stundenkonto wird berechnet/),
      ).toBeInTheDocument();
      expect(submit()).toBeDisabled();

      resolvePreview({
        currentBalanceMinutes: 600,
        deductionMinutes: 1440,
        realizedDeductionMinutes: 0,
        futureCommitmentMinutes: 0,
        futureAdjustmentMinutes: 0,
        projectedBalanceMinutes: -840,
      });
      expect(
        await screen.findByText(/fällt damit unter null/),
      ).toBeInTheDocument();
      expect(submit()).toBeDisabled();
    });
  });
});
