import { useEffect, useRef } from 'react';

/**
 * Custom hook to lock body scroll when a modal/popup is open
 * Handles scrollbar width compensation and iOS Safari
 */
export function useScrollLock(isLocked: boolean) {
  const scrollPosition = useRef(0);

  useEffect(() => {
    if (typeof window === 'undefined') return;

    if (isLocked) {
      // Save current scroll position
      scrollPosition.current = window.pageYOffset;

      // Get scrollbar width before hiding it
      const scrollBarWidth = window.innerWidth - document.documentElement.clientWidth;

      // Apply styles to lock scroll
      const html = document.documentElement;
      const body = document.body;

      // Save original styles
      const originalHtmlStyle = html.style.cssText;
      const originalBodyStyle = body.style.cssText;

      // Lock scroll with proper iOS support
      html.style.cssText = `
        position: relative;
        overflow: hidden;
        height: 100%;
      `;

      body.style.cssText = `
        position: relative;
        overflow: hidden;
        height: 100%;
        padding-right: ${scrollBarWidth}px;
      `;

      // For iOS Safari - prevent background scrolling
      const handleTouchMove = (e: TouchEvent) => {
        // Allow scrolling inside the modal
        const target = e.target as HTMLElement;
        const isModalContent = target.closest('[data-modal-content="true"]');
        
        if (!isModalContent) {
          e.preventDefault();
        }
      };

      document.addEventListener('touchmove', handleTouchMove, { passive: false });

      // Cleanup function
      return () => {
        // Restore original styles
        html.style.cssText = originalHtmlStyle;
        body.style.cssText = originalBodyStyle;

        // Restore scroll position
        window.scrollTo(0, scrollPosition.current);

        // Remove touch event listener
        document.removeEventListener('touchmove', handleTouchMove);
      };
    }
  }, [isLocked]);
}