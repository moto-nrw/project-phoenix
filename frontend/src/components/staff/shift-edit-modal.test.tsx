import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ShiftEditModal } from "./shift-edit-modal";
import { staffShiftService } from "~/lib/shift-api";
import type { StaffShift } from "~/lib/shift-helpers";

vi.mock("~/lib/hooks/use-closing-days", () => ({
  useClosingDaysState: () => ({
    closingDays: new Map(),
    closingDayRanges: [],
    isLoading: false,
  }),
}));

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
    applyCancellation: vi.fn(),
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
  shiftTypeName: null,
  shiftTypeColor: null,
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

  it("keeps existing covers when the cancel toggle is un- and re-checked (#1841)", async () => {
    const applyCancellation = vi.mocked(staffShiftService.applyCancellation);
    applyCancellation.mockClear();
    applyCancellation.mockResolvedValue(undefined as never);

    const cancelledShift: StaffShift = {
      ...oneHourShift,
      id: "5",
      startTime: "08:00",
      endTime: "16:00",
      cancelled: true,
    };
    const existingCover: StaffShift = {
      ...oneHourShift,
      id: "11",
      staffId: "9",
      startTime: "08:00",
      endTime: "12:00",
      breakMinutes: 0,
      originShiftId: "5",
    };

    render(
      <ShiftEditModal
        isOpen={true}
        mode="edit"
        staffId="7"
        staffName="Ada Lovelace"
        date="2026-07-06"
        shift={cancelledShift}
        shiftTypes={[]}
        staffOptions={[{ id: "9", firstName: "Grace", lastName: "Hopper" }]}
        existingReplacements={[existingCover]}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    const toggle = screen.getByLabelText("Diese Schicht fällt aus");
    expect(toggle).toBeChecked();

    // Reactivate then cancel again before saving: the cover must survive the
    // round-trip instead of being wiped locally.
    fireEvent.click(toggle);
    expect(toggle).not.toBeChecked();
    fireEvent.click(toggle);
    expect(toggle).toBeChecked();

    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => expect(applyCancellation).toHaveBeenCalledTimes(1));
    expect(applyCancellation).toHaveBeenCalledWith(
      "5",
      expect.objectContaining({
        cancelled: true,
        replacements: [
          expect.objectContaining({
            staffId: "9",
            startTime: "08:00",
            endTime: "12:00",
          }),
        ],
      }),
    );
  });

  it("clamps a cover's preserved break when its window is shortened below it (#1841)", async () => {
    const applyCancellation = vi.mocked(staffShiftService.applyCancellation);
    applyCancellation.mockClear();
    applyCancellation.mockResolvedValue(undefined as never);

    const cancelledShift: StaffShift = {
      ...oneHourShift,
      id: "5",
      startTime: "08:00",
      endTime: "16:00",
      cancelled: true,
    };
    // A cover edited directly in the grid carries a 60-minute break; this modal
    // has no break field to reveal or correct it.
    const existingCover: StaffShift = {
      ...oneHourShift,
      id: "11",
      staffId: "9",
      startTime: "08:00",
      endTime: "16:00",
      breakMinutes: 60,
      originShiftId: "5",
    };

    render(
      <ShiftEditModal
        isOpen={true}
        mode="edit"
        staffId="7"
        staffName="Ada Lovelace"
        date="2026-07-06"
        shift={cancelledShift}
        shiftTypes={[]}
        staffOptions={[{ id: "9", firstName: "Grace", lastName: "Hopper" }]}
        existingReplacements={[existingCover]}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    // Shorten the cover to 30 minutes — below its hidden 60-minute break.
    fireEvent.change(screen.getByLabelText("Vertretung 1 Ende"), {
      target: { value: "08:30" },
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    // The submitted break is clamped to the new duration, so the backend cannot
    // reject the save with a break-longer-than-shift error.
    await waitFor(() => expect(applyCancellation).toHaveBeenCalledTimes(1));
    expect(applyCancellation).toHaveBeenCalledWith(
      "5",
      expect.objectContaining({
        replacements: [
          expect.objectContaining({
            staffId: "9",
            endTime: "08:30",
            breakMinutes: 30,
          }),
        ],
      }),
    );
  });
});
