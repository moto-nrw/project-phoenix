import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

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
  getAbsences: vi.fn(),
  rebookAbsences: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  AbsenceRebookingBlockedError: class extends Error {},
  staffAbsenceService: {
    getVacationQuota: vi.fn().mockResolvedValue({
      staff_id: 4,
      year: 2026,
      entitled_days: 30,
      carryover_days: 0,
      taken_before_days: 0,
      taken_days: 0,
      reserved_days: 0,
      remaining_days: 30,
    }),
    getAbsences: mocks.getAbsences,
    rebookAbsences: mocks.rebookAbsences,
    getCompTimePreview: vi.fn(),
    approve: vi.fn(),
    deleteAbsence: vi.fn(),
  },
}));
vi.mock("~/lib/absence-type-api", () => ({
  absenceTypeService: {
    getAbsenceTypes: vi.fn().mockResolvedValue([
      {
        id: "12",
        name: "Krank-Urlaubstag",
        baseType: "other",
        isActive: true,
        allowanceEnabled: true,
        carryoverUntil: "03-31",
      },
    ]),
    getAllowance: vi.fn().mockResolvedValue({
      staffId: "4",
      absenceTypeId: "12",
      year: 2026,
      entitledDays: 10,
      takenDays: 0,
      reservedDays: 0,
      remainingDays: 10,
      expiresOn: "2027-03-31",
      expiredDays: 0,
      bookingDays: 0,
      carriedIn: null,
    }),
  },
}));

import { AbwesenheitenTab } from "./abwesenheiten-tab";

// Test clock: 2026-09-09. Two past Fridays on Freizeitausgleich and a past
// sick report, which cannot be rebooked.
function row(id: number, day: string, absenceType: string) {
  return {
    id,
    staff_id: 4,
    absence_type: absenceType,
    date_start: day,
    date_end: day,
    half_day: false,
    note: "",
    status: "reported",
  };
}

function history(): HTMLElement {
  const section = screen
    .getByRole("heading", { name: "Historie 2026" })
    .closest("section");
  expect(section).not.toBeNull();
  return section!;
}

describe("AbwesenheitenTab Art ändern (#3258)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getAbsences.mockResolvedValue([
      row(71, "2026-08-07", "comp_time"),
      row(72, "2026-08-14", "comp_time"),
      row(73, "2026-08-19", "sick"),
    ]);
    mocks.rebookAbsences.mockResolvedValue({
      absences: [],
      days: 1,
      balanceDeltaMinutes: 480,
      allowances: [{ year: 2026, remainingDays: 9, bookingDays: 1 }],
      allowanceExceeded: false,
      vacation: [],
      vacationExceeded: false,
      applied: true,
    });
  });

  it("wählt Einträge aus und bucht sie mit Grund um", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Swantje", lastName: "Kollegin" }}
      />,
    );

    const start = await screen.findByRole("button", { name: "Art ändern" });
    fireEvent.click(start);

    const list = history();
    // Krankmeldungen bekommen kein Häkchen.
    expect(within(list).getAllByRole("checkbox")).toHaveLength(2);
    const next = within(list).getByRole("button", { name: "Weiter" });
    expect(next).toBeDisabled();

    fireEvent.click(
      within(list).getByRole("checkbox", {
        name: /Freizeitausgleich 07\.08\.2026 auswählen/,
      }),
    );
    expect(within(list).getByText("1 Eintrag gewählt")).toBeInTheDocument();
    fireEvent.click(next);

    const dialog = await screen.findByRole("dialog", {
      name: "Art ändern: Swantje Kollegin",
    });
    fireEvent.click(
      within(dialog).getByRole("radio", { name: /Krank-Urlaubstag/ }),
    );
    await within(dialog).findByText("+8h");
    fireEvent.change(within(dialog).getByLabelText("Grund (Pflicht)"), {
      target: { value: "Kontingent angelegt" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Art ändern" }));

    await waitFor(() =>
      expect(mocks.rebookAbsences).toHaveBeenLastCalledWith("4", {
        absenceIds: [71],
        absenceType: "other",
        absenceTypeId: "12",
        reason: "Kontingent angelegt",
        dryRun: false,
      }),
    );
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
    );
    // Nach dem Speichern ist der Auswahlmodus vorbei.
    expect(within(history()).queryAllByRole("checkbox")).toHaveLength(0);
  });

  it("verlässt den Auswahlmodus mit Abbrechen", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota
        canManageSickReports
        staff={{ id: "4", firstName: "Swantje", lastName: "Kollegin" }}
      />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Art ändern" }));
    fireEvent.click(
      within(history()).getByRole("button", { name: "Abbrechen" }),
    );

    expect(within(history()).queryAllByRole("checkbox")).toHaveLength(0);
    expect(
      screen.getByRole("button", { name: "Art ändern" }),
    ).toBeInTheDocument();
  });

  it("zeigt das Umbuchen ohne Recht zum Eintragen nicht", async () => {
    render(
      <AbwesenheitenTab
        staffId="4"
        canEdit
        canEditQuota={false}
        canManageSickReports={false}
      />,
    );

    await screen.findByRole("heading", { name: "Historie 2026" });
    expect(
      screen.queryByRole("button", { name: "Art ändern" }),
    ).not.toBeInTheDocument();
  });
});
