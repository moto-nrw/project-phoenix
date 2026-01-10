import { useEffect, useRef } from "react";

/**
 * Custom hook to lock body scroll when a modal/popup is open
 * Uses event prevention instead of CSS to avoid layout shifts
 */
export function useScrollLock(isLocked: boolean) {
  const scrollPosition = useRef(0);
  const modalContentElements = useRef<WeakSet<Element>>(new WeakSet());

  useEffect(() => {
    if (typeof globalThis === "undefined") return;

    if (isLocked) {
      // Save current scroll position
      scrollPosition.current = globalThis.pageYOffset;

      // Get scrollbar width before hiding it
      const scrollBarWidth =
        globalThis.innerWidth - document.documentElement.clientWidth;

      const body = document.body;

      // Save original padding
      const originalPaddingRight = body.style.paddingRight;

      // Only add padding to compensate for scrollbar, don't change overflow
      // This prevents layout shift when scrollbar disappears
      body.style.paddingRight = `${scrollBarWidth}px`;

      // Cache modal content elements for performance
      const updateModalContentCache = () => {
        modalContentElements.current = new WeakSet(
          document.querySelectorAll('[data-modal-content="true"]'),
        );
      };

      // Initial cache update
      updateModalContentCache();

      // Update cache when DOM changes (for dynamic modals)
      const observer = new MutationObserver(updateModalContentCache);
      observer.observe(document.body, {
        childList: true,
        subtree: true,
        attributes: true,
        attributeFilter: ["data-modal-content"],
      });

      // Check if element is inside modal content
      const isInsideModalContent = (target: EventTarget | null): boolean => {
        if (!(target instanceof Element)) return false;

        let element: Element | null = target;
        while (element && element !== document.body) {
          if (modalContentElements.current.has(element)) {
            return true;
          }
          element = element.parentElement;
        }
        return false;
      };

      // Prevent wheel scroll on background
      const handleWheel = (e: WheelEvent) => {
        if (!isInsideModalContent(e.target)) {
          e.preventDefault();
        }
      };

      // Prevent touch scroll on background (iOS Safari)
      const handleTouchMove = (e: TouchEvent) => {
        if (!isInsideModalContent(e.target)) {
          e.preventDefault();
        }
      };

      // Prevent keyboard scroll on background
      const handleKeyDown = (e: KeyboardEvent) => {
        const scrollKeys = [
          "ArrowUp",
          "ArrowDown",
          "PageUp",
          "PageDown",
          "Home",
          "End",
          " ",
        ];
        if (
          scrollKeys.includes(e.key) &&
          !isInsideModalContent(e.target)
        ) {
          e.preventDefault();
        }
      };

      // Add event listeners with passive: false to allow preventDefault
      document.addEventListener("wheel", handleWheel, { passive: false });
      document.addEventListener("touchmove", handleTouchMove, {
        passive: false,
      });
      document.addEventListener("keydown", handleKeyDown);

      // Cleanup function
      return () => {
        // Restore original padding
        body.style.paddingRight = originalPaddingRight;

        // Remove event listeners
        document.removeEventListener("wheel", handleWheel);
        document.removeEventListener("touchmove", handleTouchMove);
        document.removeEventListener("keydown", handleKeyDown);

        // Disconnect observer
        observer.disconnect();
      };
    }
  }, [isLocked]);
}
