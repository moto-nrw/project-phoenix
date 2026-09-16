import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
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

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/demo${path}`,
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
    getCompTimePreview: vi.fn(),
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

// Test clock: 2026-09-09.
const year = 2026;
const allowance = {
  staffId: "4",
  absenceTypeId: "12",
  year,
  entitledDays: 3.5,
  takenDays: 0.5,
  reservedDays: 1,
  remainingDays: 2,
};
const regeneration = {
  id: "12",
  name: "Regenerationstag",
  baseType: "other",
  isActive: true,
  allowanceEnabled: true,
};

function renderTab(props: Partial<{ canEditQuota: boolean }> = {}) {
  return render(
    <AbwesenheitenTab
      staffId="4"
      canEdit
      canEditQuota={props.canEditQuota ?? true}
      canManageSickReports
      staff={{ id: "4", firstName: "Rena", lastName: "Generation" }}
    />,
  );
}

describe("AbwesenheitenTab Kontingente (#3256)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getVacationQuota.mockImplementation((_id: string, y: number) =>
      Promise.resolve({
        staff_id: 4,
        year: y,
        entitled_days: 28,
        carryover_days: 2,
        taken_before_days: 3,
        taken_days: 5,
        reserved_days: 2,
        remaining_days: 20,
      }),
    );
    mocks.getAbsences.mockResolvedValue([]);
    mocks.getAbsenceTypes.mockResolvedValue([regeneration]);
    mocks.getAllowance.mockImplementation(
      (_type: string, _id: string, y: number) =>
        Promise.resolve({ ...allowance, year: y }),
    );
    mocks.setAllowance.mockResolvedValue(allowance);
  });

  it("shows vacation and every own account with claim, taken, reserved and rest", async () => {
    renderTab();

    const vacation = await screen.findByRole("region", {
      name: `Urlaub ${year}`,
    });
    expect(within(vacation).getByText("30 Tage")).toBeInTheDocument();
    expect(
      within(vacation).getByText("28 + 2 aus dem Vorjahr"),
    ).toBeInTheDocument();
    expect(within(vacation).getByText("8 Tage")).toBeInTheDocument();
    expect(
      within(vacation).getByText("davon 3 Tage vor moto"),
    ).toBeInTheDocument();
    expect(within(vacation).getByText("20 Tage")).toBeInTheDocument();

    const own = screen.getByRole("region", {
      name: `Regenerationstag ${year}`,
    });
    for (const label of ["Anspruch", "Genommen", "Vorgemerkt", "Übrig"]) {
      expect(within(own).getByText(label)).toBeInTheDocument();
    }
    expect(within(own).getByText("3,5 Tage")).toBeInTheDocument();
    expect(within(own).getByText("2 Tage")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Abwesenheitsarten verwalten/ }),
    ).toHaveAttribute("href", "/demo/database/absence-types");
  });

  it("raises a claim with plus and requires a reason", async () => {
    renderTab();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Anspruch ändern: Regenerationstag",
      }),
    );
    const days = screen.getByLabelText(`Anspruch ${year} in Tagen`);
    expect(days).toHaveValue("3,5");

    const plus = screen.getByRole("button", { name: "Einen Tag mehr" });
    fireEvent.click(plus);
    fireEvent.click(plus);
    expect(days).toHaveValue("5,5");

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      screen.getByText("Bitte kurz sagen, warum sich der Anspruch ändert."),
    ).toBeInTheDocument();
    expect(mocks.setAllowance).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "In den Sommerferien krank" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mocks.setAllowance).toHaveBeenCalledWith("12", "4", {
        year,
        entitledDays: 5.5,
        reason: "In den Sommerferien krank",
      }),
    );
  });

  it("does not let a claim drop below the days already used", async () => {
    renderTab();
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Anspruch ändern: Regenerationstag",
      }),
    );
    // 0,5 taken + 1 reserved: one step down from 3,5 is still fine, the
    // next one would cut into used days.
    const minus = screen.getByRole("button", { name: "Einen Tag weniger" });
    fireEvent.click(minus);
    fireEvent.click(minus);
    expect(screen.getByLabelText(`Anspruch ${year} in Tagen`)).toHaveValue(
      "1,5",
    );
    expect(minus).toBeDisabled();

    fireEvent.change(screen.getByLabelText(`Anspruch ${year} in Tagen`), {
      target: { value: "1" },
    });
    fireEvent.change(screen.getByLabelText("Begründung"), {
      target: { value: "Teilzeit" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      screen.getByText(
        "Mindestens 1,5 Tage. So viele sind schon eingetragen oder beantragt.",
      ),
    ).toBeInTheDocument();
    expect(mocks.setAllowance).not.toHaveBeenCalled();
  });

  it("loads all accounts for the chosen year", async () => {
    renderTab();
    await screen.findByRole("region", { name: `Urlaub ${year}` });

    fireEvent.click(screen.getByRole("button", { name: String(year + 1) }));

    expect(
      await screen.findByRole("region", { name: `Urlaub ${year + 1}` }),
    ).toBeInTheDocument();
    expect(mocks.getVacationQuota).toHaveBeenCalledWith("4", year + 1);
    expect(mocks.getAllowance).toHaveBeenCalledWith("12", "4", year + 1);
  });

  it("keeps a retired account visible but out of new bookings", async () => {
    mocks.getAbsenceTypes.mockResolvedValue([
      { ...regeneration, isActive: false },
    ]);
    renderTab();

    expect(
      await screen.findByRole("region", { name: `Regenerationstag ${year}` }),
    ).toBeInTheDocument();
    expect(screen.getByText("ausgeschaltet")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Abwesenheit eintragen" }),
    );
    const dialog = screen.getByRole("dialog", {
      name: "Abwesenheit eintragen: Rena Generation",
    });
    expect(
      within(dialog).getByRole("radio", { name: /^Urlaub/ }),
    ).toBeInTheDocument();
    expect(
      within(dialog).queryByRole("radio", { name: /Regenerationstag/ }),
    ).not.toBeInTheDocument();
  });

  it("points to the art catalog when no own account exists", async () => {
    mocks.getAbsenceTypes.mockResolvedValue([]);
    renderTab();

    expect(
      await screen.findByText(
        /Regenerationstage oder Krank-Urlaubstage zählen/,
      ),
    ).toBeInTheDocument();
  });

  it("hides every edit entry without time_tracking:manage", async () => {
    renderTab({ canEditQuota: false });

    await screen.findByRole("region", { name: `Urlaub ${year}` });
    expect(
      screen.queryByRole("button", { name: /Anspruch ändern/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /Abwesenheitsarten verwalten/ }),
    ).not.toBeInTheDocument();
  });
});
