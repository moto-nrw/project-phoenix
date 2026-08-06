import { act, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ConfirmDeleteModal } from "./confirm-delete-modal";
import { Modal } from "./modal";
import { ModalProvider } from "../dashboard/modal-context";

// The three escapes this dialog needs to stay usable are structural, not
// visual, so they are pinned here: it must leave the card it is declared in
// (portal), it must opt back into pointer events when a Radix/Vaul dialog has
// turned them off on the body, and it must own its focus trap so the parent
// dialog it was opened from stops pulling focus back out of it.

function renderInsideCard(pointerEventsOnBody?: string) {
  if (pointerEventsOnBody !== undefined) {
    document.body.style.pointerEvents = pointerEventsOnBody;
  }

  const result = render(
    <div className="moto-content-surface overflow-hidden">
      <ConfirmDeleteModal
        isOpen
        title="Eintrag löschen"
        description="Diese Aktion kann nicht rückgängig gemacht werden."
        gate={{ mode: "twoStep" }}
        onConfirm={vi.fn()}
        onClose={vi.fn()}
        loading={false}
        error=""
      />
    </div>,
  );

  return result;
}

function getOverlay() {
  const dialog = screen
    .getByText("Eintrag löschen")
    .closest("div")?.parentElement;
  expect(dialog).not.toBeNull();
  return dialog as HTMLElement;
}

describe("ConfirmDeleteModal", () => {
  it("renders into document.body instead of the card it is declared in", () => {
    const { container } = renderInsideCard();

    // A `fixed` overlay left inside a moto-content-surface is positioned
    // against that card (backdrop-filter) and clipped by its overflow-hidden.
    expect(container.querySelector(".fixed")).toBeNull();
    expect(screen.getByText("Eintrag löschen")).toBeInTheDocument();
    expect(document.body.contains(getOverlay())).toBe(true);
  });

  it("stays interactive when the body has pointer events disabled", () => {
    renderInsideCard("none");

    expect(getOverlay().style.pointerEvents).toBe("auto");

    document.body.style.pointerEvents = "";
  });

  it("holds focus on its own controls when opened from a focus-trapped Modal", async () => {
    // The parent Modal traps focus. Portaled to the body, this dialog counts as
    // "outside" that trap, so without its own scope the parent redirects every
    // focus attempt back into itself: the confirm button is unreachable by
    // keyboard and a textConfirm input cannot hold what the user types.
    //
    // The dialog is opened by a rerender rather than mounted open, because that
    // is the real sequence — the user clicks "Löschen" inside an already-open
    // Modal. Radix pauses whichever scope is lower on its stack, so the order
    // in which the two scopes mount is exactly what the fix relies on.
    function Harness({ confirmOpen }: { readonly confirmOpen: boolean }) {
      return (
        <ModalProvider>
          <Modal isOpen onClose={vi.fn()} title="Eintrag bearbeiten">
            <p>Formularinhalt</p>
            <ConfirmDeleteModal
              isOpen={confirmOpen}
              title="Eintrag löschen"
              description="Diese Aktion kann nicht rückgängig gemacht werden."
              gate={{
                mode: "textConfirm",
                expected: "LÖSCHEN",
                inputId: "confirm-delete-input",
                label: "Zum Bestätigen LÖSCHEN eingeben",
              }}
              onConfirm={vi.fn()}
              onClose={vi.fn()}
              loading={false}
              error=""
            />
          </Modal>
        </ModalProvider>
      );
    }

    const { rerender } = render(<Harness confirmOpen={false} />);
    await act(async () => {
      rerender(<Harness confirmOpen />);
    });

    const input = screen.getByLabelText("Zum Bestätigen LÖSCHEN eingeben");
    await act(async () => {
      input.focus();
    });
    expect(document.activeElement).toBe(input);

    const cancelButton = screen.getByRole("button", { name: "Abbrechen" });
    await act(async () => {
      cancelButton.focus();
    });
    expect(document.activeElement).toBe(cancelButton);
  });
});
