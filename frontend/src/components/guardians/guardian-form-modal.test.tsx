import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import GuardianFormModal, {
  createEmptyEntry,
  toEntry,
  type GuardianEntry,
} from "./guardian-form-modal";
import type { GuardianWithRelationship } from "@/lib/guardian-helpers";

// Mock crypto.randomUUID for predictable test results
let uuidCounter = 0;
vi.stubGlobal("crypto", {
  randomUUID: () => `test-uuid-${++uuidCounter}`,
});

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

// Reset uuid counter before each test
beforeEach(() => {
  uuidCounter = 0;
});

describe("createEmptyEntry", () => {
  it("creates an entry with empty string values", () => {
    const entry = createEmptyEntry();

    expect(entry.firstName).toBe("");
    expect(entry.lastName).toBe("");
    expect(entry.email).toBe("");
    expect(entry.phone).toBe("");
    expect(entry.mobilePhone).toBe("");
  });

  it("creates an entry with default relationship type 'parent'", () => {
    const entry = createEmptyEntry();

    expect(entry.relationshipType).toBe("parent");
  });

  it("creates an entry with isEmergencyContact set to false", () => {
    const entry = createEmptyEntry();

    expect(entry.isEmergencyContact).toBe(false);
  });

  it("creates an entry with a unique id", () => {
    const entry = createEmptyEntry();

    expect(entry.id).toBe("test-uuid-1");
  });

  it("returns a valid GuardianEntry structure", () => {
    const entry = createEmptyEntry();

    const expectedKeys: (keyof GuardianEntry)[] = [
      "id",
      "firstName",
      "lastName",
      "email",
      "phone",
      "mobilePhone",
      "relationshipType",
      "isEmergencyContact",
      "isPrimary",
      "canPickup",
      "emergencyPriority",
    ];

    for (const key of expectedKeys) {
      expect(entry).toHaveProperty(key);
    }
  });

  it("creates entry with correct relationship flag defaults", () => {
    const entry = createEmptyEntry();

    expect(entry.isPrimary).toBe(false);
    expect(entry.canPickup).toBe(true);
    expect(entry.emergencyPriority).toBe(1);
  });
});

describe("toEntry", () => {
  it("converts GuardianWithRelationship to GuardianEntry", () => {
    const guardianData: GuardianWithRelationship = {
      id: "guardian-123",
      firstName: "Max",
      lastName: "Mustermann",
      email: "max@example.com",
      phone: "+49 123 456789",
      mobilePhone: "+49 170 1234567",
      preferredContactMethod: "email",
      languagePreference: "de",
      hasAccount: false,
      relationshipType: "parent",
      isEmergencyContact: true,
      isPrimary: true,
      canPickup: true,
      emergencyPriority: 1,
      relationshipId: "rel-456",
    };

    const entry = toEntry(guardianData);

    expect(entry.id).toBe("guardian-123");
    expect(entry.firstName).toBe("Max");
    expect(entry.lastName).toBe("Mustermann");
    expect(entry.email).toBe("max@example.com");
    expect(entry.phone).toBe("+49 123 456789");
    expect(entry.mobilePhone).toBe("+49 170 1234567");
    expect(entry.relationshipType).toBe("parent");
    expect(entry.isEmergencyContact).toBe(true);
  });

  it("preserves relationship flags from initialData for edit mode", () => {
    const guardianData: GuardianWithRelationship = {
      id: "guardian-123",
      firstName: "Test",
      lastName: "User",
      preferredContactMethod: "email",
      languagePreference: "de",
      hasAccount: false,
      relationshipType: "parent",
      isEmergencyContact: false,
      isPrimary: true,
      canPickup: false,
      emergencyPriority: 2,
      relationshipId: "rel-456",
    };

    const entry = toEntry(guardianData);

    // These should be preserved, not reset to defaults
    expect(entry.isPrimary).toBe(true);
    expect(entry.canPickup).toBe(false);
    expect(entry.emergencyPriority).toBe(2);
  });

  it("handles undefined optional fields with defaults", () => {
    // Using type assertion to test undefined handling
    const guardianData = {
      id: "guardian-123",
      firstName: undefined,
      lastName: undefined,
      email: undefined,
      phone: undefined,
      mobilePhone: undefined,
      preferredContactMethod: "email",
      languagePreference: "de",
      hasAccount: false,
      relationshipType: undefined,
      isEmergencyContact: undefined,
      isPrimary: false,
      canPickup: false,
      emergencyPriority: 1,
      relationshipId: "rel-456",
    } as unknown as GuardianWithRelationship;

    const entry = toEntry(guardianData);

    expect(entry.firstName).toBe("");
    expect(entry.lastName).toBe("");
    expect(entry.email).toBe("");
    expect(entry.phone).toBe("");
    expect(entry.mobilePhone).toBe("");
    expect(entry.relationshipType).toBe("parent");
    expect(entry.isEmergencyContact).toBe(false);
  });

  it("handles null values by converting to empty strings", () => {
    const guardianData = {
      id: "guardian-123",
      firstName: null,
      lastName: null,
      email: null,
      phone: null,
      mobilePhone: null,
      relationshipType: null,
      isEmergencyContact: null,
      isPrimary: false,
      canPickup: false,
      emergencyPriority: 1,
      relationshipId: "rel-456",
    } as unknown as GuardianWithRelationship;

    const entry = toEntry(guardianData);

    expect(entry.firstName).toBe("");
    expect(entry.lastName).toBe("");
    expect(entry.email).toBe("");
    expect(entry.phone).toBe("");
    expect(entry.mobilePhone).toBe("");
    expect(entry.relationshipType).toBe("parent");
    expect(entry.isEmergencyContact).toBe(false);
  });

  it("preserves the original id from guardian data", () => {
    const guardianData: GuardianWithRelationship = {
      id: "unique-guardian-id-999",
      firstName: "Test",
      lastName: "User",
      preferredContactMethod: "email",
      languagePreference: "de",
      hasAccount: false,
      relationshipType: "parent",
      isEmergencyContact: false,
      isPrimary: false,
      canPickup: false,
      emergencyPriority: 1,
      relationshipId: "rel-123",
    };

    const entry = toEntry(guardianData);

    expect(entry.id).toBe("unique-guardian-id-999");
  });

  it("handles different relationship types", () => {
    const types = [
      "parent",
      "grandparent",
      "sibling",
      "other",
      "legal_guardian",
    ];

    for (const type of types) {
      const guardianData: GuardianWithRelationship = {
        id: "guardian-123",
        firstName: "Test",
        lastName: "User",
        preferredContactMethod: "email",
        languagePreference: "de",
        hasAccount: false,
        relationshipType: type,
        isEmergencyContact: false,
        isPrimary: false,
        canPickup: false,
        emergencyPriority: 1,
        relationshipId: "rel-123",
      };

      const entry = toEntry(guardianData);
      expect(entry.relationshipType).toBe(type);
    }
  });
});

describe("GuardianFormModal", () => {
  const mockOnClose = vi.fn();
  const mockOnSubmit = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders nothing when isOpen is false", () => {
    render(
      <GuardianFormModal
        isOpen={false}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    expect(screen.queryByTestId("modal")).not.toBeInTheDocument();
  });

  it("renders modal when isOpen is true", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    expect(screen.getByTestId("modal")).toBeInTheDocument();
  });

  it("displays create title in create mode", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    expect(
      screen.getByText("Erziehungsberechtigte/n hinzufügen"),
    ).toBeInTheDocument();
  });

  it("displays edit title in edit mode", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="edit"
      />,
    );

    expect(
      screen.getByText("Erziehungsberechtigte/n bearbeiten"),
    ).toBeInTheDocument();
  });

  it("shows 'Weiteren hinzufügen' button in create mode", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    expect(screen.getByText("Weiteren hinzufügen")).toBeInTheDocument();
  });

  it("hides 'Weiteren hinzufügen' button in edit mode", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="edit"
      />,
    );

    expect(screen.queryByText("Weiteren hinzufügen")).not.toBeInTheDocument();
  });

  it("adds new entry when clicking 'Weiteren hinzufügen'", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Initially should have one first name input
    const initialInputs = screen.getAllByPlaceholderText("Max");
    expect(initialInputs.length).toBe(1);

    // Click "Weiteren hinzufügen"
    fireEvent.click(screen.getByText("Weiteren hinzufügen"));

    // Should now have two first name inputs
    const updatedInputs = screen.getAllByPlaceholderText("Max");
    expect(updatedInputs.length).toBe(2);
  });

  it("shows Person headers when multiple entries exist", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Initially no "Person 1" header
    expect(screen.queryByText("Person 1")).not.toBeInTheDocument();

    // Add another entry
    fireEvent.click(screen.getByText("Weiteren hinzufügen"));

    // Now should show "Person 1" and "Person 2"
    expect(screen.getByText("Person 1")).toBeInTheDocument();
    expect(screen.getByText("Person 2")).toBeInTheDocument();
  });

  it("shows remove buttons when multiple entries exist", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Initially no remove button
    expect(screen.queryByText("Entfernen")).not.toBeInTheDocument();

    // Add another entry
    fireEvent.click(screen.getByText("Weiteren hinzufügen"));

    // Now should show remove buttons
    const removeButtons = screen.getAllByText("Entfernen");
    expect(removeButtons.length).toBe(2);
  });

  it("removes entry when clicking remove button", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Add another entry
    fireEvent.click(screen.getByText("Weiteren hinzufügen"));

    // Should have 2 entries
    expect(screen.getAllByPlaceholderText("Max").length).toBe(2);

    // Remove first entry
    const removeButtons = screen.getAllByText("Entfernen");
    fireEvent.click(removeButtons[0]!);

    // Should have 1 entry now
    expect(screen.getAllByPlaceholderText("Max").length).toBe(1);
  });

  it("updates button text for multiple entries", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Initially shows "Hinzufügen"
    expect(screen.getByText("Hinzufügen")).toBeInTheDocument();

    // Add another entry
    fireEvent.click(screen.getByText("Weiteren hinzufügen"));

    // Should show "2 Personen hinzufügen"
    expect(screen.getByText("2 Personen hinzufügen")).toBeInTheDocument();
  });

  it("shows validation error when submitting empty form", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Submit empty form
    fireEvent.click(screen.getByText("Hinzufügen"));

    // Should show validation error
    await waitFor(() => {
      expect(
        screen.getByText("Vorname und Nachname sind erforderlich"),
      ).toBeInTheDocument();
    });

    // onSubmit should not be called
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it("shows contact validation error when no contact provided", async () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Fill in name but no contact
    const firstNameInput = screen.getByPlaceholderText("Max");
    const lastNameInput = screen.getByPlaceholderText("Mustermann");

    fireEvent.change(firstNameInput, { target: { value: "Test" } });
    fireEvent.change(lastNameInput, { target: { value: "User" } });

    // Submit form
    fireEvent.click(screen.getByText("Hinzufügen"));

    // Should show validation error
    await waitFor(() => {
      expect(
        screen.getByText("Mindestens eine Kontaktmöglichkeit ist erforderlich"),
      ).toBeInTheDocument();
    });

    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it("submits form with valid data", async () => {
    mockOnSubmit.mockResolvedValue(undefined);

    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Fill in valid data
    fireEvent.change(screen.getByPlaceholderText("Max"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByPlaceholderText("Mustermann"), {
      target: { value: "User" },
    });
    fireEvent.change(
      screen.getByPlaceholderText("max.mustermann@example.com"),
      { target: { value: "test@example.com" } },
    );

    // Submit form
    fireEvent.click(screen.getByText("Hinzufügen"));

    // Wait for submission
    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledTimes(1);
    });

    // Check submitted data - verify the call arguments directly
    const callArgs = mockOnSubmit.mock.calls[0] as [
      Array<{
        id: string;
        guardianData: { firstName: string; lastName: string; email?: string };
      }>,
      ((entryId: string) => void) | undefined,
    ];
    expect(callArgs[0]).toHaveLength(1);
    expect(callArgs[0][0]?.id).toBeDefined(); // Entry ID should be included
    expect(callArgs[0][0]?.guardianData.firstName).toBe("Test");
    expect(callArgs[0][0]?.guardianData.lastName).toBe("User");
    expect(callArgs[0][0]?.guardianData.email).toBe("test@example.com");
    expect(callArgs[1]).toBeInstanceOf(Function); // Callback should be passed
  });

  it("submits multiple guardians at once", async () => {
    mockOnSubmit.mockResolvedValue(undefined);

    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Fill in first guardian
    const firstNameInputs = screen.getAllByPlaceholderText("Max");
    const lastNameInputs = screen.getAllByPlaceholderText("Mustermann");
    const emailInputs = screen.getAllByPlaceholderText(
      "max.mustermann@example.com",
    );

    fireEvent.change(firstNameInputs[0]!, { target: { value: "First" } });
    fireEvent.change(lastNameInputs[0]!, { target: { value: "Guardian" } });
    fireEvent.change(emailInputs[0]!, {
      target: { value: "first@example.com" },
    });

    // Add second guardian
    fireEvent.click(screen.getByText("Weiteren hinzufügen"));

    // Fill in second guardian
    const updatedFirstNameInputs = screen.getAllByPlaceholderText("Max");
    const updatedLastNameInputs = screen.getAllByPlaceholderText("Mustermann");
    const updatedEmailInputs = screen.getAllByPlaceholderText(
      "max.mustermann@example.com",
    );

    fireEvent.change(updatedFirstNameInputs[1]!, {
      target: { value: "Second" },
    });
    fireEvent.change(updatedLastNameInputs[1]!, {
      target: { value: "Guardian" },
    });
    fireEvent.change(updatedEmailInputs[1]!, {
      target: { value: "second@example.com" },
    });

    // Submit form
    fireEvent.click(screen.getByText("2 Personen hinzufügen"));

    // Wait for submission
    await waitFor(() => {
      expect(mockOnSubmit).toHaveBeenCalledTimes(1);
    });

    // Check submitted data - verify the call arguments directly
    const callArgs = mockOnSubmit.mock.calls[0] as [
      Array<{ id: string; guardianData: { firstName: string } }>,
      ((entryId: string) => void) | undefined,
    ];
    expect(callArgs[0]).toHaveLength(2);
    expect(callArgs[0][0]?.id).toBeDefined();
    expect(callArgs[0][0]?.guardianData.firstName).toBe("First");
    expect(callArgs[0][1]?.id).toBeDefined();
    expect(callArgs[0][1]?.guardianData.firstName).toBe("Second");
    expect(callArgs[1]).toBeInstanceOf(Function); // Callback should be passed
  });

  it("calls onClose when cancel button is clicked", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    fireEvent.click(screen.getByText("Abbrechen"));

    expect(mockOnClose).toHaveBeenCalledTimes(1);
  });

  it("toggles emergency contact checkbox", () => {
    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    const checkbox = screen.getByLabelText("Als Notfallkontakt markieren");
    expect(checkbox).not.toBeChecked();

    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
  });

  it("shows loading state during submission", async () => {
    // Make onSubmit slow
    mockOnSubmit.mockImplementation(
      () => new Promise((resolve) => setTimeout(resolve, 1000)),
    );

    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Fill in valid data
    fireEvent.change(screen.getByPlaceholderText("Max"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByPlaceholderText("Mustermann"), {
      target: { value: "User" },
    });
    fireEvent.change(
      screen.getByPlaceholderText("max.mustermann@example.com"),
      { target: { value: "test@example.com" } },
    );

    // Submit form
    fireEvent.click(screen.getByText("Hinzufügen"));

    // Should show loading state
    await waitFor(() => {
      expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
    });
  });

  it("prefills form with initial data in edit mode", () => {
    const initialData: GuardianWithRelationship = {
      id: "guardian-123",
      firstName: "Max",
      lastName: "Mustermann",
      email: "max@example.com",
      phone: "+49 123 456789",
      mobilePhone: "+49 170 1234567",
      preferredContactMethod: "email",
      languagePreference: "de",
      hasAccount: false,
      relationshipType: "parent",
      isEmergencyContact: true,
      isPrimary: true,
      canPickup: true,
      emergencyPriority: 1,
      relationshipId: "rel-456",
    };

    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="edit"
        initialData={initialData}
      />,
    );

    expect(screen.getByDisplayValue("Max")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Mustermann")).toBeInTheDocument();
    expect(screen.getByDisplayValue("max@example.com")).toBeInTheDocument();
    expect(screen.getByDisplayValue("+49 123 456789")).toBeInTheDocument();
    expect(screen.getByDisplayValue("+49 170 1234567")).toBeInTheDocument();
  });

  it("passes callback to remove entries on partial success", async () => {
    // Simulate partial success: first guardian succeeds, second fails
    let capturedCallback: ((entryId: string) => void) | undefined;
    mockOnSubmit.mockImplementation(
      async (
        _guardians: Array<{ id: string }>,
        onEntryCreated?: (entryId: string) => void,
      ) => {
        capturedCallback = onEntryCreated;
        throw new Error("Second guardian failed");
      },
    );

    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Fill in valid data for first entry
    fireEvent.change(screen.getByPlaceholderText("Max"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByPlaceholderText("Mustermann"), {
      target: { value: "User" },
    });
    fireEvent.change(
      screen.getByPlaceholderText("max.mustermann@example.com"),
      { target: { value: "test@example.com" } },
    );

    // Submit form
    fireEvent.click(screen.getByText("Hinzufügen"));

    // Wait for submission to fail
    await waitFor(() => {
      expect(screen.getByText("Second guardian failed")).toBeInTheDocument();
    });

    // Verify callback was passed
    expect(capturedCallback).toBeInstanceOf(Function);
  });

  it("shows error message on submit failure", async () => {
    mockOnSubmit.mockRejectedValue(new Error("API Error"));

    render(
      <GuardianFormModal
        isOpen={true}
        onClose={mockOnClose}
        onSubmit={mockOnSubmit}
        mode="create"
      />,
    );

    // Fill in valid data
    fireEvent.change(screen.getByPlaceholderText("Max"), {
      target: { value: "Test" },
    });
    fireEvent.change(screen.getByPlaceholderText("Mustermann"), {
      target: { value: "User" },
    });
    fireEvent.change(
      screen.getByPlaceholderText("max.mustermann@example.com"),
      { target: { value: "test@example.com" } },
    );

    // Submit form
    fireEvent.click(screen.getByText("Hinzufügen"));

    // Should show error message
    await waitFor(() => {
      expect(screen.getByText("API Error")).toBeInTheDocument();
    });
  });
});
