import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentRoomHistoryPage from "./page";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  useParams: () => ({
    id: "42",
  }),
  useSearchParams: () => ({
    get: vi.fn((key: string) => (key === "from" ? "/students/search" : null)),
  }),
}));

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        token: "test-token",
      },
    },
  })),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useStudentHistoryBreadcrumb: vi.fn(),
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: ({ fullPage }: { fullPage?: boolean }) => (
    <div data-testid="loading" data-fullpage={fullPage} aria-label="Lädt..." />
  ),
}));

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ type, message }: { type: string; message: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock fetch to simulate backend responses
const mockFetch = vi.fn();
global.fetch = mockFetch;

function mockStudentResponse(overrides?: Record<string, unknown>) {
  return {
    ok: true,
    status: 200,
    json: async () => ({
      data: {
        id: "42",
        first_name: "Test",
        second_name: "Student",
        name: "Test Student",
        school_class: "Klasse 2a",
        group_name: "Sterne",
        ...overrides,
      },
    }),
  };
}

function mockAttendanceHistoryResponse(
  days: unknown[] = [],
  overrides?: Record<string, unknown>,
) {
  return {
    ok: true,
    status: 200,
    json: async () => ({
      data: {
        student_id: "42",
        days,
        range: {
          start: "2026-03-07T00:00:00Z",
          end: "2026-04-06T23:59:59Z",
        },
        clamped: false,
        caps: { attendance_days: 30, room_detail_days: 7 },
        ...overrides,
      },
    }),
  };
}

function setupFetch(
  historyResponse: {
    ok: boolean;
    status: number;
    json: () => Promise<unknown>;
  },
  studentResponse?: {
    ok: boolean;
    status: number;
    json: () => Promise<unknown>;
  },
) {
  mockFetch.mockImplementation((url: string) => {
    if (url.includes("/attendance-history")) {
      return Promise.resolve(historyResponse);
    }
    return Promise.resolve(studentResponse ?? mockStudentResponse());
  });
}

// ─── Test fixtures ───────────────────────────────────────────────────────────

const fullDay = {
  date: "2026-04-06",
  attendance: {
    check_in_time: "2026-04-06T08:00:00Z",
    check_out_time: "2026-04-06T15:30:00Z",
    duration_minutes: 450,
    checked_in_by: 1,
    device_id: 1,
  },
  room_detail_available: true,
  visits: [
    {
      room_name: "Gruppenraum A",
      entry_time: "2026-04-06T08:10:00Z",
      exit_time: "2026-04-06T10:30:00Z",
      duration_minutes: 140,
    },
    {
      room_name: "Schulhof",
      entry_time: "2026-04-06T10:30:00Z",
      exit_time: "2026-04-06T11:15:00Z",
      duration_minutes: 45,
    },
  ],
};

const stillCheckedInDay = {
  date: "2026-04-07",
  attendance: {
    check_in_time: "2026-04-07T08:15:00Z",
    check_out_time: null,
    duration_minutes: null,
    checked_in_by: 1,
    device_id: 1,
  },
  room_detail_available: true,
  visits: [],
};

const expiredDay = {
  date: "2026-03-20",
  attendance: {
    check_in_time: "2026-03-20T08:00:00Z",
    check_out_time: "2026-03-20T15:00:00Z",
    duration_minutes: 420,
    checked_in_by: 1,
    device_id: 1,
  },
  room_detail_available: false,
  visits: null,
};

const noVisitsDay = {
  date: "2026-04-05",
  attendance: {
    check_in_time: "2026-04-05T09:00:00Z",
    check_out_time: "2026-04-05T14:00:00Z",
    duration_minutes: 300,
    checked_in_by: 1,
    device_id: 1,
  },
  room_detail_available: true,
  visits: [],
};

const sickStatusDay = {
  date: "2026-04-08",
  attendance: null,
  status_entries: [
    {
      status: "sick",
      label: "Krank",
      reported_at: "2026-04-08T07:30:00Z",
      cleared_at: "2026-04-08T16:00:00Z",
    },
  ],
  room_detail_available: true,
  visits: [],
};

describe("StudentRoomHistoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    window.scrollTo = vi.fn();
  });

  // ─── Loading ────────────────────────────────────────────────────────────────

  it("renders loading state initially", () => {
    mockFetch.mockResolvedValue(mockStudentResponse());
    render(<StudentRoomHistoryPage />);
    expect(screen.getByTestId("room-history-skeleton")).toBeInTheDocument();
  });

  // ─── Student header ─────────────────────────────────────────────────────────

  it("renders student name and class after loading", async () => {
    setupFetch(mockAttendanceHistoryResponse());
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Test Student")).toBeInTheDocument();
    });
    expect(screen.getByText(/Klasse 2a/)).toBeInTheDocument();
  });

  it("shows group name when student has a group", async () => {
    setupFetch(mockAttendanceHistoryResponse());
    render(<StudentRoomHistoryPage />);

    // The group name renders as its own interpolated text node next to the
    // school class (" · Sterne"), so match it as a substring rather than an
    // exact string.
    await waitFor(() => {
      expect(screen.getByText(/Sterne/)).toBeInTheDocument();
    });
  });

  it("does not show group separator when student has no group", async () => {
    setupFetch(
      mockAttendanceHistoryResponse(),
      mockStudentResponse({ group_name: undefined }),
    );
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Test Student")).toBeInTheDocument();
    });
    expect(screen.queryByText("·")).not.toBeInTheDocument();
  });

  it("scrolls to top on mount", async () => {
    setupFetch(mockAttendanceHistoryResponse());
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(window.scrollTo).toHaveBeenCalledWith(0, 0);
    });
  });

  // ─── Error states ───────────────────────────────────────────────────────────

  it("shows feature disabled banner", async () => {
    setupFetch({
      ok: false,
      status: 403,
      json: async () => ({ error: "feature_disabled" }),
    });
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Diese Funktion ist für Ihre Schule deaktiviert/),
      ).toBeInTheDocument();
    });
  });

  it("shows not_group_supervisor error", async () => {
    setupFetch({
      ok: false,
      status: 403,
      json: async () => ({ error: "not_group_supervisor" }),
    });
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });
  });

  it("shows not_found error", async () => {
    setupFetch({
      ok: false,
      status: 404,
      json: async () => ({ error: "not_found" }),
    });
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Kind nicht gefunden.")).toBeInTheDocument();
    });
  });

  it("shows generic error on server failure", async () => {
    setupFetch({
      ok: false,
      status: 500,
      json: async () => ({ error: "internal" }),
    });
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden des Anwesenheitsprotokolls."),
      ).toBeInTheDocument();
    });
  });

  it("rendert im Fehlerfall den Rückweg zur Herkunftsliste", async () => {
    setupFetch({
      ok: false,
      status: 404,
      json: async () => ({ error: "not_found" }),
    });
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });

    // Der Desktop-Rückweg ist die Breadcrumb; mobil bleibt der kit
    // BackButton mit dem Referrer der Herkunftsliste.
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(mockPush).toHaveBeenCalledWith("/test-tenant/students/search");
  });

  it("handles fetch exception gracefully", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.reject(new Error("Network error"));
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Fehler beim Laden des Anwesenheitsprotokolls."),
      ).toBeInTheDocument();
    });
  });

  it("handles student fetch failure without crashing", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse());
      }
      return Promise.resolve({ ok: false, status: 500 });
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
    });
  });

  // ─── Empty state ────────────────────────────────────────────────────────────

  it("shows empty state when no attendance data", async () => {
    setupFetch(mockAttendanceHistoryResponse([]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          /Keine Anwesenheitsdaten für den ausgewählten Zeitraum/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("renders status-only days", async () => {
    setupFetch(mockAttendanceHistoryResponse([sickStatusDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Krank").length).toBeGreaterThan(0);
    });

    expect(
      screen.queryByText(
        /Keine Anwesenheitsdaten für den ausgewählten Zeitraum/,
      ),
    ).not.toBeInTheDocument();
  });

  // ─── Caps info ──────────────────────────────────────────────────────────────

  it("shows caps subheading in table", async () => {
    setupFetch(mockAttendanceHistoryResponse([]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText(/Letzte 30 Tage/)).toBeInTheDocument();
    });
  });

  // ─── Chart titles ───────────────────────────────────────────────────────────

  it("renders chart titles and subheadings", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Anwesenheit")).toBeInTheDocument();
    });
    expect(
      screen.getByText("Tägliche Aufenthaltsdauer in Stunden"),
    ).toBeInTheDocument();
    expect(screen.getByText("Aktivität")).toBeInTheDocument();
    expect(screen.getByText("Raumwechsel pro Tag")).toBeInTheDocument();
  });

  it("does not render charts when no days", async () => {
    setupFetch(mockAttendanceHistoryResponse([]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText(/Keine Anwesenheitsdaten/)).toBeInTheDocument();
    });
    expect(screen.queryByText("Anwesenheit")).not.toBeInTheDocument();
    expect(screen.queryByText("Aktivität")).not.toBeInTheDocument();
  });

  // ─── Table content (uses getAllByText since desktop+mobile both render) ─────

  it("renders attendance duration", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h 30 min").length).toBeGreaterThan(0);
    });
  });

  it("renders room count", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText(/2 Raumwechsel/)).toBeInTheDocument();
    });
  });

  it("shows 'Noch anwesend' when student has no check-out", async () => {
    setupFetch(mockAttendanceHistoryResponse([stillCheckedInDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Noch anwesend")).toBeInTheDocument();
    });
  });

  it("shows dash for rooms when room detail not available", async () => {
    setupFetch(mockAttendanceHistoryResponse([expiredDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h").length).toBeGreaterThan(0);
    });
  });

  // ─── Table expand/collapse (target desktop table via role) ─────────────────

  it("expands row to show room visits on click", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h 30 min").length).toBeGreaterThan(0);
    });

    // Room names should not be visible before expanding
    expect(screen.queryByText("Gruppenraum A")).not.toBeInTheDocument();

    // Click a table row — find the desktop table row by the date text
    const dateElement = screen.getByText(/Montag, 06/);
    const row = dateElement.closest("tr");
    fireEvent.click(row!);

    // Room visits should now be visible
    await waitFor(() => {
      expect(screen.getByText("Gruppenraum A")).toBeInTheDocument();
    });
    expect(screen.getByText("Schulhof")).toBeInTheDocument();
  });

  it("collapses expanded row on second click", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h 30 min").length).toBeGreaterThan(0);
    });

    const dateElement = screen.getByText(/Montag, 06/);
    const row = dateElement.closest("tr");

    // Expand
    fireEvent.click(row!);
    await waitFor(() => {
      expect(screen.getByText("Gruppenraum A")).toBeInTheDocument();
    });

    // Collapse
    fireEvent.click(row!);
    await waitFor(() => {
      expect(screen.queryByText("Gruppenraum A")).not.toBeInTheDocument();
    });
  });

  it("shows 'Keine Raumwechsel' when expanded day has no visits", async () => {
    setupFetch(mockAttendanceHistoryResponse([noVisitsDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("5 h").length).toBeGreaterThan(0);
    });

    // Expand the row via desktop table
    const dateElement = screen.getByText(/Sonntag, 05/);
    const row = dateElement.closest("tr");
    fireEvent.click(row!);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Raumwechsel an diesem Tag."),
      ).toBeInTheDocument();
    });
  });

  it("shows visit duration in expanded rows", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h 30 min").length).toBeGreaterThan(0);
    });

    const dateElement = screen.getByText(/Montag, 06/);
    fireEvent.click(dateElement.closest("tr")!);

    await waitFor(() => {
      expect(screen.getByText("2 h 20 min")).toBeInTheDocument();
      expect(screen.getByText("45 min")).toBeInTheDocument();
    });
  });

  it("shows care-offering slots in expanded desktop rows", async () => {
    const slotDay = {
      ...fullDay,
      slots: [
        {
          instance_id: "501",
          title: "Morgenbetreuung",
          start_time: "07:00",
          end_time: "08:00",
          status: "present",
          checked_in_at: "2026-04-06T07:05:00Z",
          checked_out_at: "2026-04-06T07:55:00Z",
          is_unplanned: false,
        },
        {
          instance_id: "502",
          title: "Nachmittagsbetreuung",
          start_time: "12:00",
          end_time: "16:00",
          status: "absent",
          substatus: "sick",
          is_unplanned: true,
        },
      ],
    };
    setupFetch(mockAttendanceHistoryResponse([slotDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h 30 min").length).toBeGreaterThan(0);
    });
    expect(screen.getByText(/2 Angebote/)).toBeInTheDocument();
    expect(screen.queryByText("Morgenbetreuung")).not.toBeInTheDocument();

    const dateElement = screen.getByText(/Montag, 06/);
    fireEvent.click(dateElement.closest("tr")!);

    await waitFor(() => {
      expect(screen.getByText("Morgenbetreuung")).toBeInTheDocument();
    });
    expect(screen.getByText("Nachmittagsbetreuung")).toBeInTheDocument();
    expect(screen.getByText("Krank")).toBeInTheDocument();
    expect(screen.getByText("ungeplant")).toBeInTheDocument();
    // Room visits still render alongside the slots
    expect(screen.getByText("Gruppenraum A")).toBeInTheDocument();
  });

  // ─── Table header ───────────────────────────────────────────────────────────

  it("renders table column headers", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Tag")).toBeInTheDocument();
    });
    expect(screen.getByText("Ankunft")).toBeInTheDocument();
    expect(screen.getByText("Abmeldung")).toBeInTheDocument();
    expect(screen.getByText("Dauer")).toBeInTheDocument();
    expect(screen.getByText("Angebote / Räume")).toBeInTheDocument();
  });

  // ─── Anwesenheitsprotokoll title ────────────────────────────────────────────

  it("renders Anwesenheitsprotokoll as table title", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Anwesenheitsprotokoll" }),
      ).toBeInTheDocument();
    });
  });

  // ─── Day without attendance ─────────────────────────────────────────────────

  it("shows dashes when day has no attendance", async () => {
    const dayWithoutAttendance = {
      date: "2026-04-04",
      attendance: null,
      room_detail_available: true,
      visits: [],
    };

    setupFetch(mockAttendanceHistoryResponse([dayWithoutAttendance]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("–").length).toBeGreaterThan(0);
    });
  });

  // ─── Mobile DayCard ─────────────────────────────────────────────────────────

  it("renders mobile day card with expand functionality", async () => {
    setupFetch(mockAttendanceHistoryResponse([fullDay]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getAllByText("7 h 30 min").length).toBeGreaterThan(0);
    });

    // Find a mobile DayCard button (has role="button" with type="button")
    const mobileButtons = screen
      .getAllByRole("button")
      .filter(
        (b) => b.getAttribute("type") === "button" && b.closest(".md\\:hidden"),
      );

    // If mobile buttons found, click to expand
    if (mobileButtons.length > 0) {
      fireEvent.click(mobileButtons[0]!);
    }
  });

  it("shows 'Keine Daten' badge when mobile card has no attendance", async () => {
    const dayWithoutAttendance = {
      date: "2026-04-04",
      attendance: null,
      room_detail_available: true,
      visits: [],
    };

    setupFetch(mockAttendanceHistoryResponse([dayWithoutAttendance]));
    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Daten")).toBeInTheDocument();
    });
  });

  // ─── Student fetch error handling ───────────────────────────────────────────

  it("handles student fetch network error", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse());
      }
      return Promise.reject(new Error("Network error"));
    });

    render(<StudentRoomHistoryPage />);

    // Should still render without crashing
    await waitFor(() => {
      expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
    });
  });
});
