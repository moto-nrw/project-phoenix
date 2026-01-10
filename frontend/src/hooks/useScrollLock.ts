import { useEffect, useRef } from "react";

/**
 * Custom hook to lock body scroll when a modal/popup is open
 * Handles scrollbar width compensation and iOS Safari
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

      // Apply styles to lock scroll
      const html = document.documentElement;
      const body = document.body;

      // Save original styles
      const originalHtmlStyle = html.style.cssText;
      const originalBodyStyle = body.style.cssText;

      // Lock scroll with position: fixed to preserve visual scroll position
      // This prevents the "jump to top" effect when modal opens
      html.style.cssText = `
        overflow: hidden;
        height: 100%;
      `;

      body.style.cssText = `
        position: fixed;
        top: -${scrollPosition.current}px;
        left: 0;
        right: 0;
        overflow-y: scroll;
        padding-right: ${scrollBarWidth}px;
      `;

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

      // For iOS Safari - prevent background scrolling with optimized check
      const handleTouchMove = (e: TouchEvent) => {
        const target = e.target;

        // Check if target is an Element before processing
        if (!(target instanceof Element)) {
          e.preventDefault();
          return;
        }

        // Check if we're inside a cached modal content element
        let element: Element | null = target;
        let isInsideModal = false;

        while (element && element !== document.body) {
          if (modalContentElements.current.has(element)) {
            isInsideModal = true;
            break;
          }
          element = element.parentElement;
        }

        if (!isInsideModal) {
          e.preventDefault();
        }
      };

      document.addEventListener("touchmove", handleTouchMove, {
        passive: false,
      });

      // Cleanup function
      return () => {
        // Restore original styles
        html.style.cssText = originalHtmlStyle;
        body.style.cssText = originalBodyStyle;

        // Restore scroll position
        globalThis.scrollTo(0, scrollPosition.current);

        // Remove touch event listener
        document.removeEventListener("touchmove", handleTouchMove);

        // Disconnect observer
        observer.disconnect();
      };
    }
  }, [isLocked]);
}
