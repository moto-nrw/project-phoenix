import { fireEvent, render, screen } from "@testing-library/react";
import { afterAll, beforeAll, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { DatePicker } from "~/components/ui/date-picker";
import { preloadDayPicker } from "~/components/ui/lazy-day-picker";
import {
  berlinTodayISO,
  isoWeekNumber,
  parseISODate,
} from "~/lib/date-helpers";
import { useBerlinToday } from "~/lib/hooks/use-berlin-today";

import {
  TEST_CLOCK_INSTANT,
  TEST_CLOCK_TODAY,
  isTestClockFrozen,
  releaseFakeTimers,
  setTestClock,
  useRealClock,
} from "./clock";

// The regression from #3100: the domain hook is pinned to one day while the
// calendar library reads the clock on its own.
vi.mock("~/lib/hooks/use-berlin-today", () => ({
  useBerlinToday: () => "2026-09-08",
}));

function ClockProbe() {
  const hookToday = useBerlinToday();
  return (
    <div>
      <output data-testid="hook-today">{hookToday}</output>
      <output data-testid="component-now">{new Date().toISOString()}</output>
      <output data-testid="component-date-now">{Date.now()}</output>
      <DatePicker
        calendarLayout="inline"
        value={parseISODate("2026-09-08")}
        onChange={() => undefined}
      />
    </div>
  );
}

// The kit loads the calendar grid lazily; load it once up front so a cold
// import on a busy machine does not race the findBy timeout.
beforeAll(() => preloadDayPicker());

describe("deterministic test clock", () => {
  it("freezes Date for the rendered component tree, not only for the mocked hook", () => {
    render(<ClockProbe />);

    expect(screen.getByTestId("hook-today")).toHaveTextContent("2026-09-08");
    expect(screen.getByTestId("component-now")).toHaveTextContent(
      TEST_CLOCK_INSTANT.toISOString(),
    );
    expect(screen.getByTestId("component-date-now")).toHaveTextContent(
      String(TEST_CLOCK_INSTANT.getTime()),
    );
    expect(berlinTodayISO()).toBe(TEST_CLOCK_TODAY);
  });

  it("keeps explicit Berlin calendar today independent of the frozen Date clock", async () => {
    render(<ClockProbe />);
    fireEvent.click(screen.getByRole("button", { name: "08.09.2026" }));

    // #5379: the kit now supplies the school's day explicitly. Mocking that
    // hook must affect the calendar marker, but must not unfreeze Date or
    // silently change its instant. The selected day is also the supplied day.
    expect(
      await screen.findByRole("button", {
        name: "Today, Dienstag, 8. September 2026, selected",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", {
        name: "Mittwoch, 9. September 2026",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^Today, Mittwoch/ }),
    ).not.toBeInTheDocument();
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime());
  });

  it("moves time with setTestClock for Berlin midnight and the DST switch", () => {
    setTestClock("2026-03-28T23:30:00+01:00");
    expect(berlinTodayISO()).toBe("2026-03-28");

    setTestClock("2026-03-29T00:30:00+01:00");
    expect(berlinTodayISO()).toBe("2026-03-29");
    expect(new Date().getHours()).toBe(0);

    // 02:00 CET does not exist on 29 March 2026; the clock jumps to 03:00 CEST.
    setTestClock("2026-03-29T01:59:59+01:00");
    expect(new Date().getTimezoneOffset()).toBe(-60);
    setTestClock("2026-03-29T03:00:00+02:00");
    expect(new Date().getTimezoneOffset()).toBe(-120);
    expect(new Date().getHours()).toBe(3);
  });

  it("supports a week change on the frozen clock", () => {
    setTestClock("2026-09-13T23:59:59+02:00");
    expect(isoWeekNumber(berlinTodayISO())).toBe(37);

    setTestClock("2026-09-14T00:00:00+02:00");
    expect(isoWeekNumber(berlinTodayISO())).toBe(38);
  });

  it("starts fake timers at the frozen instant and keeps the clock when releasing them", async () => {
    vi.useFakeTimers();
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime());

    vi.advanceTimersByTime(60 * 60 * 1000);
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime() + 60 * 60 * 1000);

    releaseFakeTimers();
    expect(vi.isFakeTimers()).toBe(false);
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime() + 60 * 60 * 1000);

    // Real timers run again while the clock stays frozen.
    const elapsed = await new Promise<number>((resolve) => {
      const start = Date.now();
      setTimeout(() => resolve(Date.now() - start), 5);
    });
    expect(elapsed).toBe(0);
  });

  it("does not freeze async timers by default", async () => {
    expect(vi.isFakeTimers()).toBe(false);

    const ticked = await new Promise<boolean>((resolve) => {
      setTimeout(() => resolve(true), 1);
    });
    expect(ticked).toBe(true);
  });
});

describe("isolation between tests", () => {
  it("leaves a changed clock and installed fake timers behind", () => {
    setTestClock("2030-01-01T00:00:00+01:00");
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2031-06-15T10:00:00+02:00"));
    expect(berlinTodayISO()).toBe("2031-06-15");
  });

  it("starts the next test on the default clock with real timers", () => {
    expect(vi.isFakeTimers()).toBe(false);
    expect(isTestClockFrozen()).toBe(true);
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime());
    expect(berlinTodayISO()).toBe(TEST_CLOCK_TODAY);
  });

  it("hands a reviewed exception the host clock", () => {
    useRealClock("proves that the exception leaves the frozen clock");

    expect(isTestClockFrozen()).toBe(false);
  });

  it("re-freezes the clock after a real-clock exception", () => {
    expect(isTestClockFrozen()).toBe(true);
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime());
  });
});

describe("isolation with nested fake-timer scopes", () => {
  describe("with file-scoped fake timers", () => {
    beforeAll(() => {
      vi.useFakeTimers();
    });

    afterAll(() => {
      vi.useRealTimers();
    });

    it("can release the file's fake-timer mode after changing time", () => {
      expect(vi.isFakeTimers()).toBe(true);
      setTestClock("2031-06-15T10:00:00+02:00");
      expect(Date.now()).toBe(new Date("2031-06-15T10:00:00+02:00").getTime());
      releaseFakeTimers();
      expect(vi.isFakeTimers()).toBe(false);
    });

    it("restores fake timers and the clock for the next scoped test", () => {
      expect(vi.isFakeTimers()).toBe(true);
      expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime());
    });
  });

  it("leaves fake timers disabled outside the nested scope", () => {
    expect(vi.isFakeTimers()).toBe(false);
    expect(Date.now()).toBe(TEST_CLOCK_INSTANT.getTime());
  });
});
