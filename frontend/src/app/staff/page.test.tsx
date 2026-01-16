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
});
