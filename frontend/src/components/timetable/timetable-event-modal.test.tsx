import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  within,
  waitFor,
} from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

// The date fields moved from native inputs to the kit picker; this stub keeps
// them settable via fireEvent.change and forwards min/max so the bound
// assertions below still pin what the component computes. Imported inside the
// factory because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const {
  mockToastSuccess,
  mockToastError,
  mockToastWarning,
  mockCreate,
  mockUpdate,
  mockCreateTemplate,
  mockConvertInstanceToSeries,
  mockUpdateTemplate,
  mockMaterialize,
  mockGetTemplate,
  mockSplitTemplate,
  mockReplanWeek,
  mockCountEditedInWindow,
  mockCheckConflicts,
  mockCheckShiftCoverage,
  mockFetchStudents,
  mockGetAllStaff,
  mockListPlanningTracks,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockToastWarning: vi.fn(),
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockCreateTemplate: vi.fn(),
  mockConvertInstanceToSeries: vi.fn(),
  mockUpdateTemplate: vi.fn(),
  mockMaterialize: vi.fn(),
  mockGetTemplate: vi.fn(),
  mockSplitTemplate: vi.fn(),
  mockReplanWeek: vi.fn(),
  mockCountEditedInWindow: vi.fn(),
  mockCheckConflicts: vi.fn(),
  mockCheckShiftCoverage: vi.fn(),
  mockFetchStudents: vi.fn(),
  mockGetAllStaff: vi.fn(),
  mockListPlanningTracks: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: mockToastError,
    warning: mockToastWarning,
  }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), info: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: mockFetchStudents,
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: {
    getAllStaff: mockGetAllStaff,
  },
}));

vi.mock("~/lib/planning-track-api", () => ({
  planningTrackService: {
    list: mockListPlanningTracks,
  },
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    create: mockCreate,
    update: mockUpdate,
    createTemplate: mockCreateTemplate,
    convertInstanceToSeries: mockConvertInstanceToSeries,
    updateTemplate: mockUpdateTemplate,
    materialize: mockMaterialize,
    getTemplate: mockGetTemplate,
    splitTemplate: mockSplitTemplate,
    replanWeek: mockReplanWeek,
    countEditedInWindow: mockCountEditedInWindow,
    checkConflicts: mockCheckConflicts,
    checkShiftCoverage: (probe: unknown) => mockCheckShiftCoverage(probe),
  },
}));

import { TimetableEventModal } from "./timetable-event-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { useTenant } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";
import type {
  EnrichedInstance,
  ShiftCoverageCheckParams,
  TimetableTemplate,
} from "~/lib/timetable-types";

const periods: CalendarPeriod[] = [
  {
    id: "5",
    tenantId: "1",
    name: "Schuljahr 2026/2027",
    periodType: "school_year",
    startDate: "2026-05-01",
    endDate: "2026-12-31",
    weekCycleLength: 1,
    weekCycleAnchor: "2026-05-04",
    isActive: true,
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
];

const templatePinnedPeriod: CalendarPeriod = {
  ...periods[0]!,
  id: "6",
  name: "Sommerplanung 2026",
  endDate: "2026-07-31",
};

const savedInstance: EnrichedInstance = {
  id: "42",
  date: "2026-05-04",
  startTime: "12:00",
  endTime: "13:00",
  title: "Mensa",
  status: "planned",
  isSpontaneous: false,
  isLive: false,
  activityType: "care",
  roomId: "3",
  roomName: "Mensa",
  staff: [],
  students: [],
  studentIds: [],
  staffCount: 0,
  absentStaffCount: 0,
  expectedStudentsCount: 0,
  notScheduledStudentsCount: 0,
  presentStudentsCount: 0,
  requiredStaffCount: 0,
  assignedStaffCount: 0,
  conflictWarnings: [],
};

const template: TimetableTemplate = {
  id: "7",
  name: "Yoga",
  type: "activity",
  categoryId: "2",
  categoryName: "AG",
  roomId: "3",
  roomName: "Turnhalle",
  isOpen: true,
  maxParticipants: 12,
  targetGroupType: "none",
  enrollmentCount: 8,
  supervisorCount: 1,
  requiredStaffCount: 1,
  assignedStaffCount: 1,
  studentIds: ["21"],
  staffIds: ["11"],
  primaryStaffId: "11",
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

const templateWithWeekdayRosters: TimetableTemplate = {
  ...template,
  studentIds: ["21", "22"],
  staffIds: ["11", "12"],
  schedules: [
    { ...template.schedules[0]!, weekday: 1 },
    { ...template.schedules[0]!, id: "10", weekday: 2 },
  ],
  weekdayAssignments: [
    {
      weekday: 1,
      staffIds: ["11"],
      studentIds: ["21"],
      primaryStaffId: "11",
    },
    {
      weekday: 2,
      staffIds: ["12"],
      studentIds: ["22"],
      primaryStaffId: "12",
    },
  ],
};

const templateWithTemplateOnlyPeriodPin: TimetableTemplate = {
  ...template,
  calendarPeriodId: "6",
  schedules: template.schedules.map((schedule) => ({
    ...schedule,
    calendarPeriodId: undefined,
  })),
};

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve;
    reject = promiseReject;
  });
  return { promise, resolve, reject };
}

function setupRefs() {
  vi.stubGlobal(
    "fetch",
    vi
      .fn()
      .mockResolvedValueOnce({
        json: async () => ({
          data: [{ id: 3, name: "Mensa", building: "Haus A" }],
        }),
      })
      .mockResolvedValueOnce({
        json: async () => ({
          data: [{ id: "2", name: "AG" }],
        }),
      })
      .mockResolvedValueOnce({
        json: async () => ({
          data: [{ id: 31, name: "Klasse 1a" }],
        }),
      }),
  );
  mockFetchStudents.mockResolvedValue({
    students: [
      {
        id: "21",
        name: "Max Kind",
        school_class: "1a",
        group_name: "OGS Blau",
      },
    ],
  });
  mockGetAllStaff.mockResolvedValue([
    {
      id: "11",
      name: "Ada Staff",
    },
  ]);
}

function setupRepeatableReferenceFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: unknown) => {
      const url = String(input);
      if (url.includes("/api/rooms")) {
        return {
          json: async () => ({
            data: [{ id: 3, name: "Mensa", building: "Haus A" }],
          }),
        };
      }
      if (url.includes("/api/activities/categories")) {
        return {
          json: async () => ({ data: [{ id: "2", name: "AG" }] }),
        };
      }
      return {
        json: async () => ({ data: [{ id: 31, name: "Klasse 1a" }] }),
      };
    }),
  );
}

function renderModal(
  props: Partial<React.ComponentProps<typeof TimetableEventModal>> = {},
) {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  const modal = (isOpen: boolean) => (
    <TimetableEventModal
      isOpen={isOpen}
      onClose={onClose}
      onSaved={onSaved}
      defaultDate="2026-05-04"
      weekFrom="2026-05-04"
      weekTo="2026-05-08"
      calendarPeriods={periods}
      defaultCalendarPeriodId="5"
      canCheckShiftCoverage
      canManageCategories
      {...props}
    />
  );
  const rendered = render(modal(true));
  return {
    ...rendered,
    onClose,
    onSaved,
    setOpen: (isOpen: boolean) => rendered.rerender(modal(isOpen)),
  };
}

/**
 * Wizard navigation helpers. The form is a three-step wizard
 * ("Termin" = 1, "Wiederholung" = 2, "Personal und Kinder" = 3); only the
 * current step is mounted, so a test that touches a field of a later step has
 * to walk there first. These helpers only navigate — they never assert.
 */
function currentStep(): number {
  const stepper = document.querySelector('[aria-label^="Schritt "]');
  const label = stepper?.getAttribute("aria-label") ?? "";
  return Number(/Schritt (\d+) von/.exec(label)?.[1] ?? 1);
}

/**
 * "Weiter" runs the unchanged validateForm and refuses to leave step 1 while
 * Titel or Raum are empty. Fill only what the test left empty so a test's own
 * values always win, and only when we actually have to pass step 1.
 */
/**
 * Pick an option from a single or multiple select. Multiple selects expose
 * native checkboxes so their checked state remains accessible.
 */
async function chooseFromSelect(
  trigger: HTMLElement,
  optionLabel: string | RegExp,
) {
  await waitFor(() => expect(trigger).not.toBeDisabled());
  fireEvent.click(trigger);
  const option = await waitFor(() => {
    const control =
      screen.queryByRole("option", { name: optionLabel }) ??
      screen.queryByRole("checkbox", { name: optionLabel });
    if (!control) throw new Error("select option is not visible");
    return control;
  });
  fireEvent.click(option);
}

async function fillStep1Requirements() {
  const title = screen.getByLabelText(/^Titel\*/) as HTMLInputElement;
  if (title.value === "") {
    fireEvent.change(title, { target: { value: "Testtermin" } });
  }
  const room = await screen.findByLabelText(/^Raum\*/);
  if (!room.textContent?.includes("Haus A - Mensa")) {
    await chooseFromSelect(room, "Haus A - Mensa");
  }
}

async function goToStep(target: 1 | 2 | 3) {
  while (currentStep() > target) {
    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
  }
  if (currentStep() === target) return;
  if (currentStep() === 1) await fillStep1Requirements();
  while (currentStep() < target) {
    const before = currentStep();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    if (currentStep() === before) {
      throw new Error(
        `Wizard-Navigation blockiert auf Schritt ${before} (Ziel ${target}).`,
      );
    }
  }
}

/**
 * #2025: "Speichern" only exists on the last wizard step ("Personal und
 * Kinder"), so every save walks there first. Tests that were already on step 3
 * are unaffected — goToStep returns immediately. The click and all assertions
 * around it stay unchanged.
 */
async function clickSave() {
  await goToStep(3);
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
}

/** Same gate as clickSave(), for the "saving is not blocked" assertions. */
async function expectSaveEnabled() {
  await goToStep(3);
  expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
}

describe("TimetableEventModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: { gradeLevelMax: 13 } as TenantInfo,
    });
    setupRefs();
    mockCreate.mockResolvedValue(savedInstance);
    mockUpdate.mockResolvedValue(savedInstance);
    mockCreateTemplate.mockResolvedValue({
      templateId: "7",
      instancesCreated: 2,
    });
    mockConvertInstanceToSeries.mockResolvedValue({
      templateId: "7",
      timeframeId: "1",
      scheduleIds: ["1"],
      linkedInstanceId: "42",
    });
    mockUpdateTemplate.mockResolvedValue(template);
    mockMaterialize.mockResolvedValue({ instancesCreated: 1, warnings: [] });
    mockGetTemplate.mockResolvedValue(template);
    mockSplitTemplate.mockResolvedValue({
      oldTemplateId: "7",
      newTemplateId: "12",
      scheduleIds: ["31"],
      deletedInstances: 1,
      instancesCreated: 4,
    });
    mockReplanWeek.mockResolvedValue({
      from: "2026-05-04",
      to: "2026-06-28",
      deletedInstances: 0,
      candidatesSkippedExisting: 0,
      instancesCreated: 2,
      instanceStudentsCreated: 0,
      instanceStaffCreated: 0,
      warnings: [],
      durationMs: 1,
    });
    // #1875: default to "no single-occurrence edits" so series-edit tests keep
    // the pre-warning flow; the warning tests override this per-case.
    mockCountEditedInWindow.mockResolvedValue({ count: 0, occurrences: [] });
    mockCheckConflicts.mockResolvedValue({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [],
    });
    mockCheckShiftCoverage.mockResolvedValue({ coverageWarnings: [] });
    mockListPlanningTracks.mockResolvedValue([
      {
        id: "8",
        name: "Mittag",
        color: "#F78C10",
        sortOrder: 1,
      },
    ]);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("creates a one-off instance with selected people", async () => {
    const { onClose, onSaved } = renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    await goToStep(1);
    fireEvent.change(screen.getByLabelText("Tagesnotiz"), {
      target: { value: "ohne Nuesse" },
    });
    await clickSave();

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({
          title: "Mensa",
          room_id: 3,
          notes: "ohne Nuesse",
          student_ids: [21],
          staff_ids: [11],
        }),
      ),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith("Termin angelegt");
    expect(onSaved).toHaveBeenCalledWith({
      kind: "instance",
      instance: savedInstance,
    });
    expect(onClose).toHaveBeenCalledOnce();
  });

  // #2032: Ein Termin darf auf einem Schließtag liegen, das Speichern fragt
  // aber einmal nach.
  const closingRanges = [
    {
      startDate: "2026-05-04",
      endDate: "2026-05-04",
      reason: "Pädagogischer Tag",
    },
  ];

  it("asks before saving on a closing day and saves after confirming (#2032)", async () => {
    renderModal({ closingDayRanges: closingRanges });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(
      screen.getByText(/Am 04\.05\.2026 ist ein Schließtag hinterlegt/),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Ferienbetreuung" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await clickSave();

    expect(
      await screen.findByText("An einem Schließtag planen?"),
    ).toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Trotzdem planen" }));

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({ title: "Ferienbetreuung" }),
      ),
    );
  });

  it("writes nothing when the closing-day warning is dismissed (#2032)", async () => {
    renderModal({ closingDayRanges: closingRanges });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Ferienbetreuung" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await clickSave();

    const dialogCancel = within(
      await screen.findByRole("dialog", {
        name: "An einem Schließtag planen?",
      }),
    ).getByRole("button", { name: "Abbrechen" });
    fireEvent.click(dialogCancel);

    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("shows the required-field errors before the closing-day question (#2032)", async () => {
    renderModal({ closingDayRanges: closingRanges });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    // Titel und Raum bleiben leer: das Formular würde gar nicht speichern.
    // #2025: Speichern gibt es erst im letzten Schritt, den Pflichtfeld-Fehler
    // zeigt deshalb schon das blockierte "Weiter".
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Bitte einen Titel eingeben."),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("An einem Schließtag planen?"),
    ).not.toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("keeps saving disabled until closing days have loaded", async () => {
    renderModal({ closingDaysLoading: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.submit(document.querySelector("#timetable-event-form")!);
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("saves without asking when the date is no closing day (#2032)", async () => {
    renderModal({
      closingDayRanges: [
        { startDate: "2026-05-05", endDate: "2026-05-05", reason: "Brücke" },
      ],
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await clickSave();

    await waitFor(() => expect(mockCreate).toHaveBeenCalled());
  });

  it("asks when a later recurring occurrence falls on a closing day (#2032)", async () => {
    renderModal({
      variant: "quick",
      closingDayRanges: [
        {
          startDate: "2026-05-11",
          endDate: "2026-05-11",
          reason: "Konzeptionstag",
        },
      ],
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Montagsangebot" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Wiederholt sich"),
      "Wöchentlich am Montag",
    );

    expect(
      screen.getByText(
        /Regeltermin fällt am 11\.05\.2026 auf einen Schließtag/,
      ),
    ).toBeInTheDocument();
    await clickSave();

    const dialog = await screen.findByRole("dialog", {
      name: "An einem Schließtag planen?",
    });
    expect(within(dialog).getByText(/11\.05\.2026/)).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();

    fireEvent.click(
      within(dialog).getByRole("button", { name: "Trotzdem planen" }),
    );
    await waitFor(() => expect(mockCreateTemplate).toHaveBeenCalled());
  });

  // #2135: a new series starts at the picked Datum. Occurrences before it can
  // never be materialized, so a closing day before the start must not trigger
  // the question.
  it("ignores closing days before the series start date", async () => {
    renderModal({
      variant: "quick",
      defaultDate: "2026-05-11",
      closingDayRanges: [
        {
          startDate: "2026-05-04",
          endDate: "2026-05-04",
          reason: "Konzeptionstag",
        },
      ],
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Montagsangebot" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Wiederholt sich"),
      "Wöchentlich am Montag",
    );

    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
    await clickSave();

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({ start_date: "2026-05-11" }),
      ),
    );
    expect(
      screen.queryByRole("dialog", { name: "An einem Schließtag planen?" }),
    ).not.toBeInTheDocument();
  });

  it("filters and bulk-selects visible student rows", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [
        {
          id: "21",
          name: "Max Kind",
          school_class: "1a",
          group_name: "OGS Blau",
        },
        {
          id: "22",
          name: "Mila Kind",
          school_class: "2b",
          group_name: "OGS Rot",
        },
      ],
    });
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    expect(screen.getByText("Max Kind")).toBeInTheDocument();
    expect(screen.getByText("Mila Kind")).toBeInTheDocument();

    fireEvent.change(screen.getByPlaceholderText("Kinder suchen …"), {
      target: { value: "Mila" },
    });
    expect(screen.queryByText("Max Kind")).not.toBeInTheDocument();
    expect(screen.getByText("Mila Kind")).toBeInTheDocument();

    fireEvent.click(
      screen.getAllByRole("button", { name: "Sichtbare auswählen" })[0]!,
    );
    expect(screen.getByText("1 ausgewählt")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Filter zurücksetzen" }),
    );
    expect(screen.getByText("Max Kind")).toBeInTheDocument();
    expect(screen.getByText("Mila Kind")).toBeInTheDocument();

    await chooseFromSelect(
      screen.getByLabelText("Nach Jahrgang filtern"),
      "Jahrgang 1",
    );
    expect(screen.getByText("Max Kind")).toBeInTheDocument();
    expect(screen.queryByText("Mila Kind")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Filter zurücksetzen" }),
    );

    fireEvent.click(
      screen.getAllByRole("button", { name: "Auswahl leeren" })[0]!,
    );
    expect(screen.getAllByText("0 ausgewählt").length).toBeGreaterThan(0);
  });

  it("loads every student page for complete roster selection", async () => {
    mockFetchStudents.mockImplementation(({ page }: { page?: number } = {}) =>
      Promise.resolve(
        page === 2
          ? {
              students: [
                {
                  id: "22",
                  name: "Mila Zweite Seite",
                  school_class: "3b",
                  group_name: "OGS Rot",
                },
              ],
              pagination: {
                current_page: 2,
                page_size: 500,
                total_pages: 2,
                total_records: 2,
              },
            }
          : {
              students: [
                {
                  id: "21",
                  name: "Max Erste Seite",
                  school_class: "3a",
                  group_name: "OGS Blau",
                },
              ],
              pagination: {
                current_page: 1,
                page_size: 500,
                total_pages: 2,
                total_records: 2,
              },
            },
      ),
    );

    renderModal();

    await goToStep(3);
    expect(await screen.findByText("Max Erste Seite")).toBeInTheDocument();
    expect(await screen.findByText("Mila Zweite Seite")).toBeInTheDocument();
    expect(mockFetchStudents).toHaveBeenCalledWith({
      page: 1,
      page_size: 500,
    });
    expect(mockFetchStudents).toHaveBeenCalledWith({
      page: 2,
      page_size: 500,
    });
  });

  it("loads planner references without waiting for the student catalog", async () => {
    const studentRequest = deferred<{
      students: Array<{
        id: string;
        name: string;
        school_class: string;
        group_name: string;
      }>;
    }>();
    mockFetchStudents.mockReturnValue(studentRequest.promise);

    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(screen.getByLabelText("Raum*")).toBeEnabled();
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await goToStep(3);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    expect(screen.getByLabelText(/^Jahrgang\*/)).toBeEnabled();
    expect(screen.getByText(/Kinderliste wird geladen/)).toBeVisible();

    await act(async () => {
      studentRequest.resolve({ students: [] });
      await studentRequest.promise;
    });
  });

  it("keeps saving available and retries when a later student page fails", async () => {
    let secondPageFails = true;
    mockFetchStudents.mockImplementation(({ page }: { page?: number } = {}) => {
      if (page === 2 && secondPageFails) {
        return Promise.reject(new Error("secondary page unavailable"));
      }
      return Promise.resolve({
        students: [
          page === 2
            ? {
                id: "22",
                name: "Mila Zweite Seite",
                school_class: "3b",
                group_name: "OGS Rot",
              }
            : {
                id: "21",
                name: "Max Erste Seite",
                school_class: "3a",
                group_name: "OGS Blau",
              },
        ],
        pagination: {
          current_page: page ?? 1,
          page_size: 500,
          total_pages: 2,
          total_records: 2,
        },
      });
    });

    renderModal();

    await goToStep(3);
    expect(
      await screen.findByText(
        "Die Kinderliste konnte nicht vollständig geladen werden. Die Kinderzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    await expectSaveEnabled();
    expect(screen.queryByText("Max Erste Seite")).not.toBeInTheDocument();

    secondPageFails = false;
    fireEvent.click(
      screen.getByRole("button", { name: "Kinder erneut laden" }),
    );

    expect(await screen.findByText("Max Erste Seite")).toBeInTheDocument();
    expect(await screen.findByText("Mila Zweite Seite")).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Kinderliste konnte nicht vollständig/),
    ).not.toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("keeps saving available and retries when staff cannot be loaded", async () => {
    mockGetAllStaff
      .mockRejectedValueOnce(new Error("forbidden"))
      .mockResolvedValueOnce([{ id: "11", name: "Ada Staff" }]);

    renderModal();

    await goToStep(3);
    expect(
      await screen.findByText(
        "Die Personalliste konnte nicht vollständig geladen werden. Die Personalzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Ada Staff")).not.toBeInTheDocument();
    await expectSaveEnabled();

    fireEvent.click(
      screen.getByRole("button", { name: "Personal erneut laden" }),
    );

    expect(await screen.findByText("Ada Staff")).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Personalliste konnte nicht vollständig/),
    ).not.toBeInTheDocument();
  });

  it("blocks the shared staff editor when a weekday roster cannot load students", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    renderModal({ initialSeries: templateWithWeekdayRosters });

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    expect(
      screen.getByText(
        /Die wochentagsspezifischen Zuordnungen können erst bearbeitet werden/,
      ),
    ).toBeVisible();
    expect(
      screen.queryByRole("checkbox", { name: /Ada Staff/ }),
    ).not.toBeInTheDocument();

    await clickSave();
    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          weekday_assignments: [
            {
              weekday: 1,
              staff_ids: [11],
              student_ids: [21],
              primary_staff_id: 11,
            },
            {
              weekday: 2,
              staff_ids: [12],
              student_ids: [22],
              primary_staff_id: 12,
            },
          ],
        }),
      ),
    );
  });

  it("blocks the shared student editor when a weekday roster cannot load staff", async () => {
    mockGetAllStaff.mockRejectedValue(new Error("forbidden"));
    renderModal({ initialSeries: templateWithWeekdayRosters });

    await goToStep(3);
    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    expect(
      screen.getByText(
        /Die wochentagsspezifischen Zuordnungen können erst bearbeitet werden/,
      ),
    ).toBeVisible();
    expect(
      screen.queryByRole("checkbox", { name: /Max Kind/ }),
    ).not.toBeInTheDocument();

    await clickSave();
    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          weekday_assignments: [
            {
              weekday: 1,
              staff_ids: [11],
              student_ids: [21],
              primary_staff_id: 11,
            },
            {
              weekday: 2,
              staff_ids: [12],
              student_ids: [22],
              primary_staff_id: 12,
            },
          ],
        }),
      ),
    );
  });

  it("reveals a student load failure immediately in quick mode", async () => {
    mockFetchStudents.mockRejectedValue(new Error("students unavailable"));

    renderModal({ variant: "quick" });

    // Die frühere "Weitere Optionen"-Disclosure gibt es im Wizard nicht mehr:
    // der Ladefehler muss trotzdem SOFORT sichtbar sein (Alert in der
    // Wizard-Hülle auf Schritt 1), der Erneut-laden-Button liegt in
    // Schritt 3 — die fachliche Aussage (Fehler sichtbar, erneut laden
    // möglich, Speichern nicht blockiert) bleibt unverändert.
    expect(
      await screen.findByText(
        "Die Kinderliste konnte nicht vollständig geladen werden. Die Kinderzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    await goToStep(3);
    expect(
      screen.getByRole("button", { name: "Kinder erneut laden" }),
    ).toBeVisible();
    await expectSaveEnabled();
  });

  it("preserves an existing roster when the account cannot load students", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    renderModal({
      initialInstance: { ...savedInstance, studentIds: ["21"] },
    });

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    expect(screen.queryByText("Max Kind")).not.toBeInTheDocument();
    await clickSave();

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ student_ids: [21] }),
      ),
    );
  });

  it("shows and preserves an existing class target without student access", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    renderModal({
      initialSeries: {
        ...template,
        targetGroupType: "klasse",
        targetSchoolClass: "Klasse 3a",
      },
      showPeriodField: true,
    });

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    const classSelect = screen.getByLabelText(/^Klasse\*/);
    expect(classSelect).toHaveTextContent("Klasse 3a");
    expect(classSelect).toBeDisabled();
    expect(
      screen.getByText(/bestehende Klassen-Zielgruppe bleibt unverändert/),
    ).toBeVisible();

    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          target_group_type: "klasse",
          target_school_class: "Klasse 3a",
          student_ids: [21],
        }),
      ),
    );
  });

  it("saves a new empty roster when the account cannot load students", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    renderModal();

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    await goToStep(1);
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await clickSave();

    await waitFor(() =>
      expect(mockCreate).toHaveBeenCalledWith(
        expect.objectContaining({ student_ids: [] }),
      ),
    );
  });

  it("ignores a stale student failure after the modal is reopened", async () => {
    setupRepeatableReferenceFetch();
    const staleRequest = deferred<{
      students: Array<{
        id: string;
        name: string;
        school_class: string;
        group_name: string;
      }>;
    }>();
    mockFetchStudents
      .mockImplementationOnce(() => staleRequest.promise)
      .mockResolvedValueOnce({
        students: [
          {
            id: "22",
            name: "Neue Kinderliste",
            school_class: "3b",
            group_name: "OGS Rot",
          },
        ],
      });

    const { setOpen } = renderModal();
    await waitFor(() => expect(mockFetchStudents).toHaveBeenCalledOnce());

    setOpen(false);
    setOpen(true);

    await goToStep(3);
    expect(await screen.findByText("Neue Kinderliste")).toBeInTheDocument();

    await act(async () => {
      staleRequest.reject(new Error("stale student request failed"));
      await staleRequest.promise.catch(() => undefined);
    });

    expect(screen.getByText("Neue Kinderliste")).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Kinderliste konnte nicht vollständig/),
    ).not.toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("ignores a stale retry after the modal is reopened", async () => {
    setupRepeatableReferenceFetch();
    const staleRetry = deferred<{
      students: Array<{
        id: string;
        name: string;
        school_class: string;
        group_name: string;
      }>;
    }>();
    mockFetchStudents
      .mockRejectedValueOnce(new Error("initial student request failed"))
      .mockImplementationOnce(() => staleRetry.promise)
      .mockResolvedValueOnce({
        students: [
          {
            id: "23",
            name: "Liste nach Wiederöffnung",
            school_class: "4a",
            group_name: "OGS Gelb",
          },
        ],
      });

    const { setOpen } = renderModal();
    await goToStep(3);
    fireEvent.click(
      await screen.findByRole("button", { name: "Kinder erneut laden" }),
    );
    await waitFor(() => expect(mockFetchStudents).toHaveBeenCalledTimes(2));

    setOpen(false);
    setOpen(true);

    await goToStep(3);
    expect(
      await screen.findByText("Liste nach Wiederöffnung"),
    ).toBeInTheDocument();

    await act(async () => {
      staleRetry.reject(new Error("stale retry failed"));
      await staleRetry.promise.catch(() => undefined);
    });

    expect(screen.getByText("Liste nach Wiederöffnung")).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Kinderliste konnte nicht vollständig/),
    ).not.toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("validates shared fields before submitting", async () => {
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Ende*"), {
      target: { value: "11:00" },
    });
    // #2025: Speichern lives on the last step only, and "Weiter" refuses to
    // leave a step whose own fields are invalid — so an invalid Endzeit is
    // caught here, with the unchanged error text and nothing written.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Endzeit muss nach der Startzeit liegen."),
    ).toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("shows inline required-field errors on submit and clears them on change", async () => {
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    // #2025: see above — the empty required fields block "Weiter" now.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Bitte einen Titel eingeben."),
    ).toBeInTheDocument();
    expect(screen.getByText("Bitte einen Raum auswählen.")).toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();

    fireEvent.change(screen.getByLabelText(/Titel\*/), {
      target: { value: "Mensa" },
    });
    expect(
      screen.queryByText("Bitte einen Titel eingeben."),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Bitte einen Raum auswählen.")).toBeInTheDocument();
  });

  it("shows inline errors for series-only fields", async () => {
    // Wizard adaptation (user-approved): the errors live on two steps and can
    // no longer be asserted in one view — step 2 errors are checked from
    // step 2, then Zurück reveals the Kategorie error on step 1. Every
    // expected error text is unchanged. Planungszeitraum and Kategorie are
    // required CustomSelects that can no longer be cleared by interaction, so
    // they start empty instead (no default period + empty category catalog).
    // #2025: since Speichern only exists on the last step, the blocked
    // "Weiter" is what surfaces each step's errors — same texts, same
    // "nothing written" guarantee.
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce({
          json: async () => ({
            data: [{ id: 3, name: "Mensa", building: "Haus A" }],
          }),
        })
        .mockResolvedValueOnce({
          json: async () => ({ data: [] }),
        })
        .mockResolvedValueOnce({
          json: async () => ({ data: [] }),
        }),
    );
    renderModal({ showPeriodField: true, defaultCalendarPeriodId: "" });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Bitte einen Planungszeitraum auswählen."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte mindestens einen Wochentag auswählen."),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    expect(
      screen.queryByText("Bitte mindestens einen Wochentag auswählen."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByText("Bitte eine Kategorie auswählen."),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
  });

  it("requires the value belonging to each selected Zielgruppe", async () => {
    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Lernzeit" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });

    await goToStep(3);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    await clickSave();
    expect(
      await screen.findByText("Bitte einen Jahrgang auswählen."),
    ).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Klasse" }), {
      button: 0,
    });
    await clickSave();
    expect(
      await screen.findByText("Bitte eine Klasse auswählen."),
    ).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Gruppe" }), {
      button: 0,
    });
    await clickSave();
    expect(
      await screen.findByText("Bitte eine Gruppe auswählen."),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
  });

  it("creates a recurring series and materializes the full period in 56-day chunks", async () => {
    const { onSaved } = renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await goToStep(1);
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await chooseFromSelect(screen.getByLabelText("Planungsspur"), "Mittag");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2026/2027",
    );
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    await chooseFromSelect(
      screen.getByLabelText("Zuständige Person"),
      "Ada Staff",
    );
    await clickSave();

    // #2135: the picked Datum (2026-05-04) is the series start — it travels
    // as start_date and becomes the schedules' valid_from. The
    // materialization windows still cover the whole period
    // (2026-05-01 … 2026-12-31) because they run tenant-wide and must keep
    // backfilling earlier-starting templates; the create call carries the
    // first 56-day window, the remaining four follow as materialize calls.
    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Yoga",
          type: "activity",
          room_id: 3,
          category_id: 2,
          planning_track_id: 8,
          calendar_period_id: 5,
          start_date: "2026-05-04",
          materialize_from: "2026-05-01",
          materialize_to: "2026-06-25",
          primary_staff_id: 11,
        }),
      ),
    );
    await waitFor(() => expect(mockMaterialize).toHaveBeenCalledTimes(4));
    expect(mockMaterialize).toHaveBeenNthCalledWith(
      1,
      "2026-06-26",
      "2026-08-20",
    );
    expect(mockMaterialize).toHaveBeenLastCalledWith(
      "2026-12-11",
      "2026-12-31",
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Regeltermin angelegt: 6 Termine eingetragen",
    );
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "7" });
  });

  it("keeps an archived planning track visible on its existing series", async () => {
    mockListPlanningTracks.mockResolvedValue([
      {
        id: "8",
        name: "Alt",
        color: "#F78C10",
        sortOrder: 1,
        archivedAt: "2026-08-03T12:00:00Z",
      },
    ]);
    renderModal({
      initialSeries: {
        ...template,
        planningTrackId: "8",
        planningTrackName: "Alt",
        planningTrackColor: "#F78C10",
        planningTrackSortOrder: 1,
      },
    });

    const trigger = await screen.findByLabelText("Planungsspur");
    await chooseFromSelect(trigger, "Alt (archiviert)");
    expect(trigger).toHaveTextContent("Alt (archiviert)");
  });

  // #2135: the picked Datum becomes the series start, so it must lie inside
  // the selected planning period.
  it("rejects a series start date outside the selected period", async () => {
    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2026/2027",
    );
    await goToStep(1);
    fireEvent.change(screen.getByLabelText("Datum*"), {
      target: { value: "2027-02-01" },
    });
    // Datum is a step-0 field, so "Weiter" already blocks on the violation.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText(
        "Das Datum muss im gewählten Planungszeitraum liegen.",
      ),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
  });

  // #2135 follow-up: schools pre-plan the next school year while the wizard's
  // Datum still defaults to today. Selecting a period that does not contain
  // the Datum moves it to the first selected weekday inside the period
  // instead of hard-blocking on the wizard's own default.
  it("moves the Datum into a future period on period selection", async () => {
    const futurePeriod: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      name: "Schuljahr 2027/2028",
      startDate: "2027-08-01",
      endDate: "2028-07-31",
      weekCycleAnchor: "2027-08-02",
    };
    const { onSaved } = renderModal({
      showPeriodField: true,
      calendarPeriods: [...periods, futurePeriod],
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await goToStep(1);
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2027/2028",
    );
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    await clickSave();

    // defaultDate 2026-05-04 is a Monday; the first Monday inside the
    // future period is 2027-08-02. Materialization still spans the whole
    // period (tenant-wide backfill), starting at 2027-08-01.
    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          calendar_period_id: 9,
          start_date: "2027-08-02",
          materialize_from: "2027-08-01",
        }),
      ),
    );
    expect(
      screen.queryByText(
        "Das Datum muss im gewählten Planungszeitraum liegen.",
      ),
    ).toBeNull();
    await waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "7" }),
    );
  });

  // The A/B predicate applies to the moved Datum too: with "Alle 2 Wochen"
  // the first selected weekday inside the future period can sit in the other
  // week slot, which the materializer would skip; the series would then start
  // a week after the displayed date.
  it("moves the Datum onto the matching A/B week when biweekly is selected", async () => {
    const abPeriod: CalendarPeriod = {
      ...periods[0]!,
      weekCycleLength: 2,
    };
    const futurePeriod: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      name: "Schuljahr 2027/2028",
      startDate: "2027-08-01",
      endDate: "2028-07-31",
      weekCycleLength: 2,
      // Anchor 2027-08-09: the period's first Monday (2027-08-02) falls into
      // Woche B, the series' Woche A first materializes on 2027-08-09.
      weekCycleAnchor: "2027-08-09",
    };
    renderModal({
      showPeriodField: true,
      calendarPeriods: [abPeriod, futurePeriod],
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Alle 2 Wochen" }), {
      button: 0,
    });
    // defaultDate 2026-05-04 is Woche A of the current period.
    expect(screen.getByRole("tab", { name: "Woche A" })).toHaveAttribute(
      "aria-selected",
      "true",
    );
    await goToStep(1);
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2027/2028",
    );
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    await clickSave();

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          calendar_period_id: 9,
          week_pattern: 1,
          start_date: "2027-08-09",
          materialize_from: "2027-08-01",
        }),
      ),
    );
    expect(
      screen.queryByText(
        "Das Datum muss im gewählten Planungszeitraum liegen.",
      ),
    ).toBeNull();
  });

  it("submits multiple selected Jahrgang targets", async () => {
    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Hausaufgabenbetreuung" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await goToStep(1);
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2026/2027",
    );

    await goToStep(3);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    fireEvent.click(screen.getByLabelText(/^Jahrgang\*/));
    expect(
      screen.getByRole("checkbox", { name: "Jahrgang 13" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("checkbox", { name: "Jahrgang 14" })).toBeNull();
    fireEvent.click(screen.getByRole("checkbox", { name: "Jahrgang 12" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "Jahrgang 13" }));

    await clickSave();

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          target_group_type: "jahrgang",
          target_grade_level: 12,
          targets: [
            { type: "jahrgang", grade_level: 12 },
            { type: "jahrgang", grade_level: 13 },
          ],
        }),
      ),
    );
  });

  it("allows removing an existing grade above the current tenant cap", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: { gradeLevelMax: 4 } as TenantInfo,
    });
    renderModal({
      initialSeries: {
        ...template,
        targetGroupType: "jahrgang",
        targetGradeLevel: 4,
        targets: [
          { type: "jahrgang", gradeLevel: 4 },
          { type: "jahrgang", gradeLevel: 13 },
        ],
      },
      showPeriodField: true,
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    const gradeSelect = screen.getByLabelText(/^Jahrgang\*/);
    expect(gradeSelect).toHaveTextContent("2 ausgewählt");
    fireEvent.click(gradeSelect);
    expect(
      screen.getByRole("checkbox", { name: "Jahrgang 13 (bestehend)" }),
    ).toBeEnabled();
    expect(
      screen.getByText(/über der aktuell konfigurierten Höchststufe 4/),
    ).toBeVisible();

    fireEvent.click(
      screen.getByRole("checkbox", { name: "Jahrgang 13 (bestehend)" }),
    );

    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          target_group_type: "jahrgang",
          target_grade_level: 4,
          targets: [{ type: "jahrgang", grade_level: 4 }],
        }),
      ),
    );
  });

  it("allows adding a valid grade while retaining one above the tenant cap", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: { gradeLevelMax: 4 } as TenantInfo,
    });
    renderModal({
      initialSeries: {
        ...template,
        targetGroupType: "jahrgang",
        targetGradeLevel: 4,
        targets: [
          { type: "jahrgang", gradeLevel: 4 },
          { type: "jahrgang", gradeLevel: 13 },
        ],
      },
      showPeriodField: true,
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    fireEvent.click(screen.getByLabelText(/^Jahrgang\*/));
    fireEvent.click(screen.getByRole("checkbox", { name: "Jahrgang 3" }));

    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          target_group_type: "jahrgang",
          target_grade_level: 4,
          targets: [
            { type: "jahrgang", grade_level: 4 },
            { type: "jahrgang", grade_level: 13 },
            { type: "jahrgang", grade_level: 3 },
          ],
        }),
      ),
    );
  });

  it("adds the selected target cohort without replacing existing children", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [
        {
          id: "21",
          name: "Mara Drei A",
          school_class: "3a",
          group_name: "OGS Blau",
        },
        {
          id: "22",
          name: "Mika Drei B",
          school_class: "3b",
          group_name: "OGS Rot",
        },
        {
          id: "23",
          name: "Nora Vier A",
          school_class: "4a",
          group_name: "OGS Gelb",
        },
      ],
    });
    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Nora Vier A/ }));
    await goToStep(1);
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Hausaufgabenbetreuung" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await goToStep(1);
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2026/2027",
    );
    await goToStep(3);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    await chooseFromSelect(screen.getByLabelText(/^Jahrgang\*/), "Jahrgang 3");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Alle 2 Kinder aus Jahrgang 3 übernehmen",
      }),
    );

    expect(screen.getByRole("checkbox", { name: /Mara Drei A/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Mika Drei B/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Nora Vier A/ })).toBeChecked();
    expect(screen.getByText("3 ausgewählt")).toBeInTheDocument();

    await clickSave();

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          target_group_type: "jahrgang",
          target_grade_level: 3,
          student_ids: expect.arrayContaining([21, 22, 23]),
        }),
      ),
    );
  });

  it("clears the grade level when switching Zielgruppe away from Jahrgang", async () => {
    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Hausaufgabenbetreuung" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await goToStep(1);
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Schuljahr 2026/2027",
    );

    await goToStep(3);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    await chooseFromSelect(screen.getByLabelText(/^Jahrgang\*/), "Jahrgang 3");
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Keine" }), {
      button: 0,
    });

    await clickSave();

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          target_group_type: "none",
          target_grade_level: undefined,
        }),
      ),
    );
  });

  it("clears the legacy group target when switching Zielgruppe to none", async () => {
    renderModal({
      initialSeries: {
        ...template,
        targetGroupType: "gruppe",
        educationGroupId: "31",
      },
      showPeriodField: true,
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Keine" }), {
      button: 0,
    });

    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          target_group_type: "none",
          education_group_id: undefined,
          targets: [],
        }),
      ),
    );
  });

  it("preserves an independent education group on an existing series", async () => {
    renderModal({
      initialSeries: {
        ...template,
        targetGroupType: "none",
        educationGroupId: "31",
      },
      showPeriodField: true,
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga aktualisiert" },
    });
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          name: "Yoga aktualisiert",
          target_group_type: "none",
          education_group_id: 31,
        }),
      ),
    );
  });

  it("initializes and saves a direct series edit with its template-only period pin", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));

    renderModal({
      initialSeries: templateWithTemplateOnlyPeriodPin,
      calendarPeriods: [...periods, templatePinnedPeriod],
      showPeriodField: true,
    });

    await screen.findByText("Regeltermin bearbeiten");
    await goToStep(2);
    expect(screen.getByLabelText("Planungszeitraum*")).toHaveTextContent(
      "Sommerplanung 2026",
    );

    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ calendar_period_id: 6 }),
      ),
    );
  });

  it("ignores closing days before a direct series segment starts", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));

    renderModal({
      initialSeries: {
        ...template,
        schedules: template.schedules.map((schedule) => ({
          ...schedule,
          validFrom: "2026-06-01",
        })),
      },
      closingDayRanges: [
        {
          startDate: "2026-05-11",
          endDate: "2026-05-11",
          reason: "Pädagogischer Tag",
        },
      ],
    });

    await screen.findByText("Regeltermin bearbeiten");
    expect(screen.queryByText(/Schließtag/)).not.toBeInTheDocument();
    await clickSave();

    await waitFor(() => expect(mockUpdateTemplate).toHaveBeenCalled());
    expect(
      screen.queryByRole("dialog", { name: "An einem Schließtag planen?" }),
    ).not.toBeInTheDocument();
  });

  it("warns before a direct Regeltermin edit when single-occurrence edits exist (#1875)", async () => {
    mockCountEditedInWindow.mockResolvedValue({
      count: 2,
      occurrences: [
        {
          instanceId: "101",
          date: "2026-05-11",
          startTime: "15:00:00",
          title: "Yoga",
          changes: ["room"],
        },
        {
          instanceId: "102",
          date: "2026-05-18",
          startTime: "15:00:00",
          title: "Yoga",
          changes: ["staff", "title"],
        },
      ],
    });

    renderModal({ initialSeries: template, defaultDate: "2026-05-04" });

    await screen.findByText("Regeltermin bearbeiten");
    await clickSave();

    // The destructive re-plan is deferred behind the warning, listing the dates.
    expect(
      await screen.findByText("Einzelanpassungen gehen verloren"),
    ).toBeInTheDocument();
    expect(screen.getByText("11.05.2026")).toBeInTheDocument();
    expect(screen.getByText("18.05.2026")).toBeInTheDocument();
    expect(mockUpdateTemplate).not.toHaveBeenCalled();

    // Confirm → the direct series edit proceeds.
    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem fortfahren" }),
    );
    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith("7", expect.anything()),
    );
  });

  it("collapses the remaining day's roster into the visible shared fields", async () => {
    renderModal({
      initialSeries: templateWithWeekdayRosters,
      defaultDate: "2026-05-04",
    });

    await goToStep(2);
    fireEvent.click(screen.getByRole("button", { name: "Di" }));
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          weekdays: [1],
          staff_ids: [11],
          student_ids: [],
          primary_staff_id: 11,
          weekday_assignments: undefined,
        }),
      ),
    );
  });

  it("uses the active weekday when switching back to one shared roster", async () => {
    renderModal({
      initialSeries: templateWithWeekdayRosters,
      defaultDate: "2026-05-04",
    });

    await goToStep(3);
    fireEvent.click(screen.getByRole("button", { name: "Di" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Für alle Tage gleich" }),
    );
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          staff_ids: [12],
          student_ids: [22],
          primary_staff_id: 12,
          weekday_assignments: undefined,
        }),
      ),
    );
  });

  it("adds a target cohort to every per-weekday child roster", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [
        { id: "21", name: "Mara Drei A", school_class: "3a" },
        { id: "22", name: "Mika Drei B", school_class: "3b" },
        { id: "23", name: "Nora Drei C", school_class: "3c" },
      ],
    });
    renderModal({
      initialSeries: {
        ...templateWithWeekdayRosters,
        targetGroupType: "jahrgang",
        targetGradeLevel: 3,
      },
      defaultDate: "2026-05-04",
    });

    await goToStep(3);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Alle 3 Kinder aus Jahrgang 3 übernehmen",
      }),
    );
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          weekday_assignments: [
            expect.objectContaining({
              weekday: 1,
              student_ids: expect.arrayContaining([21, 22, 23]),
            }),
            expect.objectContaining({
              weekday: 2,
              student_ids: expect.arrayContaining([21, 22, 23]),
            }),
          ],
        }),
      ),
    );
  });

  it("cancels a direct Regeltermin edit and writes nothing (#1875)", async () => {
    mockCountEditedInWindow.mockResolvedValue({
      count: 1,
      occurrences: [
        {
          instanceId: "101",
          date: "2026-05-11",
          startTime: "15:00:00",
          title: "Yoga",
          changes: ["room"],
        },
      ],
    });

    renderModal({ initialSeries: template, defaultDate: "2026-05-04" });

    await screen.findByText("Regeltermin bearbeiten");
    await clickSave();
    await screen.findByText("Einzelanpassungen gehen verloren");

    const warningDialog = screen.getByRole("dialog", {
      name: "Einzelanpassungen gehen verloren",
    });
    fireEvent.click(
      within(warningDialog).getByRole("button", { name: "Abbrechen" }),
    );

    await waitFor(() =>
      expect(
        screen.queryByText("Einzelanpassungen gehen verloren"),
      ).not.toBeInTheDocument(),
    );
    expect(mockUpdateTemplate).not.toHaveBeenCalled();
    expect(mockReplanWeek).not.toHaveBeenCalled();
  });

  it("updates an existing series and converts an instance to a series", async () => {
    // Freeze the calendar day so the "replan from today" window is stable.
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));

    const { onSaved: onSeriesSaved } = renderModal({
      initialSeries: template,
      defaultDate: "2026-05-04",
    });

    await screen.findByText("Regeltermin bearbeiten");
    expect(
      screen.getByText("Änderungen gelten für alle Termine dieser Serie."),
    ).toBeInTheDocument();
    await clickSave();
    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ name: "Yoga", room_id: 3 }),
      ),
    );
    // Template edits rebuild future planned instances: chunked scoped
    // re-plan from today (2026-05-04) through the period end (2026-12-31).
    await waitFor(() => expect(mockReplanWeek).toHaveBeenCalledTimes(5));
    expect(mockReplanWeek).toHaveBeenNthCalledWith(
      1,
      "2026-05-04",
      "2026-06-28",
      "7",
    );
    expect(onSeriesSaved).toHaveBeenCalledWith({
      kind: "series",
      seriesId: "7",
    });

    cleanup();
    vi.clearAllMocks();
    setupRefs();
    mockConvertInstanceToSeries.mockResolvedValue({
      templateId: "8",
      timeframeId: "1",
      scheduleIds: ["1"],
      linkedInstanceId: "42",
    });
    mockUpdate.mockResolvedValue(savedInstance);
    const { onSaved: onConvertSaved } = renderModal({
      convertInstance: { ...savedInstance, activityGroupId: undefined },
    });

    await screen.findByText("Termin wiederholen");
    await clickSave();
    // #2135: the repeated instance's date (2026-05-04) is the series start.
    await waitFor(() =>
      expect(mockConvertInstanceToSeries).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ start_date: "2026-05-04" }),
      ),
    );
    expect(mockCreateTemplate).not.toHaveBeenCalled();
    expect(mockUpdate).not.toHaveBeenCalled();
    // Conversion materializes the whole period in 56-day chunks — the
    // series' own valid_from skips its dates before the start, while the
    // tenant-wide run keeps backfilling earlier-starting templates.
    await waitFor(() => expect(mockMaterialize).toHaveBeenCalledTimes(5));
    expect(mockMaterialize).toHaveBeenNthCalledWith(
      1,
      "2026-05-01",
      "2026-06-25",
    );
    expect(onConvertSaved).toHaveBeenCalledWith({
      kind: "series",
      seriesId: "8",
      linkedInstanceId: "42",
    });
  });

  it("sends the series staffing override through the atomic conversion", async () => {
    mockConvertInstanceToSeries.mockResolvedValue({
      templateId: "8",
      timeframeId: "1",
      scheduleIds: ["1"],
      linkedInstanceId: "42",
    });
    renderModal({
      convertInstance: { ...savedInstance, activityGroupId: undefined },
    });

    await screen.findByText("Termin wiederholen");
    await goToStep(3);
    fireEvent.change(screen.getByLabelText("Benötigtes Personal"), {
      target: { value: "4" },
    });
    await clickSave();

    await waitFor(() =>
      expect(mockConvertInstanceToSeries).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ required_staff: 4 }),
      ),
    );
    expect(mockUpdate).not.toHaveBeenCalled();
  });

  it("deletes an existing series from an editable effective date", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-06T10:00:00"));
    const onDeleteSeries = vi.fn().mockResolvedValue(undefined);
    const { onClose } = renderModal({
      initialSeries: template,
      onDeleteSeries,
    });

    await screen.findByText("Regeltermin bearbeiten");
    fireEvent.click(screen.getByRole("button", { name: "Löschen" }));

    const dialog = screen.getByRole("dialog", {
      name: "Regeltermin löschen?",
    });
    const dateInput = within(dialog).getByLabelText(/Ab Datum/);
    expect(dateInput).toHaveValue("2026-05-06");

    fireEvent.change(dateInput, { target: { value: "" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Bitte ein Datum auswählen.",
    );
    expect(onDeleteSeries).not.toHaveBeenCalled();

    fireEvent.change(dateInput, { target: { value: "2026-05-05" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Das Datum darf nicht in der Vergangenheit liegen.",
    );
    expect(onDeleteSeries).not.toHaveBeenCalled();

    fireEvent.change(dateInput, { target: { value: "2026-05-07" } });
    fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));

    await waitFor(() =>
      expect(onDeleteSeries).toHaveBeenCalledWith(
        expect.objectContaining({ id: "7" }),
        "2026-05-07",
      ),
    );
    expect(onClose).toHaveBeenCalledOnce();
  });

  it("does not show the series delete action while creating", async () => {
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(screen.queryByRole("button", { name: "Löschen" })).toBeNull();
  });

  it("surfaces save failures as validation errors", async () => {
    mockCreate.mockRejectedValueOnce(new Error("Backend sagt nein"));
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await clickSave();

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Backend sagt nein",
    );
    expect(mockToastError).toHaveBeenCalledWith("Backend sagt nein");
  });

  it("quick variant renders only the quick fields with prefilled times", async () => {
    renderModal({
      variant: "quick",
      defaultStartTime: "08:00",
      defaultEndTime: "09:30",
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(screen.getByLabelText("Start*")).toHaveValue("08:00");
    expect(screen.getByLabelText("Ende*")).toHaveValue("09:30");
    // Full-form controls stay hidden while collapsed.
    expect(screen.queryByRole("tab", { name: "Jede Woche" })).toBeNull();
    expect(screen.queryByLabelText("Kategorie*")).toBeNull();
    expect(screen.queryByText("Betreuung")).toBeNull();
    expect(screen.queryByText("Personal")).toBeNull();
    expect(screen.queryByText("Kinder")).toBeNull();
    // Die Tagesnotiz steht im Wizard direkt in Schritt 1; früher lag sie
    // hinter "Weitere Optionen".
    expect(screen.getByLabelText("Tagesnotiz")).toBeInTheDocument();

    // Die Wiederholung ist Schritt 2 — im quick-Modus weiterhin der
    // Klartext-Preset statt der vollen Serien-Controls.
    await goToStep(2);
    expect(screen.getByLabelText("Wiederholt sich")).toBeInTheDocument();
    // 2026-05-04 is a Monday — the dynamic weekly option names the weekday.
    fireEvent.click(screen.getByLabelText("Wiederholt sich"));
    expect(
      screen.getByRole("option", { name: "Wöchentlich am Montag" }),
    ).toBeInTheDocument();

    // Personal und Kinder sind Schritt 3 — früher die Disclosure-Inhalte.
    await goToStep(3);
    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.getByText("Kinder")).toBeInTheDocument();
  });

  it("quick preset 'Jeden Wochentag' creates a Mo-Fr weekly series", async () => {
    renderModal({ variant: "quick" });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Wiederholt sich"),
      "Jeden Wochentag (Mo–Fr)",
    );
    await clickSave();

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Mensa",
          weekdays: [1, 2, 3, 4, 5],
          week_pattern: 0,
          calendar_period_id: 5,
          // #2135: the picked Datum is the series start; the materialize
          // window still spans the period for tenant-wide backfill.
          start_date: "2026-05-04",
          materialize_from: "2026-05-01",
          materialize_to: "2026-06-25",
        }),
      ),
    );
  });

  it("quick preset 'Benutzerdefiniert' expands the full form", async () => {
    renderModal({ variant: "quick" });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Wiederholt sich"),
      "Benutzerdefiniert …",
    );

    expect(
      await screen.findByRole("tab", { name: "Jede Woche" }),
    ).toBeInTheDocument();
    await goToStep(3);
    expect(screen.getByText("Personal")).toBeInTheDocument();
    await goToStep(2);
    expect(screen.queryByLabelText("Wiederholt sich")).toBeNull();
  });

  it("auto-expands when a hidden field fails validation", async () => {
    // No categories — the hidden Kategorie field cannot auto-default.
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce({
          json: async () => ({
            data: [{ id: 3, name: "Mensa", building: "Haus A" }],
          }),
        })
        .mockResolvedValueOnce({
          json: async () => ({ data: [] }),
        })
        .mockResolvedValueOnce({
          json: async () => ({ data: [] }),
        }),
    );
    renderModal({ variant: "quick" });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Wiederholt sich"),
      "Jeden Wochentag (Mo–Fr)",
    );
    // Die Wiederholung macht Kategorie (Schritt 1) nachträglich zur Pflicht,
    // deshalb blockt schon "Weiter" — Schritt 3 ist gar nicht mehr erreichbar.
    // Gleicher Fehlertext, gleiche "nichts geschrieben"-Garantie.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Bitte eine Kategorie auswählen."),
    ).toBeInTheDocument();
    // Die fehlgeschlagene Validierung expandiert das Formular und springt auf
    // den Fehler-Schritt: das im quick-Modus versteckte Kategorie-Feld ist samt
    // Inline-Fehler sichtbar. (Früher belegte der "Jede Woche"-Tab dasselbe;
    // er liegt jetzt in Schritt 2 und ist hinter dem Fehler nicht erreichbar.)
    expect(screen.getByLabelText("Kategorie*")).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
  });

  it("chooses the scope before showing fields for a series occurrence", async () => {
    mockGetTemplate.mockResolvedValue({
      ...templateWithWeekdayRosters,
      schedules: [
        { ...templateWithWeekdayRosters.schedules[0]!, weekday: 1 },
        {
          ...templateWithWeekdayRosters.schedules[0]!,
          id: "10",
          weekday: 2,
        },
      ],
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    expect(
      await screen.findByText("Wiederholenden Termin ändern"),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Titel*")).toBeNull();

    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() =>
      expect(screen.getByLabelText("Titel*")).toHaveValue("Yoga"),
    );
    await goToStep(2);
    expect(screen.getByRole("tab", { name: "Jede Woche" })).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(screen.getByRole("button", { name: "Mo" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "Di" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
  });

  it("loads the effective series part before a following edit", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [
        { id: "21", name: "Max Kind" },
        { id: "22", name: "Mila Kind" },
      ],
    });
    mockGetAllStaff.mockResolvedValue([
      { id: "11", name: "Ada Staff" },
      { id: "12", name: "Bea Staff" },
    ]);
    mockGetTemplate.mockResolvedValue(templateWithWeekdayRosters);
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() =>
      expect(screen.getByLabelText("Titel*")).toHaveValue("Yoga"),
    );
    await goToStep(2);
    expect(screen.getByRole("button", { name: "Mo" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByRole("button", { name: "Di" })).toHaveAttribute(
      "aria-pressed",
      "true",
    );
    expect(screen.getByText("Schuljahr 2026/2027")).toBeInTheDocument();
    await goToStep(3);
    expect(
      screen.getByRole("button", { name: "Pro Wochentag" }),
    ).toHaveAttribute("aria-pressed", "true");
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    expect(screen.getByRole("checkbox", { name: /Ada Staff/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Max Kind/ })).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Di" }));
    expect(screen.getByRole("checkbox", { name: /Bea Staff/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Mila Kind/ })).toBeChecked();
  });

  it("asks for the scope when editing a series instance and applies 'Nur diese Woche'", async () => {
    const { onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    expect(
      await screen.findByText("Wiederholenden Termin ändern"),
    ).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
    // The scope copy explains the effect, not the split mechanism.
    expect(
      screen.getByText("Lädt den ab 04.05.2026 wirksamen Serienteil."),
    ).toBeInTheDocument();
    // Neither Datum nor Notiz changed — no single-scope hint.
    expect(
      screen.queryByText(/Geändertes Datum und Notiz/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));
    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(screen.getByLabelText("Titel*")).toHaveValue("Mensa");
    await clickSave();

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ title: "Mensa", activity_group_id: 7 }),
      ),
    );
    expect(mockSplitTemplate).not.toHaveBeenCalled();
    expect(mockUpdateTemplate).not.toHaveBeenCalled();
    expect(onSaved).toHaveBeenCalledWith({
      kind: "instance",
      instance: savedInstance,
    });
  });

  it("splits the series for 'Ab jetzt dauerhaft'", async () => {
    const { onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          effective_date: "2026-05-04",
          materialize_from: "2026-05-04",
          materialize_to: "2026-06-28",
          name: "Yoga",
          // Weekdays/category/period come from the fetched template.
          weekdays: [1],
          category_id: 2,
          week_pattern: 0,
          calendar_period_id: 5,
        }),
      ),
    );
    // The remaining period windows are re-planned scoped to the old template.
    await waitFor(() => expect(mockReplanWeek).toHaveBeenCalledTimes(4));
    expect(mockReplanWeek).toHaveBeenNthCalledWith(
      1,
      "2026-06-29",
      "2026-08-23",
      "7",
    );
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "12" });
  });

  it.each([
    { scope: "Ab jetzt dauerhaft", write: "split" },
    { scope: "Alle Termine der Serie", write: "update" },
  ])(
    "warns when '$scope' replans a later occurrence on a closing day",
    async ({ scope, write }) => {
      vi.useFakeTimers({ toFake: ["Date"] });
      vi.setSystemTime(new Date("2026-05-04T10:00:00"));
      renderModal({
        initialInstance: { ...savedInstance, activityGroupId: "7" },
        closingDayRanges: [
          {
            startDate: "2026-05-11",
            endDate: "2026-05-11",
            reason: "Konzeptionstag",
          },
        ],
      });

      await screen.findByText("Wiederholenden Termin ändern");

      fireEvent.click(screen.getByRole("button", { name: new RegExp(scope) }));

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

      await clickSave();

      const dialog = await screen.findByRole("dialog", {
        name: "An einem Schließtag planen?",
      });
      expect(within(dialog).getByText(/11\.05\.2026/)).toBeInTheDocument();
      expect(mockSplitTemplate).not.toHaveBeenCalled();
      expect(mockUpdateTemplate).not.toHaveBeenCalled();

      fireEvent.click(
        within(dialog).getByRole("button", { name: "Trotzdem planen" }),
      );
      if (write === "split") {
        await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
      } else {
        await waitFor(() => expect(mockUpdateTemplate).toHaveBeenCalled());
      }
    },
  );

  it("warns before 'Alle Termine' when single-occurrence edits would be lost (#1875)", async () => {
    mockCountEditedInWindow.mockResolvedValue({
      count: 2,
      occurrences: [
        {
          instanceId: "101",
          date: "2026-05-11",
          startTime: "12:00:00",
          title: "Mensa",
          changes: ["room", "title"],
        },
        {
          instanceId: "102",
          date: "2026-05-18",
          startTime: "12:00:00",
          title: "Mensa",
          changes: ["staff"],
        },
      ],
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    // Warning lists the concrete affected dates; the series is NOT written yet.
    expect(
      await screen.findByText("Einzelanpassungen gehen verloren"),
    ).toBeInTheDocument();
    expect(screen.getByText("11.05.2026")).toBeInTheDocument();
    expect(screen.getByText("18.05.2026")).toBeInTheDocument();
    expect(mockUpdateTemplate).not.toHaveBeenCalled();

    // Confirm → the series edit proceeds.
    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem fortfahren" }),
    );
    await waitFor(() => expect(mockUpdateTemplate).toHaveBeenCalled());
  });

  it("warns before 'Ab jetzt dauerhaft' and proceeds on confirm (#1875)", async () => {
    mockCountEditedInWindow.mockResolvedValue({
      count: 1,
      occurrences: [
        {
          instanceId: "101",
          date: "2026-05-11",
          startTime: "12:00:00",
          title: "Mensa",
          changes: ["time"],
        },
      ],
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    expect(
      await screen.findByText("Einzelanpassungen gehen verloren"),
    ).toBeInTheDocument();
    expect(mockSplitTemplate).not.toHaveBeenCalled();

    fireEvent.click(
      screen.getByRole("button", { name: "Trotzdem fortfahren" }),
    );
    await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
  });

  it("probes deletions and warns for a resurrected occurrence on 'Ab jetzt dauerhaft' (#1907)", async () => {
    mockCountEditedInWindow.mockResolvedValue({
      count: 1,
      occurrences: [
        {
          instanceId: "0",
          date: "2026-05-11",
          startTime: "",
          title: "Mensa",
          changes: ["deleted"],
        },
      ],
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    // The split path must probe WITH deletions and surface the deleted date.
    await waitFor(() =>
      expect(mockCountEditedInWindow).toHaveBeenCalledWith(
        "7",
        expect.any(String),
        expect.any(String),
        true,
      ),
    );
    expect(
      await screen.findByText("Einzelanpassungen gehen verloren"),
    ).toBeInTheDocument();
    expect(screen.getByText("11.05.2026")).toBeInTheDocument();
    expect(screen.getByText("Gelöschter Termin")).toBeInTheDocument();
  });

  it("does not probe deletions on the 'Alle Termine' path (#1907)", async () => {
    mockCountEditedInWindow.mockResolvedValue({ count: 0, occurrences: [] });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await waitFor(() =>
      expect(mockCountEditedInWindow).toHaveBeenCalledWith(
        "7",
        expect.any(String),
        expect.any(String),
        false,
      ),
    );
  });

  it("cancels the series edit and keeps the single-occurrence edits (#1875)", async () => {
    mockCountEditedInWindow.mockResolvedValue({
      count: 1,
      occurrences: [
        {
          instanceId: "101",
          date: "2026-05-11",
          startTime: "12:00:00",
          title: "Mensa",
          changes: ["room"],
        },
      ],
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await screen.findByText("Einzelanpassungen gehen verloren");

    // The slide-over footer also has an "Abbrechen"; scope to the warning dialog.
    const warningDialog = screen.getByRole("dialog", {
      name: "Einzelanpassungen gehen verloren",
    });
    fireEvent.click(
      within(warningDialog).getByRole("button", { name: "Abbrechen" }),
    );

    await waitFor(() =>
      expect(
        screen.queryByText("Einzelanpassungen gehen verloren"),
      ).not.toBeInTheDocument(),
    );
    expect(mockUpdateTemplate).not.toHaveBeenCalled();
    expect(mockSplitTemplate).not.toHaveBeenCalled();
  });

  it("continues the series save when the probe reports no lost edits for status-day-only absences (#2225)", async () => {
    mockCountEditedInWindow.mockResolvedValue({ count: 0, occurrences: [] });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    // The backend returns zero when attendance changed only because of a
    // planned sick/excused/class-trip day. No warning modal may interrupt the
    // series-scope flow; the edit goes straight through.
    await waitFor(() => expect(mockUpdateTemplate).toHaveBeenCalled());
    expect(
      screen.queryByText("Einzelanpassungen gehen verloren"),
    ).not.toBeInTheDocument();
  });

  it("preserves the fetched staffing override for an untouched following edit", async () => {
    mockGetTemplate.mockResolvedValue({
      ...template,
      requiredStaffOverride: 3,
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    expect(screen.getByLabelText("Benötigtes Personal")).toHaveValue(3);
    await clickSave();

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ required_staff: 3 }),
      ),
    );
  });

  it("does not promote an occurrence substitute during an untouched following roster edit", async () => {
    mockGetAllStaff.mockResolvedValue([
      { id: "11", name: "Ada Staff" },
      { id: "31", name: "QA Substitute" },
    ]);
    mockGetTemplate.mockResolvedValue({
      ...template,
      staffIds: ["11"],
      primaryStaffId: "11",
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: true,
            isSubstitute: false,
          },
          {
            staffId: "31",
            isPrimary: false,
            isAbsent: false,
            isSubstitute: true,
          },
        ],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText("QA Substitute");
    await goToStep(1);
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa umbenannt" },
    });
    await clickSave();

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          name: "Mensa umbenannt",
          staff_ids: [11],
          primary_staff_id: 11,
        }),
      ),
    );
  });

  it.each([
    {
      scope: "Ab jetzt dauerhaft",
      buttonName: /Ab jetzt dauerhaft/,
      expectedWrite: "split",
    },
    {
      scope: "Alle Termine der Serie",
      buttonName: /Alle Termine der Serie/,
      expectedWrite: "update",
    },
  ])(
    "checks weekday-specific staff separately for '$scope'",
    async ({ buttonName, expectedWrite }) => {
      vi.useFakeTimers({ toFake: ["Date"] });
      vi.setSystemTime(new Date("2026-05-04T10:00:00"));
      const shortPeriod: CalendarPeriod = {
        ...periods[0]!,
        startDate: "2026-05-04",
        endDate: "2026-05-15",
      };
      mockGetTemplate.mockResolvedValue(templateWithWeekdayRosters);
      renderModal({
        initialInstance: {
          ...savedInstance,
          activityGroupId: "7",
          staff: [
            {
              staffId: "11",
              isPrimary: true,
              isAbsent: false,
              isSubstitute: false,
            },
          ],
        },
        calendarPeriods: [shortPeriod],
      });

      await screen.findByText("Wiederholenden Termin ändern");
      mockCheckShiftCoverage.mockClear();
      fireEvent.click(screen.getByRole("button", { name: buttonName }));

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await clickSave();

      await waitFor(() => {
        const scopeProbes = mockCheckShiftCoverage.mock.calls
          .map(([probe]) => probe as ShiftCoverageCheckParams)
          .filter((probe) => probe.calendarPeriodId === "5");
        expect(scopeProbes).toEqual([
          expect.objectContaining({
            dates: ["2026-05-04", "2026-05-11"],
            staffIds: ["11"],
          }),
          expect.objectContaining({
            dates: ["2026-05-05", "2026-05-12"],
            staffIds: ["12"],
          }),
        ]);
      });
      if (expectedWrite === "split") {
        await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
      } else {
        await waitFor(() => expect(mockUpdateTemplate).toHaveBeenCalled());
      }
    },
  );

  it("checks all following occurrences and saves despite coverage warnings", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));
    const shortPeriod: CalendarPeriod = {
      ...periods[0]!,
      startDate: "2026-05-04",
      endDate: "2026-05-15",
    };
    mockGetTemplate.mockResolvedValue({
      ...template,
      weekdayAssignments: [],
      schedules: [
        { ...template.schedules[0]!, weekday: 1 },
        { ...template.schedules[0]!, id: "10", weekday: 3 },
      ],
    });
    mockCheckShiftCoverage.mockImplementation((probe: { dates: string[] }) =>
      Promise.resolve({
        coverageWarnings:
          probe.dates.length > 1
            ? [
                {
                  staffId: "11",
                  staffName: "Ada Staff",
                  date: "2026-05-06",
                  startTime: "12:00",
                  endTime: "13:00",
                  uncoveredStartTime: "12:30",
                  uncoveredEndTime: "13:00",
                  message: "Ada Staff fehlt am Mittwoch von 12:30–13:00.",
                },
              ]
            : [],
      }),
    );
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
      calendarPeriods: [shortPeriod],
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
        expect.objectContaining({
          dates: ["2026-05-04", "2026-05-06", "2026-05-11", "2026-05-13"],
          startTime: "14:00",
          endTime: "15:00",
          staffIds: ["11"],
          calendarPeriodId: "5",
          weekPattern: 0,
        }),
      ),
    );
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Ada Staff fehlt am Mittwoch von 12:30–13:00.",
      { duration: 10_000 },
    );
    await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
  });

  it("caps the coverage probe at the schedules' validUntil for 'Ab jetzt dauerhaft'", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));
    const shortPeriod: CalendarPeriod = {
      ...periods[0]!,
      startDate: "2026-05-04",
      endDate: "2026-05-15",
    };
    mockGetTemplate.mockResolvedValue({
      ...template,
      weekdayAssignments: [],
      schedules: [
        { ...template.schedules[0]!, weekday: 1, validUntil: "2026-05-11" },
        {
          ...template.schedules[0]!,
          id: "10",
          weekday: 3,
          validUntil: "2026-05-11",
        },
      ],
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
      calendarPeriods: [shortPeriod],
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    // validUntil is exclusive: the split successor inherits the boundary,
    // so 2026-05-11 and 2026-05-13 are never created and must not be probed.
    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
        expect.objectContaining({ dates: ["2026-05-04", "2026-05-06"] }),
      ),
    );
    await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
  });

  it("caps the coverage probe at the latest validUntil when schedules diverge", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));
    const shortPeriod: CalendarPeriod = {
      ...periods[0]!,
      startDate: "2026-05-04",
      endDate: "2026-05-15",
    };
    mockGetTemplate.mockResolvedValue({
      ...template,
      weekdayAssignments: [],
      schedules: [
        { ...template.schedules[0]!, weekday: 1, validUntil: "2026-05-11" },
        {
          ...template.schedules[0]!,
          id: "10",
          weekday: 3,
          validUntil: "2026-05-13",
        },
      ],
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
      calendarPeriods: [shortPeriod],
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    // The latest boundary (2026-05-13, exclusive) wins: Monday 2026-05-11
    // stays in the probe even though the Monday schedule ends earlier.
    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
        expect.objectContaining({
          dates: ["2026-05-04", "2026-05-06", "2026-05-11"],
        }),
      ),
    );
    await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
  });

  it("preserves a template-only period pin and its end for 'Ab jetzt dauerhaft'", async () => {
    mockGetTemplate.mockResolvedValue(templateWithTemplateOnlyPeriodPin);
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
      calendarPeriods: [...periods, templatePinnedPeriod],
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          calendar_period_id: 6,
          materialize_to: "2026-06-28",
        }),
      ),
    );
    await waitFor(() => expect(mockReplanWeek).toHaveBeenCalledTimes(1));
    expect(mockReplanWeek).toHaveBeenCalledWith(
      "2026-06-29",
      "2026-07-31",
      "7",
    );
  });

  it("preserves the fetched template roster for 'Ab jetzt dauerhaft' without users:read", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    mockGetTemplate.mockResolvedValue({
      ...template,
      studentIds: ["21", "22"],
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        studentIds: ["31"],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    await clickSave();

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ student_ids: [21, 22] }),
      ),
    );
  });

  it("preserves fetched template staff for 'Ab jetzt dauerhaft' without users:read", async () => {
    mockGetAllStaff.mockRejectedValue(new Error("forbidden"));
    mockGetTemplate.mockResolvedValue({
      ...template,
      staffIds: ["11", "12"],
      primaryStaffId: "12",
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "31",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    await clickSave();

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          staff_ids: [11, 12],
          primary_staff_id: 12,
        }),
      ),
    );
  });

  it("updates the template and replans for 'Alle Termine der Serie'", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));

    const { onSaved } = renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
        expect.objectContaining({ replanActivityGroupId: "7" }),
      ),
    );

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          name: "Yoga",
          room_id: 3,
          weekdays: [1],
          category_id: 2,
        }),
      ),
    );
    await waitFor(() => expect(mockReplanWeek).toHaveBeenCalledTimes(5));
    expect(mockReplanWeek).toHaveBeenNthCalledWith(
      1,
      "2026-05-04",
      "2026-06-28",
      "7",
    );
    expect(mockSplitTemplate).not.toHaveBeenCalled();
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "7" });
  });

  it.each([
    { scope: /Alle Termine der Serie/, write: "update" },
    { scope: /Ab jetzt dauerhaft/, write: "split" },
  ])(
    "applies the selected planning track for $write series edits",
    async ({ scope, write }) => {
      renderModal({
        initialInstance: {
          ...savedInstance,
          activityGroupId: "7",
          planningTrackId: "8",
          planningTrackName: "Mittag",
          planningTrackColor: "#F78C10",
          planningTrackSortOrder: 1,
        },
      });

      await screen.findByText("Wiederholenden Termin ändern");

      fireEvent.click(screen.getByRole("button", { name: scope }));

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await chooseFromSelect(screen.getByLabelText("Planungsspur"), "Mittag");

      await clickSave();

      const expected = expect.objectContaining({ planning_track_id: 8 });
      if (write === "split") {
        await waitFor(() =>
          expect(mockSplitTemplate).toHaveBeenCalledWith("7", expected),
        );
      } else {
        await waitFor(() =>
          expect(mockUpdateTemplate).toHaveBeenCalledWith("7", expected),
        );
      }
    },
  );

  it("sends an explicit null when an all-series edit clears the planning track", async () => {
    const trackedTemplate = {
      ...template,
      planningTrackId: "8",
      planningTrackName: "Mittag",
      planningTrackColor: "#F78C10",
      planningTrackSortOrder: 1,
    };
    mockGetTemplate.mockResolvedValue(trackedTemplate);
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await chooseFromSelect(
      screen.getByLabelText("Planungsspur"),
      "Keine Planungsspur",
    );
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ planning_track_id: null }),
      ),
    );
  });

  it("preserves the fetched staffing override for an untouched all-series edit", async () => {
    mockGetTemplate.mockResolvedValue({
      ...template,
      requiredStaffOverride: 3,
    });
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    expect(screen.getByLabelText("Benötigtes Personal")).toHaveValue(3);
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ required_staff: 3 }),
      ),
    );
  });

  it("clears the template staffing override when the occurrence field was explicitly cleared", async () => {
    mockGetTemplate.mockResolvedValue({
      ...template,
      requiredStaffOverride: 3,
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        requiredStaffOverride: 2,
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    const requiredStaff = screen.getByLabelText("Benötigtes Personal");
    expect(requiredStaff).toHaveValue(3);
    fireEvent.change(requiredStaff, { target: { value: "" } });
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ required_staff: null }),
      ),
    );
  });

  it("preserves a template-only period pin and its end for 'Alle Termine der Serie'", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));
    mockGetTemplate.mockResolvedValue(templateWithTemplateOnlyPeriodPin);
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
      calendarPeriods: [...periods, templatePinnedPeriod],
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ calendar_period_id: 6 }),
      ),
    );
    await waitFor(() => expect(mockReplanWeek).toHaveBeenCalledTimes(2));
    expect(mockReplanWeek).toHaveBeenNthCalledWith(
      2,
      "2026-06-29",
      "2026-07-31",
      "7",
    );
  });

  it("preserves the fetched template roster for 'Alle Termine der Serie' without users:read", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    mockGetTemplate.mockResolvedValue({
      ...template,
      studentIds: ["21", "22"],
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        studentIds: ["31"],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ student_ids: [21, 22] }),
      ),
    );
  });

  it("preserves weekday child rosters after a successful untouched occurrence load", async () => {
    mockGetTemplate.mockResolvedValue(templateWithWeekdayRosters);
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        studentIds: ["31"],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText("Max Kind");
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          student_ids: [21, 22],
          weekday_assignments: [
            {
              weekday: 1,
              staff_ids: [11],
              student_ids: [21],
              primary_staff_id: 11,
            },
            {
              weekday: 2,
              staff_ids: [12],
              student_ids: [22],
              primary_staff_id: 12,
            },
          ],
        }),
      ),
    );
  });

  it("preserves fetched template staff for 'Alle Termine der Serie' without users:read", async () => {
    mockGetAllStaff.mockRejectedValue(new Error("forbidden"));
    mockGetTemplate.mockResolvedValue({
      ...template,
      staffIds: ["11", "12"],
      primaryStaffId: "12",
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "31",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          staff_ids: [11, 12],
          primary_staff_id: 12,
        }),
      ),
    );
  });

  it("keeps the occurrence roster for 'Nur diese Woche' without users:read", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    mockGetTemplate.mockResolvedValue({
      ...template,
      studentIds: ["21", "22"],
    });
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        studentIds: ["31"],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    await clickSave();

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ student_ids: [31] }),
      ),
    );
    expect(mockGetTemplate).not.toHaveBeenCalled();
  });

  it("keeps occurrence staff for 'Nur diese Woche' without users:read", async () => {
    mockGetAllStaff.mockRejectedValue(new Error("forbidden"));
    renderModal({
      initialInstance: {
        ...savedInstance,
        activityGroupId: "7",
        staff: [
          {
            staffId: "31",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

    await screen.findByLabelText("Titel*");

    await goToStep(3);
    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    await clickSave();

    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ staff_ids: [31] }),
      ),
    );
    expect(mockGetTemplate).not.toHaveBeenCalled();
  });

  it("does not ask for a scope when editing a one-off instance", async () => {
    const { onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: undefined },
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(screen.queryByText("Wiederholenden Termin ändern")).toBeNull();
    expect(onSaved).toHaveBeenCalledWith({
      kind: "instance",
      instance: savedInstance,
    });
  });

  it("does not expose recurrence fields before a series scope is chosen", async () => {
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    expect(
      await screen.findByText("Wiederholenden Termin ändern"),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Wiederholt sich")).not.toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  // Same class of bug for a one-off: switching to weekly must convert the
  // existing row (create series + link it), not leave the old Termin behind.
  it("converts a one-off into a series instead of creating a second appointment", async () => {
    mockConvertInstanceToSeries.mockResolvedValue({
      templateId: "9",
      timeframeId: "1",
      scheduleIds: ["1"],
      linkedInstanceId: "42",
    });
    const { onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: undefined },
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    await clickSave();

    await waitFor(() =>
      expect(mockConvertInstanceToSeries).toHaveBeenCalledWith(
        "42",
        expect.objectContaining({ start_date: savedInstance.date }),
      ),
    );
    expect(mockCreateTemplate).not.toHaveBeenCalled();
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(mockCreate).not.toHaveBeenCalled();
    expect(onSaved).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: "series",
        seriesId: "9",
        linkedInstanceId: "42",
      }),
    );
  });

  it("adds a whole class via the bulk select and renders Personal before Kinder", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [
        {
          id: "21",
          name: "Max Kind",
          school_class: "1a",
          group_name: "OGS Blau",
        },
        {
          id: "22",
          name: "Mila Kind",
          school_class: "1a",
          group_name: "OGS Rot",
        },
        {
          id: "23",
          name: "Ben Kind",
          school_class: "2b",
          group_name: "OGS Blau",
        },
      ],
    });
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);

    // Personal renders before Kinder (Streichliste 8).
    const fieldLabels = screen
      .getAllByText(/^(Personal|Kinder)$/)
      .map((node) => node.textContent);
    expect(fieldLabels).toEqual(["Personal", "Kinder"]);
    fireEvent.click(
      screen.getByLabelText("Jahrgang, Klasse oder Gruppe komplett hinzufügen"),
    );
    expect(
      screen.getByRole("option", { name: "Klasse 1a" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Klasse Klasse 1a" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("option", { name: "Klasse 1a" }));
    expect(screen.getByText("2 ausgewählt")).toBeInTheDocument();

    await chooseFromSelect(
      screen.getByLabelText("Jahrgang, Klasse oder Gruppe komplett hinzufügen"),
      "Gruppe OGS Blau",
    );
    // Union: 21 + 22 from class 1a, 23 from group OGS Blau (21 deduplicated).
    expect(screen.getByText("3 ausgewählt")).toBeInTheDocument();
  });

  it("probes conflicts after a room change and keeps Speichern enabled", async () => {
    mockCheckConflicts.mockResolvedValue({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [
        {
          kind: "room",
          resourceId: "3",
          message: "Raum Mensa ist 12:00–13:00 bereits belegt",
          conflictingInstanceId: "99",
          conflictingTitle: "Mensa",
        },
      ],
    });
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");

    // 500ms debounce, then the advisory warning renders above the footer.
    await waitFor(
      () =>
        expect(mockCheckConflicts).toHaveBeenCalledWith(
          expect.objectContaining({
            date: "2026-05-04",
            startTime: "12:00",
            endTime: "13:00",
            roomId: "3",
          }),
        ),
      { timeout: 2000 },
    );
    await goToStep(3);
    expect(
      await screen.findByText(
        "Hinweis: Raum Mensa ist 12:00–13:00 bereits belegt",
      ),
    ).toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("shows conflict hints already on step 1 without blocking the save on the last step", async () => {
    mockCheckConflicts.mockResolvedValue({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [
        {
          kind: "room",
          resourceId: "3",
          message: "Raum Mensa ist 12:00–13:00 bereits belegt",
          conflictingInstanceId: "99",
          conflictingTitle: "Mensa",
        },
      ],
    });
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");

    // Without any step navigation the advisory hint appears on step 1.
    expect(
      await screen.findByText(
        "Hinweis: Raum Mensa ist 12:00–13:00 bereits belegt",
        undefined,
        { timeout: 2000 },
      ),
    ).toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("shows uncovered-shift warnings without blocking save", async () => {
    mockCheckShiftCoverage.mockResolvedValue({
      coverageWarnings: [
        {
          staffId: "11",
          staffName: "Ada Staff",
          date: "2026-05-04",
          startTime: "12:00",
          endTime: "13:00",
          uncoveredStartTime: "12:30",
          uncoveredEndTime: "13:00",
          message:
            "Ada Staff ist für 12:00–13:00 eingeteilt; nicht durch eine Schicht abgedeckt: 12:30–13:00.",
        },
      ],
    });
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));

    expect(
      await screen.findByText(
        "Ada Staff ist für 12:00–13:00 eingeteilt; nicht durch eine Schicht abgedeckt: 12:30–13:00.",
      ),
    ).toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("aggregates large coverage results without creating assertive alerts", async () => {
    const warnings = Array.from({ length: 100 }, (_, index) => ({
      staffId: "11",
      staffName: "Ada Staff",
      date: `2026-05-${String((index % 28) + 1).padStart(2, "0")}`,
      startTime: "12:00",
      endTime: "13:00",
      uncoveredStartTime: "12:30",
      uncoveredEndTime: "13:00",
      message: `Beispiel-Lücke ${index + 1}`,
    }));
    mockCheckShiftCoverage.mockResolvedValue({
      coverageWarnings: warnings,
      coverageWarningCount: 130,
    });
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));

    expect(
      await screen.findByText(
        "130 Dienstplan-Lücken gefunden. Speichern ist weiterhin möglich.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("127 weitere Lücken anzeigen")).toBeInTheDocument();
    expect(screen.getByText("Beispiel-Lücke 1")).toBeInTheDocument();
    await expectSaveEnabled();
    expect(document.querySelectorAll('[aria-live="polite"]')).toHaveLength(1);
    expect(screen.queryAllByRole("alert")).toHaveLength(0);
  });

  it("aborts a hanging one-off pre-save coverage check and still writes", async () => {
    renderModal({
      initialInstance: {
        ...savedInstance,
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });
    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    // #2025: walk to the last step (where Speichern lives) BEFORE the fake
    // timers take over — wizard navigation awaits real-timer queries.
    await goToStep(3);
    mockCheckShiftCoverage.mockClear();
    mockCheckShiftCoverage.mockReturnValueOnce(new Promise(() => undefined));
    vi.useFakeTimers();

    await clickSave();
    await act(async () => vi.advanceTimersByTimeAsync(5_000));

    expect(mockUpdate).toHaveBeenCalled();
  });

  it("aborts a hanging direct-series pre-save coverage check and still writes", async () => {
    renderModal({ initialSeries: template });
    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    // #2025: walk to the last step (where Speichern lives) BEFORE the fake
    // timers take over — wizard navigation awaits real-timer queries.
    await goToStep(3);
    mockCheckShiftCoverage.mockClear();
    mockCheckShiftCoverage.mockReturnValueOnce(new Promise(() => undefined));
    vi.useFakeTimers();

    await clickSave();
    await act(async () => vi.advanceTimersByTimeAsync(5_000));

    expect(mockUpdateTemplate).toHaveBeenCalledWith(
      "7",
      expect.objectContaining({ week_pattern: 0 }),
    );
  });

  it("skips coverage entirely when the caller lacks the combined capability", async () => {
    renderModal({
      canCheckShiftCoverage: false,
      initialInstance: {
        ...savedInstance,
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });
    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    mockCheckShiftCoverage.mockClear();

    await clickSave();

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(mockCheckShiftCoverage).not.toHaveBeenCalled();
    expect(
      screen.queryByText(/Dienstplan-Abdeckung konnte nicht geprüft werden/),
    ).not.toBeInTheDocument();
  });

  it("checks the latest assignment before a fast save and continues after warnings", async () => {
    const coverage = deferred<{
      coverageWarnings: Array<{
        staffId: string;
        staffName: string;
        date: string;
        startTime: string;
        endTime: string;
        uncoveredStartTime: string;
        uncoveredEndTime: string;
        message: string;
      }>;
    }>();
    renderModal({
      initialInstance: {
        ...savedInstance,
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    mockCheckShiftCoverage.mockClear();
    mockCheckShiftCoverage.mockReturnValueOnce(coverage.promise);
    fireEvent.change(screen.getByLabelText("Ende*"), {
      target: { value: "13:15" },
    });
    await clickSave();

    await waitFor(() => expect(mockCheckShiftCoverage).toHaveBeenCalled());
    expect(mockUpdate).not.toHaveBeenCalled();
    await act(async () => {
      coverage.resolve({
        coverageWarnings: [
          {
            staffId: "11",
            staffName: "Ada Staff",
            date: "2026-05-04",
            startTime: "12:00",
            endTime: "13:15",
            uncoveredStartTime: "13:00",
            uncoveredEndTime: "13:15",
            message: "Ada Staff fehlt von 13:00–13:15.",
          },
        ],
      });
    });

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Ada Staff fehlt von 13:00–13:15.",
      { duration: 10_000 },
    );
  });

  it("checks every selected series weekday instead of the arbitrary anchor date", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-02T10:00:00"));
    const shortPeriod: CalendarPeriod = {
      ...periods[0]!,
      startDate: "2026-05-04",
      endDate: "2026-05-15",
    };
    const mondayWednesday: TimetableTemplate = {
      ...template,
      weekdayAssignments: [],
      schedules: [
        { ...template.schedules[0]!, weekday: 1 },
        { ...template.schedules[0]!, id: "10", weekday: 3 },
      ],
    };

    renderModal({
      defaultDate: "2026-05-05",
      initialSeries: mondayWednesday,
      calendarPeriods: [shortPeriod],
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await waitFor(
      () =>
        expect(mockCheckShiftCoverage).toHaveBeenCalledWith({
          dates: ["2026-05-04", "2026-05-06", "2026-05-11", "2026-05-13"],
          startTime: "14:00",
          endTime: "15:00",
          staffIds: ["11"],
          replanActivityGroupId: "7",
          calendarPeriodId: "5",
          weekPattern: 0,
        }),
      { timeout: 2000 },
    );
    expect(mockCheckShiftCoverage).not.toHaveBeenCalledWith(
      expect.objectContaining({
        dates: expect.arrayContaining(["2026-05-05"]),
      }),
    );
  });

  it("preserves an A-week series in both coverage and direct saves", async () => {
    const aWeekTemplate: TimetableTemplate = {
      ...template,
      schedules: template.schedules.map((schedule) => ({
        ...schedule,
        weekPattern: 1,
      })),
    };
    renderModal({ initialSeries: aWeekTemplate });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
        expect.objectContaining({ weekPattern: 1 }),
      ),
    );
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ week_pattern: 1 }),
      ),
    );
  });

  it("reads an A-week series back as biweekly with Woche A selected", async () => {
    const aWeekTemplate: TimetableTemplate = {
      ...template,
      schedules: template.schedules.map((schedule) => ({
        ...schedule,
        weekPattern: 1,
      })),
    };
    renderModal({ initialSeries: aWeekTemplate });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    expect(screen.getByRole("tab", { name: "Alle 2 Wochen" })).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(screen.getByRole("tab", { name: "Woche A" })).toHaveAttribute(
      "data-state",
      "active",
    );
  });

  it("moves a legacy Saturday to Monday even when another workday remains", async () => {
    const fridaySaturdayTemplate: TimetableTemplate = {
      ...template,
      weekdayAssignments: [],
      schedules: [
        { ...template.schedules[0]!, weekday: 5 },
        { ...template.schedules[0]!, id: "10", weekday: 6 },
      ],
    };
    renderModal({ initialSeries: fridaySaturdayTemplate });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Alten Wochenendtag Sa zu Montag ändern",
      }),
    );
    await clickSave();

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ weekdays: [1, 5] }),
      ),
    );
  });

  it("defaults a new biweekly series to the parity of the selected week", async () => {
    const cyclePeriod: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      weekCycleLength: 2,
      weekCycleAnchor: "2026-05-04",
    };
    renderModal({
      calendarPeriods: [cyclePeriod],
      defaultCalendarPeriodId: "9",
      defaultDate: "2026-05-11",
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    expect(screen.queryByRole("tab", { name: "Woche A" })).toBeNull();
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Alle 2 Wochen" }), {
      button: 0,
    });

    expect(screen.getByRole("tab", { name: "Woche B" })).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(
      screen.getByText("Woche vom 11.05.2026 ist Woche B"),
    ).toBeInTheDocument();
  });

  it.each([
    {
      name: "a one-week cycle",
      weekCycleLength: 1,
      weekCycleAnchor: "2026-05-04",
    },
    {
      name: "a cycle longer than two weeks",
      weekCycleLength: 3,
      weekCycleAnchor: "2026-05-04",
    },
    {
      name: "a two-week cycle without an anchor",
      weekCycleLength: 2,
      weekCycleAnchor: null,
    },
  ])(
    "disables 'Alle 2 Wochen' for new series in $name",
    async ({ weekCycleLength, weekCycleAnchor }) => {
      const threeWeekCycle: CalendarPeriod = {
        ...periods[0]!,
        id: "9",
        weekCycleLength,
        weekCycleAnchor,
      };
      renderModal({
        calendarPeriods: [threeWeekCycle],
        defaultCalendarPeriodId: "9",
        // 2026-05-04 is Woche A: the guard depends on the cycle length alone,
        // not on which week the modal was opened from.
        defaultDate: "2026-05-04",
      });

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(2);
      const biweeklyTab = screen.getByRole("tab", { name: "Alle 2 Wochen" });
      expect(biweeklyTab).toBeDisabled();
      fireEvent.mouseDown(biweeklyTab, { button: 0 });
      expect(biweeklyTab).toHaveAttribute("data-state", "inactive");
    },
  );

  it("keeps a stored biweekly series editable in cycles longer than two weeks", async () => {
    const threeWeekCycle: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      weekCycleLength: 3,
      weekCycleAnchor: "2026-05-04",
    };
    const aWeekTemplate: TimetableTemplate = {
      ...template,
      calendarPeriodId: "9",
      schedules: template.schedules.map((schedule) => ({
        ...schedule,
        weekPattern: 1,
        calendarPeriodId: "9",
      })),
    };
    renderModal({
      calendarPeriods: [threeWeekCycle],
      defaultCalendarPeriodId: "9",
      initialSeries: aWeekTemplate,
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    const biweeklyTab = screen.getByRole("tab", { name: "Alle 2 Wochen" });
    expect(biweeklyTab).not.toBeDisabled();
    expect(biweeklyTab).toHaveAttribute("data-state", "active");

    await clickSave();
    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ week_pattern: 1 }),
      ),
    );
  });

  it("rejects saving a biweekly series after switching to a longer-cycle period", async () => {
    const twoWeekCycle: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      name: "Zwei-Wochen-Zyklus",
      weekCycleLength: 2,
      weekCycleAnchor: "2026-05-04",
    };
    const threeWeekCycle: CalendarPeriod = {
      ...periods[0]!,
      id: "10",
      name: "Drei-Wochen-Zyklus",
      weekCycleLength: 3,
      weekCycleAnchor: "2026-05-04",
    };
    renderModal({
      calendarPeriods: [twoWeekCycle, threeWeekCycle],
      defaultCalendarPeriodId: "9",
      defaultDate: "2026-05-11",
      showPeriodField: true,
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Alle 2 Wochen" }), {
      button: 0,
    });
    await goToStep(1);
    await chooseFromSelect(screen.getByLabelText("Kategorie*"), "AG");
    await goToStep(2);
    // The tab was enabled under the two-week cycle; switching the period
    // afterwards must still be caught by validation.
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Drei-Wochen-Zyklus",
    );

    // #2025: the offending field lives on this step, so "Weiter" is what the
    // user hits first — same error text, still nothing written.
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    expect(
      await screen.findByText(
        'Der gewählte Planungszeitraum hat keinen verankerten Zwei-Wochen-Zyklus. Eine 14-tägige Wiederholung ist hier nicht möglich; bitte "Jede Woche" wählen.',
      ),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();

    // Switching back to the two-week cycle clears the error and saves.
    await chooseFromSelect(
      screen.getByLabelText("Planungszeitraum*"),
      "Zwei-Wochen-Zyklus",
    );
    expect(
      screen.queryByText(
        'Der gewählte Planungszeitraum hat keinen verankerten Zwei-Wochen-Zyklus. Eine 14-tägige Wiederholung ist hier nicht möglich; bitte "Jede Woche" wählen.',
      ),
    ).not.toBeInTheDocument();
    await clickSave();
    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({ week_pattern: 2, calendar_period_id: 9 }),
      ),
    );
  });

  it("keeps a manual Woche-A choice when the date moves to a B week", async () => {
    const cyclePeriod: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      weekCycleLength: 2,
      weekCycleAnchor: "2026-05-04",
    };
    renderModal({
      calendarPeriods: [cyclePeriod],
      defaultCalendarPeriodId: "9",
      defaultDate: "2026-05-11",
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Alle 2 Wochen" }), {
      button: 0,
    });
    expect(screen.getByRole("tab", { name: "Woche B" })).toHaveAttribute(
      "data-state",
      "active",
    );

    // Explicit switch to A, then a date change into another B week: the
    // choice must stick while the hint reflects the new week's parity.
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Woche A" }), {
      button: 0,
    });
    await goToStep(1);
    fireEvent.change(screen.getByLabelText(/Datum/), {
      target: { value: "2026-05-25" },
    });
    await goToStep(2);

    expect(screen.getByRole("tab", { name: "Woche A" })).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(
      screen.getByText("Woche vom 25.05.2026 ist Woche B"),
    ).toBeInTheDocument();
  });

  it("preserves a manual Woche-A choice across repeat-mode switches", async () => {
    const cyclePeriod: CalendarPeriod = {
      ...periods[0]!,
      id: "9",
      weekCycleLength: 2,
      weekCycleAnchor: "2026-05-04",
    };
    renderModal({
      calendarPeriods: [cyclePeriod],
      defaultCalendarPeriodId: "9",
      defaultDate: "2026-05-11",
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Alle 2 Wochen" }), {
      button: 0,
    });
    // The selected 2026-05-11 week defaults to B; pick A manually.
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Woche A" }), {
      button: 0,
    });

    // Leaving biweekly mode and coming back must restore the manual pick,
    // not re-derive the B default from the date parity.
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    expect(screen.queryByRole("tab", { name: "Woche A" })).toBeNull();
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Alle 2 Wochen" }), {
      button: 0,
    });

    expect(screen.getByRole("tab", { name: "Woche A" })).toHaveAttribute(
      "data-state",
      "active",
    );
    expect(
      screen.getByText("Woche vom 11.05.2026 ist Woche B"),
    ).toBeInTheDocument();
  });

  it("shows a non-blocking warning when shift coverage cannot be checked", async () => {
    mockCheckShiftCoverage.mockRejectedValue(new Error("probe down"));
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(3);
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));

    await waitFor(() => expect(mockCheckShiftCoverage).toHaveBeenCalled(), {
      timeout: 2000,
    });
    expect(
      await screen.findByText(
        "Hinweis: Die Dienstplan-Abdeckung konnte nicht geprüft werden. Speichern ist weiterhin möglich.",
      ),
    ).toBeInTheDocument();
    await expectSaveEnabled();
  });

  it("still saves when the final coverage check fails", async () => {
    mockCheckShiftCoverage.mockRejectedValue(new Error("probe down"));
    renderModal({
      initialInstance: {
        ...savedInstance,
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Die Dienstplan-Abdeckung konnte nicht geprüft werden. Speichern ist weiterhin möglich.",
      { duration: 10_000 },
    );
  });

  it("uses the moved converted instance's effective roster without a replan group", async () => {
    renderModal({
      convertInstance: {
        ...savedInstance,
        staff: [
          {
            staffId: "11",
            isPrimary: true,
            isAbsent: false,
            isSubstitute: false,
          },
        ],
      },
    });

    await goToStep(1);
    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Datum*"), {
      target: { value: "2026-05-05" },
    });
    await waitFor(
      () =>
        expect(mockCheckConflicts).toHaveBeenCalledWith(
          expect.objectContaining({
            roomId: "3",
            excludeInstanceId: "42",
          }),
        ),
      { timeout: 2000 },
    );
    await waitFor(
      () =>
        expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
          expect.objectContaining({
            dates: expect.arrayContaining(["2026-05-05"]),
            excludeInstanceId: "42",
            concreteInstanceDate: "2026-05-05",
          }),
        ),
      { timeout: 2000 },
    );
    const convertCoverageProbe = mockCheckShiftCoverage.mock.calls.find(
      ([probe]) => probe.concreteInstanceDate === "2026-05-05",
    )?.[0];
    expect(convertCoverageProbe).not.toHaveProperty("replanActivityGroupId");
  });

  it("skips the stale conflict probe when the modal reopens mid-debounce", async () => {
    const onClose = vi.fn();
    const onSaved = vi.fn();
    const baseProps = {
      isOpen: true,
      onClose,
      onSaved,
      defaultDate: "2026-05-04",
      weekFrom: "2026-05-04",
      weekTo: "2026-05-08",
      calendarPeriods: periods,
      defaultCalendarPeriodId: "5",
      canCheckShiftCoverage: true,
      canManageCategories: true,
    } as const;
    const { rerender } = render(<TimetableEventModal {...baseProps} />);

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await waitFor(() => expect(mockCheckConflicts).toHaveBeenCalledTimes(1), {
      timeout: 2000,
    });

    // Editing a probe-key field (Start) re-arms the ~500ms debounce; the
    // debounced draft is stale by the time the modal reopens. Reopening
    // inside that window used to fire one probe with the previous draft.
    fireEvent.change(screen.getByLabelText("Start*"), {
      target: { value: "11:00" },
    });
    mockCheckConflicts.mockClear();
    setupRefs();
    rerender(<TimetableEventModal {...baseProps} isOpen={false} />);
    rerender(<TimetableEventModal {...baseProps} isOpen />);

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 700));
    });
    expect(mockCheckConflicts).not.toHaveBeenCalled();
  });

  it("warns but still saves and closes when a materialize chunk fails after the series was created", async () => {
    mockMaterialize.mockRejectedValue(new Error("chunk down"));
    const { onClose, onSaved } = renderModal({ variant: "quick" });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    await chooseFromSelect(
      screen.getByLabelText("Wiederholt sich"),
      "Jeden Wochentag (Mo–Fr)",
    );
    await clickSave();

    // The template landed once; the follow-up failure must not re-open
    // the form (a retry would create a duplicate template).
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mockCreateTemplate).toHaveBeenCalledTimes(1);
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Regeltermin gespeichert, aber nicht alle Termine konnten eingetragen werden. Die fehlenden Termine werden beim nächsten automatischen Lauf ergänzt.",
    );
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "7" });
    expect(mockToastError).not.toHaveBeenCalled();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("warns but still saves and closes when replanning fails after the series split", async () => {
    mockReplanWeek.mockRejectedValue(new Error("replan down"));
    const { onClose, onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mockSplitTemplate).toHaveBeenCalledTimes(1);
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Regeltermin gespeichert, aber nicht alle Termine konnten eingetragen werden. Die fehlenden Termine werden beim nächsten automatischen Lauf ergänzt.",
    );
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "12" });
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("warns but still saves and closes when replanning fails after the template update", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));
    mockReplanWeek.mockRejectedValue(new Error("replan down"));
    const { onClose, onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await clickSave();

    await waitFor(() => expect(onClose).toHaveBeenCalledOnce());
    expect(mockUpdateTemplate).toHaveBeenCalledTimes(1);
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Regeltermin gespeichert, aber nicht alle Termine konnten eingetragen werden. Die fehlenden Termine werden beim nächsten automatischen Lauf ergänzt.",
    );
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "7" });
    expect(mockToastError).not.toHaveBeenCalled();
  });

  it("translates the past-effective-date backend error in the scope flow", async () => {
    mockSplitTemplate.mockRejectedValue(
      new Error("effective_date must not be in the past"),
    );
    const { onClose } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");

    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());

    await clickSave();

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Der Stichtag liegt in der Vergangenheit. Bitte einen künftigen Termin der Serie wählen.",
      ),
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("explains that a single scope opens only the selected occurrence", async () => {
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Wiederholenden Termin ändern");
    expect(
      screen.getByText("Öffnet nur den Termin am 04.05.2026."),
    ).toBeInTheDocument();
  });

  // #2025: a termin could be saved from step 1 without ever seeing
  // "Wiederholung" or "Personal und Kinder", and the grey "Weiter" drew less
  // attention than the dark "Speichern".
  it("offers Speichern only on the last step", async () => {
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeInTheDocument();

    await goToStep(2);
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();

    await goToStep(3);
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Weiter" }),
    ).not.toBeInTheDocument();
  });

  it("keeps Speichern off the first step when editing an existing instance", async () => {
    renderModal({ initialInstance: savedInstance });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
  });

  it("starts the conversion entry on step 2 and still gates Speichern", async () => {
    renderModal({
      convertInstance: { ...savedInstance, activityGroupId: undefined },
    });

    await screen.findByText("Termin wiederholen");
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Zurück" })).toBeInTheDocument();

    await goToStep(3);
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
  });

  it("sends Weiter back to step 1 when the repeat choice invalidates it", async () => {
    // Choosing a Wiederholung in step 2 retroactively makes Kategorie (a
    // step-1 field) required. "Weiter" must not carry the user to step 3 with
    // that error hidden behind them — it jumps back and shows it.
    vi.stubGlobal(
      "fetch",
      vi
        .fn()
        .mockResolvedValueOnce({
          json: async () => ({
            data: [{ id: 3, name: "Mensa", building: "Haus A" }],
          }),
        })
        // Empty category catalog — nothing to preselect, so Kategorie is empty.
        .mockResolvedValueOnce({ json: async () => ({ data: [] }) })
        .mockResolvedValueOnce({ json: async () => ({ data: [] }) }),
    );
    renderModal({ showPeriodField: true });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");
    await goToStep(2);
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });

    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(
      await screen.findByText("Bitte eine Kategorie auswählen."),
    ).toBeInTheDocument();
    expect(currentStep()).toBe(1);
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
  });

  it("keeps Weiter as the form's submit button so Enter can advance", async () => {
    // A form whose only submit control is missing suppresses implicit
    // submission entirely, so Enter in a field would do nothing. jsdom does
    // not implement implicit submission at all, so the browser-side precondition
    // is asserted here on the button, and the routing itself below.
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    const weiter = screen.getByRole("button", { name: "Weiter" });
    expect(weiter).toHaveAttribute("type", "submit");
    expect(weiter).toHaveAttribute("form", "timetable-event-form");

    await goToStep(2);
    const weiterStep2 = screen.getByRole("button", { name: "Weiter" });
    expect(weiterStep2).toHaveAttribute("type", "submit");
    expect(weiterStep2).toHaveAttribute("form", "timetable-event-form");
  });

  it("lets Enter advance instead of saving before the last step", async () => {
    // Enter in a text field submits the form implicitly, without any visible
    // Speichern button. With valid step-1 fields that used to write the
    // termin straight from step 1 and bypass the wizard gate.
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    await chooseFromSelect(screen.getByLabelText("Raum*"), "Haus A - Mensa");

    const form = document.getElementById("timetable-event-form")!;
    fireEvent.submit(form);

    expect(mockCreate).not.toHaveBeenCalled();
    expect(currentStep()).toBe(2);

    fireEvent.submit(form);

    expect(mockCreate).not.toHaveBeenCalled();
    expect(currentStep()).toBe(3);

    fireEvent.submit(form);
    await waitFor(() => expect(mockCreate).toHaveBeenCalledTimes(1));
  });

  it("keeps Enter from leaving a step whose own fields are invalid", async () => {
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    fireEvent.submit(document.getElementById("timetable-event-form")!);

    expect(
      await screen.findByText("Bitte einen Titel eingeben."),
    ).toBeInTheDocument();
    expect(currentStep()).toBe(1);
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("makes Weiter the dominant action and leaves Abbrechen/Zurück neutral", async () => {
    renderModal();

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    // Primary = bg-gray-900, outline = ring-1; the grey secondary variant
    // (bg-gray-200) must not appear in the footer at all.
    expect(screen.getByRole("button", { name: "Weiter" })).toHaveClass(
      "bg-gray-900",
    );
    expect(screen.getByRole("button", { name: "Abbrechen" })).toHaveClass(
      "ring-1",
    );
    for (const button of screen.getAllByRole("button")) {
      expect(button.className).not.toContain("bg-gray-200");
    }

    await goToStep(2);
    expect(screen.getByRole("button", { name: "Zurück" })).toHaveClass(
      "ring-1",
    );
  });

  it("omits the weekly preset for weekend dates in quick mode", async () => {
    // 2026-05-09 is a Saturday — a "Wöchentlich am Samstag" preset would
    // silently save a Monday series.
    renderModal({ variant: "quick", defaultDate: "2026-05-09" });

    await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
    await goToStep(2);
    fireEvent.click(screen.getByLabelText("Wiederholt sich"));
    expect(
      screen.queryByRole("option", { name: /Wöchentlich am/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Einmalig" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Jeden Wochentag (Mo–Fr)" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Benutzerdefiniert …" }),
    ).toBeInTheDocument();
  });

  // #2187: a series that was already split is a CHAIN of templates. The
  // clicked occurrence can still carry the id of a capped predecessor
  // segment; the backend then resolves it forward and returns the living
  // successor, flagged via `resolvedFromTemplateId`. Every series-scope write
  // must target that successor — and must update it in place instead of
  // splitting it, because the occurrence date lies before the successor's
  // valid_from and cannot be a split point.
  describe("#2187 split-chain resolution", () => {
    /** What the backend returns for the capped predecessor id "87". */
    const resolvedSuccessor: TimetableTemplate = {
      ...template,
      id: "88",
      resolvedFromTemplateId: "87",
    };

    /** An occurrence still pointing at the capped predecessor segment. */
    const chainInstance: EnrichedInstance = {
      ...savedInstance,
      activityGroupId: "87",
    };

    it("updates the resolved successor instead of splitting for 'Ab jetzt dauerhaft'", async () => {
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      const { onSaved } = renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(3);
      await screen.findByText("Max Kind");
      fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          expect.objectContaining({
            // The successor's OWN title and time survive: the form was seeded
            // from the predecessor's occurrence ("Mensa", 12:00), and a split
            // may have changed these fields for the future on purpose. Only a
            // field the user actually edits may overwrite them.
            name: "Yoga",
            start_time: "14:00",
            end_time: "15:00",
            // The clicked occurrence's date, so the backend mirrors the roster
            // change onto the predecessor's remaining occurrences.
            series_roster_from: "2026-05-04",
            // ...restricted to the child the user actually added — everyone
            // else keeps their predecessor rows.
            series_roster_scope_student_ids: [21],
          }),
        ),
      );
      // The successor IS the whole remaining series — nothing left to split.
      expect(mockSplitTemplate).not.toHaveBeenCalled();
      // The template was fetched via the occurrence's own (stale) id.
      expect(mockGetTemplate).toHaveBeenCalledWith("87", "5");
      // The follow-up re-plan runs against the RESOLVED id, not "87".
      await waitFor(() => expect(mockReplanWeek).toHaveBeenCalled());
      for (const call of mockReplanWeek.mock.calls) {
        expect(call[2]).toBe("88");
      }
      expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "88" });
    });

    it("sends today's series_roster_from for 'Alle Termine der Serie'", async () => {
      vi.useFakeTimers({ toFake: ["Date"] });
      vi.setSystemTime(new Date("2026-05-06T10:00:00"));
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Alle Termine der Serie/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(3);
      await screen.findByText("Max Kind");
      fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          // "Alle Termine" starts at today, not at the clicked occurrence.
          expect.objectContaining({ series_roster_from: "2026-05-06" }),
        ),
      );
      expect(mockSplitTemplate).not.toHaveBeenCalled();
    });

    it("reconciles only changed weekdays for a resolved multi-day series", async () => {
      mockGetTemplate.mockResolvedValue({
        ...templateWithWeekdayRosters,
        id: "88",
        resolvedFromTemplateId: "87",
      });
      renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );
      await screen.findByLabelText("Titel*");
      await goToStep(3);
      fireEvent.click(screen.getByRole("button", { name: "Di" }));
      fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          expect.objectContaining({
            series_roster_scope_weekdays: [2],
            series_roster_scope_student_ids: [21],
          }),
        ),
      );
    });

    it("keeps the successor's Hauptbetreuung when only the staff list changed", async () => {
      // The predecessor occurrence is led by 11, the successor by 12 — a
      // deliberate change made at the split. Adding a third supervisor marks
      // the staff roster as touched but says nothing about the Hauptbetreuung.
      mockGetAllStaff.mockResolvedValue([
        { id: "11", name: "Ada Staff" },
        { id: "12", name: "Bo Staff" },
        { id: "13", name: "Cem Staff" },
      ]);
      mockGetTemplate.mockResolvedValue({
        ...resolvedSuccessor,
        staffIds: ["11", "12"],
        primaryStaffId: "12",
      });
      renderModal({
        initialInstance: {
          ...chainInstance,
          staff: [
            {
              staffId: "11",
              isPrimary: true,
              isAbsent: false,
              isSubstitute: false,
            },
          ],
        },
      });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(3);
      await screen.findByText("Cem Staff");
      fireEvent.click(screen.getByRole("checkbox", { name: /Cem Staff/ }));
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          expect.objectContaining({
            primary_staff_id: 12,
            staff_ids: [11, 12, 13],
            // Only the added supervisor is reconciled on the predecessor.
            series_roster_scope_staff_ids: [13],
          }),
        ),
      );
    });

    it("omits series_roster_primary_changed unless the Hauptbetreuung moved", async () => {
      mockGetAllStaff.mockResolvedValue([
        { id: "11", name: "Ada Staff" },
        { id: "13", name: "Cem Staff" },
      ]);
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      renderModal({
        initialInstance: {
          ...chainInstance,
          staff: [
            {
              staffId: "11",
              isPrimary: true,
              isAbsent: false,
              isSubstitute: false,
            },
          ],
        },
      });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(3);
      await screen.findByText("Cem Staff");
      fireEvent.click(screen.getByRole("checkbox", { name: /Cem Staff/ }));
      await clickSave();

      await waitFor(() => expect(mockUpdateTemplate).toHaveBeenCalled());
      const body = mockUpdateTemplate.mock.calls[0]?.[1] as unknown;
      // A pure membership change must not let the successor's lead travel onto
      // the predecessor's rows.
      expect(body).not.toHaveProperty("series_roster_primary_changed");
    });

    it("still writes a scalar field the user actually edited", async () => {
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      fireEvent.change(screen.getByLabelText("Start*"), {
        target: { value: "12:30" },
      });
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          expect.objectContaining({
            // Edited → the user's value wins.
            start_time: "12:30",
            // Untouched → the successor keeps its own.
            end_time: "15:00",
            name: "Yoga",
          }),
        ),
      );
    });

    it("scopes the reconciliation to a removed child only", async () => {
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      renderModal({
        initialInstance: { ...chainInstance, studentIds: ["21"] },
      });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(3);
      await screen.findByText("Max Kind");
      // The child was on the occurrence — this click removes them.
      fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          expect.objectContaining({
            series_roster_from: "2026-05-04",
            series_roster_scope_student_ids: [21],
            student_ids: [],
          }),
        ),
      );
    });

    it("omits series_roster_from when the roster was not touched", async () => {
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      fireEvent.change(screen.getByLabelText("Titel*"), {
        target: { value: "Mensa neu" },
      });
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          expect.objectContaining({ name: "Mensa neu" }),
        ),
      );
      // A title-only edit must not ask the backend to rewrite past rosters.
      const body = mockUpdateTemplate.mock.calls[0]?.[1] as unknown;
      expect(body).not.toHaveProperty("series_roster_from");
      expect(mockSplitTemplate).not.toHaveBeenCalled();
    });

    it("still splits when the backend did not resolve a chain", async () => {
      // Control case: no resolvedFromTemplateId → the pre-#2187 split path.
      mockGetTemplate.mockResolvedValue(template);
      renderModal({
        initialInstance: { ...savedInstance, activityGroupId: "7" },
      });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await clickSave();

      await waitFor(() =>
        expect(mockSplitTemplate).toHaveBeenCalledWith(
          "7",
          expect.objectContaining({ effective_date: "2026-05-04" }),
        ),
      );
      expect(mockUpdateTemplate).not.toHaveBeenCalled();
    });

    it("merges an added child into the successor roster instead of overwriting it", async () => {
      // 999 exists only on the successor — the occurrence snapshot (101, 105)
      // predates it. Writing the form list wholesale would delete that child.
      mockFetchStudents.mockResolvedValue({
        students: [
          { id: "101", name: "Anna Alt", school_class: "1a" },
          { id: "105", name: "Bea Alt", school_class: "1a" },
          { id: "7", name: "Cem Neu", school_class: "1b" },
        ],
      });
      mockGetTemplate.mockResolvedValue({
        ...resolvedSuccessor,
        studentIds: ["101", "105", "999"],
      });
      renderModal({
        initialInstance: { ...chainInstance, studentIds: ["101", "105"] },
      });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await goToStep(3);
      await screen.findByText("Cem Neu");
      fireEvent.click(screen.getByRole("checkbox", { name: /Cem Neu/ }));
      await clickSave();

      await waitFor(() =>
        expect(mockUpdateTemplate).toHaveBeenCalledWith(
          "88",
          // Successor roster kept in full, the user's addition appended.
          expect.objectContaining({ student_ids: [101, 105, 999, 7] }),
        ),
      );
    });

    it("shows the German message when the series template cannot be loaded", async () => {
      mockGetTemplate.mockRejectedValue(
        Object.assign(new Error("template not found"), {
          httpStatus: 404,
          code: "template_not_found",
        }),
      );
      const { onClose } = renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Alle Termine der Serie/ }),
      );

      const message =
        "Der Regeltermin konnte nicht geladen werden. Bitte laden Sie die Seite neu und öffnen Sie den Termin erneut.";
      await waitFor(() => expect(mockToastError).toHaveBeenCalledWith(message));
      expect(await screen.findByText(new RegExp(message))).toBeInTheDocument();
      expect(mockUpdateTemplate).not.toHaveBeenCalled();
      expect(mockSplitTemplate).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
    });

    it("passes the resolved id and no deletion count to the lost-edits probe", async () => {
      mockGetTemplate.mockResolvedValue(resolvedSuccessor);
      renderModal({ initialInstance: chainInstance });

      await screen.findByText("Wiederholenden Termin ändern");
      fireEvent.click(
        screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }),
      );

      await waitFor(() => expect(screen.getByLabelText("Raum*")).toBeEnabled());
      await clickSave();

      // In-place update, not a split: individually deleted occurrences survive,
      // so counting them would warn about a loss that cannot happen.
      await waitFor(() =>
        expect(mockCountEditedInWindow).toHaveBeenCalledWith(
          "88",
          expect.any(String),
          expect.any(String),
          false,
        ),
      );
    });
  });
});
