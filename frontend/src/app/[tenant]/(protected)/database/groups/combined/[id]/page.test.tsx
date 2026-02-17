/* eslint-disable @typescript-eslint/no-unsafe-return, @typescript-eslint/no-empty-function */
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";

const mockPush = vi.fn();
const mockBack = vi.fn();
const mockGetCombinedGroup = vi.fn();
const mockUpdateCombinedGroup = vi.fn();
const mockDeleteCombinedGroup = vi.fn();
const mockAddGroupToCombined = vi.fn();
const mockRemoveGroupFromCombined = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush, back: mockBack }),
  useParams: () => ({ id: "1" }),
}));

vi.mock("@/lib/api", () => ({
  combinedGroupService: {
    getCombinedGroup: (id: string) => mockGetCombinedGroup(id),
    updateCombinedGroup: (id: string, data: unknown) =>
      mockUpdateCombinedGroup(id, data),
    deleteCombinedGroup: (id: string) => mockDeleteCombinedGroup(id),
    addGroupToCombined: (combinedId: string, groupId: string) =>
      mockAddGroupToCombined(combinedId, groupId),
    removeGroupFromCombined: (combinedId: string, groupId: string) =>
      mockRemoveGroupFromCombined(combinedId, groupId),
  },
}));

vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title }: { title: string }) => (
    <div data-testid="page-header">{title}</div>
  ),
}));

vi.mock("@/components/groups", () => ({
  CombinedGroupForm: ({
    formTitle,
    onCancelAction,
    onSubmitAction,
  }: {
    formTitle: string;
    onCancelAction: () => void;
    onSubmitAction: (data: unknown) => Promise<void>;
  }) => (
    <div data-testid="combined-group-form">
      <span>{formTitle}</span>
      <button data-testid="form-cancel" onClick={onCancelAction}>
        Cancel
      </button>
      <button
        data-testid="form-submit"
        onClick={() => {
          onSubmitAction({ name: "Updated" }).catch(() => {
            // Error handled by parent
          });
        }}
      >
        Save
      </button>
    </div>
  ),
}));

import CombinedGroupDetailPage from "./page";

describe("CombinedGroupDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.spyOn(console, "error").mockImplementation(() => {});
    // Mock globalThis.confirm
    globalThis.confirm = vi.fn(() => true);
  });

  afterEach(() => {
    vi.restoreAllMocks();
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

  it("shows expired badge when group is expired", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Expired Group",
      is_active: true,
      is_expired: true,
      access_policy: "all",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      const expiredElements = screen.getAllByText("Abgelaufen");
      expect(expiredElements.length).toBeGreaterThan(0);
    });
  });

  it("shows valid until badge when present", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Timed Group",
      is_active: true,
      is_expired: false,
      access_policy: "all",
      valid_until: "2025-12-31T23:59:59Z",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/Gültig bis:/)).toBeInTheDocument();
    });
  });

  it("shows time until expiration when present", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Expiring Group",
      is_active: true,
      is_expired: false,
      access_policy: "all",
      time_until_expiration: "5 Tage",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Läuft ab in: 5 Tage")).toBeInTheDocument();
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

  it("navigates back when clicking back button on error", async () => {
    mockGetCombinedGroup.mockRejectedValue(new Error("Not found"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Zurück")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zurück"));
    expect(mockBack).toHaveBeenCalled();
  });

  it("shows not found state when data is null without error", async () => {
    mockGetCombinedGroup.mockResolvedValue(null);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Gruppenkombination nicht gefunden"),
      ).toBeInTheDocument();
      expect(screen.getByText("Zurück zur Übersicht")).toBeInTheDocument();
    });
  });

  it("navigates to combined groups list from not found state", async () => {
    mockGetCombinedGroup.mockResolvedValue(null);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Zurück zur Übersicht")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Zurück zur Übersicht"));
    expect(mockPush).toHaveBeenCalledWith("/database/groups/combined");
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

  it("shows access policy label for specific group", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test",
      is_active: true,
      is_expired: false,
      access_policy: "specific",
      specific_group: { name: "Spezielle Gruppe" },
      specific_group_id: "42",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Spezifische Gruppe").length).toBeGreaterThan(
        0,
      );
      expect(screen.getByText("Spezielle Gruppe")).toBeInTheDocument();
      expect(screen.getByText(/Spezifische Gruppe: 42/)).toBeInTheDocument();
    });
  });

  it("shows access specialists when present", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Group with Specialists",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      access_specialists: [
        { id: "1", name: "Dr. Smith" },
        { id: "2", name: "Prof. Jones" },
      ],
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Dr. Smith")).toBeInTheDocument();
      expect(screen.getByText("Prof. Jones")).toBeInTheDocument();
    });
  });

  it("shows no specialists message when list is empty", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "No Specialists",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      access_specialists: [],
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine speziellen Zugriffspersonen festgelegt."),
      ).toBeInTheDocument();
    });
  });

  it("toggles to edit mode when edit button is clicked", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
      const editHeaders = screen.getAllByText("Gruppenkombination bearbeiten");
      expect(editHeaders.length).toBeGreaterThan(0);
    });
  });

  it("exits edit mode when form cancel is clicked", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
    });

    // Enter edit mode
    fireEvent.click(screen.getByText("Bearbeiten"));

    await waitFor(() => {
      expect(screen.getByTestId("form-cancel")).toBeInTheDocument();
    });

    // Cancel edit
    fireEvent.click(screen.getByTestId("form-cancel"));

    await waitFor(() => {
      expect(screen.getByText("Kombinationsdetails")).toBeInTheDocument();
    });
  });

  it("handles successful update", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockUpdateCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Updated Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    await waitFor(() => {
      fireEvent.click(screen.getByTestId("form-submit"));
    });

    await waitFor(() => {
      expect(mockUpdateCombinedGroup).toHaveBeenCalledWith("1", {
        name: "Updated",
      });
      expect(screen.getByText("Kombinationsdetails")).toBeInTheDocument();
    });
  });

  it("shows error message on update failure", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockUpdateCombinedGroup.mockRejectedValue(new Error("Update failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Bearbeiten"));
    });

    await waitFor(() => {
      expect(screen.getByTestId("form-submit")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId("form-submit"));

    // Component stays in edit mode and error is handled
    await waitFor(
      () => {
        expect(mockUpdateCombinedGroup).toHaveBeenCalled();
      },
      { timeout: 2000 },
    );
  });

  it("handles delete confirmation and success", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockDeleteCombinedGroup.mockResolvedValue(undefined);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Löschen"));
    });

    await waitFor(() => {
      expect(mockDeleteCombinedGroup).toHaveBeenCalledWith("1");
      expect(mockPush).toHaveBeenCalledWith("/database/groups/combined");
    });
  });

  it("does not delete when confirmation is cancelled", async () => {
    globalThis.confirm = vi.fn(() => false);

    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      fireEvent.click(screen.getByText("Löschen"));
    });

    expect(mockDeleteCombinedGroup).not.toHaveBeenCalled();
  });

  it("handles delete failure", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockDeleteCombinedGroup.mockRejectedValue(new Error("Delete failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Löschen")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByText("Löschen"));

    // Error is handled and does not navigate away
    await waitFor(
      () => {
        expect(mockDeleteCombinedGroup).toHaveBeenCalled();
        expect(mockPush).not.toHaveBeenCalled();
      },
      { timeout: 2000 },
    );
  });

  it("handles adding a group successfully", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockGetCombinedGroup.mockResolvedValueOnce({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [{ id: "99", name: "New Group" }],
    });

    mockAddGroupToCombined.mockResolvedValue(undefined);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByPlaceholderText("Gruppen-ID")).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText("Gruppen-ID");
    fireEvent.change(input, { target: { value: "99" } });
    fireEvent.click(screen.getByText("Hinzufügen"));

    await waitFor(() => {
      expect(mockAddGroupToCombined).toHaveBeenCalledWith("1", "99");
      expect(screen.getByText("New Group")).toBeInTheDocument();
    });
  });

  it("shows error message when adding group fails", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockAddGroupToCombined.mockRejectedValue(new Error("Add failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      const input = screen.getByPlaceholderText("Gruppen-ID");
      fireEvent.change(input, { target: { value: "99" } });
      fireEvent.click(screen.getByText("Hinzufügen"));
    });

    await waitFor(() => {
      expect(screen.getByText(/Fehler beim Hinzufügen/)).toBeInTheDocument();
    });
  });

  it("handles removing a group successfully", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [{ id: "10", name: "Group to Remove" }],
    });

    mockGetCombinedGroup.mockResolvedValueOnce({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [],
    });

    mockRemoveGroupFromCombined.mockResolvedValue(undefined);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Group to Remove")).toBeInTheDocument();
    });

    // Click the remove button (SVG button)
    const removeButtons = screen.getAllByTitle("Gruppe entfernen");
    fireEvent.click(removeButtons[0]!);

    await waitFor(() => {
      expect(mockRemoveGroupFromCombined).toHaveBeenCalledWith("1", "10");
      expect(
        screen.getByText("Keine Gruppen in dieser Kombination."),
      ).toBeInTheDocument();
    });
  });

  it("shows error message when removing group fails", async () => {
    mockGetCombinedGroup.mockResolvedValue({
      id: "1",
      name: "Test Group",
      is_active: true,
      is_expired: false,
      access_policy: "manual",
      groups: [{ id: "10", name: "Group to Remove" }],
    });

    mockRemoveGroupFromCombined.mockRejectedValue(new Error("Remove failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      const removeButtons = screen.getAllByTitle("Gruppe entfernen");
      fireEvent.click(removeButtons[0]!);
    });

    await waitFor(() => {
      expect(screen.getByText(/Fehler beim Entfernen/)).toBeInTheDocument();
    });
  });

  it("displays all access policy options correctly", async () => {
    const policies = [
      { policy: "all", label: "Alle Gruppen" },
      { policy: "first", label: "Erste Gruppe" },
      { policy: "manual", label: "Manuell" },
    ];

    for (const { policy, label } of policies) {
      mockGetCombinedGroup.mockResolvedValue({
        id: "1",
        name: "Test",
        is_active: true,
        is_expired: false,
        access_policy: policy,
        groups: [],
      });

      const { unmount } = render(<CombinedGroupDetailPage />);

      await waitFor(() => {
        expect(screen.getByText(label)).toBeInTheDocument();
      });

      unmount();
    }
  });
});
