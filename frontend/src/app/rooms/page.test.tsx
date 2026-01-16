import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import RoomsPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: vi.fn(),
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
        data-testid="filter-building"
        onClick={() => filters?.[0]?.onChange("Main")}
      >
        Building
      </button>
      <button
        data-testid="filter-occupied"
        onClick={() => filters?.[1]?.onChange("occupied")}
      >
        Occupied
      </button>
      <button data-testid="clear-filters" onClick={onClearAllFilters}>
        Clear
      </button>
    </div>
  ),
}));

import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { useSWRAuth } from "~/lib/swr";

const mockRooms = [
  {
    id: "1",
    name: "Raum 101",
    building: "Main",
    isOccupied: true,
    groupName: "Gruppe A",
  },
  {
    id: "2",
    name: "Musikraum",
    building: "Annex",
    isOccupied: false,
  },
];

describe("RoomsPage", () => {
  const mockPush = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useRouter).mockReturnValue({ push: mockPush } as never);
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

    render(<RoomsPage />);

    expect(screen.getByLabelText("Lädt...")).toBeInTheDocument();
  });

  it("filters rooms by search, building, and occupancy", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    expect(screen.getByText("Raum 101")).toBeInTheDocument();
    expect(screen.getByText("Musikraum")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "musik" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Raum 101")).not.toBeInTheDocument();
      expect(screen.getByText("Musikraum")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.getByText("Musikraum")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-building"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Musikraum")).not.toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-occupied"));

    await waitFor(() => {
      expect(screen.getByText("Raum 101")).toBeInTheDocument();
      expect(screen.queryByText("Musikraum")).not.toBeInTheDocument();
    });
  });

  it("navigates to room detail on card click", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockRooms,
      isLoading: false,
      error: null,
    } as never);

    render(<RoomsPage />);

    fireEvent.click(screen.getByRole("button", { name: /Raum 101/i }));

    expect(mockPush).toHaveBeenCalledWith("/rooms/1");
  });
});
