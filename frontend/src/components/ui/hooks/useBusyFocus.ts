"use client";

import { useLayoutEffect, useRef, type RefObject } from "react";

/** Disabled itself or inside a disabled fieldset (the busy footer). */
function isDisabledControl(element: HTMLElement): boolean {
  return (
    element.matches(":disabled") ||
    element.closest("fieldset:disabled") !== null
  );
}

/**
 * A busy dialog disables the control that started the operation (Speichern)
 * and the footer around it; browsers then drop focus to the page. Keeps focus
 * on the dialog `panelRef` (which needs `tabIndex={-1}`) while busy and hands
 * it back once the control is usable again, unless the dialog moved it on
 * meanwhile (first invalid field, #6533). Shared by `Modal` and `SlideOver`.
 */
export function useBusyFocus(
  panelRef: RefObject<HTMLElement | null>,
  busy: boolean,
): void {
  const busyFocusRef = useRef<HTMLElement | null>(null);
  useLayoutEffect(() => {
    const panel = panelRef.current;
    if (!panel) return;
    if (busy) {
      const active = document.activeElement;
      if (
        active instanceof HTMLElement &&
        panel.contains(active) &&
        isDisabledControl(active)
      ) {
        busyFocusRef.current = active;
        panel.focus({ preventScroll: true });
      }
      return;
    }
    const previous = busyFocusRef.current;
    busyFocusRef.current = null;
    if (
      previous &&
      document.activeElement === panel &&
      previous.isConnected &&
      !isDisabledControl(previous)
    ) {
      previous.focus({ preventScroll: true });
    }
  }, [panelRef, busy]);
}
