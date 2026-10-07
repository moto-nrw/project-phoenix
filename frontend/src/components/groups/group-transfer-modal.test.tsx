/**
 * Tests for GroupTransferModal Component
 * Tests the rendering and transfer functionality
 */
import { render, screen, waitFor, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { GroupTransferModal } from "./group-transfer-modal";

const mockGroup = {
  id: "1",
  name: "Gruppe A",
  studentCount: 15,
};

const mockAvailableUsers = [
  {
    id: "1",
    fullName: "John Doe",
  },
  {
    id: "2",
    fullName: "Jane Smith",
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

    const select = await screen.findByRole("combobox");
    fireEvent.click(select);

    await waitFor(() => {
      expect(
        screen.getByRole("option", { name: "John Doe" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("option", { name: "Jane Smith" }),
      ).toBeInTheDocument();
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

    const select = await screen.findByRole("combobox");
    fireEvent.click(select);
    fireEvent.click(screen.getByRole("option", { name: "John Doe" }));

    const transferButton = screen.getByText("Übergeben");
    fireEvent.click(transferButton);

    await waitFor(() => {
      expect(mockOnTransfer).toHaveBeenCalledWith("1", "John Doe");
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

    const select = await screen.findByRole("combobox");
    fireEvent.click(select);
    fireEvent.click(screen.getByRole("option", { name: "John Doe" }));

    const transferButton = screen.getByText("Übergeben");
    fireEvent.click(transferButton);

    await waitFor(() => {
      expect(screen.getByText("Wird übergeben...")).toBeInTheDocument();
    });
  });

  it("calls onCancelTransfer only after the confirmation", async () => {
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

    // #3109: taking a handover back asks first (ConfirmationModal).
    fireEvent.click(
      await screen.findByRole("button", { name: "Zurücknehmen" }),
    );
    expect(mockOnCancelTransfer).not.toHaveBeenCalled();
    expect(
      screen.getByRole("heading", { name: "Übergabe zurücknehmen?" }),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Übergabe zurücknehmen" }),
    );

    await waitFor(() => {
      expect(mockOnCancelTransfer).toHaveBeenCalledWith("s1");
    });
    await waitFor(() => {
      expect(
        screen.queryByRole("heading", { name: "Übergabe zurücknehmen?" }),
      ).not.toBeInTheDocument();
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

  it("shows a load failure instead of claiming that no staff are available", () => {
    render(
      <GroupTransferModal
        isOpen={true}
        onClose={mockOnClose}
        group={mockGroup}
        availableUsers={[]}
        loadError="Fachkräfte konnten nicht geladen werden."
        onTransfer={mockOnTransfer}
      />,
    );

    expect(
      screen.getByText("Fachkräfte konnten nicht geladen werden."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Keine pädagogische Fachkraft verfügbar/),
    ).not.toBeInTheDocument();
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

  describe("Scroll to error", () => {
    it("scrolls to error when transfer fails", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      mockOnTransfer.mockRejectedValue(
        new ApiError("boom", 500, { code: "general.server" }),
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

      // Select a user
      const select = await screen.findByRole("combobox");
      fireEvent.click(select);
      fireEvent.click(screen.getByRole("option", { name: "John Doe" }));

      const transferButton = screen.getByText("Übergeben");
      fireEvent.click(transferButton);

      await waitFor(() => {
        expect(
          screen.getByText(catalogText("general.server", "die Übergabe")),
        ).toBeInTheDocument();
      });
      expect(
        screen.getByRole("button", { name: "Wiederholen" }),
      ).toBeInTheDocument();

      await waitFor(() => {
        expect(scrollIntoViewMock).toHaveBeenCalledWith({
          behavior: "smooth",
          block: "nearest",
        });
      });
    });

    it("shows a typed server error", async () => {
      mockOnTransfer.mockRejectedValue(
        new ApiError("substitution already exists", 409, {
          code: "substitutions.already_assigned",
        }),
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

      fireEvent.click(await screen.findByRole("combobox"));
      fireEvent.click(screen.getByRole("option", { name: "John Doe" }));
      fireEvent.click(screen.getByText("Übergeben"));

      expect(
        await screen.findByText(
          catalogText("substitutions.already_assigned", "die Übergabe"),
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByText("substitution already exists"),
      ).not.toBeInTheDocument();
    });
  });
});
