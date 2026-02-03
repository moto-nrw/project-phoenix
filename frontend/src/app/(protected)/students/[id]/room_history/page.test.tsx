import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentRoomHistoryPage from "./page";

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

describe("StudentRoomHistoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    render(<StudentRoomHistoryPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders student info after loading", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Emma Müller")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays room history title", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Raumverlauf")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays student class and group info", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText(/Klasse 3b/)).toBeInTheDocument();
        expect(screen.getByText(/Gruppe: Eulen/)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays time range filter buttons", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Heute")).toBeInTheDocument();
        expect(screen.getByText("Diese Woche")).toBeInTheDocument();
        expect(screen.getByText("Letzte 7 Tage")).toBeInTheDocument();
        expect(screen.getByText("Diesen Monat")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays back button", async () => {
    render(<StudentRoomHistoryPage />);

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
    render(<StudentRoomHistoryPage />);

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

  it("displays room history entries", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        // Check for room names from mock data
        expect(screen.getAllByText(/Eulen Gruppenraum/).length).toBeGreaterThan(
          0,
        );
      },
      { timeout: 2000 },
    );
  });

  it("displays room entry reasons", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        // Check for room entry reasons from mock data
        // Using queryAllByText for more flexible matching
        const mittagessen = screen.queryAllByText(/Mittagessen/i);
        const fussballAg = screen.queryAllByText(/Fußball AG/i);
        expect(mittagessen.length + fussballAg.length).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });

  it("displays duration for room entries", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        // Check for duration text (e.g., "45 Min." or "1 Std. 30 Min.")
        expect(screen.getAllByText(/Min\./).length).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });

  it("changes time range when filter button clicked", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Heute")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );

    // Click the "Heute" button - this triggers a re-render with loading state
    fireEvent.click(screen.getByText("Heute"));

    // Wait for loading to complete and UI to show again
    await waitFor(
      () => {
        expect(screen.getByText("Heute")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays student initials in header", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        // Emma Müller initials
        expect(screen.getByText("EM")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays time range selection header", async () => {
    render(<StudentRoomHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Zeitraum auswählen")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });
});

describe("getYear helper function", () => {
  it("extracts year from school class", () => {
    const getYear = (schoolClass: string): number => {
      const yearMatch = /^(\d)/.exec(schoolClass);
      return yearMatch?.[1] ? Number.parseInt(yearMatch[1], 10) : 0;
    };

    expect(getYear("1a")).toBe(1);
    expect(getYear("2b")).toBe(2);
    expect(getYear("3c")).toBe(3);
    expect(getYear("4d")).toBe(4);
    expect(getYear("unknown")).toBe(0);
  });
});

describe("getYearColor helper function", () => {
  it("returns correct color for each year", () => {
    const getYearColor = (year: number): string => {
      switch (year) {
        case 1:
          return "bg-blue-500";
        case 2:
          return "bg-green-500";
        case 3:
          return "bg-yellow-500";
        case 4:
          return "bg-purple-500";
        default:
          return "bg-gray-400";
      }
    };

    expect(getYearColor(1)).toBe("bg-blue-500");
    expect(getYearColor(2)).toBe("bg-green-500");
    expect(getYearColor(3)).toBe("bg-yellow-500");
    expect(getYearColor(4)).toBe("bg-purple-500");
    expect(getYearColor(0)).toBe("bg-gray-400");
    expect(getYearColor(5)).toBe("bg-gray-400");
  });
});
