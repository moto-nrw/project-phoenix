import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import LinkGuardianModal from "./link-guardian-modal";
import type { GuardianSearchResult } from "@/lib/guardian-helpers";

// Mock the Modal component
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
        <h1>{title}</h1>
        <button onClick={onClose} data-testid="close-modal">
          Close
        </button>
        {children}
      </div>
    ) : null,
}));

// Mock the SearchableGuardianSelect component
vi.mock("./searchable-guardian-select", () => ({
  default: ({
    onSelect,
    excludeStudentId,
    disabled,
  }: {
    onSelect: (guardian: GuardianSearchResult) => void;
    excludeStudentId?: string;
    disabled?: boolean;
  }) => (
    <div data-testid="searchable-guardian-select">
      <span data-testid="exclude-student-id">{excludeStudentId}</span>
      <span data-testid="disabled">{disabled ? "true" : "false"}</span>
      <button
        onClick={() =>
          onSelect({
            id: "guardian-123",
            firstName: "Anna",
            lastName: "Müller",
            email: "anna@example.com",
            phone: "030-12345678",
            students: [
              {
                studentId: "101",
                firstName: "Max",
                lastName: "Müller",
                schoolClass: "1a",
              },
            ],
          })
        }
        data-testid="select-guardian"
      >
        Select Guardian
      </button>
    </div>
  ),
}));

describe("LinkGuardianModal", () => {
  const mockOnClose = vi.fn();
  const mockOnLink = vi.fn();
  const studentId = "student-456";

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when isOpen is false", () => {
    render(
      <LinkGuardianModal
        isOpen={false}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays search step initially", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    expect(
      screen.getByText("Bestehenden Erziehungsberechtigten verknüpfen"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("searchable-guardian-select")).toBeInTheDocument();
  });

  it("passes studentId as excludeStudentId to search component", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    expect(screen.getByTestId("exclude-student-id").textContent).toBe(
      studentId,
    );
  });

  it("shows configure step after selecting guardian", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    expect(
      screen.getByText("Beziehung konfigurieren"),
    ).toBeInTheDocument();
  });

  it("displays selected guardian info in configure step", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    expect(screen.getByText("Anna Müller")).toBeInTheDocument();
    expect(screen.getByText("anna@example.com")).toBeInTheDocument();
    expect(screen.getByText("030-12345678")).toBeInTheDocument();
  });

  it("shows linked students for selected guardian", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    expect(screen.getByText(/Bereits zugeordnet zu:/)).toBeInTheDocument();
    expect(screen.getByText(/Max Müller \(1a\)/)).toBeInTheDocument();
  });

  it("allows going back to search step", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    // Go to configure step
    fireEvent.click(screen.getByTestId("select-guardian"));
    expect(screen.getByText("Beziehung konfigurieren")).toBeInTheDocument();

    // Go back
    fireEvent.click(screen.getByText("Zurück zur Suche"));

    expect(
      screen.getByText("Bestehenden Erziehungsberechtigten verknüpfen"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("searchable-guardian-select")).toBeInTheDocument();
  });

  it("has relationship type dropdown defaulting to parent", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    const select = screen.getByLabelText("Beziehungstyp") as HTMLSelectElement;
    expect(select.value).toBe("parent");
  });

  it("has can pickup checkbox checked by default", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    const checkbox = screen.getByLabelText("Darf das Kind abholen");
    expect(checkbox).toBeChecked();
  });

  it("has primary contact checkbox unchecked by default", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    const checkbox = screen.getByLabelText("Hauptansprechpartner");
    expect(checkbox).not.toBeChecked();
  });

  it("has emergency contact checkbox unchecked by default", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    const checkbox = screen.getByLabelText("Notfallkontakt");
    expect(checkbox).not.toBeChecked();
  });

  it("allows toggling checkboxes", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    // Toggle primary
    const primaryCheckbox = screen.getByLabelText("Hauptansprechpartner");
    fireEvent.click(primaryCheckbox);
    expect(primaryCheckbox).toBeChecked();

    // Toggle emergency
    const emergencyCheckbox = screen.getByLabelText("Notfallkontakt");
    fireEvent.click(emergencyCheckbox);
    expect(emergencyCheckbox).toBeChecked();

    // Toggle can pickup (starts checked)
    const pickupCheckbox = screen.getByLabelText("Darf das Kind abholen");
    fireEvent.click(pickupCheckbox);
    expect(pickupCheckbox).not.toBeChecked();
  });

  it("calls onLink with correct data when form is submitted", async () => {
    mockOnLink.mockResolvedValue(undefined);

    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    // Configure the relationship
    fireEvent.click(screen.getByLabelText("Hauptansprechpartner"));
    fireEvent.click(screen.getByLabelText("Notfallkontakt"));

    // Submit the form
    fireEvent.click(screen.getByText("Verknüpfen"));

    await waitFor(() => {
      expect(mockOnLink).toHaveBeenCalledWith("guardian-123", {
        relationshipType: "parent",
        isPrimary: true,
        isEmergencyContact: true,
        canPickup: true,
        emergencyPriority: 1,
      });
    });
  });

  it("calls onClose after successful link", async () => {
    mockOnLink.mockResolvedValue(undefined);

    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));
    fireEvent.click(screen.getByText("Verknüpfen"));

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it("shows loading state during submission", async () => {
    let resolveLink: () => void;
    mockOnLink.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveLink = resolve;
        }),
    );

    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));
    fireEvent.click(screen.getByText("Verknüpfen"));

    expect(screen.getByText("Wird verknüpft...")).toBeInTheDocument();

    resolveLink!();

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  it("shows error message on link failure", async () => {
    mockOnLink.mockRejectedValue(new Error("Verknüpfung fehlgeschlagen"));

    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));
    fireEvent.click(screen.getByText("Verknüpfen"));

    await waitFor(() => {
      expect(screen.getByText("Verknüpfung fehlgeschlagen")).toBeInTheDocument();
    });

    // onClose should not be called on error
    expect(mockOnClose).not.toHaveBeenCalled();
  });

  it("shows generic error message for non-Error exceptions", async () => {
    mockOnLink.mockRejectedValue("Unknown error");

    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));
    fireEvent.click(screen.getByText("Verknüpfen"));

    await waitFor(() => {
      expect(screen.getByText("Fehler beim Verknüpfen")).toBeInTheDocument();
    });
  });

  it("calls onClose when cancel button is clicked in search step", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByText("Abbrechen"));

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("calls onClose when cancel button is clicked in configure step", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    // Find the cancel button (there are two with text "Abbrechen")
    const cancelButtons = screen.getAllByText("Abbrechen");
    fireEvent.click(cancelButtons[0]!);

    expect(mockOnClose).toHaveBeenCalled();
  });

  it("resets state when modal reopens", () => {
    const { rerender } = render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    // Go to configure step
    fireEvent.click(screen.getByTestId("select-guardian"));
    expect(screen.getByText("Beziehung konfigurieren")).toBeInTheDocument();

    // Close modal
    rerender(
      <LinkGuardianModal
        isOpen={false}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    // Reopen modal
    rerender(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    // Should be back to search step
    expect(
      screen.getByText("Bestehenden Erziehungsberechtigten verknüpfen"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("searchable-guardian-select")).toBeInTheDocument();
  });

  it("allows changing relationship type", () => {
    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));

    const select = screen.getByLabelText("Beziehungstyp") as HTMLSelectElement;
    fireEvent.change(select, { target: { value: "guardian" } });

    expect(select.value).toBe("guardian");
  });

  it("disables form elements during submission", async () => {
    let resolveLink: () => void;
    mockOnLink.mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveLink = resolve;
        }),
    );

    render(
      <LinkGuardianModal
        isOpen={true}
        onClose={mockOnClose}
        onLink={mockOnLink}
        studentId={studentId}
      />,
    );

    fireEvent.click(screen.getByTestId("select-guardian"));
    fireEvent.click(screen.getByText("Verknüpfen"));

    // Checkboxes should be disabled during loading
    expect(screen.getByLabelText("Hauptansprechpartner")).toBeDisabled();
    expect(screen.getByLabelText("Darf das Kind abholen")).toBeDisabled();
    expect(screen.getByLabelText("Notfallkontakt")).toBeDisabled();
    expect(screen.getByLabelText("Beziehungstyp")).toBeDisabled();

    resolveLink!();

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled();
    });
  });
});
