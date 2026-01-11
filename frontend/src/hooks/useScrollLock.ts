import { useEffect } from "react";

/**
 * Custom hook to lock body scroll when a modal/popup is open.
 *
 * Uses overflow: hidden with scrollbar-gutter: stable (set in globals.css)
 * to prevent scrolling while avoiding layout shifts.
 *
 * This simple approach works because:
 * - scrollbar-gutter: stable reserves scrollbar space even when hidden
 * - overflow: hidden on body prevents all scrolling (including scrollbar drag)
 * - No position: fixed needed, so sticky elements (header, sidebar) work correctly
 */
export function useScrollLock(isLocked: boolean) {
  useEffect(() => {
    if (typeof document === "undefined") return;

    if (isLocked) {
      // Add modal-open class to body (CSS handles overflow: hidden)
      document.body.classList.add("modal-open");

      // Cleanup function
      return () => {
        document.body.classList.remove("modal-open");
      };
    }
  }, [isLocked]);
}
