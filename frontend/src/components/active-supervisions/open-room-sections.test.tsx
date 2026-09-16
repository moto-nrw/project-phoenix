import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import {
  OpenRoomSections,
  type OpenRoomBlockContext,
} from "./open-room-sections";
import { openRoomSections, type OpenRoomSessionView } from "./view-model";

const rosters = vi.hoisted(() => new Map<string, unknown>());

vi.mock("~/lib/swr", () => ({
  useSWRAuth: vi.fn((key: string | null) => ({
    data: key ? rosters.get(key) : undefined,
    error: undefined,
    isLoading: false,
    mutate: vi.fn(),
  })),
}));

function row(overrides: Partial<TimetableRosterRow>): TimetableRosterRow {
  return {
    studentId: "1",
    studentName: "Marie Muster",
    schoolClass: "2b",
    groupName: "Sonnengruppe",
    planned: true,
    isUnplanned: false,
    currentlyPresent: false,
    visitId: null,
    status: "expected",
    substatus: null,
    note: null,
    checkedInAt: null,
    checkedOutAt: null,
    visitEntryTime: null,
    pickupTime: null,
    warnings: [],
    careDayStatus: "scheduled",
    parallelPresentIn: null,
    ...overrides,
  };
}

function roster(instanceId: string, canOperate: boolean): TimetableRoster {
  return {
    instance: {
      id: instanceId,
      title: `Block ${instanceId}`,
      status: "active",
      isSpontaneous: false,
      activeGroupId: instanceId,
      roomId: "schulhof",
      roomName: "Schulhof",
      date: "2026-09-09",
      startTime: "11:00",
      endTime: "11:45",
      canComplete: true,
      completeAvailableAt: "2026-09-09T09:00:00Z",
    },
    rows: [
      row({ studentId: `${instanceId}-1` }),
      row({
        studentId: `${instanceId}-2`,
        studentName: "Ben Beispiel",
        currentlyPresent: true,
        status: "present",
        visitId: `${instanceId}-visit`,
      }),
    ],
    pickupTimesLoaded: true,
    canOperate,
  };
}

function block(
  id: string,
  options: { own?: boolean; planned?: boolean; canOperate: boolean },
): OpenRoomSessionView {
  return {
    activeGroupId: id,
    title: `GT ${id}`,
    independent: false,
    isUserSupervising: options.own === true,
    canAssign: options.own === true,
    studentCount: 1,
    block: {
      instanceId: id,
      startTime: "11:00",
      endTime: "11:45",
      isUserAssigned: options.planned === true,
      canOperate: options.canOperate,
    },
  };
}

function context(overviewEnabled: boolean): OpenRoomBlockContext {
  return {
    allRooms: [],
    currentStaffId: "staff-1",
    mutateDashboard: vi.fn(),
    refresh: vi.fn(),
    adoptSession: vi.fn(() => "/active-supervisions"),
    setSelectedTimetableInstanceId: vi.fn(),
    setError: vi.fn(),
    router: { push: vi.fn() },
    reopenableInstanceId: null,
    rememberReopenable: vi.fn(),
    clearReopenable: vi.fn(),
    attendanceWebEnabled: true,
    showTimetableCounts: false,
    canExcuseRestOfDay: false,
    overviewEnabled,
    onAddSupervisor: vi.fn(),
  };
}

function renderRoom(
  sessions: readonly OpenRoomSessionView[],
  overviewEnabled = true,
) {
  const sections = openRoomSections({ sessions });
  if (!sections) throw new Error("the room has blocks");
  render(
    <OpenRoomSections
      sections={sections}
      students={[]}
      filteredStudents={[]}
      grid={{
        pickupTimesData: undefined,
        arrivalTimesData: undefined,
        trackingData: undefined,
        myGroupIds: [],
        myGroupRooms: [],
        now: new Date(),
        onOpenStudent: vi.fn(),
      }}
      blocks={context(overviewEnabled)}
    />,
  );
}

const BLOCK_ACTIONS = [
  "Einchecken",
  "Raum verlassen",
  "Erwartete bestätigen",
  "Kind hinzufügen",
  "Beenden",
];

describe("OpenRoomSections (#3281)", () => {
  beforeEach(() => {
    rosters.clear();
    rosters.set("timetable-roster-own", roster("own", true));
    rosters.set("timetable-roster-planned", roster("planned", true));
    rosters.set("timetable-roster-foreign", roster("foreign", false));
  });

  it("gives a caregiver with two blocks in the room every action in both", () => {
    renderRoom([
      block("foreign", { canOperate: false }),
      block("own", { own: true, canOperate: true }),
      block("planned", { planned: true, canOperate: true }),
    ]);

    for (const name of BLOCK_ACTIONS) {
      expect(screen.getAllByRole("button", { name })).toHaveLength(2);
    }
    expect(
      screen
        .getAllByRole("heading", { name: /^Block / })
        .map((h) => h.textContent),
    ).toEqual(["Block own", "Block planned"]);
    expect(screen.getByRole("heading", { name: "GT foreign" })).toBeVisible();
    expect(
      screen.queryByText(/Sie sind für diese Aktivität nicht eingeplant/),
    ).not.toBeInTheDocument();
  });

  it("shows a foreign block read-only once opened", () => {
    renderRoom([
      block("own", { own: true, canOperate: true }),
      block("foreign", { canOperate: false }),
    ]);

    fireEvent.click(
      screen.getByRole("button", { name: "GT foreign ausklappen" }),
    );

    const foreign = screen.getByRole("group", { name: "GT foreign" });
    expect(
      within(foreign).getByRole("heading", { name: "Block foreign" }),
    ).toBeInTheDocument();
    expect(
      within(foreign).getByText(
        /Sie sind für diese Aktivität nicht eingeplant/,
      ),
    ).toBeInTheDocument();
    for (const name of BLOCK_ACTIONS) {
      expect(
        within(foreign).queryByRole("button", { name }),
      ).not.toBeInTheDocument();
    }
    // The own block keeps its actions next to it.
    expect(screen.getAllByRole("button", { name: "Beenden" })).toHaveLength(1);

    fireEvent.click(
      within(foreign).getByRole("button", { name: "GT foreign einklappen" }),
    );
    expect(
      screen.queryByRole("heading", { name: "Block foreign" }),
    ).not.toBeInTheDocument();
  });

  it("lets an admin operate a block that is not their own", () => {
    rosters.set("timetable-roster-foreign", roster("foreign", true));
    renderRoom([block("foreign", { canOperate: true })], false);

    fireEvent.click(
      screen.getByRole("button", { name: "GT foreign ausklappen" }),
    );

    for (const name of BLOCK_ACTIONS) {
      expect(screen.getByRole("button", { name })).toBeInTheDocument();
    }
  });
});
