import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const {
  mockToastSuccess,
  mockToastError,
  mockCreate,
  mockUpdate,
  mockCreateTemplate,
  mockUpdateTemplate,
  mockMaterialize,
  mockFetchStudents,
  mockGetAllStaff,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockCreate: vi.fn(),
  mockUpdate: vi.fn(),
  mockCreateTemplate: vi.fn(),
  mockUpdateTemplate: vi.fn(),
  mockMaterialize: vi.fn(),
  mockFetchStudents: vi.fn(),
  mockGetAllStaff: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
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
  },
}));

import { TimetableEventModal } from "./timetable-event-modal";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
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
  enrollmentCount: 8,
  supervisorCount: 1,
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

function renderModal(
  props: Partial<React.ComponentProps<typeof TimetableEventModal>> = {},
) {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  render(
    <TimetableEventModal
      isOpen
      onClose={onClose}
      onSaved={onSaved}
      defaultDate="2026-05-04"
      weekFrom="2026-05-04"
      weekTo="2026-05-08"
      calendarPeriods={periods}
      defaultCalendarPeriodId="5"
      {...props}
    />,
  );
  return { onClose, onSaved };
}

describe("TimetableEventModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    setupRefs();
    mockCreate.mockResolvedValue(savedInstance);
    mockUpdate.mockResolvedValue(savedInstance);
    mockCreateTemplate.mockResolvedValue({
      templateId: "7",
      instancesCreated: 2,
    });
    mockUpdateTemplate.mockResolvedValue(template);
    mockMaterialize.mockResolvedValue({ instancesCreated: 1, warnings: [] });
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

    fireEvent.change(screen.getByPlaceholderText("Kinder suchen ..."), {
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

    fireEvent.click(
      screen.getAllByRole("button", { name: "Auswahl leeren" })[0]!,
    );
    expect(screen.getAllByText("0 ausgewählt").length).toBeGreaterThan(0);
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

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Endzeit muss nach der Startzeit liegen.",
    );
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("creates a recurring series and materializes the visible week", async () => {
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
    fireEvent.change(screen.getByLabelText("Planungsperiode*"), {
      target: { value: "5" },
    });
    fireEvent.click(screen.getByRole("checkbox", { name: /Ada Staff/ }));
    fireEvent.change(screen.getByLabelText("Hauptverantwortlich"), {
      target: { value: "11" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(mockCreateTemplate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Yoga",
          type: "activity",
          room_id: 3,
          category_id: 2,
          calendar_period_id: 5,
          materialize_from: "2026-05-04",
          materialize_to: "2026-05-08",
          primary_staff_id: 11,
        }),
      ),
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Serientermin angelegt - 2 Termine eingeplant",
    );
    expect(onSaved).toHaveBeenCalledWith({ kind: "series", seriesId: "7" });
  });

  it("updates an existing series and converts an instance to a series", async () => {
    const { onSaved: onSeriesSaved } = renderModal({
      initialSeries: template,
      defaultDate: "2026-05-04",
    });

    await screen.findByText("Serie bearbeiten");
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mockUpdateTemplate).toHaveBeenCalledWith(
        "7",
        expect.objectContaining({ name: "Yoga", room_id: 3 }),
      ),
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
    expect(mockMaterialize).toHaveBeenCalledWith("2026-05-04", "2026-05-08");
    expect(onConvertSaved).toHaveBeenCalledWith({
      kind: "series",
      seriesId: "8",
      linkedInstanceId: "42",
    });
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
});
