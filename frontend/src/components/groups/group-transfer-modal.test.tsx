/**
 * Tests for GroupTransferModal Component
 * Tests the rendering and transfer functionality
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { GroupTransferModal } from "./group-transfer-modal";

// Mock Modal component
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    onClose,
    title,
    children,
  }: {
    isOpen: boolean;
    onClose: () => void;
    title: string;
    children: React.ReactNode;
  }) =>
    isOpen ? (
      <div data-testid="modal">
        <h2>{title}</h2>
        <button onClick={onClose} data-testid="modal-close">
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

const mockGroup = {
  id: "1",
  name: "Gruppe A",
  studentCount: 15,
};

const mockAvailableUsers = [
  {
    id: "1",
    personId: "p1",
    firstName: "John",
    lastName: "Doe",
    fullName: "John Doe",
    email: "john@example.com",
  },
  {
    id: "2",
    personId: "p2",
    firstName: "Jane",
    lastName: "Smith",
    fullName: "Jane Smith",
    email: "jane@example.com",
  },
];

const mockExistingTransfers = [
  {
    targetName: "Mike Johnson",
    substitutionId: "s1",
    targetStaffId: "staff1",
  },
];

describe("GroupTransferModal", () => {
  const mockOnClose = vi.fn();
  const mockOnTransfer = vi.fn();
  const mockOnCancelTransfer = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockOnTransfer.mockResolvedValue(undefined);
    mockOnCancelTransfer.mockResolvedValue(undefined);
  });

  it("renders nothing when group is null", () => {
    const { container } = render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={null}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("renders modal with group name in title", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    await waitFor(() => {
      expect(
        screen.getByText(/Gruppe "Gruppe A" übergeben/),
      ).toBeInTheDocument();
    });
  });

  it("displays group information", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Gruppe A")).toBeInTheDocument();
      expect(screen.getByText(/15 Kinder insgesamt/)).toBeInTheDocument();
    });
  });

  it("renders user dropdown with available users", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("John Doe")).toBeInTheDocument();
      expect(screen.getByText("Jane Smith")).toBeInTheDocument();
    });
  });

  it("shows existing transfers when provided", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
        existingTransfers={mockExistingTransfers}
        onCancelTransfer={mockOnCancelTransfer}
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Aktuell übergeben an:")).toBeInTheDocument();
      expect(screen.getByText("Mike Johnson")).toBeInTheDocument();
    });
  });

  it("calls onTransfer when transfer button is clicked", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    await waitFor(() => {
      const select = screen.getByRole("combobox");
      fireEvent.change(select, { target: { value: "p1" } });
    });

    const transferButton = screen.getByText("Übergeben");
    fireEvent.click(transferButton);

    await waitFor(() => {
      expect(mockOnTransfer).toHaveBeenCalledWith("p1", "John Doe");
    });
  });

  it("disables transfer button when no user is selected", () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    const transferButton = screen.getByText("Übergeben");
    expect(transferButton).toBeDisabled();
  });

  it("shows loading state during transfer", async () => {
    mockOnTransfer.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    await waitFor(() => {
      const select = screen.getByRole("combobox");
      fireEvent.change(select, { target: { value: "p1" } });
    });

    const transferButton = screen.getByText("Übergeben");
    fireEvent.click(transferButton);

    await waitFor(() => {
      expect(screen.getByText("Wird übergeben...")).toBeInTheDocument();
    });
  });

  it("calls onCancelTransfer when remove button is clicked", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
        existingTransfers={mockExistingTransfers}
        onCancelTransfer={mockOnCancelTransfer}
      />,
    );

    await waitFor(() => {
      const removeButton = screen.getByText("Entfernen");
      fireEvent.click(removeButton);
    });

    await waitFor(() => {
      expect(mockOnCancelTransfer).toHaveBeenCalledWith("s1");
    });
  });

  it("disables transfer button when no users available", () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={[]}
        onTransfer={mockOnTransfer}
      />,
    );

    const transferButton = screen.getByText("Übergeben");
    expect(transferButton).toBeDisabled();
  });

  it("closes modal when cancel is clicked", async () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={mockAvailableUsers}
        onTransfer={mockOnTransfer}
      />,
    );

    const cancelButton = screen.getByText("Abbrechen");
    fireEvent.click(cancelButton);

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });
});
