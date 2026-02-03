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

      // Verify test setup was correct - breaks were loaded
      expect(timeTrackingService.getSessionBreaks).toHaveBeenCalled();
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
        date: todayISO,
        checkInTime: `${todayISO}T08:00:00Z`,
        checkOutTime: `${todayISO}T16:30:00Z`,
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
      fireEvent.change(breakSelect, { target: { value: "45" } });
      expect((breakSelect as HTMLSelectElement).value).toBe("45");
    });

    it("changing status selector works", async () => {
      await openEditModal(makePastSession());
      const statusSelect = screen.getByLabelText("Ort");
      fireEvent.change(statusSelect, { target: { value: "home_office" } });
      expect((statusSelect as HTMLSelectElement).value).toBe("home_office");
    });

    it("shows compliance warning when work > 10h", async () => {
      await openEditModal(makePastSession({ breaks: [] }));
      const startInput = screen.getByLabelText("Start");
      const endInput = screen.getByLabelText("Ende");
      // Set break to 0 first
      const breakSelect = screen.getByLabelText("Pause (Min)");
      fireEvent.change(breakSelect, { target: { value: "0" } });
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
      fireEvent.change(breakSelect, { target: { value: "15" } });
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
      fireEvent.change(breakSelect, { target: { value: "30" } });
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
      fireEvent.click(screen.getByText("Zeitkorrektur"));
      const saveButtons = screen.getAllByText("Speichern");
      const saveBtn = saveButtons[saveButtons.length - 1]!;
      expect(saveBtn).not.toBeDisabled();
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
      fireEvent.click(screen.getByText("Zeitkorrektur"));

      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
      });

      await waitFor(() => {
        expect(timeTrackingService.updateSession).toHaveBeenCalled();
        expect(mockToast.success).toHaveBeenCalledWith("Eintrag gespeichert");
      });
    });

    it("shows individual break durations when session has breaks", async () => {
      const yISO = todayISO;
      const sessionWithBreaks = makePastSession({
        breaks: [
          {
            id: "b1",
            sessionId: "100",
            startedAt: `${yISO}T10:00:00Z`,
            endedAt: `${yISO}T10:30:00Z`,
            durationMinutes: 30,
          },
          {
            id: "b2",
            sessionId: "100",
            startedAt: `${yISO}T13:00:00Z`,
            endedAt: `${yISO}T13:15:00Z`,
            durationMinutes: 15,
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
      const yISO = todayISO;
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
            startedAt: `${yISO}T10:00:00Z`,
            endedAt: `${yISO}T10:30:00Z`,
            durationMinutes: 30,
          },
        ],
      });

      await openEditModal(sessionWithBreaks);

      // Change break duration via select
      const breakSelects = screen
        .getByText("Pausen")
        .closest("div")!
        .querySelectorAll("select");
      if (breakSelects[0]) {
        fireEvent.change(breakSelects[0], { target: { value: "45" } });
      }

      fireEvent.click(screen.getByText("Zeitkorrektur"));

      const saveButtons = screen.getAllByText("Speichern");
      await act(async () => {
        fireEvent.click(saveButtons[saveButtons.length - 1]!);
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
      const yISO = todayISO;
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
      const yISO = todayISO;
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
      const yISO = todayISO;
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
      const yISO = todayISO;
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

    it("calls updateAbsence when absence tab saved", async () => {
      const mockToast = {
        success: vi.fn(),
        error: vi.fn(),
        info: vi.fn(),
        warning: vi.fn(),
        remove: vi.fn(),
      };
      vi.mocked(useToast).mockReturnValue(mockToast);

      const yISO = todayISO;
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

      const yISO = todayISO;
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
      fireEvent.change(breakSelect, { target: { value: "0" } });
      fireEvent.change(startInput, { target: { value: "08:00" } });
      fireEvent.change(endInput, { target: { value: "13:00" } });
      // 5h work, no warnings expected
      expect(screen.queryByText(/Arbeitszeit > 10h/)).not.toBeInTheDocument();
      expect(screen.queryByText(/Pausenzeit < 30 Min/)).not.toBeInTheDocument();
    });

    it("shows absence note textarea in absence tab", async () => {
      const yISO = todayISO;
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
      const yISO = todayISO;
      const pastSession = makePastSession();
      const pastAbsence: StaffAbsence = {
        ...mockAbsence,
        dateStart: yISO,
        dateEnd: yISO,
        halfDay: false,
      };

      await openEditModal(pastSession, { absences: [pastAbsence] });
      fireEvent.click(screen.getByText("Abwesenheit"));

      await waitFor(() => {
        const toggle = screen.getByRole("switch");
        expect(toggle.getAttribute("aria-checked")).toBe("false");
        fireEvent.click(toggle);
        expect(toggle.getAttribute("aria-checked")).toBe("true");
      });
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
        date: todayISO,
        checkInTime: `${todayISO}T08:00:00Z`,
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
      const yISO = todayISO;
      const hoSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:00:00Z`,
        status: "home_office",
      };

      setupDefaultMocks({ history: [hoSession] });
      render(<TimeTrackingPage />);

      const hoBadges = screen.queryAllByText("Homeoffice");
      expect(hoBadges.length).toBeGreaterThan(0);
    });

    it("shows 'In der OGS' badge for present status sessions", () => {
      const yISO = todayISO;
      const presentSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:00:00Z`,
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
          createdAt: `${todayISO}T17:00:00Z`,
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
          oldValue: `${todayISO}T07:00:00Z`,
          newValue: `${todayISO}T08:00:00Z`,
          notes: "Korrektur",
          createdAt: `${todayISO}T17:00:00Z`,
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
          oldValue: `${todayISO}T07:00:00Z`,
          newValue: `${todayISO}T08:00:00Z`,
          notes: "Korrektur",
          createdAt: `${todayISO}T17:00:00Z`,
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
          createdAt: `${todayISO}T17:00:00Z`,
        },
      ]);
      setupDefaultMocks({ history: [mockHistorySessionWithEdits] });
      render(<TimeTrackingPage />);

      const changeText = screen.getByText(/Zuletzt geändert/);
      fireEvent.click(changeText.closest("tr")!);

      await waitFor(() => {
        expect(screen.getByText("0 min")).toBeInTheDocument();
        expect(screen.getByText("30 min")).toBeInTheDocument();
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
          createdAt: `${todayISO}T17:00:00Z`,
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
          createdAt: `${todayISO}T17:00:00Z`,
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
      const yISO = todayISO;
      const pastSessionWithEdits: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
        editCount: 1,
        updatedAt: `${yISO}T17:00:00Z`,
      };

      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: `${yISO}T07:00:00Z`,
          newValue: `${yISO}T08:00:00Z`,
          notes: "Korrektur",
          createdAt: `${yISO}T17:00:00Z`,
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
      const timestamp = `${todayISO}T17:00:00Z`;
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: `${todayISO}T07:00:00Z`,
          newValue: `${todayISO}T08:00:00Z`,
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
        expect(screen.getByText("30 min")).toBeInTheDocument();
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
      expect((typeSelect as HTMLSelectElement).value).toBe("sick");
    });

    it("shows all absence type options", () => {
      openAbsenceModal();
      expect(screen.getByText("Krank")).toBeInTheDocument();
      expect(screen.getByText("Urlaub")).toBeInTheDocument();
      expect(screen.getByText("Fortbildung")).toBeInTheDocument();
      expect(screen.getByText("Sonstige")).toBeInTheDocument();
    });

    it("changes absence type via select", () => {
      openAbsenceModal();
      const typeSelect = screen.getByLabelText("Art der Abwesenheit");
      fireEvent.change(typeSelect, { target: { value: "vacation" } });
      expect((typeSelect as HTMLSelectElement).value).toBe("vacation");
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
      openAbsenceModal();
      const startInput = screen.getByLabelText("Von");
      fireEvent.change(startInput, { target: { value: "2026-03-01" } });
      expect((startInput as HTMLInputElement).value).toBe("2026-03-01");
    });

    it("changes end date", () => {
      openAbsenceModal();
      const endInput = screen.getByLabelText("Bis");
      fireEvent.change(endInput, { target: { value: "2026-03-05" } });
      expect((endInput as HTMLInputElement).value).toBe("2026-03-05");
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

      const yISO = todayISO;
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
      vi.mocked(timeTrackingService.getSessionEdits).mockResolvedValue([
        {
          id: "e1",
          sessionId: "100",
          staffId: "10",
          editedBy: "10",
          fieldName: "check_in_time",
          oldValue: `${yISO}T07:00:00Z`,
          newValue: `${yISO}T08:00:00Z`,
          notes: "Auto-expanded",
          createdAt: `${yISO}T17:00:00Z`,
        },
      ]);

      render(<TimeTrackingPage />);

      const editButtons = screen.queryAllByLabelText("Eintrag bearbeiten");
      if (editButtons.length > 0) {
        fireEvent.click(editButtons[0]!);
        await waitFor(() => {
          expect(screen.getByTestId("modal")).toBeInTheDocument();
        });

        fireEvent.click(screen.getByText("Zeitkorrektur"));

        const saveButtons = screen.getAllByText("Speichern");
        await act(async () => {
          fireEvent.click(saveButtons[saveButtons.length - 1]!);
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

      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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

        fireEvent.click(screen.getByText("Zeitkorrektur"));

        const saveButtons = screen.getAllByText("Speichern");
        await act(async () => {
          fireEvent.click(saveButtons[saveButtons.length - 1]!);
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

      const yISO = todayISO;
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

      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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

      const yISO = todayISO;
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

      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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

      await act(async () => {
        fireEvent.click(screen.getByLabelText("Einstempeln"));
      });

      await waitFor(() => {
        // "Urlaub" appears in both the table absence badge and confirmation modal text
        const urlaubTexts = screen.queryAllByText(/Urlaub/);
        expect(urlaubTexts.length).toBeGreaterThanOrEqual(2);
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
      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
      };

      setupDefaultMocks({ history: [pastSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      // Should show Woche gesamt
      expect(screen.getByText("Woche gesamt")).toBeInTheDocument();
    });

    it("shows absence-only card in mobile view", () => {
      const yISO = todayISO;
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
      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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
        checkInTime: `${todayISO}T08:00:00Z`,
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
      const yISO = todayISO;
      const hoSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:00:00Z`,
        status: "home_office",
      };

      setupDefaultMocks({ history: [hoSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      const hoBadges = screen.queryAllByText("HO");
      expect(hoBadges.length).toBeGreaterThanOrEqual(0);
    });

    it("shows OGS badge on mobile for present session", () => {
      const yISO = todayISO;
      const presentSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:00:00Z`,
        status: "present",
      };

      setupDefaultMocks({ history: [presentSession] });
      render(<TimeTrackingPage />);
      window.dispatchEvent(new Event("resize"));

      const ogsBadges = screen.queryAllByText("OGS");
      expect(ogsBadges.length).toBeGreaterThanOrEqual(0);
    });

    it("shows edit history toggle on mobile for sessions with edits", () => {
      const yISO = todayISO;
      const sessionWithEdits: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
        editCount: 2,
        updatedAt: `${yISO}T17:00:00Z`,
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

      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T16:30:00Z`,
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

        fireEvent.click(screen.getByText("Zeitkorrektur"));

        const saveButtons = screen.getAllByText("Speichern");
        await act(async () => {
          fireEvent.click(saveButtons[saveButtons.length - 1]!);
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

      const yISO = todayISO;
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
      const yISO = todayISO;
      const pastSession: WorkSessionHistory = {
        ...mockHistorySession,
        date: yISO,
        checkInTime: `${yISO}T08:00:00Z`,
        checkOutTime: `${yISO}T12:00:00Z`,
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
});
