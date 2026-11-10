// #6533: Schließen während des Speicherns ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Mit dem echten FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

import { StaffNoticeModal } from "./staff-notice-modal";

function renderAndSave() {
  const save = deferred();
  const onSubmit = vi.fn(() => save.promise);
  const onClose = vi.fn();
  const view = render(
    <StaffNoticeModal
      isOpen
      notice={null}
      onClose={onClose}
      onSubmit={onSubmit}
    />,
  );
  fireEvent.change(screen.getByLabelText("Titel"), {
    target: { value: "Dienstbesprechung" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  return { ...view, save, onSubmit, onClose };
}

describe("StaffNoticeModal pending lock (#6533)", () => {
  it("keeps the dialog shut while saving and unlocks it after a failure", async () => {
    const { save, onClose } = renderAndSave();

    expectSaveLocksDialog("Neue Tagesinformation");

    await act(async () => {
      save.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Neue Tagesinformation");
    expect(screen.getByLabelText("Titel")).toHaveValue("Dienstbesprechung");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes nothing when the answer arrives after unmount", async () => {
    const { save, onSubmit, onClose, unmount } = renderAndSave();
    expect(onSubmit).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      save.resolve();
    });

    expect(onClose).not.toHaveBeenCalled();
  });
});
