import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it } from "vitest";
import { setTestClock } from "~/test/clock";
import { DemoEveningRow, isOutsideSchoolDay } from "./demo-evening-notice";

const TITLE = "Der Demo-Tag läuft zu Ihrer Uhrzeit";

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
  });

  it("stays hidden during the school day", () => {
    render(<DemoEveningRow inParentsApp={false} />);
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
  });

  it("shows on a weekend evening too, which runs on Friday's plan", () => {
    setTestClock("2026-09-26T20:00:00+02:00");
    render(<DemoEveningRow inParentsApp={false} />);
    expect(screen.getByRole("note")).toHaveTextContent(
      "Uhrzeiten heute verschoben.",
    );
  });

  it("explains the moved times once per weekday evening", async () => {
    setTestClock("2026-09-09T20:30:00+02:00");
    const user = userEvent.setup();
    const { unmount } = render(<DemoEveningRow inParentsApp={false} />);

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

    expect(
      await screen.findByText(
        "Deshalb hat Ihr Kind heute ungewohnte Abholzeiten.",
        {},
        { timeout: 4000 },
      ),
    ).toBeInTheDocument();
  });
});
