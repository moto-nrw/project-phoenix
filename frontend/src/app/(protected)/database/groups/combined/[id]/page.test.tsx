/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-empty-function */
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import CombinedGroupDetailPage from "./page";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), back: vi.fn() }),
  useParams: () => ({ id: "1" }),
}));

const mockGetCombinedGroup = vi.fn();
vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    getCombinedGroup: (id: string) => mockGetCombinedGroup(id),
    updateCombinedGroup: vi.fn(),
    deleteCombinedGroup: vi.fn(),
    addGroupToCombined: vi.fn(),
    removeGroupFromCombined: vi.fn(),
  },
}));

vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("@/components/groups", () => ({
  CombinedGroupForm: ({ formTitle }: { formTitle: string }) => (
    <div data-testid="combined-group-form">{formTitle}</div>
  ),
}));

describe("CombinedGroupDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows loading state initially", () => {
    mockGetCombinedGroup.mockReturnValue(new Promise(() => {}));
    render(<CombinedGroupDetailPage />);

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
  });

  it("renders combined group details after loading", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Kombination",
      is_active: true,
      is_expired: false,
      access_policy: "all",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Test Kombination").length).toBeGreaterThan(0);
      expect(screen.getByText("Kombinationsdetails")).toBeInTheDocument();
      expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
      expect(screen.getByText("Löschen")).toBeInTheDocument();
    });
  });

  it("shows active status label", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Active Group",
      is_active: true,
      is_expired: false,
      access_policy: "all",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Aktiv").length).toBeGreaterThan(0);
    });
  });

  it("shows error state on API failure", async () => {
    mockGetCombinedGroup.mockRejectedValue(new Error("Not found"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Fehler")).toBeInTheDocument();
      expect(screen.getByText("Zurück")).toBeInTheDocument();
    });
  });

  it("shows no groups message when group list is empty", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Empty Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Gruppen in dieser Kombination."),
      ).toBeInTheDocument();
    });
  });

  it("renders groups in the combination", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "With Groups",
      is_active: true,
      is_expired: false,
      access_policy: "all",
      groups: [
        { id: "10", name: "Gruppe A", room_name: "Raum 1" },
        { id: "20", name: "Gruppe B" },
      ],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
      expect(screen.getByText("Raum: Raum 1")).toBeInTheDocument();
      expect(screen.getByText("Gruppe B")).toBeInTheDocument();
    });
  });

  it("shows access policy label", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test",
      is_active: true,
      is_expired: false,
      access_policy: "specific",
      specific_group: { name: "Spezielle Gruppe" },
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Spezifische Gruppe").length).toBeGreaterThan(
        0,
      );
      expect(screen.getByText("Spezielle Gruppe")).toBeInTheDocument();
    });
  });
});
