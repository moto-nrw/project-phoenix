// #6593: Speichern, Abwesenheit speichern und löschen sperren das Panel
// „Eintrag bearbeiten“; eine späte Antwort schließt nichts mehr.
import {
  act,
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { service, mockMutate, mockToastSuccess } = vi.hoisted(() => ({
  service: {
    checkIn: vi.fn(),
    checkOut: vi.fn(),
    getCurrentSession: vi.fn(),
    getHistory: vi.fn(),
    startBreak: vi.fn(),
    endBreak: vi.fn(),
    getSessionBreaks: vi.fn(),
    getSessionEdits: vi.fn(),
    updateSession: vi.fn(),
    getAbsences: vi.fn(),
    createAbsence: vi.fn(),
    updateAbsence: vi.fn(),
    deleteAbsence: vi.fn(),
    requestVacation: vi.fn(),
    cancelAbsence: vi.fn(),
    getVacationQuota: vi.fn(),
    getQuestionedAbsences: vi.fn(),
    getOwnAbsenceAllowances: vi.fn(),
    exportSessions: vi.fn(),
  },
  mockMutate: vi.fn(),
  mockToastSuccess: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1", token: "test-token" } },
    status: "authenticated",
    update: vi.fn(),
  }),
}));
vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: () => ({
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  }),
}));
vi.mock("~/lib/swr", () => ({
  useLiveRefreshInterval: (fallbackMs = 60_000) => fallbackMs,
  useSWRAuth: vi.fn(),
  useTenantMutate: () => vi.fn(),
  useTenantMutateMatching: () => vi.fn(),
  useInvalidateMatching: () => vi.fn(),
}));
vi.mock("~/lib/hooks/use-user-context", () => ({
  useUserContext: () => ({
    userContext: { currentStaff: { id: "10", personId: "1" } },
    isLoading: false,
    error: undefined,
    isIncomplete: false,
    unavailableSections: [],
    isReady: true,
  }),
}));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));
vi.mock("~/lib/time-tracking-api", () => ({
  DEVIATION_REASON_REQUIRED_CODE: "iot.deviation_reason_required",
  PLANNED_START_NOT_REACHED_CODE: "iot.planned_start_not_reached",
  REOPEN_STATUS_CONFLICT_CODE: "reopen_status_conflict",
  timeTrackingService: service,
}));
vi.mock("~/lib/tenant-context", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/tenant-context")>()),
  useModuleBooked: () => true,
}));
vi.mock("~/components/staff/staff-export-button", () => ({
  StaffExportButton: () => null,
}));
// Wie page.test.tsx: die Tabelle reicht nur den Stift „Eintrag bearbeiten“
// an die Seite durch; das Panel selbst ist echt.
vi.mock("~/components/staff/staff-session-table", () => ({
  StaffSessionTable: ({
    sessions = [],
    absences = [],
    onEditDay,
  }: {
    sessions?: Array<{ id?: number; date: string }>;
    absences?: Array<{ date_start: string }>;
    onEditDay?: (
      date: Date,
      session: { id?: number } | null,
      absence: unknown,
    ) => void;
  }) => {
    const session = sessions[0];
    const absence = absences[0];
    const date = new Date(
      `${session?.date ?? absence?.date_start ?? "2026-09-09"}T12:00:00`,
    );
    return (
      <button
        type="button"
        onClick={() => onEditDay?.(date, session ?? null, absence ?? null)}
      >
        Eintrag bearbeiten
      </button>
    );
  },
}));
vi.mock("~/components/ui/chart", () => ({
  ChartContainer: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
  ChartLegend: () => null,
  ChartLegendContent: () => null,
  ChartTooltip: () => null,
  ChartTooltipContent: () => null,
}));
vi.mock("recharts", () => ({
  Bar: () => null,
  BarChart: ({ children }: { children: ReactNode }) => <div>{children}</div>,
  CartesianGrid: () => null,
  XAxis: () => null,
  YAxis: () => null,
  ResponsiveContainer: ({ children }: { children: ReactNode }) => (
    <div>{children}</div>
  ),
}));
vi.mock("~/components/help/context-help-link", () => ({
  ContextHelpLink: () => null,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { useSWRAuth } from "~/lib/swr";
import type {
  StaffAbsence,
  WorkSessionHistory,
} from "~/lib/time-tracking-helpers";
import TimeTrackingPage from "./page";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

// Die Testuhr steht auf Mittwoch, 09.09.2026.
const DAY = "2026-09-09";

const session: WorkSessionHistory = {
  id: "100",
  staffId: "10",
  date: DAY,
  status: "present",
  checkInTime: `${DAY}T08:00:00`,
  checkOutTime: `${DAY}T16:30:00`,
  breakMinutes: 30,
  notes: "",
  autoCheckedOut: false,
  createdBy: "10",
  updatedBy: null,
  createdAt: `${DAY}T08:00:00`,
  updatedAt: `${DAY}T08:00:00`,
  netMinutes: 480,
  isOvertime: false,
  isBreakCompliant: true,
  restPeriodWarning: null,
  breaks: [],
  editCount: 0,
};

const absence: StaffAbsence = {
  id: "200",
  staffId: "10",
  absenceType: "sick",
  dateStart: DAY,
  dateEnd: DAY,
  halfDay: false,
  startHalfDay: false,
  endHalfDay: false,
  note: "",
  status: "pending",
  approvedBy: null,
  approvedAt: null,
  createdBy: "10",
  createdAt: `${DAY}T07:00:00`,
  updatedAt: `${DAY}T07:00:00`,
  durationDays: 1,
  workingDays: 1,
  decisionNote: "",
  requestedAt: `${DAY}T07:00:00`,
  substituteStaffId: null,
};

function mockData(absences: StaffAbsence[]) {
  const history = { sessions: [session], weeklySummaries: [], laborDays: [] };
  vi.mocked(useSWRAuth).mockImplementation(((key: string | null) => {
    const answer = (data: unknown) => ({
      data,
      isLoading: false,
      isValidating: false,
      mutate: mockMutate,
      error: undefined,
    });
    if (key === null) return answer(undefined);
    if (key === "time-tracking-current") return answer(null);
    if (key === "time-tracking-config") {
      return answer({ accountStartDate: "" });
    }
    if (key.startsWith("time-tracking-schedule-targets")) {
      return answer(new Map<string, number>());
    }
    if (key.startsWith("time-tracking-month-summary")) return answer(undefined);
    if (key.startsWith("time-tracking-table-absences")) return answer(absences);
    if (key.startsWith("time-tracking-absences-")) return answer(absences);
    if (
      key.startsWith("time-tracking-history") ||
      key.startsWith("time-tracking-table-") ||
      key.startsWith("time-tracking-own-history")
    ) {
      return answer(history);
    }
    if (key.startsWith("me-staff")) return answer({ id: "10" });
    if (key.startsWith("time-tracking-own-schedule")) return answer(null);
    if (key.startsWith("time-tracking-own-absences")) return answer([]);
    if (key === "staff-absence-types") return answer([]);
    return answer(absences);
  }) as never);
  service.getSessionBreaks.mockResolvedValue([]);
  service.getSessionEdits.mockResolvedValue([]);
  service.getAbsences.mockResolvedValue(absences);
  service.getQuestionedAbsences.mockResolvedValue([]);
  service.getOwnAbsenceAllowances.mockResolvedValue([]);
  service.getVacationQuota.mockResolvedValue({
    staff_id: 10,
    year: 2026,
    entitled_days: 30,
    carryover_days: 0,
    taken_days: 0,
    reserved_days: 0,
    remaining_days: 30,
  });
}

async function openPanel(title: string) {
  fireEvent.click(screen.getByRole("button", { name: "Eintrag bearbeiten" }));
  await screen.findByRole("heading", { name: title });
}

async function startSessionSave() {
  mockData([]);
  const answer = deferred<unknown>();
  service.updateSession.mockReturnValue(answer.promise);
  const view = render(<TimeTrackingPage />);
  await openPanel("Eintrag bearbeiten");
  fireEvent.click(screen.getByRole("button", { name: "Zeitkorrektur" }));
  const save = screen.getByRole("button", { name: "Speichern" });
  expect(save).toBeEnabled();
  fireEvent.click(save);
  await screen.findByRole("button", { name: "Speichern…" });
  return { ...view, answer };
}

describe("Zeiterfassung: Eintrag bearbeiten sperrt beim Speichern (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockResolvedValue(undefined);
  });

  it("locks the panel while saving the times and unlocks it with the error after a failure", async () => {
    const { answer } = await startSessionSave();

    expectSlideOverSaveLocked("Eintrag bearbeiten", [
      "Abbrechen",
      "Speichern…",
    ]);

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectSlideOverUnlocked("Eintrag bearbeiten");
    expect(screen.getByLabelText(/Grund der Änderung/)).toHaveValue(
      "Zeitkorrektur",
    );
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("closes the panel after the times are saved", async () => {
    const { answer } = await startSessionSave();

    await act(async () => {
      answer.resolve(session);
    });

    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Die Arbeitszeit ist gespeichert.",
    );
    // Anker: die Zeiterfassung ist wieder bedienbar.
    expect(
      screen.getByRole("button", { name: "Eintrag bearbeiten" }),
    ).toBeEnabled();
    expect(
      screen.queryByRole("heading", { name: "Eintrag bearbeiten" }),
    ).toBeNull();
  });

  it("locks the panel while the absence is deleted and refreshes after unmount", async () => {
    mockData([absence]);
    const answer = deferred<void>();
    service.deleteAbsence.mockReturnValue(answer.promise);
    const { unmount } = render(<TimeTrackingPage />);
    await openPanel("Tag bearbeiten");
    const panel = screen
      .getByRole("heading", { name: "Tag bearbeiten" })
      .closest<HTMLElement>('[role="dialog"]')!;
    fireEvent.click(within(panel).getByRole("button", { name: "Abwesenheit" }));
    const remove = await screen.findByRole("button", {
      name: "Abwesenheit löschen",
    });
    expect(remove).toBeEnabled();
    fireEvent.click(remove);

    expectSlideOverSaveLocked("Tag bearbeiten", ["Abbrechen", "Speichern"]);
    expect(
      screen.getByRole("button", { name: "Abwesenheit wird gelöscht…" }),
    ).toBeDisabled();

    unmount();
    await act(async () => {
      answer.resolve();
    });

    expect(service.deleteAbsence).toHaveBeenCalledWith("200");
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Die Abwesenheit ist gelöscht.",
    );
  });

  it("locks the panel while the absence is saved and unlocks it after a failure", async () => {
    mockData([absence]);
    const answer = deferred<void>();
    service.updateAbsence.mockReturnValue(answer.promise);
    render(<TimeTrackingPage />);
    await openPanel("Tag bearbeiten");
    const panel = screen
      .getByRole("heading", { name: "Tag bearbeiten" })
      .closest<HTMLElement>('[role="dialog"]')!;
    fireEvent.click(within(panel).getByRole("button", { name: "Abwesenheit" }));
    fireEvent.change(await screen.findByLabelText("Bemerkung (optional)"), {
      target: { value: "Arzttermin" },
    });
    const save = within(panel).getByRole("button", { name: "Speichern" });
    expect(save).toBeEnabled();
    fireEvent.click(save);
    await screen.findByRole("button", { name: "Speichern…" });

    expectSlideOverSaveLocked("Tag bearbeiten", ["Abbrechen", "Speichern…"]);
    expect(
      screen.getByRole("button", { name: "Abwesenheit löschen" }),
    ).toBeDisabled();

    await act(async () => {
      answer.reject(new ApiError("down", 503, { code: "general.unavailable" }));
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectSlideOverUnlocked("Tag bearbeiten");
    expect(screen.getByLabelText("Bemerkung (optional)")).toHaveValue(
      "Arzttermin",
    );
  });
});
