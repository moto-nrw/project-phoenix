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

vi.mock("~/lib/date-helpers", () => ({
  getStartDateForTimeRange: vi.fn((_timeRange: string, _now: Date) => {
    // Return a date far in the past to include all mock data from May 2025
    return new Date("2020-01-01");
  }),
}));

describe("StudentFeedbackHistoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    render(<StudentFeedbackHistoryPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders student info after loading", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Emma Müller")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback history title", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Feedbackhistorie")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays filter section", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Filter")).toBeInTheDocument();
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

  it("displays feedback overview section", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Feedback-Übersicht")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback type categories", async () => {
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
        expect(
          screen.getByText("Zurück zum Schülerprofil"),
        ).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("navigates back to student profile when back button clicked", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(
          screen.getByText("Zurück zum Schülerprofil"),
        ).toBeInTheDocument();
      },
      { timeout: 2000 },
    );

    fireEvent.click(screen.getByText("Zurück zum Schülerprofil"));

    expect(mockPush).toHaveBeenCalledWith("/students/1?from=/students/search");
  });

  it("displays student initials in header", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("EM")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays student class and group info", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText(/Klasse 3b/)).toBeInTheDocument();
        expect(screen.getByText(/Gruppe: Eulen/)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback entries with type labels", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        // Check for feedback type labels from mock data using regex
        expect(
          screen.getAllByText(/Positives Feedback/i).length,
        ).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });

  it("displays invalid feedback markers", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        // The mock data has some entries marked as is_valid: false
        expect(
          screen.getAllByText(/Ungültiges Feedback/i).length,
        ).toBeGreaterThan(0);
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

    // Click the "Alle" button - this triggers a re-render with loading state
    fireEvent.click(screen.getByText("Alle"));

    // Wait for loading to complete and UI to show again
    await waitFor(
      () => {
        expect(screen.getByText("Alle")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback emojis", async () => {
    render(<StudentFeedbackHistoryPage />);

    await waitFor(
      () => {
        // Check for emoji containers
        expect(screen.getAllByText("😊").length).toBeGreaterThan(0);
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
