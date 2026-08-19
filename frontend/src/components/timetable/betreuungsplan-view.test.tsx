import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { GapInstance, TimetableTemplate } from "~/lib/timetable-types";

const {
  mockUseSession,
  mockToastSuccess,
  mockToastError,
  mockToastWarning,
  mockTenantMutate,
  mockUseSWRAuth,
  mockMaterialize,
  mockStart,
  mockComplete,
  mockCancel,
  mockApplyDeviations,
  mockPatchAttendance,
  mockDeleteCancelled,
  mockEndTemplate,
  mockArchiveTemplate,
  mockBootstrap,
  mockEventModalProps,
  mockLoggerWarn,
  mockLoggerError,
  mockListPhases,
} = vi.hoisted(() => ({
  mockUseSession: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockToastWarning: vi.fn(),
  mockTenantMutate: vi.fn(),
  mockUseSWRAuth: vi.fn(),
  mockMaterialize: vi.fn(),
  mockStart: vi.fn(),
  mockComplete: vi.fn(),
  mockCancel: vi.fn(),
  mockApplyDeviations: vi.fn(),
  mockPatchAttendance: vi.fn(),
  mockDeleteCancelled: vi.fn(),
  mockEndTemplate: vi.fn(),
  mockArchiveTemplate: vi.fn(),
  mockBootstrap: vi.fn(),
  mockEventModalProps: vi.fn(),
  mockLoggerWarn: vi.fn(),
  mockLoggerError: vi.fn(),
  mockListPhases: vi.fn(),
}));

// useSearchParams spiegelt die echte URL wider; kombiniert mit dem
// replaceState->popstate-Spy (siehe beforeEach) macht das die View auf
// updateUrlParams reaktiv — genau wie Next.js' History-Instrumentierung in
// Produktion. Der Betreuungsplan nutzt useUrlParams mit syncPopstate.
vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    warning: mockToastWarning,
  }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    info: vi.fn(),
    error: mockLoggerError,
    warn: mockLoggerWarn,
  }),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: mockUseSWRAuth,
  useTenantMutate: () => mockTenantMutate,
}));

vi.mock("~/lib/hooks/use-timetable-day-hours", () => ({
  useTimetableDayHours: () => ({ dayStartHour: 9, dayEndHour: 17 }),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getWeek: vi.fn(),
    getTemplates: vi.fn(),
    getGaps: vi.fn(),
    materialize: mockMaterialize,
    replanWeek: vi.fn(),
    start: mockStart,
    complete: mockComplete,
    cancel: mockCancel,
    applyDeviations: mockApplyDeviations,
    patchAttendance: mockPatchAttendance,
    deleteCancelled: mockDeleteCancelled,
    endTemplate: mockEndTemplate,
    archiveTemplate: mockArchiveTemplate,
  },
}));

vi.mock("~/components/ui/modal", () => ({
  ConfirmationModal: ({
    isOpen,
    onClose,
    onConfirm,
    title,
    children,
    confirmText = "Bestätigen",
    cancelText = "Abbrechen",
    isConfirmLoading = false,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onConfirm: () => void;
    title: string;
    children: React.ReactNode;
    confirmText?: string;
    cancelText?: string;
    isConfirmLoading?: boolean;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        <h2>{title}</h2>
        <div>{children}</div>
        <button type="button" onClick={onClose}>
          {cancelText}
        </button>
        <button type="button" disabled={isConfirmLoading} onClick={onConfirm}>
          {isConfirmLoading ? "Wird geladen..." : confirmText}
        </button>
      </div>
    ) : null,
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    list: vi.fn(),
    bootstrap: mockBootstrap,
  },
}));

// Nur den Netzwerk-Fetcher mocken; getSettingValue & Co. bleiben die echten
// Implementierungen, damit die Tests nicht gegen eine driftende Kopie laufen.
vi.mock("~/lib/settings-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/settings-api")>()),
  fetchSettingsSchema: vi.fn(),
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: vi.fn(),
  },
}));

vi.mock("~/lib/enrollment-phase-api", () => ({
  listPhases: mockListPhases,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading</div>,
}));

vi.mock("~/components/timetable/timetable-add-menu", () => ({
  TimetableAddMenu: ({
    onAddInstance,
    onAddSeries,
  }: {
    onAddInstance: () => void;
    onAddSeries: () => void;
  }) => (
    <div>
      <button type="button" onClick={onAddInstance}>
        add-instance
      </button>
      <button type="button" onClick={onAddSeries}>
        add-series
      </button>
    </div>
  ),
}));

vi.mock("~/components/timetable/period-switcher-dropdown", () => ({
  PeriodSwitcherDropdown: ({
    periods,
    onCreate,
    onEdit,
    onSelect,
  }: {
    periods: Array<{ id: string; name: string }>;
    onCreate: () => void;
    onEdit: (period: { id: string; name: string }) => void;
    onSelect: (period: {
      id: string;
      name: string;
      startDate: string;
      endDate: string;
    }) => void;
  }) => (
    <div data-testid="period-switcher">
      <button type="button" onClick={onCreate}>
        create-period
      </button>
      {periods[0] && (
        <>
          <button type="button" onClick={() => onEdit(periods[0]!)}>
            edit-period
          </button>
          <button
            type="button"
            onClick={() =>
              onSelect({
                ...periods[0]!,
                startDate: "2026-05-04",
                endDate: "2026-05-08",
              })
            }
          >
            select-period
          </button>
        </>
      )}
    </div>
  ),
}));

vi.mock("~/components/timetable/conflict-warnings-banner", () => ({
  ConflictWarningsBanner: ({
    openConflicts,
    hiddenConflicts,
    periodLabel,
  }: {
    openConflicts: unknown[];
    hiddenConflicts: unknown[];
    periodLabel: string;
  }) => (
    <div data-testid="conflicts">
      {openConflicts.length}
      <span data-testid="conflicts-hidden">{hiddenConflicts.length}</span>
      <span data-testid="conflicts-period">{periodLabel}</span>
    </div>
  ),
}));

vi.mock("~/components/timetable/month-planner-grid", () => ({
  MonthPlannerGrid: ({
    onDayClick,
    onInstanceClick,
  }: {
    onDayClick: (date: string) => void;
    onInstanceClick?: (instance: { id: string }) => void;
  }) => (
    <div>
      <button type="button" onClick={() => onDayClick("2026-05-06")}>
        month-grid
      </button>
      <button type="button" onClick={() => onInstanceClick?.({ id: "42" })}>
        month-instance
      </button>
    </div>
  ),
}));

vi.mock("~/components/timetable/weekly-calendar-grid", () => ({
  WeeklyCalendarGrid: ({
    weekDays,
    instances,
    onInstanceClick,
    onSlotClick,
    gapInstanceIds,
  }: {
    weekDays: Date[];
    instances: Array<{ id: string; conflictWarnings: unknown[] }>;
    onInstanceClick: (instance: { id: string } | null) => void;
    onSlotClick?: (dateISO: string, hour: number) => void;
    gapInstanceIds?: ReadonlySet<string>;
  }) => (
    <div>
      <span data-testid="grid-week-days">{weekDays.length}</span>
      <span data-testid="grid-gap-ids">
        {gapInstanceIds ? [...gapInstanceIds].join(",") : ""}
      </span>
      <span data-testid="grid-conflict-count">
        {instances.reduce(
          (count, item) => count + item.conflictWarnings.length,
          0,
        )}
      </span>
      <button
        type="button"
        onClick={() => onInstanceClick(instances[0] ?? null)}
      >
        week-grid
      </button>
      {onSlotClick && (
        <button type="button" onClick={() => onSlotClick("2026-05-05", 14)}>
          slot-click
        </button>
      )}
    </div>
  ),
}));

vi.mock("~/components/timetable/template-list", () => ({
  TemplateList: ({
    templates,
    onCreate,
    onEdit,
    onApply,
    onArchive,
  }: {
    templates: TimetableTemplate[];
    onCreate: () => void;
    onEdit: (template: TimetableTemplate) => void;
    onApply: (template: TimetableTemplate) => void;
    onArchive: (template: TimetableTemplate) => void;
  }) => (
    <div>
      <button type="button" onClick={onCreate}>
        create-template
      </button>
      {templates[0] && (
        <>
          <button type="button" onClick={() => onEdit(templates[0]!)}>
            edit-template
          </button>
          <button type="button" onClick={() => onArchive(templates[0]!)}>
            archive-template
          </button>
          <button type="button" onClick={() => onApply(templates[0]!)}>
            apply-template
          </button>
        </>
      )}
    </div>
  ),
}));

vi.mock("~/components/timetable/instance-detail-modal", () => ({
  InstanceDetailModal: ({
    instance,
    onClose,
    onLifecycleAction,
    onDeleteCancelled,
    onDeleteFollowing,
    onEdit,
    onRepeat,
    onAttendancePatch,
    canManageStaffPool,
    canManage,
    fetchParticipantNames,
  }: {
    instance: { id: string } | null;
    onClose: () => void;
    onLifecycleAction: (
      action: "start" | "complete" | "cancel",
    ) => Promise<void>;
    onDeleteCancelled: (instance: { id: string }) => Promise<void>;
    onDeleteFollowing: (instance: {
      id: string;
      activityGroupId?: string;
      date?: string;
    }) => Promise<void>;
    onEdit: (instance: { id: string }) => void;
    onRepeat: (instance: { id: string }) => void;
    onAttendancePatch: (
      instanceId: string,
      studentId: string,
      body: { status: "present" },
    ) => Promise<void>;
    canManageStaffPool: boolean;
    canManage?: boolean;
    fetchParticipantNames?: boolean;
  }) =>
    instance ? (
      <div>
        <span data-testid="detail-can-manage-pool">
          {String(canManageStaffPool)}
        </span>
        <span data-testid="detail-can-manage">{String(canManage)}</span>
        <span data-testid="detail-fetch-participants">
          {String(fetchParticipantNames)}
        </span>
        <button type="button" onClick={onClose}>
          detail-close
        </button>
        <button type="button" onClick={() => void onLifecycleAction("start")}>
          detail-start
        </button>
        <button
          type="button"
          onClick={() => void onLifecycleAction("complete")}
        >
          detail-complete
        </button>
        <button type="button" onClick={() => void onLifecycleAction("cancel")}>
          detail-cancel
        </button>
        <button type="button" onClick={() => void onDeleteCancelled(instance)}>
          detail-delete
        </button>
        {/* Das echte Modal schluckt eine Rejection (bereits gemeldeter
            Fehler) und lässt den Auswahldialog offen — der Mock muss das
            nachbilden, sonst wird daraus eine Unhandled Rejection. */}
        <button
          type="button"
          onClick={() => void onDeleteFollowing(instance).catch(() => null)}
        >
          detail-delete-following
        </button>
        <button type="button" onClick={() => onEdit(instance)}>
          detail-edit
        </button>
        <button type="button" onClick={() => onRepeat(instance)}>
          detail-repeat
        </button>
        <button
          type="button"
          onClick={() =>
            void onAttendancePatch(instance.id, "21", { status: "present" })
          }
        >
          detail-attendance
        </button>
      </div>
    ) : null,
}));

vi.mock("~/components/timetable/timetable-event-modal", () => ({
  TimetableEventModal: (props: {
    isOpen: boolean;
    onClose: () => void;
    onSaved: (
      result:
        | { kind: "instance"; instance: { id: string } }
        | { kind: "series"; seriesId: string; linkedInstanceId?: string },
    ) => void;
    variant?: "full" | "quick";
    defaultDate?: string;
    defaultStartTime?: string;
    defaultEndTime?: string;
    closingDaysLoading?: boolean;
    calendarPeriods?: Array<{ id: string }>;
    defaultCalendarPeriodId?: string | null;
    initialSeries?: { id: string } | null;
    onDeleteSeries?: (
      template: { id: string },
      effectiveDate: string,
    ) => Promise<void>;
  }) => {
    mockEventModalProps(props);
    return props.isOpen ? (
      <div>
        <button type="button" onClick={props.onClose}>
          event-close
        </button>
        <button
          type="button"
          onClick={() =>
            props.onSaved({ kind: "instance", instance: { id: "42" } })
          }
        >
          event-save
        </button>
        <button
          type="button"
          onClick={() =>
            props.onSaved({
              kind: "series",
              seriesId: "7",
              linkedInstanceId: "42",
            })
          }
        >
          event-save-series
        </button>
        {props.initialSeries && props.onDeleteSeries && (
          <button
            type="button"
            onClick={() =>
              void props.onDeleteSeries?.(props.initialSeries!, "2026-05-06")
            }
          >
            event-delete-series
          </button>
        )}
      </div>
    ) : null;
  },
}));

vi.mock("~/components/timetable/calendar-period-modal", () => ({
  CalendarPeriodModal: ({
    isOpen,
    onClose,
    onSaved,
    onDeleted,
    usage,
  }: {
    isOpen: boolean;
    onClose: () => void;
    onSaved: () => void;
    onDeleted: () => void;
    usage?: {
      enrollmentPhaseCount: number;
      activityGroupCount: number;
      scheduleCount: number;
      studentEnrollmentCount: number;
      supervisorCount: number;
      activityInstanceCount: number;
    };
  }) =>
    isOpen ? (
      <div data-testid="period-modal" data-usage={JSON.stringify(usage)}>
        <button type="button" onClick={onClose}>
          period-close
        </button>
        <button type="button" onClick={onSaved}>
          period-save
        </button>
        <button type="button" onClick={onDeleted}>
          period-delete
        </button>
      </div>
    ) : null,
}));

import { BetreuungsplanView } from "./betreuungsplan-view";

const instance = {
  id: "42",
  date: "2026-05-04",
  startTime: "12:00",
  endTime: "13:00",
  title: "Mensa",
  status: "planned" as const,
  isSpontaneous: false,
  isLive: false,
  activityType: "care" as const,
  activityGroupId: "7",
  roomId: "3",
  roomName: "Mensa",
  staff: [
    {
      staffId: "11",
      isPrimary: true,
      isAbsent: true,
      isSubstitute: false,
    },
  ],
  students: [],
  studentIds: ["21"],
  staffCount: 1,
  absentStaffCount: 1,
  expectedStudentsCount: 1,
  notScheduledStudentsCount: 0,
  presentStudentsCount: 0,
  requiredStaffCount: 3,
  assignedStaffCount: 1,
  conflictWarnings: [
    {
      kind: "staff" as const,
      resourceId: "11",
      message:
        "Personal ist von 14:30–15:00 auch bei „Lernzeit“ eingeplant (anderer Raum).",
      canOverride: true,
      fingerprint: "aaaa1111bbbb2222cccc3333dddd4444",
      conflictingInstanceId: "99",
      conflictingTitle: "Lernzeit",
      overlapStart: "14:30",
      overlapEnd: "15:00",
    },
  ],
};

const period = {
  id: "5",
  name: "Schuljahr 2026/2027",
  periodType: "school_year" as const,
  startDate: "2026-05-01",
  endDate: "2026-12-31",
  weekCycleLength: 1,
  isActive: true,
  enrollmentPhaseCount: 1,
  activityGroupCount: 2,
  scheduleCount: 3,
  studentEnrollmentCount: 4,
  supervisorCount: 5,
  activityInstanceCount: 6,
};

const laterPeriod = {
  id: "6",
  name: "Schuljahr 2027/2028",
  periodType: "school_year" as const,
  startDate: "2027-08-01",
  endDate: "2028-07-31",
  weekCycleLength: 1,
  isActive: true,
  enrollmentPhaseCount: 0,
  activityGroupCount: 0,
  scheduleCount: 0,
  studentEnrollmentCount: 0,
  supervisorCount: 0,
  activityInstanceCount: 0,
};

const template: TimetableTemplate = {
  id: "7",
  name: "Yoga",
  type: "activity" as const,
  categoryId: "2",
  categoryName: "AG",
  isOpen: true,
  maxParticipants: 12,
  targetGroupType: "none",
  enrollmentCount: 8,
  supervisorCount: 1,
  requiredStaffCount: 3,
  assignedStaffCount: 1,
  studentIds: ["21"],
  staffIds: ["11"],
  weekdayAssignments: [],
  schedules: [
    {
      id: "9",
      weekday: 1,
      startTime: "14:00",
      endTime: "15:00",
      weekPattern: 0,
      calendarPeriodId: "5",
    },
  ],
};

const gap: GapInstance = {
  instanceId: "42",
  date: "2026-05-04",
  title: "Mensa",
  startTime: "12:00",
  endTime: "13:00",
  roomId: "3",
  status: "planned",
  assignedStaffCount: 1,
  absentStaffCount: 1,
  presentStaffCount: 1,
  plannedStaffCount: 3,
};

function setupSWR({
  periods = [period],
  templates = [template],
  settingsSchema = null,
  settingsSchemaLoading = false,
  careOfferingLink = null,
  gaps = [] as GapInstance[],
  gapsState = "ready" as "ready" | "loading" | "error",
  phases = [] as Array<Record<string, unknown>>,
  phasesState = "ready" as "ready" | "loading" | "error",
  closingDaysLoading = false,
  conflictAcks = [] as string[],
}: {
  periods?: Array<typeof period>;
  templates?: TimetableTemplate[];
  settingsSchema?: unknown;
  settingsSchemaLoading?: boolean;
  /** null = the planner admin cannot read enrollment (status "unknown"). */
  careOfferingLink?: { total: number; linked: number } | null;
  gaps?: GapInstance[];
  gapsState?: "ready" | "loading" | "error";
  phases?: Array<Record<string, unknown>>;
  phasesState?: "ready" | "loading" | "error";
  closingDaysLoading?: boolean;
  conflictAcks?: string[];
} = {}) {
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === null) return {};
    if (key === "timetable-care-offering-link") {
      return { data: careOfferingLink, isLoading: false };
    }
    if (key === "settings-schema") {
      return settingsSchemaLoading
        ? { isLoading: true }
        : { data: settingsSchema, isLoading: false };
    }
    if (key === "database-calendar-periods-list") {
      return { data: periods, isLoading: false };
    }
    if (key === "planning-closing-days") {
      return closingDaysLoading
        ? { data: undefined, isLoading: true }
        : { data: [], isLoading: false };
    }
    if (key === "timetable-enrollment-phases") {
      if (phasesState === "loading") return { isLoading: true };
      if (phasesState === "error") {
        return { error: new Error("phases kaputt"), isLoading: false };
      }
      return { data: phases, isLoading: false };
    }
    if (key === "timetable-staff-list") {
      return {
        data: [
          {
            id: "11",
            name: "Ada Absent",
            firstName: "Ada",
            lastName: "Absent",
            hasRfid: false,
            isTeacher: false,
            isSupervising: false,
            supervisions: [],
          },
        ],
      };
    }
    if (key === "timetable-student-list") {
      return { data: { students: [{ id: "21", name: "Max Kind" }] } };
    }
    if (key === "timetable-conflict-acks") {
      return { data: conflictAcks };
    }
    if (key.startsWith("timetable-gaps")) {
      if (gapsState === "loading") return { isLoading: true };
      if (gapsState === "error") {
        return { error: new Error("gaps kaputt"), isLoading: false };
      }
      return { data: { gaps, acknowledged: [] }, isLoading: false };
    }
    if (key.startsWith("timetable-templates")) {
      return { data: { templates }, isLoading: false };
    }
    if (key.startsWith("timetable-")) {
      return {
        data: { from: "2026-05-04", to: "2026-05-10", instances: [instance] },
        isLoading: false,
      };
    }
    return {};
  });
}

/** Setzt die URL, bevor gerendert wird (der syncPopstate-Hook liest daraus). */
function setUrl(search: string) {
  window.history.replaceState(
    null,
    "",
    `/acme/betreuungsplan${search ? `?${search}` : ""}`,
  );
}

function urlParams() {
  return new URLSearchParams(window.location.search);
}

/** Radix-Tab: Aktivierung per mousedown (Kit-Regel), nicht click. */
function selectTab(name: string) {
  fireEvent.mouseDown(screen.getByRole("tab", { name }));
}

let realReplaceState: typeof window.history.replaceState;

describe("BetreuungsplanView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.setSystemTime(new Date("2026-05-06T12:00:00"));
    // Leseansicht (#2283): Editier-Kontrollen hängen jetzt an
    // schedules:manage, die Kinderliste an users:read. Die Bestands-Tests
    // beschreiben Planer-Flows, also ist die Default-Session ein Admin;
    // die Leseansicht hat eigene Tests weiter unten.
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: {
        user: {
          permissions: [
            "schedules:read",
            "schedules:manage",
            "schedules:create",
            "users:read",
            "config:read",
          ],
        },
      },
    });
    mockTenantMutate.mockResolvedValue(undefined);
    mockMaterialize.mockResolvedValue({
      instancesCreated: 2,
      warnings: [],
    });
    mockBootstrap.mockResolvedValue({ periods: [], created: false });
    mockStart.mockResolvedValue({ warnings: [] });
    mockComplete.mockResolvedValue({});
    mockCancel.mockResolvedValue({});
    mockApplyDeviations.mockResolvedValue({
      instanceId: "42",
      cancelled: false,
      understaffedAck: false,
      affectedInstances: [{ instanceId: "42" }],
      warnings: [],
    });
    mockPatchAttendance.mockResolvedValue({});
    mockDeleteCancelled.mockResolvedValue({});
    mockEndTemplate.mockImplementation(
      (_templateId: string, body: { effective_date: string }) =>
        Promise.resolve({
          templateId: "7",
          effectiveDate: body.effective_date,
          deletedInstances: 3,
        }),
    );
    mockArchiveTemplate.mockResolvedValue({});
    mockListPhases.mockResolvedValue([]);
    setupSWR();

    // Der syncPopstate-Hook reagiert auf popstate, nicht auf manuelle
    // replaceState-Aufrufe. Next.js instrumentiert die History in Produktion;
    // im Test bilden wir das nach, indem replaceState ein popstate feuert.
    realReplaceState = window.history.replaceState.bind(window.history);
    vi.spyOn(window.history, "replaceState").mockImplementation(
      (data: unknown, unused: string, url?: string | URL | null) => {
        realReplaceState(data, unused, url);
        window.dispatchEvent(new PopStateEvent("popstate"));
      },
    );
    setUrl("");
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders the week view by default with the calendar as first content", () => {
    render(<BetreuungsplanView />);

    expect(
      screen.getByRole("heading", { name: "Betreuungsplan" }),
    ).toBeVisible();
    expect(screen.getByText("week-grid")).toBeVisible();
    expect(screen.getByTestId("grid-week-days")).toHaveTextContent("5");
    expect(screen.getByTestId("conflicts")).toHaveTextContent("1");
    expect(screen.getByTestId("conflicts-hidden")).toHaveTextContent("0");
    expect(screen.getByTestId("conflicts-period")).toHaveTextContent(
      "diese Woche",
    );
    // Kein Alt-URL-Parameter überlebt.
    expect(urlParams().has("week")).toBe(false);
    expect(urlParams().has("month")).toBe(false);
  });

  it("counts an acknowledged conflict as hidden, not open (#2139)", () => {
    setupSWR({ conflictAcks: ["aaaa1111bbbb2222cccc3333dddd4444"] });

    render(<BetreuungsplanView />);

    expect(screen.getByTestId("conflicts")).toHaveTextContent("0");
    expect(screen.getByTestId("conflicts-hidden")).toHaveTextContent("1");
  });

  it("passes schedules:manage to the staff-pool controls", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:manage"] } },
    });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByRole("button", { name: "week-grid" }));
    expect(screen.getByTestId("detail-can-manage-pool")).toHaveTextContent(
      "true",
    );
  });

  it("keeps staff-pool controls read-only without schedules:manage", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByRole("button", { name: "week-grid" }));
    expect(screen.getByTestId("detail-can-manage-pool")).toHaveTextContent(
      "false",
    );
  });

  it("hides export without the permission to read staff names", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    render(<BetreuungsplanView />);

    expect(
      screen.queryByRole("button", {
        name: "Betreuungsplan drucken oder exportieren",
      }),
    ).not.toBeInTheDocument();
  });

  // --- Leseansicht (#2283): schedules:read ohne schedules:manage ---

  it("hides the add menu and shows the read-only badge without schedules:manage", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    render(<BetreuungsplanView />);

    expect(screen.queryByText("add-instance")).not.toBeInTheDocument();
    expect(screen.getByText("Nur ansehen")).toBeInTheDocument();
  });

  it("removes conflict warnings from read-only calendar instances", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-conflict-count")).toHaveTextContent("0");
  });

  it("does not bootstrap periods for read-only staff", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    setupSWR({ periods: [] });
    render(<BetreuungsplanView />);

    expect(mockBootstrap).not.toHaveBeenCalled();
  });

  it("skips the tenant student roster without users:read", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    render(<BetreuungsplanView />);

    expect(mockUseSWRAuth).not.toHaveBeenCalledWith(
      "timetable-student-list",
      expect.anything(),
    );
  });

  it("forces the week view and hides the view switcher for read-only staff", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    setUrl("view=monat");
    render(<BetreuungsplanView />);

    // ?view=monat fällt still auf die Woche zurück, der Umschalter fehlt.
    expect(screen.getByText("week-grid")).toBeVisible();
    expect(screen.queryByText("Monat")).not.toBeInTheDocument();
    expect(screen.queryByText("Serien")).not.toBeInTheDocument();
  });

  it("hides print and skips gaps/conflict-acks fetches for read-only staff", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      // users:read allein schaltet den Druck-Export nicht mehr frei (#2283
      // Feedback: die Leseansicht zeigt nur die Woche, sonst nichts).
      data: { user: { permissions: ["schedules:read", "users:read"] } },
    });
    render(<BetreuungsplanView />);

    expect(
      screen.queryByRole("button", {
        name: "Betreuungsplan drucken oder exportieren",
      }),
    ).not.toBeInTheDocument();
    const swrKeys = mockUseSWRAuth.mock.calls.map((call) => call[0]);
    expect(
      swrKeys.some(
        (key) => typeof key === "string" && key.startsWith("timetable-gaps"),
      ),
    ).toBe(false);
    expect(swrKeys).not.toContain("timetable-conflict-acks");
  });

  it("passes read-only props to the detail modal", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByRole("button", { name: "week-grid" }));
    expect(screen.getByTestId("detail-can-manage")).toHaveTextContent("false");
    expect(screen.getByTestId("detail-fetch-participants")).toHaveTextContent(
      "true",
    );
  });

  it("falls back to the week for an unknown view value", () => {
    setUrl("view=quatsch");
    render(<BetreuungsplanView />);

    expect(screen.getByText("week-grid")).toBeVisible();
  });

  it("shows month and series views from the view param", () => {
    setUrl("view=monat");
    const { unmount } = render(<BetreuungsplanView />);
    expect(screen.getByText("month-grid")).toBeVisible();
    unmount();

    setUrl("view=serien");
    render(<BetreuungsplanView />);
    expect(screen.getByText("create-template")).toBeVisible();
  });

  it("writes d on week navigation and view on the segment switch", () => {
    render(<BetreuungsplanView />);

    // Aktuelle Woche (Mo 2026-05-04): Weiter -> Montag der Folgewoche.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(urlParams().get("d")).toBe("2026-05-11");
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(urlParams().get("d")).toBe("2026-05-04");

    selectTab("Monat");
    expect(urlParams().get("view")).toBe("monat");
    selectTab("Serien");
    expect(urlParams().get("view")).toBe("serien");
    selectTab("Woche");
    expect(urlParams().has("view")).toBe(false);
  });

  it("enables the Heute button only when the visible week differs from today's", () => {
    render(<BetreuungsplanView />);
    // In der laufenden Woche bleibt der Button sichtbar und ist deaktiviert
    // (#2031: die Navigationsgruppe behält ihre Geometrie).
    expect(screen.getByRole("button", { name: "Heute" })).toBeDisabled();

    setUrl("d=2026-09-14");
    render(<BetreuungsplanView />);
    const todayButton = screen.getAllByRole("button", { name: "Heute" })[0]!;
    fireEvent.click(todayButton);
    expect(urlParams().get("d")).toBe("2026-05-06");
  });

  it("snaps weekend deep links and the Heute target to the following Monday", () => {
    setUrl("d=2026-05-09");
    const { unmount } = render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("add-instance"));
    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({ defaultDate: "2026-05-11" }),
    );

    unmount();
    vi.setSystemTime(new Date("2026-05-09T12:00:00Z"));
    setUrl("d=2026-05-06");
    render(<BetreuungsplanView />);
    fireEvent.click(screen.getAllByRole("button", { name: "Heute" })[0]!);
    expect(urlParams().get("d")).toBe("2026-05-11");
  });

  it("does not query gaps for a finished Friday workweek on Saturday", () => {
    vi.setSystemTime(new Date("2026-05-09T12:00:00Z"));
    setUrl("view=woche&d=2026-05-04");
    render(<BetreuungsplanView />);

    const gapKeys = mockUseSWRAuth.mock.calls
      .map(([key]) => key)
      .filter(
        (key) => typeof key === "string" && key.startsWith("timetable-gaps"),
      );
    expect(gapKeys).toEqual([]);
  });

  it("switches to the week and sets d when a month day is clicked", () => {
    setUrl("view=monat");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("month-grid"));
    expect(urlParams().get("d")).toBe("2026-05-06");
    expect(urlParams().has("view")).toBe(false);
    expect(screen.getByText("week-grid")).toBeVisible();
  });

  it("keeps a weekend month anchor and opens retained month instances", () => {
    setUrl("view=monat&d=2026-05-31");
    render(<BetreuungsplanView />);

    expect(screen.getByText("Mai 2026")).toBeVisible();
    fireEvent.click(screen.getByText("month-instance"));
    expect(screen.getByText("detail-close")).toBeVisible();
  });

  it("opens and closes the slide-over via the block param", async () => {
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    expect(screen.getByText("detail-close")).toBeVisible();
    fireEvent.click(screen.getByText("detail-close"));

    await waitFor(() =>
      expect(screen.queryByText("detail-close")).not.toBeInTheDocument(),
    );
    expect(urlParams().has("block")).toBe(false);
  });

  it("marks open-gap instances on the week grid", () => {
    setupSWR({ gaps: [gap] });
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-gap-ids")).toHaveTextContent("42");
  });

  it("jumps to a gap: the jump list sets d and block and opens the slide-over", async () => {
    setupSWR({ gaps: [gap] });
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByRole("button", { name: /1 Lücke/ }));
    // Sprungliste-Zeile (Titel des Blocks) im Popover.
    const popover = screen.getByRole("region", { name: "Offene Lücken" });
    fireEvent.click(within(popover).getByRole("button", { name: /Mensa/ }));

    await waitFor(() => expect(screen.getByText("detail-close")).toBeVisible());
    expect(urlParams().get("d")).toBe("2026-05-04");
    expect(urlParams().get("block")).toBe("42");
  });

  it("shows a neutral checking chip while gaps load instead of 'Keine Lücken'", () => {
    setupSWR({ gapsState: "loading" });
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(screen.getByText("Lücken werden geprüft …")).toBeInTheDocument();
    expect(screen.queryByText("Keine Lücken")).not.toBeInTheDocument();
  });

  it("hides the gap chip when the gaps request fails", () => {
    setupSWR({ gapsState: "error" });
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(screen.queryByText("Keine Lücken")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Lücken werden geprüft …"),
    ).not.toBeInTheDocument();
  });

  it("shows a neutral chip while phases load instead of 'keine Anmeldung verknüpft'", () => {
    setupSWR({ phasesState: "loading" });
    render(<BetreuungsplanView />);

    expect(screen.getByText("Bedarf wird ermittelt …")).toBeInTheDocument();
    expect(
      screen.queryByText("Bedarf: keine Anmeldung verknüpft"),
    ).not.toBeInTheDocument();
  });

  it("hides the demand-origin chip when the phases request fails", () => {
    setupSWR({ phasesState: "error" });
    render(<BetreuungsplanView />);

    expect(screen.queryByText(/^Bedarf/)).not.toBeInTheDocument();
  });

  it("shows the demand-origin chip, linked to enrollment-phases when unresolved", () => {
    render(<BetreuungsplanView />);

    const chip = screen.getByText("Bedarf: keine Anmeldung verknüpft");
    // Tenant-bewusster Pfad (useTenantAwarePath) — im Pfad-Routing-Modus
    // muss der Slug vorangestellt sein.
    expect(chip.closest("a")).toHaveAttribute(
      "href",
      "/test-tenant/enrollment-phases",
    );
  });

  it("names the single matching phase in the demand-origin chip", () => {
    setupSWR({
      phases: [
        {
          id: "1",
          name: "Anmeldung HJ1",
          calendar_period_id: "5",
          service_start_date: "2026-08-01",
          service_end_date: "2027-07-31",
        },
      ],
    });
    render(<BetreuungsplanView />);

    const chip = screen.getByText("Bedarf: Anmeldung HJ1");
    expect(chip.closest("a")).toBeNull();
  });

  it("opens create/edit/delete period flows from the period chip", async () => {
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("create-period"));
    expect(screen.getByText("period-save")).toBeInTheDocument();
    fireEvent.click(screen.getByText("period-close"));
    expect(screen.queryByText("period-save")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("create-period"));
    fireEvent.click(screen.getByText("period-save"));
    await waitFor(() =>
      expect(mockTenantMutate).toHaveBeenCalledWith(
        "database-calendar-periods-list",
      ),
    );

    fireEvent.click(screen.getByText("edit-period"));
    expect(screen.getByText("period-delete")).toBeInTheDocument();
    expect(screen.getByTestId("period-modal")).toHaveAttribute(
      "data-usage",
      JSON.stringify({
        enrollmentPhaseCount: 1,
        activityGroupCount: 2,
        scheduleCount: 3,
        studentEnrollmentCount: 4,
        supervisorCount: 5,
        activityInstanceCount: 6,
      }),
    );
    fireEvent.click(screen.getByText("period-delete"));
    await waitFor(() =>
      expect(mockTenantMutate).toHaveBeenCalledWith(
        "database-calendar-periods-list",
      ),
    );
  });

  it("runs slide-over lifecycle, attendance and delete actions", async () => {
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-start"));
    await waitFor(() => expect(mockStart).toHaveBeenCalledWith("42"));
    fireEvent.click(screen.getByText("detail-complete"));
    await waitFor(() => expect(mockComplete).toHaveBeenCalledWith("42", []));
    fireEvent.click(screen.getByText("detail-cancel"));
    await waitFor(() => expect(mockCancel).toHaveBeenCalledWith("42"));
    fireEvent.click(screen.getByText("detail-attendance"));
    await waitFor(() =>
      expect(mockPatchAttendance).toHaveBeenCalledWith("42", "21", {
        status: "present",
      }),
    );

    fireEvent.click(screen.getByText("detail-delete"));
    await waitFor(() => expect(mockDeleteCancelled).toHaveBeenCalledWith("42"));
  });

  it("ends following instances from the slide-over", async () => {
    // Der Regeltermin lässt sich nur ab heute beenden, also muss der Termin
    // der Fixture (2026-05-04) hier "heute" sein.
    vi.setSystemTime(new Date("2026-05-04T08:00:00"));
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-delete-following"));
    await waitFor(() =>
      expect(mockEndTemplate).toHaveBeenCalledWith("7", {
        effective_date: "2026-05-04",
      }),
    );
  });

  it("refuses to end a series from a past occurrence", async () => {
    // Uhr steht auf 2026-05-06, der Termin der Fixture auf 2026-05-04: das
    // Backend lehnt ein effective_date in der Vergangenheit ab, also fängt
    // die View den Aufruf mit einer verständlichen Meldung ab.
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-delete-following"));
    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        expect.stringContaining("nur ab heute beendet werden"),
      ),
    );
    expect(mockEndTemplate).not.toHaveBeenCalled();
  });

  it("repeats an instance into a series from the slide-over", async () => {
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-repeat"));
    expect(screen.getByText("event-save-series")).toBeInTheDocument();
    fireEvent.click(screen.getByText("event-save-series"));
    await waitFor(() => expect(mockTenantMutate).toHaveBeenCalled());
  });

  it("handles series create, edit, apply, delete and archive", async () => {
    setUrl("view=serien");
    render(<BetreuungsplanView />);

    expect(screen.getByText("create-template")).toBeVisible();
    fireEvent.click(screen.getByText("create-template"));
    expect(screen.getByText("event-close")).toBeInTheDocument();
    fireEvent.click(screen.getByText("event-close"));

    fireEvent.click(screen.getByText("edit-template"));
    expect(screen.getByText("event-close")).toBeInTheDocument();
    fireEvent.click(screen.getByText("event-delete-series"));
    await waitFor(() =>
      expect(mockEndTemplate).toHaveBeenCalledWith("7", {
        effective_date: "2026-05-06",
      }),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Regeltermin ab 06.05.2026 gelöscht",
    );
    expect(mockTenantMutate).toHaveBeenCalledWith("timetable-templates-5");

    fireEvent.click(screen.getByText("edit-template"));
    fireEvent.click(screen.getByText("event-close"));

    fireEvent.click(screen.getByText("apply-template"));
    await waitFor(() => expect(mockMaterialize).toHaveBeenCalled());

    fireEvent.click(screen.getByText("archive-template"));
    expect(
      screen.getByRole("dialog", { name: "Regeltermin archivieren?" }),
    ).toHaveTextContent("Regeltermin „Yoga“");
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(mockArchiveTemplate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("archive-template"));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));
    await waitFor(() => expect(mockArchiveTemplate).toHaveBeenCalledWith("7"));
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Regeltermin "Yoga" archiviert',
    );
    expect(mockTenantMutate).toHaveBeenCalledWith("timetable-templates-5");
  });

  it("adds a one-off appointment from the + Neu menu", () => {
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("add-instance"));
    expect(screen.getByText("event-save")).toBeInTheDocument();
  });

  it("keeps archive confirmation open and reports errors", async () => {
    setUrl("view=serien");
    mockArchiveTemplate.mockRejectedValueOnce(new Error("Archivierung kaputt"));
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("archive-template"));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith("Archivierung kaputt"),
    );
    expect(
      screen.getByRole("dialog", { name: "Regeltermin archivieren?" }),
    ).toBeInTheDocument();
  });

  it("shows loading state while the session loads", () => {
    mockUseSession.mockReturnValue({ status: "loading" });

    render(<BetreuungsplanView />);

    // Chrome (PlanningContextBar title/navigation) renders immediately —
    // only the calendar-grid content region skeletonizes (showSkeleton
    // pattern, mirrors staff/page.tsx and rooms/page.tsx).
    expect(screen.getByText("Betreuungsplan")).toBeVisible();
    expect(screen.getByTestId("timetable-content-skeleton")).toBeVisible();
    expect(
      screen.queryByTestId("timetable-page-skeleton"),
    ).not.toBeInTheDocument();
    expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
  });

  it("shows a calendar-shaped skeleton while timetable data loads", () => {
    mockUseSWRAuth.mockImplementation((key: string | null) => {
      if (key === null) return {};
      if (key === "database-calendar-periods-list") {
        return { data: [period], isLoading: false };
      }
      if (key.startsWith("timetable-")) {
        return { isLoading: true };
      }
      return {};
    });
    setUrl("view=woche");

    render(<BetreuungsplanView />);

    expect(screen.getByTestId("timetable-content-skeleton")).toBeVisible();
    expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
  });

  it("bootstraps a default period exactly once when none exist", async () => {
    setupSWR({ periods: [] });
    mockBootstrap.mockResolvedValue({ periods: [period], created: true });

    const { rerender } = render(<BetreuungsplanView />);

    await waitFor(() => expect(mockBootstrap).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(mockTenantMutate).toHaveBeenCalledWith(
        "database-calendar-periods-list",
      ),
    );

    rerender(<BetreuungsplanView />);
    expect(mockBootstrap).toHaveBeenCalledTimes(1);
  });

  it("does not bootstrap when periods already exist", () => {
    render(<BetreuungsplanView />);

    expect(mockBootstrap).not.toHaveBeenCalled();
  });

  it("passes the series list period to the event modal", () => {
    setupSWR({ periods: [period, laterPeriod] });
    // d liegt im späteren Zeitraum -> sichtbarer Zeitraum = 6.
    setUrl("view=serien&d=2027-09-01");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("create-template"));

    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({
        isOpen: true,
        defaultCalendarPeriodId: "6",
        calendarPeriods: expect.arrayContaining([
          expect.objectContaining({ id: "6" }),
        ]),
      }),
    );
  });

  it("materializes a template-only period pin instead of the current period", async () => {
    setupSWR({
      periods: [period, laterPeriod],
      templates: [
        {
          ...template,
          calendarPeriodId: "6",
          schedules: template.schedules.map((schedule) => ({
            ...schedule,
            calendarPeriodId: undefined,
          })),
        },
      ],
    });
    setUrl("view=serien");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("apply-template"));

    await waitFor(() =>
      expect(mockMaterialize).toHaveBeenNthCalledWith(
        1,
        "2027-08-01",
        "2027-09-25",
      ),
    );
  });

  it("shows the period chip whenever at least one period exists", () => {
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("period-switcher")).toBeInTheDocument();
  });

  it("renders no period chip when no periods exist", () => {
    setupSWR({ periods: [] });
    render(<BetreuungsplanView />);

    expect(screen.queryByTestId("period-switcher")).not.toBeInTheDocument();
  });

  it("renders the disabled state when the timetable setting is off", () => {
    setupSWR({
      settingsSchema: {
        tabs: [
          {
            id: "operations",
            label: "Betrieb",
            categories: [
              {
                id: "timetable",
                label: "Betreuungsplan",
                items: [{ key: "timetable.enabled", value: false }],
              },
            ],
          },
        ],
      },
    });
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("timetable-disabled-state")).toBeVisible();
    expect(screen.getByText("Betreuungsplan ist deaktiviert")).toBeVisible();
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();
    expect(mockBootstrap).not.toHaveBeenCalled();
  });

  it("shows the skeleton while the settings schema loads", () => {
    setupSWR({ settingsSchemaLoading: true });
    render(<BetreuungsplanView />);

    // Chrome renders immediately; only the content region skeletonizes
    // while the settings schema (and therefore timetableDisabled) is
    // still unresolved.
    expect(screen.getByText("Betreuungsplan")).toBeVisible();
    expect(screen.getByTestId("timetable-content-skeleton")).toBeVisible();
    expect(
      screen.queryByTestId("timetable-page-skeleton"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();
  });

  it("logs a warning when the bootstrap fails with 403", async () => {
    setupSWR({ periods: [] });
    mockBootstrap.mockRejectedValue(
      Object.assign(new Error("permission denied"), { httpStatus: 403 }),
    );

    render(<BetreuungsplanView />);

    await waitFor(() =>
      expect(mockLoggerWarn).toHaveBeenCalledWith("periods_bootstrap_failed", {
        error: "permission denied",
        status: 403,
      }),
    );
    expect(mockLoggerError).not.toHaveBeenCalledWith(
      "periods_bootstrap_failed",
      expect.anything(),
    );
  });

  it("logs an error when the bootstrap fails for other reasons", async () => {
    setupSWR({ periods: [] });
    mockBootstrap.mockRejectedValue(new Error("backend down"));

    render(<BetreuungsplanView />);

    await waitFor(() =>
      expect(mockLoggerError).toHaveBeenCalledWith("periods_bootstrap_failed", {
        error: "backend down",
      }),
    );
  });

  it("opens the quick-create modal from an empty week slot", () => {
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("slot-click"));

    expect(screen.getByText("event-save")).toBeInTheDocument();
    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({
        isOpen: true,
        variant: "quick",
        defaultDate: "2026-05-05",
        defaultStartTime: "14:00",
        defaultEndTime: "15:00",
      }),
    );

    fireEvent.click(screen.getByText("event-close"));
    fireEvent.click(screen.getByText("add-instance"));
    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({
        isOpen: true,
        variant: "full",
        defaultStartTime: undefined,
        defaultEndTime: undefined,
      }),
    );
  });

  it("passes the closing-day loading state to the event modal", () => {
    // Der Termin-Editor öffnet sich nur noch für Planende (#2283); die
    // Schließtage lädt der Hook weiterhin über schedules:read.
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read", "schedules:manage"] } },
    });
    setupSWR({ closingDaysLoading: true });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("add-instance"));

    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({
        isOpen: true,
        closingDaysLoading: true,
      }),
    );
  });

  // --- Chrome-Abbau: Kalender als erstes Inhaltselement, kein Setup-Chrome ---

  it("renders no setup chrome and shows the calendar as the first content when a period exists", () => {
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    // Kalenderraster ist da; die abgebauten Karten (Overview/SetupGuide) nicht.
    expect(screen.getByText("week-grid")).toBeVisible();
    expect(
      screen.queryByText("Betreuungsplan im Blick"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Einrichtung")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Mit der Anmeldung verknüpfen"),
    ).not.toBeInTheDocument();
    // Kein Leerzustand-Hinweis, solange ein Zeitraum existiert.
    expect(
      screen.queryByText("Noch kein Planungszeitraum"),
    ).not.toBeInTheDocument();
  });

  it("shows the empty-period hint instead of the grid when no period exists, and its action opens the period modal", () => {
    setupSWR({ periods: [] });
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(screen.getByText("Noch kein Planungszeitraum")).toBeVisible();
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Planungszeitraum anlegen" }),
    );
    expect(screen.getByText("period-save")).toBeInTheDocument();
  });
});
