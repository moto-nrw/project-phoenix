/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-empty-function */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import CombinedGroupsPage from "./page";

vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: {
      user: {
        id: "1",
        name: "Test User",
        email: "test@test.com",
        token: "tok",
        roles: ["admin"],
      },
    },
    status: "authenticated",
  })),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  redirect: vi.fn(),
}));

const mockGetCombinedGroups = vi.fn();
vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    getCombinedGroups: () => mockGetCombinedGroups(),
  },
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading...</div>,
}));

vi.mock("@/components/dashboard", () => ({
  DataListPage: ({
    title,
    data,
    renderEntity,
  }: {
    title: string;
    data: unknown[];
    renderEntity: (item: unknown) => React.ReactNode;
  }) => (
    <div data-testid="data-list-page">
      <h1>{title}</h1>
      <span data-testid="item-count">{data.length}</span>
      {data.map((item, i) => (
        <div key={i} data-testid="list-item">
          {renderEntity(item)}
        </div>
      ))}
    </div>
  ),
}));

describe("CombinedGroupsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders loading state initially", () => {
    mockGetCombinedGroups.mockReturnValue(new Promise(() => {}));
    render(<CombinedGroupsPage />);
    expect(screen.getByTestId("loading")).toBeInTheDocument();
  });

  it("renders combined groups list after loading", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Kombination A",
        is_active: true,
        is_expired: false,
        access_policy: "all",
        group_count: 3,
      },
    ]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppenkombinationen")).toBeInTheDocument();
      expect(screen.getByText("Kombination A")).toBeInTheDocument();
    });
  });

  it("shows active badge for active groups", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Active Group",
        is_active: true,
        is_expired: false,
        access_policy: "all",
      },
    ]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });
  });

  it("shows expired badge for expired groups", async () => {
    mockGetCombinedGroups.mockResolvedValue([
      {
        id: "1",
        name: "Expired Group",
        is_active: true,
        is_expired: true,
        access_policy: "all",
      },
    ]);

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Abgelaufen")).toBeInTheDocument();
    });
  });

  it("shows error state on API failure", async () => {
    mockGetCombinedGroups.mockRejectedValue(new Error("API Error"));

    render(<CombinedGroupsPage />);

    await waitFor(() => {
      expect(screen.getByText("Fehler")).toBeInTheDocument();
      expect(screen.getByText("Erneut versuchen")).toBeInTheDocument();
    });
  });
});
