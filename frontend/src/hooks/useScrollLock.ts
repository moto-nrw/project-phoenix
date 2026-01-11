import { useEffect } from "react";

/**
 * Custom hook to lock body scroll when a modal/popup is open.
 *
 * Uses overflow: hidden with scrollbar-gutter: stable (set in globals.css)
 * to prevent all scrolling (including scrollbar dragging) without layout shifts.
 */
export function useScrollLock(isLocked: boolean) {
  useEffect(() => {
    if (typeof document === "undefined") return;

    if (isLocked) {
      document.body.classList.add("modal-open");

      return () => {
        document.body.classList.remove("modal-open");
      };
    }
  }, [isLocked]);
}
