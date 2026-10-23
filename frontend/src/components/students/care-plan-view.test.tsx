import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { berlinTodayISO } from "~/lib/date-helpers";
import type { StudentStatusDay } from "~/lib/student-status-days-api";
import { useSWRAuth } from "~/lib/swr/hooks";
import { useWeekendFollowsFriday } from "~/lib/tenant-context";
import { setTestClock } from "~/test/clock";

import { CarePlanView } from "./care-plan-view";

// useSWRAuth is NOT globally mocked (only the raw `swr` package is) — mock the
// exact subpath the component imports so the day/week fetch is driven per test.
vi.mock("~/lib/swr/hooks", () => ({
  useSWRAuth: vi.fn(),
}));

// Keep the fetchers inert — useSWRAuth is mocked, so they never actually run.
vi.mock("~/lib/student-care-plan-api", () => ({
  fetchStudentCarePlanDay: vi.fn(),
  fetchStudentCarePlanWeek: vi.fn(),
}));

const mockDay = {
  studentId: "1",
  date: berlinTodayISO(),
  weekday: 3,
  arrival: { expectedTime: "08:00", source: "schedule" as const },
  instances: [],
  pickup: { expectedTime: "15:30", source: "schedule" as const },
};

const mockWeek = {
  studentId: "1",
  from: "2026-07-20",
  to: "2026-07-24",
  days: [],
};

type SWRState = { data?: unknown; isLoading: boolean; error: unknown };

/** Drive the two useSWRAuth call sites by key (day vs week vs inactive). */
function setSWR(
  day: SWRState,
  week: SWRState = { isLoading: false, error: null },
) {
  vi.mocked(useSWRAuth).mockImplementation((key) => {
    if (key === null) {
      return {
        data: undefined,
        isLoading: false,
        error: null,
        mutate: vi.fn(),
      } as never;
    }
    const state = String(key).startsWith("care-plan-week") ? week : day;
    return { ...state, mutate: vi.fn() } as never;
  });
}

// Radix activates a tab on pointer-down; a plain click does nothing in jsdom.
const selectTab = (name: string) => {
  const tab = screen.getByRole("button", { name });
  fireEvent.pointerDown(tab, { button: 0, pointerType: "mouse" });
  fireEvent.click(tab);
};

describe("CarePlanView", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(false);
    setSWR({ data: mockDay, isLoading: false, error: null });
  });

  it("renders the day timeline from the day fetch", () => {
    render(<CarePlanView studentId="1" statusDays={[]} />);
    expect(screen.getByText("Ankunft")).toBeInTheDocument();
    expect(screen.getByText("Abholung")).toBeInTheDocument();
    expect(screen.getByText("Freie Betreuung")).toBeInTheDocument();
  });

  it("shows the loading state while fetching", () => {
    setSWR({ data: undefined, isLoading: true, error: null });
    render(<CarePlanView studentId="1" statusDays={[]} />);
    expect(screen.getByText("Betreuungsplan wird geladen")).toBeInTheDocument();
  });

  it("shows a failed load in place with the catalog text and a retry", async () => {
    const mutate = vi.fn();
    const error = new ApiError("Boom", 503, { code: "general.unavailable" });
    vi.mocked(useSWRAuth).mockImplementation(
      (key) =>
        (key === null
          ? { data: undefined, isLoading: false, error: null, mutate }
          : { data: undefined, isLoading: false, error, mutate }) as never,
    );
    render(<CarePlanView studentId="1" statusDays={[]} />);
    expect(
      await screen.findByText(
        "Die Ansicht des Betreuungsplans ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Boom")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mutate).toHaveBeenCalled();
  });

  it("switches to the week view via the Woche tab", () => {
    setSWR(
      { data: mockDay, isLoading: false, error: null },
      { data: mockWeek, isLoading: false, error: null },
    );
    render(<CarePlanView studentId="1" statusDays={[]} />);
    // Day nav present initially.
    expect(
      screen.getByRole("button", { name: "Nächster Tag" }),
    ).toBeInTheDocument();

    selectTab("Woche");

    // Week nav replaces day nav.
    expect(
      screen.getByRole("button", { name: "Vorherige Woche" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Nächste Woche" }),
    ).toBeInTheDocument();
  });

  it("overlays a Krank deviation for today from statusDays", () => {
    const statusDays: StudentStatusDay[] = [
      {
        id: "1",
        student_id: "1",
        date: berlinTodayISO(),
        status: "sick",
        label: "Krank",
        reported_at: "",
        cleared_at: null,
        source: "manual",
        note: null,
        created_at: "",
        updated_at: "",
      },
    ];
    render(<CarePlanView studentId="1" statusDays={statusDays} />);
    expect(screen.getByText("Krank")).toBeInTheDocument();
  });

  it("does not fetch or report a range while the tab is inactive", () => {
    const onVisibleDateRangeChange = vi.fn();
    render(
      <CarePlanView
        studentId="1"
        statusDays={[]}
        active={false}
        onVisibleDateRangeChange={onVisibleDateRangeChange}
      />,
    );
    // No data fetched -> empty state, and the parent is not asked to widen.
    expect(
      screen.getByText("Kein Betreuungsplan für diesen Tag."),
    ).toBeInTheDocument();
    expect(onVisibleDateRangeChange).not.toHaveBeenCalled();
  });

  it("reports the visible range so the parent can widen its status-day fetch", () => {
    setSWR(
      { data: mockDay, isLoading: false, error: null },
      { data: mockWeek, isLoading: false, error: null },
    );
    const onVisibleDateRangeChange = vi.fn();
    render(
      <CarePlanView
        studentId="1"
        statusDays={[]}
        onVisibleDateRangeChange={onVisibleDateRangeChange}
      />,
    );
    // Day mode reports the single selected day.
    expect(onVisibleDateRangeChange).toHaveBeenCalledWith(
      berlinTodayISO(),
      berlinTodayISO(),
    );

    selectTab("Woche");

    // Week mode reports the Mon–Fri span (from < to, both ISO dates).
    const last = onVisibleDateRangeChange.mock.calls.at(-1);
    expect(last?.[0]).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(last?.[1]).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    expect(String(last?.[0]) < String(last?.[1])).toBe(true);
  });

  it("includes Saturday and Sunday in the week view when the weekend follows Friday", () => {
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(true);
    setTestClock(new Date("2026-09-09T12:00:00+02:00"));
    setSWR(
      { data: mockDay, isLoading: false, error: null },
      { data: mockWeek, isLoading: false, error: null },
    );
    const onVisibleDateRangeChange = vi.fn();
    render(
      <CarePlanView
        studentId="1"
        statusDays={[]}
        onVisibleDateRangeChange={onVisibleDateRangeChange}
      />,
    );

    selectTab("Woche");

    expect(onVisibleDateRangeChange).toHaveBeenLastCalledWith(
      "2026-09-07",
      "2026-09-13",
    );
    expect(
      vi
        .mocked(useSWRAuth)
        .mock.calls.some(
          ([key]) => key === "care-plan-week-1-2026-09-07-2026-09-13",
        ),
    ).toBe(true);
    expect(screen.getAllByText("Sa")).not.toHaveLength(0);
    expect(screen.getAllByText("So")).not.toHaveLength(0);
  });

  it("selects the current weekend day after the school setting loads", async () => {
    setTestClock(new Date("2026-09-12T12:00:00+02:00"));
    setSWR(
      { data: mockDay, isLoading: false, error: null },
      { data: mockWeek, isLoading: false, error: null },
    );
    const view = render(<CarePlanView studentId="1" statusDays={[]} />);

    selectTab("Woche");
    expect(screen.queryByText("Sa")).not.toBeInTheDocument();

    vi.mocked(useWeekendFollowsFriday).mockReturnValue(true);
    view.rerender(<CarePlanView studentId="1" statusDays={[]} />);

    await waitFor(() => {
      const saturday = screen
        .getAllByRole("button")
        .find((button) => button.textContent === "Sa12.09.");
      expect(saturday).toHaveClass("bg-gray-900");
    });
  });

  it.each([
    [false, "2026-09-14"],
    [true, "2026-09-12"],
  ])(
    "steps from Friday to the next care day (weekend follows Friday: %s, #3921)",
    (weekendOpen, next) => {
      vi.mocked(useWeekendFollowsFriday).mockReturnValue(weekendOpen);
      setTestClock(new Date("2026-09-11T12:00:00+02:00"));
      const onVisibleDateRangeChange = vi.fn();
      render(
        <CarePlanView
          studentId="1"
          statusDays={[]}
          onVisibleDateRangeChange={onVisibleDateRangeChange}
        />,
      );

      fireEvent.click(screen.getByRole("button", { name: "Nächster Tag" }));
      expect(onVisibleDateRangeChange).toHaveBeenLastCalledWith(next, next);
      vi.mocked(useWeekendFollowsFriday).mockReturnValue(false);
    },
  );
});
