import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import {
  render,
  screen,
  fireEvent,
  waitFor,
  cleanup,
  act,
  within,
} from "@testing-library/react";
import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import StudentDetailPage from "./page";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { SWRConfig } from "swr";
import { useSession } from "next-auth/react";
import { getDayData as getArrivalDayData } from "~/lib/arrival-schedule-helpers";
import { getDayData as getPickupDayData } from "~/lib/pickup-schedule-helpers";
import { useWeekendFollowsFriday } from "~/lib/tenant-context";
import { setTestClock } from "~/test/clock";
import { fetchStudentPickupData } from "~/lib/pickup-schedule-api";

const { mockSWRMutate, MockStudentStatusDayConflictError } = vi.hoisted(() => ({
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
  getSession: vi.fn(() =>
    Promise.resolve({ user: { id: "1", token: "test-token" } }),
  ),
  // Staff reads parent data with users:read; the role „Gast“ holds only
  // students:read (ADR 0091, see „students:read only“ below).
  useSession: vi.fn(() => ({
    data: {
      user: { id: "1", permissions: ["config:manage", "users:read"] },
    },
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
  Alert: ({
    message,
    type,
    action,
  }: {
    message: string;
    type: string;
    action?: React.ReactNode;
  }) => (
    <div data-testid={`alert-${type}`}>
      {message}
      {action}
    </div>
  ),
}));

// Der Rückweg der Kopfkarte (TenantPage) als Link mit seinem Ziel.
vi.mock("~/components/ui/back-link", () => ({
  BackLink: ({ href, label }: { href: string; label: string }) => (
    <a data-testid="back-button" href={href} data-referrer={href}>
      {label}
    </a>
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
  // Der Entitätskopf ist seit dem Gerüstumbau in seine Teile zerlegt: die
  // Seite setzt Foto, Titel, Statuszeile und Aufenthaltsort in TenantPage
  // zusammen.
  StudentHeaderAvatar: () => <div data-testid="student-header" />,
  studentHeaderTitle: (student: { name: string }) => student.name,
  StudentHeaderStats: ({ student }: { student: { school_class: string } }) => (
    <span data-testid="student-class">{student.school_class}</span>
  ),
  StudentHeaderLocation: () => <span data-testid="student-location" />,
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

// Mock des Bearbeiten-Zustands im Stammdaten-Reiter (kein Modal mehr).
vi.mock("~/components/students/holiday-guest-section", () => ({
  HolidayGuestSection: ({
    studentId,
    canRead,
  }: {
    studentId: string;
    canRead: boolean;
  }) => (
    <div data-testid="holiday-guest-section">
      {studentId}:{String(canRead)}
    </div>
  ),
}));

vi.mock("~/components/students/personal-info-edit-panel", () => ({
  PersonalInfoEditPanel: ({
    onCancel,
    student,
    onSave,
  }: {
    onCancel: () => void;
    student: { name: string };
    onSave: (student: { name: string }) => Promise<void>;
  }) => {
    const isOpen = true;
    const onClose = onCancel;
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
    canPlanPartialExcusal,
  }: {
    isOpen: boolean;
    status: "sick" | "excused" | "class_trip";
    onClose: () => void;
    onSubmit: (dates: string[]) => Promise<void>;
    canPlanPartialExcusal?: boolean;
  }) =>
    isOpen ? (
      <div
        data-testid={`planned-status-modal-${status}`}
        data-can-plan-partial={String(canPlanPartialExcusal ?? true)}
      >
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
    weekendFollowsFriday,
  }: {
    studentId: string;
    onUpdate?: () => void;
    weekendFollowsFriday?: boolean;
  }) => (
    <div
      data-testid="care-schedule-manager"
      data-student-id={studentId}
      data-weekend-follows-friday={weekendFollowsFriday}
    >
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

vi.mock("~/components/students/dokumente-tab", () => ({
  StudentDokumenteTab: ({
    canRecordAccidents,
  }: {
    canRecordAccidents: boolean;
  }) => (
    <div
      data-testid="student-dokumente-tab"
      data-can-record-accidents={String(canRecordAccidents)}
    />
  ),
}));

// These child panels are real here; isolate their read APIs as well. Otherwise
// mounts and SWR retries outlive the page test and hit localhost:3000.
vi.mock("~/lib/change-request-list-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/change-request-list-api")>();
  return {
    ...actual,
    getFamilyProtection: vi.fn(async (studentId: string) => ({
      student_id: studentId,
      enabled: false,
    })),
  };
});

vi.mock("~/lib/student-notes-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/student-notes-api")>();
  vi.spyOn(actual.studentNotesService, "list").mockResolvedValue([]);
  vi.spyOn(actual.studentNotesService, "listActivities").mockResolvedValue([]);
  return actual;
});

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

vi.mock("~/lib/arrival-schedule-helpers", () => ({
  getDayData: vi.fn().mockReturnValue({
    effectiveTime: undefined,
    effectiveReason: undefined,
    isException: false,
    isAbsent: false,
  }),
  formatArrivalTime: vi.fn((time: string) => time),
}));

const mockSchoolCheckinStudent = vi.fn();
vi.mock("~/lib/student-api", () => ({
  schoolCheckinStudent: (studentId: string, action: "in" | "out") =>
    mockSchoolCheckinStudent(studentId, action) as Promise<
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
  extra_info: string;
  pickup_status: string;
  sick: boolean;
}
interface MockStudentDataResult {
  student: MockStudent | null;
  loading: boolean;
  error: unknown;
  hasFullAccess: boolean;
  hasWriteAccess: boolean;
  hasAbsenceWriteAccess: boolean;
  hasSickExcusedWriteAccess: boolean;
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
const mockShowActionError = vi.fn();
// Form and load errors use the real hooks (no provider needed); toasts and
// the toast error path are spies.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: vi.fn(() => ({
    success: mockToastSuccess,
    error: mockToastError,
    info: vi.fn(),
    warning: mockToastWarning,
    remove: vi.fn(),
  })),
  useApiErrorDisplay: vi.fn(() => ({ show: mockShowActionError })),
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
    mockSearchParams.delete("tab");
    vi.mocked(useSession).mockReturnValue({
      data: {
        user: {
          id: "1",
          // Staff with absence rights; the route of every absence write
          // checks users:update or users:absence (ADR 0091 gate).
          permissions: [
            "config:manage",
            "users:checkin",
            "users:read",
            "users:absence",
          ],
        },
      },
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
      hasSickExcusedWriteAccess: true,
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

    mockCreateStudentStatusDays.mockResolvedValue([]);
    mockFetchStudentStatusDays.mockResolvedValue([]);
    mockSWRMutate.mockResolvedValue(undefined);
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(false);
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
        hasSickExcusedWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("student-detail-skeleton")).toBeInTheDocument();
      expect(screen.getByLabelText("Kind wird geladen")).toBeInTheDocument();
      expect(
        screen.getByRole("heading", { level: 1, name: "Kindakte" }),
      ).toBeInTheDocument();
      expect(screen.getByTestId("back-button")).toHaveAttribute(
        "data-referrer",
        "/test-tenant/students/search",
      );
      expect(screen.getByRole("tablist")).toBeInTheDocument();
    });
  });

  describe("Error State", () => {
    it("shows the catalog text with retry when fetching fails", async () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: new ApiError("boom", 500, {
          code: "general.server",
          instance: "req-student",
        }),
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        hasSickExcusedWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(await screen.findByTestId("alert-error")).toHaveTextContent(
        "Die Kindakte konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      );
    });

    it("says the child was not found for a 404", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: new ApiError("not found", 404),
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        hasSickExcusedWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("alert-error")).toHaveTextContent(
        "Kind nicht gefunden",
      );
    });

    it("shows error when student is null", () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        hasSickExcusedWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("alert-error")).toBeInTheDocument();
    });

    it("rendert im Fehlerfall den Rückweg zur Herkunftsliste", async () => {
      mockUseStudentData.mockReturnValue({
        student: null,
        loading: false,
        error: "Error",
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        hasSickExcusedWriteAccess: false,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      // Der Desktop-Rückweg ist die Breadcrumb; mobil trägt das Seitengerüst
      // den Referrer der Herkunftsliste.
      expect(screen.getByTestId("back-button")).toHaveAttribute(
        "data-referrer",
        "/test-tenant/students/search",
      );
    });
  });

  describe("Full Access View", () => {
    it.each(schoolDateCases("2026-05-27"))(
      "fetches the Berlin status-day range in %s",
      async (zone, instant) => {
        setSchoolDateClock(zone, instant, "2026-05-27");
        render(
          <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
            <StudentDetailPage />
          </SWRConfig>,
        );
        await waitFor(() =>
          expect(mockFetchStudentStatusDays).toHaveBeenCalledWith(
            "1",
            "2026-05-13",
            "2026-08-25",
          ),
        );
      },
    );

    it("uses Berlin's calendar day for the header plans", async () => {
      setTestClock("2026-09-12T12:00:00+02:00");
      vi.mocked(useWeekendFollowsFriday).mockReturnValue(true);

      render(
        <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
          <StudentDetailPage />
        </SWRConfig>,
      );

      await waitFor(() => {
        expect(getArrivalDayData).toHaveBeenLastCalledWith(
          new Date(2026, 8, 12),
          [],
          [],
          [],
          false,
          false,
          null,
          true,
          "2026-09-12",
        );
        expect(getPickupDayData).toHaveBeenLastCalledWith(
          new Date(2026, 8, 12),
          [],
          [],
          false,
          undefined,
          false,
          null,
          undefined,
          true,
          [],
          "2026-09-12",
        );
      });
      expect(screen.getByTestId("care-schedule-manager")).toHaveAttribute(
        "data-weekend-follows-friday",
        "true",
      );
    });

    it("renders student header with name", () => {
      render(<StudentDetailPage />);

      expect(screen.getByTestId("student-header")).toBeInTheDocument();
      // Der Name ist der Titel der Kopfkarte.
      expect(
        screen.getByRole("heading", { level: 1, name: "Max Mustermann" }),
      ).toBeInTheDocument();
    });

    it("renders full access personal info section", () => {
      render(<StudentDetailPage />);

      expect(
        screen.getByTestId("full-access-personal-info"),
      ).toBeInTheDocument();
    });

    it("shows a withdrawn consent in the Stammdaten tab", () => {
      mockUseStudentData.mockReturnValue({
        student: {
          ...mockStudent,
          consents: [
            {
              key: "photo",
              state: "withdrawn",
              changed_at: "2026-08-31T15:00:00Z",
            },
          ],
        },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: ["Raum 101"],
        mySupervisedRooms: ["Raum 101"],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(
        screen.getByRole("heading", {
          name: "Einwilligungen und Bestätigungen",
        }),
      ).toBeInTheDocument();
      expect(screen.getByText("Widerrufen am 31.08.2026")).toBeInTheDocument();
    });

    it("hides sick and excused actions without hiding class trips or child edits", () => {
      const current: MockStudentDataResult = mockUseStudentData("1");
      mockUseStudentData.mockReturnValue({
        ...current,
        hasSickExcusedWriteAccess: false,
      });
      render(<StudentDetailPage />);
      expect(
        screen.queryByTestId("sick-report-section"),
      ).not.toBeInTheDocument();
      expect(
        screen.queryByTestId("excused-report-section"),
      ).not.toBeInTheDocument();
      expect(screen.getByText("Klassenfahrt planen")).toBeInTheDocument();
      expect(screen.getByTestId("edit-personal-info")).toBeInTheDocument();
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
        hasSickExcusedWriteAccess: false,
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

    it("hides the notes tab because its endpoint requires full child read access", () => {
      render(<StudentDetailPage />);

      // Anker: die eingeschränkte Ansicht ist geladen, nicht das Skelett.
      expect(screen.getByTestId("supervisors-card")).toBeInTheDocument();
      expect(
        screen.queryByRole("tab", { name: "Notizen" }),
      ).not.toBeInTheDocument();
    });

    it("shows the notes tab in the full access view", () => {
      mockUseStudentData.mockReturnValue({
        ...mockUseStudentData(),
        hasFullAccess: true,
      });

      render(<StudentDetailPage />);

      expect(screen.getByRole("tab", { name: "Notizen" })).toBeInTheDocument();
    });

    it("renders guardian manager in read-only mode in limited view", () => {
      // All staff can view guardian info (read-only), only supervisors can edit
      render(<StudentDetailPage />);

      expect(screen.getByTestId("guardian-manager")).toBeInTheDocument();
    });

    it("renders documents for accident report readers", () => {
      mockSearchParams.set("tab", "dokumente");
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: { id: "1", permissions: ["student_accidents:read"] },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tab", { name: "Dokumente" }),
      ).toBeInTheDocument();
      expect(screen.getByTestId("student-dokumente-tab")).toHaveAttribute(
        "data-can-record-accidents",
        "false",
      );
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
      const payload = mockUpdateStudent.mock.calls[0]?.[1] as Record<
        string,
        unknown
      >;
      expect(payload).not.toHaveProperty("privacy_consent_accepted");
      expect(payload).not.toHaveProperty("data_retention_days");
      expect(payload).not.toHaveProperty("supervisor_notes");
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
        hasSickExcusedWriteAccess: true,
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
      mockSchoolCheckinStudent.mockResolvedValue({});

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
        expect(mockSchoolCheckinStudent).toHaveBeenCalledWith("1", "out");
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("shows the error in the dialog when checkout fails", async () => {
      const failure = new ApiError("Checkout failed", 500, {
        code: "general.server",
      });
      mockSchoolCheckinStudent.mockRejectedValue(failure);

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

      // Der Dialog bleibt offen; ein Toast läge hinter seinem Hintergrund.
      const dialog = screen.getByTestId("modal-kind-abmelden");
      expect(
        await within(dialog).findByText(
          catalogText("general.server", "die Anwesenheit"),
        ),
      ).toBeInTheDocument();
      // Der Bestätigen-Knopf wiederholt mit dem aktuellen Stand.
      expect(within(dialog).getByTestId("modal-confirm")).toBeEnabled();
      expect(mockToastError).not.toHaveBeenCalled();
    });
  });

  describe("Checkin Functionality", () => {
    beforeEach(() => {
      // Mock student at home
      mockUseStudentData.mockReturnValue({
        student: mockStudentAtHome,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
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

    it("allows checkin without selecting an active room or NFC device", async () => {
      mockSchoolCheckinStudent.mockResolvedValue({});

      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("checkin-button"));

      const confirmButton = await screen.findByTestId("modal-confirm");
      expect(confirmButton).toBeEnabled();
      // Im Dialog steht keine Raum- oder Geräteauswahl. Innerhalb des Dialogs
      // geprüft: die Reiterleiste des Seitengerüsts rendert unter sm selbst
      // eine Auswahlliste.
      expect(
        within(screen.getByTestId("modal-kind-anmelden")).queryByRole(
          "combobox",
        ),
      ).not.toBeInTheDocument();

      await act(async () => {
        fireEvent.click(confirmButton);
      });

      await waitFor(() => {
        expect(mockSchoolCheckinStudent).toHaveBeenCalledWith("1", "in");
        expect(mockRefreshData).toHaveBeenCalled();
        expect(mockToastSuccess).toHaveBeenCalled();
      });
    });

    it("shows the error in the dialog when checkin fails", async () => {
      const failure = new ApiError("Checkin failed", 500, {
        code: "general.server",
      });
      mockSchoolCheckinStudent.mockRejectedValue(failure);

      render(<StudentDetailPage />);

      const checkinButton = screen.getByTestId("checkin-button");
      fireEvent.click(checkinButton);

      await waitFor(() => {
        expect(screen.getByTestId("modal-kind-anmelden")).toBeInTheDocument();
      });

      const confirmButton = screen.getByTestId("modal-confirm");
      await act(async () => {
        fireEvent.click(confirmButton);
      });

      // Der Dialog bleibt offen; ein Toast läge hinter seinem Hintergrund.
      const dialog = screen.getByTestId("modal-kind-anmelden");
      expect(
        await within(dialog).findByText(
          catalogText("general.server", "die Anwesenheit"),
        ),
      ).toBeInTheDocument();
      // Der Bestätigen-Knopf wiederholt mit dem aktuellen Stand.
      expect(within(dialog).getByTestId("modal-confirm")).toBeEnabled();
      expect(mockToastError).not.toHaveBeenCalled();
    });
  });

  describe("URL Parameters", () => {
    it("uses default referrer when no 'from' param is present", () => {
      render(<StudentDetailPage />);

      const backButton = screen.getByTestId("back-button");
      expect(backButton).toHaveAttribute(
        "data-referrer",
        "/test-tenant/students/search",
      );
    });

    it("falls back for an external referrer from URL params", () => {
      mockSearchParams.set("from", "//attacker.example");

      render(<StudentDetailPage />);

      const backButton = screen.getByTestId("back-button");
      expect(backButton).toHaveAttribute(
        "data-referrer",
        "/test-tenant/students/search",
      );
    });

    it.each(["/messages", "/absences", "/ogs-groups"])(
      "keeps %s as a valid referrer",
      (referrer) => {
        mockSearchParams.set("from", referrer);

        render(<StudentDetailPage />);

        expect(screen.getByTestId("back-button")).toHaveAttribute(
          "data-referrer",
          `/test-tenant${referrer}`,
        );
      },
    );
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
        hasSickExcusedWriteAccess: true,
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

    it("does not emit a toast when planned sickness saving fails", async () => {
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

      expect(mockToastError).not.toHaveBeenCalled();
    });

    it("does not emit a toast for an atomic conflict", async () => {
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

      await waitFor(() =>
        expect(mockCreateStudentStatusDays).toHaveBeenCalled(),
      );
      expect(mockToastWarning).not.toHaveBeenCalled();
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
        hasSickExcusedWriteAccess: true,
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

    it("shows a failed sick toggle in the dialog, not as a toast", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, sick: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
        supervisors: [],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });
      mockUpdateStudent.mockRejectedValue(
        new ApiError("boom", 500, {
          code: "general.server",
          instance: "req-sick",
        }),
      );

      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("sick-toggle-button"));
      const dialog = await screen.findByTestId("modal-krankmeldung-aufheben");
      await act(async () => {
        fireEvent.click(within(dialog).getByTestId("modal-confirm"));
      });

      expect(
        await within(dialog).findByText(
          catalogText("general.server", "die Krankmeldung"),
        ),
      ).toBeInTheDocument();
      // Der Bestätigen-Knopf wiederholt mit dem aktuellen Stand.
      expect(within(dialog).getByTestId("modal-confirm")).toBeEnabled();
      expect(mockShowActionError).not.toHaveBeenCalled();
      expect(mockToastError).not.toHaveBeenCalled();
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
        hasSickExcusedWriteAccess: false,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      // Anker: die eingeschränkte Ansicht ist geladen, nicht das Skelett.
      expect(screen.getByTestId("supervisors-card")).toBeInTheDocument();
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
        hasSickExcusedWriteAccess: true,
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
        hasSickExcusedWriteAccess: true,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("sick-report-section")).toBeInTheDocument();
    });

    // The planning dialog reads the child's existing status days before it
    // saves. Gating that read on full access left it blind for absence-only
    // staff: planned days stayed invisible and could not be cleared.
    it("loads status days for absence-only staff", async () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      // Own cache so the shared SWR dedupe window of an earlier render in this
      // file cannot swallow the request under test.
      render(
        <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
          <StudentDetailPage />
        </SWRConfig>,
      );

      await waitFor(() => {
        expect(mockFetchStudentStatusDays).toHaveBeenCalled();
      });
    });

    it("does not load status days without read or absence access", () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: false,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: false,
        hasSickExcusedWriteAccess: false,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(
        <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
          <StudentDetailPage />
        </SWRConfig>,
      );

      // Anker: die eingeschränkte Ansicht ist geladen, nicht das Skelett.
      expect(screen.getByTestId("supervisors-card")).toBeInTheDocument();
      expect(mockFetchStudentStatusDays).not.toHaveBeenCalled();
    });

    // "Ab Uhrzeit" writes a pickup exception, which needs users:update plus
    // full care access — neither of which the absence permission grants.
    it("hides the partial excusal option for absence-only staff", async () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: false,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
        supervisors: [{ name: "Frau Schmidt" }],
        myGroups: ["1"],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-excused"),
        ).toHaveAttribute("data-can-plan-partial", "false");
      });
    });

    it("offers the partial excusal option to staff with Stammdaten write access", async () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "1",
            permissions: ["config:manage", "users:update", "users:read"],
          },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);
      fireEvent.click(screen.getByTestId("excused-toggle-button"));

      await waitFor(() => {
        expect(
          screen.getByTestId("planned-status-modal-excused"),
        ).toHaveAttribute("data-can-plan-partial", "true");
      });
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

    it("does not emit a toast when planned excusal saving fails", async () => {
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
      expect(mockToastError).not.toHaveBeenCalled();
    });

    it("shows aufheben modal when student is already excused", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, excused: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
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

    describe("lifting today's excusal (#3854)", () => {
      const renderExcusedToday = () => {
        mockUseStudentData.mockReturnValue({
          student: { ...mockStudent, excused: true },
          loading: false,
          error: null,
          hasFullAccess: true,
          hasWriteAccess: true,
          hasAbsenceWriteAccess: true,
          hasSickExcusedWriteAccess: true,
          supervisors: [],
          myGroups: ["1"],
          myGroupRooms: [],
          mySupervisedRooms: [],
          refreshData: mockRefreshData,
        });
        // Own cache: the status days of an earlier render must not leak in.
        render(
          <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
            <StudentDetailPage />
          </SWRConfig>,
        );
      };
      const openLiftDialog = async () => {
        fireEvent.click(screen.getByTestId("excused-toggle-button"));
        return screen.findByTestId("modal-entschuldigung-aufheben");
      };

      it("reports success only when the response no longer shows the excusal", async () => {
        mockUpdateStudent.mockResolvedValue({ excused: false });
        renderExcusedToday();
        await openLiftDialog();
        await act(async () => {
          fireEvent.click(screen.getByTestId("modal-confirm"));
        });
        await waitFor(() => {
          expect(mockUpdateStudent).toHaveBeenCalledWith("1", {
            excused: false,
          });
          expect(mockToastSuccess).toHaveBeenCalled();
        });
      });

      it("keeps the dialog open and explains when the excusal stays", async () => {
        mockUpdateStudent.mockResolvedValue({ excused: true });
        renderExcusedToday();
        const dialog = await openLiftDialog();
        await act(async () => {
          fireEvent.click(within(dialog).getByTestId("modal-confirm"));
        });
        expect(
          await within(dialog).findByText(
            /Die Entschuldigung für heute ist noch eingetragen\./,
          ),
        ).toBeInTheDocument();
        expect(mockToastSuccess).not.toHaveBeenCalled();
        expect(mockToastError).not.toHaveBeenCalled();
      });

      it("names a parent's Abmeldung in the dialog", async () => {
        mockFetchStudentStatusDays.mockResolvedValue([
          {
            id: "7",
            student_id: "1",
            date: "2026-09-09",
            status: "excused",
            label: "Entschuldigt",
            reported_at: "2026-09-09T06:30:00Z",
            cleared_at: null,
            source: "parent",
            created_at: "2026-09-09T06:30:00Z",
            updated_at: "2026-09-09T06:30:00Z",
          },
        ]);
        renderExcusedToday();
        const dialog = await openLiftDialog();
        expect(
          await within(dialog).findByText(
            /Die Abmeldung für heute kam von den Eltern\./,
          ),
        ).toBeInTheDocument();
      });

      it("does not name the parents for a staff excusal", async () => {
        renderExcusedToday();
        const dialog = await openLiftDialog();
        expect(
          within(dialog).queryByText(/kam von den Eltern/),
        ).not.toBeInTheDocument();
      });

      it("opens the list of entered days from the lift dialog", async () => {
        renderExcusedToday();
        const dialog = await openLiftDialog();
        fireEvent.click(
          within(dialog).getByRole("button", {
            name: "Alle entschuldigten Tage ansehen",
          }),
        );
        expect(
          await screen.findByTestId("planned-status-modal-excused"),
        ).toBeInTheDocument();
        expect(
          screen.queryByTestId("modal-entschuldigung-aufheben"),
        ).not.toBeInTheDocument();
        expect(mockUpdateStudent).not.toHaveBeenCalled();
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
        hasSickExcusedWriteAccess: true,
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
        hasSickExcusedWriteAccess: true,
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
        hasSickExcusedWriteAccess: true,
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
        hasSickExcusedWriteAccess: true,
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

    it("shows the error in the dialog if the switch request fails", async () => {
      const failure = new ApiError("boom", 500, { code: "general.server" });
      mockUpdateStudent.mockRejectedValue(failure);
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, excused: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
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
      // Der Dialog bleibt offen; ein Toast läge hinter seinem Hintergrund.
      const dialog = screen.getByTestId("modal-als-krank-melden?");
      expect(
        await within(dialog).findByText(
          catalogText("general.server", "die Änderung des Status"),
        ),
      ).toBeInTheDocument();
      // Der Bestätigen-Knopf wiederholt mit dem aktuellen Stand.
      expect(within(dialog).getByTestId("modal-confirm")).toBeEnabled();
      expect(mockToastError).not.toHaveBeenCalled();
    });

    it("closes the switch dialog on cancel without calling the API", async () => {
      mockUseStudentData.mockReturnValue({
        student: { ...mockStudent, sick: true },
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
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
    it("hides check-in actions without users:checkin", () => {
      vi.mocked(useSession).mockReturnValue({
        data: { user: { id: "1", permissions: ["config:manage"] } },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      // Anker: die Aktionsleiste ist geladen, nur Ein-/Auschecken fehlt.
      expect(screen.getByTestId("sick-report-section")).toBeInTheDocument();
      expect(screen.queryByTestId("checkout-section")).not.toBeInTheDocument();
      expect(screen.queryByTestId("checkin-section")).not.toBeInTheDocument();
    });

    it("shows checkout with users:checkin without group or room access", () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudent,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
        supervisors: [],
        myGroups: [],
        myGroupRooms: [],
        mySupervisedRooms: [],
        refreshData: mockRefreshData,
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("checkout-section")).toBeInTheDocument();
      expect(screen.queryByTestId("checkin-section")).not.toBeInTheDocument();
    });

    it("shows checkin with users:checkin without group or room access", () => {
      mockUseStudentData.mockReturnValue({
        student: mockStudentAtHome,
        loading: false,
        error: null,
        hasFullAccess: true,
        hasWriteAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: true,
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
      hasSickExcusedWriteAccess: false,
      supervisors: [{ name: "Frau Schmidt" }],
      myGroups: ["1"],
      myGroupRooms: [],
      mySupervisedRooms: [],
      refreshData: mockRefreshData,
    };

    // Die Reiter des Seitengerüsts sind einfache Schaltflächen und schalten
    // auf click (kein Radix, das auf pointerDown reagiert). Seit der Kindakte
    // höchstens vier Seitenreiter trägt (BAUARTEN-SPEC, Teil 3), stehen die
    // selten gebrauchten als Menüeinträge hinter dem Reiter „Verwaltung" —
    // die Helfer unten finden einen Bereich in beiden Formen.
    const openBundledTabs = () => {
      // Der gebündelte Reiter meldet sich als Reiter, nicht als Schaltfläche.
      const trigger = screen.queryByRole("tab", { name: "Verwaltung" });
      if (trigger && trigger.getAttribute("aria-expanded") !== "true") {
        fireEvent.click(trigger);
      }
    };
    const queryTab = (name: string) => {
      const direct = screen.queryByRole("tab", { name });
      if (direct) return direct;
      openBundledTabs();
      return screen.queryByRole("menuitem", { name });
    };
    const getTab = (name: string) => {
      const found = queryTab(name);
      if (!found) throw new Error(`Bereich "${name}" nicht gefunden`);
      return found;
    };
    const selectTab = (name: string) => {
      fireEvent.click(getTab(name));
    };

    it("renders all section tabs in the full access view", () => {
      render(<StudentDetailPage />);

      expect(getTab("Stammdaten")).toBeInTheDocument();
      expect(getTab("Erziehungsberechtigte")).toBeInTheDocument();
      expect(getTab("Betreuungszeiten")).toBeInTheDocument();
      expect(getTab("Anmeldungen")).toBeInTheDocument();
      expect(getTab("Historie")).toBeInTheDocument();
    });

    // ADR 0091: the role „Gast“ opens the child with students:read only.
    // Parent data, notes and messages stay on users:read, so those tabs and
    // the guardian manager are absent instead of answering 403.
    it.each([
      ["full", true],
      ["limited", false],
    ])(
      "hides parent data, notes and messages with students:read only (%s view)",
      (_view, hasFullAccess) => {
        vi.mocked(useSession).mockReturnValue({
          data: {
            user: { id: "1", permissions: ["students:read"] },
          },
          status: "authenticated",
        } as ReturnType<typeof useSession>);
        mockUseStudentData.mockReturnValue({
          ...limitedAccess,
          hasFullAccess,
        });

        render(<StudentDetailPage />);

        expect(getTab("Stammdaten")).toBeInTheDocument();
        expect(getTab("Historie")).toBeInTheDocument();
        for (const name of [
          "Erziehungsberechtigte",
          "Notizen",
          "Nachrichten",
        ]) {
          expect(queryTab(name)).not.toBeInTheDocument();
        }
        expect(
          screen.queryByTestId("guardian-manager"),
        ).not.toBeInTheDocument();
      },
    );

    // ADR 0091: the backend may report full access and absence write for
    // the Gast, but the routes behind Betreuungszeiten, pickup, arrival and
    // status days need users:read, absence writes users:update or
    // users:absence. Without them the page asks for nothing and offers no
    // action: no tab, no request, no „Klassenfahrt planen“, no empty row.
    it("loads and offers nothing a students:read session cannot use", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: { id: "1", permissions: ["students:read"] },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);
      vi.mocked(fetchStudentPickupData).mockClear();
      mockUseStudentData.mockReturnValue({
        ...limitedAccess,
        hasFullAccess: true,
        hasAbsenceWriteAccess: true,
        hasSickExcusedWriteAccess: false,
      });

      render(<StudentDetailPage />);

      // Anker: die Akte ist geladen.
      expect(getTab("Stammdaten")).toBeInTheDocument();
      expect(queryTab("Betreuungszeiten")).not.toBeInTheDocument();
      expect(
        screen.queryByTestId("care-schedule-manager"),
      ).not.toBeInTheDocument();
      expect(screen.queryByText("Klassenfahrt planen")).not.toBeInTheDocument();
      expect(
        screen.queryByLabelText("Schnellaktionen"),
      ).not.toBeInTheDocument();
      expect(fetchStudentPickupData).not.toHaveBeenCalled();
    });

    it("shows the Änderungsprotokoll tab with users:update", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "1",
            permissions: ["config:manage", "users:update", "users:read"],
          },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(getTab("Änderungsprotokoll")).toBeInTheDocument();
    });

    it("hides the Änderungsprotokoll tab without users:update or users:absence", () => {
      // Ohne users:update und users:absence würde der Aggregations-Endpunkt
      // 403 antworten, also gibt es den Reiter gar nicht erst (#2437).
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: { id: "1", permissions: ["config:manage", "users:read"] },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);
      render(<StudentDetailPage />);

      // Anker: die geladene volle Ansicht. Nur sie rendert die
      // Betreuungszeiten; das Skelett zeigt alle Reiter, aber keinen Inhalt,
      // und die eingeschränkte Ansicht hat keine Betreuungszeiten.
      expect(screen.getByTestId("care-schedule-manager")).toBeInTheDocument();
      expect(queryTab("Änderungsprotokoll")).not.toBeInTheDocument();
    });

    // #5025: eine Betreuungskraft hat users:absence, aber kein users:delete.
    // Die Liste aller Abmeldungen der Schule verlangt users:delete und
    // antwortet ihr mit 403; das Änderungsprotokoll liest deshalb nur die
    // abgeschlossenen Abmeldungen dieses Kindes über die Kind-Route.
    describe("Änderungsprotokoll ohne users:delete (#5025)", () => {
      const json = (status: number, body: unknown) =>
        new Response(JSON.stringify(body), {
          status,
          headers: { "Content-Type": "application/json" },
        });
      const requestedPaths = () =>
        vi
          .mocked(globalThis.fetch)
          .mock.calls.map(([input]) =>
            new URL(String(input), "http://localhost").pathname.toString(),
          );

      beforeEach(() => {
        const original = globalThis.fetch;
        vi.spyOn(globalThis, "fetch").mockImplementation(
          async (input, init) => {
            const path = new URL(String(input), "http://localhost").pathname;
            if (path === "/api/students/change-requests") {
              return json(200, { data: { items: [] } });
            }
            if (path === "/api/students/1/care-withdrawals") {
              return json(200, {
                data: {
                  items: [
                    {
                      id: "withdrawal-done",
                      student_id: "1",
                      first_name: "Max",
                      last_name: "Mustermann",
                      school_class: "3a",
                      first_bookingless_day: "2026-09-01",
                      urgency: "overdue",
                      state: "resolved",
                      outcome: "care_ended",
                      resolved_at: "2026-09-02T10:00:00+02:00",
                    },
                  ],
                  total: 1,
                  page: 1,
                  page_size: 25,
                },
              });
            }
            if (path.startsWith("/api/students/care-withdrawals")) {
              return json(403, {
                status: 403,
                code: "general.permission",
                detail: "forbidden",
              });
            }
            return original(input, init);
          },
        );
        vi.mocked(useSession).mockReturnValue({
          data: {
            user: { id: "7", permissions: ["users:read", "users:absence"] },
          },
          status: "authenticated",
        } as ReturnType<typeof useSession>);
        mockSearchParams.set("tab", "aenderungsprotokoll");
      });

      afterEach(() => {
        vi.mocked(globalThis.fetch).mockRestore();
      });

      it("loads the child's completed withdrawals in the full view", async () => {
        render(<StudentDetailPage />);

        // Anker: die abgeschlossene Abmeldung aus der Kind-Route steht im
        // Protokoll; ein 403 der Schul-Liste hätte stattdessen den Ladefehler
        // „die Liste der Abmeldungen“ gezeigt (siehe Test darunter).
        expect(await screen.findByText("Betreuung beendet")).toBeVisible();
        expect(requestedPaths()).toContain("/api/students/1/care-withdrawals");
        expect(
          requestedPaths().some((path) =>
            path.startsWith("/api/students/care-withdrawals"),
          ),
        ).toBe(false);
        expect(
          screen.queryByText(/Liste der Abmeldungen/),
        ).not.toBeInTheDocument();
      });

      // Mit users:delete oder als Admin bleibt es beim Protokoll mit den
      // Abmeldungen des Kindes (#5030); auch hier fragt die Kindakte nur die
      // Kind-Route.
      it.each([
        ["users:delete", ["users:read", "users:update", "users:delete"]],
        ["admin", ["admin:*"]],
      ])(
        "keeps the child's withdrawals with %s",
        async (account, permissions) => {
          vi.mocked(useSession).mockReturnValue({
            data: { user: { id: `delete-${account}`, permissions } },
            status: "authenticated",
          } as ReturnType<typeof useSession>);

          render(<StudentDetailPage />);

          // Anker: die abgeschlossene Abmeldung steht im Protokoll.
          expect(await screen.findByText("Betreuung beendet")).toBeVisible();
          expect(requestedPaths()).toContain(
            "/api/students/1/care-withdrawals",
          );
          // Die Kopfkarte fragt mit users:delete die offene Abmeldung über
          // die Schul-Liste ab; der Verlauf des Protokolls nicht mehr.
          const resolvedFromSchoolList = vi
            .mocked(globalThis.fetch)
            .mock.calls.map(
              ([input]) => new URL(String(input), "http://localhost"),
            )
            .filter(
              (url) =>
                url.pathname === "/api/students/care-withdrawals" &&
                url.searchParams.get("state") === "resolved",
            );
          expect(resolvedFromSchoolList).toEqual([]);
        },
      );

      // users:absence ist ein Schreibrecht und darf ohne users:read weder das
      // Änderungsprotokoll noch dessen Datenquellen öffnen.
      it("hides the request log for absence-only staff", async () => {
        vi.mocked(useSession).mockReturnValue({
          data: { user: { id: "7", permissions: ["users:absence"] } },
          status: "authenticated",
        } as ReturnType<typeof useSession>);
        mockUseStudentData.mockReturnValue({
          ...limitedAccess,
          hasFullAccess: true,
        });

        render(<StudentDetailPage />);

        // Anker: die eingeschränkte Ansicht hat keine Betreuungszeiten.
        expect(await screen.findByTestId("readonly-name")).toHaveTextContent(
          "Max Mustermann",
        );
        expect(
          screen.queryByTestId("care-schedule-manager"),
        ).not.toBeInTheDocument();
        expect(queryTab("Änderungsprotokoll")).not.toBeInTheDocument();
        expect(
          requestedPaths().some((path) =>
            path.startsWith("/api/students/care-withdrawals"),
          ),
        ).toBe(false);
        expect(requestedPaths()).not.toContain(
          "/api/students/1/care-withdrawals",
        );
      });

      it("loads the request log without withdrawals in the limited view", async () => {
        // Ein eigenes Konto: der SWR-Cache der Liste gilt je Konto, sonst
        // käme die Seite aus dem Cache des vorigen Tests ohne Abruf.
        vi.mocked(useSession).mockReturnValue({
          data: {
            user: { id: "8", permissions: ["users:read", "users:absence"] },
          },
          status: "authenticated",
        } as ReturnType<typeof useSession>);
        mockUseStudentData.mockReturnValue(limitedAccess);

        render(<StudentDetailPage />);

        await waitFor(() =>
          expect(requestedPaths()).toContain("/api/students/change-requests"),
        );
        // Anker: das Protokoll ist geladen und zeigt seinen Leerzustand.
        expect(
          await screen.findByText("Noch keine entschiedenen Anfragen."),
        ).toBeInTheDocument();
        expect(
          requestedPaths().some((path) => path.includes("care-withdrawals")),
        ).toBe(false);
        expect(
          screen.queryByText(/Liste der Abmeldungen/),
        ).not.toBeInTheDocument();
      });
    });

    it("hides the enrollment tab without config:manage", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: { id: "1", permissions: ["config:read", "users:read"] },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      // Anker: die geladene volle Ansicht (Betreuungszeiten).
      expect(screen.getByTestId("care-schedule-manager")).toBeInTheDocument();
      expect(queryTab("Anmeldungen")).not.toBeInTheDocument();
    });

    it("hides the Betreuungsplan tab without schedules:read", () => {
      // Default mock session is config:manage only -> no schedules:read.
      render(<StudentDetailPage />);

      // Anker: die geladene volle Ansicht (Betreuungszeiten).
      expect(screen.getByTestId("care-schedule-manager")).toBeInTheDocument();
      expect(
        screen.queryByRole("tab", { name: "Betreuungsplan" }),
      ).not.toBeInTheDocument();
    });

    it("shows the Dokumente tab with student_accidents:manage", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: { id: "1", permissions: ["student_accidents:manage"] },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tab", { name: "Dokumente" }),
      ).toBeInTheDocument();
    });

    it("shows the Betreuungsplan tab with schedules:read", () => {
      vi.mocked(useSession).mockReturnValue({
        data: {
          user: {
            id: "1",
            permissions: ["config:manage", "schedules:read", "users:read"],
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
            id: "1",
            permissions: ["config:manage", "schedules:read", "users:read"],
          },
        },
        status: "authenticated",
      } as ReturnType<typeof useSession>);

      render(<StudentDetailPage />);

      // Anker: Inhalt der geladenen eingeschränkten Ansicht.
      expect(screen.getByTestId("supervisors-card")).toBeInTheDocument();
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
          user: { id: "1", permissions: ["schedules:read", "users:read"] },
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

      expect(queryTab("Betreuungszeiten")).not.toBeInTheDocument();
      expect(getTab("Stammdaten")).toBeInTheDocument();
      expect(getTab("Historie")).toBeInTheDocument();
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
        hasSickExcusedWriteAccess: false,
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
        hasSickExcusedWriteAccess: true,
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

  describe("Holiday guest (#4666)", () => {
    it("shows only the holiday guest sheet instead of the sections of the record", () => {
      mockUseStudentData.mockReturnValue({
        ...mockUseStudentData(),
        student: { ...mockStudent, school_class: "Feriengast" },
      });

      render(<StudentDetailPage />);

      expect(screen.getByTestId("holiday-guest-section")).toHaveTextContent(
        "1:true",
      );
      expect(
        screen.queryByRole("tablist", { name: "Bereiche der Kindakte" }),
      ).not.toBeInTheDocument();
    });

    it("keeps the sections of the record for a child of the school", () => {
      render(<StudentDetailPage />);

      expect(
        screen.getByRole("tablist", { name: "Bereiche der Kindakte" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByTestId("holiday-guest-section"),
      ).not.toBeInTheDocument();
    });
  });
});
