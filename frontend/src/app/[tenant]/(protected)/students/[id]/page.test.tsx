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
import { useSession } from "next-auth/react";
import { useNFCEnabled } from "~/lib/tenant-context";

const {
  mockSWRMutate,
  MockStudentStatusDayConflictError,
  MockStudentStatusDayPartialAbsenceConflictError,
} = vi.hoisted(() => ({
  mockSWRMutate: vi.fn(),
  MockStudentStatusDayConflictError: class extends Error {
    conflicts: unknown[];
    totalCount: number;

    constructor(conflicts: unknown[], totalCount?: number) {
      super("conflict");
      this.name = "StudentStatusDayConflictError";
      this.conflicts = conflicts;
      this.totalCount = totalCount ?? conflicts.length;
    }
  },
  MockStudentStatusDayPartialAbsenceConflictError: class extends Error {
    constructor() {
      super(
        "Für diesen Tag liegt bereits eine Abmeldung ab einer Uhrzeit vor. Bitte zuerst die Teilabwesenheit entfernen.",
      );
      this.name = "StudentStatusDayPartialAbsenceConflictError";
    }
  },
}));
vi.mock("swr", async (importOriginal) => {
  const actual = await importOriginal<typeof import("swr")>();
  return {
    ...actual,
    useSWRConfig: () => ({ mutate: mockSWRMutate }),
  };
});

// Mock next-auth/react
vi.mock("next-auth/react", () => ({
  getSession: vi.fn(() => Promise.resolve({ user: { token: "test-token" } })),
  useSession: vi.fn(() => ({
    data: { user: { token: "test-token", permissions: ["config:manage"] } },
    status: "authenticated",
  })),
}));

// Mock next/navigation
const mockPush = vi.fn();
const mockReplace = vi.fn();
const mockSearchParams = new URLSearchParams();
vi.mock("next/navigation", () => ({
  useRouter: vi.fn(() => ({
    push: mockPush,
    replace: mockReplace,
    back: vi.fn(),
  })),
  useParams: vi.fn(() => ({ id: "1" })),
  usePathname: vi.fn(() => "/students/1"),
  useSearchParams: vi.fn(() => mockSearchParams),
}));

// Mock breadcrumb context
vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
  useBreadcrumb: vi.fn(() => ({ breadcrumb: {}, setBreadcrumb: vi.fn() })),
  BreadcrumbProvider: ({ children }: { children: React.ReactNode }) => (
    <>{children}</>
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
    <button type="button" data-testid="back-button" data-referrer={referrer}>
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
        <button type="button" data-testid="modal-cancel" onClick={onClose}>
          Abbrechen
        </button>
        <button
          type="button"
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
    enrollmentExtraGroups,
    showEditButton,
    onEditClick,
  }: {
    student: { name: string; school_class: string };
    enrollmentExtraGroups?: Array<{
      fields: Array<{ label: string; value: unknown }>;
    }>;
    showEditButton?: boolean;
    onEditClick?: () => void;
  }) => (
    <div
      data-testid={
        showEditButton ? "full-access-personal-info" : "personal-info-readonly"
      }
    >
      <span data-testid={showEditButton ? "fullaccess-name" : "readonly-name"}>
        {student.name}
      </span>
      <span data-testid="readonly-class">{student.school_class}</span>
      <span data-testid="enrollment-extra-count">
        {enrollmentExtraGroups?.flatMap((group) => group.fields).length ?? 0}
      </span>
      {showEditButton && onEditClick && (
        <button
          type="button"
          data-testid="edit-personal-info"
          onClick={onEditClick}
        >
          Bearbeiten
        </button>
      )}
    </div>
  ),
  StudentHistorySection: () => (
    <div data-testid="student-history">Historie</div>
  ),
}));

// Mock PersonalInfoFormModal
vi.mock("~/components/students/personal-info-form-modal", () => ({
  PersonalInfoFormModal: ({
    isOpen,
    onClose,
    student,
    onSave,
  }: {
    isOpen: boolean;
    onClose: () => void;
    student: { name: string };
    onSave: (student: { name: string }) => Promise<void>;
  }) => {
    const handleSave = async () => {
      try {
        await onSave(student);
        onClose();
      } catch {
        // Error is handled by the modal (shows toast)
      }
    };
    return isOpen ? (
      <div data-testid="personal-info-modal">
        <span data-testid="modal-student-name">{student.name}</span>
        <button
          type="button"
          data-testid="save-personal-info"
          onClick={() => void handleSave()}
        >
          Speichern
        </button>
        <button type="button" data-testid="cancel-edit" onClick={onClose}>
          Abbrechen
        </button>
      </div>
    ) : null;
  },
}));

vi.mock("~/components/students/planned-status-days-modal", () => ({
  PlannedStatusDaysModal: ({
    isOpen,
    status,
    onClose,
    onSubmit,
  }: {
    isOpen: boolean;
    status: "sick" | "excused" | "class_trip";
    onClose: () => void;
    onSubmit: (dates: string[]) => Promise<void>;
  }) =>
    isOpen ? (
      <div data-testid={`planned-status-modal-${status}`}>
        <button
          type="button"
          data-testid="planned-status-submit"
          onClick={() => void onSubmit(["2026-05-25"]).catch(() => undefined)}
        >
          Speichern
        </button>
        <button
          type="button"
          data-testid="planned-status-cancel"
          onClick={onClose}
        >
          Abbrechen
        </button>
      </div>
    ) : null,
}));

// Track which action type getStudentActionType returns
let mockActionType: "checkout" | "checkin" | "none" = "none";

// Mock checkout section
vi.mock("~/components/students/student-checkout-section", () => ({
  StudentCheckoutSection: ({
    onCheckoutClick,
  }: {
    onCheckoutClick: () => void;
  }) => (
    <div data-testid="checkout-section">
      <button
        type="button"
        data-testid="checkout-button"
        onClick={onCheckoutClick}
      >
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
      <button
        type="button"
        data-testid="checkin-button"
        onClick={onCheckinClick}
      >
        Kind anmelden
      </button>
    </div>
  ),
  StudentStatusActionsMenu: ({
    onPlanClassTrip,
  }: {
    isClassTrip: boolean;
    classTripSince?: string;
    onPlanClassTrip: () => void;
    isLoading: boolean;
  }) => (
    <div data-testid="status-actions-menu">
      <button
        type="button"
        data-testid="class-trip-action"
        onClick={onPlanClassTrip}
      >
        Klassenfahrt planen
      </button>
    </div>
  ),
  StudentSickReportSection: ({
    isSick,
    onToggle,
    isLoading,
  }: {
    isSick: boolean;
    sickSince?: string;
    onToggle: () => void;
    isLoading: boolean;
  }) => (
    <div data-testid="sick-report-section">
      <button
        type="button"
        data-testid="sick-toggle-button"
        onClick={onToggle}
        disabled={isLoading}
      >
        {isSick ? "Gesund melden" : "Krank melden"}
      </button>
    </div>
  ),
  StudentExcusedReportSection: ({
    isExcused,
    onToggle,
    isLoading,
  }: {
    isExcused: boolean;
    excusedSince?: string;
    onToggle: () => void;
    isLoading: boolean;
  }) => (
    <div data-testid="excused-report-section">
      <button
        type="button"
        data-testid="excused-toggle-button"
        onClick={onToggle}
        disabled={isLoading}
      >
        {isExcused ? "Entschuldigung aufheben" : "Entschuldigen"}
      </button>
    </div>
  ),
  getStudentActionType: vi.fn(() => mockActionType),
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
      <button type="button" data-testid="update-guardians" onClick={onUpdate}>
        Update
      </button>
    </div>
  ),
}));

vi.mock("~/components/students/care-schedule-manager", () => ({
  CareScheduleManager: ({
    studentId,
    onUpdate,
  }: {
    studentId: string;
    onUpdate?: () => void;
  }) => (
    <div data-testid="care-schedule-manager" data-student-id={studentId}>
      <button
        type="button"
        data-testid="update-care-schedule"
        onClick={() => onUpdate?.()}
      >
        Update Care
      </button>
    </div>
  ),
}));

vi.mock("~/components/students/student-enrollments-tab", () => ({
  StudentEnrollmentsTab: () => <div data-testid="student-enrollments-tab" />,
}));

// Mock pickup schedule API
vi.mock("~/lib/pickup-schedule-api", () => ({
  fetchStudentPickupData: vi.fn().mockResolvedValue({
    schedules: [],
    exceptions: [],
  }),
}));

// Mock arrival schedule API
vi.mock("~/lib/student-arrival-api", () => ({
  fetchArrivalData: vi.fn().mockResolvedValue({
    schedules: [],
    exceptions: [],
    notes: [],
  }),
}));

// Mock pickup schedule helpers
vi.mock("~/lib/pickup-schedule-helpers", () => ({
  getDayData: vi.fn().mockReturnValue({
    effectiveTime: null,
    effectiveNotes: null,
    isException: false,
  }),
  formatPickupTime: vi.fn().mockReturnValue("15:30"),
}));

// Mock checkin API
const mockPerformImmediateCheckin = vi.fn();
vi.mock("~/lib/checkin-api", () => ({
  performImmediateCheckin: (studentId: number, activeGroupId: number) =>
    mockPerformImmediateCheckin(studentId, activeGroupId) as Promise<
      Record<string, unknown>
    >,
}));

// Mock useStudentData hook
const mockRefreshData = vi.fn();
interface MockStudent {
  id: string;
  first_name: string;
  second_name: string;
  name: string;
  school_class: string;
  group_id: string;
  group_name: string;
  current_location: string;
  bus: boolean;
  buskind: boolean;
  birthday: string;
  health_info: string;
  supervisor_notes: string;
  extra_info: string;
  pickup_status: string;
  sick: boolean;
}
interface MockStudentDataResult {
  student: MockStudent | null;
  loading: boolean;
  error: string | null;
  hasFullAccess: boolean;
  hasWriteAccess: boolean;
  hasAbsenceWriteAccess: boolean;
  supervisors: Array<{ name: string; phone?: string }>;
  myGroups: string[];
  myGroupRooms: string[];
  mySupervisedRooms: string[];
  refreshData: () => void;
}
const mockUseStudentData = vi.fn();
vi.mock("~/lib/hooks/use-student-data", () => ({
  useStudentData: (studentId: string): MockStudentDataResult =>
    mockUseStudentData(studentId) as MockStudentDataResult,
}));

const mockUseStudentEnrollmentExtraFields = vi.fn();
vi.mock("~/lib/hooks/use-student-enrollment-extra-fields", () => ({
  useStudentEnrollmentExtraFields: (
    studentId: string,
    hasFullAccess: boolean,
  ) => mockUseStudentEnrollmentExtraFields(studentId, hasFullAccess),
}));

// Mock active service
const mockCheckoutStudent = vi.fn();
interface MockActiveGroup {
  id: string;
  room?: { name: string };
  actualGroup?: { name: string };
}
const mockGetActiveGroups = vi.fn();
vi.mock("~/lib/active-service", () => ({
  activeService: {
    checkoutStudent: (studentId: string): Promise<Record<string, unknown>> =>
      mockCheckoutStudent(studentId) as Promise<Record<string, unknown>>,
    getActiveGroups: (params: {
      active: boolean;
    }): Promise<MockActiveGroup[]> =>
      mockGetActiveGroups(params) as Promise<MockActiveGroup[]>,
  },
}));

// Mock student service
const mockUpdateStudent = vi.fn();
vi.mock("~/lib/api", () => ({
  studentService: {
    updateStudent: (
      id: string,
      data: unknown,
    ): Promise<Record<string, unknown>> =>
      mockUpdateStudent(id, data) as Promise<Record<string, unknown>>,
  },
}));

const mockCreateStudentStatusDays = vi.fn();
const mockFetchStudentStatusDays = vi.fn();
vi.mock("~/lib/student-status-days-api", () => ({
  StudentStatusDayConflictError: MockStudentStatusDayConflictError,
  StudentStatusDayPartialAbsenceConflictError:
    MockStudentStatusDayPartialAbsenceConflictError,
  createStudentStatusDays: (
    studentId: string,
    status: "sick" | "excused" | "class_trip",
    dates: string[],
  ) => mockCreateStudentStatusDays(studentId, status, dates),
  fetchStudentStatusDays: (studentId: string, from: string, to: string) =>
    mockFetchStudentStatusDays(studentId, from, to),
  deleteStudentStatusDay: vi.fn(),
}));

vi.mock("~/lib/student-partial-absences-api", () => ({
  fetchStudentPartialAbsences: vi.fn().mockResolvedValue([]),
  saveStudentPartialAbsence: vi.fn(),
  deleteStudentPartialAbsence: vi.fn(),
}));

// Mock useToast hook
const mockToastSuccess = vi.fn();
const mockToastError = vi.fn();
const mockToastWarning = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
    warning: mockToastWarning,
    remove: vi.fn(),
  })),
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
  address_street: "Musterstraße 12",
  address_postal_code: "50667",
  address_city: "Köln",
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
    vi.mocked(useNFCEnabled).mockReturnValue(true);
    mockSearchParams.delete("from");
    mockSearchParams.delete("tab");
    mockActionType = "none";
    vi.mocked(useSession).mockReturnValue({
      data: { user: { token: "test-token", permissions: ["config:manage"] } },
      status: "authenticated",
    } as ReturnType<typeof useSession>);

    // Default mock implementations
    mockUseStudentData.mockReturnValue({
      student: mockStudent,
      loading: false,
      error: null,
      hasFullAccess: true,
      hasWriteAccess: true,
      hasAbsenceWriteAccess: true,
      supervisors: [{ name: "Frau Schmidt", phone: "0123456" }],
      myGroups: ["1"],
      myGroupRooms: ["Raum 101"],
      mySupervisedRooms: ["Raum 101"],
      refreshData: mockRefreshData,
    });
    mockUseStudentEnrollmentExtraFields.mockReturnValue({
      groups: [],
      loading: false,
      hasError: false,
    });

    mockGetActiveGroups.mockResolvedValue([
      {
        id: "1",
        room: { name: "Raum 101" },
        actualGroup: { name: "Gruppe A" },
      },
    ]);
    mockCreateStudentStatusDays.mockResolvedValue([]);
    mockFetchStudentStatusDays.mockResolvedValue([]);
    mockSWRMutate.mockResolvedValue(undefined);
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
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("student-detail-skeleton")).toBeInTheDocument();
      expect(screen.getByLabelText("Kind wird geladen")).toBeInTheDocument();
    });
  });

  describe("Error State", () => {
    it("shows error message when fetching fails", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: "Kind nicht gefunden",
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
      expect(screen.getByText("Kind nicht gefunden")).toBeInTheDocument();
    });

    it("shows error when student is null", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
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
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      const backButton = screen.getByRole("button", { name: /zurück/i });
      fireEvent.click(backButton);

      expect(mockPush).toHaveBeenCalledWith("/test-tenant/students/search");
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

    it("passes enrollment extra fields into full access personal info", () => {
      mockUseStudentEnrollmentExtraFields.mockReturnValue({
        groups: [
          {
            request_id: "77",
            phase_name: "Anmeldung 2026",
            submitted_at: "2026-06-01T12:00:00Z",
            fields: [
              {
                key: "swimming_level",
                label: "Schwimmfähigkeit",
                type: "text",
                value: "Kann sicher schwimmen",
              },
            ],
          },
        ],
        loading: false,
        hasError: false,
      });

      render(<StudentDetailPage />);

      expect(mockUseStudentEnrollmentExtraFields).toHaveBeenCalledWith(
        "1",
        true,
      );
      expect(screen.getByTestId("enrollment-extra-count")).toHaveTextContent(
        "1",
      );
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
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
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
      expect(mockUseStudentEnrollmentExtraFields).not.toHaveBeenCalled();
      expect(screen.getByTestId("enrollment-extra-count")).toHaveTextContent(
        "0",
      );
    });

    it("renders guardian manager in read-only mode in limited view", () => {
      // All staff can view guardian info (read-only), only supervisors can edit
      render(<StudentDetailPage />);

      expect(screen.getByTestId("guardian-manager")).toBeInTheDocument();
    });
  });

  describe("Edit Personal Information", () => {
    it("shows edit modal when edit button is clicked", async () => {
      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });
    });

    it("closes modal when cancel button is clicked", async () => {
      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });

      const cancelButton = screen.getByTestId("cancel-edit");
      fireEvent.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByTestId("personal-info-modal"),
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
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });

      const saveButton = screen.getByTestId("save-personal-info");
      await act(async () => {
        fireEvent.click(saveButton);
      });

      await waitFor(() => {
        expect(mockUpdateStudent).toHaveBeenCalledWith(
          "1",
          expect.objectContaining({
            address_street: "Musterstraße 12",
            address_postal_code: "50667",
            address_city: "Köln",
          }),
        );
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("revalidates field history after saving personal info", async () => {
      mockUpdateStudent.mockResolvedValue({});

      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("edit-personal-info"));
      await waitFor(() => {
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByTestId("save-personal-info"));
      });

      await waitFor(() => {
        expect(mockSWRMutate).toHaveBeenCalledWith(
          "/api/students/1/change-history",
        );
      });
    });

    it("sends explicit departure_days when saving personal info", async () => {
      mockUpdateStudent.mockResolvedValue({});
      mockUseStudentData.mockReturnValue({
        student: {
          ...mockStudent,
          bus_days: { mon: true },
          pickup_days: { wed: true },
          departure_days: undefined,
        },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: ["Raum 101"],
        mySupervisedRooms: ["Raum 101"],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      fireEvent.click(screen.getByTestId("edit-personal-info"));
      await waitFor(() => {
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });

      await act(async () => {
        fireEvent.click(screen.getByTestId("save-personal-info"));
      });

      await waitFor(() => {
        expect(mockUpdateStudent).toHaveBeenCalledWith(
          "1",
          expect.objectContaining({
            bus_days: { mon: true },
            pickup_days: { wed: true },
            departure_days: { mon: "bus", wed: "pickup" },
          }),
        );
      });
    });

    it("does not close modal when save fails", async () => {
      mockUpdateStudent.mockRejectedValue(new Error("Save failed"));

      render(<StudentDetailPage />);

      const editButton = screen.getByTestId("edit-personal-info");
      fireEvent.click(editButton);

      await waitFor(() => {
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });

      const saveButton = screen.getByTestId("save-personal-info");
      await act(async () => {
        fireEvent.click(saveButton);
      });

      // Modal should remain open after failed save
      await waitFor(() => {
        expect(screen.getByTestId("personal-info-modal")).toBeInTheDocument();
      });
    });
  });

  describe("Checkout Functionality", () => {
    beforeEach(() => {
      mockActionType = "checkout";
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
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("shows error toast when checkout fails", async () => {
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
        expect(mockToastError).toHaveBeenCalled();
      });
    });
  });

  describe("Checkin Functionality", () => {
    beforeEach(() => {
      mockActionType = "checkin";
      // Mock student at home
      mockUseStudentData.mockReturnValue({
        student: mockStudentAtHome,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
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
      const select = await screen.findByRole("combobox");
      fireEvent.click(select);
      fireEvent.click(
        screen.getByRole("option", { name: "Raum 101 (Gruppe A)" }),
      );

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockPerformImmediateCheckin).toHaveBeenCalledWith(1, 1);
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("shows error toast when checkin fails", async () => {
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
      const select = await screen.findByRole("combobox");
      fireEvent.click(select);
      fireEvent.click(
        screen.getByRole("option", { name: "Raum 101 (Gruppe A)" }),
      );

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalled();
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

    it("shows the NFC instruction when no active rooms are available for an NFC tenant", async () => {
      mockGetActiveGroups.mockResolvedValue([]);

      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(
          screen.getByText(
            "Keine aktiven Räume verfügbar. Bitte starten Sie zuerst eine aktive Aufsicht in einem Raum über ein NFC-Tablet.",
          ),
        ).toBeInTheDocument();
      });
    });

    it("shows a web instruction when no active rooms are available without NFC", async () => {
      vi.mocked(useNFCEnabled).mockReturnValue(false);
      mockGetActiveGroups.mockResolvedValue([]);

      render(<StudentDetailPage />);

      fireEvent.click(screen.getByTestId("checkin-button"));

      await waitFor(() => {
        expect(
          screen.getByText(
            "Keine aktiven Räume verfügbar. Bitte starten Sie zuerst eine aktive Aufsicht in einem Raum über die Web-App.",
          ),
        ).toBeInTheDocument();
      });
      expect(screen.queryByText(/NFC-Tablet/i)).not.toBeInTheDocument();
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

  describe("Embedded Manager Updates", () => {
    it("refreshes data when guardian manager triggers update", async () => {
      render(<StudentDetailPage />);

      const updateButton = screen.getByTestId("update-guardians");
      fireEvent.click(updateButton);

      await waitFor(() => {
        expect(mockRefreshData).toHaveBeenCalled();
      });
    });

    it("revalidates field history when the care schedule triggers an update", async () => {
      render(<StudentDetailPage />);

      fireEvent.click(screen.getByTestId("update-care-schedule"));

      await waitFor(() => {
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockSWRMutate).toHaveBeenCalledWith(
          "/api/students/1/change-history",
        );
      });
    });
  });

  describe("Sick Report Functionality", () => {
    it("renders sick report section in full access view", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("sick-report-section")).toBeInTheDocument();
      expect(screen.getByTestId("sick-toggle-button")).toHaveTextContent(
        "Krank melden",
      );
    });

    it("shows sick date picker modal when sick button is clicked", async () => {
      render(<StudentDetailPage />);

      const sickButton = screen.getByTestId("sick-toggle-button");
      fireEvent.click(sickButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-sick"),
        ).toBeInTheDocument();
      });
    });

    it("creates sick status days successfully", async () => {
      mockCreateStudentStatusDays.mockResolvedValue([]);

      render(<StudentDetailPage />);

      const sickButton = screen.getByTestId("sick-toggle-button");
      fireEvent.click(sickButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-sick"),
        ).toBeInTheDocument();
      });

      const confirmButton = screen.getByTestId("planned-status-submit");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockCreateStudentStatusDays).toHaveBeenCalledWith("1", "sick", [
          "2026-05-25",
        ]);
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("does not send bus through the sick date picker flow", async () => {
      mockCreateStudentStatusDays.mockResolvedValue([]);

      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, buskind: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      const sickButton = screen.getByTestId("sick-toggle-button");
      fireEvent.click(sickButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-sick"),
        ).toBeInTheDocument();
      });

      const confirmButton = screen.getByTestId("planned-status-submit");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockCreateStudentStatusDays).toHaveBeenCalledWith("1", "sick", [
          "2026-05-25",
        ]);
        expect(mockUpdateStudent).not.toHaveBeenCalled();
      });
    });

    it("shows error toast when sick toggle fails", async () => {
      mockCreateStudentStatusDays.mockRejectedValue(new Error("Toggle failed"));

      render(<StudentDetailPage />);

      const sickButton = screen.getByTestId("sick-toggle-button");
      fireEvent.click(sickButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-sick"),
        ).toBeInTheDocument();
      });

      const confirmButton = screen.getByTestId("planned-status-submit");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalled();
      });
    });

    it("explains an atomic conflict without reporting a successful save", async () => {
      mockCreateStudentStatusDays.mockRejectedValue(
        new MockStudentStatusDayConflictError([
          {
            id: "7",
            student_id: "1",
            date: "2026-05-25",
            status: "excused",
            label: "Entschuldigt",
          },
        ]),
      );

      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("sick-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-sick"),
        ).toBeInTheDocument();
      });
      fireEvent.click(screen.getByTestId("planned-status-submit"));

      await waitFor(() => {
        expect(mockToastWarning).toHaveBeenCalledWith(
          "25.05.2026 (entschuldigt) wurde zwischenzeitlich eingetragen und nicht überschrieben. Bitte Auswahl prüfen.",
        );
      });
      expect(mockToastSuccess).not.toHaveBeenCalled();
    });

    it("shows healthy-toggle modal when student is already sick", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, sick: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      const sickButton = screen.getByTestId("sick-toggle-button");
      expect(sickButton).toHaveTextContent("Gesund melden");
      fireEvent.click(sickButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("modal-krankmeldung-aufheben"),
        ).toBeInTheDocument();
      });
    });

    it("closes sick date picker modal when cancel is clicked", async () => {
      render(<StudentDetailPage />);

      const sickButton = screen.getByTestId("sick-toggle-button");
      fireEvent.click(sickButton);

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-sick"),
        ).toBeInTheDocument();
      });

      const cancelButton = screen.getByTestId("planned-status-cancel");
      fireEvent.click(cancelButton);

      await waitFor(() => {
        expect(
          screen.queryByTestId("planned-status-modal-sick"),
        ).not.toBeInTheDocument();
      });
    });

    it("does not show sick report in limited access view", () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(
        screen.queryByTestId("sick-report-section"),
      ).not.toBeInTheDocument();
    });

    // Offene Betreuung (#2232): without fixed groups the absence gate stands on
    // its own, so the action is offered even where Stammdaten stay read-only —
    // in the limited view too, since the child has no supervisor to fall back on.
    it("shows the absence actions without Stammdaten write access", () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: true,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("sick-report-section")).toBeInTheDocument();
      expect(screen.getByTestId("excused-report-section")).toBeInTheDocument();
      expect(
        screen.queryByTestId("edit-personal-info"),
      ).not.toBeInTheDocument();
    });

    it("shows the absence actions in the limited access view", () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: true,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("sick-report-section")).toBeInTheDocument();
    });
  });

  describe("Excused Report Functionality", () => {
    it("renders excused report section in full access view", () => {
      render(<StudentDetailPage />);
      expect(screen.getByTestId("excused-report-section")).toBeInTheDocument();
      expect(screen.getByTestId("excused-toggle-button")).toHaveTextContent(
        "Entschuldigen",
      );
    });

    it("shows excused date picker modal when excused button is clicked", async () => {
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-excused"),
        ).toBeInTheDocument();
      });
    });

    it("creates excused status days successfully", async () => {
      mockCreateStudentStatusDays.mockResolvedValue([]);
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-excused"),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByTestId("planned-status-submit"));
      });
      await waitFor(() => {
        expect(mockCreateStudentStatusDays).toHaveBeenCalledWith(
          "1",
          "excused",
          ["2026-05-25"],
        );
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("shows error toast when excused toggle fails", async () => {
      mockCreateStudentStatusDays.mockRejectedValue(new Error("fail"));
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-excused"),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByTestId("planned-status-submit"));
      });
      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalled();
      });
    });

    it("shows aufheben modal when student is already excused", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, excused: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      render(<StudentDetailPage />);
      const btn = screen.getByTestId("excused-toggle-button");
      expect(btn).toHaveTextContent("Entschuldigung aufheben");
      fireEvent.click(btn);
      await waitFor(() => {
        expect(
          screen.getByTestId("modal-entschuldigung-aufheben"),
        ).toBeInTheDocument();
      });
    });

    it("keeps class-trip students in the planned excused flow", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, excused: true, class_trip: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      const btn = screen.getByTestId("excused-toggle-button");
      expect(btn).toHaveTextContent("Entschuldigen");
      fireEvent.click(btn);

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-excused"),
        ).toBeInTheDocument();
      });
      expect(
        screen.queryByTestId("modal-entschuldigung-aufheben"),
      ).not.toBeInTheDocument();
      expect(mockUpdateStudent).not.toHaveBeenCalled();
    });
  });

  describe("Switch Status Dialog (mutual exclusion)", () => {
    it("opens the switch dialog when clicking Krank on an excused student", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, excused: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("sick-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("modal-als-krank-melden?"),
        ).toBeInTheDocument();
      });
    });

    it("opens the switch dialog when clicking Entschuldigt on a sick student", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, sick: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("modal-als-entschuldigt-markieren?"),
        ).toBeInTheDocument();
      });
    });

    it("sends clear-one + set-other when confirming a switch", async () => {
      mockUpdateStudent.mockResolvedValue({});
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, sick: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("modal-als-entschuldigt-markieren?"),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByTestId("modal-confirm"));
      });
      await waitFor(() => {
        expect(mockUpdateStudent).toHaveBeenCalledWith("1", {
          sick: false,
          excused: true,
        });
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("shows error toast if the switch request fails", async () => {
      mockUpdateStudent.mockRejectedValue(new Error("boom"));
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, excused: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("sick-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("modal-als-krank-melden?"),
        ).toBeInTheDocument();
      });
      await act(async () => {
        fireEvent.click(screen.getByTestId("modal-confirm"));
      });
      await waitFor(() => {
        expect(mockToastError).toHaveBeenCalled();
      });
    });

    it("closes the switch dialog on cancel without calling the API", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, sick: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));
      await waitFor(() => {
        expect(
          screen.getByTestId("modal-als-entschuldigt-markieren?"),
        ).toBeInTheDocument();
      });
      fireEvent.click(screen.getByTestId("modal-cancel"));
      await waitFor(() => {
        expect(
          screen.queryByTestId("modal-als-entschuldigt-markieren?"),
        ).not.toBeInTheDocument();
      });
      expect(mockUpdateStudent).not.toHaveBeenCalled();
    });
  });

  describe("Action Card Visibility", () => {
    it("hides checkout and checkin when action type is none", () => {
      mockActionType = "none";

      render(<StudentDetailPage />);

      expect(screen.queryByTestId("checkout-section")).not.toBeInTheDocument();
      expect(screen.queryByTestId("checkin-section")).not.toBeInTheDocument();
    });

    it("shows checkout section when action type is checkout", () => {
      mockActionType = "checkout";

      render(<StudentDetailPage />);

      expect(screen.getByTestId("checkout-section")).toBeInTheDocument();
      expect(screen.queryByTestId("checkin-section")).not.toBeInTheDocument();
    });

    it("shows checkin section when action type is checkin", () => {
      mockActionType = "checkin";
      mockUseStudentData.mockReturnValue({
        student: mockStudentAtHome,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("checkin-section")).toBeInTheDocument();
      expect(screen.queryByTestId("checkout-section")).not.toBeInTheDocument();
    });
  });

  describe("Tabbed Navigation (#1501)", () => {
    const limitedAccess = {
      student: mockStudent,
      loading: false,
      error: null,
      hasFullAccess: false,
      hasWriteAccess: false,
      hasAbsenceWriteAccess: false,
      supervisors: [{ name: "Frau Schmidt" }],
      myGroups: ["1"],
      myGroupRooms: [],
      mySupervisedRooms: [],
      refreshData: mockRefreshData,
    };

    // Radix activates a tab on pointer-down (mouseDown is the legacy fallback).
    // Fire both, mirroring the precedent in ui/tabs.test.tsx, so these tests
    // don't hinge on a single internal Radix event path and survive a bump.
    const selectTab = (name: string) => {
      const tab = screen.getByRole("tab", { name });
      fireEvent.pointerDown(tab, { button: 0, pointerType: "mouse" });
      fireEvent.mouseDown(tab, { button: 0 });
    };

    it("renders all section tabs in the full access view", () => {
      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tab", { name: "Stammdaten" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Erziehungsberechtigte" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Betreuungszeiten" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Anmeldungen" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: "Historie" })).toBeInTheDocument();
    });

    it("hides the enrollment tab without config:manage", () => {
      vi.mocked(useSession).mockReturnValue({
        data: { user: { token: "test-token", permissions: ["config:read"] } },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(
        screen.queryByRole("tab", { name: "Anmeldungen" }),
      ).not.toBeInTheDocument();
    });

    it("hides the Betreuungsplan tab without schedules:read", () => {
      // Default mock session is config:manage only -> no schedules:read.
      render(<StudentDetailPage />);

      expect(
        screen.queryByRole("tab", { name: "Betreuungsplan" }),
      ).not.toBeInTheDocument();
    });

    it("shows the Betreuungsplan tab with schedules:read", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            token: "test-token",
            permissions: ["config:manage", "schedules:read"],
          },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tab", { name: "Betreuungsplan" }),
      ).toBeInTheDocument();
    });

    it("hides the Betreuungsplan tab in the limited-access view despite schedules:read", () => {
      // Deliberate, not an oversight: has_full_access on the student response is
      // authorize.CanReadStudent, the SAME predicate the backend applies inside
      // GET /timetable/student/{id}/{day,week} (api/timetable/api.go gates the
      // route on schedules:read, resolveStudentForRead then runs CanReadStudent).
      // A limited-access viewer therefore gets 403 from the care-plan endpoints,
      // so showing the tab would only offer a permanently failing panel.
      mockUseStudentData.mockReturnValue(limitedAccess);
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            token: "test-token",
            permissions: ["config:manage", "schedules:read"],
          },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(
        screen.queryByRole("tab", { name: "Betreuungsplan" }),
      ).not.toBeInTheDocument();
    });

    it("shows the Betreuungsplan tab to a non-supervisor under gdpr.student_data_scope=all_staff", () => {
      // The complement of the test above, and the case worth pinning: under
      // `all_staff` the backend's read predicate (authorize.CanReadStudent)
      // returns true for EVERY staff member, so has_full_access arrives true
      // even for someone who supervises none of this child's groups — and the
      // care-plan endpoints, gated on that same predicate, will serve them.
      // The tab must therefore appear. It is gated on read access, never on the
      // stricter supervisor/admin write predicate (has_write_access).
      mockUseStudentData.mockReturnValue({
        ...limitedAccess,
        hasFullAccess: true, // all_staff scope grants read access…
        hasWriteAccess: false, // …while writes stay supervisor-only
        myGroups: [],
        mySupervisedRooms: [],
      });
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            token: "test-token",
            permissions: ["schedules:read"],
          },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tab", { name: "Betreuungsplan" }),
      ).toBeInTheDocument();
    });

    it("defaults to the Stammdaten tab when no tab param is set", () => {
      render(<StudentDetailPage />);

      expect(screen.getByRole("tab", { name: "Stammdaten" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });

    it("marks only the active panel active and the rest inactive (hidden via CSS)", () => {
      // forceMount keeps every panel mounted; Radix sets data-state, and the
      // `data-[state=inactive]:hidden` class hides the inactive ones in the
      // browser. jsdom applies no CSS, so we assert the data-state wiring that
      // drives the hiding instead of computed visibility.
      render(<StudentDetailPage />);

      const panels = screen.getAllByRole("tabpanel", { hidden: true });
      const active = panels.filter(
        (panel) => panel.getAttribute("data-state") === "active",
      );
      const inactive = panels.filter(
        (panel) => panel.getAttribute("data-state") === "inactive",
      );

      expect(active).toHaveLength(1);
      expect(inactive.length).toBeGreaterThan(0);
      for (const panel of inactive) {
        expect(panel.className).toContain("data-[state=inactive]:hidden");
      }
    });

    it("activates the tab from the ?tab= query param (deep link)", () => {
      mockSearchParams.set("tab", "betreuungszeiten");

      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tab", { name: "Betreuungszeiten" }),
      ).toHaveAttribute("aria-selected", "true");
    });

    it("updates the URL with the tab param when a tab is selected", () => {
      render(<StudentDetailPage />);

      selectTab("Historie");

      expect(mockReplace).toHaveBeenCalledWith("/students/1?tab=historie", {
        scroll: false,
      });
    });

    it("drops the tab param when returning to the default tab", () => {
      mockSearchParams.set("tab", "historie");

      render(<StudentDetailPage />);

      selectTab("Stammdaten");

      expect(mockReplace).toHaveBeenCalledWith("/students/1", {
        scroll: false,
      });
    });

    it("preserves the from referrer when switching tabs", () => {
      mockSearchParams.set("from", "/my-room");

      render(<StudentDetailPage />);

      selectTab("Historie");

      expect(mockReplace).toHaveBeenCalledWith(
        "/students/1?from=%2Fmy-room&tab=historie",
        { scroll: false },
      );
    });

    it("hides the Betreuungszeiten tab in the limited access view", () => {
      mockUseStudentData.mockReturnValue(limitedAccess);

      render(<StudentDetailPage />);

      expect(
        screen.queryByRole("tab", { name: "Betreuungszeiten" }),
      ).not.toBeInTheDocument();
      expect(
        screen.getByRole("tab", { name: "Stammdaten" }),
      ).toBeInTheDocument();
      expect(screen.getByRole("tab", { name: "Historie" })).toBeInTheDocument();
    });

    it("falls back to the default tab when deep-linking a tab the access level lacks", () => {
      mockSearchParams.set("tab", "betreuungszeiten");
      mockUseStudentData.mockReturnValue(limitedAccess);

      render(<StudentDetailPage />);

      expect(screen.getByRole("tab", { name: "Stammdaten" })).toHaveAttribute(
        "aria-selected",
        "true",
      );
    });

    it("rewrites the URL to drop an inaccessible deep-linked tab", () => {
      mockSearchParams.set("tab", "betreuungszeiten");
      mockUseStudentData.mockReturnValue(limitedAccess);

      render(<StudentDetailPage />);

      // Clamped to the default tab, so the bogus param is stripped from the URL
      // rather than left advertising a tab the user can't open.
      expect(mockReplace).toHaveBeenCalledWith("/students/1", {
        scroll: false,
      });
    });

    it("rewrites the URL to drop an unknown tab value", () => {
      mockSearchParams.set("tab", "nonsense");

      render(<StudentDetailPage />);

      expect(mockReplace).toHaveBeenCalledWith("/students/1", {
        scroll: false,
      });
    });

    // Regression guard for the Rules-of-Hooks ordering bug: the tab self-heal
    // useEffect must be called before the loading/error early returns, or the
    // first loaded render adds a hook that the loading render skipped and React
    // throws "Rendered more hooks than during the previous render". A single
    // mounted instance must survive the loading -> loaded transition. oxlint
    // does not run the react-hooks plugin, so only this test catches it.
    it("survives the loading -> loaded transition without a hook-order crash", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: true,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      const { rerender } = render(<StudentDetailPage />);
      expect(screen.getByTestId("student-detail-skeleton")).toBeInTheDocument();

      // Same component instance now finishes loading with full access.
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        supervisors: [{ name: "Frau Schmidt", phone: "0123456" }],
        myGroups: ["1"],
        myGroupRooms: ["Raum 101"],
        mySupervisedRooms: ["Raum 101"],
        refreshData: mockRefreshData,
      });

      expect(() => rerender(<StudentDetailPage />)).not.toThrow();
      expect(
        screen.getByRole("tab", { name: "Stammdaten" }),
      ).toBeInTheDocument();
    });
  });
});
