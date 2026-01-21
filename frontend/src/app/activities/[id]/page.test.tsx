import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import ActivityDetailPage from "./page";

const mockPush = vi.fn();
const mockBack = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
    back: mockBack,
  }),
  useParams: () => ({
    id: "1",
  }),
}));

vi.mock("@/components/dashboard", () => ({
  PageHeader: ({
    title,
    backUrl,
  }: {
    title: string;
    backUrl: string;
  }) => (
    <div data-testid="page-header" data-backurl={backUrl}>
      {title}
    </div>
  ),
}));

const mockActivity = {
  id: "1",
  name: "Schachclub",
  category_name: "Sport",
  is_open_ags: true,
  max_participant: 10,
  participant_count: 5,
  supervisor_id: "sup-1",
  ag_category_id: "cat-1",
  created_at: new Date("2024-01-01"),
  updated_at: new Date("2024-01-15"),
  supervisors: [
    {
      id: "sup-1",
      staff_id: "staff-1",
      first_name: "Max",
      last_name: "Mustermann",
      is_primary: true,
    },
    {
      id: "sup-2",
      staff_id: "staff-2",
      first_name: "Anna",
      last_name: "Schmidt",
      is_primary: false,
    },
  ],
  times: [
    {
      id: "time-1",
      weekday: "1",
      timeframe_id: "tf-1",
      activity_id: "1",
      created_at: new Date("2024-01-01"),
      updated_at: new Date("2024-01-15"),
    },
  ],
};

const mockStudents = [
  {
    id: "student-1",
    student_id: "s-1",
    name: "Peter Müller",
    school_class: "3a",
    activity_id: "1",
    current_location: "Raum 101",
    created_at: new Date("2024-01-01"),
    updated_at: new Date("2024-01-15"),
  },
  {
    id: "student-2",
    student_id: "s-2",
    name: "Lisa Schmidt",
    school_class: "3b",
    activity_id: "1",
    current_location: "Raum 101",
    created_at: new Date("2024-01-01"),
    updated_at: new Date("2024-01-15"),
  },
];

const mockTimeframes = [
  {
    id: "tf-1",
    name: "Nachmittag",
    display_name: "Nachmittag",
    description: "Nachmittagszeitraum",
    start_time: "2024-01-01T14:00:00Z",
    end_time: "2024-01-01T16:00:00Z",
  },
];

vi.mock("@/lib/activity-api", () => ({
  fetchActivity: vi.fn(),
  getEnrolledStudents: vi.fn(),
  getTimeframes: vi.fn(),
}));

vi.mock("@/lib/activity-helpers", () => ({
  getActivityCategoryColor: vi.fn(() => "from-blue-500 to-blue-600"),
  getWeekdayFullName: vi.fn((weekday: number) => {
    const days = [
      "Sonntag",
      "Montag",
      "Dienstag",
      "Mittwoch",
      "Donnerstag",
      "Freitag",
      "Samstag",
    ];
    return days[weekday] ?? "Unbekannt";
  }),
}));

import {
  fetchActivity,
  getEnrolledStudents,
  getTimeframes,
} from "@/lib/activity-api";

describe("ActivityDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(fetchActivity).mockResolvedValue(mockActivity);
    vi.mocked(getEnrolledStudents).mockResolvedValue(mockStudents);
    vi.mocked(getTimeframes).mockResolvedValue(mockTimeframes);
  });

  it("renders loading state initially", () => {
    vi.mocked(fetchActivity).mockImplementation(
      // eslint-disable-next-line @typescript-eslint/no-empty-function
      () => new Promise(() => {}), // Never resolves
    );

    render(<ActivityDetailPage />);

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
  });

  it("renders activity details after loading", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      // Activity name appears multiple times (header and details)
      const activityNames = screen.getAllByText("Schachclub");
      expect(activityNames.length).toBeGreaterThan(0);
      expect(screen.getByText("Kategorie: Sport")).toBeInTheDocument();
    });
  });

  it("displays page header with correct title and back URL", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      const header = screen.getByTestId("page-header");
      expect(header).toHaveTextContent("Aktivitätsdetails");
      expect(header).toHaveAttribute("data-backurl", "/activities");
    });
  });

  it("displays activity supervisors", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Betreuer")).toBeInTheDocument();
      expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
      expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
      expect(screen.getByText("Hauptbetreuer")).toBeInTheDocument();
    });
  });

  it("displays enrolled students", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Teilnehmende Schüler")).toBeInTheDocument();
      expect(screen.getByText("Peter Müller")).toBeInTheDocument();
      expect(screen.getByText("Lisa Schmidt")).toBeInTheDocument();
      expect(screen.getByText("Klasse: 3a")).toBeInTheDocument();
    });
  });

  it("displays schedule with timeframes", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Zeitplan")).toBeInTheDocument();
      expect(screen.getByText("Montag")).toBeInTheDocument();
    });
  });

  it("displays activity status badge for open activity", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      // "Offen" appears in header ("Offen für Teilnahme") and as badge
      expect(screen.getByText("Offen für Teilnahme")).toBeInTheDocument();
      // Check for the status badge (text is "Offen" in badge, "Offen für Teilnahme" in header)
      const badges = screen.getAllByText(/Offen/);
      expect(badges.length).toBeGreaterThanOrEqual(1);
    });
  });

  it("displays closed status badge when activity is not open", async () => {
    vi.mocked(fetchActivity).mockResolvedValue({
      ...mockActivity,
      is_open_ags: false,
    });

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Geschlossen")).toBeInTheDocument();
    });
  });

  it("displays participant count", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Max. Teilnehmer")).toBeInTheDocument();
      expect(screen.getByText("10")).toBeInTheDocument();
      expect(screen.getByText("Aktuelle Teilnehmer")).toBeInTheDocument();
      expect(screen.getByText("5")).toBeInTheDocument();
    });
  });

  it("shows error state when fetch fails", async () => {
    vi.mocked(fetchActivity).mockRejectedValue(new Error("Fetch failed"));

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Fehler")).toBeInTheDocument();
      expect(
        screen.getByText("Fehler beim Laden der Aktivitätsdaten."),
      ).toBeInTheDocument();
    });
  });

  it("handles back button click in error state", async () => {
    vi.mocked(fetchActivity).mockRejectedValue(new Error("Fetch failed"));

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Zurück")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zurück"));
    expect(mockBack).toHaveBeenCalled();
  });

  it("shows not found state when activity is null", async () => {
    vi.mocked(fetchActivity).mockResolvedValue(null as never);

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Aktivität nicht gefunden")).toBeInTheDocument();
    });
  });

  it("handles back to overview button in not found state", async () => {
    vi.mocked(fetchActivity).mockResolvedValue(null as never);

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Zurück zur Übersicht")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zurück zur Übersicht"));
    expect(mockPush).toHaveBeenCalledWith("/activities");
  });

  it("displays empty students message when no students enrolled", async () => {
    vi.mocked(getEnrolledStudents).mockResolvedValue([]);

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Schüler eingeschrieben")).toBeInTheDocument();
    });
  });

  it("displays no supervisors message when none assigned", async () => {
    vi.mocked(fetchActivity).mockResolvedValue({
      ...mockActivity,
      supervisors: [],
    });

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Keine Betreuer zugewiesen.")).toBeInTheDocument();
    });
  });

  it("displays no schedule message when no times defined", async () => {
    vi.mocked(fetchActivity).mockResolvedValue({
      ...mockActivity,
      times: [],
    });

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Kein Zeitplan definiert.")).toBeInTheDocument();
    });
  });

  it("navigates to add students page when button clicked", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Schüler hinzufügen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Schüler hinzufügen"));
    expect(mockPush).toHaveBeenCalledWith("/database/activities/1/add-students");
  });

  it("navigates to student detail when student is clicked", async () => {
    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Peter Müller")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Peter Müller"));
    expect(mockPush).toHaveBeenCalledWith("/students/s-1?from=/activities/1");
  });

  it("handles enrolled students fetch failure gracefully", async () => {
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(getEnrolledStudents).mockRejectedValue(
      new Error("Students fetch failed"),
    );

    render(<ActivityDetailPage />);

    await waitFor(() => {
      // Activity should still render (name appears multiple times)
      const activityNames = screen.getAllByText("Schachclub");
      expect(activityNames.length).toBeGreaterThan(0);
      // But with no students
      expect(screen.getByText("Keine Schüler eingeschrieben")).toBeInTheDocument();
    });

    consoleSpy.mockRestore();
  });

  it("handles timeframes fetch failure gracefully", async () => {
    // eslint-disable-next-line @typescript-eslint/no-empty-function
    const consoleSpy = vi.spyOn(console, "error").mockImplementation(() => {});
    vi.mocked(getTimeframes).mockRejectedValue(
      new Error("Timeframes fetch failed"),
    );

    render(<ActivityDetailPage />);

    await waitFor(() => {
      // Activity should still render (name appears multiple times)
      const activityNames = screen.getAllByText("Schachclub");
      expect(activityNames.length).toBeGreaterThan(0);
      // Schedule section should still exist
      expect(screen.getByText("Zeitplan")).toBeInTheDocument();
    });

    consoleSpy.mockRestore();
  });

  it("displays category name or default when not assigned", async () => {
    vi.mocked(fetchActivity).mockResolvedValue({
      ...mockActivity,
      category_name: undefined,
    });

    render(<ActivityDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Nicht zugewiesen")).toBeInTheDocument();
    });
  });
});
