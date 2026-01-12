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

      // Block scrollbar dragging via CSS (events don't catch this)
      // Note: Must set on documentElement (html) because globals.css sets overflow-y: scroll on html
      document.documentElement.style.overflow = "hidden";

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

      // Shared handler for wheel and touch scroll prevention
      const preventBackgroundScroll = (e: Event) => {
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
        if (scrollKeys.includes(e.key) && !isInsideModalContent(e.target)) {
          e.preventDefault();
        }
      };

      // Add event listeners with passive: false to allow preventDefault
      document.addEventListener("wheel", preventBackgroundScroll, {
        passive: false,
      });
      document.addEventListener("touchmove", preventBackgroundScroll, {
        passive: false,
      });
      document.addEventListener("keydown", handleKeyDown);

      // Cleanup function
      return () => {
        document.documentElement.style.overflow = "";
        document.removeEventListener("wheel", preventBackgroundScroll);
        document.removeEventListener("touchmove", preventBackgroundScroll);
        document.removeEventListener("keydown", handleKeyDown);
        observer.disconnect();
      };
    }
  }, [isLocked]);
}
