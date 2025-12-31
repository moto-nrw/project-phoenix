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
 */
export const stopPropagation = {
  onClick: (e: React.MouseEvent) => e.stopPropagation(),
  onKeyDown: (e: React.KeyboardEvent) => e.stopPropagation(),
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
