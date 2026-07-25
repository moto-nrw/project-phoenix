import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import StudentGuardianManager from "./student-guardian-manager";
import type { GuardianWithRelationship } from "@/lib/guardian-helpers";

// Mock all guardian API functions with proper typing
const mockFetchStudentGuardians = vi.fn();
const mockCreateGuardian = vi.fn();
const mockCreateStudentGuardians = vi.fn();
const mockUpdateGuardian = vi.fn();
const mockLinkGuardianToStudent = vi.fn();
const mockUpdateStudentGuardianRelationship = vi.fn();
const mockRemoveGuardianFromStudent = vi.fn();
const mockDeleteGuardian = vi.fn();
const mockFetchGuardianDeletePreview = vi.fn();
const mockAddGuardianPhoneNumber = vi.fn();
const mockUpdateGuardianPhoneNumber = vi.fn();
const mockDeleteGuardianPhoneNumber = vi.fn();
const mockSetGuardianPrimaryPhone = vi.fn();
const mockFetchOpenInvitationsByGuardian = vi.fn();
const mockFetchInvitationDelivery = vi.fn();

// Real subclass exported from the guardian-api mock so any code path that
// branches on `err instanceof GuardianApiError` still resolves against the
// mocked module. Built via vi.hoisted so it exists before the hoisted vi.mock
// factory runs.
const { mockGuardianApiError } = vi.hoisted(() => ({
  mockGuardianApiError: class extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = "GuardianApiError";
      this.status = status;
    }
  },
}));

vi.mock("@/lib/guardian-api", () => ({
  fetchStudentGuardians: () => mockFetchStudentGuardians(),
  createGuardian: (data: unknown) => mockCreateGuardian(data),
  createStudentGuardians: (studentId: string, guardians: unknown) =>
    mockCreateStudentGuardians(studentId, guardians),
  updateGuardian: (id: string, data: unknown) => mockUpdateGuardian(id, data),
  linkGuardianToStudent: (studentId: string, data: unknown) =>
    mockLinkGuardianToStudent(studentId, data),
  updateStudentGuardianRelationship: (relationshipId: string, data: unknown) =>
    mockUpdateStudentGuardianRelationship(relationshipId, data),
  removeGuardianFromStudent: (studentId: string, guardianId: string) =>
    mockRemoveGuardianFromStudent(studentId, guardianId),
  deleteGuardian: (
    guardianId: string,
    opts?: { force?: boolean; expectedAffectedLinkIds?: readonly string[] },
  ) => mockDeleteGuardian(guardianId, opts),
  fetchGuardianDeletePreview: (guardianId: string) =>
    mockFetchGuardianDeletePreview(guardianId),
  GuardianApiError: mockGuardianApiError,
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
  fetchOpenInvitationsByGuardian: () => mockFetchOpenInvitationsByGuardian(),
  fetchInvitationDelivery: (invitationId: string) =>
    mockFetchInvitationDelivery(invitationId),
}));

// Session mock — drives the admin-only "Komplett löschen" affordance. Default
// is a non-admin session; tests needing the full-delete path push an admin
// wildcard into mockPermissions before rendering.
let mockPermissions: string[] = [];
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: mockPermissions } },
    status: "authenticated",
  }),
}));

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
    readOnly,
    deliveryByGuardianId,
  }: {
    guardians: GuardianWithRelationship[];
    onEdit?: (g: GuardianWithRelationship) => void;
    readOnly?: boolean;
    deliveryByGuardianId?: ReadonlyMap<string, { label: string }>;
  }) => (
    <div data-testid="guardian-list">
      <p data-testid="guardian-count">Guardians: {guardians.length}</p>
      {guardians.map((g) => (
        <div key={g.id} data-testid={`guardian-${g.id}`}>
          <span>{`${g.firstName ?? ""} ${g.lastName ?? ""}`}</span>
          {deliveryByGuardianId?.get(g.id) && (
            <span data-testid={`delivery-${g.id}`}>
              {deliveryByGuardianId.get(g.id)!.label}
            </span>
          )}
          {!readOnly && onEdit && (
            <button
              type="button"
              onClick={() => onEdit(g)}
              data-testid={`edit-${g.id}`}
            >
              Edit {g.id}
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
    onDelete,
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
    onDelete?: () => void;
    mode: "create" | "edit";
  }) =>
    isOpen ? (
      <div data-testid="guardian-form-modal">
        <h2>{mode === "create" ? "Create Guardian" : "Edit Guardian"}</h2>
        <button type="button" onClick={onClose} data-testid="close-modal">
          Close Modal
        </button>
        <button
          type="button"
          onClick={() =>
            onSubmit(
              [
                {
                  id: "test-id",
                  guardianData: { firstName: "Test", lastName: "Guardian" },
                  relationshipData: {
                    relationshipType: "parent",
                    guardianRole: "legal_guardian",
                    isPrimary: false,
                    isEmergencyContact: false,
                    canPickup: false,
                    emergencyPriority: 1,
                  },
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
        {mode === "edit" && onDelete && (
          <button
            type="button"
            onClick={onDelete}
            data-testid="modal-delete-button"
          >
            Delete Guardian
          </button>
        )}
      </div>
    ) : null,
}));

// Mock the inline existing-guardian picker panel. It's mounted only while the
// picker is open, so it renders here whenever present and exposes one button
// that fires onSelect with a canned EXISTING guardian + relationship (the shape
// the real panel emits). This exercises the manager's link-existing wiring
// without driving the real search UI. A second button exercises onCancel.
vi.mock("./guardian-picker-panel", () => ({
  default: ({
    onSelect,
    onCancel,
  }: {
    onSelect: (guardian: unknown, relationship: unknown) => void;
    onCancel: () => void;
    excludeProfileIds?: readonly string[];
  }) => (
    <div data-testid="guardian-picker-panel">
      <button
        type="button"
        data-testid="picker-select"
        onClick={() =>
          onSelect(
            {
              id: "777",
              firstName: "Hans",
              lastName: "Schmidt",
              email: "hans@example.com",
              phoneNumbers: [],
              preferredContactMethod: "",
              languagePreference: "",
              hasAccount: false,
            },
            {
              relationshipType: "parent",
              isPrimary: false,
              isEmergencyContact: false,
              canPickup: true,
              emergencyPriority: 1,
            },
          )
        }
      >
        MockPickExisting
      </button>
      <button type="button" data-testid="picker-cancel" onClick={onCancel}>
        MockCancelPicker
      </button>
    </div>
  ),
}));

vi.mock("./guardian-delete-modal", () => ({
  GuardianDeleteModal: ({
    isOpen,
    onClose,
    onSelectUnlink,
    onSelectFullDelete,
    onConfirmUnlink,
    onConfirmFullDelete,
    onBack,
    guardianName,
    step,
    canFullDelete,
    fullDeleteWarning,
    isWarningLoading,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSelectUnlink: () => void;
    onSelectFullDelete: () => void;
    onConfirmUnlink: () => void;
    onConfirmFullDelete: () => void;
    onBack: () => void;
    guardianName: string;
    step?: "choose" | "confirm-unlink" | "confirm-full";
    canFullDelete?: boolean;
    fullDeleteWarning?: string | null;
    isWarningLoading?: boolean;
  }) =>
    isOpen ? (
      <div data-testid="guardian-delete-modal" data-step={step}>
        <p data-testid="delete-guardian-name">Delete {guardianName}?</p>
        {fullDeleteWarning && (
          <p data-testid="full-delete-warning">{fullDeleteWarning}</p>
        )}
        <button type="button" onClick={onClose} data-testid="cancel-delete">
          Cancel
        </button>
        {step === "choose" && (
          <>
            <button
              type="button"
              onClick={onSelectUnlink}
              data-testid="select-unlink"
            >
              Choose Unlink
            </button>
            {canFullDelete && (
              <button
                type="button"
                onClick={onSelectFullDelete}
                data-testid="select-full-delete"
              >
                Choose Full Delete
              </button>
            )}
          </>
        )}
        {step === "confirm-unlink" && (
          <>
            <button type="button" onClick={onBack} data-testid="back">
              Back
            </button>
            <button
              type="button"
              onClick={onConfirmUnlink}
              data-testid="confirm-delete"
            >
              Confirm Unlink
            </button>
          </>
        )}
        {step === "confirm-full" && (
          <>
            <button type="button" onClick={onBack} data-testid="back">
              Back
            </button>
            <button
              type="button"
              onClick={onConfirmFullDelete}
              data-testid="confirm-full-delete"
              disabled={isWarningLoading}
            >
              Confirm Full Delete
            </button>
          </>
        )}
      </div>
    ) : null,
}));

// Mock helper function - note: we only mock what we need
vi.mock("@/lib/guardian-helpers", async () => {
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
    mockCreateStudentGuardians.mockResolvedValue(undefined);
    // Deterministic default: phone adds (used by the edit flow) resolve to a row
    // with an id. clearAllMocks() clears call records but NOT implementations, so
    // setting this here avoids depending on another test's leaked mockResolvedValue.
    mockAddGuardianPhoneNumber.mockResolvedValue({ id: "new-phone-1" });
    mockFetchOpenInvitationsByGuardian.mockResolvedValue(new Map());
    mockFetchInvitationDelivery.mockResolvedValue({
      invitationId: "invitation-1",
      attempts: [],
    });
    mockPermissions = [];
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

    it("refreshes invitation delivery status while an invitation is open", async () => {
      vi.useFakeTimers();
      mockFetchOpenInvitationsByGuardian.mockResolvedValue(
        new Map([["guardian-1", "invitation-1"]]),
      );
      mockFetchInvitationDelivery
        .mockResolvedValueOnce({
          invitationId: "invitation-1",
          attempts: [
            {
              outboxId: "outbox-1",
              dispatchStatus: "pending",
              deliveryStatus: "unknown",
              queuedAt: "2026-07-25T10:31:02Z",
              attempts: 0,
              events: [],
            },
          ],
        })
        .mockResolvedValue({
          invitationId: "invitation-1",
          attempts: [
            {
              outboxId: "outbox-1",
              dispatchStatus: "sent",
              deliveryStatus: "delivered",
              queuedAt: "2026-07-25T10:31:02Z",
              sentAt: "2026-07-25T10:31:03Z",
              deliveryStatusAt: "2026-07-25T10:31:04Z",
              attempts: 1,
              events: [],
            },
          ],
        });

      const view = render(<StudentGuardianManager studentId="student-123" />);

      try {
        await act(async () => {
          await vi.advanceTimersByTimeAsync(0);
        });
        expect(screen.getByTestId("delivery-guardian-1")).toHaveTextContent(
          "Einladung eingeplant",
        );

        await act(async () => {
          await vi.advanceTimersByTimeAsync(10_000);
        });

        expect(mockFetchInvitationDelivery).toHaveBeenCalledTimes(2);
        expect(screen.getByTestId("delivery-guardian-1")).toHaveTextContent(
          "Zugestellt",
        );
      } finally {
        view.unmount();
        vi.useRealTimers();
      }
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

    it("creates guardian with phone numbers atomically and shows success toast", async () => {
      // The create flow now goes through ONE atomic backend call
      // (createStudentGuardians) instead of separate create→link→add-phones
      // calls — the whole batch succeeds or rolls back server-side (#819), so
      // there is no client-side rollback to orphan a profile.
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Erziehungsberechtigte/n hinzufügen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTitle("Erziehungsberechtigte/n hinzufügen"));
      fireEvent.click(screen.getByTestId("submit-form"));

      await waitFor(() => {
        expect(mockCreateStudentGuardians).toHaveBeenCalledWith("student-123", [
          expect.objectContaining({
            firstName: "Test",
            lastName: "Guardian",
            relationshipType: "parent",
            phoneNumbers: [
              expect.objectContaining({
                phoneNumber: "+49 123 456",
                phoneType: "mobile",
                isPrimary: true,
              }),
            ],
          }),
        ]);
      });

      // No per-step orchestration any more.
      expect(mockCreateGuardian).not.toHaveBeenCalled();
      expect(mockLinkGuardianToStudent).not.toHaveBeenCalled();
      expect(mockAddGuardianPhoneNumber).not.toHaveBeenCalled();

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Erziehungsberechtigte/r erfolgreich hinzugefügt",
        );
      });
    });

    it("calls onUpdate callback after successful creation", async () => {
      const mockOnUpdate = vi.fn();

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
          guardianRole: "legal_guardian",
          isPrimary: false,
          isEmergencyContact: false,
          canPickup: false,
          emergencyPriority: 1,
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
    it("opens delete modal when delete button is clicked in edit modal", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      // First open edit modal
      await waitFor(() => {
        expect(screen.getByTestId("edit-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("edit-guardian-1"));

      // Click delete button in edit modal
      expect(screen.getByTestId("modal-delete-button")).toBeInTheDocument();
      fireEvent.click(screen.getByTestId("modal-delete-button"));

      // Delete confirmation modal should open
      expect(screen.getByTestId("guardian-delete-modal")).toBeInTheDocument();
      expect(screen.getByTestId("delete-guardian-name")).toHaveTextContent(
        "Delete Anna Müller?",
      );
    });

    it("deletes guardian and shows success toast", async () => {
      mockRemoveGuardianFromStudent.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);

      // First open edit modal
      await waitFor(() => {
        expect(screen.getByTestId("edit-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("edit-guardian-1"));

      // Click delete button in edit modal
      fireEvent.click(screen.getByTestId("modal-delete-button"));

      // Confirm deletion
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

      // First open edit modal
      await waitFor(() => {
        expect(screen.getByTestId("edit-guardian-1")).toBeInTheDocument();
      });

      fireEvent.click(screen.getByTestId("edit-guardian-1"));

      // Click delete button in edit modal
      fireEvent.click(screen.getByTestId("modal-delete-button"));

      expect(screen.getByTestId("guardian-delete-modal")).toBeInTheDocument();

      fireEvent.click(screen.getByTestId("cancel-delete"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-delete-modal"),
        ).not.toBeInTheDocument();
      });
    });

    it("does not show delete button in edit modal when readOnly", async () => {
      render(
        <StudentGuardianManager studentId="student-123" readOnly={true} />,
      );

      await waitFor(() => {
        expect(screen.getByTestId("guardian-list")).toBeInTheDocument();
      });

      // Edit buttons should not be shown in readOnly mode
      expect(screen.queryByTestId("edit-guardian-1")).not.toBeInTheDocument();
      expect(screen.queryByTestId("edit-guardian-2")).not.toBeInTheDocument();
    });
  });

  describe("Full Delete Flow", () => {
    async function openDeleteModal(guardianId = "guardian-1") {
      await waitFor(() => {
        expect(screen.getByTestId(`edit-${guardianId}`)).toBeInTheDocument();
      });
      fireEvent.click(screen.getByTestId(`edit-${guardianId}`));
      fireEvent.click(screen.getByTestId("modal-delete-button"));
      expect(screen.getByTestId("guardian-delete-modal")).toBeInTheDocument();
    }

    function deferred<T>() {
      let resolve!: (value: T) => void;
      const promise = new Promise<T>((res) => {
        resolve = res;
      });
      return { promise, resolve };
    }

    it("sends non-admins straight to the unlink confirmation, with no choice screen", async () => {
      render(<StudentGuardianManager studentId="student-123" />);
      await openDeleteModal();

      // No choice screen → goes directly to the unlink confirmation.
      expect(screen.getByTestId("guardian-delete-modal")).toHaveAttribute(
        "data-step",
        "confirm-unlink",
      );
      expect(
        screen.queryByTestId("select-full-delete"),
      ).not.toBeInTheDocument();
    });

    it("admins choosing unlink still pass through the unlink confirmation", async () => {
      mockPermissions = ["admin:*"];
      mockRemoveGuardianFromStudent.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);
      await openDeleteModal();

      // Admin starts on the choice screen.
      expect(screen.getByTestId("guardian-delete-modal")).toHaveAttribute(
        "data-step",
        "choose",
      );

      fireEvent.click(screen.getByTestId("select-unlink"));
      await waitFor(() => {
        expect(screen.getByTestId("guardian-delete-modal")).toHaveAttribute(
          "data-step",
          "confirm-unlink",
        );
      });

      fireEvent.click(screen.getByTestId("confirm-delete"));
      await waitFor(() => {
        expect(mockRemoveGuardianFromStudent).toHaveBeenCalledWith(
          "student-123",
          "guardian-1",
        );
      });
    });

    it("admins choose full delete, get the affected-children warning, then force delete", async () => {
      mockPermissions = ["admin:*"];
      // Read-only preview returns the affected children (no delete happens yet).
      mockFetchGuardianDeletePreview.mockResolvedValue({
        linkedCount: 2,
        affectedNames: ["Anna Müller", "Ben Müller"],
        affectedLinkIds: ["10", "20"],
        warning:
          "Die Person ist mit 2 Kindern verknüpft und wird bei allen entfernt: Anna Müller, Ben Müller.",
      });
      mockDeleteGuardian.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);
      await openDeleteModal();

      // Admin picks the full-delete option from the choice screen.
      fireEvent.click(screen.getByTestId("select-full-delete"));

      // The read-only preview ran and surfaced the warning — no destructive
      // delete was attempted while merely opening the confirmation.
      await waitFor(() => {
        expect(mockFetchGuardianDeletePreview).toHaveBeenCalledWith(
          "guardian-1",
        );
      });
      expect(mockDeleteGuardian).not.toHaveBeenCalled();
      await waitFor(() => {
        expect(screen.getByTestId("full-delete-warning")).toHaveTextContent(
          "mit 2 Kindern verknüpft",
        );
      });
      expect(screen.getByTestId("guardian-delete-modal")).toHaveAttribute(
        "data-step",
        "confirm-full",
      );

      // Confirm → force delete + success toast.
      fireEvent.click(screen.getByTestId("confirm-full-delete"));

      await waitFor(() => {
        expect(mockDeleteGuardian).toHaveBeenCalledWith("guardian-1", {
          force: true,
          expectedAffectedLinkIds: ["10", "20"],
        });
      });
      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Anna Müller wurde vollständig gelöscht",
        );
      });
    });

    it("can step back from the full-delete confirmation to the choice step", async () => {
      mockPermissions = ["admin:*"];
      mockFetchGuardianDeletePreview.mockResolvedValue({
        linkedCount: 1,
        affectedNames: ["Anna Müller"],
        affectedLinkIds: ["10"],
        warning:
          "Die Person ist nur mit diesem Kind verknüpft und wird mit dem Profil vollständig gelöscht.",
      });

      render(<StudentGuardianManager studentId="student-123" />);
      await openDeleteModal();

      fireEvent.click(screen.getByTestId("select-full-delete"));
      await waitFor(() => {
        expect(screen.getByTestId("guardian-delete-modal")).toHaveAttribute(
          "data-step",
          "confirm-full",
        );
      });

      fireEvent.click(screen.getByTestId("back"));
      await waitFor(() => {
        expect(screen.getByTestId("guardian-delete-modal")).toHaveAttribute(
          "data-step",
          "choose",
        );
      });
      // Choice screen is back, force delete was never called.
      expect(screen.getByTestId("select-unlink")).toBeInTheDocument();
      expect(mockDeleteGuardian).not.toHaveBeenCalledWith("guardian-1", {
        force: true,
      });
    });

    it("ignores stale full-delete previews from a previously selected guardian", async () => {
      mockPermissions = ["admin:*"];
      const guardianOnePreview = deferred<{
        linkedCount: number;
        affectedNames: string[];
        affectedLinkIds: string[];
        warning: string;
      }>();
      const guardianTwoPreview = deferred<{
        linkedCount: number;
        affectedNames: string[];
        affectedLinkIds: string[];
        warning: string;
      }>();
      mockFetchGuardianDeletePreview.mockImplementation((guardianId) => {
        if (guardianId === "guardian-1") return guardianOnePreview.promise;
        if (guardianId === "guardian-2") return guardianTwoPreview.promise;
        throw new Error(`Unexpected guardian id: ${guardianId}`);
      });
      mockDeleteGuardian.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);
      await openDeleteModal("guardian-1");

      fireEvent.click(screen.getByTestId("select-full-delete"));
      await waitFor(() => {
        expect(mockFetchGuardianDeletePreview).toHaveBeenCalledWith(
          "guardian-1",
        );
      });

      fireEvent.click(screen.getByTestId("cancel-delete"));
      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-delete-modal"),
        ).not.toBeInTheDocument();
      });

      await openDeleteModal("guardian-2");
      fireEvent.click(screen.getByTestId("select-full-delete"));
      await waitFor(() => {
        expect(mockFetchGuardianDeletePreview).toHaveBeenCalledWith(
          "guardian-2",
        );
      });

      await act(async () => {
        guardianOnePreview.resolve({
          linkedCount: 1,
          affectedNames: ["Anna Müller"],
          affectedLinkIds: ["10"],
          warning: "STALE guardian-1 warning",
        });
        await guardianOnePreview.promise;
      });

      expect(
        screen.queryByText("STALE guardian-1 warning"),
      ).not.toBeInTheDocument();
      expect(screen.getByTestId("confirm-full-delete")).toBeDisabled();

      await act(async () => {
        guardianTwoPreview.resolve({
          linkedCount: 2,
          affectedNames: ["Hans Müller", "Ben Müller"],
          affectedLinkIds: ["20", "30"],
          warning: "CURRENT guardian-2 warning",
        });
        await guardianTwoPreview.promise;
      });

      await waitFor(() => {
        expect(screen.getByTestId("full-delete-warning")).toHaveTextContent(
          "CURRENT guardian-2 warning",
        );
      });
      expect(screen.getByTestId("confirm-full-delete")).not.toBeDisabled();

      fireEvent.click(screen.getByTestId("confirm-full-delete"));
      await waitFor(() => {
        expect(mockDeleteGuardian).toHaveBeenCalledWith("guardian-2", {
          force: true,
          expectedAffectedLinkIds: ["20", "30"],
        });
      });
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

  describe("Select Existing Guardian Flow", () => {
    it("opens the picker when the search button is clicked", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(
        screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
      );

      expect(screen.getByTestId("guardian-picker-panel")).toBeInTheDocument();
    });

    it("links the picked existing guardian and shows a success toast", async () => {
      mockLinkGuardianToStudent.mockResolvedValue(undefined);

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(
        screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
      );
      fireEvent.click(screen.getByTestId("picker-select"));

      // The existing-guardian path links (never creates) with the profile id +
      // the relationship flags chosen for THIS child.
      await waitFor(() => {
        expect(mockLinkGuardianToStudent).toHaveBeenCalledWith("student-123", {
          guardianProfileId: "777",
          relationshipType: "parent",
          isPrimary: false,
          isEmergencyContact: false,
          canPickup: true,
          emergencyPriority: 1,
        });
      });
      // Never creates a profile or writes phone numbers on the existing path.
      expect(mockCreateGuardian).not.toHaveBeenCalled();
      expect(mockAddGuardianPhoneNumber).not.toHaveBeenCalled();

      await waitFor(() => {
        expect(mockToastSuccess).toHaveBeenCalledWith(
          "Hans Schmidt wurde erfolgreich hinzugefügt",
        );
      });
      // Picker closes after a selection.
      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-picker-panel"),
        ).not.toBeInTheDocument();
      });
    });

    it("shows a German error toast when linking the existing guardian fails", async () => {
      mockLinkGuardianToStudent.mockRejectedValue(
        new Error("guardian already linked"),
      );

      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(
        screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
      );
      fireEvent.click(screen.getByTestId("picker-select"));

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalledWith(
          "Fehler beim Verknüpfen der/des Erziehungsberechtigten",
        );
      });
      // A raw backend message must never reach the user.
      expect(mockToastError).not.toHaveBeenCalledWith(
        "guardian already linked",
      );
    });

    it("closes the picker without linking when cancelled", async () => {
      render(<StudentGuardianManager studentId="student-123" />);

      await waitFor(() => {
        expect(
          screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
        ).toBeInTheDocument();
      });

      fireEvent.click(
        screen.getByTitle("Vorhandene/n Erziehungsberechtigte/n suchen"),
      );
      expect(screen.getByTestId("guardian-picker-panel")).toBeInTheDocument();

      fireEvent.click(screen.getByTestId("picker-cancel"));

      await waitFor(() => {
        expect(
          screen.queryByTestId("guardian-picker-panel"),
        ).not.toBeInTheDocument();
      });
      expect(mockLinkGuardianToStudent).not.toHaveBeenCalled();
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
