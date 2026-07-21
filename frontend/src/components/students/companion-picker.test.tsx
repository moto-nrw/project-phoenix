import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";

import { CompanionPicker } from "./companion-picker";
import type { StudentCompanion } from "~/lib/student-companion-api";

vi.mock("~/lib/student-api", () => ({
  fetchStudents: vi.fn().mockResolvedValue({ students: [] }),
}));

const LINKED: StudentCompanion[] = [
  {
    companion_student_id: "42",
    first_name: "Mia",
    last_name: "Muster",
    weekdays: ["mon", "tue"],
  },
];

describe("CompanionPicker weekday toggles", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the companion while it still has a weekday", () => {
    const onChange = vi.fn();
    render(
      <CompanionPicker
        value={LINKED}
        onChange={onChange}
        allowedDays={["mon", "tue"]}
      />,
    );

    fireEvent.click(screen.getByLabelText("Mia Muster: Dienstag"));

    expect(onChange).toHaveBeenCalledWith([
      { ...LINKED[0], weekdays: ["mon"] },
    ]);
  });

  // Unticking the last day is how a user says "not this child after all".
  // Keeping an entry with no weekday would look like a valid edit and then be
  // refused on save, with no field-level explanation anywhere in the row.
  it("drops the companion when its last weekday is unticked", () => {
    const onChange = vi.fn();
    render(
      <CompanionPicker
        value={[{ ...LINKED[0]!, weekdays: ["mon"] }]}
        onChange={onChange}
        allowedDays={["mon", "tue"]}
      />,
    );

    fireEvent.click(screen.getByLabelText("Mia Muster: Montag"));

    expect(onChange).toHaveBeenCalledWith([]);
  });

  // A day the plan does not currently allow has no checkbox in the row, so
  // ticking another one says nothing about it. It can exist because the stored
  // links were re-fetched after a remote write while the plan prop is still the
  // pre-write one — dropping it would delete somebody else's day.
  it("keeps a weekday outside the plan when another day is ticked", () => {
    const onChange = vi.fn();
    render(
      <CompanionPicker
        value={[{ ...LINKED[0]!, weekdays: ["thu"] }]}
        onChange={onChange}
        allowedDays={["mon", "tue"]}
      />,
    );

    fireEvent.click(screen.getByLabelText("Mia Muster: Montag"));

    expect(onChange).toHaveBeenCalledWith([
      { ...LINKED[0], weekdays: ["mon", "thu"] },
    ]);
  });
});

describe("CompanionPicker plan trimming", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // The links are re-fetched on their own when someone else changes them, and
  // the fresh list can name a weekday the plan prop above has not caught up
  // with yet. Trimming it away here would delete that link on the next save —
  // with the fingerprint of exactly the list it came from, so the backend's
  // baseline check would accept the deletion.
  it("does not trim a reloaded link against an unchanged plan", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <CompanionPicker value={[]} onChange={onChange} allowedDays={["mon"]} />,
    );

    rerender(
      <CompanionPicker
        value={[{ ...LINKED[0]!, weekdays: ["thu"] }]}
        onChange={onChange}
        allowedDays={["mon"]}
      />,
    );

    expect(onChange).not.toHaveBeenCalled();
  });

  it("trims links when the plan itself drops a weekday", () => {
    const onChange = vi.fn();
    const { rerender } = render(
      <CompanionPicker
        value={LINKED}
        onChange={onChange}
        allowedDays={["mon", "tue"]}
      />,
    );

    rerender(
      <CompanionPicker
        value={LINKED}
        onChange={onChange}
        allowedDays={["mon"]}
      />,
    );

    expect(onChange).toHaveBeenCalledWith([
      { ...LINKED[0], weekdays: ["mon"] },
    ]);
  });
});
