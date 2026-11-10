// #6593: Solange ein Termin gespeichert oder ein Regeltermin gelöscht wird,
// ist das Panel gesperrt; eine späte Antwort schließt kein neu geöffnetes
// Panel und wechselt keinen Dialog. Echter SlideOver und echter Löschdialog.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { CalendarPeriod } from "~/lib/calendar-period-helpers";
import type { TenantInfo } from "~/lib/tenant-api";
import { useTenant } from "~/lib/tenant-context";
import type {
  EnrichedInstance,
  TimetableTemplate,
} from "~/lib/timetable-types";
import { catalogText } from "~/test/error-catalog-text";
import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

const { mockToastSuccess, mockCreate } = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockCreate: vi.fn(),
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: () => Promise.resolve({ students: [] }),
}));

vi.mock("~/lib/staff-api", () => ({
  staffService: { getAllStaff: () => Promise.resolve([]) },
}));

vi.mock("~/lib/planning-track-api", () => ({
  planningTrackService: { list: () => Promise.resolve([]) },
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    create: mockCreate,
    checkConflicts: () =>
      Promise.resolve({
        date: "2026-09-14",
        startTime: "12:00",
        endTime: "13:00",
        warnings: [],
      }),
    getOfferingSources: () => Promise.resolve([]),
    getCombinedOfferingCounts: () =>
      Promise.resolve({ totalCount: 0, gradeCounts: {}, students: [] }),
    getOfferingPreview: () =>
      Promise.resolve({ from: "2026-09-09", weekdays: [], segments: [] }),
  },
}));

import { TimetableEventModal } from "./timetable-event-modal";

const periods: CalendarPeriod[] = [
  {
    id: "5",
    tenantId: "1",
    name: "Schuljahr 2026/2027",
    periodType: "school_year",
    startDate: "2026-08-01",
    endDate: "2026-12-31",
    weekCycleLength: 1,
    weekCycleAnchor: "2026-08-03",
    isActive: true,
    createdAt: "2026-05-01T00:00:00Z",
    updatedAt: "2026-05-01T00:00:00Z",
  },
];

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
  studentIds: [],
  staffIds: [],
  primaryStaffId: "",
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

const savedInstance = {
  id: "42",
  date: "2026-09-14",
  title: "Mensa",
} as EnrichedInstance;

function stubReferenceFetch() {
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: unknown) => {
      const url = String(input);
      const data = url.includes("/api/rooms")
        ? [{ id: 3, name: "Mensa", building: "Haus A" }]
        : url.includes("/api/activities/categories")
          ? [{ id: "2", name: "AG" }]
          : [];
      return { json: async () => ({ data }) };
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
      defaultDate="2026-09-14"
      weekFrom="2026-09-14"
      weekTo="2026-09-18"
      calendarPeriods={periods}
      planningPeriods={periods}
      defaultCalendarPeriodId="5"
      canCheckShiftCoverage={false}
      canManageCategories={false}
      {...props}
    />
  );
  const view = render(modal(true));
  return {
    ...view,
    onClose,
    onSaved,
    reopen: () => {
      view.rerender(modal(false));
      view.rerender(modal(true));
    },
  };
}

/** Fills Titel and Raum, walks to the last step and clicks Speichern. */
async function createOneOff() {
  const answer = deferred<EnrichedInstance>();
  mockCreate.mockReturnValue(answer.promise);
  const view = renderModal();
  fireEvent.change(screen.getByLabelText(/^Titel$/), {
    target: { value: "Mensa" },
  });
  const room = screen.getByLabelText(/^Raum$/);
  await waitFor(() => expect(room).toBeEnabled());
  fireEvent.click(room);
  fireEvent.click(
    await screen.findByRole("option", { name: "Haus A - Mensa" }),
  );
  fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
  fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
  const saveButton = screen.getByRole("button", { name: "Speichern" });
  await waitFor(() => expect(saveButton).toBeEnabled());
  fireEvent.click(saveButton);
  await waitFor(() => expect(mockCreate).toHaveBeenCalledOnce());
  return { ...view, answer };
}

async function deleteSeries() {
  const answer = deferred();
  const onDeleteSeries = vi.fn(() => answer.promise);
  const view = renderModal({ initialSeries: template, onDeleteSeries });
  await screen.findByRole("heading", { name: "Regeltermin bearbeiten" });
  fireEvent.click(screen.getByRole("button", { name: "Löschen" }));
  const dialog = screen.getByRole("dialog", { name: "Regeltermin löschen" });
  fireEvent.click(within(dialog).getByRole("button", { name: "Löschen" }));
  fireEvent.click(
    within(dialog).getByRole("button", { name: "Endgültig löschen" }),
  );
  expect(onDeleteSeries).toHaveBeenCalledWith(
    expect.objectContaining({ id: "7" }),
    "2026-09-09",
  );
  return { ...view, answer, dialog };
}

describe("TimetableEventModal pending lock (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useTenant).mockReturnValue({
      tenantSlug: "test-tenant",
      routingMode: "path",
      tenant: { gradeLevelMax: 13 } as TenantInfo,
    });
    stubReferenceFetch();
  });

  describe("Speichern", () => {
    it("sperrt das Panel beim Speichern und gibt es nach einem Fehler mit allen Eingaben frei", async () => {
      const { answer, onClose } = await createOneOff();

      expectSlideOverSaveLocked("Termin", [
        "Abbrechen",
        "Zurück",
        "Speichere …",
      ]);

      await act(async () => {
        answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
      });

      expect(await screen.findByRole("alert")).toBeInTheDocument();
      expectSlideOverUnlocked("Termin");
      fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
      fireEvent.click(screen.getByRole("button", { name: "Zurück" }));
      expect(screen.getByLabelText(/^Titel$/)).toHaveValue("Mensa");
      expect(onClose).not.toHaveBeenCalled();
    });

    it("schließt das Panel nach dem Speichern", async () => {
      const { answer, onClose, onSaved } = await createOneOff();

      await act(async () => {
        answer.resolve(savedInstance);
      });

      expect(onSaved).toHaveBeenCalledWith({
        kind: "instance",
        instance: savedInstance,
      });
      expect(onClose).toHaveBeenCalledOnce();
    });

    it("schließt und wechselt nach einer späten Antwort nichts im neu geöffneten Panel", async () => {
      const { answer, onClose, onSaved, reopen } = await createOneOff();

      reopen();
      expectSlideOverUnlocked("Termin");
      await act(async () => {
        answer.resolve(savedInstance);
      });

      expect(mockToastSuccess).toHaveBeenCalledWith("Der Termin ist angelegt.");
      expect(onSaved).not.toHaveBeenCalled();
      expect(onClose).not.toHaveBeenCalled();
      expect(
        screen.getByRole("heading", { name: "Termin" }),
      ).toBeInTheDocument();
    });

    it("zeigt einen späten Fehler nicht im neu geöffneten Panel", async () => {
      const { answer, reopen } = await createOneOff();

      reopen();
      await act(async () => {
        answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
      });

      expect(
        screen.getByRole("heading", { name: "Termin" }),
      ).toBeInTheDocument();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });
  });

  describe("Regeltermin löschen", () => {
    it("sperrt das Panel beim Löschen und gibt es nach einem Fehler frei", async () => {
      const { answer, dialog, onClose } = await deleteSeries();

      expectSlideOverSaveLocked("Regeltermin bearbeiten", [
        "Löschen",
        "Abbrechen",
        "Weiter",
      ]);

      await act(async () => {
        answer.reject(
          new ApiError("weg", 503, { code: "general.unavailable" }),
        );
      });

      expect(
        await within(dialog).findByText(
          catalogText("general.unavailable", "das Löschen des Regeltermins"),
        ),
      ).toBeInTheDocument();
      expectSlideOverUnlocked("Regeltermin bearbeiten");
      expect(onClose).not.toHaveBeenCalled();
    });

    it("schließt das Panel nach dem Löschen", async () => {
      const { answer, onClose } = await deleteSeries();

      await act(async () => {
        answer.resolve();
      });

      expect(onClose).toHaveBeenCalledOnce();
    });

    it("schließt nach einem späten Löschen nichts", async () => {
      const { answer, onClose, unmount } = await deleteSeries();

      unmount();
      await act(async () => {
        answer.resolve();
      });

      expect(onClose).not.toHaveBeenCalled();
    });
  });
});
