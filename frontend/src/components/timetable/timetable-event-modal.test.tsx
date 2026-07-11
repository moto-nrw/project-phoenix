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

const {
  mockToastSuccess,
  mockToastError,
  mockToastWarning,
  mockCreate,
  mockUpdate,
  mockCreateTemplate,
  mockUpdateTemplate,
  mockMaterialize,
  mockGetTemplate,
  mockSplitTemplate,
  mockReplanWeek,
  mockCheckConflicts,
  mockCheckShiftCoverage,
  mockFetchStudents,
  mockGetAllStaff,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockToastWarning: vi.fn(),
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockCreateTemplate: vi.fn(),
  mockUpdateTemplate: vi.fn(),
  mockMaterialize: vi.fn(),
  mockGetTemplate: vi.fn(),
  mockSplitTemplate: vi.fn(),
  mockReplanWeek: vi.fn(),
  mockCheckConflicts: vi.fn(),
  mockCheckShiftCoverage: vi.fn(),
  mockFetchStudents: vi.fn(),
  mockGetAllStaff: vi.fn(),
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

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    create: mockCreate,
    update: mockUpdate,
    createTemplate: mockCreateTemplate,
    updateTemplate: mockUpdateTemplate,
    materialize: mockMaterialize,
    getTemplate: mockGetTemplate,
    splitTemplate: mockSplitTemplate,
    replanWeek: mockReplanWeek,
    checkConflicts: mockCheckConflicts,
    checkShiftCoverage: mockCheckShiftCoverage,
  },
}));

import { TimetableEventModal } from "./timetable-event-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import { useTenant } from "~/lib/tenant-context";
import type { TenantInfo } from "~/lib/tenant-api";
import type {
  EnrichedInstance,
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
    mockCheckConflicts.mockResolvedValue({
      date: "2026-05-04",
      startTime: "12:00",
      endTime: "13:00",
      warnings: [],
    });
    mockCheckShiftCoverage.mockResolvedValue({ coverageWarnings: [] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("creates a one-off instance with selected people", async () => {
    const { onClose, onSaved } = renderModal();

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /Max Kind/ }));
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    fireEvent.change(screen.getByLabelText("Notiz"), {
      target: { value: "ohne Nuesse" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText("Haus A - Mensa");
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

    fireEvent.change(screen.getByLabelText("Nach Jahrgang filtern"), {
      target: { value: "1" },
    });
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

    expect(await screen.findByText("Haus A - Mensa")).toBeInTheDocument();
    expect(screen.getByLabelText("Raum*")).toBeEnabled();
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
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

    expect(
      await screen.findByText(
        "Die Kinderliste konnte nicht vollständig geladen werden. Die Kinderzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
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
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("keeps saving available and retries when staff cannot be loaded", async () => {
    mockGetAllStaff
      .mockRejectedValueOnce(new Error("forbidden"))
      .mockResolvedValueOnce([{ id: "11", name: "Ada Staff" }]);

    renderModal();

    expect(
      await screen.findByText(
        "Die Personalliste konnte nicht vollständig geladen werden. Die Personalzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Ada Staff")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();

    fireEvent.click(
      screen.getByRole("button", { name: "Personal erneut laden" }),
    );

    expect(await screen.findByText("Ada Staff")).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Personalliste konnte nicht vollständig/),
    ).not.toBeInTheDocument();
  });

  it("reveals a student load failure immediately in quick mode", async () => {
    mockFetchStudents.mockRejectedValue(new Error("students unavailable"));

    renderModal({ variant: "quick" });

    expect(
      await screen.findByText(
        "Die Kinderliste konnte nicht vollständig geladen werden. Die Kinderzuordnung kann deshalb nicht bearbeitet werden und bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Weitere Optionen" }),
    ).toHaveAttribute("aria-expanded", "true");
    expect(
      screen.getByRole("button", { name: "Kinder erneut laden" }),
    ).toBeVisible();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("preserves an existing roster when the account cannot load students", async () => {
    mockFetchStudents.mockRejectedValue(new Error("forbidden"));
    renderModal({
      initialInstance: { ...savedInstance, studentIds: ["21"] },
    });

    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    expect(screen.queryByText("Max Kind")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    const classSelect = screen.getByLabelText(/^Klasse\*/);
    expect(classSelect).toHaveValue("Klasse 3a");
    expect(classSelect).toBeDisabled();
    expect(
      screen.getByRole("option", { name: "Klasse 3a" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/bestehende Klassen-Zielgruppe bleibt unverändert/),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    expect(await screen.findByText("Neue Kinderliste")).toBeInTheDocument();

    await act(async () => {
      staleRequest.reject(new Error("stale student request failed"));
      await staleRequest.promise.catch(() => undefined);
    });

    expect(screen.getByText("Neue Kinderliste")).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Kinderliste konnte nicht vollständig/),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
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
    fireEvent.click(
      await screen.findByRole("button", { name: "Kinder erneut laden" }),
    );
    await waitFor(() => expect(mockFetchStudents).toHaveBeenCalledTimes(2));

    setOpen(false);
    setOpen(true);

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
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("validates shared fields before submitting", async () => {
    renderModal();

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.change(screen.getByLabelText("Ende*"), {
      target: { value: "11:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Endzeit muss nach der Startzeit liegen."),
    ).toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("shows inline required-field errors on submit and clears them on change", async () => {
    renderModal();

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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
    renderModal({ showPeriodField: true });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    fireEvent.change(screen.getByLabelText("Kategorie*"), {
      target: { value: "" },
    });
    fireEvent.change(screen.getByLabelText("Planungszeitraum*"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte eine Kategorie auswählen."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte einen Planungszeitraum auswählen."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Bitte mindestens einen Wochentag auswählen."),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Mo" }));
    expect(
      screen.queryByText("Bitte mindestens einen Wochentag auswählen."),
    ).not.toBeInTheDocument();
  });

  it("requires the value belonging to each selected Zielgruppe", async () => {
    renderModal({ showPeriodField: true });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Lernzeit" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte einen Jahrgang auswählen."),
    ).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Klasse" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte eine Klasse auswählen."),
    ).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Gruppe" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText("Bitte eine Gruppe auswählen."),
    ).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
  });

  it("creates a recurring series and materializes the full period in 56-day chunks", async () => {
    const { onSaved } = renderModal({ showPeriodField: true });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Yoga" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    fireEvent.change(screen.getByLabelText("Kategorie*"), {
      target: { value: "2" },
    });
    fireEvent.change(screen.getByLabelText("Planungszeitraum*"), {
      target: { value: "5" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    fireEvent.change(screen.getByLabelText("Zuständige Person"), {
      target: { value: "11" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    // The create call carries the first 56-day window of the period
    // (2026-05-01 … 2026-12-31); the remaining four windows follow as
    // separate materialize calls.
    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Yoga",
          type: "activity",
          room_id: 3,
          category_id: 2,
          calendar_period_id: 5,
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

  it("submits Zielgruppe Jahrgang with the selected grade level", async () => {
    renderModal({ showPeriodField: true });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Hausaufgabenbetreuung" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    fireEvent.change(screen.getByLabelText("Kategorie*"), {
      target: { value: "2" },
    });
    fireEvent.change(screen.getByLabelText("Planungszeitraum*"), {
      target: { value: "5" },
    });

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    expect(
      screen.getByRole("option", { name: "Jahrgang 13" }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "Jahrgang 14" })).toBeNull();
    fireEvent.change(screen.getByLabelText(/^Jahrgang\*/), {
      target: { value: "13" },
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          target_group_type: "jahrgang",
          target_grade_level: 13,
        }),
      ),
    );
  });

  it("shows and preserves an existing grade above the current tenant cap", async () => {
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: { gradeLevelMax: 4 } as TenantInfo,
    });
    renderModal({
      initialSeries: {
        ...template,
        targetGroupType: "jahrgang",
        targetGradeLevel: 13,
      },
      showPeriodField: true,
    });

    await screen.findByText("Haus A - Mensa");
    const gradeSelect = screen.getByLabelText(/^Jahrgang\*/);
    expect(gradeSelect).toHaveValue("13");
    expect(
      screen.getByRole("option", { name: "Jahrgang 13 (bestehend)" }),
    ).toBeDisabled();
    expect(
      screen.getByText(/über der aktuell konfigurierten Höchststufe 4/),
    ).toBeVisible();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          target_group_type: "jahrgang",
          target_grade_level: 13,
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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("checkbox", { name: /Nora Vier A/ }));
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Hausaufgabenbetreuung" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    fireEvent.change(screen.getByLabelText("Kategorie*"), {
      target: { value: "2" },
    });
    fireEvent.change(screen.getByLabelText("Planungszeitraum*"), {
      target: { value: "5" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    fireEvent.change(screen.getByLabelText(/^Jahrgang\*/), {
      target: { value: "3" },
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Alle 2 Kinder aus Jahrgang 3 übernehmen",
      }),
    );

    expect(screen.getByRole("checkbox", { name: /Mara Drei A/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Mika Drei B/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /Nora Vier A/ })).toBeChecked();
    expect(screen.getByText("3 ausgewählt")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Hausaufgabenbetreuung" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jede Woche" }), {
      button: 0,
    });
    fireEvent.click(screen.getByRole("button", { name: /AG Yoga/ }));
    fireEvent.change(screen.getByLabelText("Kategorie*"), {
      target: { value: "2" },
    });
    fireEvent.change(screen.getByLabelText("Planungszeitraum*"), {
      target: { value: "5" },
    });

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Jahrgang" }), {
      button: 0,
    });
    fireEvent.change(screen.getByLabelText(/^Jahrgang\*/), {
      target: { value: "3" },
    });
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Keine" }), {
      button: 0,
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          target_group_type: "none",
          target_grade_level: undefined,
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
    expect(screen.getByLabelText("Planungszeitraum*")).toHaveValue("6");

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ calendar_period_id: 6 }),
      ),
    );
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
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
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
    mockCreateTemplate.mockResolvedValue({ templateId: "8" });
    mockUpdate.mockResolvedValue(savedInstance);
    const { onSaved: onConvertSaved } = renderModal({
      convertInstance: { ...savedInstance, activityGroupId: undefined },
    });

    await screen.findByText("Termin wiederholen");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mockCreateTemplate).toHaveBeenCalled());
    expect(mockUpdate).toHaveBeenCalledWith(
      "42",
      expect.objectContaining({ activity_group_id: 8 }),
    );
    // Conversion materializes the whole period in 56-day chunks.
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

    await screen.findByText("Haus A - Mensa");
    expect(screen.queryByRole("button", { name: "Löschen" })).toBeNull();
  });

  it("surfaces save failures as validation errors", async () => {
    mockCreate.mockRejectedValueOnce(new Error("Backend sagt nein"));
    renderModal();

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText("Haus A - Mensa");
    expect(screen.getByLabelText("Start*")).toHaveValue("08:00");
    expect(screen.getByLabelText("Ende*")).toHaveValue("09:30");
    expect(screen.getByLabelText("Wiederholt sich")).toBeInTheDocument();
    // 2026-05-04 is a Monday — the dynamic weekly option names the weekday.
    expect(
      screen.getByRole("option", { name: "Wöchentlich am Montag" }),
    ).toBeInTheDocument();
    // Full-form controls stay hidden while collapsed.
    expect(screen.queryByRole("tab", { name: "Jede Woche" })).toBeNull();
    expect(screen.queryByLabelText("Kategorie*")).toBeNull();
    expect(screen.queryByText("Betreuung")).toBeNull();
    expect(screen.queryByText("Personal")).toBeNull();
    expect(screen.queryByText("Kinder")).toBeNull();

    // The disclosure reveals people and notes.
    fireEvent.click(screen.getByRole("button", { name: /Weitere Optionen/ }));
    expect(screen.getByText("Personal")).toBeInTheDocument();
    expect(screen.getByText("Kinder")).toBeInTheDocument();
    expect(screen.getByLabelText("Notiz")).toBeInTheDocument();
  });

  it("quick preset 'Jeden Wochentag' creates a Mo-Fr weekly series", async () => {
    renderModal({ variant: "quick" });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.change(screen.getByLabelText("Wiederholt sich"), {
      target: { value: "jeden-wochentag" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Mensa",
          weekdays: [1, 2, 3, 4, 5],
          week_pattern: 0,
          calendar_period_id: 5,
          materialize_from: "2026-05-01",
          materialize_to: "2026-06-25",
        }),
      ),
    );
  });

  it("quick preset 'Benutzerdefiniert' expands the full form", async () => {
    renderModal({ variant: "quick" });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Wiederholt sich"), {
      target: { value: "benutzerdefiniert" },
    });

    expect(
      await screen.findByRole("tab", { name: "Jede Woche" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Personal")).toBeInTheDocument();
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

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.change(screen.getByLabelText("Wiederholt sich"), {
      target: { value: "jeden-wochentag" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Bitte eine Kategorie auswählen."),
    ).toBeInTheDocument();
    // The form expanded so the inline error is visible.
    expect(screen.getByRole("tab", { name: "Jede Woche" })).toBeInTheDocument();
    expect(mockCreateTemplate).not.toHaveBeenCalled();
  });

  it("asks for the scope when editing a series instance and applies 'Nur diese Woche'", async () => {
    const { onSaved } = renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText("Wiederholenden Termin ändern"),
    ).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
    // The scope copy explains the effect, not the split mechanism.
    expect(
      screen.getByText(
        "Ändert diesen und alle künftigen Termine ab dem 04.05.2026 dauerhaft; frühere Termine bleiben unverändert.",
      ),
    ).toBeInTheDocument();
    // Neither Datum nor Notiz changed — no single-scope hint.
    expect(
      screen.queryByText(/Geändertes Datum und Notiz/),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() =>
      expect(mockSplitTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          effective_date: "2026-05-04",
          materialize_from: "2026-05-04",
          materialize_to: "2026-06-28",
          name: "Mensa",
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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith({
        dates: ["2026-05-04", "2026-05-06", "2026-05-11", "2026-05-13"],
        startTime: "12:00",
        endTime: "13:00",
        staffIds: ["11"],
        calendarPeriodId: "5",
        weekPattern: 0,
      }),
    );
    expect(mockToastWarning).toHaveBeenCalledWith(
      "Ada Staff fehlt am Mittwoch von 12:30–13:00.",
      { duration: 10_000 },
    );
    await waitFor(() => expect(mockSplitTemplate).toHaveBeenCalled());
  });

  it("preserves a template-only period pin and its end for 'Ab jetzt dauerhaft'", async () => {
    mockGetTemplate.mockResolvedValue(templateWithTemplateOnlyPeriodPin);
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
      calendarPeriods: [...periods, templatePinnedPeriod],
    });

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

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

    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

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

    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() =>
      expect(mockCheckShiftCoverage).toHaveBeenCalledWith(
        expect.objectContaining({ replanActivityGroupId: "7" }),
      ),
    );

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({
          name: "Mensa",
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

  it("preserves a template-only period pin and its end for 'Alle Termine der Serie'", async () => {
    vi.useFakeTimers({ toFake: ["Date"] });
    vi.setSystemTime(new Date("2026-05-04T10:00:00"));
    mockGetTemplate.mockResolvedValue(templateWithTemplateOnlyPeriodPin);
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
      calendarPeriods: [...periods, templatePinnedPeriod],
    });

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

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

    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ student_ids: [21, 22] }),
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

    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

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

    await screen.findByText(/Die Kinderliste konnte nicht vollständig/);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

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

    await screen.findByText(/Die Personalliste konnte nicht vollständig/);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Nur diese Woche/ }));

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(mockUpdate).toHaveBeenCalled());
    expect(screen.queryByText("Wiederholenden Termin ändern")).toBeNull();
    expect(onSaved).toHaveBeenCalledWith({
      kind: "instance",
      instance: savedInstance,
    });
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

    await screen.findByText("Haus A - Mensa");

    // Personal renders before Kinder (Streichliste 8).
    const fieldLabels = screen
      .getAllByText(/^(Personal|Kinder)$/)
      .map((node) => node.textContent);
    expect(fieldLabels).toEqual(["Personal", "Kinder"]);
    expect(
      screen.getByRole("option", { name: "Klasse 1a" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "Klasse Klasse 1a" }),
    ).not.toBeInTheDocument();

    fireEvent.change(
      screen.getByLabelText("Jahrgang, Klasse oder Gruppe komplett hinzufügen"),
      { target: { value: "class:1a" } },
    );
    expect(screen.getByText("2 ausgewählt")).toBeInTheDocument();

    fireEvent.change(
      screen.getByLabelText("Jahrgang, Klasse oder Gruppe komplett hinzufügen"),
      { target: { value: "group:OGS Blau" } },
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

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });

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
    expect(
      await screen.findByText(
        "Hinweis: Raum Mensa ist 12:00–13:00 bereits belegt",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));

    expect(
      await screen.findByText(
        "Hinweis: Ada Staff ist für 12:00–13:00 eingeteilt; nicht durch eine Schicht abgedeckt: 12:30–13:00.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
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

    await screen.findByText("Haus A - Mensa");
    mockCheckShiftCoverage.mockClear();
    mockCheckShiftCoverage.mockReturnValueOnce(coverage.promise);
    fireEvent.change(screen.getByLabelText("Ende*"), {
      target: { value: "13:15" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText("Haus A - Mensa");
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

  it("shows a non-blocking warning when shift coverage cannot be checked", async () => {
    mockCheckShiftCoverage.mockRejectedValue(new Error("probe down"));
    renderModal();

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));

    await waitFor(() => expect(mockCheckShiftCoverage).toHaveBeenCalled(), {
      timeout: 2000,
    });
    expect(
      await screen.findByText(
        "Hinweis: Die Dienstplan-Abdeckung konnte nicht geprüft werden. Speichern ist weiterhin möglich.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText("Haus A - Mensa");
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
    } as const;
    const { rerender } = render(<TimetableEventModal {...baseProps} />);

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    await waitFor(() => expect(mockCheckConflicts).toHaveBeenCalledTimes(1), {
      timeout: 2000,
    });

    // Clearing the room changes the probe key; the debounced key still
    // holds the room for ~500ms. Reopening inside that window used to
    // fire one probe with the previous draft.
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "" },
    });
    mockCheckConflicts.mockClear();
    setupRefs();
    rerender(<TimetableEventModal {...baseProps} isOpen={false} />);
    rerender(<TimetableEventModal {...baseProps} isOpen />);

    await screen.findByText("Haus A - Mensa");
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 700));
    });
    expect(mockCheckConflicts).not.toHaveBeenCalled();
  });

  it("warns but still saves and closes when a materialize chunk fails after the series was created", async () => {
    mockMaterialize.mockRejectedValue(new Error("chunk down"));
    const { onClose, onSaved } = renderModal({ variant: "quick" });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Titel*"), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText("Raum*"), {
      target: { value: "3" },
    });
    fireEvent.change(screen.getByLabelText("Wiederholt sich"), {
      target: { value: "jeden-wochentag" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(
      screen.getByRole("button", { name: /Alle Termine der Serie/ }),
    );

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

    await screen.findByText("Haus A - Mensa");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Wiederholenden Termin ändern");
    fireEvent.click(screen.getByRole("button", { name: /Ab jetzt dauerhaft/ }));

    await waitFor(() =>
      expect(mockToastError).toHaveBeenCalledWith(
        "Der Stichtag liegt in der Vergangenheit. Bitte einen künftigen Termin der Serie wählen.",
      ),
    );
    expect(onClose).not.toHaveBeenCalled();
  });

  it("flags date and note as single-scope-only when the note changed", async () => {
    renderModal({
      initialInstance: { ...savedInstance, activityGroupId: "7" },
    });

    await screen.findByText("Haus A - Mensa");
    fireEvent.change(screen.getByLabelText("Notiz"), {
      target: { value: "neuer Hinweis" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await screen.findByText("Wiederholenden Termin ändern");
    expect(
      screen.getByText(
        /Geändertes Datum und Notiz gelten nur bei „Nur diese Woche“\./,
      ),
    ).toBeInTheDocument();
  });

  it("omits the weekly preset for weekend dates in quick mode", async () => {
    // 2026-05-09 is a Saturday — a "Wöchentlich am Samstag" preset would
    // silently save a Monday series.
    renderModal({ variant: "quick", defaultDate: "2026-05-09" });

    await screen.findByText("Haus A - Mensa");
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
});
