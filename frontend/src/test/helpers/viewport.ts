import { act } from "@testing-library/react";
import type { Window as HappyWindow } from "happy-dom";
import { afterEach, beforeEach } from "vitest";

/** iPhone 12/13/14 width, below Tailwind's `md` breakpoint. */
export const PHONE_WIDTH = 390;
/** The shared desktop width every DOM test file starts with. */
export const DESKTOP_WIDTH = 1920;

/**
 * Resizes the Happy DOM window. Its `matchMedia` follows the new size and
 * notifies `change` listeners, so components that pick a layout by media
 * query (DataTable's `stackedOnMobile`) switch as in a browser.
 *
 * Happy DOM quirk: a `change` listener starts from `matches === false`, not
 * from the real value. A query that matched when the listener was added
 * (phone width, `max-width`) reports no change when it stops matching.
 * Narrowing a mounted component works; to widen one, unmount it, resize and
 * render it again.
 */
export function setViewportWidth(width: number): void {
  const { happyDOM } = window as unknown as HappyWindow;
  act(() => {
    happyDOM.setViewport({ width, height: window.innerHeight });
  });
}

/**
 * Runs every test of the enclosing `describe` at phone width. The worker
 * only restores the window after the file, so this resets it per test.
 */
export function usePhoneViewport(): void {
  beforeEach(() => setViewportWidth(PHONE_WIDTH));
  afterEach(() => setViewportWidth(DESKTOP_WIDTH));
}
