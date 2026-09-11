import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { berlinTodayISO } from "~/lib/date-helpers";
import type { StudentStatusDay } from "~/lib/student-status-days-api";
import { useSWRAuth } from "~/lib/swr/hooks";

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

  it("shows an error alert when the fetch fails", () => {
    setSWR({ data: undefined, isLoading: false, error: new Error("Boom") });
    render(<CarePlanView studentId="1" statusDays={[]} />);
    expect(screen.getByText("Boom")).toBeInTheDocument();
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
});
