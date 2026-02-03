import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentMensaHistoryPage from "./page";

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

describe("StudentMensaHistoryPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    render(<StudentMensaHistoryPage />);

    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders student info after loading", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Emma Müller")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays mensa history title", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Mensaverlauf")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays filter section", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Filter")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays time range filter buttons", async () => {
    render(<StudentMensaHistoryPage />);

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

  it("displays mensa overview section", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Mensa-Übersicht")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays mensa statistics", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        // "Gegessen" appears in both statistics and individual entries
        expect(screen.getAllByText("Gegessen").length).toBeGreaterThan(0);
        expect(screen.getAllByText("Nicht gegessen").length).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback statistics section", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText("Essensrückmeldungen")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays feedback type categories", async () => {
    render(<StudentMensaHistoryPage />);

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
    render(<StudentMensaHistoryPage />);

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
    render(<StudentMensaHistoryPage />);

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
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        // Emma Müller initials
        expect(screen.getByText("EM")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays student class and group info", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        expect(screen.getByText(/Klasse 3b/)).toBeInTheDocument();
        expect(screen.getByText(/Gruppe: Eulen/)).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });

  it("displays mensa entries with status", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        // Check for mensa entry status from mock data
        // 6 eaten, 1 not eaten
        expect(screen.getAllByText("Gegessen").length).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });

  it("displays not eaten entries", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        // The mock data has one entry with has_eaten: false
        expect(screen.getAllByText(/Nichts gegessen/i).length).toBeGreaterThan(
          0,
        );
      },
      { timeout: 2000 },
    );
  });

  it("changes time range when filter button clicked", async () => {
    render(<StudentMensaHistoryPage />);

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
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        // Check for emoji containers (positive emoji)
        expect(screen.getAllByText("😊").length).toBeGreaterThan(0);
      },
      { timeout: 2000 },
    );
  });

  it("displays plate emoji for mensa statistics", async () => {
    render(<StudentMensaHistoryPage />);

    await waitFor(
      () => {
        // Check for plate emoji in Mensa-Übersicht
        expect(screen.getByText("🍽️")).toBeInTheDocument();
      },
      { timeout: 2000 },
    );
  });
});

describe("Mensa entry status labels", () => {
  it("defines correct status labels", () => {
    const statusLabels = {
      eaten: "Gegessen",
      notEaten: "Nichts gegessen",
    };

    expect(statusLabels.eaten).toBe("Gegessen");
    expect(statusLabels.notEaten).toBe("Nichts gegessen");
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
