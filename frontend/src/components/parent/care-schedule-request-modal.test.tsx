import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  CareScheduleRequestModal,
  type CareScheduleInitialValues,
} from "./care-schedule-request-modal";

// The shared Modal renders via createPortal to document.body, so query the
// document for the form fields rather than the render container.
function timeInputs(): HTMLInputElement[] {
  return Array.from(
    document.querySelectorAll<HTMLInputElement>('input[type="time"]'),
  );
}

// Field order per weekday row is arrival, then pickup; rows run Mon..Fri.
const MON_ARRIVAL = 0;
const MON_PICKUP = 1;

const initial: CareScheduleInitialValues = {
  1: { mode: "pickup", arrival: "08:00", pickup: "15:00" },
  2: { mode: "", arrival: "08:00", pickup: "15:00" },
  3: { mode: "", arrival: "", pickup: "" },
  4: { mode: "", arrival: "", pickup: "" },
  5: { mode: "", arrival: "", pickup: "" },
};

describe("CareScheduleRequestModal", () => {
  it("prefills the per-day rows from initialValues", () => {
    render(
      <CareScheduleRequestModal
        initialValues={initial}
        onClose={vi.fn()}
        onSubmit={vi.fn().mockResolvedValue(undefined)}
      />,
    );
    const inputs = timeInputs();
    expect(inputs[MON_ARRIVAL]).toHaveValue("08:00");
    expect(inputs[MON_PICKUP]).toHaveValue("15:00");
  });

  it("submits only weekday fields that differ from the initial values", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(
      <CareScheduleRequestModal
        initialValues={initial}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );
    // Change only Monday's pickup 15:00 -> 16:00; everything else stays as-is.
    fireEvent.change(timeInputs()[MON_PICKUP]!, { target: { value: "16:00" } });
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith({
      weekdays: [{ weekday: 1, pickup: "16:00" }],
    });
  });

  it("does not submit and warns when nothing changed from the current plan", () => {
    const onSubmit = vi.fn();
    render(
      <CareScheduleRequestModal
        initialValues={initial}
        onClose={vi.fn()}
        onSubmit={onSubmit}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    expect(onSubmit).not.toHaveBeenCalled();
    expect(
      screen.getByText("Bitte mindestens einen Tag anpassen."),
    ).toBeInTheDocument();
  });

  it("treats every filled field as a change when no initialValues are given", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<CareScheduleRequestModal onClose={vi.fn()} onSubmit={onSubmit} />);
    fireEvent.change(timeInputs()[MON_ARRIVAL]!, {
      target: { value: "07:30" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Anfrage senden" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1));
    expect(onSubmit).toHaveBeenCalledWith({
      weekdays: [{ weekday: 1, arrival: "07:30" }],
    });
  });
});
