import { fireEvent, render as rtlRender, screen } from "@testing-library/react";
import type { ReactElement } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { catalogText } from "~/test/error-catalog-text";
import { SchoolSupervisionsView } from "./supervisions-view";

// Aktionen melden Fehler als Toast (#2517); der Provider zeigt ihn echt an.
function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  checkIn: vi.fn(),
  instances: [] as unknown[],
  roster: null as unknown,
}));

vi.mock("~/lib/school-supervisions-api", () => ({
  schoolSupervisionsApi: {
    myDay: vi.fn(),
    roster: vi.fn(),
    start: vi.fn(),
    checkIn: mocks.checkIn,
    checkOut: vi.fn(),
    patchAttendance: vi.fn(),
    complete: vi.fn(),
    studentSheet: vi.fn(),
  },
}));

// Die Liste des Tages und die Kinderliste kommen je über einen eigenen Key.
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) => {
    if (key === null) {
      return { data: undefined, isLoading: false, mutate: vi.fn() };
    }
    const data = key.startsWith("school-supervision-roster-")
      ? mocks.roster
      : mocks.instances;
    return {
      data,
      isLoading: false,
      error: undefined,
      mutate: vi.fn(() => Promise.resolve()),
    };
  },
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  }),
}));

// Die echte Kinderliste ist schwer; hier zählt nur, dass ihr Einchecken
// bei schoolSupervisionsApi.checkIn ankommt.
vi.mock("~/components/active-supervisions/timetable-roster", () => ({
  TimetableRosterContent: ({
    roster,
    onRosterAction,
  }: {
    roster: TimetableRoster;
    onRosterAction: (action: string, row: TimetableRosterRow) => unknown;
  }) => (
    <button
      type="button"
      onClick={() => void onRosterAction("check-in", roster.rows[0]!)}
    >
      Einchecken
    </button>
  ),
}));

vi.mock("./student-sheet-modal", () => ({
  StudentSheetModal: () => null,
}));

const row: TimetableRosterRow = {
  studentId: "7",
  studentName: "Emma Meyer",
  schoolClass: "1a",
  groupName: "OGS 1",
  planned: true,
  isUnplanned: false,
  currentlyPresent: false,
  visitId: null,
  status: "expected",
  substatus: null,
  note: null,
  checkedInAt: null,
  visitEntryTime: null,
  warnings: [],
  careDayStatus: "scheduled",
};

const runningInstance = {
  id: "11",
  title: "Betreuung",
  date: "2026-09-09",
  startTime: "11:30",
  endTime: "13:00",
  roomId: "3",
  roomName: "Turnhalle",
  status: "active",
  isOverdue: false,
  minutesUntilStart: -30,
  expectedStudentsCount: 1,
  presentStudentsCount: 0,
  notScheduledStudentsCount: 0,
  assignedStaffIds: [],
  isAssigned: true,
  isPrimary: true,
  isSubstitute: false,
  isAbsent: false,
  rosterPreview: [],
} as unknown as PlannedTimetableInstance;

const SCHOOL_CAPACITY_HINT = "Mehr Plätze kann die OGS freigeben.";

function codedError(code: ErrorCode, details: Record<string, unknown>) {
  return new ApiError("conflict", 409, { code, details });
}

/** The catalog text of `code` with its details filled in. */
function capacityText(code: ErrorCode, details: Record<string, unknown>) {
  const maximum =
    code === "presence.activity_participant_limit_reached"
      ? details.max_participants
      : details.max_capacity;
  const current = details.current_occupancy;
  const freeSlots =
    typeof maximum === "number" && typeof current === "number"
      ? Math.max(0, maximum - current)
      : undefined;
  return Object.entries({ ...details, free_slots: freeSlots }).reduce(
    (text, [key, value]) => text.replace(`{${key}}`, String(value)),
    catalogText(code, ""),
  );
}

async function checkInAndFindError(err: unknown, expected: string) {
  mocks.checkIn.mockRejectedValueOnce(err);
  render(<SchoolSupervisionsView />);
  fireEvent.click(screen.getByRole("button", { name: "Einchecken" }));
  const shown = await screen.findByRole("alert");
  expect(shown).toHaveTextContent(expected);
  return shown;
}

describe("SchoolSupervisionsView: Fehler beim Einchecken (#3633)", () => {
  beforeEach(() => {
    mocks.checkIn.mockReset();
    mocks.instances = [runningInstance];
    mocks.roster = {
      instance: { id: runningInstance.id },
      rows: [row],
      pickupTimesLoaded: true,
    };
  });

  it("nennt die volle Aktivität mit ihrer Belegung", async () => {
    const details = {
      activity_name: "Betreuung",
      current_occupancy: 45,
      max_participants: 45,
      incoming_students: 1,
    };
    const expected = capacityText(
      "presence.activity_participant_limit_reached",
      details,
    );
    const shown = await checkInAndFindError(
      codedError("presence.activity_participant_limit_reached", details),
      expected,
    );

    expect(shown).toBeInTheDocument();
    expect(expected).toContain("45 von 45");
    expect(expected).not.toContain("Datenverwaltung");
    expect(screen.getByRole("alert")).toHaveTextContent(SCHOOL_CAPACITY_HINT);
    expect(mocks.checkIn).toHaveBeenCalledWith("11", "7");
  });

  it("nennt den vollen Raum mit seiner Belegung", async () => {
    const details = {
      room_name: "Turnhalle",
      current_occupancy: 30,
      max_capacity: 30,
      incoming_students: 1,
    };
    const expected = capacityText("presence.room_capacity_exceeded", details);
    const shown = await checkInAndFindError(
      codedError("presence.room_capacity_exceeded", details),
      expected,
    );

    expect(shown).toBeInTheDocument();
    expect(expected).toContain("Turnhalle");
    expect(expected).not.toContain("Datenverwaltung");
    expect(screen.getByRole("alert")).toHaveTextContent(SCHOOL_CAPACITY_HINT);
  });

  it("zeigt bei anderen Fehlern den Katalogtext mit Wiederholen", async () => {
    const shown = await checkInAndFindError(
      new ApiError("Netzwerkfehler", 503, { code: "general.unavailable" }),
      catalogText("general.unavailable", "die Anwesenheit von Emma Meyer"),
    );

    expect(shown).toBeInTheDocument();
    mocks.checkIn.mockResolvedValueOnce(undefined);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await vi.waitFor(() => expect(mocks.checkIn).toHaveBeenCalledTimes(2));
  });
});
