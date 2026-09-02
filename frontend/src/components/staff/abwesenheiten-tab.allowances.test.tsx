import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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

const stable = vi.hoisted(() => ({
  toast: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
  mutateMatching: vi.fn().mockResolvedValue(undefined),
  swrMutate: vi.fn().mockResolvedValue(undefined),
}));

vi.mock("~/contexts/ToastContext", () => ({ useToast: () => stable.toast }));
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
  getAbsenceTypes: vi.fn(),
  getAllowance: vi.fn(),
  setAllowance: vi.fn(),
  createAbsence: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: {
    getVacationQuota: mocks.getVacationQuota,
    getAbsences: mocks.getAbsences,
    createAbsence: mocks.createAbsence,
    approve: vi.fn(),
    deleteAbsence: vi.fn(),
  },
}));
vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: {
    getAbsenceTypes: mocks.getAbsenceTypes,
    getAllowance: mocks.getAllowance,
    setAllowance: mocks.setAllowance,
  },
}));

import { AbwesenheitenTab } from "./abwesenheiten-tab";

const year = new Date().getFullYear();
const allowance = {
  staffId: "4",
  absenceTypeId: "12",
  year,
  entitledDays: 3.5,
  takenDays: 0.5,
  reservedDays: 1,
  remainingDays: 2,
};

describe("AbwesenheitenTab custom allowances", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getVacationQuota.mockResolvedValue({
      staff_id: 4,
      year,
      entitled_days: 30,
      carryover_days: 0,
      taken_before_days: 0,
      taken_days: 0,
      reserved_days: 0,
      remaining_days: 30,
    });
    mocks.getAbsences.mockResolvedValue([]);
    mocks.getAbsenceTypes.mockResolvedValue([
      {
        id: "12",
        name: "Regenerationstag",
        baseType: "other",
        isActive: true,
        allowanceEnabled: true,
        overrunPolicy: "block",
      },
    ]);
    mocks.getAllowance.mockResolvedValue(allowance);
    mocks.setAllowance.mockResolvedValue(allowance);
  });

  it("shows the account and requires a reason for a correction", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Rena", lastName: "Generation" }}
      />,
    );

    expect(await screen.findByText("Regenerationstag")).toBeInTheDocument();
    expect(screen.getByText("Weitere Kontingente")).toBeInTheDocument();
    expect(screen.getByText("2 Tage")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anspruch ändern" }));
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.change(screen.getByLabelText("Anspruch in Tagen"), {
      target: { value: "4,5" },
    });
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Neuer Tarifvertrag" },
    });
    fireEvent.click(save);

    await waitFor(() =>
      expect(mocks.setAllowance).toHaveBeenCalledWith("12", "4", {
        year,
        entitledDays: 4.5,
        reason: "Neuer Tarifvertrag",
      }),
    );
  });

  it("blocks a booking when the configured account has no days left", async () => {
    mocks.getAllowance.mockResolvedValue({ ...allowance, remainingDays: -0.5 });
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Rena", lastName: "Generation" }}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Weitere Abwesenheit" }),
    );
    expect(
      screen.getByText(
        "Das Kontingent reicht nicht aus. Die Buchung ist nicht möglich.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Eintragen" })).toBeDisabled();
  });

  it("charges only the part of an extension that is not already booked", async () => {
    mocks.getAllowance.mockResolvedValue({ ...allowance, remainingDays: 1 });
    mocks.getAbsences.mockResolvedValue([
      {
        id: 31,
        staff_id: 4,
        absence_type: "other",
        absence_type_id: "12",
        date_start: `${year}-09-07`,
        date_end: `${year}-09-07`,
        half_day: false,
        note: "",
        status: "reported",
      },
    ]);
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Rena", lastName: "Generation" }}
      />,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Weitere Abwesenheit" }),
    );
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: `${year}-09-07` },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: `${year}-09-08` },
    });
    expect(screen.getByText("Danach verbleiben 0 Tage.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Eintragen" })).toBeEnabled();
  });

  it("keeps inactive accounts visible but excludes them from new bookings", async () => {
    mocks.getAbsenceTypes.mockResolvedValue([
      {
        id: "12",
        name: "Regenerationstag",
        baseType: "other",
        isActive: false,
        allowanceEnabled: true,
        overrunPolicy: "block",
      },
    ]);
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Rena", lastName: "Generation" }}
      />,
    );

    expect(await screen.findByText("Regenerationstag")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weitere Abwesenheit" }),
    ).not.toBeInTheDocument();
  });

  it("loads and edits the selected allowance year", async () => {
    mocks.getAllowance.mockImplementation(
      (_typeId: string, _staffId: string, requestedYear: number) =>
        Promise.resolve({
          ...allowance,
          year: requestedYear,
          remainingDays: requestedYear === year ? 1 : -5,
        }),
    );
    mocks.getAbsences.mockResolvedValue([
      {
        id: 31,
        staff_id: 4,
        absence_type: "other",
        absence_type_id: "12",
        date_start: `${year}-09-07`,
        date_end: `${year}-09-07`,
        half_day: false,
        note: "",
        status: "reported",
      },
    ]);
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Rena", lastName: "Generation" }}
      />,
    );
    await screen.findByText("Regenerationstag");
    fireEvent.click(screen.getByRole("combobox", { name: "Kalenderjahr" }));
    fireEvent.click(
      await screen.findByRole("option", { name: String(year + 1) }),
    );
    await waitFor(() =>
      expect(mocks.getAllowance).toHaveBeenCalledWith("12", "4", year + 1),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Weitere Abwesenheit" }),
    );
    fireEvent.change(screen.getByLabelText("Von"), {
      target: { value: `${year}-09-07` },
    });
    fireEvent.change(screen.getByLabelText("Bis"), {
      target: { value: `${year}-09-08` },
    });
    expect(screen.getByText("Danach verbleiben 0 Tage.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Eintragen" })).toBeEnabled();
  });
});
