import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentGuardianManager from "./student-guardian-manager";
import type { GuardianWithRelationship } from "@/lib/guardian-helpers";

// Mock all guardian API functions with proper typing
const mockFetchStudentGuardians = vi.fn();
const mockCreateGuardian = vi.fn();
const mockUpdateGuardian = vi.fn();
const mockLinkGuardianToStudent = vi.fn();
const mockUpdateStudentGuardianRelationship = vi.fn();
const mockRemoveGuardianFromStudent = vi.fn();
const mockAddGuardianPhoneNumber = vi.fn();
const mockUpdateGuardianPhoneNumber = vi.fn();
const mockDeleteGuardianPhoneNumber = vi.fn();
const mockSetGuardianPrimaryPhone = vi.fn();

/* eslint-disable @typescript-eslint/no-unsafe-return */
vi.mock("@/lib/guardian-api", () => ({
  fetchStudentGuardians: () => mockFetchStudentGuardians(),
  createGuardian: (data: unknown) => mockCreateGuardian(data),
  updateGuardian: (id: string, data: unknown) => mockUpdateGuardian(id, data),
  linkGuardianToStudent: (studentId: string, data: unknown) =>
    mockLinkGuardianToStudent(studentId, data),
  updateStudentGuardianRelationship: (relationshipId: string, data: unknown) =>
    mockUpdateStudentGuardianRelationship(relationshipId, data),
  removeGuardianFromStudent: (studentId: string, guardianId: string) =>
    mockRemoveGuardianFromStudent(studentId, guardianId),
  addGuardianPhoneNumber: (guardianId: string, data: unknown) =>
    mockAddGuardianPhoneNumber(guardianId, data),
  updateGuardianPhoneNumber: (
    guardianId: string,
    phoneId: string,
    data: unknown,
  ) => mockUpdateGuardianPhoneNumber(guardianId, phoneId, data),
  deleteGuardianPhoneNumber: (guardianId: string, phoneId: string) =>
    mockDeleteGuardianPhoneNumber(guardianId, phoneId),
  setGuardianPrimaryPhone: (guardianId: string, phoneId: string) =>
    mockSetGuardianPrimaryPhone(guardianId, phoneId),
}));
/* eslint-enable @typescript-eslint/no-unsafe-return */

// Mock toast context
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
  }),
}));

// Mock child components
vi.mock("./guardian-list", () => ({
  default: ({
    guardians,
    onEdit,
    onDelete,
    readOnly,
  }: {
    guardians: GuardianWithRelationship[];
    onEdit?: (g: GuardianWithRelationship) => void;
    onDelete?: (g: GuardianWithRelationship) => void;
    readOnly?: boolean;
  }) => (
    <div data-testid="guardian-list">
      <p data-testid="guardian-count">Guardians: {guardians.length}</p>
      {guardians.map((g) => (
        <div key={g.id} data-testid={`guardian-${g.id}`}>
          <span>{`${g.firstName ?? ""} ${g.lastName ?? ""}`}</span>
          {!readOnly && onEdit && (
            <button onClick={() => onEdit(g)} data-testid={`edit-${g.id}`}>
              Edit {g.id}
            </button>
          )}
          {!readOnly && onDelete && (
            <button onClick={() => onDelete(g)} data-testid={`delete-${g.id}`}>
              Delete {g.id}
            </button>
          )}
        </div>
      ))}
    </div>
  ),
}));

vi.mock("./guardian-form-modal", () => ({
  default: ({
    isOpen,
    onClose,
    onSubmit,
    mode,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSubmit: (
      data: Array<{
        id: string;
        guardianData: unknown;
        relationshipData: unknown;
        phoneNumbers?: unknown[];
      }>,
      onEntryCreated?: (entryId: string) => void,
    ) => Promise<void>;
    mode: "create" | "edit";
  }) =>
    isOpen ? (
      <div data-testid="guardian-form-modal">
        <h2>{mode === "create" ? "Create Guardian" : "Edit Guardian"}</h2>
        <button onClick={onClose} data-testid="close-modal">
          Close Modal
        </button>
        <button
          onClick={() =>
            onSubmit(
              [
                {
                  id: "test-id",
                  guardianData: { firstName: "Test", lastName: "Guardian" },
                  relationshipData: { relationshipType: "parent" },
                  phoneNumbers: [
                    {
                      phoneNumber: "+49 123 456",
                      phoneType: "mobile",
                      isPrimary: true,
                    },
                  ],
                },
              ],
              undefined,
            )
          }
          data-testid="submit-form"
        >
          Submit Form
        </button>
      </div>
    ) : null,
}));

vi.mock("./guardian-delete-modal", () => ({
  GuardianDeleteModal: ({
    isOpen,
    onClose,
    onConfirm,
    guardianName,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    guardianName: string;
  }) =>
    isOpen ? (
      <div data-testid="guardian-delete-modal">
        <p data-testid="delete-guardian-name">Delete {guardianName}?</p>
        <button onClick={onClose} data-testid="cancel-delete">
          Cancel
        </button>
        <button onClick={onConfirm} data-testid="confirm-delete">
          Confirm Delete
        </button>
      </div>
    ) : null,
}));

// Mock helper function - note: we only mock what we need
vi.mock("@/lib/guardian-helpers", async () => {
  // eslint-disable-next-line @typescript-eslint/consistent-type-imports
  const actual = await vi.importActual<typeof import("@/lib/guardian-helpers")>(
    "@/lib/guardian-helpers",
  );
  return {
    ...actual,
    getGuardianFullName: (g: { firstName?: string; lastName?: string }) =>
      `${g.firstName ?? ""} ${g.lastName ?? ""}`.trim(),
  };
});

const mockGuardians: GuardianWithRelationship[] = [
  {
    id: "guardian-1",
    firstName: "Anna",
    lastName: "Müller",
    email: "anna@example.com",
    preferredContactMethod: "email",
    languagePreference: "de",
    hasAccount: false,
    phoneNumbers: [
      {
        id: "phone-1",
        phoneNumber: "+49 123 456",
        phoneType: "mobile",
        isPrimary: true,
        priority: 1,
      },
    ],
    relationshipId: "rel-1",
    relationshipType: "mother",
    isPrimary: true,
    isEmergencyContact: true,
    canPickup: true,
    emergencyPriority: 1,
  },
  {
    id: "guardian-2",
    firstName: "Hans",
    lastName: "Müller",
    email: "hans@example.com",
    preferredContactMethod: "phone",
    languagePreference: "de",
    hasAccount: false,
    phoneNumbers: [],
    relationshipId: "rel-2",
    relationshipType: "father",
    isPrimary: false,
    isEmergencyContact: false,
    canPickup: true,
    emergencyPriority: 2,
  },
];

describe("StudentGuardianManager", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchStudentGuardians.mockResolvedValue(mockGuardians);
  });

  describe("Initial Loading", () => {
    it("shows loading spinner on initial load", () => {
      mockFetchStudentGuardians.mockImplementation(
        () =>
          new Promise(() => {
            // This promise never resolves, keeping component in loading state
          }),
      );

      const { container } = render(
        <StudentGuardianManager studentId="student-123" />,
      );

      // Look for the animate-spin class on the loading spinner
      expect(container.querySelector(".animate-spin")).toBeInTheDocument();
    });

    it("fetches guardians on mount", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(mockFetchStudentGuardians).toHaveBeenCalledTimes(1);
      });
    });

    it("displays guardians after loading", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("guardian-list")).toBeInTheDocument();
      });

      expect(screen.getByTestId("guardian-count")).toHaveTextContent(
        "Guardians: 2",
      );
    });
  });

  describe("Error Handling", () => {
    it("displays error message when fetch fails", async () => {
      mockFetchStudentGuardians.mockRejectedValue(
        new Error("Failed to fetch guardians"),
      );

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByText("Failed to fetch guardians"),
        ).toBeInTheDocument();
      });
    });

    it("displays generic error message for non-Error objects", async () => {
      mockFetchStudentGuardians.mockRejectedValue("Unknown error");

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByText("Fehler beim Laden der Erziehungsberechtigten"),
        ).toBeInTheDocument();
      });
    });
  });

  describe("Header and Controls", () => {
    it("displays component title", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByText("Erziehungsberechtigte")).toBeInTheDocument();
      });
    });

    it("shows add button when not readOnly", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Erziehungsberechtigte/n hinzufügen"),
        ).toBeInTheDocument();
      });
    });

    it("shows readonly badge when readOnly is true", async () => {
      render(
        <StudentGuardianManager studentId="student-123" readOnly={true} />,
      );

      await waitFor(() => {
        expect(screen.getByText("Nur Ansicht")).toBeInTheDocument();
      });
    });

    it("hides add button when readOnly is true", async () => {
      render(
        <StudentGuardianManager studentId="student-123" readOnly={true} />,
      );

      await waitFor(() => {
        expect(screen.getByTestId("guardian-list")).toBeInTheDocument();
      });

      expect(
        screen.queryByTitle("Erziehungsberechtigte/n hinzufügen"),
      ).not.toBeInTheDocument();
    });
  });

  describe("Create Guardian Flow", () => {
    it("opens create modal when add button is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Erziehungsberechtigte/n hinzufügen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle("Erziehungsberechtigte/n hinzufügen"));

      expect(screen.getByTestId("guardian-form-modal")).toBeInTheDocument();
      expect(screen.getByText("Create Guardian")).toBeInTheDocument();
    });

    it("creates guardian with phone numbers and shows success toast", async () => {
      mockCreateGuardian.mockResolvedValue({ id: "new-guardian-1" });
      mockLinkGuardianToStudent.mockResolvedValue(undefined);
      mockAddGuardianPhoneNumber.mockResolvedValue({ id: "new-phone-1" });

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Erziehungsberechtigte/n hinzufügen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle("Erziehungsberechtigte/n hinzufügen"));
      fireEvent.click(screen.getByTestId("submit-form"));

      await waitFor(() => {
        expect(mockCreateGuardian).toHaveBeenCalledWith({
          firstName: "Test",
          lastName: "Guardian",
        });
      });

      expect(mockLinkGuardianToStudent).toHaveBeenCalledWith("student-123", {
        guardianProfileId: "new-guardian-1",
        relationshipType: "parent",
      });

      expect(mockAddGuardianPhoneNumber).toHaveBeenCalledWith(
        "new-guardian-1",
        {
          phoneNumber: "+49 123 456",
          phoneType: "mobile",
          isPrimary: true,
          label: undefined,
        },
      );

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Erziehungsberechtigte/r erfolgreich hinzugefügt",
        );
      });
    });

    it("calls onUpdate callback after successful creation", async () => {
      const mockOnUpdate = vi.fn();
      mockCreateGuardian.mockResolvedValue({ id: "new-guardian-1" });
      mockLinkGuardianToStudent.mockResolvedValue(undefined);
      mockAddGuardianPhoneNumber.mockResolvedValue({ id: "new-phone-1" });

      render(
        <StudentGuardianManager
          studentId="student-123"
          onUpdate={mockOnUpdate}
        />,
      );

      await waitFor(() => {
        expect(
          screen.getByTitle("Erziehungsberechtigte/n hinzufügen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle("Erziehungsberechtigte/n hinzufügen"));
      fireEvent.click(screen.getByTestId("submit-form"));

      await waitFor(() => {
        expect(mockOnUpdate).toHaveBeenCalled();
      });
    });
  });

  describe("Edit Guardian Flow", () => {
    it("opens edit modal when edit button is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("edit-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("edit-guardian-1"));

      expect(screen.getByTestId("guardian-form-modal")).toBeInTheDocument();
      expect(screen.getByText("Edit Guardian")).toBeInTheDocument();
    });

    it("updates guardian and shows success toast", async () => {
      mockUpdateGuardian.mockResolvedValue(undefined);
      mockUpdateStudentGuardianRelationship.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("edit-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("edit-guardian-1"));
      fireEvent.click(screen.getByTestId("submit-form"));

      await waitFor(() => {
        expect(mockUpdateGuardian).toHaveBeenCalledWith("guardian-1", {
          firstName: "Test",
          lastName: "Guardian",
        });
      });

      expect(mockUpdateStudentGuardianRelationship).toHaveBeenCalledWith(
        "rel-1",
        {
          relationshipType: "parent",
        },
      );

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Erziehungsberechtigte/r erfolgreich aktualisiert",
        );
      });
    });

    it("does not show edit buttons when readOnly", async () => {
      render(
        <StudentGuardianManager studentId="student-123" readOnly={true} />,
      );

      await waitFor(() => {
        expect(screen.getByTestId("guardian-list")).toBeInTheDocument();
      });

      expect(screen.queryByTestId("edit-guardian-1")).not.toBeInTheDocument();
      expect(screen.queryByTestId("edit-guardian-2")).not.toBeInTheDocument();
    });
  });

  describe("Delete Guardian Flow", () => {
    it("opens delete modal when delete button is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("delete-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("delete-guardian-1"));

      expect(screen.getByTestId("guardian-delete-modal")).toBeInTheDocument();
      expect(screen.getByTestId("delete-guardian-name")).toHaveTextContent(
        "Delete Anna Müller?",
      );
    });

    it("deletes guardian and shows success toast", async () => {
      mockRemoveGuardianFromStudent.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("delete-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("delete-guardian-1"));
      fireEvent.click(screen.getByTestId("confirm-delete"));

      await waitFor(() => {
        expect(mockRemoveGuardianFromStudent).toHaveBeenCalledWith(
          "student-123",
          "guardian-1",
        );
      });

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Anna Müller wurde erfolgreich entfernt",
        );
      });
    });

    it("closes delete modal when cancel is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("delete-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("delete-guardian-1"));

      expect(screen.getByTestId("guardian-delete-modal")).toBeInTheDocument();

      fireEvent.click(screen.getByTestId("cancel-delete"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-delete-modal"),
        ).not.toBeInTheDocument();
      });
    });

    it("does not show delete buttons when readOnly", async () => {
      render(
        <StudentGuardianManager studentId="student-123" readOnly={true} />,
      );

      await waitFor(() => {
        expect(screen.getByTestId("guardian-list")).toBeInTheDocument();
      });

      expect(screen.queryByTestId("delete-guardian-1")).not.toBeInTheDocument();
      expect(screen.queryByTestId("delete-guardian-2")).not.toBeInTheDocument();
    });
  });

  describe("Modal Close Behavior", () => {
    it("closes create modal when close button is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Erziehungsberechtigte/n hinzufügen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle("Erziehungsberechtigte/n hinzufügen"));

      expect(screen.getByTestId("guardian-form-modal")).toBeInTheDocument();

      fireEvent.click(screen.getByTestId("close-modal"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-form-modal"),
        ).not.toBeInTheDocument();
      });
    });

    it("closes edit modal when close button is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("edit-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("edit-guardian-1"));

      expect(screen.getByTestId("guardian-form-modal")).toBeInTheDocument();

      fireEvent.click(screen.getByTestId("close-modal"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-form-modal"),
        ).not.toBeInTheDocument();
      });
    });
  });

  describe("Empty State", () => {
    it("renders with no guardians", async () => {
      mockFetchStudentGuardians.mockResolvedValue([]);

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(screen.getByTestId("guardian-list")).toBeInTheDocument();
      });

      expect(screen.getByTestId("guardian-count")).toHaveTextContent(
        "Guardians: 0",
      );
    });
  });

  describe("Refetch on studentId Change", () => {
    it("refetches guardians when studentId changes", async () => {
      const { rerender } = render(
        <StudentGuardianManager studentId="student-123" />,
      );

      await waitFor(() => {
        expect(mockFetchStudentGuardians).toHaveBeenCalledTimes(1);
      });

      rerender(<StudentGuardianManager studentId="student-456" />);

      await waitFor(() => {
        expect(mockFetchStudentGuardians).toHaveBeenCalledTimes(2);
      });
    });
  });
});
