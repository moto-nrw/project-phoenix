"use client";

import { useEffect } from "react";
import type { RefObject } from "react";

/**
 * Calls `onDismiss` when a pointer press lands outside `ref`, or when the
 * Escape key is pressed. Centralises the click-outside + Escape handling that
 * the timetable's three header dropdowns (density menu, period switcher,
 * planning menu) each used to hand-roll in their own `useEffect`.
 *
 * Pass the element that wraps BOTH the trigger and the popover as `ref` so
 * clicking the trigger to close does not register as an outside press.
 *
 * @param ref       Container element ref (trigger + popover).
 * @param onDismiss Called on an outside press or Escape.
 * @param enabled   Gate the listeners — pass the open state so the handlers
 *                  only run while the menu is open. Defaults to `true`.
 *
 * SSR-safe: the effect only runs in the browser.
 */
export function useClickOutside<T extends HTMLElement>(
  ref: RefObject<T | null>,
  onDismiss: () => void,
  enabled = true,
): void {
  useEffect(() => {
    if (!enabled) return;

    const handlePointer = (event: MouseEvent | TouchEvent) => {
      const el = ref.current;
      if (el && event.target instanceof Node && !el.contains(event.target)) {
        onDismiss();
      }
    };

    const handleKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onDismiss();
    };

    document.addEventListener("mousedown", handlePointer);
    document.addEventListener("touchstart", handlePointer, { passive: true });
    document.addEventListener("keydown", handleKey);

    return () => {
      document.removeEventListener("mousedown", handlePointer);
      document.removeEventListener("touchstart", handlePointer);
      document.removeEventListener("keydown", handleKey);
    };
  }, [ref, onDismiss, enabled]);
}
