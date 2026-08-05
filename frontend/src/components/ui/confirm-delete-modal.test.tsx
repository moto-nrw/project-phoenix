import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ConfirmDeleteModal } from "./confirm-delete-modal";

// The two escapes this dialog needs to stay usable are structural, not visual,
// so they are pinned here: it must leave the card it is declared in (portal),
// and it must opt back into pointer events when a Radix/Vaul dialog has turned
// them off on the body.

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
});
