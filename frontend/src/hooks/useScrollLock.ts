import { useEffect, useRef } from "react";

/**
 * Custom hook to lock body scroll when a modal/popup is open.
 *
 * Uses the position-fixed approach to completely prevent scrolling
 * (including scrollbar dragging) while avoiding layout shifts:
 * - Sets body to position: fixed with top offset matching scroll position
 * - Keeps overflow-y: scroll to maintain scrollbar gutter
 * - Restores scroll position on unlock
 */
export function useScrollLock(isLocked: boolean) {
  const scrollPositionRef = useRef(0);

  useEffect(() => {
    if (typeof globalThis === "undefined") return;

    if (isLocked) {
      // Save current scroll position
      scrollPositionRef.current = globalThis.scrollY;

      // Set CSS variable for scroll offset (used by modal-open class)
      document.documentElement.style.setProperty(
        "--scroll-y",
        `${scrollPositionRef.current}px`,
      );

      // Add modal-open class to body (CSS handles the rest)
      document.body.classList.add("modal-open");

      // Cleanup function
      return () => {
        // Remove modal-open class
        document.body.classList.remove("modal-open");

        // Remove CSS variable
        document.documentElement.style.removeProperty("--scroll-y");

        // Restore scroll position
        globalThis.scrollTo(0, scrollPositionRef.current);
      };
    }
  }, [isLocked]);
}
