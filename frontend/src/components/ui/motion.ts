/**
 * Durations of kit animations whose end a timer waits for. Each value is the
 * one source for the CSS that animates and the timer that acts afterwards,
 * so the two cannot drift apart again (#6575).
 */

/** Entrance of the kit `Modal` panel (`.animate-modalEnter`). */
export const MODAL_ENTER_MS = 250;

/**
 * Exit of the kit `Modal` panel (`.animate-modalExit`). `Modal` calls
 * `onClose` once it has run.
 */
export const MODAL_EXIT_MS = 200;

/**
 * CSS custom properties `Modal` sets on its root; `globals.css` reads them in
 * `.animate-modalEnter` / `.animate-modalExit`.
 */
export const MODAL_MOTION_STYLE = {
  "--moto-modal-enter": `${MODAL_ENTER_MS}ms`,
  "--moto-modal-exit": `${MODAL_EXIT_MS}ms`,
} as const;

/**
 * Fade of a toast. The Tailwind class is the source: Tailwind only emits a
 * class it finds spelled out, so the milliseconds are read from it.
 */
export const TOAST_FADE_CLASS = "duration-300";
export const TOAST_FADE_MS = Number(TOAST_FADE_CLASS.slice("duration-".length));
