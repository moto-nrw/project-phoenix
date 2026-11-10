// #6533: Schließen während der Übernahme ist gesperrt (bisher nur der
// Hintergrund), eine späte Antwort schließt keinen anderen Dialog.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
}));

import { CareWeeklyPlanModal } from "./care-weekly-plan-modal";

const TITLE = "Wochenplan festlegen";

function renderAndSubmit() {
  const save = deferred();
  const onSubmit = vi.fn(() => save.promise);
  const onClose = vi.fn();
  const view = render(
    <CareWeeklyPlanModal
      isOpen
      onClose={onClose}
      careDaysSource="weekly_plan"
      initialArrivalSchedules={[
        { weekday: 1, inCare: true, expected_arrival: "08:00", notes: null },
      ]}
      initialPickupSchedules={[{ weekday: 1, pickupTime: "15:00" }]}
      onSubmit={onSubmit}
      successMessage="Betreuungszeiten übernommen"
    />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Übernehmen" }));
  return { ...view, save, onSubmit, onClose };
}

describe("CareWeeklyPlanModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps X, Escape and Abbrechen shut while saving, not only the backdrop", async () => {
    const { save, onClose } = renderAndSubmit();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      save.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes nothing when the answer arrives after unmount", async () => {
    const { save, onSubmit, onClose, unmount } = renderAndSubmit();
    expect(onSubmit).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      save.resolve();
    });

    expect(onClose).not.toHaveBeenCalled();
  });
});
