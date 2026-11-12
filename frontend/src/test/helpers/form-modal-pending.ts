// Assertions for the pending contract of FormModal (#6533) and SlideOver
// (#6593): while a save runs, the dialog must not close through X, Escape,
// backdrop, swipe or a footer action.
import { fireEvent, screen, within } from "@testing-library/react";
import { expect } from "vitest";

/** A promise the test settles by hand, to hold a save in flight. */
export function deferred<T = void>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

/**
 * Asserts that the open dialog `name` is locked by a running save: busy, X
 * and backdrop disabled, Escape starts no exit animation, and the named
 * footer actions are disabled. Returns the dialog.
 */
export function expectSaveLocksDialog(
  name: string | RegExp,
  footerActions: readonly (string | RegExp)[] = ["Abbrechen"],
): HTMLElement {
  const dialog = screen.getByRole("dialog", { name });
  expect(dialog).toHaveAttribute("aria-busy", "true");
  expect(
    within(dialog).getByRole("button", { name: "Modal schließen" }),
  ).toBeDisabled();
  expect(
    screen.getByRole("button", { name: /hintergrund.*schließen/i }),
  ).toBeDisabled();
  for (const action of footerActions) {
    expect(within(dialog).getByRole("button", { name: action })).toBeDisabled();
  }
  // Bubbles to the dialog's document listener.
  fireEvent.keyDown(dialog, { key: "Escape" });
  expect(dialog).not.toHaveClass("animate-modalExit");
  return dialog;
}

/** Asserts that the dialog `name` can be closed again after a failed save. */
export function expectDialogUnlocked(name: string | RegExp): HTMLElement {
  const dialog = screen.getByRole("dialog", { name });
  expect(dialog).not.toHaveAttribute("aria-busy");
  expect(
    within(dialog).getByRole("button", { name: "Modal schließen" }),
  ).toBeEnabled();
  return dialog;
}

/** The open kit `SlideOver` panel whose title is `title` (vaul mock). */
function slideOverPanel(title: string | RegExp): HTMLElement {
  const panel = screen
    .getByRole("heading", { name: title })
    .closest<HTMLElement>('[role="dialog"]');
  expect(panel).not.toBeNull();
  return panel as HTMLElement;
}

/**
 * Asserts that the open `SlideOver` titled `title` is locked by a running
 * save (#6593): busy, X disabled, no swipe, the named footer actions
 * disabled, and neither Escape nor a click on the backdrop closes it. Needs
 * `vi.mock("vaul", () => import("~/test/mocks/vaul"))`. Returns the panel.
 */
export function expectSlideOverSaveLocked(
  title: string | RegExp,
  footerActions: readonly (string | RegExp)[] = ["Abbrechen"],
  closeLabel: string | RegExp = "Schließen",
): HTMLElement {
  const panel = slideOverPanel(title);
  expect(panel).toHaveAttribute("aria-busy", "true");
  expect(
    within(panel).getByRole("button", { name: closeLabel }),
  ).toBeDisabled();
  expect(panel.closest("[data-dismissible]")).toHaveAttribute(
    "data-dismissible",
    "false",
  );
  for (const action of footerActions) {
    expect(within(panel).getByRole("button", { name: action })).toBeDisabled();
  }
  fireEvent.keyDown(panel, { key: "Escape" });
  const backdrop = document.querySelector("[data-vaul-overlay]");
  expect(backdrop).not.toBeNull();
  fireEvent.click(backdrop as Element);
  // The parent closes on onOpenChange(false); a locked panel stays.
  expect(slideOverPanel(title)).toBe(panel);
  return panel;
}

/** Asserts that the `SlideOver` titled `title` can be closed again. */
export function expectSlideOverUnlocked(
  title: string | RegExp,
  closeLabel: string | RegExp = "Schließen",
): HTMLElement {
  const panel = slideOverPanel(title);
  expect(panel).not.toHaveAttribute("aria-busy");
  expect(within(panel).getByRole("button", { name: closeLabel })).toBeEnabled();
  expect(panel.closest("[data-dismissible]")).toHaveAttribute(
    "data-dismissible",
    "true",
  );
  return panel;
}
