// Assertions for the FormModal pending contract (#6533): while a save runs,
// the dialog must not close through X, Escape, backdrop or a footer action.
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
  fireEvent.keyDown(document, { key: "Escape" });
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
