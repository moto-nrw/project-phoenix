import { useLayoutEffect, useRef } from "react";

// Locks stack: a confirmation over a form modal holds two at once. Only the
// first lock records the page's own value and only the last release restores
// it. Per-lock bookkeeping wrote "hidden" back when the lower modal released
// before the upper one, and the page stayed unscrollable until reload.
let activeLocks = 0;
let overflowBeforeFirstLock = "";

/**
 * Custom hook to lock body scroll when a modal/popup is open
 * Relies on scrollbar-gutter: stable in globals.css to prevent layout shift
 */
export function useScrollLock(isLocked: boolean) {
  const modalContentElements = useRef<WeakSet<Element>>(new WeakSet());

  useLayoutEffect(() => {
    if (typeof globalThis === "undefined") return;

    if (isLocked) {
      // Simply hide overflow - scrollbar-gutter: stable in globals.css
      // prevents layout shift from scrollbar disappearing. Capture the prior
      // value so cleanup RESTORES it instead of forcing "" — otherwise closing
      // a modal opened over a page whose inline overflow someone else set
      // would clear it. (The chat locks through `data-chat-page-lock` and
      // CSS, independent of this value.)
      if (activeLocks === 0) {
        overflowBeforeFirstLock = document.documentElement.style.overflow;
      }
      activeLocks += 1;
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
        activeLocks -= 1;
        if (activeLocks === 0) {
          document.documentElement.style.overflow = overflowBeforeFirstLock;
        }
        document.removeEventListener("wheel", preventBackgroundScroll);
        document.removeEventListener("touchmove", preventBackgroundScroll);
        document.removeEventListener("keydown", handleKeyDown);
        observer.disconnect();
      };
    }
  }, [isLocked]);
}
