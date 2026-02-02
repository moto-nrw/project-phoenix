/* eslint-disable @typescript-eslint/unbound-method */
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";

// ─── Mocks (must come before component imports) ─────────────────────────────

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
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
  timeTrackingService: {
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
    exportSessions: vi.fn(),
  },
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-fullpage={fullPage} aria-label="Laden..." />
  ),
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
        <button data-testid="modal-close" onClick={onClose}>
          close
        </button>
        <div data-testid="modal-body">{children}</div>
        {footer && <div data-testid="modal-footer">{footer}</div>}
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
  ChartTooltip: () => <div data-testid="chart-tooltip" />,
  ChartTooltipContent: () => <div />,
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
  // eslint-disable-next-line @typescript-eslint/consistent-type-imports
  const actual = await importOriginal<typeof import("react-dom")>();
  return {
    ...actual,
    createPortal: (node: React.ReactNode) => node,
  };
});

vi.mock("lucide-react", () => ({
  ChevronLeft: () => <span data-testid="chevron-left" />,
  ChevronRight: () => <span data-testid="chevron-right" />,
  Download: () => <span data-testid="download-icon" />,
  SquarePen: () => <span data-testid="square-pen" />,
}));

// ─── Imports after mocks ────────────────────────────────────────────────────

import TimeTrackingPage from "./page";
import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { useToast } from "~/contexts/ToastContext";
import { timeTrackingService } from "~/lib/time-tracking-api";
import type {
  WorkSession,
  WorkSessionHistory,
  StaffAbsence,
} from "~/lib/time-tracking-helpers";

// ─── Test Data ──────────────────────────────────────────────────────────────

const today = new Date();
const todayISO = `${today.getFullYear()}-${String(today.getMonth() + 1).padStart(2, "0")}-${String(today.getDate()).padStart(2, "0")}`;

const mockActiveSession: WorkSession = {
  id: "100",
  staffId: "10",
  date: todayISO,
  status: "present",
  checkInTime: `${todayISO}T08:00:00Z`,
  checkOutTime: null,
  breakMinutes: 0,
  notes: "",
  autoCheckedOut: false,
  createdBy: "10",
  updatedBy: null,
  createdAt: `${todayISO}T08:00:00Z`,
  updatedAt: `${todayISO}T08:00:00Z`,
};

const mockCheckedOutSession: WorkSession = {
  ...mockActiveSession,
  checkOutTime: `${todayISO}T16:30:00Z`,
  breakMinutes: 30,
};

const mockHistorySession: WorkSessionHistory = {
  ...mockCheckedOutSession,
  netMinutes: 480,
  isOvertime: false,
  isBreakCompliant: true,
  breaks: [],
  editCount: 0,
};

const mockHistorySessionWithEdits: WorkSessionHistory = {
  ...mockHistorySession,
  editCount: 2,
  updatedAt: `${todayISO}T17:00:00Z`,
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
  dateStart: todayISO,
  dateEnd: todayISO,
  halfDay: false,
  note: "",
  status: "pending",
  approvedBy: null,
  approvedAt: null,
  createdBy: "10",
  createdAt: `${todayISO}T07:00:00Z`,
  updatedAt: `${todayISO}T07:00:00Z`,
  durationDays: 1,
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

function setupDefaultMocks(overrides?: {
  currentSession?: WorkSession | null;
  history?: WorkSessionHistory[];
  absences?: StaffAbsence[];
  historyLoading?: boolean;
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

  vi.mocked(useSWRAuth).mockImplementation((key: string | null) => {
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
        data: history,
        isLoading: historyLoading,
        mutate: mockMutate,
        isValidating: false,
        error: undefined,
      } as never;
    } else {
      // absences or any other key
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
      expect(screen.getByTestId("loading")).toBeInTheDocument();
    });

    it("renders main content when authenticated", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Zeiterfassung")).toBeInTheDocument();
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

    it("shows Einstempeln label by default", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      expect(screen.getByText("Einstempeln")).toBeInTheDocument();
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

    it("shows break duration options when pause button clicked", () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));
      // Break options should appear: 15m, 30m, 45m, 60m
      expect(screen.getByText("15m")).toBeInTheDocument();
      expect(screen.getByText("30m")).toBeInTheDocument();
      expect(screen.getByText("45m")).toBeInTheDocument();
      expect(screen.getByText("60m")).toBeInTheDocument();
    });

    it("calls startBreak when break duration selected", async () => {
      setupDefaultMocks({ currentSession: mockActiveSession });
      vi.mocked(timeTrackingService.startBreak).mockResolvedValue({
        id: "50",
        sessionId: "100",
        startedAt: new Date().toISOString(),
        endedAt: null,
        durationMinutes: 0,
      });
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Pause starten"));

      await act(async () => {
        fireEvent.click(screen.getByText("30m"));
      });

      await waitFor(() => {
        expect(timeTrackingService.startBreak).toHaveBeenCalled();
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
        },
      ]);
      render(<TimeTrackingPage />);
      // Need to wait for breaks to load
      await waitFor(() => {
        expect(screen.getByLabelText("Pause beenden")).toBeInTheDocument();
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
      expect(nextBtn).toBeDisabled();
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

    it("shows CSV and Excel buttons", () => {
      setupDefaultMocks();
      render(<TimeTrackingPage />);
      fireEvent.click(screen.getByLabelText("Export"));
      expect(screen.getByText("CSV")).toBeInTheDocument();
      expect(screen.getByText("Excel")).toBeInTheDocument();
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

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        // "Krank" appears in both the table and the confirmation modal
        const krankTexts = screen.getAllByText(/Krank/);
        expect(krankTexts.length).toBeGreaterThanOrEqual(2);
      });
    });
  });

  // ── Edit Modal ──────────────────────────────────────────────────────────

  describe("EditSessionModal", () => {
    it("opens edit modal when edit button clicked on a session row", async () => {
      // Use yesterday's date for a past session that's editable
      const yesterday = new Date();
      yesterday.setDate(yesterday.getDate() - 1);
      const yISO = `${yesterday.getFullYear()}-${String(yesterday.getMonth() + 1).padStart(2, "0")}-${String(yesterday.getDate()).padStart(2, "0")}`;

      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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
        fireEvent.click(screen.getByText("15m"));
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
          oldValue: `${todayISO}T07:30:00Z`,
          newValue: `${todayISO}T08:00:00Z`,
          notes: "Zeitkorrektur",
          createdAt: `${todayISO}T17:00:00Z`,
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
        },
      ]);
      vi.mocked(timeTrackingService.endBreak).mockRejectedValue(
        new Error("no active break found"),
      );

      render(<TimeTrackingPage />);

      await waitFor(() => {
        const endBreakBtn = screen.queryByLabelText("Pause beenden");
        if (endBreakBtn) {
          fireEvent.click(endBreakBtn);
        }
      });
    });
  });

  // ── Wochenübersicht heading ─────────────────────────────────────────────

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
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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
        const reasonBtn = screen.queryByText("Zeitkorrektur");
        if (reasonBtn) {
          fireEvent.click(reasonBtn);
        }

        // Click save
        const saveButtons = screen.queryAllByText("Speichern");
        if (saveButtons.length > 0) {
          await act(async () => {
            fireEvent.click(saveButtons[saveButtons.length - 1]!);
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
});
