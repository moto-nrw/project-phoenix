import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { EnrichedInstance } from "~/lib/timetable-types";
import { RoomPlanControls, RoomPlanTab } from "./room-plan-tab";

const { swrState, getWeekMock } = vi.hoisted(() => ({
  swrState: {
    key: null as string | null,
    fetcher: null as null | (() => unknown),
    data: undefined as { instances: EnrichedInstance[] } | undefined,
    error: undefined as unknown,
  },
  getWeekMock: vi.fn(),
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string, fetcher: () => unknown) => {
    swrState.key = key;
    swrState.fetcher = fetcher;
    return {
      data: swrState.data,
      error: swrState.error,
      isLoading: swrState.data === undefined && !swrState.error,
    };
  },
  useTenantMutate: () => vi.fn(),
}));

vi.mock("~/lib/timetable-api", () => ({
  timetableService: { getWeek: getWeekMock },
}));

vi.mock("~/lib/hooks/use-timetable-day-hours", () => ({
  useTimetableDayHours: () => ({ dayStartHour: 8, dayEndHour: 17 }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useWeekendFollowsFriday: () => false,
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

function block(
  id: string,
  date: string,
  startTime: string,
  endTime: string,
  extra: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  const instance: EnrichedInstance = {
    id,
    date,
    startTime,
    endTime,
    title: `Block ${id}`,
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "3",
    roomName: "Werkraum",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 0,
    absentStaffCount: 0,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    notScheduledStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
    ...extra,
  };
  return instance;
}

describe("RoomPlanTab (#4768)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrState.data = undefined;
    swrState.error = undefined;
  });

  it("loads the room's blocks for the working week", () => {
    swrState.data = { instances: [] };
    render(<RoomPlanTab roomId="3" view="woche" anchorISO="2026-09-09" />);

    expect(swrState.key).toBe("room-plan-3-2026-09-07-2026-09-11");
    void swrState.fetcher?.();
    expect(getWeekMock).toHaveBeenCalledWith("2026-09-07", "2026-09-11", {
      groupId: null,
      staffId: null,
      roomId: "3",
    });
    // Fünf Betreuungstage, jeder ganz frei.
    for (const day of ["07", "08", "09", "10", "11"]) {
      expect(
        within(screen.getByTestId(`room-plan-day-2026-09-${day}`)).getByText(
          "Frei",
        ),
      ).toBeInTheDocument();
    }
  });

  it("shows blocks, free times and cancelled blocks of a day", () => {
    swrState.data = {
      instances: [
        block("1", "2026-09-09", "12:00", "13:00", {
          title: "Mittagessen",
          groupName: "Sonnengruppe",
        }),
        block("2", "2026-09-09", "14:00", "15:00", {
          title: "Kunst-AG",
          status: "cancelled",
          cancelReason: "Krank",
        }),
      ],
    };
    render(<RoomPlanTab roomId="3" view="tag" anchorISO="2026-09-09" />);

    expect(swrState.key).toBe("room-plan-3-2026-09-09-2026-09-09");
    const day = screen.getByTestId("room-plan-day-2026-09-09");
    expect(
      within(day)
        .getAllByRole("listitem")
        .map((row) => row.textContent),
    ).toMatchInlineSnapshot(`
      [
        "08:00–12:00Frei",
        "12:00–13:00Mittagessen · Sonnengruppe",
        "13:00–17:00Frei",
        "14:00–15:00Kunst-AGAbgesagt",
      ]
    `);
    expect(screen.getByText("Abgesagt")).toHaveAttribute("title", "Krank");
  });

  it("snaps a weekend day to the next Monday", () => {
    swrState.data = { instances: [] };
    render(<RoomPlanTab roomId="3" view="tag" anchorISO="2026-09-12" />);

    expect(swrState.key).toBe("room-plan-3-2026-09-14-2026-09-14");
  });

  it("shows a load error instead of an empty plan", async () => {
    swrState.error = new Error("offline");
    render(<RoomPlanTab roomId="3" view="woche" anchorISO="2026-09-09" />);

    expect(screen.queryByTestId("room-plan")).not.toBeInTheDocument();
    expect(screen.queryByText("Frei")).not.toBeInTheDocument();
    expect(await screen.findByRole("alert")).toBeInTheDocument();
  });
});

describe("RoomPlanControls (#4768)", () => {
  it("steps by week and switches to the day", () => {
    const onChange = vi.fn();
    render(
      <RoomPlanControls
        view="woche"
        anchorISO="2026-09-09"
        onChange={onChange}
        planHref="/calendar?room=3"
        planLinkLabel="Im Betreuungsplan ansehen"
      />,
    );

    expect(screen.getByText("KW 37 · 07.09.–11.09.2026")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Nächste Woche" }));
    expect(onChange).toHaveBeenLastCalledWith({ d: "2026-09-14" });
    fireEvent.click(screen.getByRole("button", { name: "Tag" }));
    expect(onChange).toHaveBeenLastCalledWith({ view: "tag" });
    expect(
      screen.getByRole("link", { name: "Im Betreuungsplan ansehen" }),
    ).toHaveAttribute("href", "/calendar?room=3");
  });

  it("steps from Friday to Monday in the day view", () => {
    const onChange = vi.fn();
    render(
      <RoomPlanControls
        view="tag"
        anchorISO="2026-09-11"
        onChange={onChange}
        planHref={null}
        planLinkLabel=""
      />,
    );

    expect(screen.getByText("Freitag, 11.09.2026")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Nächster Tag" }));
    expect(onChange).toHaveBeenLastCalledWith({ d: "2026-09-14" });
  });
});
