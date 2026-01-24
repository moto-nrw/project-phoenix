import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import CombinedGroupDetailPage from "./page";

// Mock next/navigation
const mockPush = vi.fn();
const mockBack = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: mockPush,
    back: mockBack,
  })),
  useParams: vi.fn(() => ({
    id: "1",
  })),
}));

// Mock combinedGroupService
const mockGetCombinedGroup = vi.fn();
const mockUpdateCombinedGroup = vi.fn();
const mockDeleteCombinedGroup = vi.fn();
const mockAddGroupToCombined = vi.fn();
const mockRemoveGroupFromCombined = vi.fn();
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

// Mock UI components
vi.mock("@/components/dashboard", () => ({
  PageHeader: ({ title }: { title: string }) => (
    <div data-testid="page-header">
      <h2>{title}</h2>
    </div>
  ),
}));

vi.mock("@/components/groups", () => ({
  CombinedGroupForm: ({
    onSubmitAction,
    onCancelAction,
    isLoading,
    submitLabel,
  }: {
    initialData: unknown;
    onSubmitAction: (data: {
      name: string;
      is_active: boolean;
      access_policy: string;
    }) => Promise<void>;
    onCancelAction: () => void;
    isLoading: boolean;
    formTitle: string;
    submitLabel: string;
  }) => (
    <div data-testid="combined-group-form">
      <span data-testid="form-loading">{String(isLoading)}</span>
      <button
        data-testid="submit-form"
        onClick={() => {
          onSubmitAction({
            name: "Updated Group",
            is_active: true,
            access_policy: "all",
          }).catch(() => {
            // Error is handled by the component
          });
        }}
      >
        {submitLabel}
      </button>
      <button data-testid="cancel-form" onClick={onCancelAction}>
        Cancel
      </button>
    </div>
  ),
}));

// Mock globalThis.confirm
const mockConfirm = vi.fn(() => true);
vi.stubGlobal("confirm", mockConfirm);

const mockCombinedGroup = {
  id: "1",
  name: "Test Combined Group",
  is_active: true,
  is_expired: false,
  access_policy: "all",
  valid_until: "2025-12-31",
  time_until_expiration: "10 days",
  specific_group_id: "5",
  specific_group: { id: "5", name: "Specific Group" },
  groups: [
    { id: "10", name: "Group A", room_name: "Room 101" },
    { id: "20", name: "Group B", room_name: null },
  ],
  access_specialists: [
    { id: "100", name: "Specialist 1" },
    { id: "200", name: "Specialist 2" },
  ],
};

describe("CombinedGroupDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetCombinedGroup.mockResolvedValue(mockCombinedGroup);
  });

  it("renders the page with combined group details", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      // Use heading role to get the h1 element specifically
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });
  });

  it("shows loading state while fetching data", () => {
    mockGetCombinedGroup.mockImplementation(() => new Promise(() => {}));

    render(<CombinedGroupDetailPage />);

    expect(screen.getByText("Daten werden geladen...")).toBeInTheDocument();
  });

  it("shows error message when fetch fails", async () => {
    mockGetCombinedGroup.mockRejectedValueOnce(new Error("Failed to fetch"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Fehler" }),
      ).toBeInTheDocument();
    });
  });

  it("shows not found message when combined group is null", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce(null);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Gruppenkombination nicht gefunden"),
      ).toBeInTheDocument();
    });
  });

  it("displays active status badge", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    // "Aktiv" may appear multiple times - just check it exists
    expect(screen.getAllByText("Aktiv").length).toBeGreaterThan(0);
  });

  it("displays expired status badge when expired", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      is_active: true,
      is_expired: true,
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      // "Abgelaufen" may appear multiple times - just check it exists
      expect(screen.getAllByText("Abgelaufen").length).toBeGreaterThan(0);
    });
  });

  it("displays valid_until date", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/Gültig bis:/)).toBeInTheDocument();
    });
  });

  it("displays time until expiration", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/Läuft ab in: 10 days/)).toBeInTheDocument();
    });
  });

  it("displays access policy correctly", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Alle Gruppen")).toBeInTheDocument();
    });
  });

  it("displays access policy as 'Erste Gruppe'", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      access_policy: "first",
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Erste Gruppe")).toBeInTheDocument();
    });
  });

  it("displays access policy as 'Spezifische Gruppe'", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      access_policy: "specific",
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      // "Spezifische Gruppe" appears in multiple places - check for at least one
      expect(screen.getAllByText("Spezifische Gruppe").length).toBeGreaterThan(
        0,
      );
    });
  });

  it("displays access policy as 'Manuell' for unknown policy", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      access_policy: "unknown",
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Manuell")).toBeInTheDocument();
    });
  });

  it("displays specific group when access policy is specific", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      access_policy: "specific",
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getAllByText("Spezifische Gruppe").length).toBeGreaterThan(
        0,
      );
      expect(screen.getByText("Specific Group")).toBeInTheDocument();
    });
  });

  it("displays groups in the combination", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Group A")).toBeInTheDocument();
      expect(screen.getByText("Group B")).toBeInTheDocument();
    });
  });

  it("displays room name for groups that have it", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/Raum: Room 101/)).toBeInTheDocument();
    });
  });

  it("displays access specialists", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Specialist 1")).toBeInTheDocument();
      expect(screen.getByText("Specialist 2")).toBeInTheDocument();
    });
  });

  it("shows message when no groups in combination", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      groups: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine Gruppen in dieser Kombination."),
      ).toBeInTheDocument();
    });
  });

  it("shows message when no access specialists", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      access_specialists: [],
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Keine speziellen Zugriffspersonen festgelegt."),
      ).toBeInTheDocument();
    });
  });

  it("switches to edit mode when edit button is clicked", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const editButton = screen.getByText("Bearbeiten");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
    });
  });

  it("cancels edit mode and returns to detail view", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const editButton = screen.getByText("Bearbeiten");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
    });

    const cancelButton = screen.getByTestId("cancel-form");
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(
        screen.queryByTestId("combined-group-form"),
      ).not.toBeInTheDocument();
    });
  });

  it("calls update service when form is submitted", async () => {
    mockUpdateCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      name: "Updated Group",
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const editButton = screen.getByText("Bearbeiten");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
    });

    const submitButton = screen.getByTestId("submit-form");
    fireEvent.click(submitButton);

    await waitFor(() => {
      expect(mockUpdateCombinedGroup).toHaveBeenCalledWith("1", {
        name: "Updated Group",
        is_active: true,
        access_policy: "all",
      });
    });
  });

  it("shows error when update fails", async () => {
    mockUpdateCombinedGroup.mockRejectedValueOnce(new Error("Update failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const editButton = screen.getByText("Bearbeiten");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(screen.getByTestId("combined-group-form")).toBeInTheDocument();
    });

    const submitButton = screen.getByTestId("submit-form");
    fireEvent.click(submitButton);

    // Wait for update to be attempted
    await waitFor(() => {
      expect(mockUpdateCombinedGroup).toHaveBeenCalledWith("1", {
        name: "Updated Group",
        is_active: true,
        access_policy: "all",
      });
    });

    // Cancel the form to go back to detail view where error is visible
    const cancelButton = screen.getByTestId("cancel-form");
    fireEvent.click(cancelButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Aktualisieren der Gruppenkombination/),
      ).toBeInTheDocument();
    });
  });

  it("deletes combined group when delete button is clicked and confirmed", async () => {
    mockDeleteCombinedGroup.mockResolvedValueOnce({});
    mockConfirm.mockReturnValueOnce(true);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const deleteButton = screen.getByText("Löschen");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(mockDeleteCombinedGroup).toHaveBeenCalledWith("1");
      expect(mockPush).toHaveBeenCalledWith("/database/groups/combined");
    });
  });

  it("does not delete when confirmation is cancelled", async () => {
    mockConfirm.mockReturnValueOnce(false);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const deleteButton = screen.getByText("Löschen");
    fireEvent.click(deleteButton);

    expect(mockDeleteCombinedGroup).not.toHaveBeenCalled();
  });

  it("shows error when delete fails", async () => {
    mockDeleteCombinedGroup.mockRejectedValueOnce(new Error("Delete failed"));
    mockConfirm.mockReturnValueOnce(true);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const deleteButton = screen.getByText("Löschen");
    fireEvent.click(deleteButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Löschen der Gruppenkombination/),
      ).toBeInTheDocument();
    });
  });

  it("adds a group to the combination", async () => {
    mockAddGroupToCombined.mockResolvedValueOnce({});
    mockGetCombinedGroup.mockResolvedValue(mockCombinedGroup);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText("Gruppen-ID");
    fireEvent.change(input, { target: { value: "30" } });

    const addButton = screen.getByText("Hinzufügen");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(mockAddGroupToCombined).toHaveBeenCalledWith("1", "30");
    });
  });

  it("does not add group with empty input", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const addButton = screen.getByText("Hinzufügen");
    fireEvent.click(addButton);

    expect(mockAddGroupToCombined).not.toHaveBeenCalled();
  });

  it("shows error when adding group fails", async () => {
    mockAddGroupToCombined.mockRejectedValueOnce(new Error("Add group failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { level: 1, name: "Test Combined Group" }),
      ).toBeInTheDocument();
    });

    const input = screen.getByPlaceholderText("Gruppen-ID");
    fireEvent.change(input, { target: { value: "30" } });

    const addButton = screen.getByText("Hinzufügen");
    fireEvent.click(addButton);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Hinzufügen der Gruppe/),
      ).toBeInTheDocument();
    });
  });

  it("removes a group from the combination", async () => {
    mockRemoveGroupFromCombined.mockResolvedValueOnce({});
    mockGetCombinedGroup.mockResolvedValue(mockCombinedGroup);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Group A")).toBeInTheDocument();
    });

    // Click the remove button for Group A (first X button)
    const removeButtons = screen.getAllByTitle("Gruppe entfernen");
    fireEvent.click(removeButtons[0]);

    await waitFor(() => {
      expect(mockRemoveGroupFromCombined).toHaveBeenCalledWith("1", "10");
    });
  });

  it("shows error when removing group fails", async () => {
    mockRemoveGroupFromCombined.mockRejectedValueOnce(
      new Error("Remove group failed"),
    );

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Group A")).toBeInTheDocument();
    });

    const removeButtons = screen.getAllByTitle("Gruppe entfernen");
    fireEvent.click(removeButtons[0]);

    await waitFor(() => {
      expect(
        screen.getByText(/Fehler beim Entfernen der Gruppe/),
      ).toBeInTheDocument();
    });
  });

  it("navigates back when back button is clicked on error page", async () => {
    mockGetCombinedGroup.mockRejectedValueOnce(new Error("Failed"));

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Fehler" }),
      ).toBeInTheDocument();
    });

    const backButton = screen.getByText("Zurück");
    fireEvent.click(backButton);

    expect(mockBack).toHaveBeenCalled();
  });

  it("navigates to overview when back button is clicked on not found page", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce(null);

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Gruppenkombination nicht gefunden"),
      ).toBeInTheDocument();
    });

    const backButton = screen.getByText("Zurück zur Übersicht");
    fireEvent.click(backButton);

    expect(mockPush).toHaveBeenCalledWith("/database/groups/combined");
  });

  it("displays status as Inaktiv when not active and not expired", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      is_active: false,
      is_expired: false,
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Inaktiv")).toBeInTheDocument();
    });
  });

  it("displays 'Kein Ablaufdatum' when valid_until is not set", async () => {
    mockGetCombinedGroup.mockResolvedValueOnce({
      ...mockCombinedGroup,
      valid_until: null,
    });

    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText("Kein Ablaufdatum")).toBeInTheDocument();
    });
  });

  it("displays combination ID", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/Kombination: 1/)).toBeInTheDocument();
    });
  });

  it("displays specific group ID when set", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(screen.getByText(/Spezifische Gruppe: 5/)).toBeInTheDocument();
    });
  });

  it("shows correct page header title based on editing state", async () => {
    render(<CombinedGroupDetailPage />);

    await waitFor(() => {
      expect(
        screen.getByText("Gruppenkombination Details"),
      ).toBeInTheDocument();
    });

    const editButton = screen.getByText("Bearbeiten");
    fireEvent.click(editButton);

    await waitFor(() => {
      expect(
        screen.getByText("Gruppenkombination bearbeiten"),
      ).toBeInTheDocument();
    });
  });
});
