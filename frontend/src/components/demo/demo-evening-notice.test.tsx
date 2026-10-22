import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";
import { DemoEveningRow, isOutsideSchoolDay } from "./demo-evening-notice";

const TITLE = "Der Demo-Tag läuft zu Ihrer Uhrzeit";

// The notice opens by itself only after a settle delay (SETTLE_MS in
// demo-weekend-notice.tsx). Fake timers skip that wait; shouldAdvanceTime keeps
// findBy and userEvent running.
const SETTLE_MS = 1500;

describe("isOutsideSchoolDay", () => {
  it("reads 17:00 to 06:59 Berlin time as outside the school day", () => {
    expect(isOutsideSchoolDay(new Date("2026-09-09T16:59:00+02:00"))).toBe(
      false,
    );
    expect(isOutsideSchoolDay(new Date("2026-09-09T17:00:00+02:00"))).toBe(
      true,
    );
    expect(isOutsideSchoolDay(new Date("2026-09-09T06:59:00+02:00"))).toBe(
      true,
    );
    expect(isOutsideSchoolDay(new Date("2026-09-09T07:00:00+02:00"))).toBe(
      false,
    );
  });
});

describe("DemoEveningRow", () => {
  beforeEach(() => {
    globalThis.localStorage.clear();
    vi.useFakeTimers({ shouldAdvanceTime: true });
  });

  it("stays hidden during the school day", () => {
    const { unmount } = render(<DemoEveningRow inParentsApp={false} />);
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
    unmount();

    setTestClock("2026-09-09T20:30:00+02:00");
    render(<DemoEveningRow inParentsApp={false} />);
    // Anker: am Abend desselben Tages erscheint die Zeile im selben Aufbau.
    expect(screen.getByRole("note")).toBeInTheDocument();
  });

  it("shows on a weekend evening too, but leaves the dialog to the weekend notice", async () => {
    setTestClock("2026-09-26T20:00:00+02:00");
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    render(<DemoEveningRow inParentsApp={false} />);
    expect(screen.getByRole("note")).toHaveTextContent(
      "Uhrzeiten heute verschoben.",
    );
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000);
    });
    expect(screen.queryByText(TITLE)).not.toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Warum?" }));
    expect(await screen.findByText(TITLE)).toBeInTheDocument();
  });

  it("explains the moved times once per weekday evening", async () => {
    setTestClock("2026-09-09T20:30:00+02:00");
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime });
    const { unmount } = render(<DemoEveningRow inParentsApp={false} />);
    act(() => {
      vi.advanceTimersByTime(SETTLE_MS);
    });

    expect(
      await screen.findByText(TITLE, {}, { timeout: 4000 }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Deshalb stehen Blöcke und Abholzeiten heute zu ungewohnten Uhrzeiten.",
      ),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "Verstanden" }));
    unmount();

    // Next page view the same evening: no dialog, but the button reopens it.
    render(<DemoEveningRow inParentsApp={false} />);
    expect(screen.queryByText(TITLE)).not.toBeInTheDocument();
    // The row stays in view and says what is going on without a click.
    expect(screen.getByRole("note")).toHaveTextContent(
      "Uhrzeiten heute verschoben.",
    );
    await user.click(screen.getByRole("button", { name: "Warum?" }));
    expect(await screen.findByText(TITLE)).toBeInTheDocument();
  });

  it("speaks about the own child in the parents app at night", async () => {
    setTestClock("2026-09-10T05:00:00+02:00");
    render(<DemoEveningRow inParentsApp />);
    act(() => {
      vi.advanceTimersByTime(SETTLE_MS);
    });

    expect(
      await screen.findByText(
        "Deshalb hat Ihr Kind heute ungewohnte Abholzeiten.",
        {},
        { timeout: 4000 },
      ),
    ).toBeInTheDocument();
  });
});
