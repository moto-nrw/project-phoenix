import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StaffPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(),
  useRouter: vi.fn(() => ({
    push: vi.fn(),
    replace: vi.fn(),
    back: vi.fn(),
    forward: vi.fn(),
    refresh: vi.fn(),
    prefetch: vi.fn(),
  })),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn(),
  useTenantMutate: vi.fn(() => vi.fn()),
}));

vi.mock("~/lib/hooks/use-staff-pending-absences", () => ({
  useStaffPendingAbsences: vi.fn(() => ({ rows: [], canReview: false })),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
  ),
}));

vi.mock("~/components/ui/page-header/PageHeaderWithSearch", () => ({
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
        type="button"
        data-testid="filter-im-raum"
        onClick={() => filters?.[0]?.onChange("im_raum")}
      >
        Im Raum
      </button>
      <button
        type="button"
        data-testid="filter-abwesend"
        onClick={() => filters?.[0]?.onChange("abwesend")}
      >
        Abwesend
      </button>
      <button
        type="button"
        data-testid="clear-filters"
        onClick={onClearAllFilters}
      >
        Clear
      </button>
    </div>
  ),
}));

import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { useStaffPendingAbsences } from "~/lib/hooks/use-staff-pending-absences";

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
    currentLocation: "Raum 101",
    qualifications: "Erste Hilfe",
  },
];

describe("StaffPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useStaffPendingAbsences).mockReturnValue({
      rows: [],
      canReview: false,
    });
    vi.mocked(useSession).mockReturnValue({
      data: { user: { id: "1", permissions: ["users:read"] } },
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

    expect(
      screen.getByLabelText("Mitarbeitende werden geladen"),
    ).toBeInTheDocument();
  });

  it("filters staff by search and location", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(screen.getByText("Anna Meyer")).toBeInTheDocument();
    expect(screen.getByText("Ben Schulz")).toBeInTheDocument();

    fireEvent.change(screen.getByTestId("search-input"), {
      target: { value: "Schulz" },
    });

    await waitFor(() => {
      expect(screen.queryByText("Anna Meyer")).not.toBeInTheDocument();
      expect(screen.getByText("Ben Schulz")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("clear-filters"));

    await waitFor(() => {
      expect(screen.getByText("Anna Meyer")).toBeInTheDocument();
      expect(screen.getByText("Ben Schulz")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-im-raum"));

    await waitFor(() => {
      expect(screen.queryByText("Anna Meyer")).not.toBeInTheDocument();
      expect(screen.getByText("Ben Schulz")).toBeInTheDocument();
    });
  });

  it("classifies Freizeitausgleich as absence rather than a room", async () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [
        {
          ...mockStaff[0],
          id: "3",
          name: "Carla Frei",
          firstName: "Carla",
          lastName: "Frei",
          currentLocation: "Freizeitausgleich",
        },
      ],
      isLoading: false,
      error: null,
    } as never);

    render(<StaffPage />);
    fireEvent.click(screen.getByTestId("filter-abwesend"));

    await waitFor(() => {
      expect(screen.getByText("Carla Frei")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("filter-im-raum"));

    await waitFor(() => {
      expect(screen.queryByText("Carla Frei")).not.toBeInTheDocument();
    });
  });

  it("shows the catalog text with retry when staff fetch fails", async () => {
    const mutate = vi.fn();
    vi.mocked(useSWRAuth).mockReturnValue({
      data: [],
      isLoading: false,
      error: new ApiError("boom", 500, {
        code: "general.server",
        instance: "req-staff",
      }),
      mutate,
    } as never);

    render(<StaffPage />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Personalliste"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-staff");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalled();
    // Without loaded staff there is nothing to count: "0 Personen" would
    // read as a school without staff (#2514).
    expect(screen.queryByText(/0 Personen/)).not.toBeInTheDocument();
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

  it("shows open absence requests as a separate action row", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);
    vi.mocked(useStaffPendingAbsences).mockReturnValue({
      rows: [{ staff_id: 1 }, { staff_id: 1 }] as never,
      canReview: true,
    });

    render(<StaffPage />);

    const requestLabel = screen.getByText(
      (_content, element) =>
        element?.tagName === "SPAN" &&
        element.textContent?.trim() === "2 offene Abwesenheitsanträge",
    );
    const requestRow = requestLabel.parentElement;
    expect(requestRow).toHaveClass("text-moto-orange-strong");
    expect(requestRow).not.toHaveClass("text-gray-600");
    expect(requestRow).not.toHaveClass(
      "border-moto-orange/30",
      "bg-moto-orange/10",
    );
    expect(within(requestLabel).getByText("2")).toHaveClass("font-semibold");
    expect(screen.queryByText("2 Anfragen")).not.toBeInTheDocument();
  });

  it("points approvers to the Anfragen module with the open count", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);
    vi.mocked(useStaffPendingAbsences).mockReturnValue({
      rows: [{ staff_id: 1 }, { staff_id: 2 }] as never,
      canReview: true,
    });

    render(<StaffPage />);

    const link = screen.getByRole("link", {
      name: /Anträge von Mitarbeitenden/,
    });
    expect(link).toHaveAttribute("href", "/test-tenant/anfragen");
    expect(screen.getByLabelText("2 offene Anträge")).toBeInTheDocument();
  });

  it("hides the Anfragen pointer without vacation:approve", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: mockStaff,
      isLoading: false,
      error: null,
    } as never);
    vi.mocked(useStaffPendingAbsences).mockReturnValue({
      rows: [],
      canReview: false,
    });

    render(<StaffPage />);

    expect(
      screen.queryByRole("link", { name: /Anträge von Mitarbeitenden/ }),
    ).not.toBeInTheDocument();
  });

  it("shows loading state while data is loading", () => {
    vi.mocked(useSWRAuth).mockReturnValue({
      data: null,
      isLoading: true,
      error: null,
    } as never);

    render(<StaffPage />);

    expect(
      screen.getByLabelText("Mitarbeitende werden geladen"),
    ).toBeInTheDocument();
  });

  it("disables previous-key data for time-account filters", () => {
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          permissions: ["users:read", "time_tracking:manage"],
        },
      },
      status: "authenticated",
    } as never);
    vi.mocked(useSWRAuth).mockImplementation((key) => {
      if (key === "staff-list") {
        return { data: mockStaff, isLoading: false, error: null } as never;
      }
      return {
        data: undefined,
        isLoading: false,
        isValidating: false,
        error: null,
        mutate: vi.fn(),
      } as never;
    });

    render(<StaffPage />);

    const accountsTab = screen.getByRole("tab", { name: "Zeitkonten" });
    fireEvent.pointerDown(accountsTab, {
      button: 0,
      pointerType: "mouse",
    });
    fireEvent.mouseDown(accountsTab, { button: 0 });
    fireEvent.click(accountsTab);

    const accountsCall = vi
      .mocked(useSWRAuth)
      .mock.calls.find(
        ([key]) =>
          typeof key === "string" && key.startsWith("staff-time-accounts-"),
      );
    expect(accountsCall?.[2]).toEqual({
      keepPreviousData: false,
      revalidateOnFocus: false,
    });
  });
});
