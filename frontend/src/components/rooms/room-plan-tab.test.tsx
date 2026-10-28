import "@testing-library/jest-dom/vitest";
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { RoomPlanBlock } from "~/lib/room-plan";
import { setTestClock } from "~/test/clock";
import { RoomPlanControls, RoomPlanTab } from "./room-plan-tab";

const { swrState, fetchBlocksMock, mockWeekendFollowsFriday } = vi.hoisted(
  () => ({
    swrState: {
      key: null as string | null,
      fetcher: null as null | (() => unknown),
      data: undefined as RoomPlanBlock[] | undefined,
      error: undefined as unknown,
    },
    fetchBlocksMock: vi.fn(),
    mockWeekendFollowsFriday: vi.fn(() => false),
  }),
);

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

vi.mock("~/lib/room-plan-api", () => ({
  fetchRoomPlanBlocks: fetchBlocksMock,
}));

vi.mock("~/lib/hooks/use-timetable-day-hours", () => ({
  useTimetableDayHours: () => ({
    dayStartHour: 8,
    dayEndHour: 17,
    dayStartTime: "08:00",
    dayEndTime: "17:00",
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useWeekendFollowsFriday: mockWeekendFollowsFriday,
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

function block(
  id: string,
  date: string,
  startTime: string,
  endTime: string,
  extra: Partial<RoomPlanBlock> = {},
): RoomPlanBlock {
  return {
    id,
    date,
    startTime,
    endTime,
    title: `Block ${id}`,
    status: "planned",
    ...extra,
  };
}

describe("RoomPlanTab (#4768)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockWeekendFollowsFriday.mockReturnValue(false);
    swrState.data = undefined;
    swrState.error = undefined;
  });

  it("loads the room's blocks for the working week", () => {
    swrState.data = [];
    render(<RoomPlanTab roomId="3" view="woche" anchorISO="2026-09-09" />);

    expect(swrState.key).toBe("room-plan-3-2026-09-07-2026-09-11");
    void swrState.fetcher?.();
    expect(fetchBlocksMock).toHaveBeenCalledWith(
      "3",
      "2026-09-07",
      "2026-09-11",
    );
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
    swrState.data = [
      block("1", "2026-09-09", "12:00", "13:00", {
        title: "Mittagessen",
        groupName: "Sonnengruppe",
      }),
      block("2", "2026-09-09", "14:00", "15:00", {
        title: "Kunst-AG",
        status: "cancelled",
        cancelReason: "Krank",
      }),
    ];
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
    swrState.data = [];
    render(<RoomPlanTab roomId="3" view="tag" anchorISO="2026-09-12" />);

    expect(swrState.key).toBe("room-plan-3-2026-09-14-2026-09-14");
  });

  it("extends an open week after Berlin midnight", () => {
    mockWeekendFollowsFriday.mockReturnValue(true);
    setTestClock("2026-09-11T23:59:00+02:00"); // Friday
    vi.useFakeTimers();
    swrState.data = [];
    render(<RoomPlanTab roomId="3" view="woche" anchorISO="2026-09-11" />);

    expect(swrState.key).toBe("room-plan-3-2026-09-07-2026-09-11");

    act(() => {
      vi.advanceTimersByTime(60_000);
    });

    expect(swrState.key).toBe("room-plan-3-2026-09-07-2026-09-13");
    expect(screen.getByTestId("room-plan-day-2026-09-12")).toBeInTheDocument();
    expect(screen.getByTestId("room-plan-day-2026-09-13")).toBeInTheDocument();
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
