import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { PickupTimeModal } from "./child-care";
import type { CareException } from "~/lib/parent-api";

// Regression coverage for the failed-preload save path: when the care-exception
// list never loaded (careExceptionsLoaded=false), the modal must not let a
// parent save. submitCareException treats an omitted leg as an authoritative
// clear, so saving from an unknown state could silently wipe an existing
// override the UI never managed to prefill.

const LOAD_ERROR_DE =
  "Die aktuell hinterlegten Zeiten konnten nicht geladen werden. Bitte laden Sie die Seite neu und versuchen Sie es erneut.";

function renderModal(
  overrides: Partial<React.ComponentProps<typeof PickupTimeModal>> = {},
) {
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const onRemove = vi.fn().mockResolvedValue(undefined);
  const onClose = vi.fn();
  const { container } = render(
    <PickupTimeModal
      careExceptions={[]}
      careExceptionsLoaded
      pickupChangeEnabled
      onClose={onClose}
      onSubmit={onSubmit}
      onRemove={onRemove}
      {...overrides}
    />,
  );
  const dateInput =
    container.querySelector<HTMLInputElement>('input[type="date"]')!;
  const timeInputs = Array.from(
    container.querySelectorAll<HTMLInputElement>('input[type="time"]'),
  );
  // Field order in the modal is arrival, then pickup.
  const [arrivalInput, pickupInput] = timeInputs;
  return { onSubmit, onRemove, onClose, dateInput, arrivalInput, pickupInput };
}

describe("PickupTimeModal — failed preload guard", () => {
  it("blocks saving and warns when the exception list failed to load", () => {
    const { onSubmit } = renderModal({ careExceptionsLoaded: false });

    // The load-failure notice is shown.
    expect(screen.getByText(LOAD_ERROR_DE)).toBeInTheDocument();

    // The save button is disabled so an omitted leg can't be sent as a clear.
    const saveButton = screen.getByRole("button", { name: "Speichern" });
    expect(saveButton).toBeDisabled();

    // Even if a click slips through, handleSubmit refuses to submit.
    fireEvent.click(saveButton);
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("allows saving once the exception list is known", () => {
    const { onSubmit, pickupInput } = renderModal({
      careExceptionsLoaded: true,
    });

    // No load-failure notice when the list loaded.
    expect(screen.queryByText(LOAD_ERROR_DE)).not.toBeInTheDocument();

    fireEvent.change(pickupInput!, { target: { value: "14:30" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ pickupTime: "14:30" }),
    );
  });

  it("preserves an existing arrival leg the parent did not touch", () => {
    // A loaded override with both legs: the parent edits pickup only. Because
    // the list loaded, the arrival field is prefilled and travels back intact
    // rather than being cleared.
    const existing: CareException = {
      date: "2026-03-17",
      pickup_time: "15:00",
      arrival_time: "08:30",
      source: "guardian",
      updated_at: "2026-03-10T09:00:00Z",
    };
    const { onSubmit, dateInput, pickupInput } = renderModal({
      careExceptions: [existing],
      careExceptionsLoaded: true,
    });

    // Point the picker at the override's date so the fields prefill from it.
    fireEvent.change(dateInput, { target: { value: "2026-03-17" } });

    // Change only the pickup time; arrival stays at its prefilled value.
    fireEvent.change(pickupInput!, { target: { value: "16:00" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(onSubmit).toHaveBeenCalledWith({
      date: "2026-03-17",
      pickupTime: "16:00",
      arrivalTime: "08:30",
    });
  });
});
