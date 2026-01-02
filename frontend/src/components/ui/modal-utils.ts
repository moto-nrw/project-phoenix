import type React from "react";

/**
 * Creates keyboard handler for modal backdrop that mirrors click-to-close behavior.
 * Accessibility: allows Enter/Space to close modal when backdrop is focused.
 */
export function createBackdropKeyHandler(onClose: () => void) {
  return (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      onClose();
    }
  };
}

/**
 * Stops event propagation for both click and keyboard events.
 * Used on modal content to prevent backdrop close when interacting with modal.
 * Note: Escape key is allowed to bubble so document-level close handler works.
 */
export const stopPropagation = {
  onClick: (e: React.MouseEvent) => e.stopPropagation(),
  onKeyDown: (e: React.KeyboardEvent) => {
    // Allow Escape to bubble up to document-level handler for modal close
    if (e.key !== "Escape") {
      e.stopPropagation();
    }
  },
};

/**
 * Common ARIA props for modal backdrop.
 */
export const backdropAriaProps = {
  role: "button" as const,
  tabIndex: -1,
  "aria-label": "Hintergrund - Klicken zum Schließen",
};

/**
 * Common ARIA props for modal dialog container.
 */
export const dialogAriaProps = {
  role: "dialog" as const,
  "aria-modal": true,
};

/**
 * Returns the appropriate animation class for modal enter/exit transitions.
 * Used for consistent animation behavior across all modals.
 */
export function getModalAnimationClass(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  if (isAnimating && !isExiting) return "animate-modalEnter";
  if (isExiting) return "animate-modalExit";
  return "translate-y-8 scale-75 -rotate-1 opacity-0";
}

/**
 * Creates keyboard handler for interactive elements that should respond to Enter/Space.
 * Accessibility: makes clickable divs work with keyboard navigation.
 *
 * @example
 * <div
 *   role="button"
 *   tabIndex={0}
 *   onClick={handleClick}
 *   onKeyDown={createInteractiveKeyHandler(handleClick)}
 * >
 */
export function createInteractiveKeyHandler<
  T extends (...args: unknown[]) => unknown,
>(callback: T) {
  return (e: React.KeyboardEvent<HTMLDivElement>) => {
    if (e.key === "Enter" || e.key === " ") {
      e.preventDefault();
      callback();
    }
  };
}

/**
 * Returns the className for modal backdrop based on animation state.
 * Used for consistent backdrop styling across all modals.
 */
export function getBackdropClassName(
  isAnimating: boolean,
  isExiting: boolean,
): string {
  const bgClass = isAnimating && !isExiting ? "bg-black/40" : "bg-black/0";
  return `fixed inset-0 z-[9999] flex items-center justify-center transition-all duration-400 ease-out ${bgClass}`;
}

/**
 * Returns the style object for modal backdrop.
 */
export function getBackdropStyle(
  isAnimating: boolean,
  isExiting: boolean,
): React.CSSProperties {
  return {
    position: "fixed",
    top: 0,
    left: 0,
    right: 0,
    bottom: 0,
    animation:
      isAnimating && !isExiting ? "backdropEnter 400ms ease-out" : undefined,
  };
}

/**
 * Common glassmorphism style for modal containers.
 */
export const modalContainerStyle: React.CSSProperties = {
  background:
    "linear-gradient(135deg, rgba(255,255,255,0.95) 0%, rgba(248,250,252,0.98) 100%)",
  backdropFilter: "blur(20px)",
  boxShadow:
    "0 25px 50px -12px rgba(0, 0, 0, 0.25), 0 8px 16px -8px rgba(80, 128, 216, 0.15)",
  animationFillMode: "both",
};
