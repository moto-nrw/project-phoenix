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

// Helper to build a mock student response
function mockStudentResponse() {
  return {
    ok: true,
    status: 200,
    json: async () => ({
      data: {
        id: "42",
        first_name: "Test",
        second_name: "Student",
        name: "Test Student",
        school_class: "2a",
        group_name: "Sterne",
      },
    }),
  };
}

// Helper to build a mock attendance-history response
function mockAttendanceHistoryResponse(days: unknown[] = []) {
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
      },
    }),
  };
}

function mockFeatureDisabledResponse() {
  return {
    ok: false,
    status: 403,
    json: async () => ({ error: "feature_disabled" }),
  };
}

function mockNotGroupSupervisorResponse() {
  return {
    ok: false,
    status: 403,
    json: async () => ({ error: "not_group_supervisor" }),
  };
}

describe("StudentRoomHistoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    mockFetch.mockResolvedValue(mockStudentResponse());
    render(<StudentRoomHistoryPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders student profile header after loading", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse());
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Test Student")).toBeInTheDocument();
    });
    expect(screen.getByText("Klasse 2a")).toBeInTheDocument();
    expect(screen.getByText(/Gruppe: Sterne/)).toBeInTheDocument();
  });

  it("displays page title", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse());
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Anwesenheitsprotokoll")).toBeInTheDocument();
    });
  });

  it("displays back button that navigates to student profile", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse());
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Zurück zum Schülerprofil")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zurück zum Schülerprofil"));
    expect(mockPush).toHaveBeenCalledWith(
      "/test-tenant/students/42?from=/students/search",
    );
  });

  it("shows feature disabled message on 403", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockFeatureDisabledResponse());
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Diese Funktion ist für Ihre Schule deaktiviert/),
      ).toBeInTheDocument();
    });
  });

  it("shows not_group_supervisor error", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockNotGroupSupervisorResponse());
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/nur.*Anwesenheitsprotokoll.*betreuten Gruppen/i),
      ).toBeInTheDocument();
    });
  });

  it("shows empty state when no attendance data", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse([]));
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText(
          /Keine Anwesenheitsdaten für den ausgewählten Zeitraum/,
        ),
      ).toBeInTheDocument();
    });
  });

  it("renders attendance data with check-in/check-out", async () => {
    const day = {
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
      ],
    };

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse([day]));
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppenraum A")).toBeInTheDocument();
    });
    // Check attendance duration
    expect(screen.getByText("7 h 30 min")).toBeInTheDocument();
  });

  it("shows room detail unavailable note for old days", async () => {
    const day = {
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

    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse([day]));
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(
        screen.getByText(/Raumdetails.*nicht mehr verfügbar/),
      ).toBeInTheDocument();
    });
  });

  it("displays caps info banner", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse([]));
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText(/letzten 30 Tage/)).toBeInTheDocument();
      expect(screen.getByText(/letzten 7 Tage/)).toBeInTheDocument();
    });
  });

  it("displays student initials in header", async () => {
    mockFetch.mockImplementation((url: string) => {
      if (url.includes("/attendance-history")) {
        return Promise.resolve(mockAttendanceHistoryResponse());
      }
      return Promise.resolve(mockStudentResponse());
    });

    render(<StudentRoomHistoryPage />);

    await waitFor(() => {
      expect(screen.getByText("TS")).toBeInTheDocument();
    });
  });
});
