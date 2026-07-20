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
});
