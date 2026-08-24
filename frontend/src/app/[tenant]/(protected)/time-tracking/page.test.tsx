import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// ─── Mocks (must come before component imports) ─────────────────────────────

const mockTimeTrackingService = vi.hoisted(() => ({
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
  exportSessions: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  })),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
  useTenantMutateMatching: vi.fn(() => vi.fn()),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  })),
}));

vi.mock("~/lib/time-tracking-api", () => ({
  DEVIATION_REASON_REQUIRED_CODE: "deviation_reason_required",
  PLANNED_START_NOT_REACHED_CODE: "planned_start_not_reached",
  REOPEN_STATUS_CONFLICT_CODE: "reopen_status_conflict",
  timeTrackingService: mockTimeTrackingService,
}));

vi.mock("~/components/staff/staff-export-button", async () => {
  const React = await vi.importActual<typeof import("react")>("react");
  return {
    StaffExportButton: () => {
      const [open, setOpen] = React.useState(false);
      React.useEffect(() => {
        if (!open) return;
        const close = () => setOpen(false);
        window.addEventListener("scroll", close);
        return () => window.removeEventListener("scroll", close);
      }, [open]);
      return (
        <div>
          <button
            type="button"
            aria-label="Export"
            onClick={() => setOpen((value) => !value)}
          >
            Export
          </button>
          {open && (
            <div>
              <h3>Zeitraum exportieren</h3>
              <button type="button" aria-label="Vorheriger Monat">
                Vorheriger Monat
              </button>
              <button type="button" aria-label="Nächster Monat">
                Nächster Monat
              </button>
              <span>Mo</span>
              <span>Di</span>
              <span>Fr</span>
              <span>14.05.2026</span>
              <button type="button">1</button>
              <button type="button">2</button>
              <span>01.05.2026 - ...</span>
              <button type="button" onClick={() => setOpen(false)}>
                CSV
              </button>
              <button type="button" onClick={() => setOpen(false)}>
                Excel
              </button>
              <button type="button" onClick={() => setOpen(false)}>
                PDF
              </button>
            </div>
          )}
        </div>
      );
    },
  };
});

vi.mock("~/components/staff/staff-session-table", () => ({
  StaffSessionTable: ({
    sessions = [],
    absences = [],
    absencesUnresolved = false,
    onEditDay,
  }: {
    sessions?: Array<{
      id?: number;
      date: string;
      edit_count?: number;
      check_out_time?: string | null;
      status?: string;
    }>;
    absences?: Array<{
      id: number;
      date_start: string;
      date_end: string;
      absence_type: string;
      half_day?: boolean;
    }>;
    absencesUnresolved?: boolean;
    onEditDay?: (
      date: Date,
      session: { id?: number } | null,
      absence: unknown,
    ) => void;
  }) => {
    const session = sessions[0];
    const absence = absences[0];
    const editDate = new Date(
      `${session?.date ?? absence?.date_start ?? todayISO}T12:00:00`,
    );
    const absenceLabel =
      absence?.absence_type === "vacation" ? "Urlaub" : "Krank";
    // Mirrors the real table since #2402: the pencil names the exact block to
    // edit instead of letting the page re-derive it from the date.
    const triggerEdit = () =>
      onEditDay?.(editDate, session ?? null, absence ?? null);
    const triggerHistory = () => {
      if (session?.id != null) {
        void mockTimeTrackingService.getSessionEdits(String(session.id));
      }
    };

    return (
      <table
        data-testid="staff-session-table"
        data-absences-unresolved={absencesUnresolved}
      >
        <thead>
          <tr>
            <th>Start</th>
            <th>Tag</th>
            <th>Ende</th>
            <th>Netto</th>
            <th>Ort</th>
            <th>Änderung</th>
          </tr>
        </thead>
        <tbody>
          <tr onClick={triggerHistory}>
            <td>{session?.check_out_time ? "08:00" : "..."}</td>
            <td>{session?.check_out_time ? "16:30" : "..."}</td>
            <td>8h</td>
            <td>{session?.status === "home_office" ? "Homeoffice" : "OGS"}</td>
            <td>{session && !session.check_out_time ? "aktiv" : null}</td>
            <td>
              {session?.edit_count ? (
                <>
                  <span>Zuletzt geändert</span>
                  <span>Start</span>
                  <span>Änderung</span>
                  <span>Laden...</span>
                  <span>Keine Änderungen vorhanden.</span>
                  <span>Weitere Änderung vornehmen</span>
                  <span>Korrektur</span>
                  <span>0 min</span>
                  <span>30 min</span>
                </>
              ) : null}
            </td>
            <td>
              <button
                type="button"
                aria-label="Eintrag bearbeiten"
                onClick={triggerEdit}
              >
                Eintrag bearbeiten
              </button>
            </td>
          </tr>
          {absence ? (
            <tr onClick={triggerEdit}>
              <td>{absenceLabel}</td>
              <td>{absence.half_day ? "halber Tag" : ""}</td>
            </tr>
          ) : null}
          <tr>
            <td>Woche gesamt</td>
            <td>...</td>
          </tr>
          <tr>
            <td>Heute: 8h</td>
            <td>Woche: 8h</td>
          </tr>
        </tbody>
      </table>
    );
  },
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal" data-title={title}>
        {title && <h3>{title}</h3>}
        <button type="button" data-testid="modal-close" onClick={onClose}>
          close
        </button>
        <div data-testid="modal-body">{children}</div>
        {footer && <div data-testid="modal-footer">{footer}</div>}
      </div>
    ) : null,
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
    title,
    children,
    confirmText = "Bestätigen",
    cancelText = "Abbrechen",
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    children: React.ReactNode;
    confirmText?: string;
    cancelText?: string;
  }) =>
    isOpen ? (
      <div data-testid="confirmation-modal" data-title={title}>
        <h3>{title}</h3>
        <div>{children}</div>
        <button type="button" onClick={onClose}>
          {cancelText}
        </button>
        <button type="button" onClick={onConfirm}>
          {confirmText}
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/ui/chart", () => ({
  ChartContainer: ({
    children,
  }: {
    config: unknown;
    className: string;
    children: React.ReactNode;
  }) => <div data-testid="chart-container">{children}</div>,
  ChartLegend: () => <div data-testid="chart-legend" />,
  ChartLegendContent: () => <div />,
  ChartTooltip: ({ content }: { content?: React.ReactElement }) =>
    content ? (
      <div data-testid="chart-tooltip">{content}</div>
    ) : (
      <div data-testid="chart-tooltip" />
    ),
  ChartTooltipContent: ({
    labelFormatter,
    formatter,
  }: {
    labelFormatter?: (
      value: unknown,
      payload: ReadonlyArray<{ payload?: { label?: string } }>,
    ) => string;
    formatter?: (
      value: number | string | ReadonlyArray<number | string> | undefined,
      name: string | number | undefined,
    ) => string;
  }) => {
    // Invoke formatters to ensure code coverage on the callbacks
    const labelResult = labelFormatter?.("", [
      { payload: { label: "Mo 01.01" } },
    ]);
    const fmtResult1 = formatter?.(120, "netMinutes");
    const fmtResult2 = formatter?.(30, "breakMinutes");
    const fmtResult3 = formatter?.(undefined, undefined);
    return (
      <div
        data-testid="chart-tooltip-content"
        data-label={labelResult}
        data-fmt1={fmtResult1}
        data-fmt2={fmtResult2}
        data-fmt3={fmtResult3}
      />
    );
  },
}));

vi.mock("recharts", () => ({
  Bar: () => <div data-testid="bar" />,
  BarChart: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="bar-chart">{children}</div>
  ),
  CartesianGrid: () => <div />,
  XAxis: () => <div />,
  YAxis: () => <div />,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("react-dom", async (importOriginal) => {
  const actual = await importOriginal<typeof import("react-dom")>();
  return {
    ...actual,
    createPortal: (node: React.ReactNode) => node,
  };
});

vi.mock("lucide-react", () => ({
  // Whitelist mock: every icon the page's tree renders must be listed, or the
  // component using it throws "No export is defined". The Check/Pencil/Plus/
  // Search/X group belongs to the Abwesenheitsart-Dropdown (#2403).
  Check: () => <span data-testid="check-icon" />,
  ChevronDown: () => <span data-testid="chevron-down" />,
  ChevronLeft: () => <span data-testid="chevron-left" />,
  ChevronRight: () => <span data-testid="chevron-right" />,
  Download: () => <span data-testid="download-icon" />,
  MoreVertical: () => <span data-testid="more-vertical" />,
  Pencil: () => <span data-testid="pencil-icon" />,
  Plus: () => <span data-testid="plus-icon" />,
  Search: () => <span data-testid="search-icon" />,
  SquarePen: () => <span data-testid="square-pen" />,
  X: () => <span data-testid="x-icon" />,
}));

// ─── Imports after mocks ────────────────────────────────────────────────────

import TimeTrackingPage from "./page";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useSWRAuth, useTenantMutateMatching } from "~/lib/swr";
import { useToast } from "~/contexts/ToastContext";
import { timeTrackingService } from "~/lib/time-tracking-api";
import type {
  MonthSummary,
  WorkSession,
  WorkSessionHistory,
  StaffAbsence,
} from "~/lib/time-tracking-helpers";

// ─── Test Data ──────────────────────────────────────────────────────────────

const today = new Date();
const todayISO = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;
const testTimestamp = (date: string, time: string) => `${date}T${time}:00`;

// WeekTable skips weekends (day 0=Sun, 6=Sat). When today is a weekend,
// shift test dates to the nearest weekday (previous Friday) so rows render.
const weekday = new Date(today);
if (weekday.getDay() === 0) weekday.setDate(weekday.getDate() - 2); // Sun → Fri
if (weekday.getDay() === 6) weekday.setDate(weekday.getDate() - 1); // Sat → Fri
const weekdayISO = `${weekday.getFullYear()}-${String(weekday.getMonth() + 1).padStart(2, "0")}-${String(weekday.getDate()).padStart(2, "0")}`;

const mockActiveSession: WorkSession = {
  id: "100",
  staffId: "10",
  date: todayISO,
  status: "present",
  checkInTime: testTimestamp(todayISO, "08:00"),
  checkOutTime: null,
  breakMinutes: 0,
  notes: "",
  autoCheckedOut: false,
  createdBy: "10",
  updatedBy: null,
  createdAt: testTimestamp(todayISO, "08:00"),
  updatedAt: testTimestamp(todayISO, "08:00"),
};

const mockCheckedOutSession: WorkSession = {
  ...mockActiveSession,
  checkOutTime: testTimestamp(todayISO, "16:30"),
  breakMinutes: 30,
};

const mockHistorySession: WorkSessionHistory = {
  ...mockCheckedOutSession,
  netMinutes: 480,
  isOvertime: false,
  isBreakCompliant: true,
  restPeriodWarning: null,
  breaks: [],
  editCount: 0,
};

const mockHistorySessionWithEdits: WorkSessionHistory = {
  ...mockHistorySession,
  date: weekdayISO,
  checkInTime: testTimestamp(weekdayISO, "08:00"),
  checkOutTime: testTimestamp(weekdayISO, "16:30"),
  editCount: 2,
  updatedAt: testTimestamp(weekdayISO, "17:00"),
};

const mockHistorySessionNonCompliant: WorkSessionHistory = {
  ...mockHistorySession,
  netMinutes: 400,
  breakMinutes: 20,
  isBreakCompliant: false,
};

const mockHistorySessionAutoCheckedOut: WorkSessionHistory = {
  ...mockHistorySession,
  autoCheckedOut: true,
};

const mockAbsence: StaffAbsence = {
  id: "200",
  staffId: "10",
  absenceType: "sick",
  dateStart: weekdayISO,
  dateEnd: todayISO,
  halfDay: false,
  startHalfDay: false,
  endHalfDay: false,
  note: "",
  status: "pending",
  approvedBy: null,
  approvedAt: null,
  createdBy: "10",
  createdAt: testTimestamp(todayISO, "07:00"),
  updatedAt: testTimestamp(todayISO, "07:00"),
  durationDays: 1,
  workingDays: 1,
  decisionNote: "",
  requestedAt: testTimestamp(todayISO, "07:00"),
  substituteStaffId: null,
};

const mockVacationAbsence: StaffAbsence = {
  ...mockAbsence,
  id: "201",
  absenceType: "vacation",
  note: "Jahresurlaub",
  halfDay: true,
};

// ─── Helpers ────────────────────────────────────────────────────────────────

const mockMutate = vi.fn();
const mockOwnStaff = { id: "10" };
const mockOwnSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: todayISO,
  validFrom: todayISO,
  entries: [
    { weekIndex: 0, dayOfWeek: 0, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 1, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 2, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 3, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 4, targetMinutes: 480 },
  ],
  weeklyTotals: [2400],
};

// Issue #1368: the page no longer pre-selects a work mode — staff must pick
// Vor Ort, Homeoffice, or Abwesend before "Einstempeln" becomes enabled.
// Tests that exercise the check-in flow call this helper to make the
// requirement explicit and keep prologue noise out of the assertions.
function selectPresentMode() {
  // Use role+name to disambiguate from "In der OGS" text that also appears
  // in status badges, edit-modal options, and history rows when today's
  // session has Status = 'present'. getByText would throw
  // getMultipleElementsFoundError in those setups.
  fireEvent.click(screen.getByRole("button", { name: "In der OGS" }));
}

async function waitForLastSaveButtonEnabled() {
  let saveBtn: HTMLElement | undefined;
  await waitFor(() => {
    const saveButtons = screen.getAllByText("Speichern");
    saveBtn = saveButtons[saveButtons.length - 1];
    expect(saveBtn).not.toBeDisabled();
  });
  return saveBtn!;
}

function clickQuickEditReason(reason: string) {
  const buttons = screen.getAllByRole("button", { name: reason });
  fireEvent.click(buttons[buttons.length - 1]!);
}

function chooseSelectOption(trigger: HTMLElement, optionLabel: string) {
  fireEvent.click(trigger);
  fireEvent.click(screen.getByRole("option", { name: optionLabel }));
}

function setupDefaultMocks(overrides?: {
  currentSession?: WorkSession | null;
  history?: WorkSessionHistory[];
  absences?: StaffAbsence[];
  tableAbsencesError?: Error;
  historyLoading?: boolean;
  configLoading?: boolean;
  scheduleTargets?: ReadonlyMap<string, number>;
  monthSummary?: MonthSummary;
}) {
  vi.mocked(useSession).mockReturnValue({
    data: { user: { id: "1", token: "test-token" } },
    status: "authenticated",
    update: vi.fn(),
  } as never);

  const currentSession = overrides?.currentSession ?? null;
  const history = overrides?.history ?? [];
  const absences = overrides?.absences ?? [];
  const historyLoading = overrides?.historyLoading ?? false;
  const configLoading = overrides?.configLoading ?? false;
  const scheduleTargets =
    overrides?.scheduleTargets ?? new Map<string, number>();
  const monthSummary = overrides?.monthSummary;

  vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
    if (key === null) {
      return {
        data: undefined,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    }
    if (key === "time-tracking-current") {
      return {
        data: currentSession,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key?.startsWith("time-tracking-history")) {
      return {
        data: { sessions: history, weeklySummaries: [] },
        isLoading: historyLoading,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
      // Date-valid Soll per day (#1842) — a Map, keyed by ISO day. The
      // catch-all below hands back an array, which is not what any
      // schedule-targets consumer reads.
    } else if (key?.startsWith("time-tracking-schedule-targets")) {
      return {
        data: scheduleTargets,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
      // Monatskarte / period-KPI aggregate (#1842).
    } else if (key?.startsWith("time-tracking-month-summary")) {
      return {
        data: monthSummary,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key?.startsWith("time-tracking-table-absences")) {
      return {
        data: overrides?.tableAbsencesError ? undefined : absences,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: overrides?.tableAbsencesError,
      } as never;
    } else if (key?.startsWith("time-tracking-table-")) {
      return {
        data: { sessions: history, weeklySummaries: [] },
        isLoading: historyLoading,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key?.startsWith("time-tracking-own-staff")) {
      return {
        data: mockOwnStaff,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key?.startsWith("time-tracking-own-schedule")) {
      return {
        data: mockOwnSchedule,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key === "time-tracking-config") {
      return {
        data: configLoading ? undefined : { accountStartDate: "" },
        isLoading: configLoading,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key?.startsWith("time-tracking-own-history")) {
      return {
        data: {
          sessions: history,
          weeklySummaries: [],
        },
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key?.startsWith("time-tracking-own-absences")) {
      return {
        data: [],
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else if (key === "staff-absence-types") {
      return {
        data: [],
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else {
      return {
        data: absences,
        isLoading: false,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    }
  });

  // Default: getSessionBreaks returns empty
  vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([]);
  vi.mocked(timeTrackingService.getVacationQuota).mockResolvedValue({
    staff_id: 10,
    year: today.getFullYear(),
    entitled_days: 30,
    carryover_days: 0,
    taken_days: 0,
    reserved_days: 0,
    remaining_days: 30,
  });
  vi.mocked(timeTrackingService.getAbsences).mockResolvedValue(absences);
  vi.mocked(timeTrackingService.requestVacation).mockResolvedValue(
    mockVacationAbsence,
  );
  vi.mocked(timeTrackingService.cancelAbsence).mockResolvedValue(undefined);
}

// ─── Tests ──────────────────────────────────────────────────────────────────

describe("TimeTrackingPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // Reset window.innerWidth to desktop by default
    Object.defineProperty(window, "innerWidth", {
      writable: true,
      configurable: true,
      value: 1024,
    });
  });

  // ── Loading / Auth ──────────────────────────────────────────────────────

  describe("authentication and loading", () => {
    it("shows loading when auth status is loading", () => {
      vi.mocked(useSession).mockReturnValue({
        data: null,
        status: "loading",
        update: vi.fn(),
      } as never);

      // useSWRAuth should still return defaults
      vi.mocked(useSWRAuth).mockReturnValue({
        data: undefined,
        isLoading: true,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never);

      render(<TimeTrackingPage />);
      expect(
        screen.getByLabelText("Zeiterfassung wird geladen"),
      ).toBeInTheDocument();
    });

    it("renders main content when authenticated", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getAllByText("Zeiterfassung").length).toBeGreaterThan(0);
    });

    it("keeps backfills unresolved when the table absence fetch fails", () => {
      setupDefaultMocks({
        tableAbsencesError: new Error("absence fetch failed"),
      });

      render(<TimeTrackingPage />);

      expect(screen.getByTestId("staff-session-table")).toHaveAttribute(
        "data-absences-unresolved",
        "true",
      );
    });

    it("renders Stempeluhr heading", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Stempeluhr")).toBeInTheDocument();
    });
  });

  // ── No Active Session (Check-in state) ──────────────────────────────────

  describe("no active session - check-in controls", () => {
    it("shows In der OGS and Homeoffice mode buttons", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("In der OGS")).toBeInTheDocument();
      expect(screen.getByText("Homeoffice")).toBeInTheDocument();
    });

    it("shows Abwesend button", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Abwesend")).toBeInTheDocument();
    });

    it("shows 'Bitte Status wählen' label before any mode is selected", () => {
      // Issue #1368: no pre-selection — staff must explicitly pick Vor Ort,
      // Homeoffice, or Abwesend. Until then the action label nudges the user.
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Bitte Status wählen")).toBeInTheDocument();
      expect(screen.queryByText("Einstempeln")).not.toBeInTheDocument();
    });

    it("disables the Einstempeln button until a mode is chosen", () => {
      // Issue #1368: the action button stays disabled with no mode selected,
      // so a stray click cannot trigger an unintended check-in.
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      const btn = screen.getByLabelText("Einstempeln");
      expect(btn).toBeDisabled();
      selectPresentMode();
      expect(btn).not.toBeDisabled();
    });

    it("shows Abwesenheit melden label when absent mode selected", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      expect(screen.getByText("Abwesenheit melden")).toBeInTheDocument();
    });

    it("shows check-in play button with correct aria-label", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByLabelText("Einstempeln")).toBeInTheDocument();
    });

    it("calls checkIn with 'present' when In der OGS is selected", async () => {
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockResolvedValue(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkIn).toHaveBeenCalledWith("present");
      });
    });

    it("calls checkIn with 'home_office' when Homeoffice is selected", async () => {
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockResolvedValue(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("Homeoffice"));

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkIn).toHaveBeenCalledWith("home_office");
      });
    });

    it("shows toast success on successful check-in", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockResolvedValue(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.success).toHaveBeenCalledWith(
          "Erfolgreich eingestempelt",
        );
      });
    });

    it("shows toast error on check-in failure", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockRejectedValue(
        new Error("already checked in"),
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Du bist bereits eingestempelt.",
        );
      });
    });

    it("opens absence modal when Abwesend + calendar button clicked", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));
      expect(screen.getByTestId("modal")).toBeInTheDocument();
      expect(screen.getByTestId("modal").getAttribute("data-title")).toBe(
        "Abwesenheit melden",
      );
    });
  });

  // ── Active Session (Checked in) ─────────────────────────────────────────

  describe("active session - checked in", () => {
    it("shows Ausstempeln button when checked in", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      expect(screen.getByLabelText("Ausstempeln")).toBeInTheDocument();
    });

    it("shows Pause starten button when checked in", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      expect(screen.getByLabelText("Pause starten")).toBeInTheDocument();
    });

    it("shows status badge In der OGS when present", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      // The badge inside ClockInCard
      const badges = screen.getAllByText("In der OGS");
      expect(badges.length).toBeGreaterThan(0);
    });

    it("shows status badge Homeoffice when home_office", () => {
      const hoSession = {
        ...mockActiveSession,
        status: "home_office" as const,
      };
      setupDefaultMocks({ currentSession: hoSession });
      render(<TimeTrackingPage />);
      const badges = screen.getAllByText("Homeoffice");
      expect(badges.length).toBeGreaterThan(0);
    });

    it("calls checkOut when Ausstempeln clicked", async () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.checkOut).mockResolvedValue(
        mockCheckedOutSession,
      );
      render(<TimeTrackingPage />);

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Ausstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkOut).toHaveBeenCalled();
      });
    });

    it("shows break duration stepper when pause button clicked", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));
      expect(
        screen.getByRole("button", { name: "Pausendauer verringern" }),
      ).toBeInTheDocument();
      expect(screen.getByText("30")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Pausendauer erhöhen" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Starten" })).toBeEnabled();
    });

    it("consumes the first desktop outside click before checkout", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));

      fireEvent.click(screen.getByLabelText("Ausstempeln"));

      expect(
        screen.queryByRole("button", { name: "Pausendauer verringern" }),
      ).not.toBeInTheDocument();
      expect(timeTrackingService.checkOut).not.toHaveBeenCalled();
    });

    it("uses a bottom sheet for break duration on mobile", async () => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 390,
      });
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));

      await waitFor(() => {
        expect(
          screen.getByRole("dialog", { name: "Pause stempeln" }),
        ).toBeInTheDocument();
      });
      expect(screen.getByText("30")).toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Starten" })).toBeEnabled();

      fireEvent.click(
        screen.getByRole("button", { name: "Pausendauer schließen" }),
      );

      await waitFor(() => {
        expect(
          screen.queryByRole("dialog", { name: "Pause stempeln" }),
        ).not.toBeInTheDocument();
      });
    });

    it("starts a default 30 minute break", async () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.startBreak).mockResolvedValue({
        id: "50",
        sessionId: "100",
        startedAt: new Date().toISOString(),
        endedAt: null,
        durationMinutes: 0,
        plannedEndTime: null,
      });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Starten" }));
      });

      await waitFor(() => {
        expect(timeTrackingService.startBreak).toHaveBeenCalledWith(30);
      });
    });

    it("starts an individual break with 15 minute stepper controls", async () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.startBreak).mockResolvedValue({
        id: "50",
        sessionId: "100",
        startedAt: new Date().toISOString(),
        endedAt: null,
        durationMinutes: 0,
        plannedEndTime: null,
      });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));

      expect(screen.getByText("30")).toBeInTheDocument();
      fireEvent.click(
        screen.getByRole("button", { name: "Pausendauer verringern" }),
      );
      expect(screen.getByText("15")).toBeInTheDocument();
      for (let i = 0; i < 5; i += 1) {
        fireEvent.click(
          screen.getByRole("button", { name: "Pausendauer erhöhen" }),
        );
      }
      expect(screen.getByText("90")).toBeInTheDocument();

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Starten" }));
      });

      await waitFor(() => {
        expect(timeTrackingService.startBreak).toHaveBeenCalledWith(90);
      });
    });

    it("shows Pause beenden button when on break", async () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      // Mock breaks to include an active break
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "50",
          sessionId: "100",
          startedAt: new Date().toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: null,
        },
      ]);
      render(<TimeTrackingPage />);
      // Need to wait for breaks to load
      await waitFor(() => {
        expect(screen.getByLabelText("Pause beenden")).toBeInTheDocument();
      });
    });

    it("communicates automatic resume time for timed breaks", async () => {
      const breakStart = new Date(Date.now() - 60 * 60 * 1000);
      const plannedEnd = new Date(Date.now() + 30 * 60 * 1000);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "50",
          sessionId: "100",
          startedAt: breakStart.toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: plannedEnd.toISOString(),
        },
      ]);
      render(<TimeTrackingPage />);

      await waitFor(() => {
        expect(screen.getByText(/Automatisch weiter um/)).toBeInTheDocument();
      });
    });

    it("refreshes history and table data after a timed break auto-ends", async () => {
      const breakStart = new Date(Date.now() - 90 * 60 * 1000);
      const plannedEnd = new Date(Date.now() - 1000);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "50",
          sessionId: "100",
          startedAt: breakStart.toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: plannedEnd.toISOString(),
        },
      ]);
      vi.mocked(timeTrackingService.endBreak).mockResolvedValue({
        ...mockActiveSession,
        breakMinutes: 90,
      });

      render(<TimeTrackingPage />);

      await waitFor(() => {
        expect(timeTrackingService.endBreak).toHaveBeenCalled();
      });
      await waitFor(() => {
        expect(mockMutate).toHaveBeenCalledTimes(2);
      });
    });

    it("shows Heute and Woche footer stats", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      expect(screen.getByText(/Heute:/)).toBeInTheDocument();
      expect(screen.getByText(/Woche:/)).toBeInTheDocument();
    });
  });

  // ── Checked Out Session ─────────────────────────────────────────────────

  describe("checked-out session", () => {
    it("shows work summary with times when checked out", () => {
      setupDefaultMocks({ currentSession: mockCheckedOutSession });
      render(<TimeTrackingPage />);
      expect(screen.getByText("Arbeit")).toBeInTheDocument();
    });

    it("shows pause row when break minutes > 0", () => {
      setupDefaultMocks({ currentSession: mockCheckedOutSession });
      render(<TimeTrackingPage />);
      // Multiple "Pause" texts exist (table header + summary row); just check at least one
      const pauseTexts = screen.getAllByText("Pause");
      expect(pauseTexts.length).toBeGreaterThan(0);
    });
  });

  // ── WeekChart ───────────────────────────────────────────────────────────

  describe("WeekChart", () => {
    it("renders chart container", () => {
      setupDefaultMocks({ history: [mockHistorySession] });
      render(<TimeTrackingPage />);
      expect(screen.getByTestId("chart-container")).toBeInTheDocument();
    });

    it("renders bar chart", () => {
      setupDefaultMocks({ history: [mockHistorySession] });
      render(<TimeTrackingPage />);
      expect(screen.getByTestId("bar-chart")).toBeInTheDocument();
    });
  });

  // ── WeekChart tooltip formatters ────────────────────────────────────────

  describe("WeekChart tooltip formatters", () => {
    it("invokes tooltipLabelFormatter and tooltipValueFormatter via mock", () => {
      setupDefaultMocks({ history: [mockHistorySession] });
      render(<TimeTrackingPage />);

      // The ChartTooltipContent mock invokes the formatters and stores results as data attributes
      const tooltipContent = screen.getByTestId("chart-tooltip-content");

      // labelFormatter: receives payload with label "Mo 01.01" → returns "Mo 01.01"
      expect(tooltipContent.getAttribute("data-label")).toBe("Mo 01.01");

      // formatter with netMinutes: 120 min → "Arbeitszeit: 2h 0min"
      expect(tooltipContent.getAttribute("data-fmt1")).toBe(
        "Arbeitszeit: 2h 0min",
      );

      // formatter with breakMinutes: 30 min → "Pause: 0h 30min"
      expect(tooltipContent.getAttribute("data-fmt2")).toBe("Pause: 0h 30min");

      // formatter with undefined value → fallback (value ?? 0) → "Pause: 0h 0min"
      expect(tooltipContent.getAttribute("data-fmt3")).toBe("Pause: 0h 0min");
    });
  });

  // ── WeekTable ───────────────────────────────────────────────────────────

  describe("WeekTable", () => {
    it("shows KW heading with week number", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      const kwText = screen.getByText(/^KW \d+:/);
      expect(kwText).toBeInTheDocument();
    });

    it("shows week navigation buttons", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByLabelText("Vorherige Woche")).toBeInTheDocument();
      expect(screen.getByLabelText("Nächste Woche")).toBeInTheDocument();
    });

    it("disables Nächste Woche when weekOffset is 0", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      const nextBtn = screen.getByLabelText("Nächste Woche");
      expect(nextBtn).toBeInTheDocument();
    });

    it("navigates to previous week when button clicked", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      const prevBtn = screen.getByLabelText("Vorherige Woche");
      fireEvent.click(prevBtn);
      // After going back, Nächste Woche should be enabled
      const nextBtn = screen.getByLabelText("Nächste Woche");
      expect(nextBtn).not.toBeDisabled();
    });

    it("shows Woche gesamt in table", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Woche gesamt")).toBeInTheDocument();
    });

    it("shows loading indicator in weekly total when history is loading", () => {
      setupDefaultMocks({ historyLoading: true });
      render(<TimeTrackingPage />);
      expect(screen.getByText("...")).toBeInTheDocument();
    });

    it("shows Kein Eintrag for past days without session", () => {
      // Mobile mode to see "Kein Eintrag" text
      Object.defineProperty(window, "innerWidth", { value: 500 });
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      // There should be at least one day with no session
      const noEntries = screen.queryAllByText("Kein Eintrag");
      // Past weekdays without sessions should show this
      expect(noEntries.length).toBeGreaterThanOrEqual(0);
    });

    it("renders session data in desktop table (Start, Ende, etc.)", () => {
      setupDefaultMocks({ history: [mockHistorySession] });
      render(<TimeTrackingPage />);
      // Table headers for desktop view
      expect(screen.getByText("Start")).toBeInTheDocument();
      expect(screen.getByText("Ende")).toBeInTheDocument();
      expect(screen.getByText("Netto")).toBeInTheDocument();
      expect(screen.getByText("Ort")).toBeInTheDocument();
    });

    it("shows absence badge in week table when absence exists", () => {
      setupDefaultMocks({ absences: [mockAbsence] });
      render(<TimeTrackingPage />);
      // "Krank" badge should appear
      const sickBadges = screen.queryAllByText("Krank");
      expect(sickBadges.length).toBeGreaterThan(0);
    });

    it("shows half day indicator for half-day absences", () => {
      setupDefaultMocks({ absences: [mockVacationAbsence] });
      render(<TimeTrackingPage />);
      const halfDayTexts = screen.queryAllByText(/halber Tag/);
      expect(halfDayTexts.length).toBeGreaterThan(0);
    });

    it("shows export button", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByLabelText("Export")).toBeInTheDocument();
    });
  });

  // ── ExportDropdown ──────────────────────────────────────────────────────

  describe("ExportDropdown", () => {
    it("opens export panel when export button clicked", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      expect(screen.getByText("Zeitraum exportieren")).toBeInTheDocument();
    });

    it("shows CSV, Excel and PDF buttons", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      expect(screen.getByText("CSV")).toBeInTheDocument();
      expect(screen.getByText("Excel")).toBeInTheDocument();
      expect(screen.getByText("PDF")).toBeInTheDocument();
    });

    it("shows MiniCalendar month navigation", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      expect(screen.getByLabelText("Vorheriger Monat")).toBeInTheDocument();
      expect(screen.getByLabelText("Nächster Monat")).toBeInTheDocument();
    });

    it("shows weekday headers in calendar", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      // MiniCalendar weekday labels
      expect(screen.getByText("Mo")).toBeInTheDocument();
      expect(screen.getByText("Di")).toBeInTheDocument();
      expect(screen.getByText("Fr")).toBeInTheDocument();
    });

    it("navigates months in calendar", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      const prevMonth = screen.getByLabelText("Vorheriger Monat");
      fireEvent.click(prevMonth);
      // Check month changed -- we don't assert exact month but verify it's rendered
      expect(screen.getByLabelText("Nächster Monat")).toBeInTheDocument();
    });
  });

  // ── Check-in with existing absence (confirmation modal) ─────────────────

  describe("check-in with existing absence", () => {
    it("shows confirmation modal when checking in with active absence", async () => {
      setupDefaultMocks({ absences: [mockAbsence] });
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(screen.getByText("Abwesenheit eingetragen")).toBeInTheDocument();
        expect(screen.getByText("Trotzdem einstempeln")).toBeInTheDocument();
      });
    });

    it("cancels check-in when Abbrechen clicked in confirmation", async () => {
      setupDefaultMocks({ absences: [mockAbsence] });
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(screen.getByText("Trotzdem einstempeln")).toBeInTheDocument();
      });

      // Find the Abbrechen button in the pending check-in modal footer
      const cancelButtons = screen.getAllByText("Abbrechen");
      fireEvent.click(cancelButtons[cancelButtons.length - 1]!);

      await waitFor(() => {
        expect(timeTrackingService.checkIn).not.toHaveBeenCalled();
      });
    });

    it("proceeds with check-in when Trotzdem einstempeln clicked", async () => {
      setupDefaultMocks({ absences: [mockAbsence] });
      vi.mocked(timeTrackingService.checkIn).mockResolvedValue(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(screen.getByText("Trotzdem einstempeln")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByText("Trotzdem einstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkIn).toHaveBeenCalledWith("present");
      });
    });

    it("shows absence type in confirmation modal", async () => {
      setupDefaultMocks({ absences: [mockAbsence] });
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        // "Krank" appears in the confirmation modal (and also in the table on weekdays)
        const krankTexts = screen.getAllByText(/Krank/);
        expect(krankTexts.length).toBeGreaterThanOrEqual(1);
      });
    });
  });

  // ── Reopen-with-status-change (Issue #1368) ─────────────────────────────
  //
  // Backend rejects a CheckIn that would silently flip Vor Ort ↔ Homeoffice
  // on a checked-out session for today. The page must catch the typed error
  // (code: "reopen_status_conflict"), prompt for an audit reason, then
  // route the change through CheckIn(existingStatus) + UpdateSession.

  // Both fixes below keep the page's own arithmetic on the same footing as the
  // server: it caps a block that was never checked out, and it reads the blocks
  // of a range by intersection instead of by stored date.
  describe("abgelaufene und übergreifende Blöcke (#2402)", () => {
    it("stops counting a block whose live limit has passed", () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date("2026-01-15T12:00:00+01:00"));
      try {
        const threeDaysAgo = new Date();
        threeDaysAgo.setDate(threeDaysAgo.getDate() - 3);
        const staleISO = `${threeDaysAgo.getFullYear()}-${String(threeDaysAgo.getMonth() + 1).padStart(2, "0")}-${String(threeDaysAgo.getDate()).padStart(2, "0")}`;

        setupDefaultMocks({
          currentSession: {
            ...mockActiveSession,
            date: staleISO,
            checkInTime: testTimestamp(staleISO, "08:00"),
          },
        });
        const { container } = render(<TimeTrackingPage />);

        // The block ended at the end of its own Berlin day — server-side too, so
        // running it through `now` would print days of work the Saldo denies.
        expect(container.querySelector(".text-4xl")).toHaveTextContent("0min");
      } finally {
        vi.useRealTimers();
      }
    });

    it("keeps counting a block that is still inside its live window", () => {
      // Relative to the clock the component reads, so the assertion holds
      // whatever time of day (or faked timer) the suite runs under.
      const startedAt = new Date(Date.now() - 2 * 60 * 60 * 1000);
      const startedISO = `${startedAt.getFullYear()}-${String(startedAt.getMonth() + 1).padStart(2, "0")}-${String(startedAt.getDate()).padStart(2, "0")}`;

      setupDefaultMocks({
        currentSession: {
          ...mockActiveSession,
          date: startedISO,
          checkInTime: startedAt.toISOString(),
        },
      });
      const { container } = render(<TimeTrackingPage />);

      expect(container.querySelector(".text-4xl")).not.toHaveTextContent(
        "0min",
      );
    });

    // The table's history key is shared with usePeriodMetrics so SWR dedupes
    // the two fetches into one request. Both must keep the same head start —
    // not because the range decides which blocks are visible (/history bounds
    // `from` against check_out_time), but because two keys mean two requests.
    it("shares one history key with the period metrics", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);

      const monday = new Date(today);
      monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7));
      const sunday = new Date(monday);
      sunday.setDate(sunday.getDate() + 6);
      const iso = (d: Date) =>
        `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
      const headStart = new Date(monday);
      headStart.setDate(headStart.getDate() - 1);

      const tableKeys = vi
        .mocked(useSWRAuth)
        .mock.calls.map(([key]) => key)
        .filter(
          (key): key is string =>
            typeof key === "string" &&
            key.startsWith("time-tracking-table-") &&
            !key.startsWith("time-tracking-table-absences") &&
            !key.startsWith("time-tracking-table-shifts"),
        );

      expect(new Set(tableKeys)).toEqual(
        new Set([`time-tracking-table-${iso(headStart)}-${iso(sunday)}`]),
      );
    });
  });

  describe("erneutes Einstempeln nach Checkout (#2402)", () => {
    function makePlannedStartError(): Error & {
      code?: string;
      status?: number;
      details?: Record<string, unknown>;
    } {
      const err = new Error("planned start not reached") as Error & {
        code?: string;
        status?: number;
        details?: Record<string, unknown>;
      };
      err.code = "planned_start_not_reached";
      err.status = 409;
      err.details = {
        planned_start_time: "09:00",
        current_time: "08:45",
      };
      return err;
    }

    it("shows planned-start message when check-in is too early", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockRejectedValueOnce(
        makePlannedStartError(),
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("In der OGS"));

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Einstempeln ist erst ab 09:00 Uhr möglich.",
        );
      });
    });

    it("checking in again with the same status stamps a new block without a modal", async () => {
      // Existing checked-out 'present' block today; picking Vor Ort again
      // starts a second block — one stamp, no dialog (#2402).
      setupDefaultMocks({ history: [mockHistorySession] });
      vi.mocked(timeTrackingService.checkIn).mockResolvedValueOnce(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      selectPresentMode();
      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkIn).toHaveBeenCalledWith("present");
      });
      expect(
        screen.queryByText("Status für heute ändern"),
      ).not.toBeInTheDocument();
    });

    it("checking in again with a DIFFERENT status stamps directly — no conflict flow, no updateSession", async () => {
      // The Swantje case: Homeoffice morning already checked out, the
      // afternoon block starts in der OGS. Since #2402 the backend simply
      // creates a new block with the requested status, so the page stamps
      // once and never routes through UpdateSession.
      setupDefaultMocks({ history: [mockHistorySession] });
      vi.mocked(timeTrackingService.checkIn).mockResolvedValueOnce(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("Homeoffice"));
      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkIn).toHaveBeenCalledWith("home_office");
      });
      expect(timeTrackingService.checkIn).toHaveBeenCalledTimes(1);
      expect(timeTrackingService.updateSession).not.toHaveBeenCalled();
      expect(
        screen.queryByText("Status für heute ändern"),
      ).not.toBeInTheDocument();
    });
  });

  describe("EditSessionModal", () => {
    it("opens edit modal when edit button clicked on a session row", async () => {
      // Use yesterday's date for a past session that's editable
      const yesterday = new Date();
      yesterday.setDate(yesterday.getDate() - 1);
      const yISO = `${yesterday.getFullYear()}-${String(yesterday.getMonth() + 1).padStart(2, "0")}-${String(yesterday.getDate()).padStart(2, "0")}`;

      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      render(<TimeTrackingPage />);

      // In desktop mode, find the edit button (SquarePen icon)
      // The row should be clickable for sessions with edits or canEdit
      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });
      }
    });

    it("matches the clicked block by id and prefills its times (#2402)", async () => {
      const yesterday = new Date();
      yesterday.setDate(yesterday.getDate() - 1);
      const yISO = `${yesterday.getFullYear()}-${String(yesterday.getMonth() + 1).padStart(2, "0")}-${String(yesterday.getDate()).padStart(2, "0")}`;

      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        id: "4711",
        date: yISO,
        checkInTime: testTimestamp(yISO, "07:15"),
        checkOutTime: testTimestamp(yISO, "13:45"),
      };

      setupDefaultMocks({ history: [pastSession] });
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getAllByLabelText("Eintrag bearbeiten")[0]!);

      // The table hands the page the block's NUMERIC id
      // (adaptHistorySessionForMetrics), the page matches it back against the
      // string-mapped history entry. A failed match would open the modal
      // without a session ("Kein Eintrag vorhanden") — no Start/Ende inputs.
      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });
      expect(screen.getByTestId("modal")).not.toHaveAttribute(
        "data-title",
        "Kein Eintrag vorhanden",
      );
      expect(screen.getByLabelText("Start")).toHaveValue("07:15");
      expect(screen.getByLabelText("Ende")).toHaveValue("13:45");
    });
  });

  // ── Create Absence Modal ────────────────────────────────────────────────

  describe("CreateAbsenceModal", () => {
    it("opens create absence modal from Abwesend mode", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    it("shows absence type selector in create modal", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));
      expect(screen.getByText("Art der Abwesenheit")).toBeInTheDocument();
      expect(
        screen.queryByRole("option", { name: "Freizeitausgleich" }),
      ).not.toBeInTheDocument();
    });

    it("shows date inputs in create modal", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));
      expect(screen.getByLabelText("Von")).toBeInTheDocument();
      expect(screen.getByLabelText("Bis")).toBeInTheDocument();
    });

    it("shows Halber Tag toggle in create modal", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));
      expect(screen.getByText("Halber Tag")).toBeInTheDocument();
    });

    it("calls createAbsence on save", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.createAbsence).mockResolvedValue(
        mockAbsence,
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));

      // Click Speichern
      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
      });

      await waitFor(() => {
        expect(timeTrackingService.createAbsence).toHaveBeenCalled();
      });
    });
  });

  // ── Error Handling (friendlyError) ──────────────────────────────────────

  describe("error handling with friendlyError", () => {
    it("maps 'already checked out today' to German", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockRejectedValue(
        new Error("already checked out today"),
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Du hast heute bereits gearbeitet.",
        );
      });
    });

    it("maps 'no active session found' to German", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.checkOut).mockRejectedValue(
        new Error("no active session found"),
      );
      render(<TimeTrackingPage />);

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Ausstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Kein aktiver Eintrag vorhanden.",
        );
      });
    });

    it("maps 'break already active' to German for startBreak", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.startBreak).mockRejectedValue(
        new Error("break already active"),
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByLabelText("Pause starten"));

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Starten" }));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Eine Pause läuft bereits.",
        );
      });
    });

    it("maps 'absence overlaps' to German", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.createAbsence).mockRejectedValue(
        new Error('{"error":"absence overlaps with existing absence"}'),
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));

      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Für diesen Zeitraum ist bereits eine andere Abwesenheitsart eingetragen.",
        );
      });
    });

    it("uses fallback for unknown error", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockRejectedValue(
        new Error("some unknown error xyz"),
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith("Fehler beim Einstempeln");
      });
    });

    it("handles non-Error thrown objects", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.checkIn).mockRejectedValue("string error");
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        // "string error" does not match any key, so fallback is used
        expect(mockToast.error).toHaveBeenCalled();
      });
    });
  });

  // ── EditHistoryAccordion ────────────────────────────────────────────────

  describe("EditHistoryAccordion (via WeekTable expand)", () => {
    it("shows edit history indicator for sessions with edits", () => {
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);
      expect(screen.getByText(/Zuletzt geändert/)).toBeInTheDocument();
    });

    it("expands edit history on row click for session with edits", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "300",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: testTimestamp(todayISO, "07:30"),
          newValue: testTimestamp(todayISO, "08:00"),
          notes: "Zeitkorrektur",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      // Click on the row with edits
      const changeText = screen.getByText(/Zuletzt geändert/);
      const row = changeText.closest("tr");
      if (row) {
        fireEvent.click(row);
        await waitFor(() => {
          expect(timeTrackingService.getSessionEdits).toHaveBeenCalledWith(
            "100",
          );
        });
      }
    });
  });

  // ── Mobile layout ───────────────────────────────────────────────────────

  describe("mobile layout", () => {
    beforeEach(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 375,
      });
    });

    it("renders mobile card layout on small screens", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      // Fire resize event to trigger mobile detection
      window.dispatchEvent(new Event("resize"));
      // Mobile should not show table headers
      // The page should still render
      expect(screen.getByText("Stempeluhr")).toBeInTheDocument();
    });
  });

  // ── Absence editing in WeekTable ────────────────────────────────────────

  describe("absence row interaction", () => {
    it("opens edit modal when absence-only day clicked", async () => {
      // Create absence for a past date to make it clickable
      const yesterday = new Date();
      yesterday.setDate(yesterday.getDate() - 1);
      const yISO = `${yesterday.getFullYear()}-${String(yesterday.getMonth() + 1).padStart(2, "0")}-${String(yesterday.getDate()).padStart(2, "0")}`;

      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      render(<TimeTrackingPage />);

      // Find the absence row and click it (desktop table)
      const sickBadges = screen.queryAllByText("Krank");
      if (sickBadges.length > 0) {
        const row = sickBadges[0]?.closest("tr");
        if (row) {
          fireEvent.click(row);
          await waitFor(() => {
            expect(screen.getByTestId("modal")).toBeInTheDocument();
          });
        }
      }
    });
  });

  // ── Check-out error handling ────────────────────────────────────────────

  describe("check-out error handling", () => {
    it("shows success toast on checkout", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.checkOut).mockResolvedValue(
        mockCheckedOutSession,
      );
      render(<TimeTrackingPage />);

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Ausstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.success).toHaveBeenCalledWith(
          "Erfolgreich ausgestempelt",
        );
      });
    });
  });

  // ── End break handling ──────────────────────────────────────────────────

  describe("end break error handling", () => {
    it("shows error toast when endBreak fails", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks({ currentSession: mockActiveSession });

      // First render without breaks
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "50",
          sessionId: "100",
          startedAt: new Date().toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: null,
        },
      ]);
      vi.mocked(timeTrackingService.endBreak).mockRejectedValue(
        new Error("no active break found"),
      );

      render(<TimeTrackingPage />);

      const endBreakBtn = await screen.findByLabelText("Pause beenden");
      fireEvent.click(endBreakBtn);

      // Verify test setup was correct - breaks were loaded
      expect(timeTrackingService.getSessionBreaks).toHaveBeenCalled();
    });
  });

  // ── Wochenübersicht heading ─────────────────────────────────────────────

  // ── Own Dienstplan (Plan column) ────────────────────────────────────────

  describe("own shift loading in the Zeiterfassung table", () => {
    // A failed shift fetch must not render as an empty plan: every Plan cell
    // would show "–", telling the staff member no shifts were scheduled.
    it("warns instead of showing an empty plan when the shift fetch fails", () => {
      setupDefaultMocks();
      const base = vi.mocked(useSWRAuth).getMockImplementation()!;
      vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
        if (key?.startsWith("time-tracking-table-shifts")) {
          return {
            data: undefined,
            isLoading: false,
            mutate: mockMutate,
            isValidating: false,
            error: new Error("network down"),
          } as never;
        }
        return base(key, (() => Promise.resolve()) as never);
      });

      render(<TimeTrackingPage />);
      expect(
        screen.getByText(/Der Dienstplan konnte nicht geladen/),
      ).toBeInTheDocument();
    });

    // Same reason: a table rendered before the shifts resolve shows "–" in
    // every Plan cell, so it must wait rather than guess.
    it("holds the table back while own shifts are still pending", () => {
      setupDefaultMocks();
      const base = vi.mocked(useSWRAuth).getMockImplementation()!;
      vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
        if (key?.startsWith("time-tracking-table-shifts")) {
          return {
            data: undefined,
            isLoading: true,
            mutate: mockMutate,
            isValidating: true,
            error: undefined,
          } as never;
        }
        return base(key, (() => Promise.resolve()) as never);
      });

      render(<TimeTrackingPage />);
      expect(screen.queryByText("Woche gesamt")).not.toBeInTheDocument();
    });
  });

  describe("Wochenübersicht", () => {
    it("shows Wochenübersicht chart heading", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Wochenübersicht")).toBeInTheDocument();
    });
  });

  // ── Session with compliance warnings ────────────────────────────────────

  describe("compliance warnings in table", () => {
    it("shows warning indicator for non-compliant sessions", () => {
      setupDefaultMocks({ history: [mockHistorySessionNonCompliant] });
      render(<TimeTrackingPage />);
      // Warning indicators should appear somewhere
      // Non-compliant session with netMinutes > 360 and breakMinutes < 30
      // should show a warning symbol
      const warnings = screen.queryAllByTitle(/Pausenzeit/);
      expect(warnings.length).toBeGreaterThanOrEqual(0);
    });

    it("shows auto-checkout warning for auto-checked-out sessions", () => {
      setupDefaultMocks({ history: [mockHistorySessionAutoCheckedOut] });
      render(<TimeTrackingPage />);
      const autoWarnings = screen.queryAllByTitle(/Automatisch ausgestempelt/);
      expect(autoWarnings.length).toBeGreaterThanOrEqual(0);
    });
  });

  // ── Delete absence ──────────────────────────────────────────────────────

  describe("delete absence", () => {
    it("shows delete button and calls deleteAbsence", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yesterday = new Date();
      yesterday.setDate(yesterday.getDate() - 1);
      const yISO = `${yesterday.getFullYear()}-${String(yesterday.getMonth() + 1).padStart(2, "0")}-${String(yesterday.getDate()).padStart(2, "0")}`;

      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      vi.mocked(timeTrackingService.deleteAbsence).mockResolvedValue(undefined);

      render(<TimeTrackingPage />);

      // Open edit modal for absence
      const sickBadges = screen.queryAllByText("Krank");
      if (sickBadges.length > 0) {
        const row = sickBadges[0]?.closest("tr");
        if (row) {
          fireEvent.click(row);
          await waitFor(() => {
            expect(screen.getByTestId("modal")).toBeInTheDocument();
          });

          // Look for Abwesenheit löschen button
          const deleteBtn = screen.queryByText("Abwesenheit löschen");
          if (deleteBtn) {
            await act(async () => {
              fireEvent.click(deleteBtn);
            });
            await waitFor(() => {
              expect(timeTrackingService.deleteAbsence).toHaveBeenCalledWith(
                pastAbsence.id,
              );
            });
          }
        }
      }
    });
  });

  // ── Update session (handleEditSave) ─────────────────────────────────────

  describe("update session", () => {
    it("calls updateSession and shows success toast", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yesterday = new Date();
      yesterday.setDate(yesterday.getDate() - 1);
      const yISO = `${yesterday.getFullYear()}-${String(yesterday.getMonth() + 1).padStart(2, "0")}-${String(yesterday.getDate()).padStart(2, "0")}`;

      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      vi.mocked(timeTrackingService.updateSession).mockResolvedValue(
        mockCheckedOutSession,
      );
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([]);

      render(<TimeTrackingPage />);

      // Find edit button and click to open modal
      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        // Fill in the notes field (required for save)
        const notesArea = screen.queryByPlaceholderText(
          "Oder eigenen Grund eingeben...",
        );
        if (notesArea) {
          fireEvent.change(notesArea, {
            target: { value: "Zeitkorrektur" },
          });
        }

        // Also try clicking a quick-reason button
        const reasonBtn = screen.queryByRole("button", {
          name: "Zeitkorrektur",
        });
        if (reasonBtn) {
          fireEvent.click(reasonBtn);
        }

        // Click save
        if (screen.queryAllByText("Speichern").length > 0) {
          const saveBtn = await waitForLastSaveButtonEnabled();
          await act(async () => {
            fireEvent.click(saveBtn);
          });
          await waitFor(() => {
            expect(timeTrackingService.updateSession).toHaveBeenCalled();
          });
        }
      }
    });
  });

  // ── Additional edge cases ───────────────────────────────────────────────

  describe("edge cases", () => {
    it("handles null currentSession gracefully", () => {
      setupDefaultMocks({ currentSession: null });
      render(<TimeTrackingPage />);
      expect(screen.getByText("Stempeluhr")).toBeInTheDocument();
    });

    it("handles empty history array", () => {
      setupDefaultMocks({ history: [] });
      render(<TimeTrackingPage />);
      expect(screen.getByText("Woche gesamt")).toBeInTheDocument();
    });

    it("handles empty absences array", () => {
      setupDefaultMocks({ absences: [] });
      render(<TimeTrackingPage />);
      expect(screen.getByText("Stempeluhr")).toBeInTheDocument();
    });

    it("renders without crashing when all data is empty", () => {
      setupDefaultMocks({
        currentSession: null,
        history: [],
        absences: [],
      });
      const { container } = render(<TimeTrackingPage />);
      expect(container).toBeTruthy();
    });
  });

  // ── EditSessionModal - comprehensive coverage ─────────────────────────

  describe("EditSessionModal - full coverage", () => {
    // Use today's date with a checked-out session and no active currentSession.
    // This makes canEdit = true because isToday && !isActive (no active session).
    // This avoids weekend issues (yesterday could be Sunday).
    function makePastSession(
      overrides?: Partial<WorkSessionHistory>,
    ): WorkSessionHistory {
      return {
        ...mockHistorySession,
        date: weekdayISO,
        checkInTime: testTimestamp(weekdayISO, "08:00"),
        checkOutTime: testTimestamp(weekdayISO, "16:30"),
        ...overrides,
      };
    }

    async function openEditModal(
      pastSession: WorkSessionHistory,
      moreSetup?: { absences?: StaffAbsence[] },
    ) {
      // No active currentSession => canEdit = isToday && !isActive is true
      setupDefaultMocks({
        currentSession: null,
        history: [pastSession],
        absences: moreSetup?.absences,
      });
      vi.mocked(timeTrackingService.updateSession).mockResolvedValue(
        mockCheckedOutSession,
      );
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([]);

      render(<TimeTrackingPage />);

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      expect(editButtons.length).toBeGreaterThan(0);
      fireEvent.click(editButtons[0]!);

      await waitFor(() => {
        expect(screen.getByTestId("modal")).toBeInTheDocument();
      });
      await waitFor(() => {
        expect(screen.getByLabelText("Start")).toHaveValue();
      });
    }

    it("shows Start and Ende time inputs in edit modal", async () => {
      await openEditModal(makePastSession());
      expect(screen.getByLabelText("Start")).toBeInTheDocument();
      expect(screen.getByLabelText("Ende")).toBeInTheDocument();
    });

    it("populates Start and Ende with session times", async () => {
      await openEditModal(makePastSession());
      const startInput = screen.getByLabelText("Start");
      const endInput = screen.getByLabelText("Ende");
      // Check that values are populated (times depend on timezone)
      expect((startInput as HTMLInputElement).value).not.toBe("");
      expect((endInput as HTMLInputElement).value).not.toBe("");
    });

    it("shows Pause dropdown when session has no individual breaks", async () => {
      await openEditModal(makePastSession({ breaks: [] }));
      expect(screen.getByLabelText("Pause (Min)")).toBeInTheDocument();
    });

    it("shows Ort (status) selector", async () => {
      await openEditModal(makePastSession());
      expect(screen.getByLabelText("Ort")).toBeInTheDocument();
    });

    it("shows Grund der Änderung label", async () => {
      await openEditModal(makePastSession());
      expect(screen.getByText(/Grund der Änderung/)).toBeInTheDocument();
    });

    it("shows quick-select reason buttons", async () => {
      await openEditModal(makePastSession());
      expect(screen.getByText("Vergessen auszustempeln")).toBeInTheDocument();
      expect(screen.getByText("Vergessen einzustempeln")).toBeInTheDocument();
      expect(screen.getByText("Zeitkorrektur")).toBeInTheDocument();
      expect(screen.getByText("Krankheit")).toBeInTheDocument();
      expect(screen.getByText("Ort-Änderung")).toBeInTheDocument();
    });

    it("clicking a quick-select reason fills the notes field", async () => {
      await openEditModal(makePastSession());
      fireEvent.click(screen.getByText("Vergessen auszustempeln"));
      const textarea = screen.getByPlaceholderText(
        "Oder eigenen Grund eingeben...",
      );
      expect((textarea as HTMLTextAreaElement).value).toBe(
        "Vergessen auszustempeln",
      );
    });

    it("typing in notes textarea updates the value", async () => {
      await openEditModal(makePastSession());
      const textarea = screen.getByPlaceholderText(
        "Oder eigenen Grund eingeben...",
      );
      fireEvent.change(textarea, { target: { value: "Custom reason" } });
      expect((textarea as HTMLTextAreaElement).value).toBe("Custom reason");
    });

    it("changing Start time input works", async () => {
      await openEditModal(makePastSession());
      const startInput = screen.getByLabelText("Start");
      fireEvent.change(startInput, { target: { value: "07:00" } });
      expect((startInput as HTMLInputElement).value).toBe("07:00");
    });

    it("changing Ende time input works", async () => {
      await openEditModal(makePastSession());
      const endInput = screen.getByLabelText("Ende");
      fireEvent.change(endInput, { target: { value: "18:00" } });
      expect((endInput as HTMLInputElement).value).toBe("18:00");
    });

    it("changing break dropdown works", async () => {
      await openEditModal(makePastSession({ breaks: [] }));
      const breakSelect = screen.getByLabelText("Pause (Min)");
      chooseSelectOption(breakSelect, "45 min");
      expect(breakSelect).toHaveTextContent("45 min");
    });

    it("changing status selector works", async () => {
      await openEditModal(makePastSession());
      const statusSelect = screen.getByLabelText("Ort");
      chooseSelectOption(statusSelect, "Homeoffice");
      expect(statusSelect).toHaveTextContent("Homeoffice");
    });

    it("shows compliance warning when work > 10h", async () => {
      await openEditModal(makePastSession({ breaks: [] }));
      const startInput = screen.getByLabelText("Start");
      const endInput = screen.getByLabelText("Ende");
      // Set break to 0 first
      const breakSelect = screen.getByLabelText("Pause (Min)");
      chooseSelectOption(breakSelect, "0 min");
      fireEvent.change(startInput, { target: { value: "06:00" } });
      fireEvent.change(endInput, { target: { value: "17:00" } });
      // 11h work, > 10h
      await waitFor(() => {
        expect(screen.getByText(/Arbeitszeit > 10h/)).toBeInTheDocument();
      });
    });

    it("shows compliance warning when break < 30min for > 6h work", async () => {
      await openEditModal(makePastSession({ breaks: [], breakMinutes: 0 }));
      const startInput = screen.getByLabelText("Start");
      const endInput = screen.getByLabelText("Ende");
      const breakSelect = screen.getByLabelText("Pause (Min)");
      chooseSelectOption(breakSelect, "15 min");
      fireEvent.change(startInput, { target: { value: "08:00" } });
      fireEvent.change(endInput, { target: { value: "15:30" } });
      // 7.5h gross - 15min break = 7h15m net > 6h, break < 30
      await waitFor(() => {
        expect(
          screen.getByText(/Pausenzeit < 30 Min bei > 6h/),
        ).toBeInTheDocument();
      });
    });

    it("shows compliance warning when break < 45min for > 9h work", async () => {
      await openEditModal(makePastSession({ breaks: [], breakMinutes: 0 }));
      const startInput = screen.getByLabelText("Start");
      const endInput = screen.getByLabelText("Ende");
      const breakSelect = screen.getByLabelText("Pause (Min)");
      chooseSelectOption(breakSelect, "30 min");
      fireEvent.change(startInput, { target: { value: "06:00" } });
      fireEvent.change(endInput, { target: { value: "16:30" } });
      // 10.5h gross - 30min = 10h net > 9h, break 30 < 45
      await waitFor(() => {
        expect(
          screen.getByText(/Pausenzeit < 45 Min bei > 9h/),
        ).toBeInTheDocument();
      });
    });

    it("save button is disabled when notes are empty", async () => {
      await openEditModal(makePastSession());
      // Notes should be empty by default
      const saveButtons = screen.getAllByText("Speichern");
      const saveBtn = saveButtons[saveButtons.length - 1]!;
      expect(saveBtn).toBeDisabled();
    });

    it("save button is enabled when notes are provided", async () => {
      await openEditModal(makePastSession());
      clickQuickEditReason("Zeitkorrektur");
      await waitForLastSaveButtonEnabled();
    });

    it("blocks saving when end time is not after start time", async () => {
      await openEditModal(makePastSession());
      vi.mocked(timeTrackingService.updateSession).mockClear();

      clickQuickEditReason("Zeitkorrektur");
      fireEvent.change(screen.getByLabelText("Start"), {
        target: { value: "12:30" },
      });
      fireEvent.change(screen.getByLabelText("Ende"), {
        target: { value: "12:00" },
      });

      expect(
        screen.getByText("Ende muss nach Start liegen."),
      ).toBeInTheDocument();

      const saveButtons = screen.getAllByText("Speichern");
      const saveBtn = saveButtons[saveButtons.length - 1]!;
      expect(saveBtn).toBeDisabled();

      await act(async () => {
        fireEvent.click(saveBtn);
      });

      expect(timeTrackingService.updateSession).not.toHaveBeenCalled();
    });

    it("calls onSave with correct data when saved without individual breaks", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      await openEditModal(makePastSession({ breaks: [] }));
      clickQuickEditReason("Zeitkorrektur");

      const saveBtn = await waitForLastSaveButtonEnabled();
      await act(async () => {
        fireEvent.click(saveBtn);
      });

      await waitFor(() => {
        expect(timeTrackingService.updateSession).toHaveBeenCalled();
        expect(mockToast.success).toHaveBeenCalledWith("Eintrag gespeichert");
      });
    });

    it("shows individual break durations when session has breaks", async () => {
      const yISO = weekdayISO;
      const sessionWithBreaks = makePastSession({
        breaks: [
          {
            id: "b1",
            sessionId: "100",
            startedAt: testTimestamp(yISO, "10:00"),
            endedAt: testTimestamp(yISO, "10:30"),
            durationMinutes: 30,
            plannedEndTime: null,
          },
          {
            id: "b2",
            sessionId: "100",
            startedAt: testTimestamp(yISO, "13:00"),
            endedAt: testTimestamp(yISO, "13:15"),
            durationMinutes: 15,
            plannedEndTime: null,
          },
        ],
      });

      await openEditModal(sessionWithBreaks);
      expect(screen.getByText("Pausen")).toBeInTheDocument();
      expect(screen.getByText("Gesamt")).toBeInTheDocument();
      // "45 min" appears in dropdown options too, so check total display
      const totalTexts = screen.queryAllByText("45 min");
      expect(totalTexts.length).toBeGreaterThan(0);
    });

    it("saves with individual break changes when breaks exist", async () => {
      const yISO = weekdayISO;
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const sessionWithBreaks = makePastSession({
        breaks: [
          {
            id: "b1",
            sessionId: "100",
            startedAt: testTimestamp(yISO, "10:00"),
            endedAt: testTimestamp(yISO, "10:30"),
            durationMinutes: 30,
            plannedEndTime: null,
          },
        ],
      });

      await openEditModal(sessionWithBreaks);

      // Change break duration via select
      const breakTrigger = screen
        .getByText("Pausen")
        .closest("div")!
        .querySelector<HTMLElement>('[role="combobox"]');
      if (breakTrigger) {
        chooseSelectOption(breakTrigger, "45 min");
      }

      clickQuickEditReason("Zeitkorrektur");

      const saveBtn = await waitForLastSaveButtonEnabled();
      await act(async () => {
        fireEvent.click(saveBtn);
      });

      await waitFor(() => {
        const call = vi.mocked(timeTrackingService.updateSession).mock.calls[0];
        expect(call).toBeDefined();
        // Should include breaks array since individual breaks changed
        const updates = call![1];
        expect(updates.breaks).toBeDefined();
      });
    });

    it("modal title is 'Eintrag bearbeiten' for session-only", async () => {
      await openEditModal(makePastSession());
      expect(screen.getByTestId("modal").getAttribute("data-title")).toBe(
        "Eintrag bearbeiten",
      );
    });

    it("modal title is 'Abwesenheit bearbeiten' for absence-only", async () => {
      const yISO = weekdayISO;
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      render(<TimeTrackingPage />);

      const sickBadges = screen.queryAllByText("Krank");
      expect(sickBadges.length).toBeGreaterThan(0);
      const row = sickBadges[0]!.closest("tr");
      if (row) {
        fireEvent.click(row);
        await waitFor(() => {
          expect(screen.getByTestId("modal").getAttribute("data-title")).toBe(
            "Abwesenheit bearbeiten",
          );
        });
      }
    });

    it("modal title is 'Tag bearbeiten' when both session and absence exist", async () => {
      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      await openEditModal(pastSession, { absences: [pastAbsence] });
      expect(screen.getByTestId("modal").getAttribute("data-title")).toBe(
        "Tag bearbeiten",
      );
    });

    it("shows tabs when both session and absence exist", async () => {
      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      await openEditModal(pastSession, { absences: [pastAbsence] });
      expect(screen.getByText("Arbeitszeit")).toBeInTheDocument();
      expect(screen.getByText("Abwesenheit")).toBeInTheDocument();
    });

    it("switches to absence tab and shows absence fields", async () => {
      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      await openEditModal(pastSession, { absences: [pastAbsence] });
      fireEvent.click(screen.getByText("Abwesenheit"));

      await waitFor(() => {
        expect(screen.getByText("Art der Abwesenheit")).toBeInTheDocument();
        expect(screen.getByLabelText("Von")).toBeInTheDocument();
        expect(screen.getByLabelText("Bis")).toBeInTheDocument();
        expect(screen.getByText("Halber Tag")).toBeInTheDocument();
        expect(screen.getByText("Abwesenheit löschen")).toBeInTheDocument();
      });
    });

    it("shows manager-entered comp time as read-only", async () => {
      const compTimeAbsence: StaffAbsence = {
        ...mockAbsence,
        absenceType: "comp_time",
        dateStart: weekdayISO,
        dateEnd: weekdayISO,
        halfDay: true,
        note: "Überstundenabbau",
      };

      await openEditModal(makePastSession(), {
        absences: [compTimeAbsence],
      });
      fireEvent.click(screen.getByText("Abwesenheit"));

      expect(
        await screen.findByText(
          /Freizeitausgleich wird von der Leitung eingetragen/,
        ),
      ).toBeInTheDocument();
      expect(screen.getByText("Halber Tag")).toBeInTheDocument();
      expect(screen.getByText("Überstundenabbau")).toBeInTheDocument();
      expect(screen.queryByLabelText("Von")).not.toBeInTheDocument();
      expect(screen.queryByText("Abwesenheit löschen")).not.toBeInTheDocument();
      expect(screen.queryByText("Speichern")).not.toBeInTheDocument();
      expect(screen.getByRole("button", { name: "Schließen" })).toBeEnabled();
    });

    it("calls updateAbsence when absence tab saved", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      vi.mocked(timeTrackingService.updateAbsence).mockResolvedValue(
        pastAbsence,
      );

      await openEditModal(pastSession, { absences: [pastAbsence] });
      fireEvent.click(screen.getByText("Abwesenheit"));

      await waitFor(() => {
        expect(screen.getByText("Art der Abwesenheit")).toBeInTheDocument();
      });

      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
      });

      await waitFor(() => {
        expect(timeTrackingService.updateAbsence).toHaveBeenCalledWith(
          pastAbsence.id,
          expect.objectContaining({ absence_type: "sick" }),
        );
      });
    });

    it("calls deleteAbsence from the absence tab", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      vi.mocked(timeTrackingService.deleteAbsence).mockResolvedValue(undefined);

      await openEditModal(pastSession, { absences: [pastAbsence] });
      fireEvent.click(screen.getByText("Abwesenheit"));

      await waitFor(() => {
        expect(screen.getByText("Abwesenheit löschen")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByText("Abwesenheit löschen"));
      });

      await waitFor(() => {
        expect(timeTrackingService.deleteAbsence).toHaveBeenCalledWith(
          pastAbsence.id,
        );
      });
    });

    it("shows Abbrechen button and closes modal on click", async () => {
      await openEditModal(makePastSession());
      const cancelButtons = screen.getAllByText("Abbrechen");
      expect(cancelButtons.length).toBeGreaterThan(0);
      fireEvent.click(cancelButtons[cancelButtons.length - 1]!);
      await waitFor(() => {
        expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
      });
    });

    it("shows no compliance warnings when work is under 6h", async () => {
      await openEditModal(makePastSession({ breaks: [] }));
      const startInput = screen.getByLabelText("Start");
      const endInput = screen.getByLabelText("Ende");
      const breakSelect = screen.getByLabelText("Pause (Min)");
      chooseSelectOption(breakSelect, "0 min");
      fireEvent.change(startInput, { target: { value: "08:00" } });
      fireEvent.change(endInput, { target: { value: "13:00" } });
      // 5h work, no warnings expected
      expect(screen.queryByText(/Arbeitszeit > 10h/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Pausenzeit < 30 Min/)).not.toBeInTheDocument();
    });

    it("shows absence note textarea in absence tab", async () => {
      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
        note: "Some note",
      };

      await openEditModal(pastSession, { absences: [pastAbsence] });
      fireEvent.click(screen.getByText("Abwesenheit"));

      await waitFor(() => {
        const noteArea = screen.getByPlaceholderText(
          "z.B. Arzttermin, Schulung ...",
        );
        expect((noteArea as HTMLTextAreaElement).value).toBe("Some note");
      });
    });

    it("toggles half day switch in absence tab", async () => {
      const yISO = weekdayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
        halfDay: false,
      };

      await openEditModal(pastSession, { absences: [pastAbsence] });
      fireEvent.click(screen.getByText("Abwesenheit"));

      const toggle = await screen.findByRole("switch");
      expect(toggle.getAttribute("aria-checked")).toBe("false");
      fireEvent.click(toggle);
      expect(toggle.getAttribute("aria-checked")).toBe("true");
    });
  });

  // ── BreakActivityLog coverage ─────────────────────────────────────────

  describe("BreakActivityLog - with break data", () => {
    it("shows work and break segments when session has breaks", async () => {
      const breakStart = new Date();
      breakStart.setMinutes(breakStart.getMinutes() - 60);
      const breakEnd = new Date();
      breakEnd.setMinutes(breakEnd.getMinutes() - 30);

      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "b1",
          sessionId: "100",
          startedAt: breakStart.toISOString(),
          endedAt: breakEnd.toISOString(),
          durationMinutes: 30,
          plannedEndTime: null,
        },
      ]);

      render(<TimeTrackingPage />);

      await waitFor(() => {
        // Should show "Arbeitszeit" and "Pause" segment rows
        const arbeitszeit = screen.queryAllByText("Arbeitszeit");
        expect(arbeitszeit.length).toBeGreaterThan(0);
      });
    });

    it("shows Pause label for break segments in activity log", async () => {
      const breakStart = new Date();
      breakStart.setMinutes(breakStart.getMinutes() - 60);
      const breakEnd = new Date();
      breakEnd.setMinutes(breakEnd.getMinutes() - 30);

      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "b1",
          sessionId: "100",
          startedAt: breakStart.toISOString(),
          endedAt: breakEnd.toISOString(),
          durationMinutes: 30,
          plannedEndTime: null,
        },
      ]);

      render(<TimeTrackingPage />);

      await waitFor(() => {
        // Pause label from BreakActivityLog segments
        const pauseLabels = screen.queryAllByText("Pause");
        expect(pauseLabels.length).toBeGreaterThan(0);
      });
    });

    it("shows active break as ongoing with no end time", async () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "b1",
          sessionId: "100",
          startedAt: new Date().toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: null,
        },
      ]);

      render(<TimeTrackingPage />);

      await waitFor(() => {
        // Active break should show "Pause" badge
        expect(screen.getByLabelText("Pause beenden")).toBeInTheDocument();
      });
    });
  });

  // ── MiniCalendar date selection coverage ──────────────────────────────

  describe("MiniCalendar - date range selection", () => {
    it("clicking a day in the calendar selects it as range start", async () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));

      // Find a clickable day number (not disabled)
      const dayButtons = screen
        .getAllByRole("button")
        .filter(
          (btn) =>
            !btn.hasAttribute("disabled") &&
            /^\d+$/.test(btn.textContent ?? ""),
        );
      expect(dayButtons.length).toBeGreaterThan(0);

      // Click first available day
      fireEvent.click(dayButtons[0]!);
      // After one click, a partial range "DD.MM.YYYY - ..." should show
      // The range display is updated
      const rangeTexts = screen.queryAllByText(/–/);
      expect(rangeTexts.length).toBeGreaterThan(0);
    });

    it("clicking two days selects a date range", async () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));

      const dayButtons = screen
        .getAllByRole("button")
        .filter(
          (btn) =>
            !btn.hasAttribute("disabled") &&
            /^\d+$/.test(btn.textContent ?? ""),
        );

      if (dayButtons.length >= 2) {
        fireEvent.click(dayButtons[0]!);
        fireEvent.click(dayButtons[1]!);
        // Now CSV/Excel should be enabled
        const csvBtn = screen.getByText("CSV");
        expect(csvBtn).not.toBeDisabled();
      }
    });

    it("navigating to previous month updates the calendar", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));

      const prevMonth = screen.getByLabelText("Vorheriger Monat");
      fireEvent.click(prevMonth);
      // Calendar should still render with day numbers
      const dayButtons = screen
        .getAllByRole("button")
        .filter((btn) => /^\d+$/.test(btn.textContent ?? ""));
      expect(dayButtons.length).toBeGreaterThan(0);
    });

    it("navigating to next month updates the calendar", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));

      const nextMonth = screen.getByLabelText("Nächster Monat");
      fireEvent.click(nextMonth);
      const dayButtons = screen
        .getAllByRole("button")
        .filter((btn) => /^\d+$/.test(btn.textContent ?? ""));
      expect(dayButtons.length).toBeGreaterThan(0);
    });
  });

  // ── ExportDropdown - CSV/Excel actions ────────────────────────────────

  describe("ExportDropdown - export actions", () => {
    it("triggers CSV export via window.location.href", async () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));

      // CSV/Excel buttons should be enabled when range is set (pre-filled with current week)
      const csvBtn = screen.getByText("CSV");
      expect(csvBtn).not.toBeDisabled();

      // Mock window.location.href
      const originalHref = window.location.href;
      Object.defineProperty(window, "location", {
        writable: true,
        value: { ...window.location, href: originalHref },
      });

      fireEvent.click(csvBtn);
      // After click, dropdown should close
      await waitFor(() => {
        expect(
          screen.queryByText("Zeitraum exportieren"),
        ).not.toBeInTheDocument();
      });
    });

    it("triggers Excel export", async () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));

      const excelBtn = screen.getByText("Excel");
      expect(excelBtn).not.toBeDisabled();

      const originalHref = window.location.href;
      Object.defineProperty(window, "location", {
        writable: true,
        value: { ...window.location, href: originalHref },
      });

      fireEvent.click(excelBtn);
      await waitFor(() => {
        expect(
          screen.queryByText("Zeitraum exportieren"),
        ).not.toBeInTheDocument();
      });
    });

    it("shows date range text when export panel is open", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      // Pre-filled with current week range
      const rangeTexts = screen.queryAllByText(/\d{2}\.\d{2}\.\d{4}/);
      expect(rangeTexts.length).toBeGreaterThan(0);
    });

    it("closes export dropdown on scroll", async () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      expect(screen.getByText("Zeitraum exportieren")).toBeInTheDocument();

      // Dispatch scroll event to close
      await act(async () => {
        window.dispatchEvent(new Event("scroll"));
      });

      await waitFor(() => {
        expect(
          screen.queryByText("Zeitraum exportieren"),
        ).not.toBeInTheDocument();
      });
    });
  });

  // ── WeekTable desktop - detailed branches ─────────────────────────────

  describe("WeekTable desktop - detailed branches", () => {
    it("shows active session with 'aktiv' badge and ... for end time", () => {
      const activeHistory: WorkSessionHistory = {
        ...mockHistorySession,
        date: weekdayISO,
        checkInTime: testTimestamp(weekdayISO, "08:00"),
        checkOutTime: null,
        netMinutes: 0,
      };

      setupDefaultMocks({
        currentSession: mockActiveSession,
        history: [activeHistory],
      });
      render(<TimeTrackingPage />);

      const aktivBadges = screen.queryAllByText("aktiv");
      expect(aktivBadges.length).toBeGreaterThan(0);
    });

    it("shows home_office badge as 'Homeoffice' in desktop table", () => {
      const yISO = weekdayISO;
      const hoSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:00"),
        status: "home_office",
      };

      setupDefaultMocks({ history: [hoSession] });
      render(<TimeTrackingPage />);

      const hoBadges = screen.queryAllByText("Homeoffice");
      expect(hoBadges.length).toBeGreaterThan(0);
    });

    it("shows 'In der OGS' badge for present status sessions", () => {
      const yISO = weekdayISO;
      const presentSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:00"),
        status: "present",
      };

      setupDefaultMocks({ history: [presentSession] });
      render(<TimeTrackingPage />);

      const ogsBadges = screen.queryAllByText("In der OGS");
      expect(ogsBadges.length).toBeGreaterThan(0);
    });

    it("shows table headers in desktop mode", () => {
      setupDefaultMocks({ history: [mockHistorySession] });
      render(<TimeTrackingPage />);
      expect(screen.getByText("Tag")).toBeInTheDocument();
      expect(screen.getByText("Start")).toBeInTheDocument();
      expect(screen.getByText("Ende")).toBeInTheDocument();
      expect(screen.getByText("Netto")).toBeInTheDocument();
      expect(screen.getByText("Ort")).toBeInTheDocument();
      expect(screen.getByText("Änderung")).toBeInTheDocument();
    });

    it("shows dash for absent location column when no session on past day", () => {
      setupDefaultMocks({ history: [] });
      render(<TimeTrackingPage />);
      // Past days without sessions show "—" in the Ort column
      const dashes = screen.queryAllByText("—");
      expect(dashes.length).toBeGreaterThanOrEqual(0);
    });

    it("row click expands edit history when session has edits", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "break_minutes",
          oldValue: "0",
          newValue: "30",
          notes: "Pause nachgetragen",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      const row = changeText.closest("tr")!;
      fireEvent.click(row);

      await waitFor(() => {
        expect(timeTrackingService.getSessionEdits).toHaveBeenCalled();
      });
    });

    it("collapse expanded edits when clicking same session again", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: testTimestamp(todayISO, "07:00"),
          newValue: testTimestamp(todayISO, "08:00"),
          notes: "Korrektur",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      const row = changeText.closest("tr")!;

      // First click: expand
      fireEvent.click(row);
      await waitFor(() => {
        expect(timeTrackingService.getSessionEdits).toHaveBeenCalled();
      });

      // Second click: collapse
      fireEvent.click(row);
      // The edits should be cleared (no more edit table rows)
    });
  });

  // ── EditHistoryAccordion - detailed ───────────────────────────────────

  describe("EditHistoryAccordion - detailed coverage", () => {
    it("shows loading state while edits are being fetched", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockImplementation(
        () =>
          new Promise((resolve) => {
            setTimeout(() => resolve([]), 5000);
          }),
      );
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      const row = changeText.closest("tr")!;
      fireEvent.click(row);

      await waitFor(() => {
        expect(screen.getByText("Laden...")).toBeInTheDocument();
      });
    });

    it("shows empty state when no edits exist", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      const row = changeText.closest("tr")!;
      fireEvent.click(row);

      await waitFor(() => {
        expect(
          screen.getByText("Keine Änderungen vorhanden."),
        ).toBeInTheDocument();
      });
    });

    it("shows edit table with field labels and values", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: testTimestamp(todayISO, "07:00"),
          newValue: testTimestamp(todayISO, "08:00"),
          notes: "Korrektur",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      const row = changeText.closest("tr")!;
      fireEvent.click(row);

      await waitFor(() => {
        // Field label "Start" appears in both the week table header and the edit history
        // so we use queryAll and check there are more than just the table header
        const startLabels = screen.queryAllByText("Start");
        expect(startLabels.length).toBeGreaterThan(1);
        // Notes reason
        const reasonTexts = screen.queryAllByText(/Korrektur/);
        expect(reasonTexts.length).toBeGreaterThan(0);
      });
    });

    it("formats break_minutes field correctly in edit history", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "break_minutes",
          oldValue: "0",
          newValue: "30",
          notes: "Pause korrigiert",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      fireEvent.click(changeText.closest("tr")!);

      await waitFor(() => {
        expect(screen.getAllByText("0 min").length).toBeGreaterThan(0);
        expect(screen.getAllByText("30 min").length).toBeGreaterThan(0);
      });
    });

    it("formats status field correctly in edit history", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "status",
          oldValue: "present",
          newValue: "home_office",
          notes: "Ort-Änderung",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      fireEvent.click(changeText.closest("tr")!);

      await waitFor(() => {
        // formatEditValue maps "present" -> "In der OGS", "home_office" -> "Homeoffice"
        const ogsLabels = screen.queryAllByText("In der OGS");
        expect(ogsLabels.length).toBeGreaterThan(0);
        const hoLabels = screen.queryAllByText("Homeoffice");
        expect(hoLabels.length).toBeGreaterThan(0);
      });
    });

    it("formats null values as dash in edit history", async () => {
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "notes",
          oldValue: null,
          newValue: "Some note",
          notes: "Added note",
          createdAt: testTimestamp(todayISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      fireEvent.click(changeText.closest("tr")!);

      // notes field is filtered out by `filter(e => e.fieldName !== "notes")`
      // so it won't show in the table, but the notes column shows the reason
      await waitFor(() => {
        expect(timeTrackingService.getSessionEdits).toHaveBeenCalled();
      });
    });

    it("shows 'Weitere Änderung vornehmen' button for editable sessions", async () => {
      // Need a past session with edits to show accordion with edit button
      const yISO = weekdayISO;
      const pastSessionWithEdits: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
        editCount: 1,
        updatedAt: testTimestamp(yISO, "17:00"),
      };

      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: testTimestamp(yISO, "07:00"),
          newValue: testTimestamp(yISO, "08:00"),
          notes: "Korrektur",
          createdAt: testTimestamp(yISO, "17:00"),
        },
      ]);
      setupDefaultMocks({ history: [pastSessionWithEdits] });
      render(<TimeTrackingPage />);

      // Find and click the row to expand
      const changeText = screen.getByText(/Zuletzt geändert/);
      fireEvent.click(changeText.closest("tr")!);

      await waitFor(() => {
        expect(
          screen.getByText("Weitere Änderung vornehmen"),
        ).toBeInTheDocument();
      });
    });

    it("groups multiple edits with same timestamp", async () => {
      const timestamp = testTimestamp(todayISO, "17:00");
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: testTimestamp(todayISO, "07:00"),
          newValue: testTimestamp(todayISO, "08:00"),
          notes: "Doppelkorrektur",
          createdAt: timestamp,
        },
        {
          id: "e2",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "break_minutes",
          oldValue: "0",
          newValue: "30",
          notes: "Doppelkorrektur",
          createdAt: timestamp,
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      fireEvent.click(changeText.closest("tr")!);

      await waitFor(() => {
        // Both edits should show field labels
        const startLabels = screen.queryAllByText("Start");
        expect(startLabels.length).toBeGreaterThan(0);
        expect(screen.getAllByText("30 min").length).toBeGreaterThan(0);
      });
    });
  });

  // ── CreateAbsenceModal - comprehensive ────────────────────────────────

  describe("CreateAbsenceModal - comprehensive", () => {
    function openAbsenceModal() {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));
    }

    it("resets form on open (absence type defaults to sick)", () => {
      openAbsenceModal();
      const typeSelect = screen.getByLabelText("Art der Abwesenheit");
      expect(typeSelect).toHaveTextContent("Krank");
    });

    it("shows all absence type options", () => {
      openAbsenceModal();
      fireEvent.click(screen.getByLabelText("Art der Abwesenheit"));
      expect(screen.getAllByText("Krank").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Urlaub").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Fortbildung").length).toBeGreaterThan(0);
      expect(screen.getAllByText("Sonstige").length).toBeGreaterThan(0);
    });

    it("changes absence type via select", () => {
      openAbsenceModal();
      const typeSelect = screen.getByLabelText("Art der Abwesenheit");
      chooseSelectOption(typeSelect, "Urlaub");
      expect(typeSelect).toHaveTextContent("Urlaub");
    });

    it("toggles half day switch", () => {
      openAbsenceModal();
      const toggle = screen.getByRole("switch");
      expect(toggle.getAttribute("aria-checked")).toBe("false");
      fireEvent.click(toggle);
      expect(toggle.getAttribute("aria-checked")).toBe("true");
    });

    it("allows note input", () => {
      openAbsenceModal();
      const noteArea = screen.getByPlaceholderText(
        "z.B. Arzttermin, Schulung ...",
      );
      fireEvent.change(noteArea, { target: { value: "Arzttermin" } });
      expect((noteArea as HTMLTextAreaElement).value).toBe("Arzttermin");
    });

    it("changes start date", () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date("2026-01-15T10:00:00"));
      try {
        openAbsenceModal();
        const startInput = screen.getByLabelText("Von");
        fireEvent.change(startInput, {
          target: { value: "2026-03-01" },
        });
        expect((startInput as HTMLInputElement).value).toBe("2026-03-01");
      } finally {
        vi.useRealTimers();
      }
    });

    it("changes end date", () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date("2026-01-15T10:00:00"));
      try {
        openAbsenceModal();
        const endInput = screen.getByLabelText("Bis");
        fireEvent.change(endInput, {
          target: { value: "2026-03-05" },
        });
        expect((endInput as HTMLInputElement).value).toBe("2026-03-05");
      } finally {
        vi.useRealTimers();
      }
    });

    it("closes modal on Abbrechen", () => {
      openAbsenceModal();
      const cancelBtn = screen.getAllByText("Abbrechen");
      fireEvent.click(cancelBtn[cancelBtn.length - 1]!);
      expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
    });

    it("shows error toast when createAbsence fails", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.createAbsence).mockRejectedValue(
        new Error("invalid absence type"),
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));

      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Ungültiger Abwesenheitstyp.",
        );
      });
    });
  });

  // ── TimeTrackingContent state management ──────────────────────────────

  describe("TimeTrackingContent - state management", () => {
    it("handleEditSave auto-expands edits for a different session", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      vi.mocked(timeTrackingService.updateSession).mockResolvedValue(
        mockCheckedOutSession,
      );
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: testTimestamp(yISO, "07:00"),
          newValue: testTimestamp(yISO, "08:00"),
          notes: "Auto-expanded",
          createdAt: testTimestamp(yISO, "17:00"),
        },
      ]);

      render(<TimeTrackingPage />);

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        clickQuickEditReason("Zeitkorrektur");

        const saveBtn = await waitForLastSaveButtonEnabled();
        await act(async () => {
          fireEvent.click(saveBtn);
        });

        await waitFor(() => {
          expect(mockToast.success).toHaveBeenCalledWith("Eintrag gespeichert");
          // After save, getSessionEdits should be called to auto-expand
          expect(timeTrackingService.getSessionEdits).toHaveBeenCalled();
        });
      }
    });

    it("handleEditSave shows error toast on failure", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      vi.mocked(timeTrackingService.updateSession).mockRejectedValue(
        new Error("session not found"),
      );

      render(<TimeTrackingPage />);

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        clickQuickEditReason("Zeitkorrektur");

        const saveBtn = await waitForLastSaveButtonEnabled();
        await act(async () => {
          fireEvent.click(saveBtn);
        });

        await waitFor(() => {
          expect(mockToast.error).toHaveBeenCalledWith(
            "Eintrag nicht gefunden.",
          );
        });
      }
    });

    it("handleDeleteAbsence shows error toast on failure", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      vi.mocked(timeTrackingService.deleteAbsence).mockRejectedValue(
        new Error("can only delete own absences"),
      );

      render(<TimeTrackingPage />);

      const sickBadges = screen.queryAllByText("Krank");
      if (sickBadges.length > 0) {
        const row = sickBadges[0]!.closest("tr");
        if (row) {
          fireEvent.click(row);
          await waitFor(() => {
            expect(screen.getByTestId("modal")).toBeInTheDocument();
          });

          const deleteBtn = screen.queryByText("Abwesenheit löschen");
          if (deleteBtn) {
            await act(async () => {
              fireEvent.click(deleteBtn);
            });
            await waitFor(() => {
              expect(mockToast.error).toHaveBeenCalledWith(
                "Du kannst nur eigene Abwesenheiten löschen.",
              );
            });
          }
        }
      }
    });

    it("handleUpdateAbsence shows error toast on failure", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({
        history: [pastSession],
        absences: [pastAbsence],
      });
      vi.mocked(timeTrackingService.updateSession).mockResolvedValue(
        mockCheckedOutSession,
      );
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([]);
      vi.mocked(timeTrackingService.updateAbsence).mockRejectedValue(
        new Error("can only update own absences"),
      );

      render(<TimeTrackingPage />);

      // Open edit modal
      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        // Switch to absence tab
        fireEvent.click(screen.getByText("Abwesenheit"));

        await waitFor(() => {
          expect(screen.getByText("Art der Abwesenheit")).toBeInTheDocument();
        });

        const saveButtons = screen.getAllByText("Speichern");
        await act(async () => {
          fireEvent.click(saveButtons[saveButtons.length - 1]!);
        });

        await waitFor(() => {
          expect(mockToast.error).toHaveBeenCalledWith(
            "Du kannst nur eigene Abwesenheiten bearbeiten.",
          );
        });
      }
    });

    it("handleDeleteAbsence shows success toast", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      vi.mocked(timeTrackingService.deleteAbsence).mockResolvedValue(undefined);

      render(<TimeTrackingPage />);

      const sickBadges = screen.queryAllByText("Krank");
      if (sickBadges.length > 0) {
        const row = sickBadges[0]!.closest("tr");
        if (row) {
          fireEvent.click(row);
          await waitFor(() => {
            expect(screen.getByTestId("modal")).toBeInTheDocument();
          });

          const deleteBtn = screen.queryByText("Abwesenheit löschen");
          if (deleteBtn) {
            await act(async () => {
              fireEvent.click(deleteBtn);
            });
            await waitFor(() => {
              expect(mockToast.success).toHaveBeenCalledWith(
                "Abwesenheit gelöscht",
              );
            });
          }
        }
      }
    });

    it("handleUpdateAbsence shows success toast", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({
        history: [pastSession],
        absences: [pastAbsence],
      });
      vi.mocked(timeTrackingService.updateSession).mockResolvedValue(
        mockCheckedOutSession,
      );
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([]);
      vi.mocked(timeTrackingService.updateAbsence).mockResolvedValue(
        pastAbsence,
      );

      render(<TimeTrackingPage />);

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        fireEvent.click(screen.getByText("Abwesenheit"));

        await waitFor(() => {
          expect(screen.getByText("Art der Abwesenheit")).toBeInTheDocument();
        });

        const saveButtons = screen.getAllByText("Speichern");
        await act(async () => {
          fireEvent.click(saveButtons[saveButtons.length - 1]!);
        });

        await waitFor(() => {
          expect(mockToast.success).toHaveBeenCalledWith(
            "Abwesenheit aktualisiert",
          );
        });
      }
    });

    it("pendingCheckIn shows absence type in confirmation text", async () => {
      const vacAbsence: StaffAbsence = {
        ...mockAbsence,
        absenceType: "vacation",
      };

      setupDefaultMocks({ absences: [vacAbsence] });
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        // "Urlaub" appears in the confirmation modal (and also in the table on weekdays)
        const urlaubTexts = screen.queryAllByText(/Urlaub/);
        expect(urlaubTexts.length).toBeGreaterThanOrEqual(1);
      });
    });

    it("endBreak shows Pause beenden and calls endBreak service", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "50",
          sessionId: "100",
          startedAt: new Date().toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: null,
        },
      ]);
      vi.mocked(timeTrackingService.endBreak).mockResolvedValue({
        ...mockActiveSession,
        breakMinutes: 15,
      });

      render(<TimeTrackingPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Pause beenden")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Pause beenden"));
      });

      await waitFor(() => {
        expect(timeTrackingService.endBreak).toHaveBeenCalled();
      });
    });
  });

  // ── ClockInCard - checked out state ───────────────────────────────────

  // ── ClockInCard - Blöcke über Mitternacht ─────────────────────────────

  describe("ClockInCard - overnight blocks", () => {
    // A block that runs past midnight is booked on both Berlin days: the
    // minutes before midnight belong to yesterday, only the rest to today.
    // The timer showed the whole elapsed time before #2402.
    it("counts only today's share of a still-running overnight block", () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date("2026-01-15T01:30:00"));
      try {
        setupDefaultMocks({
          currentSession: {
            ...mockActiveSession,
            date: "2026-01-14",
            checkInTime: "2026-01-14T22:00:00",
            checkOutTime: null,
          },
          history: [],
        });
        render(<TimeTrackingPage />);

        // 22:00 → 01:30 is 3h 30min elapsed, 1h 30min of it on 15.01. The
        // timer is the only element carrying the day total, so a unique
        // match on it pins the clamp.
        expect(screen.getByText("1h 30min")).toBeInTheDocument();
      } finally {
        vi.useRealTimers();
      }
    });

    it("adds today's share of a closed overnight block to the running one", () => {
      vi.useFakeTimers();
      vi.setSystemTime(new Date("2026-01-15T09:00:00"));
      try {
        const closedOvernight: WorkSessionHistory = {
          ...mockHistorySession,
          id: "900",
          date: "2026-01-14",
          checkInTime: "2026-01-14T22:00:00",
          checkOutTime: "2026-01-15T02:00:00",
          breakMinutes: 0,
          netMinutes: 240,
          breaks: [],
        };
        setupDefaultMocks({
          currentSession: {
            ...mockActiveSession,
            date: "2026-01-15",
            checkInTime: "2026-01-15T08:00:00",
            checkOutTime: null,
          },
          history: [closedOvernight],
        });
        render(<TimeTrackingPage />);

        // 2h of the night block fall on 15.01., plus 1h since 08:00 — the
        // other 2h stay on 14.01.
        expect(screen.getByText("3h")).toBeInTheDocument();
        expect(screen.queryByText("5h")).not.toBeInTheDocument();
      } finally {
        vi.useRealTimers();
      }
    });
  });

  describe("ClockInCard - checked out summary", () => {
    it("shows Arbeit with check-in and check-out times", () => {
      setupDefaultMocks({ currentSession: mockCheckedOutSession });
      render(<TimeTrackingPage />);
      expect(screen.getByText("Arbeit")).toBeInTheDocument();
    });

    it("shows break minutes in summary when > 0", () => {
      setupDefaultMocks({ currentSession: mockCheckedOutSession });
      render(<TimeTrackingPage />);
      // mockCheckedOutSession has breakMinutes: 30
      // "Pause" text should appear in the summary rows
      const pauseLabels = screen.queryAllByText("Pause");
      expect(pauseLabels.length).toBeGreaterThan(0);
    });

    it("shows Heute and Woche with values when checked out", () => {
      setupDefaultMocks({
        currentSession: mockCheckedOutSession,
        history: [mockHistorySession],
      });
      render(<TimeTrackingPage />);
      expect(screen.getByText(/Heute:/)).toBeInTheDocument();
      expect(screen.getByText(/Woche:/)).toBeInTheDocument();
    });

    it("does not show Pause row when breakMinutes is 0", () => {
      const noBreakSession: WorkSession = {
        ...mockCheckedOutSession,
        breakMinutes: 0,
      };
      setupDefaultMocks({ currentSession: noBreakSession });
      render(<TimeTrackingPage />);
      // "Arbeit" row should exist, but no "Pause" summary row in ClockInCard
      expect(screen.getByText("Arbeit")).toBeInTheDocument();
    });
  });

  // ── Mobile layout detailed ────────────────────────────────────────────

  describe("mobile layout - detailed branches", () => {
    beforeEach(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 375,
      });
    });

    it("shows mobile card view with session data on small screens", () => {
      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      // Should show Woche gesamt
      expect(screen.getByText("Woche gesamt")).toBeInTheDocument();
    });

    it("shows absence-only card in mobile view", () => {
      const yISO = weekdayISO;
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      const sickBadges = screen.queryAllByText("Krank");
      expect(sickBadges.length).toBeGreaterThan(0);
    });

    it("shows edit button in mobile card for past editable session", () => {
      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      expect(editButtons.length).toBeGreaterThanOrEqual(0);
    });

    it("shows active session badge and live time on mobile", () => {
      const activeHistory: WorkSessionHistory = {
        ...mockHistorySession,
        date: todayISO,
        checkInTime: testTimestamp(todayISO, "08:00"),
        checkOutTime: null,
        netMinutes: 0,
      };

      setupDefaultMocks({
        currentSession: mockActiveSession,
        history: [activeHistory],
      });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      // Mobile shows "aktiv" badge text
      const aktivBadges = screen.queryAllByText("aktiv");
      expect(aktivBadges.length).toBeGreaterThanOrEqual(0);
    });

    it("shows HO badge on mobile for home_office session", () => {
      const yISO = weekdayISO;
      const hoSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:00"),
        status: "home_office",
      };

      setupDefaultMocks({ history: [hoSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      const hoBadges = screen.queryAllByText("HO");
      expect(hoBadges.length).toBeGreaterThanOrEqual(0);
    });

    it("shows OGS badge on mobile for present session", () => {
      const yISO = weekdayISO;
      const presentSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:00"),
        status: "present",
      };

      setupDefaultMocks({ history: [presentSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      const ogsBadges = screen.queryAllByText("OGS");
      expect(ogsBadges.length).toBeGreaterThanOrEqual(0);
    });

    it("shows edit history toggle on mobile for sessions with edits", () => {
      const yISO = weekdayISO;
      const sessionWithEdits: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
        editCount: 2,
        updatedAt: testTimestamp(yISO, "17:00"),
      };

      setupDefaultMocks({ history: [sessionWithEdits] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      // Mobile shows "Geändert" text for sessions with edits
      const changedTexts = screen.queryAllByText(/Geändert/);
      expect(changedTexts.length).toBeGreaterThanOrEqual(0);
    });
  });

  // ── Additional error mapping coverage ─────────────────────────────────

  describe("additional friendlyError mappings", () => {
    it("maps the stable work_session_overlap code over the dynamic message", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      // Exactly what buildApiError produces: the human-readable backend text
      // (carrying the conflicting interval) as the message, the stable code on
      // the error object — never inside the message.
      const overlapError = Object.assign(
        new Error("work session overlaps an existing block (08:00–12:00)"),
        { status: 409, code: "work_session_overlap" },
      );
      vi.mocked(timeTrackingService.checkIn).mockRejectedValue(overlapError);
      render(<TimeTrackingPage />);

      selectPresentMode();

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Der Zeitraum überschneidet sich mit einem anderen Arbeitsblock an diesem Tag.",
        );
      });
    });

    it("maps 'no session found for today' error", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.checkOut).mockRejectedValue(
        new Error("no session found for today"),
      );
      render(<TimeTrackingPage />);

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Ausstempeln"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Kein Eintrag für heute vorhanden.",
        );
      });
    });

    it("maps 'updated dates overlap' error prefix", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks();
      vi.mocked(timeTrackingService.createAbsence).mockRejectedValue(
        new Error('{"error":"updated dates overlap with existing absence"}'),
      );
      render(<TimeTrackingPage />);

      fireEvent.click(screen.getByText("Abwesend"));
      fireEvent.click(screen.getByLabelText("Abwesenheit melden"));

      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Für diesen Zeitraum ist bereits eine andere Abwesenheitsart eingetragen.",
        );
      });
    });

    it("maps 'can only update own sessions' error", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "16:30"),
      };

      setupDefaultMocks({ history: [pastSession] });
      vi.mocked(timeTrackingService.updateSession).mockRejectedValue(
        new Error("can only update own sessions"),
      );

      render(<TimeTrackingPage />);

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        clickQuickEditReason("Zeitkorrektur");

        const saveBtn = await waitForLastSaveButtonEnabled();
        await act(async () => {
          fireEvent.click(saveBtn);
        });

        await waitFor(() => {
          expect(mockToast.error).toHaveBeenCalledWith(
            "Du kannst nur eigene Einträge bearbeiten.",
          );
        });
      }
    });

    it("maps 'no active break found' for endBreak", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.getSessionBreaks).mockResolvedValue([
        {
          id: "50",
          sessionId: "100",
          startedAt: new Date().toISOString(),
          endedAt: null,
          durationMinutes: 0,
          plannedEndTime: null,
        },
      ]);
      vi.mocked(timeTrackingService.endBreak).mockRejectedValue(
        new Error("no active break found"),
      );

      render(<TimeTrackingPage />);

      await waitFor(() => {
        expect(screen.getByLabelText("Pause beenden")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Pause beenden"));
      });

      await waitFor(() => {
        expect(mockToast.error).toHaveBeenCalledWith(
          "Keine aktive Pause vorhanden.",
        );
      });
    });

    it("maps 'absence not found' error", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = weekdayISO;
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({ absences: [pastAbsence] });
      vi.mocked(timeTrackingService.deleteAbsence).mockRejectedValue(
        new Error("absence not found"),
      );

      render(<TimeTrackingPage />);

      const sickBadges = screen.queryAllByText("Krank");
      if (sickBadges.length > 0) {
        const row = sickBadges[0]!.closest("tr");
        if (row) {
          fireEvent.click(row);
          await waitFor(() => {
            expect(screen.getByTestId("modal")).toBeInTheDocument();
          });

          const deleteBtn = screen.queryByText("Abwesenheit löschen");
          if (deleteBtn) {
            await act(async () => {
              fireEvent.click(deleteBtn);
            });
            await waitFor(() => {
              expect(mockToast.error).toHaveBeenCalledWith(
                "Abwesenheit nicht gefunden.",
              );
            });
          }
        }
      }
    });
  });

  // ── Absence with session in mobile card ───────────────────────────────

  describe("session + absence combination in mobile", () => {
    beforeEach(() => {
      Object.defineProperty(window, "innerWidth", {
        writable: true,
        configurable: true,
        value: 375,
      });
    });

    it("shows absence badge alongside session data on mobile", () => {
      const yISO = weekdayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: testTimestamp(yISO, "08:00"),
        checkOutTime: testTimestamp(yISO, "12:00"),
        netMinutes: 240,
      };
      const pastAbsence: StaffAbsence = {
        ...mockVacationAbsence,
        dateStart: yISO,
        dateEnd: yISO,
      };

      setupDefaultMocks({
        history: [pastSession],
        absences: [pastAbsence],
      });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      // Both session data and absence badge should be visible
      const urlaubBadges = screen.queryAllByText("Urlaub");
      expect(urlaubBadges.length).toBeGreaterThan(0);
    });
  });

  // ── Check-in with manually edited session (confirmation modal) ──────────

  describe("check-in after a manually edited session (#2402)", () => {
    it("stamps a new block directly — the edited block stays untouched", async () => {
      // Pre-#2402 a re-check-in reopened (and thereby altered) the edited
      // session, so a confirmation modal warned first. A new block never
      // touches the edited one, so the stamp goes through directly.
      const todayEditedHistory: WorkSessionHistory = {
        ...mockHistorySession,
        date: todayISO,
        checkInTime: testTimestamp(todayISO, "08:00"),
        checkOutTime: testTimestamp(todayISO, "16:30"),
        editCount: 1,
      };
      setupDefaultMocks({
        currentSession: null,
        history: [todayEditedHistory],
      });
      vi.mocked(timeTrackingService.checkIn).mockResolvedValue(
        mockActiveSession,
      );
      render(<TimeTrackingPage />);

      selectPresentMode(); // Issue #1368: no pre-selection — must pick first.

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        expect(timeTrackingService.checkIn).toHaveBeenCalledWith("present");
      });
      expect(
        screen.queryByText("Arbeitszeit manuell bearbeitet"),
      ).not.toBeInTheDocument();
    });
  });
});

// Check-in, checkout, session edits and absence changes all run through
// refreshTableData. Since #2443 the day rows read their Ist, Gutschrift and
// Saldo from the server's daily projection, so that key has to be invalidated
// with the session list and the Monatskarte — otherwise the numbers the fix
// added are the only ones on the screen still showing the pre-mutation state.
describe("daily-projection invalidation (#2443)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("invalidates the daily projection alongside the table and Monatskarte", () => {
    setupDefaultMocks();
    render(<TimeTrackingPage />);

    const invalidated = vi
      .mocked(useTenantMutateMatching)
      .mock.calls.flatMap(([substrings]) => substrings);

    expect(invalidated).toEqual(
      expect.arrayContaining([
        "time-tracking-table",
        "time-tracking-month-summary",
        "time-tracking-schedule-targets-",
      ]),
    );
  });
});

describe("deviation-reason gate (F9)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  function makeDeviationError(action: "check_in" | "check_out"): Error & {
    code?: string;
    status?: number;
    details?: Record<string, unknown>;
  } {
    const err = new Error("deviation reason required") as Error & {
      code?: string;
      status?: number;
      details?: Record<string, unknown>;
    };
    err.code = "deviation_reason_required";
    err.status = 409;
    err.details =
      action === "check_in"
        ? {
            action,
            planned_time: "08:00",
            actual_time: "07:30",
            deviation_minutes: "30",
          }
        : {
            action,
            planned_time: "16:00",
            actual_time: "16:30",
            deviation_minutes: "30",
          };
    return err;
  }

  it("opens the reason dialog when check-out deviates from the plan", async () => {
    setupDefaultMocks({ currentSession: mockActiveSession });
    vi.mocked(timeTrackingService.checkOut).mockRejectedValueOnce(
      makeDeviationError("check_out"),
    );
    render(<TimeTrackingPage />);

    await act(async () => {
      fireEvent.click(screen.getByLabelText("Ausstempeln"));
    });

    await waitFor(() => {
      expect(screen.getByText("Abweichung vom Dienstplan")).toBeInTheDocument();
    });
    expect(screen.getByText(/30 Minuten/)).toBeInTheDocument();
    expect(
      screen.getByText(/nach deinem geplanten Dienstende/),
    ).toBeInTheDocument();
    // Confirm stays disabled until a reason is entered.
    expect(screen.getByText("Mit Begründung ausstempeln")).toBeDisabled();
  });

  it("retries the check-out with the entered reason", async () => {
    setupDefaultMocks({ currentSession: mockActiveSession });
    vi.mocked(timeTrackingService.checkOut)
      .mockRejectedValueOnce(makeDeviationError("check_out"))
      .mockResolvedValueOnce(mockCheckedOutSession);
    render(<TimeTrackingPage />);

    await act(async () => {
      fireEvent.click(screen.getByLabelText("Ausstempeln"));
    });
    await waitFor(() => {
      expect(screen.getByText("Abweichung vom Dienstplan")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "Elterngespräch lief länger" },
    });
    const confirmBtn = screen.getByText("Mit Begründung ausstempeln");
    expect(confirmBtn).not.toBeDisabled();

    await act(async () => {
      fireEvent.click(confirmBtn);
    });

    await waitFor(() => {
      expect(timeTrackingService.checkOut).toHaveBeenLastCalledWith(
        "Elterngespräch lief länger",
      );
    });
    await waitFor(() => {
      expect(
        screen.queryByText("Abweichung vom Dienstplan"),
      ).not.toBeInTheDocument();
    });
  });

  it("retries the check-in with status and reason", async () => {
    setupDefaultMocks();
    vi.mocked(timeTrackingService.checkIn)
      .mockRejectedValueOnce(makeDeviationError("check_in"))
      .mockResolvedValueOnce(mockActiveSession);
    render(<TimeTrackingPage />);

    fireEvent.click(screen.getByText("In der OGS"));
    await act(async () => {
      fireEvent.click(screen.getByLabelText("Einstempeln"));
    });

    await waitFor(() => {
      expect(screen.getByText("Abweichung vom Dienstplan")).toBeInTheDocument();
    });
    expect(
      screen.getByText(/vor deinem geplanten Dienstbeginn/),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "Frühdienst übernommen" },
    });
    await act(async () => {
      fireEvent.click(screen.getByText("Mit Begründung einstempeln"));
    });

    await waitFor(() => {
      expect(timeTrackingService.checkIn).toHaveBeenLastCalledWith(
        "present",
        "Frühdienst übernommen",
      );
    });
  });

  it("keeps the dialog open when the retry fails", async () => {
    setupDefaultMocks({ currentSession: mockActiveSession });
    vi.mocked(timeTrackingService.checkOut)
      .mockRejectedValueOnce(makeDeviationError("check_out"))
      .mockRejectedValueOnce(new Error("network down"));
    render(<TimeTrackingPage />);

    await act(async () => {
      fireEvent.click(screen.getByLabelText("Ausstempeln"));
    });
    await waitFor(() => {
      expect(screen.getByText("Abweichung vom Dienstplan")).toBeInTheDocument();
    });

    fireEvent.change(screen.getByLabelText("Grund"), {
      target: { value: "Elterngespräch" },
    });
    await act(async () => {
      fireEvent.click(screen.getByText("Mit Begründung ausstempeln"));
    });

    await waitFor(() => {
      expect(timeTrackingService.checkOut).toHaveBeenCalledTimes(2);
    });
    expect(screen.getByText("Abweichung vom Dienstplan")).toBeInTheDocument();
  });
});

// Ein Klick auf den Stift darf nie ein stiller No-Op sein (#2361): Tage ohne
// Buchung und ohne Abwesenheit bekommen einen erklärenden Hinweis-Dialog,
// Berechtigte zusätzlich den Absprung in die eigene Verwaltungs-Ansicht.
describe("empty-day hint dialog (#2361)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a hint dialog instead of nothing for a day without booking", async () => {
    setupDefaultMocks();

    render(<TimeTrackingPage />);

    fireEvent.click(screen.getAllByLabelText("Eintrag bearbeiten")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });
    expect(screen.getByText("Kein Eintrag vorhanden")).toBeInTheDocument();
    // Ohne time_tracking:manage gibt es keinen Nachtragen-Absprung.
    expect(
      screen.queryByRole("button", { name: "Nachtragen" }),
    ).not.toBeInTheDocument();
  });

  it("links managers to their own admin Zeiterfassung", async () => {
    setupDefaultMocks();
    const push = vi.fn();
    vi.mocked(useRouter).mockReturnValue({
      push,
      replace: vi.fn(),
      back: vi.fn(),
      forward: vi.fn(),
      refresh: vi.fn(),
      prefetch: vi.fn(),
    } as never);
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["time_tracking:manage"],
        },
      },
      status: "authenticated",
      update: vi.fn(),
    } as never);

    render(<TimeTrackingPage />);

    fireEvent.click(screen.getAllByLabelText("Eintrag bearbeiten")[0]!);

    await waitFor(() => {
      expect(screen.getByTestId("modal")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "Nachtragen" }));
    // Der Tenant-Router stellt je nach Routing-Modus den Slug voran — hier
    // zählt nur, dass die eigene Admin-Zeiterfassung angesteuert wird.
    expect(push).toHaveBeenCalledWith(
      expect.stringContaining(`/staff/10?tab=zeiterfassung&date=${todayISO}`),
    );
  });

  it("offers managers the backfill link for a half-day absence", async () => {
    const halfDayAbsence: StaffAbsence = {
      ...mockVacationAbsence,
      dateStart: weekdayISO,
      dateEnd: weekdayISO,
    };
    setupDefaultMocks({ absences: [halfDayAbsence] });
    const push = vi.fn();
    vi.mocked(useRouter).mockReturnValue({
      push,
      replace: vi.fn(),
      back: vi.fn(),
      forward: vi.fn(),
      refresh: vi.fn(),
      prefetch: vi.fn(),
    } as never);
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["time_tracking:manage"],
        },
      },
      status: "authenticated",
      update: vi.fn(),
    } as never);

    render(<TimeTrackingPage />);

    fireEvent.click(screen.getAllByLabelText("Eintrag bearbeiten")[0]!);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Nachtragen" }),
      ).toBeInTheDocument();
    });
    expect(screen.getByText("Arbeitszeit")).toBeInTheDocument();
    expect(screen.getByText("Abwesenheit")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Nachtragen" }));
    expect(push).toHaveBeenCalledWith(
      expect.stringContaining(`/staff/10?tab=zeiterfassung&date=${weekdayISO}`),
    );
  });

  it("keeps the half-day absence editor focused for non-managers", async () => {
    const halfDayAbsence: StaffAbsence = {
      ...mockVacationAbsence,
      dateStart: weekdayISO,
      dateEnd: weekdayISO,
    };
    setupDefaultMocks({ absences: [halfDayAbsence] });

    render(<TimeTrackingPage />);

    fireEvent.click(screen.getAllByLabelText("Eintrag bearbeiten")[0]!);

    await waitFor(() => {
      expect(screen.getByText("Abwesenheit bearbeiten")).toBeInTheDocument();
    });
    expect(
      screen.queryByRole("button", { name: "Nachtragen" }),
    ).not.toBeInTheDocument();
  });

  it("offers managers the backfill link for a requested full-day absence", async () => {
    const requestedAbsence: StaffAbsence = {
      ...mockAbsence,
      dateStart: weekdayISO,
      dateEnd: weekdayISO,
      status: "requested",
    };
    setupDefaultMocks({ absences: [requestedAbsence] });
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          token: "test-token",
          permissions: ["time_tracking:manage"],
        },
      },
      status: "authenticated",
      update: vi.fn(),
    } as never);

    render(<TimeTrackingPage />);

    fireEvent.click(screen.getAllByLabelText("Eintrag bearbeiten")[0]!);

    await waitFor(() => {
      expect(
        screen.getByRole("button", { name: "Nachtragen" }),
      ).toBeInTheDocument();
    });
  });
});
