import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockSearch,
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
  mockSubstitute,
  mockPatchAttendance,
  mockDeleteCancelled,
  mockEndTemplate,
  mockArchiveTemplate,
  mockBootstrap,
  mockEventModalProps,
  mockLoggerWarn,
  mockLoggerError,
  mockListPhases,
  mockListCareOfferings,
} = vi.hoisted(() => ({
  mockSearch: { value: "" },
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
  mockSubstitute: vi.fn(),
  mockPatchAttendance: vi.fn(),
  mockDeleteCancelled: vi.fn(),
  mockEndTemplate: vi.fn(),
  mockArchiveTemplate: vi.fn(),
  mockBootstrap: vi.fn(),
  mockEventModalProps: vi.fn(),
  mockLoggerWarn: vi.fn(),
  mockLoggerError: vi.fn(),
  mockListPhases: vi.fn(),
  mockListCareOfferings: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(mockSearch.value),
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
    getExceptionConflicts: vi.fn(),
    materialize: mockMaterialize,
    replanWeek: vi.fn(),
    start: mockStart,
    complete: mockComplete,
    cancel: mockCancel,
    substitute: mockSubstitute,
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

vi.mock("~/lib/settings-api", () => ({
  SETTINGS_SCHEMA_SWR_KEY: "settings-schema",
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

vi.mock("~/lib/care-offering-api", () => ({
  listCareOfferings: mockListCareOfferings,
}));

vi.mock("~/components/ui/loading", () => ({
  Loading: () => <div data-testid="loading">Loading</div>,
}));

vi.mock("~/components/timetable/timetable-toolbar", () => ({
  DENSITY_TO_HOUR_HEIGHT_PX: { compact: 60, normal: 90, comfortable: 120 },
  TimetableToolbar: ({
    view,
    rangeLabel,
    onViewChange,
    onPrev,
    onNext,
    onToday,
    onDensityChange,
    onManagePeriods,
    periodSwitcher,
    planWeekAction,
  }: {
    view: string;
    rangeLabel: string;
    onViewChange: (view: "week" | "month" | "year" | "series") => void;
    onPrev: () => void;
    onNext: () => void;
    onToday: () => void;
    onDensityChange: (density: "compact" | "normal" | "comfortable") => void;
    onManagePeriods: () => void;
    periodSwitcher: React.ReactNode;
    planWeekAction: React.ReactNode;
  }) => (
    <div data-testid="toolbar">
      <span>
        {view}:{rangeLabel}
      </span>
      <button type="button" onClick={onManagePeriods}>
        manage-periods
      </button>
      <button type="button" onClick={() => onViewChange("week")}>
        week-view
      </button>
      <button type="button" onClick={() => onViewChange("month")}>
        month-view
      </button>
      <button type="button" onClick={() => onViewChange("year")}>
        year-view
      </button>
      <button type="button" onClick={() => onViewChange("series")}>
        series-view
      </button>
      <button type="button" onClick={onPrev}>
        prev
      </button>
      <button type="button" onClick={onNext}>
        next
      </button>
      <button type="button" onClick={onToday}>
        today
      </button>
      <button type="button" onClick={() => onDensityChange("comfortable")}>
        density
      </button>
      {periodSwitcher}
      {planWeekAction}
    </div>
  ),
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
  ConflictWarningsBanner: ({ conflictCount }: { conflictCount: number }) => (
    <div data-testid="conflicts">{conflictCount}</div>
  ),
}));

vi.mock("~/components/timetable/month-planner-grid", () => ({
  MonthPlannerGrid: ({
    onDayClick,
  }: {
    onDayClick: (date: string) => void;
  }) => (
    <button type="button" onClick={() => onDayClick("2026-05-06")}>
      month-grid
    </button>
  ),
}));

vi.mock("~/components/timetable/year-planner-grid", () => ({
  YearPlannerGrid: ({
    onMonthClick,
    onDayClick,
  }: {
    onMonthClick: (month: Date) => void;
    onDayClick: (date: string) => void;
  }) => (
    <div>
      <button
        type="button"
        onClick={() => onMonthClick(new Date("2026-05-01T00:00:00"))}
      >
        year-month
      </button>
      <button type="button" onClick={() => onDayClick("2026-05-06")}>
        year-day
      </button>
    </div>
  ),
}));

vi.mock("~/components/timetable/weekly-calendar-grid", () => ({
  WeeklyCalendarGrid: ({
    instances,
    onInstanceClick,
    onSlotClick,
  }: {
    instances: Array<{ id: string }>;
    onInstanceClick: (instance: { id: string } | null) => void;
    onSlotClick?: (dateISO: string, hour: number) => void;
  }) => (
    <div>
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

vi.mock("~/components/timetable/plan-quality-panel", () => ({
  PlanQualityPanel: ({
    onSelectInstance,
    onEditInstance,
    onSubstitute,
  }: {
    onSelectInstance: (id: string) => void;
    onEditInstance: (id: string) => void;
    onSubstitute: (
      absent: string,
      substitute: string,
      date: string,
    ) => Promise<void>;
  }) => (
    <div>
      <button type="button" onClick={() => onSelectInstance("42")}>
        quality-select
      </button>
      <button type="button" onClick={() => onEditInstance("42")}>
        quality-edit
      </button>
      <button
        type="button"
        onClick={() => void onSubstitute("11", "12", "2026-05-04")}
      >
        quality-substitute
      </button>
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
    templates: Array<{ id: string; name: string }>;
    onCreate: () => void;
    onEdit: (template: { id: string; name: string }) => void;
    onApply: (template: {
      id: string;
      name: string;
      schedules: Array<{ calendarPeriodId?: string }>;
    }) => void;
    onArchive: (template: { id: string; name: string }) => void;
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
          <button
            type="button"
            onClick={() =>
              onApply({
                ...templates[0]!,
                schedules: [{ calendarPeriodId: "5" }],
              })
            }
          >
            apply-template
          </button>
        </>
      )}
    </div>
  ),
}));

vi.mock("~/components/timetable/instance-detail-slide-over", () => ({
  InstanceDetailSlideOver: ({
    instance,
    onClose,
    onLifecycleAction,
    onDeleteCancelled,
    onDeleteFollowing,
    onEdit,
    onRepeat,
    onAttendancePatch,
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
  }) =>
    instance ? (
      <div>
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
        <button type="button" onClick={() => void onDeleteFollowing(instance)}>
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

import TimetablesPage, { loadCareOfferingLinkSummary } from "./page";

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
  presentStudentsCount: 0,
  requiredStaffCount: 3,
  assignedStaffCount: 1,
  conflictWarnings: [
    {
      kind: "room" as const,
      resourceId: "3",
      message: "Raum doppelt",
      canOverride: true,
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

const template = {
  id: "7",
  name: "Yoga",
  type: "activity" as const,
  categoryId: "2",
  categoryName: "AG",
  isOpen: true,
  maxParticipants: 12,
  enrollmentCount: 8,
  supervisorCount: 1,
  requiredStaffCount: 3,
  assignedStaffCount: 1,
  studentIds: ["21"],
  staffIds: ["11"],
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

function setupSWR({
  periods = [period],
  settingsSchema = null,
  settingsSchemaLoading = false,
  careOfferingLink = null,
}: {
  periods?: Array<typeof period>;
  settingsSchema?: unknown;
  settingsSchemaLoading?: boolean;
  /** null = the planner admin cannot read enrollment (status "unknown"). */
  careOfferingLink?: { total: number; linked: number } | null;
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
    if (key.startsWith("timetable-gaps")) {
      return { data: { gaps: [] }, isLoading: false };
    }
    if (key.startsWith("timetable-exception-conflicts")) {
      return { data: { conflicts: [] }, isLoading: false };
    }
    if (key.startsWith("timetable-templates")) {
      return { data: { templates: [template] }, isLoading: false };
    }
    if (key.startsWith("timetable-")) {
      return {
        data: { from: "2026-05-04", to: "2026-05-08", instances: [instance] },
        isLoading: false,
      };
    }
    return {};
  });
}

describe("TimetablesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.setSystemTime(new Date("2026-05-06T12:00:00"));
    mockSearch.value = "";
    mockUseSession.mockReturnValue({ status: "authenticated" });
    mockTenantMutate.mockResolvedValue(undefined);
    mockMaterialize.mockResolvedValue({
      instancesCreated: 2,
      warnings: [],
    });
    mockBootstrap.mockResolvedValue({ periods: [], created: false });
    mockStart.mockResolvedValue({ warnings: [] });
    mockComplete.mockResolvedValue({});
    mockCancel.mockResolvedValue({});
    mockSubstitute.mockResolvedValue({
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
    mockListCareOfferings.mockResolvedValue([]);
    setupSWR();
    window.history.replaceState(null, "", "/acme/timetables");
  });

  it("renders month view by default and opens create/edit period flows", async () => {
    render(<TimetablesPage />);

    expect(
      screen.getByRole("heading", { name: "Betreuungsplan" }),
    ).toBeVisible();
    expect(screen.getByText(/month:/)).toBeInTheDocument();
    expect(screen.getByTestId("conflicts")).toHaveTextContent("1");

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

  it("uses staffing ratios for the overview in every planner view", () => {
    render(<TimetablesPage />);

    const expectUnderstaffed = () => {
      const label = screen.getByText("Unterbesetzt");
      expect(label.parentElement).toHaveTextContent("1");
      expect(
        screen.getByText("zusätzliches Personal nötig"),
      ).toBeInTheDocument();
    };

    expectUnderstaffed();

    for (const viewButton of ["week-view", "year-view", "series-view"]) {
      fireEvent.click(screen.getByText(viewButton));
      expectUnderstaffed();
    }
  });

  it("navigates views and runs week actions", async () => {
    mockSearch.value = "view=week&instance=42";
    render(<TimetablesPage />);

    fireEvent.click(screen.getByText("week-view"));
    await waitFor(() => expect(screen.getByText("week-grid")).toBeVisible());
    fireEvent.click(screen.getByText("week-grid"));
    fireEvent.click(screen.getByText("detail-start"));
    await waitFor(() => expect(mockStart).toHaveBeenCalledWith("42"));
    fireEvent.click(screen.getByText("detail-complete"));
    await waitFor(() => expect(mockComplete).toHaveBeenCalledWith("42"));
    fireEvent.click(screen.getByText("detail-cancel"));
    await waitFor(() => expect(mockCancel).toHaveBeenCalledWith("42"));
    fireEvent.click(screen.getByText("detail-attendance"));
    await waitFor(() =>
      expect(mockPatchAttendance).toHaveBeenCalledWith("42", "21", {
        status: "present",
      }),
    );
    fireEvent.click(screen.getByText("quality-substitute"));
    await waitFor(() =>
      expect(mockSubstitute).toHaveBeenCalledWith("11", "12", "2026-05-04"),
    );

    fireEvent.click(screen.getByText("quality-edit"));
    expect(screen.getByText("event-save")).toBeInTheDocument();
    fireEvent.click(screen.getByText("event-save"));
    await waitFor(() => expect(mockTenantMutate).toHaveBeenCalled());

    fireEvent.click(screen.getByText("week-grid"));
    await waitFor(() =>
      expect(screen.getByText("detail-delete")).toBeVisible(),
    );
    fireEvent.click(screen.getByText("detail-repeat"));
    expect(screen.getByText("event-save-series")).toBeInTheDocument();
    fireEvent.click(screen.getByText("event-save-series"));
    await waitFor(() => expect(mockTenantMutate).toHaveBeenCalled());

    fireEvent.click(screen.getByText("detail-delete"));
    await waitFor(() => expect(mockDeleteCancelled).toHaveBeenCalledWith("42"));

    fireEvent.click(screen.getByText("week-grid"));
    fireEvent.click(screen.getByText("detail-delete-following"));
    await waitFor(() =>
      expect(mockEndTemplate).toHaveBeenCalledWith("7", {
        effective_date: "2026-05-04",
      }),
    );
  });

  it("handles series, month and year navigation callbacks", async () => {
    mockSearch.value = "view=series&period=5";
    render(<TimetablesPage />);

    expect(screen.getByText(/series:/)).toBeInTheDocument();
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
    expect(
      screen.queryByRole("dialog", { name: "Regeltermin archivieren?" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("archive-template"));
    fireEvent.click(screen.getByRole("button", { name: "Archivieren" }));
    await waitFor(() => expect(mockArchiveTemplate).toHaveBeenCalledWith("7"));
    expect(mockToastSuccess).toHaveBeenCalledWith(
      'Regeltermin "Yoga" archiviert',
    );
    expect(mockTenantMutate).toHaveBeenCalledWith("timetable-templates-5");

    fireEvent.click(screen.getByText("month-view"));
    await waitFor(() => expect(screen.getByText("month-grid")).toBeVisible());
    fireEvent.click(screen.getByText("month-grid"));

    fireEvent.click(screen.getByText("year-view"));
    await waitFor(() => expect(screen.getByText("year-month")).toBeVisible());
    fireEvent.click(screen.getByText("year-month"));
    fireEvent.click(screen.getByText("year-view"));
    await waitFor(() => expect(screen.getByText("year-day")).toBeVisible());
    fireEvent.click(screen.getByText("year-day"));

    fireEvent.click(screen.getByText("prev"));
    fireEvent.click(screen.getByText("next"));
    fireEvent.click(screen.getByText("today"));
    fireEvent.click(screen.getByText("density"));
    fireEvent.click(screen.getByText("add-instance"));
    expect(screen.getByText("event-save")).toBeInTheDocument();
  });

  it("keeps archive confirmation open and reports errors", async () => {
    mockSearch.value = "view=series&period=5";
    mockArchiveTemplate.mockRejectedValueOnce(new Error("Archivierung kaputt"));
    render(<TimetablesPage />);

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

    render(<TimetablesPage />);

    expect(screen.getByTestId("timetable-page-skeleton")).toBeVisible();
    expect(screen.getByTestId("timetable-toolbar-skeleton")).toBeVisible();
    expect(screen.getByLabelText("Monatsplan wird geladen")).toBeVisible();
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

    render(<TimetablesPage />);

    expect(screen.getByTestId("timetable-content-skeleton")).toBeVisible();
    expect(screen.getByLabelText("Monatsplan wird geladen")).toBeVisible();
    expect(screen.queryByTestId("loading")).not.toBeInTheDocument();
  });

  it("bootstraps a default period exactly once when none exist", async () => {
    setupSWR({ periods: [] });
    mockBootstrap.mockResolvedValue({ periods: [period], created: true });

    const { rerender } = render(<TimetablesPage />);

    await waitFor(() => expect(mockBootstrap).toHaveBeenCalledTimes(1));
    await waitFor(() =>
      expect(mockTenantMutate).toHaveBeenCalledWith(
        "database-calendar-periods-list",
      ),
    );

    rerender(<TimetablesPage />);
    expect(mockBootstrap).toHaveBeenCalledTimes(1);
  });

  it("does not bootstrap when periods already exist", () => {
    render(<TimetablesPage />);

    expect(mockBootstrap).not.toHaveBeenCalled();
  });

  // Issue #1651: the setup step reflects real care-offering linkage, not the
  // mere existence of an active enrollment phase.
  it("keeps the enrollment setup step open while no care offering is linked", () => {
    setupSWR({ careOfferingLink: { total: 3, linked: 0 } });

    render(<TimetablesPage />);

    const step = screen.getByRole("link", {
      name: /Mit der Anmeldung verknüpfen/,
    });
    expect(step).toHaveAttribute("href", "/care-offerings");
    expect(step).toHaveTextContent("0 von 3 Angeboten verknüpft");
  });

  it("marks the enrollment setup step done once a care offering is linked", () => {
    setupSWR({ careOfferingLink: { total: 3, linked: 2 } });

    render(<TimetablesPage />);

    const step = screen.getByRole("link", {
      name: /Mit der Anmeldung verknüpfen/,
    });
    expect(step).toHaveTextContent("2 von 3 Angeboten verknüpft");
    expect(step).toHaveTextContent("Angebote öffnen");
  });

  it("hides the enrollment setup step when the linkage cannot be read", () => {
    setupSWR({ careOfferingLink: null });

    render(<TimetablesPage />);

    expect(
      screen.queryByText("Mit der Anmeldung verknüpfen"),
    ).not.toBeInTheDocument();
  });

  it("treats a failed active-phase offering request as unknown", async () => {
    mockListPhases.mockResolvedValue([
      { id: "10", is_active: true },
      { id: "11", is_active: true },
    ]);
    mockListCareOfferings.mockImplementation((phaseId: string) => {
      if (phaseId === "11") return Promise.reject(new Error("forbidden"));
      return Promise.resolve([
        {
          phase_id: "10",
          is_active: true,
          activity_group_id: "7",
        },
      ]);
    });

    await expect(loadCareOfferingLinkSummary()).resolves.toBeNull();
    expect(mockLoggerWarn).toHaveBeenCalledWith(
      "timetable_care_offering_link_load_failed",
      { error: "forbidden" },
    );
  });

  it("summarizes only active offerings from active phases", async () => {
    mockListPhases.mockResolvedValue([
      { id: "10", is_active: true },
      { id: "11", is_active: false },
    ]);
    mockListCareOfferings.mockResolvedValue([
      {
        phase_id: "10",
        is_active: true,
        activity_group_id: "7",
      },
      {
        phase_id: "10",
        is_active: true,
        activity_group_id: null,
      },
      {
        phase_id: "10",
        is_active: false,
        activity_group_id: "8",
      },
    ]);

    await expect(loadCareOfferingLinkSummary()).resolves.toEqual({
      total: 2,
      linked: 1,
    });
    expect(mockListCareOfferings).toHaveBeenCalledTimes(1);
    expect(mockListCareOfferings).toHaveBeenCalledWith("10");
  });

  // A fresh school has no enrollment phases at all — "0 von 0 Angeboten
  // verknüpft" would be nonsense on the very screen the guide exists for.
  it("shows a plain hint instead of a 0-of-0 ratio when no offerings exist", () => {
    setupSWR({ careOfferingLink: { total: 0, linked: 0 } });

    render(<TimetablesPage />);

    const step = screen.getByRole("link", {
      name: /Mit der Anmeldung verknüpfen/,
    });
    expect(step).toHaveTextContent("Noch keine Angebote");
    expect(step).not.toHaveTextContent("0 von 0");
  });

  it("logs a warning when the bootstrap fails with 403", async () => {
    setupSWR({ periods: [] });
    mockBootstrap.mockRejectedValue(
      Object.assign(new Error("permission denied"), { httpStatus: 403 }),
    );

    render(<TimetablesPage />);

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

    render(<TimetablesPage />);

    await waitFor(() =>
      expect(mockLoggerError).toHaveBeenCalledWith("periods_bootstrap_failed", {
        error: "backend down",
      }),
    );
    expect(mockLoggerWarn).not.toHaveBeenCalledWith(
      "periods_bootstrap_failed",
      expect.anything(),
    );
  });

  it("opens the quick-create modal from an empty week slot", () => {
    mockSearch.value = "view=week";
    render(<TimetablesPage />);

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

    // Closing resets the prefill; the next manual create is the full form.
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

  it("passes the series list period to the event modal", () => {
    setupSWR({ periods: [period, laterPeriod] });
    mockSearch.value = "view=series&period=6";
    render(<TimetablesPage />);

    fireEvent.click(screen.getByText("create-template"));

    expect(mockEventModalProps).toHaveBeenLastCalledWith(
      expect.objectContaining({
        isOpen: true,
        defaultCalendarPeriodId: "6",
      }),
    );
  });

  it("hides the period pill for a fully covered week and reveals it via Verwaltung", () => {
    mockSearch.value = "view=week";
    render(<TimetablesPage />);

    expect(screen.queryByTestId("period-switcher")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("manage-periods"));
    expect(screen.getByTestId("period-switcher")).toBeInTheDocument();
  });

  it("renders no period UI when no periods exist and Verwaltung opens create", () => {
    setupSWR({ periods: [] });
    render(<TimetablesPage />);

    expect(screen.queryByTestId("period-switcher")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("manage-periods"));
    expect(screen.getByText("period-save")).toBeInTheDocument();
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
    render(<TimetablesPage />);

    expect(screen.getByTestId("timetable-disabled-state")).toBeVisible();
    expect(screen.getByText("Betreuungsplan ist deaktiviert")).toBeVisible();
    expect(screen.queryByTestId("toolbar")).not.toBeInTheDocument();
    expect(mockBootstrap).not.toHaveBeenCalled();
  });

  it("shows the skeleton while the settings schema loads", () => {
    setupSWR({ settingsSchemaLoading: true });
    render(<TimetablesPage />);

    expect(screen.getByTestId("timetable-page-skeleton")).toBeVisible();
    expect(screen.queryByTestId("toolbar")).not.toBeInTheDocument();
  });
});
