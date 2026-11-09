"use client";

import { useEffect, type RefObject } from "react";

/** Below this the keyboard does not count as open (browser bars move too). */
const KEYBOARD_MIN_PX = 120;

/**
 * Keeps a bottom sheet directly above the phone keyboard.
 *
 * Vaul lifts a sheet by the keyboard height (`innerHeight - visualViewport
 * .height`) while a field in it has focus. The installed iPhone app also pans
 * the visual viewport down by that height, so the lift counted twice: the
 * sheet left the screen and the person typed into a field they could not see.
 * The distance that matters is the one between the layout viewport's bottom
 * and the visible bottom, which already includes the pan.
 */
export function useSheetAboveKeyboard(
  ref: RefObject<HTMLElement | null>,
  active: boolean,
) {
  useEffect(() => {
    const vv = window.visualViewport;
    if (!active || !vv) return;

    const fit = () => {
      const el = ref.current;
      if (!el) return;
      // A pinch zoom shrinks the visual viewport too; it is not a keyboard.
      const keyboardOpen =
        Math.abs(vv.scale - 1) < 0.01 &&
        window.innerHeight - vv.height > KEYBOARD_MIN_PX;
      if (!keyboardOpen) return;
      const hidden = window.innerHeight - vv.height - vv.offsetTop;
      el.style.bottom = `${Math.max(hidden, 0)}px`;
    };

    // Vaul writes its own value in the same resize event; correct it after.
    let frame = 0;
    const fitAfterVaul = () => {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(fit);
    };
    // The pan is not always reported as an event of its own, so a focus
    // change measures again while the keyboard moves.
    let timers: number[] = [];
    const refitWhileKeyboardMoves = () => {
      timers.forEach((timer) => window.clearTimeout(timer));
      timers = [100, 300, 600, 1000].map((delay) =>
        window.setTimeout(fit, delay),
      );
    };

    refitWhileKeyboardMoves();
    vv.addEventListener("resize", fitAfterVaul);
    vv.addEventListener("scroll", fitAfterVaul);
    document.addEventListener("focusin", refitWhileKeyboardMoves);
    return () => {
      vv.removeEventListener("resize", fitAfterVaul);
      vv.removeEventListener("scroll", fitAfterVaul);
      document.removeEventListener("focusin", refitWhileKeyboardMoves);
      window.cancelAnimationFrame(frame);
      timers.forEach((timer) => window.clearTimeout(timer));
    };
  }, [ref, active]);
}
