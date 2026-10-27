import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";
import {
  useTimetableEnabled,
  useWeekendFollowsFriday,
} from "~/lib/tenant-context";
import { fetchStudents } from "~/lib/student-api";
import { ApiError, transportFetch } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import type { GapInstance, TimetableTemplate } from "~/lib/timetable-types";

vi.unmock("swr");

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
  mockReopen,
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
  mockShowActionError,
  mockAcknowledgeConflict,
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
  mockReopen: vi.fn(),
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
  mockShowActionError: vi.fn(),
  mockAcknowledgeConflict: vi.fn(),
}));

// useSearchParams spiegelt die echte URL wider; kombiniert mit dem
// replaceState->popstate-Spy (siehe beforeEach) macht das die View auf
// updateUrlParams reaktiv — genau wie Next.js' History-Instrumentierung in
// Produktion. Der Betreuungsplan nutzt useUrlParams mit syncPopstate.
// Vaul (SlideOver) rendert in jsdom nichts. Derselbe Ersatz wie in
// components/ui/slide-over.test.tsx — die Struktur bleibt, nur die
// Animationsschicht fällt weg.
vi.mock("vaul", async () => {
  const React = await import("react");

  return {
    Drawer: {
      Root: ({
        children,
        open,
      }: {
        children: React.ReactNode;
        open?: boolean;
      }) => (open === false ? null : <div>{children}</div>),
      Portal: ({ children }: { children: React.ReactNode }) => <>{children}</>,
      Overlay: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Content: React.forwardRef<
        HTMLDivElement,
        React.HTMLAttributes<HTMLDivElement>
      >((props, ref) => <div ref={ref} {...props} />),
      Close: React.forwardRef<
        HTMLButtonElement,
        React.ButtonHTMLAttributes<HTMLButtonElement>
      >((props, ref) => <button ref={ref} {...props} />),
      Title: React.forwardRef<
        HTMLHeadingElement,
        React.HTMLAttributes<HTMLHeadingElement>
      >(({ children, ...props }, ref) => (
        <h2 ref={ref} {...props}>
          {children ?? "Titel"}
        </h2>
      )),
      Description: React.forwardRef<
        HTMLParagraphElement,
        React.HTMLAttributes<HTMLParagraphElement>
      >((props, ref) => <p ref={ref} {...props} />),
    },
  };
});

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(window.location.search),
}));

vi.mock("next-auth/react", () => ({
  useSession: mockUseSession,
}));

// Lade- und Dialogfehler laufen über die echten Hooks (#2516). Der Toast für
// Aktionen ohne Formular geht über denselben Katalog in mockToastError, mit
// „Wiederholen“ als Aktion, wie es useApiErrorDisplay im Provider tut.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    warning: mockToastWarning,
  }),
  useApiErrorDisplay: () => ({ show: mockShowActionError }),
}));

async function showActionErrorAsToast(
  err: unknown,
  options: { object: string; retry?: () => void },
) {
  const { presentError } = await import("~/lib/error-presentation");
  const presentation = presentError(err, options.object);
  mockToastError(
    presentation.message,
    presentation.retryable && options.retry
      ? { action: { label: "Wiederholen", onClick: options.retry } }
      : undefined,
  );
}

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
    reopen: mockReopen,
    applyDeviations: mockApplyDeviations,
    patchAttendance: mockPatchAttendance,
    deleteCancelled: mockDeleteCancelled,
    endTemplate: mockEndTemplate,
    archiveTemplate: mockArchiveTemplate,
    acknowledgeConflict: mockAcknowledgeConflict,
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
    disabled = false,
  }: {
    onAddInstance: () => void;
    onAddSeries: () => void;
    disabled?: boolean;
  }) => (
    <div>
      <button type="button" onClick={onAddInstance} disabled={disabled}>
        add-instance
      </button>
      <button type="button" onClick={onAddSeries} disabled={disabled}>
        add-series
      </button>
    </div>
  ),
}));

vi.mock("~/components/timetable/period-switcher-dropdown", () => ({
  PeriodSwitcherDropdown: ({
    periods,
    onCreate,
    onSelect,
  }: {
    periods: Array<{ id: string; name: string }>;
    onCreate: () => void;
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
    onHide,
  }: {
    openConflicts: unknown[];
    hiddenConflicts: unknown[];
    periodLabel: string;
    onHide: (entry: unknown) => Promise<void>;
  }) => (
    <div data-testid="conflicts">
      {openConflicts.length}
      <span data-testid="conflicts-hidden">{hiddenConflicts.length}</span>
      <span data-testid="conflicts-period">{periodLabel}</span>
      {openConflicts[0] !== undefined && (
        <button type="button" onClick={() => void onHide(openConflicts[0])}>
          conflict-hide
        </button>
      )}
    </div>
  ),
}));

vi.mock("~/components/timetable/month-planner-grid", () => ({
  MonthPlannerGrid: ({
    onDayClick,
    onInstanceClick,
    planningDisabledDateISOs,
  }: {
    onDayClick: (date: string) => void;
    onInstanceClick?: (instance: { id: string }) => void;
    planningDisabledDateISOs?: ReadonlySet<string>;
  }) => (
    <div>
      <span data-testid="month-disabled-dates">
        {planningDisabledDateISOs
          ? [...planningDisabledDateISOs].join(",")
          : ""}
      </span>
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
    planningDisabledDateISOs,
    emptyState,
  }: {
    weekDays: Date[];
    instances: Array<{ id: string; conflictWarnings: unknown[] }>;
    onInstanceClick: (instance: { id: string } | null) => void;
    onSlotClick?: (dateISO: string, hour: number) => void;
    gapInstanceIds?: ReadonlySet<string>;
    planningDisabledDateISOs?: ReadonlySet<string>;
    emptyState?: { title: string; description: string };
  }) => (
    <div>
      <span data-testid="grid-week-days">{weekDays.length}</span>
      <span data-testid="grid-gap-ids">
        {gapInstanceIds ? [...gapInstanceIds].join(",") : ""}
      </span>
      <span data-testid="grid-disabled-dates">
        {planningDisabledDateISOs
          ? [...planningDisabledDateISOs].join(",")
          : ""}
      </span>
      <span data-testid="grid-conflict-count">
        {instances.reduce(
          (count, item) => count + item.conflictWarnings.length,
          0,
        )}
      </span>
      {emptyState && (
        <div data-testid="grid-empty-state">
          <span>{emptyState.title}</span>
          <span>{emptyState.description}</span>
        </div>
      )}
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
    seriesPeriodId,
    seriesRosterMaintenance,
    studentsPending,
    studentsLoadError,
  }: {
    instance: { id: string; students?: Array<{ note?: string }> } | null;
    onClose: () => void;
    onLifecycleAction: (
      action: "start" | "complete" | "cancel" | "reopen",
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
    seriesPeriodId?: string;
    seriesRosterMaintenance?: unknown;
    studentsPending?: boolean;
    studentsLoadError?: { message?: string } | string | null;
  }) =>
    instance ? (
      <div>
        <span data-testid="detail-students-error">
          {typeof studentsLoadError === "object"
            ? (studentsLoadError?.message ?? "")
            : (studentsLoadError ?? "")}
        </span>
        <span data-testid="detail-students">
          {studentsPending ? "pending" : String(instance.students?.length ?? 0)}
        </span>
        <span data-testid="detail-student-note">
          {instance.students?.[0]?.note ?? ""}
        </span>
        <span data-testid="detail-can-manage-pool">
          {String(canManageStaffPool)}
        </span>
        <span data-testid="detail-can-manage">{String(canManage)}</span>
        <span data-testid="detail-fetch-participants">
          {String(fetchParticipantNames)}
        </span>
        <span data-testid="detail-series-period">{seriesPeriodId ?? ""}</span>
        <span data-testid="detail-series-maintenance">
          {seriesRosterMaintenance ? "known" : "missing"}
        </span>
        <button type="button" onClick={onClose}>
          detail-close
        </button>
        {/* Das echte Panel fängt die Rejection eines schon gemeldeten
            Fehlers (awaitReportedAction); der Mock bildet das nach. */}
        <button
          type="button"
          onClick={() => void onLifecycleAction("start").catch(() => null)}
        >
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
        {/* Ein fehlgeschlagener Schritt wirft nach der Meldung weiter. */}
        <button
          type="button"
          onClick={() => void onLifecycleAction("reopen").catch(() => null)}
        >
          detail-reopen
        </button>
        <button
          type="button"
          onClick={() => void onDeleteCancelled(instance).catch(() => null)}
        >
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
            void onAttendancePatch(instance.id, "21", {
              status: "present",
            }).catch(() => null)
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
  calendarPeriodId: "5",
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
  periodsState = "ready" as "ready" | "loading" | "error",
  templates = [template],
  careOfferingLink = null,
  gaps = [] as GapInstance[],
  gapsState = "ready" as "ready" | "loading" | "error",
  phases = [] as Array<Record<string, unknown>>,
  phasesState = "ready" as "ready" | "loading" | "error",
  closingDaysLoading = false,
  conflictAcks = [] as string[],
  instances = [instance],
  filterGroups = [] as Array<{ id: string; name: string }>,
  filterRooms = [] as Array<{ id: string; name: string }>,
}: {
  periods?: Array<typeof period>;
  periodsState?: "ready" | "loading" | "error";
  templates?: TimetableTemplate[];
  /** null = the planner admin cannot read enrollment (status "unknown"). */
  careOfferingLink?: { total: number; linked: number } | null;
  gaps?: GapInstance[];
  gapsState?: "ready" | "loading" | "error";
  phases?: Array<Record<string, unknown>>;
  phasesState?: "ready" | "loading" | "error";
  closingDaysLoading?: boolean;
  conflictAcks?: string[];
  instances?: (typeof instance)[];
  filterGroups?: Array<{ id: string; name: string }>;
  filterRooms?: Array<{ id: string; name: string }>;
} = {}) {
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === null) return {};
    if (key === "timetable-care-offering-link") {
      return { data: careOfferingLink, isLoading: false };
    }
    if (key === "database-calendar-periods-list") {
      if (periodsState === "loading") return { isLoading: true };
      if (periodsState === "error") {
        return {
          error: new ApiError("Zeiträume kaputt", 503, {
            code: "general.unavailable",
          }),
          isLoading: false,
        };
      }
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
    if (key === "timetable-filter-rooms") {
      return { data: filterRooms };
    }
    if (key === "timetable-filter-groups") {
      return { data: filterGroups };
    }
    if (key.startsWith("timetable-gaps")) {
      if (gapsState === "loading") return { isLoading: true };
      if (gapsState === "error") {
        return {
          error: new ApiError("gaps kaputt", 500, { code: "general.server" }),
          isLoading: false,
        };
      }
      return { data: { gaps, acknowledged: [] }, isLoading: false };
    }
    if (key.startsWith("timetable-templates")) {
      return { data: { templates }, isLoading: false };
    }
    if (key.startsWith("timetable-")) {
      return {
        data: { from: "2026-05-04", to: "2026-05-10", instances },
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
  fireEvent.click(screen.getByRole("button", { name }));
}

let realReplaceState: typeof window.history.replaceState;

describe("BetreuungsplanView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setTestClock(new Date("2026-05-06T12:00:00"));
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
    mockShowActionError.mockImplementation(showActionErrorAsToast);
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
    vi.useRealTimers();
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
      data: { user: { permissions: ["schedules:read", "schedules:manage"] } },
    });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    // Anker: das Menü ist offen und zeigt seine übrigen Einträge.
    expect(screen.getByText("Termine im Zeitraum absagen")).toBeInTheDocument();
    expect(
      screen.queryByText("Drucken oder exportieren"),
    ).not.toBeInTheDocument();
  });

  it("offers export with the permission to read staff names", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: {
        user: {
          permissions: ["schedules:read", "schedules:manage", "users:read"],
        },
      },
    });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    expect(screen.getByText("Drucken oder exportieren")).toBeInTheDocument();
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

  // Raster und Detailpanel brauchen nur Namen: die Namens-Projektion statt
  // der vollen Kinderliste (#4996), und kein Settings-Schema.
  it("loads the student roster as names only", async () => {
    setupSWR();
    render(<BetreuungsplanView />);

    const call = mockUseSWRAuth.mock.calls.find(
      ([key]) => key === "timetable-student-list",
    );
    expect(call).toBeDefined();
    await (call![1] as () => Promise<unknown>)();
    expect(fetchStudents).toHaveBeenCalledWith({
      page_size: 500,
      view: "names",
    });
    expect(mockUseSWRAuth).not.toHaveBeenCalledWith(
      "settings-schema",
      expect.anything(),
      expect.anything(),
    );
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

  it("forces the week view and offers only Tag/Woche for read-only staff", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    setUrl("view=monat");
    render(<BetreuungsplanView />);

    // ?view=monat fällt still auf die Woche zurück; Monat und Serien sind
    // Planungsansichten und stehen der Leseansicht nicht offen (#2621: Tag
    // und Woche schon).
    expect(screen.getByText("week-grid")).toBeVisible();
    expect(screen.queryByText("Monat")).not.toBeInTheDocument();
    expect(screen.queryByText("Serien")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Tag" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Woche" })).toBeVisible();
  });

  it("öffnet die Tagesansicht für Teammitglieder ohne Planungsrecht", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    setUrl("view=tag&d=2026-05-06");
    render(<BetreuungsplanView />);

    // Ein Deeplink auf den Tag überlebt das Neuladen …
    expect(screen.getByTestId("grid-week-days")).toHaveTextContent("1");
    expect(screen.getByText("Mittwoch, 06.05.2026")).toBeVisible();
    // … und bleibt vollständig lesend: kein Anlegen aus dem Raster.
    expect(
      screen.queryByRole("button", { name: "slot-click" }),
    ).not.toBeInTheDocument();
  });

  it("hides print and skips gaps/conflict-acks fetches for read-only staff", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      // users:read allein schaltet den Druck-Export nicht mehr frei (#2283
      // Feedback: die Leseansicht zeigt nur die Woche, sonst nichts).
      data: { user: { permissions: ["schedules:read", "users:read"] } },
    });
    render(<BetreuungsplanView />);

    // Anker: die geladene Leseansicht mit ihrem Hinweis steht da.
    expect(screen.getByText("Nur ansehen")).toBeInTheDocument();
    expect(screen.getByText("week-grid")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
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

  // --- Tagesansicht (#2621) ---

  it("zeigt in der Tagesansicht genau einen Tag mit ausgeschriebenem Datum", () => {
    setUrl("view=tag&d=2026-05-06");
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-week-days")).toHaveTextContent("1");
    expect(screen.getByText("Mittwoch, 06.05.2026")).toBeVisible();
    expect(screen.getByTestId("conflicts-period")).toHaveTextContent(
      "an diesem Tag",
    );
  });

  it("formuliert die leeren Tageszustände für Planende in der Sie-Form", () => {
    setUrl("view=tag&d=2026-05-06");
    setupSWR({ instances: [] });
    const { unmount } = render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-empty-state")).toHaveTextContent(
      "Planen Sie Angebote als Regeltermin oder legen Sie einen einzelnen Termin an.",
    );
    unmount();

    setupSWR({
      instances: [],
      periods: [{ ...period, endDate: "2026-05-05" }],
    });
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-empty-state")).toHaveTextContent(
      "Legen Sie zuerst einen aktiven Planungszeitraum an.",
    );
  });

  it("markiert einen Tag außerhalb des Planungszeitraums als nicht planbar", () => {
    setUrl("view=tag&d=2026-05-06");
    setupSWR({
      instances: [],
      periods: [{ ...period, endDate: "2026-05-05" }],
    });

    render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-disabled-dates")).toHaveTextContent(
      "2026-05-06",
    );
  });

  it("markiert Tage außerhalb des Planungszeitraums auch im Monat", () => {
    setUrl("view=monat&d=2026-05-06");
    setupSWR({
      instances: [],
      periods: [{ ...period, endDate: "2026-05-05" }],
    });

    render(<BetreuungsplanView />);

    expect(screen.getByTestId("month-disabled-dates")).toHaveTextContent(
      "2026-05-06",
    );
  });

  it("wählt für den globalen Einmaltermin einen sichtbaren planbaren Tag", () => {
    setUrl("view=woche&d=2026-05-06");
    setupSWR({ periods: [{ ...period, startDate: "2026-05-07" }] });

    render(<BetreuungsplanView />);

    expect(screen.getByText("add-instance")).toBeEnabled();
    fireEvent.click(screen.getByText("add-instance"));
    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({ defaultDate: "2026-05-07" }),
    );
  });

  it("wählt im Monat keinen planbaren Tag aus dem Vormonat", () => {
    setUrl("view=monat&d=2026-05-06");
    setupSWR({
      periods: [{ ...period, startDate: "2026-04-27", endDate: "2026-05-04" }],
    });

    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("add-instance"));
    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({ defaultDate: "2026-05-01" }),
    );
  });

  it("behält einen kurzen Zeitraum bis Freitag als Standarddatum bei", () => {
    setUrl("view=woche&d=2026-05-04");
    setupSWR({
      periods: [{ ...period, startDate: "2026-05-08", endDate: "2026-05-08" }],
    });

    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("add-instance"));
    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({ defaultDate: "2026-05-08" }),
    );
  });

  it("sperrt das Menü Neu ohne sichtbaren Planungszeitraum", () => {
    setUrl("view=woche&d=2026-05-04");
    setupSWR({ periods: [laterPeriod] });
    render(<BetreuungsplanView />);

    expect(screen.getByText("add-instance")).toBeDisabled();
    expect(screen.getByText("add-series")).toBeDisabled();
  });

  it("behält einen nur am Wochenende liegenden Zeitraum als deaktivierten Fallback bei", () => {
    setUrl("view=monat&d=2026-05-09");
    setupSWR({
      periods: [{ ...period, startDate: "2026-05-09", endDate: "2026-05-09" }],
    });
    render(<BetreuungsplanView />);

    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({ defaultDate: "2026-05-09" }),
    );
    expect(screen.getByText("add-instance")).toBeDisabled();
    expect(screen.getByText("add-series")).toBeDisabled();
  });

  it("navigiert in der Tagesansicht von Schultag zu Schultag", () => {
    setUrl("view=tag&d=2026-05-08");
    const { unmount } = render(<BetreuungsplanView />);

    // Freitag -> Weiter überspringt das Wochenende.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(urlParams().get("d")).toBe("2026-05-11");
    unmount();

    setUrl("view=tag&d=2026-05-04");
    render(<BetreuungsplanView />);
    // Montag -> Zurück landet auf dem vorigen Freitag.
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    expect(urlParams().get("d")).toBe("2026-05-01");
  });

  describe("wenn das Wochenende dem Freitagsplan folgt (#3921)", () => {
    beforeEach(() => {
      vi.mocked(useWeekendFollowsFriday).mockReturnValue(true);
    });
    afterEach(() => {
      vi.mocked(useWeekendFollowsFriday).mockReturnValue(false);
    });

    it("navigiert an einem Werktag in der Tagesansicht über das Wochenende hinweg (#5028)", () => {
      // Testuhr: Mittwoch 2026-09-09, also kein heutiges Wochenende.
      setUrl("view=tag&d=2026-05-08");
      const { unmount } = render(<BetreuungsplanView />);
      fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
      expect(urlParams().get("d")).toBe("2026-05-11");
      unmount();

      setUrl("view=tag&d=2026-05-11");
      render(<BetreuungsplanView />);
      fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
      expect(urlParams().get("d")).toBe("2026-05-08");
    });

    it("zeigt an einem Werktag fünf Spalten (#5028)", () => {
      setUrl("d=2026-09-09");
      render(<BetreuungsplanView />);

      expect(screen.getByTestId("grid-week-days")).toHaveTextContent("5");
    });

    it("navigiert am Wochenende in der Tagesansicht auch über Samstag und Sonntag", () => {
      setTestClock(new Date("2026-05-09T12:00:00+02:00")); // Samstag
      setUrl("view=tag&d=2026-05-08");
      const { unmount } = render(<BetreuungsplanView />);
      fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
      expect(urlParams().get("d")).toBe("2026-05-09");
      unmount();

      setUrl("view=tag&d=2026-05-11");
      render(<BetreuungsplanView />);
      fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
      expect(urlParams().get("d")).toBe("2026-05-10");
    });

    it("bleibt am Wochenende auf dem heutigen Tag und zeigt sieben Spalten", () => {
      setTestClock(new Date("2026-05-09T12:00:00Z"));
      setUrl("d=2026-05-09");
      render(<BetreuungsplanView />);

      expect(screen.getByTestId("grid-week-days")).toHaveTextContent("7");
      expect(urlParams().get("d")).not.toBe("2026-05-11");
    });

    it("führt Heute am Samstag auf den Samstag statt auf Montag", () => {
      setTestClock(new Date("2026-05-09T12:00:00Z"));
      setUrl("view=tag&d=2026-05-04");
      render(<BetreuungsplanView />);

      fireEvent.click(screen.getAllByRole("button", { name: "Heute" })[0]!);
      expect(urlParams().get("d")).toBe("2026-05-09");
    });
  });

  it("zeigt ohne Wochenend-Betreuung nur Mo bis Fr", () => {
    render(<BetreuungsplanView />);
    expect(screen.getByTestId("grid-week-days")).toHaveTextContent("5");
  });

  it("führt Heute in der Tagesansicht am Wochenende auf den nächsten Schultag", () => {
    setTestClock(new Date("2026-05-09T12:00:00Z"));
    setUrl("view=tag&d=2026-05-04");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getAllByRole("button", { name: "Heute" })[0]!);
    expect(urlParams().get("d")).toBe("2026-05-11");
  });

  it("hält Tag und Woche als Umschalter-Werte in der URL", () => {
    render(<BetreuungsplanView />);

    selectTab("Tag");
    expect(urlParams().get("view")).toBe("tag");
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
    setTestClock(new Date("2026-05-09T12:00:00Z"));
    setUrl("d=2026-05-06");
    render(<BetreuungsplanView />);
    fireEvent.click(screen.getAllByRole("button", { name: "Heute" })[0]!);
    expect(urlParams().get("d")).toBe("2026-05-11");
  });

  it("does not query gaps for a finished Friday workweek on Saturday", () => {
    setTestClock(new Date("2026-05-09T12:00:00Z"));
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

  // Die Woche kommt ohne Kinderzeilen (#4996): das Detailpanel bekommt sie aus
  // dem vollständig nachgeladenen Tag des Termins.
  it("loads the children of the open block with its whole day", async () => {
    const { timetableService } = await import("~/lib/timetable-api");
    const base = mockUseSWRAuth.getMockImplementation()!;
    mockUseSWRAuth.mockImplementation((key: string | null) =>
      key?.includes("-detail-")
        ? {
            data: {
              from: instance.date,
              to: instance.date,
              instances: [
                {
                  ...instance,
                  students: [
                    {
                      studentId: "21",
                      status: "present",
                      careDayStatus: "scheduled",
                    },
                  ],
                },
              ],
            },
          }
        : base(key),
    );
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("detail-students")).toHaveTextContent("1");
    const detail = mockUseSWRAuth.mock.calls.find(
      ([key]) => typeof key === "string" && key.includes("-detail-"),
    ) as [string, () => Promise<unknown>] | undefined;
    expect(detail?.[0]).toMatch(
      new RegExp(`^timetable-week-.+-detail-${instance.date}$`),
    );
    void detail![1]();
    expect(timetableService.getWeek).toHaveBeenLastCalledWith(
      instance.date,
      instance.date,
      { groupId: null, staffId: null, roomId: null },
    );
  });

  it("refreshes the open block's notes even when the slim week is unchanged", async () => {
    const { useSWRAuth: useRealSWRAuth, SWRConfig } =
      await vi.importActual<typeof import("~/lib/swr")>("~/lib/swr");
    const { timetableService } = await import("~/lib/timetable-api");
    const base = mockUseSWRAuth.getMockImplementation()!;
    const revalidators = new Map<string, () => Promise<unknown>>();
    const week = {
      from: instance.date,
      to: instance.date,
      instances: [instance],
    };
    let note = "Alte Notiz";
    vi.mocked(timetableService.getWeek).mockImplementation(
      async (_from, _to, _filter, options) =>
        options?.view === "slim"
          ? week
          : {
              ...week,
              instances: [
                {
                  ...instance,
                  students: [{ studentId: "21", status: "absent", note }],
                },
              ],
            },
    );
    mockUseSWRAuth.mockImplementation(
      function usePlanSWR(key, fetcher, options) {
        const isPlan = key?.startsWith("timetable-week-");
        const response = useRealSWRAuth(isPlan ? key : null, fetcher, options);
        if (isPlan) revalidators.set(key, () => response.mutate());
        return isPlan ? response : base(key);
      },
    );
    await mockTenantMutate.withImplementation(
      (key: string) => revalidators.get(key)?.(),
      async () => {
        setUrl("view=woche&block=42");
        render(
          <SWRConfig value={{ provider: () => new Map() }}>
            <BetreuungsplanView />
          </SWRConfig>,
        );
        await waitFor(() =>
          expect(screen.getByTestId("detail-student-note")).toHaveTextContent(
            "Alte Notiz",
          ),
        );

        // Nur die Notiz ändert sich. Die schlanke Wochenantwort behält exakt
        // dieselbe Referenz, und es gibt keine zusätzliche SSE-Benachrichtigung.
        note = "Neue Notiz";
        fireEvent.click(screen.getByText("detail-attendance"));
        await waitFor(() =>
          expect(mockPatchAttendance).toHaveBeenCalledWith("42", "21", {
            status: "present",
          }),
        );
        await waitFor(() =>
          expect(screen.getByTestId("detail-student-note")).toHaveTextContent(
            "Neue Notiz",
          ),
        );
      },
    );
  });

  it("shows the children as loading until the block's day arrives", () => {
    const base = mockUseSWRAuth.getMockImplementation()!;
    mockUseSWRAuth.mockImplementation((key: string | null) =>
      key?.includes("-detail-") ? { isLoading: true } : base(key),
    );
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("detail-students")).toHaveTextContent("pending");
  });

  it("does not reuse the previous day's children while the selected day loads", async () => {
    const { useSWRAuth: useRealSWRAuth, SWRConfig } =
      await vi.importActual<typeof import("~/lib/swr")>("~/lib/swr");
    const { timetableService } = await import("~/lib/timetable-api");
    const nextInstance = { ...instance, id: "43", date: "2026-05-05" };
    setupSWR({ instances: [instance, nextInstance] });
    const base = mockUseSWRAuth.getMockImplementation()!;
    mockUseSWRAuth.mockImplementation(
      function useDetailSWR(key, fetcher, options) {
        const isDetail = key?.includes("-detail-");
        const response = useRealSWRAuth(
          isDetail ? key : null,
          fetcher,
          options,
        );
        return isDetail ? response : base(key);
      },
    );
    let finishNextDay!: (
      value: Awaited<ReturnType<typeof timetableService.getWeek>>,
    ) => void;
    vi.mocked(timetableService.getWeek)
      .mockResolvedValueOnce({
        from: instance.date,
        to: instance.date,
        instances: [
          {
            ...instance,
            students: [
              {
                studentId: "21",
                status: "present",
                careDayStatus: "scheduled",
              },
            ],
          },
        ],
      })
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            finishNextDay = resolve;
          }),
      );
    setUrl("view=woche&block=42");
    render(
      <SWRConfig value={{ provider: () => new Map() }}>
        <BetreuungsplanView />
      </SWRConfig>,
    );
    await waitFor(() =>
      expect(screen.getByTestId("detail-students")).toHaveTextContent("1"),
    );

    act(() => setUrl("view=woche&block=43"));
    expect(screen.getByTestId("detail-students")).toHaveTextContent("pending");
    await act(async () =>
      finishNextDay({
        from: nextInstance.date,
        to: nextInstance.date,
        instances: [nextInstance],
      }),
    );
    expect(screen.getByTestId("detail-students")).toHaveTextContent("0");
  });

  it("names the children's list in the load error of an open block", async () => {
    const base = mockUseSWRAuth.getMockImplementation()!;
    mockUseSWRAuth.mockImplementation((key: string | null) =>
      key?.includes("-detail-")
        ? {
            error: new ApiError("down", 503, { code: "general.unavailable" }),
            isLoading: false,
          }
        : base(key),
    );
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    await waitFor(() =>
      expect(screen.getByTestId("detail-students-error")).toHaveTextContent(
        catalogText("general.unavailable", "die Liste der Kinder"),
      ),
    );
  });

  // Regeltermine braucht nur die Serienansicht und das Detailpanel (#4996).
  it("loads the Regeltermine only for the series view or an open block", () => {
    setUrl("view=woche");
    const { unmount } = render(<BetreuungsplanView />);
    const templateKeys = () =>
      mockUseSWRAuth.mock.calls
        .map(([key]) => key)
        .filter(
          (key): key is string =>
            typeof key === "string" && key.startsWith("timetable-templates-"),
        );
    expect(mockUseSWRAuth).toHaveBeenCalledWith(
      expect.stringMatching(/^timetable-week-/),
      expect.any(Function),
      { onSuccess: expect.any(Function) },
    );
    expect(templateKeys()).toEqual([]);
    unmount();

    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);
    expect(templateKeys().length).toBeGreaterThan(0);
  });

  // #2957: Ohne einen weiteren Plan-Parameter wäre `block` beim Schließen der
  // letzte gewesen — die Kalenderfläche hätte danach auf "Meine Termine"
  // umgeschaltet.
  it("keeps the visible day in the URL when the slide-over closes (#2957)", async () => {
    setUrl("block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-close"));

    await waitFor(() =>
      expect(screen.queryByText("detail-close")).not.toBeInTheDocument(),
    );
    expect(urlParams().has("block")).toBe(false);
    // Der sichtbare Tag (Systemzeit der Suite) bleibt als Plan-Parameter stehen.
    expect(urlParams().get("d")).toBe("2026-05-06");
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

  it("hides the gap chip and says so in place when the gaps request fails", async () => {
    setupSWR({ gapsState: "error" });
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(screen.queryByText("Keine Lücken")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Lücken werden geprüft …"),
    ).not.toBeInTheDocument();
    expect(
      await screen.findByText(
        catalogText("general.server", "die Prüfung der Besetzung"),
      ),
    ).toBeVisible();
    expect(screen.queryByText(/gaps kaputt/)).not.toBeInTheDocument();
    // Ein Ladefehler ist kein Toast (#2516).
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("shows a network failure of the week in place, never the browser text (#1577)", async () => {
    // Was der Client bei einem Netzfehler wirft: fetch scheitert mit
    // TypeError, transportFetch macht daraus general.unavailable.
    vi.stubGlobal(
      "fetch",
      vi.fn().mockRejectedValue(new TypeError("Failed to fetch")),
    );
    const networkError = await transportFetch("/api/timetable/instances").catch(
      (err: unknown) => err,
    );
    vi.unstubAllGlobals();
    const base = mockUseSWRAuth.getMockImplementation()!;
    mockUseSWRAuth.mockImplementation((key: string | null) =>
      key?.startsWith("timetable-week-")
        ? { error: networkError, isLoading: false }
        : base(key),
    );
    setUrl("view=woche");
    render(<BetreuungsplanView />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Termine"),
      ),
    ).toBeVisible();
    expect(screen.queryByText(/Failed to fetch/)).not.toBeInTheDocument();
    // Kein leerer Plan, der aussieht, als sei nichts geplant, und keine
    // Zahl in der Statuszeile, die nie geladen wurde.
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();
    expect(screen.queryByText("0 Termine")).not.toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mockTenantMutate).toHaveBeenCalledWith(
      expect.stringMatching(/^timetable-week-/),
    );
  });

  it("reports a failed conflict hide as a toast with the catalog text", async () => {
    mockAcknowledgeConflict.mockRejectedValueOnce(
      new ApiError("forbidden", 403, { code: "general.permission" }),
    );
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("conflict-hide"));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        catalogText("general.permission", "das Ausblenden des Konflikts"),
        undefined,
      ),
    );
    expect(mockAcknowledgeConflict).toHaveBeenCalledWith(
      "aaaa1111bbbb2222cccc3333dddd4444",
    );
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

    // Anker: die Kontextzeile mit dem Zeitraum-Umschalter steht da.
    expect(screen.getByTestId("period-switcher")).toBeInTheDocument();
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

  it("opens the create period flow from the period chip; editing lives on the route", async () => {
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("create-period"));
    expect(screen.getByText("period-save")).toBeInTheDocument();
    // Anlegen kennt keinen Bestand: keine Nutzungszahlen, ein
    // Löschen gibt es hier nicht (Bearbeiten und Löschen liegen auf
    // /calendar-periods, BAUARTEN-SPEC Bauart 1 Regel 9).
    expect(screen.getByTestId("period-modal")).not.toHaveAttribute(
      "data-usage",
    );
    expect(screen.queryByText("edit-period")).not.toBeInTheDocument();
    fireEvent.click(screen.getByText("period-close"));
    expect(screen.queryByText("period-save")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("create-period"));
    fireEvent.click(screen.getByText("period-save"));
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

  it("names the full room from the details when reopening is refused (#3633)", async () => {
    mockReopen.mockRejectedValue(
      new ApiError("room capacity exceeded: Turnhalle (30/30)", 409, {
        code: "presence.room_capacity_exceeded",
        details: {
          room_name: "Turnhalle",
          current_occupancy: 30,
          max_capacity: 30,
          incoming_students: 1,
        },
      }),
    );
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-reopen"));

    // Name und Belegung kommen aus den Details, nie aus dem Satz des Servers.
    const expected = Object.entries({
      room_name: "Turnhalle",
      current_occupancy: 30,
      max_capacity: 30,
      incoming_students: 1,
      free_slots: 0,
    }).reduce(
      (text, [key, value]) => text.replace(`{${key}}`, String(value)),
      catalogText("presence.room_capacity_exceeded", ""),
    );
    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(expected, undefined),
    );
    expect(mockToastError).not.toHaveBeenCalledWith(
      expect.stringContaining("room capacity exceeded"),
      expect.anything(),
    );
  });

  it("reports a failed start with the catalog text and retries the same block", async () => {
    mockStart
      .mockRejectedValueOnce(
        new ApiError("start window expired", 409, {
          code: "timetable.start_window_expired",
        }),
      )
      .mockRejectedValueOnce(
        new ApiError("upstream down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({ warnings: [] });
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-start"));
    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        catalogText("timetable.start_window_expired", "die Aktivität"),
        undefined,
      ),
    );

    fireEvent.click(screen.getByText("detail-start"));
    await waitFor(() => expect(mockToastError).toHaveBeenCalledTimes(2));
    const [message, options] = mockToastError.mock.calls[1] as [
      string,
      { action: { label: string; onClick: () => void } },
    ];
    expect(message).toBe(catalogText("general.unavailable", "die Aktivität"));
    expect(options.action.label).toBe("Wiederholen");

    options.action.onClick();
    await waitFor(() =>
      expect(mockToastSuccess).toHaveBeenCalledWith(
        "Die Aktivität ist gestartet.",
      ),
    );
    expect(mockStart).toHaveBeenCalledTimes(3);
    expect(mockStart).toHaveBeenLastCalledWith("42");
  });

  it("reports a failed attendance change as a toast with the catalog text", async () => {
    mockPatchAttendance.mockRejectedValueOnce(
      new ApiError("attendance disabled", 409, {
        code: "attendance.web_disabled",
      }),
    );
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-attendance"));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        catalogText("attendance.web_disabled", "die Anwesenheit"),
        undefined,
      ),
    );
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("leaves a failed delete to the dialog of the detail panel", async () => {
    mockDeleteCancelled.mockRejectedValueOnce(
      new ApiError("server down", 500, { code: "general.server" }),
    );
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-delete"));

    await waitFor(() => expect(mockDeleteCancelled).toHaveBeenCalledWith("42"));
    await waitFor(() =>
      expect(mockLoggerError).toHaveBeenCalledWith(
        "instance_delete_failed",
        expect.objectContaining({ instance_id: "42" }),
      ),
    );
    expect(mockToastError).not.toHaveBeenCalled();
    expect(mockShowActionError).not.toHaveBeenCalled();
  });

  it("ends following instances from the slide-over", async () => {
    // Der Regeltermin lässt sich nur ab heute beenden, also muss der Termin
    // der Fixture (2026-05-04) hier "heute" sein.
    setTestClock(new Date("2026-05-04T08:00:00"));
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
    // die View den Aufruf ab. Den Grund nennt der Löschdialog, der über den
    // Toasts liegt; hier wird nur protokolliert und abgelehnt.
    setUrl("view=woche&block=42");
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("detail-delete-following"));
    await waitFor(() =>
      expect(mockLoggerWarn).toHaveBeenCalledWith(
        "template_end_in_past_refused",
        expect.objectContaining({ effective_date: "2026-05-04" }),
      ),
    );
    expect(mockEndTemplate).not.toHaveBeenCalled();
    expect(mockToastError).not.toHaveBeenCalled();
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
      "Der Regeltermin ist ab 06.05.2026 gelöscht.",
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
      "Der Regeltermin „Yoga“ ist archiviert.",
    );
    expect(mockTenantMutate).toHaveBeenCalledWith("timetable-templates-5");
  });

  it("adds a one-off appointment from the + Neu menu", () => {
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("add-instance"));
    expect(screen.getByText("event-save")).toBeInTheDocument();
  });

  it("keeps archive confirmation open and shows the error inside it", async () => {
    setUrl("view=serien");
    mockArchiveTemplate
      .mockRejectedValueOnce(
        new ApiError("Archivierung kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce({});
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("archive-template"));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));

    const dialog = screen.getByRole("dialog", {
      name: "Regeltermin archivieren?",
    });
    // Der Dialog liegt über den Toasts: der Fehler steht im Dialog.
    expect(
      await within(dialog).findByText(
        catalogText("general.server", "das Archivieren des Regeltermins"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Archivierung kaputt")).not.toBeInTheDocument();
    expect(mockToastError).not.toHaveBeenCalled();

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Wiederholen" }),
    );
    await waitFor(() => expect(mockArchiveTemplate).toHaveBeenCalledTimes(2));
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", { name: "Regeltermin archivieren?" }),
      ).not.toBeInTheDocument(),
    );
  });

  it("reports a failed Regeltermin apply as a toast with retry", async () => {
    setUrl("view=serien");
    mockMaterialize.mockRejectedValueOnce(
      new ApiError("db timeout", 503, { code: "general.unavailable" }),
    );
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("apply-template"));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        catalogText("general.unavailable", "das Eintragen des Regeltermins"),
        expect.objectContaining({
          action: expect.objectContaining({ label: "Wiederholen" }),
        }),
      ),
    );
    const [, options] = mockToastError.mock.calls[0] as [
      string,
      { action: { onClick: () => void } },
    ];
    options.action.onClick();
    await waitFor(() =>
      // Der Mock legt je 56-Tage-Abschnitt des Zeitraums zwei Termine an.
      expect(mockToastSuccess).toHaveBeenCalledWith(
        expect.stringMatching(/^Für „Yoga“ sind \d+ Termine angelegt\.$/),
      ),
    );
  });

  it("shows loading state while the session loads", () => {
    mockUseSession.mockReturnValue({ status: "loading" });

    render(<BetreuungsplanView />);

    // Chrome (PlanningContextBar title/navigation) renders immediately —
    // only the calendar-grid content region skeletonizes (showSkeleton
    // pattern, mirrors staff/page.tsx and rooms/page.tsx).
    expect(screen.getByText("Betreuungsplan")).toBeVisible();
    // Der Ladezustand kommt aus dem TenantPage-Gerüst, nicht aus einem
    // eigenen Seiten-Skelett.
    expect(
      screen.getByRole("status", { name: "Betreuungsplan wird geladen…" }),
    ).toBeVisible();
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

    // Der Ladezustand kommt aus dem TenantPage-Gerüst, nicht aus einem
    // eigenen Seiten-Skelett.
    expect(
      screen.getByRole("status", { name: "Betreuungsplan wird geladen…" }),
    ).toBeVisible();
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

  it("uses an instance's pinned period for its Regeltermin indicator", () => {
    const overlappingPeriod = {
      ...period,
      id: "9",
      name: "Projektwoche",
      startDate: "2026-05-04",
      endDate: "2026-05-08",
    };
    setupSWR({
      periods: [period, overlappingPeriod],
      instances: [{ ...instance, activityGroupId: "7", calendarPeriodId: "9" }],
      templates: [
        {
          ...template,
          rosterMaintenance: {
            mode: "automatic",
            offeringNames: ["Mittagessen"],
            gradeLevels: [],
            schoolClasses: [],
            inactiveOfferingNames: [],
            invalidOfferingNames: [],
            dynamicTargets: false,
            careOfferingsDisabled: false,
          },
        },
      ],
    });
    render(<BetreuungsplanView />);

    fireEvent.click(screen.getByText("week-grid"));

    expect(screen.getByTestId("detail-series-period")).toHaveTextContent("9");
    expect(screen.getByTestId("detail-series-maintenance")).toHaveTextContent(
      "missing",
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

    // Anker: die geladene Fläche zeigt den Hinweis ohne Planungszeitraum.
    expect(screen.getByText("Noch kein Planungszeitraum")).toBeInTheDocument();
    expect(screen.queryByTestId("period-switcher")).not.toBeInTheDocument();
  });

  it("renders the disabled state when the timetable setting is off", () => {
    setupSWR();
    vi.mocked(useTimetableEnabled).mockReturnValue(false);
    try {
      render(<BetreuungsplanView />);
    } finally {
      vi.mocked(useTimetableEnabled).mockReturnValue(true);
    }

    expect(screen.getByTestId("timetable-disabled-state")).toBeVisible();
    expect(screen.getByText("Betreuungsplan ist deaktiviert")).toBeVisible();
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();
    expect(mockBootstrap).not.toHaveBeenCalled();
  });

  it("shows the skeleton while the session loads", () => {
    setupSWR();
    mockUseSession.mockReturnValue({ status: "loading", data: null });
    render(<BetreuungsplanView />);

    // Chrome renders immediately; only the content region skeletonizes
    // while the session (and therefore the caller's permissions) is still
    // unresolved.
    expect(screen.getByText("Betreuungsplan")).toBeVisible();
    // Der Ladezustand kommt aus dem TenantPage-Gerüst, nicht aus einem
    // eigenen Seiten-Skelett.
    expect(
      screen.getByRole("status", { name: "Betreuungsplan wird geladen…" }),
    ).toBeVisible();
    expect(
      screen.queryByTestId("timetable-page-skeleton"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();
  });

  it("logs a warning when the bootstrap fails with 403", async () => {
    setupSWR({ periods: [] });
    mockBootstrap.mockRejectedValue(new ApiError("permission denied", 403));

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

  it.each([
    {
      name: "vollständig abgedeckt",
      periods: [period],
      title: "Diese Woche hat noch keine Termine",
      description:
        "Planen Sie Angebote als Regeltermin oder legen Sie einen einzelnen Termin an.",
      disabledDates: "",
    },
    {
      name: "nicht abgedeckt",
      periods: [laterPeriod],
      title: "Diese Woche hat keinen Planungszeitraum",
      description: "Legen Sie zuerst einen aktiven Planungszeitraum an.",
      disabledDates: "2026-05-04,2026-05-05,2026-05-06,2026-05-07,2026-05-08",
    },
    {
      name: "ab Mittwoch abgedeckt",
      periods: [{ ...period, startDate: "2026-05-06" }],
      title: "Diese Woche hat noch keine Termine",
      description:
        "Einige Tage sind nicht planbar. An den anderen Tagen können Sie Termine planen.",
      disabledDates: "2026-05-04,2026-05-05",
    },
    {
      name: "bis Mittwoch abgedeckt",
      periods: [{ ...period, endDate: "2026-05-06" }],
      title: "Diese Woche hat noch keine Termine",
      description:
        "Einige Tage sind nicht planbar. An den anderen Tagen können Sie Termine planen.",
      disabledDates: "2026-05-07,2026-05-08",
    },
  ])(
    "unterscheidet eine $name Woche im leeren Wochenzustand",
    ({ periods, title, description, disabledDates }) => {
      setUrl("view=woche&d=2026-05-04");
      setupSWR({ periods, instances: [] });

      render(<BetreuungsplanView />);

      expect(screen.getByTestId("grid-empty-state")).toHaveTextContent(title);
      expect(screen.getByTestId("grid-empty-state")).toHaveTextContent(
        description,
      );
      expect(screen.getByTestId("grid-disabled-dates")).toHaveTextContent(
        disabledDates,
      );
    },
  );

  it("erklärt eine teilweise abgedeckte Woche auch in der Leseansicht", () => {
    mockUseSession.mockReturnValue({
      status: "authenticated",
      data: { user: { permissions: ["schedules:read"] } },
    });
    setUrl("view=woche&d=2026-05-04");
    setupSWR({
      periods: [{ ...period, startDate: "2026-05-06" }],
      instances: [],
    });

    render(<BetreuungsplanView />);

    expect(screen.getByTestId("grid-empty-state")).toHaveTextContent(
      "Für diese Woche ist noch nichts geplant. Geplant wird von den Admins Ihrer Schule.",
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

  it("zeigt beim Laden der Planungszeiträume keinen unplanbaren Kalender", () => {
    setupSWR({ periodsState: "loading" });
    render(<BetreuungsplanView />);

    expect(screen.getByTestId("timetable-content-skeleton")).toBeVisible();
    expect(screen.queryByText("week-grid")).not.toBeInTheDocument();
    expect(
      screen.queryByText("Noch kein Planungszeitraum"),
    ).not.toBeInTheDocument();
  });

  it("zeigt bei einem Fehler der Planungszeiträume weiterhin geladene Termine", async () => {
    setupSWR({ periodsState: "error" });
    render(<BetreuungsplanView />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Planungszeiträume"),
      ),
    ).toBeVisible();
    expect(screen.queryByText("Zeiträume kaputt")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mockTenantMutate).toHaveBeenCalledWith(
      "database-calendar-periods-list",
    );
    expect(screen.getByText("week-grid")).toBeVisible();
    expect(screen.getByText("add-instance")).toBeDisabled();
    expect(
      screen.queryByText("Noch kein Planungszeitraum"),
    ).not.toBeInTheDocument();
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

  it("sperrt das Menü Neu ohne Planungszeitraum", () => {
    setupSWR({ periods: [] });
    render(<BetreuungsplanView />);

    expect(screen.getByText("add-instance")).toBeDisabled();
    expect(screen.getByText("add-series")).toBeDisabled();
  });

  it("sperrt das Menü Neu bei ausschließlich inaktiven Planungszeiträumen", () => {
    setupSWR({ periods: [{ ...period, isActive: false }] });
    render(<BetreuungsplanView />);

    expect(screen.getByText("add-instance")).toBeDisabled();
    expect(screen.getByText("add-series")).toBeDisabled();
  });

  // --- Gruppen- und Personenfilter (#4766) ---

  describe("Filter Gruppe und Person (#4766)", () => {
    function grantGroups() {
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
              "groups:read",
            ],
          },
        },
      });
    }

    /** SWR-Schlüssel und Fetcher des Instanz-Abrufs der Ansicht. */
    function instanceFetch(view: string) {
      const call = [...mockUseSWRAuth.mock.calls].reverse().find(
        ([key]) =>
          typeof key === "string" &&
          key.startsWith(`timetable-${view}-`) &&
          // Der nachgeladene Tag eines offenen Blocks teilt das Präfix.
          !key.includes("-detail-"),
      ) as [string, () => Promise<unknown>] | undefined;
      expect(call).toBeDefined();
      return { key: call![0], fetcher: call![1] };
    }

    it("lädt Tag, Woche und Monat mit dem Filter aus der URL", async () => {
      grantGroups();
      const { timetableService } = await import("~/lib/timetable-api");

      for (const [viewParam, view] of [
        ["view=tag&", "day"],
        ["", "week"],
        ["view=monat&", "month"],
      ] as const) {
        setUrl(`${viewParam}group=5&staff=11`);
        const { unmount } = render(<BetreuungsplanView />);
        const { key, fetcher } = instanceFetch(view);
        expect(key).toMatch(/-g5-p11$/);
        void fetcher();
        expect(timetableService.getWeek).toHaveBeenLastCalledWith(
          expect.any(String),
          expect.any(String),
          { groupId: "5", staffId: "11", roomId: null },
          { view: "slim" },
        );
        unmount();
      }
    });

    it("lädt ohne Filter den ganzen Plan unter dem bisherigen Schlüssel", async () => {
      const { timetableService } = await import("~/lib/timetable-api");
      setUrl("staff=abc");
      render(<BetreuungsplanView />);

      const { key, fetcher } = instanceFetch("week");
      expect(key).toBe("timetable-week-2026-05-04-2026-05-08");
      void fetcher();
      expect(timetableService.getWeek).toHaveBeenLastCalledWith(
        "2026-05-04",
        "2026-05-08",
        { groupId: null, staffId: null, roomId: null },
        { view: "slim" },
      );
    });

    it("bietet die Gruppe nur mit Gruppenliste an und ignoriert sonst den Parameter", () => {
      setUrl("group=5&staff=11");
      render(<BetreuungsplanView />);

      expect(screen.getByTestId("plan-filter-staff")).toBeInTheDocument();
      expect(screen.queryByTestId("plan-filter-group")).not.toBeInTheDocument();
      expect(instanceFetch("week").key).toMatch(/-g-p11$/);
    });

    it("schreibt die gewählte Person und Gruppe in die URL und schließt den Block", () => {
      grantGroups();
      setupSWR({ filterGroups: [{ id: "5", name: "Sonnen" }] });
      setUrl("block=42");
      render(<BetreuungsplanView />);

      fireEvent.click(
        screen.getByRole("combobox", { name: "Nach Person filtern" }),
      );
      fireEvent.click(screen.getByRole("option", { name: "Ada Absent" }));
      expect(urlParams().get("staff")).toBe("11");
      expect(urlParams().get("block")).toBeNull();
      expect(urlParams().get("d")).toBe("2026-05-06");

      fireEvent.click(
        screen.getByRole("combobox", { name: "Nach Gruppe filtern" }),
      );
      fireEvent.click(screen.getByRole("option", { name: "Sonnen" }));
      expect(urlParams().get("group")).toBe("5");
      expect(urlParams().get("staff")).toBe("11");

      fireEvent.click(
        screen.getByRole("combobox", { name: "Nach Person filtern" }),
      );
      fireEvent.click(screen.getByRole("option", { name: "Alle Personen" }));
      expect(urlParams().get("staff")).toBeNull();
      expect(urlParams().get("group")).toBe("5");
    });

    it("bietet dieselben Filter in Tag und Woche der Leseansicht", () => {
      mockUseSession.mockReturnValue({
        status: "authenticated",
        data: { user: { permissions: ["schedules:read", "groups:read"] } },
      });
      setupSWR({ filterGroups: [{ id: "5", name: "Sonnen" }] });

      for (const [viewParam, view] of [
        ["view=tag&", "day"],
        ["", "week"],
      ] as const) {
        setUrl(`${viewParam}group=5&staff=11`);
        const { unmount } = render(<BetreuungsplanView />);
        expect(screen.getByText("Nur ansehen")).toBeInTheDocument();
        expect(
          screen.getByRole("combobox", { name: "Nach Gruppe filtern" }),
        ).toHaveTextContent("Sonnen");
        expect(
          screen.getByRole("combobox", { name: "Nach Person filtern" }),
        ).toBeInTheDocument();
        expect(instanceFetch(view).key).toMatch(/-g5-p11$/);
        unmount();
      }
    });

    it("bietet im Monat auch ohne users:read Personen aus der Übersicht an", () => {
      mockUseSession.mockReturnValue({
        status: "authenticated",
        data: { user: { permissions: ["schedules:read", "schedules:manage"] } },
      });
      const defaultSWR = mockUseSWRAuth.getMockImplementation()!;
      mockUseSWRAuth.mockImplementation((key: string | null) => {
        if (
          typeof key === "string" &&
          key.startsWith("timetable-plan-staff-names-")
        ) {
          return {
            data: {
              appointments: [],
              staff: [{ id: "11", name: "Ada Absent", isExternal: false }],
            },
          };
        }
        return defaultSWR(key);
      });
      setUrl("view=monat&d=2026-05-06");

      render(<BetreuungsplanView />);

      fireEvent.click(
        screen.getByRole("combobox", { name: "Nach Person filtern" }),
      );
      fireEvent.click(screen.getByRole("option", { name: "Ada Absent" }));
      expect(urlParams().get("staff")).toBe("11");
      expect(
        mockUseSWRAuth.mock.calls.some(
          ([key]) =>
            typeof key === "string" &&
            key.startsWith("timetable-plan-staff-names-"),
        ),
      ).toBe(true);
    });

    it("filtert nach Raum aus der URL und schreibt die Auswahl zurück (#4768)", async () => {
      mockUseSession.mockReturnValue({
        status: "authenticated",
        data: {
          user: {
            permissions: [
              "schedules:read",
              "schedules:manage",
              "users:read",
              "rooms:read",
            ],
          },
        },
      });
      setupSWR({ filterRooms: [{ id: "3", name: "Werkraum" }] });
      const { timetableService } = await import("~/lib/timetable-api");
      setUrl("room=3&block=42");
      render(<BetreuungsplanView />);

      const { key, fetcher } = instanceFetch("week");
      expect(key).toMatch(/-g-p-r3$/);
      void fetcher();
      expect(timetableService.getWeek).toHaveBeenLastCalledWith(
        expect.any(String),
        expect.any(String),
        { groupId: null, staffId: null, roomId: "3" },
        { view: "slim" },
      );
      expect(
        screen.getByRole("combobox", { name: "Nach Raum filtern" }),
      ).toHaveTextContent("Werkraum");

      fireEvent.click(
        screen.getByRole("combobox", { name: "Nach Raum filtern" }),
      );
      fireEvent.click(screen.getByRole("option", { name: "Alle Räume" }));
      expect(urlParams().get("room")).toBeNull();
      expect(urlParams().get("block")).toBeNull();
    });

    it("ignoriert den Raum ohne Raumliste (#4768)", () => {
      setUrl("room=3&staff=11");
      render(<BetreuungsplanView />);

      expect(screen.getByTestId("plan-filter-staff")).toBeInTheDocument();
      expect(screen.queryByTestId("plan-filter-room")).not.toBeInTheDocument();
      expect(instanceFetch("week").key).toMatch(/-g-p11$/);
    });

    it("nennt einen leeren gefilterten Plan als leere Auswahl", () => {
      setupSWR({ instances: [] });
      setUrl("staff=11");
      render(<BetreuungsplanView />);

      expect(screen.getByTestId("grid-empty-state")).toHaveTextContent(
        "Für diese Auswahl ist nichts geplant",
      );
    });

    it("zeigt mit Filter nur Lücken der sichtbaren Blöcke", () => {
      setupSWR({ gaps: [gap, { ...gap, instanceId: "99" }] });
      const { unmount } = render(<BetreuungsplanView />);
      expect(screen.getByTestId("grid-gap-ids")).toHaveTextContent("42,99");
      unmount();

      setUrl("staff=11");
      render(<BetreuungsplanView />);
      expect(screen.getByTestId("grid-gap-ids")).toHaveTextContent(/^42$/);
    });
  });
});
