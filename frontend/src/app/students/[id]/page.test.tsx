import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
  act,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import StudentDetailPage from "./page";

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token" } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockSearchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: mockPush,
    replace: vi.fn(),
    back: vi.fn(),
  })),
  useParams: vi.fn(() => ({ id: "1" })),
  useSearchParams: vi.fn(() => mockSearchParams),
}));

// Mock ResponsiveLayout
vi.mock("~/components/dashboard", () => ({
  ResponsiveLayout: ({
    children,
    studentName,
  }: {
    children: React.ReactNode;
    studentName?: string;
  }) => (
    <div data-testid="responsive-layout" data-student-name={studentName}>
      {children}
    </div>
  ),
}));

// Mock Loading component
vi.mock("~/components/ui/loading", () => ({
  Loading: ({ message }: { message?: string }) => (
    <div data-testid="loading">{message ?? "Loading..."}</div>
  ),
}));

// Mock Alert component
vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
}));

// Mock BackButton component
vi.mock("~/components/ui/back-button", () => ({
  BackButton: ({ referrer }: { referrer: string }) => (
    <button data-testid="back-button" data-referrer={referrer}>
      Zurück
    </button>
  ),
}));

// Mock ConfirmationModal
vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
    title,
    children,
    confirmText,
    isConfirmDisabled,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    children: React.ReactNode;
    confirmText: string;
    isConfirmDisabled?: boolean;
  }) =>
    isOpen ? (
      <div data-testid={`modal-${title.toLowerCase().replace(/\s+/g, "-")}`}>
        <h2>{title}</h2>
        <div data-testid="modal-content">{children}</div>
        <button data-testid="modal-cancel" onClick={onClose}>
          Abbrechen
        </button>
        <button
          data-testid="modal-confirm"
          onClick={onConfirm}
          disabled={isConfirmDisabled}
        >
          {confirmText}
        </button>
      </div>
    ) : null,
}));

// Mock student detail components
vi.mock("~/components/students/student-detail-components", () => ({
  StudentDetailHeader: ({
    student,
  }: {
    student: { name: string; school_class: string };
  }) => (
    <div data-testid="student-header">
      <h1 data-testid="student-name">{student.name}</h1>
      <span data-testid="student-class">{student.school_class}</span>
    </div>
  ),
  SupervisorsCard: ({
    supervisors,
    studentName,
  }: {
    supervisors: Array<{ name: string }>;
    studentName: string;
  }) => (
    <div data-testid="supervisors-card" data-student={studentName}>
      {supervisors.map((s: { name: string }, i: number) => (
        <span key={i}>{s.name}</span>
      ))}
    </div>
  ),
  PersonalInfoReadOnly: ({
    student,
  }: {
    student: { name: string; school_class: string };
  }) => (
    <div data-testid="personal-info-readonly">
      <span data-testid="readonly-name">{student.name}</span>
      <span data-testid="readonly-class">{student.school_class}</span>
    </div>
  ),
  FullAccessPersonalInfoReadOnly: ({
    student,
    onEditClick,
  }: {
    student: { name: string };
    onEditClick: () => void;
  }) => (
    <div data-testid="full-access-personal-info">
      <span data-testid="fullaccess-name">{student.name}</span>
      <button data-testid="edit-personal-info" onClick={onEditClick}>
        Bearbeiten
      </button>
    </div>
  ),
  StudentHistorySection: () => (
    <div data-testid="student-history">Historie</div>
  ),
}));

// Mock PersonalInfoEditForm
vi.mock("~/components/students/student-personal-info-form", () => ({
  PersonalInfoEditForm: ({
    editedStudent,
    onSave,
    onCancel,
  }: {
    editedStudent: { name: string };
    onSave: () => void;
    onCancel: () => void;
  }) => (
    <div data-testid="personal-info-edit-form">
      <span data-testid="edit-form-name">{editedStudent.name}</span>
      <button data-testid="save-personal-info" onClick={onSave}>
        Speichern
      </button>
      <button data-testid="cancel-edit" onClick={onCancel}>
        Abbrechen
      </button>
    </div>
  ),
}));

// Track whether checkout/checkin sections should be shown
let showCheckoutSection = false;
let showCheckinSection = false;

// Mock checkout section
vi.mock("~/components/students/student-checkout-section", () => ({
  StudentCheckoutSection: ({
    onCheckoutClick,
  }: {
    onCheckoutClick: () => void;
  }) => (
    <div data-testid="checkout-section">
      <button data-testid="checkout-button" onClick={onCheckoutClick}>
        Kind abmelden
      </button>
    </div>
  ),
  StudentCheckinSection: ({
    onCheckinClick,
  }: {
    onCheckinClick: () => void;
  }) => (
    <div data-testid="checkin-section">
      <button data-testid="checkin-button" onClick={onCheckinClick}>
        Kind anmelden
      </button>
    </div>
  ),
  getStudentActionType: vi.fn(() => (showCheckinSection ? "checkin" : "none")),
}));

// Mock guardian manager
vi.mock("~/components/guardians/student-guardian-manager", () => ({
  default: ({
    studentId,
    onUpdate,
  }: {
    studentId: string;
    onUpdate: () => void;
  }) => (
    <div data-testid="guardian-manager" data-student-id={studentId}>
      <button data-testid="update-guardians" onClick={onUpdate}>
        Update
      </button>
    </div>
  ),
}));

// Mock checkin API
const mockPerformImmediateCheckin = vi.fn();
vi.mock("~/lib/checkin-api", () => ({
  performImmediateCheckin: (studentId: number, activeGroupId: number) =>
    mockPerformImmediateCheckin(studentId, activeGroupId),
}));

// Mock useStudentData hook
const mockRefreshData = vi.fn();
const mockUseStudentData = vi.fn();
vi.mock("~/lib/hooks/use-student-data", () => ({
  useStudentData: (studentId: string) => mockUseStudentData(studentId),
  shouldShowCheckoutSection: vi.fn(() => showCheckoutSection),
}));

// Mock active service
const mockCheckoutStudent = vi.fn();
const mockGetActiveGroups = vi.fn();
vi.mock("~/lib/active-service", () => ({
  activeService: {
    checkoutStudent: (studentId: string) => mockCheckoutStudent(studentId),
    getActiveGroups: (params: { active: boolean }) =>
      mockGetActiveGroups(params),
  },
}));

// Mock student service
const mockUpdateStudent = vi.fn();
vi.mock("~/lib/api", () => ({
  studentService: {
    updateStudent: (id: string, data: unknown) => mockUpdateStudent(id, data),
  },
}));

// Test data
const mockStudent = {
  id: "1",
  first_name: "Max",
  second_name: "Mustermann",
  name: "Max Mustermann",
  school_class: "1a",
  group_id: "1",
  group_name: "Gruppe A",
  current_location: "Raum 101",
  bus: false,
  buskind: false,
  birthday: "2015-05-15",
  health_info: "",
  supervisor_notes: "",
  extra_info: "",
  pickup_status: "",
  sick: false,
};

const mockStudentAtHome = {
  ...mockStudent,
  current_location: "Zuhause",
};

describe("StudentDetailPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockSearchParams.delete("from");
    showCheckoutSection = false;
    showCheckinSection = false;

    // Default mock implementations
    mockUseStudentData.mockReturnValue({
      student: mockStudent,
      loading: false,
      error: null,
      hasFullAccess: true,
      supervisors: [{ name: "Frau Schmidt", phone: "0123456" }],
      myGroups: ["1"],
      myGroupRooms: ["Raum 101"],
      mySupervisedRooms: ["Raum 101"],
      refreshData: mockRefreshData,
    });

    mockGetActiveGroups.mockResolvedValue([
      {
        id: "1",
        room: { name: "Raum 101" },
        actualGroup: { name: "Gruppe A" },
      },
    ]);
  });

  afterEach(() => {
    cleanup();
  });

  describe("Loading State", () => {
    it("shows loading spinner while data is loading", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: true,
        error: null,
        hasFullAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("loading")).toBeInTheDocument();
      expect(screen.getByText("Laden...")).toBeInTheDocument();
    });
  });

  describe("Error State", () => {
    it("shows error message when fetching fails", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: "Schüler nicht gefunden",
        hasFullAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      expect(screen.getByText("Schüler nicht gefunden")).toBeInTheDocument();
    });

    it("shows error when student is null", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: null,
        hasFullAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });

    it("navigates back when back button is clicked in error state", async () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: "Error",
        hasFullAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      const backButton = screen.getByRole("button", { name: /zurück/i });
      fireEvent.click(backButton);

      expect(mockPush).toHaveBeenCalledWith("/students/search");
    });
  });

  describe("Full Access View", () => {
    it("renders student header with name", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("student-header")).toBeInTheDocument();
      expect(screen.getByTestId("student-name")).toHaveTextContent(
        "Max Mustermann",
      );
    });

    it("renders full access personal info section", () => {
      render(<StudentDetailPage />);

      expect(
        screen.getByTestId("full-access-personal-info"),
      ).toBeInTheDocument();
    });

    it("renders student history section", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("student-history")).toBeInTheDocument();
    });

    it("renders guardian manager", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("guardian-manager")).toBeInTheDocument();
      expect(screen.getByTestId("guardian-manager")).toHaveAttribute(
        "data-student-id",
        "1",
      );
    });
  });

  describe("Limited Access View", () => {
    beforeEach(() => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: false,
        supervisors: [{ name: "Frau Schmidt", phone: "0123456" }],
        myGroups: ["1"],
        myGroupRooms: ["Raum 101"],
        mySupervisedRooms: ["Raum 101"],
        refreshData: mockRefreshData,
      });
    });

    it("renders supervisors card in limited view", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("supervisors-card")).toBeInTheDocument();
    });

    it("renders personal info readonly in limited view", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("personal-info-readonly")).toBeInTheDocument();
    });

    it("does not render guardian manager in limited view", () => {
      render(<StudentDetailPage />);

      expect(screen.queryByTestId("guardian-manager")).not.toBeInTheDocument();
    });
  });

  describe("Edit Personal Information", () => {
    it("shows edit form when edit button is clicked", async () => {
      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("personal-info-edit-form"),
        ).toBeInTheDocument();
      });
    });

    it("cancels edit mode when cancel button is clicked", async () => {
      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("personal-info-edit-form"),
        ).toBeInTheDocument();
      });

      const cancelButton = screen.getByTestId("cancel-edit");
      fireEvent.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByTestId("personal-info-edit-form"),
        ).not.toBeInTheDocument();
        expect(
          screen.getByTestId("full-access-personal-info"),
        ).toBeInTheDocument();
      });
    });

    it("saves personal info successfully", async () => {
      mockUpdateStudent.mockResolvedValue({});

      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("personal-info-edit-form"),
        ).toBeInTheDocument();
      });

      const saveButton = screen.getByTestId("save-personal-info");
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(mockUpdateStudent).toHaveBeenCalledWith("1", expect.any(Object));
        expect(mockRefreshData).toHaveBeenCalled();
        expect(screen.getByTestId("alert-success")).toBeInTheDocument();
      });
    });

    it("shows error alert when save fails", async () => {
      mockUpdateStudent.mockRejectedValue(new Error("Save failed"));

      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("personal-info-edit-form"),
        ).toBeInTheDocument();
      });

      const saveButton = screen.getByTestId("save-personal-info");
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      });
    });
  });

  describe("Checkout Functionality", () => {
    beforeEach(() => {
      showCheckoutSection = true;
    });

    it("shows checkout modal when checkout button is clicked", async () => {
      render(<StudentDetailPage />);

      const checkoutButton = screen.getByTestId("checkout-button");
      fireEvent.click(checkoutButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-abmelden")).toBeInTheDocument();
      });
    });

    it("closes checkout modal when cancel is clicked", async () => {
      render(<StudentDetailPage />);

      const checkoutButton = screen.getByTestId("checkout-button");
      fireEvent.click(checkoutButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-abmelden")).toBeInTheDocument();
      });

      const cancelButton = screen.getByTestId("modal-cancel");
      fireEvent.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByTestId("modal-kind-abmelden"),
        ).not.toBeInTheDocument();
      });
    });

    it("performs checkout successfully", async () => {
      mockCheckoutStudent.mockResolvedValue({});

      render(<StudentDetailPage />);

      const checkoutButton = screen.getByTestId("checkout-button");
      fireEvent.click(checkoutButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-abmelden")).toBeInTheDocument();
      });

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockCheckoutStudent).toHaveBeenCalledWith("1");
        expect(mockRefreshData).toHaveBeenCalled();
        expect(screen.getByTestId("alert-success")).toBeInTheDocument();
      });
    });

    it("shows error when checkout fails", async () => {
      mockCheckoutStudent.mockRejectedValue(new Error("Checkout failed"));

      render(<StudentDetailPage />);

      const checkoutButton = screen.getByTestId("checkout-button");
      fireEvent.click(checkoutButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-abmelden")).toBeInTheDocument();
      });

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      });
    });
  });

  describe("Checkin Functionality", () => {
    beforeEach(() => {
      showCheckinSection = true;
      // Mock student at home
      mockUseStudentData.mockReturnValue({
        student: mockStudentAtHome,
        loading: false,
        error: null,
        hasFullAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
    });

    it("shows checkin modal when checkin button is clicked", async () => {
      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-anmelden")).toBeInTheDocument();
      });
    });

    it("loads active groups when checkin modal opens", async () => {
      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(mockGetActiveGroups).toHaveBeenCalledWith({ active: true });
      });
    });

    it("performs checkin successfully when room is selected", async () => {
      mockPerformImmediateCheckin.mockResolvedValue({});

      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-anmelden")).toBeInTheDocument();
      });

      // Wait for active groups to load and select one
      await waitFor(() => {
        const select = screen.getByRole("combobox");
        fireEvent.change(select, { target: { value: "1" } });
      });

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockPerformImmediateCheckin).toHaveBeenCalledWith(1, 1);
        expect(mockRefreshData).toHaveBeenCalled();
        expect(screen.getByTestId("alert-success")).toBeInTheDocument();
      });
    });

    it("shows error when checkin fails", async () => {
      mockPerformImmediateCheckin.mockRejectedValue(
        new Error("Checkin failed"),
      );

      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-anmelden")).toBeInTheDocument();
      });

      // Select a room first
      await waitFor(() => {
        const select = screen.getByRole("combobox");
        fireEvent.change(select, { target: { value: "1" } });
      });

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      });
    });

    it("disables confirm button when no room is selected", async () => {
      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-anmelden")).toBeInTheDocument();
        const confirmButton = screen.getByTestId("modal-confirm");
        expect(confirmButton).toBeDisabled();
      });
    });

    it("shows loading state while fetching active groups", async () => {
      // Make getActiveGroups take time
      mockGetActiveGroups.mockImplementation(
        () =>
          new Promise((resolve) =>
            setTimeout(
              () =>
                resolve([
                  {
                    id: "1",
                    room: { name: "Raum 101" },
                    actualGroup: { name: "Gruppe A" },
                  },
                ]),
              100,
            ),
          ),
      );

      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(screen.getByText(/Räume werden geladen/i)).toBeInTheDocument();
      });
    });

    it("shows message when no active rooms available", async () => {
      mockGetActiveGroups.mockResolvedValue([]);

      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(
          screen.getByText(/Keine aktiven Räume verfügbar/i),
        ).toBeInTheDocument();
      });
    });
  });

  describe("URL Parameters", () => {
    it("uses default referrer when no 'from' param is present", () => {
      render(<StudentDetailPage />);

      const backButton = screen.getByTestId("back-button");
      expect(backButton).toHaveAttribute("data-referrer", "/students/search");
    });

    it("uses custom referrer from URL params", () => {
      mockSearchParams.set("from", "/my-room");

      render(<StudentDetailPage />);

      const backButton = screen.getByTestId("back-button");
      expect(backButton).toHaveAttribute("data-referrer", "/my-room");
    });
  });

  describe("Guardian Manager Updates", () => {
    it("refreshes data when guardian manager triggers update", async () => {
      render(<StudentDetailPage />);

      const updateButton = screen.getByTestId("update-guardians");
      fireEvent.click(updateButton);

      await waitFor(() => {
        expect(mockRefreshData).toHaveBeenCalled();
      });
    });
  });
});
