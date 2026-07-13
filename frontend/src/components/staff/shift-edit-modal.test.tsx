import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ShiftEditModal } from "./shift-edit-modal";
import type { StaffShift } from "~/lib/shift-helpers";

vi.mock("~/lib/shift-api", () => ({
  ShiftApiError: class ShiftApiError extends Error {
    readonly status: number;
    readonly detail: string;

    constructor(status: number, detail: string) {
      super(detail);
      this.status = status;
      this.detail = detail;
    }
  },
  staffShiftService: {
    createShift: vi.fn(),
    updateShift: vi.fn(),
    deleteShift: vi.fn(),
  },
  staffShiftSeriesService: {
    createSeries: vi.fn(),
    splitSeries: vi.fn(),
    endSeries: vi.fn(),
  },
}));

const oneHourShift: StaffShift = {
  id: "1",
  staffId: "7",
  date: "2026-07-06",
  startTime: "08:00",
  endTime: "09:00",
  breakMinutes: 30,
  shiftTypeId: null,
  notes: "",
  seriesId: null,
  detached: false,
  cancelled: false,
  changeReason: null,
  originShiftId: null,
};

describe("ShiftEditModal", () => {
  it("caps break minutes at the selected shift duration", () => {
    render(
      <ShiftEditModal
        isOpen={true}
        mode="edit"
        staffId="7"
        staffName="Ada Lovelace"
        date="2026-07-06"
        shift={oneHourShift}
        shiftTypes={[]}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    const breakInput = screen.getByLabelText("Pause (Minuten)");
    expect(breakInput).toHaveAttribute("max", "60");

    const saveButton = screen.getByRole("button", {
      name: "Änderungen speichern",
    });
    expect(saveButton).toBeEnabled();

    fireEvent.change(breakInput, { target: { value: "61" } });
    expect(saveButton).toBeDisabled();

    fireEvent.change(breakInput, { target: { value: "60" } });
    expect(saveButton).toBeEnabled();
  });
});
