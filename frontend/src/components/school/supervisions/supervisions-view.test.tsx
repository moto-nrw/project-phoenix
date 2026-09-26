import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  PlannedTimetableInstance,
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { SchoolSupervisionsView } from "./supervisions-view";

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

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message, type }: { message: string; type: string }) => (
    <div data-testid={`alert-${type}`}>{message}</div>
  ),
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

const OGS_HINT = "Mehr Plätze kann die OGS freigeben.";

function codedError(code: string, details: Record<string, unknown>) {
  return Object.assign(new Error("conflict"), { code, details });
}

async function checkInAndReadError(err: unknown): Promise<string> {
  mocks.checkIn.mockRejectedValueOnce(err);
  render(<SchoolSupervisionsView />);
  fireEvent.click(screen.getByRole("button", { name: "Einchecken" }));
  const alert = await screen.findByTestId("alert-error");
  return alert.textContent ?? "";
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

  it("nennt die volle Aktivität und verweist auf die OGS", async () => {
    const text = await checkInAndReadError(
      codedError("presence.activity_participant_limit_reached", {
        activity_name: "Betreuung",
        current_occupancy: 45,
        max_participants: 45,
        incoming_students: 1,
      }),
    );

    expect(text).toBe(
      `Die Aktivität „Betreuung“ ist voll (45 von 45 Kindern). ${OGS_HINT}`,
    );
    expect(text).not.toContain("Datenverwaltung");
    expect(mocks.checkIn).toHaveBeenCalledWith("11", "7");
  });

  it("nennt den vollen Raum und verweist auf die OGS", async () => {
    const text = await checkInAndReadError(
      codedError("presence.room_capacity_exceeded", {
        room_name: "Turnhalle",
        current_occupancy: 30,
        max_capacity: 30,
        incoming_students: 1,
      }),
    );

    expect(text).toBe(
      `Der Raum „Turnhalle“ ist voll (30 von 30 Plätzen). ${OGS_HINT}`,
    );
    expect(text).not.toContain("Datenverwaltung");
  });

  it("zeigt bei anderen Fehlern den allgemeinen Text", async () => {
    const text = await checkInAndReadError(new Error("Netzwerkfehler"));

    expect(text).toBe(
      "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.",
    );
  });
});
