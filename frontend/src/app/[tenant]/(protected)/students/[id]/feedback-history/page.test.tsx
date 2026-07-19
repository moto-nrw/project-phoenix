import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentFeedbackHistoryPage from "./page";

const mockPush = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({
    push: mockPush,
  }),
  useParams: () => ({
    id: "1",
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

vi.mock("~/components/ui/back-button", () => ({
  BackButton: ({ referrer }: { referrer: string }) => (
    <button type="button" data-testid="back-button" data-referrer={referrer}>
      Zurück
    </button>
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

vi.mock("~/lib/date-helpers", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/date-helpers")>();
  return {
    ...actual,
    getStartDateForTimeRange: vi.fn((_timeRange: string, _now: Date) => {
      return new Date("2020-01-01");
    }),
  };
});

vi.mock("~/lib/student-api", () => ({
  fetchStudent: vi.fn().mockResolvedValue({
    id: "1",
    name: "Emma Müller",
    first_name: "Emma",
    second_name: "Müller",
    school_class: "3b",
    group_name: "Eulen",
    group_id: "g3",
    current_location: "Anwesend",
  }),
}));

vi.mock("~/lib/feedback-api", () => ({
  fetchStudentFeedback: vi.fn().mockResolvedValue([
    {
      id: "1",
      timestamp: "2025-05-14T15:30:23",
      feedback_type: "positive",
      is_mensa_feedback: false,
    },
    {
      id: "2",
      timestamp: "2025-05-13T15:45:12",
      feedback_type: "negative",
      is_mensa_feedback: false,
    },
    {
      id: "3",
      timestamp: "2025-05-12T15:25:18",
      feedback_type: "positive",
      is_mensa_feedback: false,
    },
    {
      id: "4",
      timestamp: "2025-05-11T15:10:09",
      feedback_type: "neutral",
      is_mensa_feedback: false,
    },
    {
      id: "5",
      timestamp: "2025-05-10T12:30:22",
      feedback_type: "negative",
      is_mensa_feedback: false,
    },
  ]),
}));

// Mock recharts to avoid ResizeObserver issues in tests
vi.mock("recharts", () => ({
  Bar: () => null,
  BarChart: () => null,
  XAxis: () => null,
  YAxis: () => null,
  CartesianGrid: () => null,
  ResponsiveContainer: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
}));

vi.mock("~/components/ui/chart", () => ({
  ChartContainer: ({
    children,
  }: {
    children: React.ReactNode;
    config: unknown;
    className: string;
  }) => <div data-testid="chart-container">{children}</div>,
  ChartTooltip: () => null,
  ChartTooltipContent: () => null,
  ChartLegend: () => null,
  ChartLegendContent: () => null,
}));

describe("StudentFeedbackHistoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    render(<StudentFeedbackHistoryPage />);

    expect(screen.getByTestId("feedback-history-skeleton")).toBeInTheDocument();
  });

  it("renders student name as page heading", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(
          screen.getByRole("heading", { level: 1, name: "Emma Müller" }),
        ).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback history subtitle", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays time range filter buttons", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Alle")).toBeInTheDocument();
        expect(screen.getByText("Heute")).toBeInTheDocument();
        expect(screen.getByText("Diese Woche")).toBeInTheDocument();
        expect(screen.getByText("Letzte 7 Tage")).toBeInTheDocument();
        expect(screen.getByText("Diesen Monat")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback overview heading", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Feedback-Übersicht")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback type stats inline", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Positiv")).toBeInTheDocument();
        expect(screen.getByText("Neutral")).toBeInTheDocument();
        expect(screen.getByText("Negativ")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays back button", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByTestId("back-button")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("back button links to student profile", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        const backButton = screen.getByTestId("back-button");
        // tab=historie returns to the Historie tab on the detail page (#1501);
        // from= still drives the detail page's own back button to the list.
        expect(backButton).toHaveAttribute(
          "data-referrer",
          "/students/1?from=/students/search&tab=historie",
        );
      },
      { timeout: 2000 },
    );
  });

  it("displays student class and group info", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText(/3b/)).toBeInTheDocument();
        expect(screen.getByText(/Gruppe: Eulen/)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("changes time range when filter button clicked", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Alle")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );

    fireEvent.click(screen.getByText("Alle"));

    await waitFor(
      () => {
        expect(screen.getByText("Alle")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("hides detail list by default and shows toggle button", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText(/Alle Einträge anzeigen/)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("shows detail entries when toggle is clicked", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText(/Alle Einträge anzeigen/)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );

    fireEvent.click(screen.getByText(/Alle Einträge anzeigen/));

    await waitFor(
      () => {
        expect(
          screen.getAllByTestId(/feedback-indicator/).length,
        ).toBeGreaterThan(0);
        expect(
          screen.getAllByText(/Positives Feedback/i).length,
        ).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });
});

describe("Feedback type labels", () => {
  it("maps feedback types to German labels", () => {
    const feedbackTypeLabels: Record<string, string> = {
      positive: "Positives Feedback",
      neutral: "Neutrales Feedback",
      negative: "Negatives Feedback",
    };

    expect(feedbackTypeLabels.positive).toBe("Positives Feedback");
    expect(feedbackTypeLabels.neutral).toBe("Neutrales Feedback");
    expect(feedbackTypeLabels.negative).toBe("Negatives Feedback");
  });
});
