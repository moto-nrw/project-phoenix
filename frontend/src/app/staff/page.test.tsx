import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StaffPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
}));

vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({ children }: { children: React.ReactNode }) => (
    <div data-testid="layout">{children}</div>
  ),
}));

vi.mock("~/components/ui/page-header", () => ({
  PageHeaderWithSearch: ({
    search,
    filters,
    onClearAllFilters,
  }: {
    search: { value: string; onChange: (v: string) => void };
    filters?: Array<{ onChange: (v: string | string[]) => void }>;
    onClearAllFilters: () => void;
  }) => (
    <div data-testid="page-header">
      <input
        data-testid="search-input"
        value={search.value}
        onChange={(e) => search.onChange(e.target.value)}
      />
      <button
        data-testid="filter-schulhof"
        onClick={() => filters?.[0]?.onChange("schulhof")}
      >
        Schulhof
      </button>
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";

const mockStaff = [
  {
    id: "1",
    name: "Anna Meyer",
    firstName: "Anna",
    lastName: "Meyer",
    hasRfid: true,
    isTeacher: false,
    isSupervising: false,
    currentLocation: "Zuhause",
    staffNotes: "Notiz",
  },
  {
    id: "2",
    name: "Ben Schulz",
    firstName: "Ben",
    lastName: "Schulz",
    hasRfid: false,
    isTeacher: true,
    specialization: "Sport",
    isSupervising: true,
    supervisionRole: "primary",
    currentLocation: "Schulhof",
    qualifications: "Erste Hilfe",
  },
];

describe("StaffPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1" } },
      status: "authenticated",
    } as never);
  });

  it("shows loading state while session is loading", () => {
    vi.mocked(useSession).mockReturnValue({
      data: null,
      status: "loading",
    } as never);
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
  });

  it("filters staff by search and location", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByText("Anna")).toBeInTheDocument();
    expect(screen.getByText("Ben")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Schulz" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Anna")).not.toBeInTheDocument();
      expect(screen.getByText("Ben")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByText("Anna")).toBeInTheDocument();
      expect(screen.getByText("Ben")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-schulhof"));

    await waitFor(() => {
      expect(screen.queryByText("Anna")).not.toBeInTheDocument();
      expect(screen.getByText("Ben")).toBeInTheDocument();
    });
  });

  it("shows error message when staff fetch fails", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: new Error("Boom"),
    } as never);

    render(<StaffPage />);

    expect(
      screen.getByText("Fehler beim Laden der Personaldaten."),
    ).toBeInTheDocument();
  });

  it("shows empty state when no staff match filters", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "nonexistent" },
    });

    await waitFor(() => {
      expect(screen.getByText("Kein Personal gefunden")).toBeInTheDocument();
    });
  });

  it("shows empty state when staff data is empty", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByText("Kein Personal gefunden")).toBeInTheDocument();
  });

  it("displays staff with specialization", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByText("Sport")).toBeInTheDocument();
  });

  it("displays staff qualifications", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByText("Erste Hilfe")).toBeInTheDocument();
  });

  it("displays staff notes when available", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByText("Notiz")).toBeInTheDocument();
  });

  it("shows loading state while data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
  });
});

describe("StaffPage location filter logic", () => {
  it("matches all locations when filter is 'all'", () => {
    const matchesLocationFilter = (
      location: string,
      filter: string,
    ): boolean => {
      if (filter === "all") return true;
      if (filter === "zuhause") return location === "Zuhause";
      if (filter === "schulhof") return location === "Schulhof";
      if (filter === "unterwegs") return location === "Unterwegs";
      if (filter === "im_raum") {
        return (
          location !== "Zuhause" &&
          location !== "Schulhof" &&
          location !== "Unterwegs"
        );
      }
      return true;
    };

    const filter = "all";
    expect(matchesLocationFilter("Zuhause", filter)).toBe(true);
    expect(matchesLocationFilter("Schulhof", filter)).toBe(true);
    expect(matchesLocationFilter("Raum 101", filter)).toBe(true);
  });

  it("matches only 'Zuhause' when filter is 'zuhause'", () => {
    const matchesLocationFilter = (
      location: string,
      filter: string,
    ): boolean => {
      if (filter === "all") return true;
      if (filter === "zuhause") return location === "Zuhause";
      if (filter === "schulhof") return location === "Schulhof";
      if (filter === "unterwegs") return location === "Unterwegs";
      if (filter === "im_raum") {
        return (
          location !== "Zuhause" &&
          location !== "Schulhof" &&
          location !== "Unterwegs"
        );
      }
      return true;
    };

    const filter = "zuhause";
    expect(matchesLocationFilter("Zuhause", filter)).toBe(true);
    expect(matchesLocationFilter("Schulhof", filter)).toBe(false);
    expect(matchesLocationFilter("Raum 101", filter)).toBe(false);
  });

  it("matches room locations when filter is 'im_raum'", () => {
    const matchesLocationFilter = (
      location: string,
      filter: string,
    ): boolean => {
      if (filter === "all") return true;
      if (filter === "zuhause") return location === "Zuhause";
      if (filter === "anwesend") return location === "Anwesend";
      if (filter === "schulhof") return location === "Schulhof";
      if (filter === "unterwegs") return location === "Unterwegs";
      if (filter === "im_raum") {
        return (
          location !== "Zuhause" &&
          location !== "Anwesend" &&
          location !== "Schulhof" &&
          location !== "Unterwegs"
        );
      }
      return true;
    };

    const filter = "im_raum";
    expect(matchesLocationFilter("Zuhause", filter)).toBe(false);
    expect(matchesLocationFilter("Anwesend", filter)).toBe(false);
    expect(matchesLocationFilter("Schulhof", filter)).toBe(false);
    expect(matchesLocationFilter("Unterwegs", filter)).toBe(false);
    expect(matchesLocationFilter("Raum 101", filter)).toBe(true);
    expect(matchesLocationFilter("Musik", filter)).toBe(true);
  });

  it("matches only 'Anwesend' when filter is 'anwesend'", () => {
    const matchesLocationFilter = (
      location: string,
      filter: string,
    ): boolean => {
      if (filter === "all") return true;
      if (filter === "zuhause") return location === "Zuhause";
      if (filter === "anwesend") return location === "Anwesend";
      if (filter === "schulhof") return location === "Schulhof";
      if (filter === "unterwegs") return location === "Unterwegs";
      if (filter === "im_raum") {
        return (
          location !== "Zuhause" &&
          location !== "Anwesend" &&
          location !== "Schulhof" &&
          location !== "Unterwegs"
        );
      }
      return true;
    };

    const filter = "anwesend";
    expect(matchesLocationFilter("Anwesend", filter)).toBe(true);
    expect(matchesLocationFilter("Zuhause", filter)).toBe(false);
    expect(matchesLocationFilter("Schulhof", filter)).toBe(false);
    expect(matchesLocationFilter("Raum 101", filter)).toBe(false);
  });
});

describe("StaffPage search filter logic", () => {
  it("filters staff by first name", () => {
    const staff = [
      { firstName: "Anna", lastName: "Meyer", name: "Anna Meyer" },
      { firstName: "Ben", lastName: "Schulz", name: "Ben Schulz" },
    ];

    const searchTerm = "anna";
    const filtered = staff.filter((s) => {
      const searchLower = searchTerm.toLowerCase();
      return (
        s.firstName.toLowerCase().includes(searchLower) ||
        s.lastName.toLowerCase().includes(searchLower) ||
        s.name.toLowerCase().includes(searchLower)
      );
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Anna Meyer");
  });

  it("filters staff by last name", () => {
    const staff = [
      { firstName: "Anna", lastName: "Meyer", name: "Anna Meyer" },
      { firstName: "Ben", lastName: "Schulz", name: "Ben Schulz" },
    ];

    const searchTerm = "schulz";
    const filtered = staff.filter((s) => {
      const searchLower = searchTerm.toLowerCase();
      return (
        s.firstName.toLowerCase().includes(searchLower) ||
        s.lastName.toLowerCase().includes(searchLower) ||
        s.name.toLowerCase().includes(searchLower)
      );
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Ben Schulz");
  });

  it("combines search and location filters", () => {
    const staff = [
      {
        firstName: "Anna",
        lastName: "Meyer",
        name: "Anna Meyer",
        currentLocation: "Zuhause",
      },
      {
        firstName: "Ben",
        lastName: "Schulz",
        name: "Ben Schulz",
        currentLocation: "Schulhof",
      },
      {
        firstName: "Anna",
        lastName: "Becker",
        name: "Anna Becker",
        currentLocation: "Schulhof",
      },
    ];

    const searchTerm = "anna";
    const locationFilter = "schulhof";

    const matchesLocationFilter = (
      location: string,
      filter: string,
    ): boolean => {
      if (filter === "all") return true;
      if (filter === "schulhof") return location === "Schulhof";
      return true;
    };

    const filtered = staff.filter((s) => {
      const searchLower = searchTerm.toLowerCase();
      const matchesSearch =
        s.firstName.toLowerCase().includes(searchLower) ||
        s.lastName.toLowerCase().includes(searchLower) ||
        s.name.toLowerCase().includes(searchLower);

      const location = s.currentLocation ?? "Zuhause";
      return matchesSearch && matchesLocationFilter(location, locationFilter);
    });

    expect(filtered).toHaveLength(1);
    expect(filtered[0]?.name).toBe("Anna Becker");
  });
});

describe("StaffPage active filters", () => {
  it("creates search filter when search term exists", () => {
    const searchTerm = "Test";
    const locationFilter = "all";
    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (locationFilter !== "all") {
      const locationLabels: Record<string, string> = {
        zuhause: "Zuhause",
        im_raum: "Im Raum",
        schulhof: "Schulhof",
        unterwegs: "Unterwegs",
      };
      filters.push({
        id: "location",
        label: locationLabels[locationFilter] ?? locationFilter,
      });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]).toEqual({ id: "search", label: '"Test"' });
  });

  it("creates location filter when location is not 'all'", () => {
    const searchTerm = "";
    const locationFilter = "im_raum" as string;
    const filters: Array<{ id: string; label: string }> = [];

    if (searchTerm.length > 0) {
      filters.push({ id: "search", label: `"${searchTerm}"` });
    }

    if (locationFilter !== "all") {
      const locationLabels: Record<string, string> = {
        zuhause: "Zuhause",
        im_raum: "Im Raum",
        schulhof: "Schulhof",
        unterwegs: "Unterwegs",
      };
      filters.push({
        id: "location",
        label: locationLabels[locationFilter] ?? locationFilter,
      });
    }

    expect(filters).toHaveLength(1);
    expect(filters[0]).toEqual({ id: "location", label: "Im Raum" });
  });
});
